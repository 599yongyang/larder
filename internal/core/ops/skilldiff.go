package ops

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"larder/internal/core/model"
	"larder/internal/platform"
)

type fileFingerprint struct {
	size   int64
	hash   string
	binary bool
}

// BuildSkillUpdateDiff downloads the upstream Skill and compares it with the
// current local folder and its installation baseline. Read-only.
func BuildSkillUpdateDiff(ctx context.Context, skill model.Skill, repo string, fetcher platform.Fetcher, baseline map[string]string) (model.SkillUpdateDiff, error) {
	plan, err := PlanSkillUpdate(skill, repo)
	if err != nil {
		return model.SkillUpdateDiff{}, err
	}
	if fetcher == nil {
		fetcher = platform.HTTPFetcher{}
	}
	type localScan struct {
		files map[string]fileFingerprint
		err   error
	}
	localCh := make(chan localScan, 1)
	go func() {
		files, scanErr := fingerprintTree(skill.Path)
		localCh <- localScan{files: files, err: scanErr}
	}()
	tarball, err := fetcher.Fetch(ctx, plan.SourceURL)
	if err != nil {
		return model.SkillUpdateDiff{}, err
	}
	incomingDir, err := os.MkdirTemp("", "larder-update-")
	if err != nil {
		return model.SkillUpdateDiff{}, err
	}
	defer os.RemoveAll(incomingDir)
	if err := extractSkillFolder(tarball, plan.SourcePath, incomingDir); err != nil {
		return model.SkillUpdateDiff{}, err
	}
	incoming, err := fingerprintTree(incomingDir)
	if err != nil {
		return model.SkillUpdateDiff{}, err
	}
	localResult := <-localCh
	if localResult.err != nil {
		return model.SkillUpdateDiff{}, localResult.err
	}
	local := localResult.files
	diff := model.SkillUpdateDiff{SkillID: skill.ID, SkillName: skill.Name, Repo: repo, Files: []model.SkillUpdateFile{}, HasBaseline: len(baseline) > 0, ScannedAt: time.Now()}
	paths := map[string]bool{}
	for path := range local {
		paths[path] = true
	}
	for path := range incoming {
		paths[path] = true
	}
	for path := range baseline {
		paths[path] = true
	}
	for path := range paths {
		localFile, localOK := local[path]
		incomingFile, incomingOK := incoming[path]
		baseHash, baseOK := baseline[path]
		file := model.SkillUpdateFile{Path: path}
		switch {
		case !incomingOK && localOK:
			if diff.HasBaseline && baseOK {
				file.LocalChanged = localFile.hash != baseHash
				if file.LocalChanged {
					file.Status = "conflict"
					diff.Conflicted++
				} else {
					file.Status = "removed"
				}
			} else {
				file.Status = "local"
				diff.LocalOnly++
			}
			file.LocalSize = localFile.size
			file.Binary = localFile.binary
			if !file.Binary {
				content, _ := os.ReadFile(filepath.Join(skill.Path, filepath.FromSlash(path)))
				file.Lines, file.Truncated = lineDiffBytes(content, nil)
				file.Additions, file.Deletions = countDiffLines(file.Lines)
			}
		case incomingOK && !localOK:
			file.UpstreamChanged = true
			file.IncomingSize = incomingFile.size
			file.Binary = incomingFile.binary
			if !file.Binary {
				content, _ := os.ReadFile(filepath.Join(incomingDir, filepath.FromSlash(path)))
				file.Lines, file.Truncated = lineDiffBytes(nil, content)
				file.Additions, file.Deletions = countDiffLines(file.Lines)
			}
			if diff.HasBaseline && baseOK && incomingFile.hash != baseHash {
				file.Status = "conflict"
				file.LocalChanged = true
				diff.Conflicted++
			} else {
				file.Status = "added"
				diff.Added++
			}
		case incomingOK && localOK:
			if localFile.hash == incomingFile.hash {
				continue
			}
			file.LocalSize = localFile.size
			file.IncomingSize = incomingFile.size
			file.Binary = localFile.binary || incomingFile.binary
			if diff.HasBaseline {
				file.LocalChanged = !baseOK || localFile.hash != baseHash
				file.UpstreamChanged = !baseOK || incomingFile.hash != baseHash
				switch {
				case file.LocalChanged && file.UpstreamChanged:
					file.Status = "conflict"
					diff.Conflicted++
				case file.LocalChanged:
					file.Status = "local_modified"
				default:
					file.Status = "modified"
					diff.Modified++
				}
			} else {
				file.Status = "modified"
				diff.Modified++
			}
			if !file.Binary {
				file.Lines, file.Truncated = lineDiff(
					filepath.Join(skill.Path, filepath.FromSlash(path)),
					filepath.Join(incomingDir, filepath.FromSlash(path)),
				)
				add, del := countDiffLines(file.Lines)
				file.Additions, file.Deletions = add, del
			}
		default:
			continue
		}
		diff.Files = append(diff.Files, file)
	}
	sort.Slice(diff.Files, func(i, j int) bool {
		rank := map[string]int{"conflict": 0, "modified": 1, "removed": 2, "added": 3, "local_modified": 4, "local": 5}
		if rank[diff.Files[i].Status] != rank[diff.Files[j].Status] {
			return rank[diff.Files[i].Status] < rank[diff.Files[j].Status]
		}
		return diff.Files[i].Path < diff.Files[j].Path
	})
	return diff, nil
}

func fingerprintTree(root string) (map[string]fileFingerprint, error) {
	result := map[string]fileFingerprint{}
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		relative = filepath.ToSlash(relative)
		if relative == "." || relative == "" {
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		file, err := os.Open(path)
		if err != nil {
			return err
		}
		hash := sha256.New()
		first := make([]byte, 512)
		firstN := 0
		buf := make([]byte, 32<<10)
		var read int64
		for {
			n, readErr := file.Read(buf)
			if n > 0 {
				if firstN < len(first) {
					copied := copy(first[firstN:], buf[:n])
					firstN += copied
				}
				if read < 64<<20 {
					remaining := int64(64<<20) - read
					if int64(n) > remaining {
						n = int(remaining)
					}
					_, _ = hash.Write(buf[:n])
					read += int64(n)
				}
			}
			if readErr == io.EOF {
				break
			}
			if readErr != nil {
				_ = file.Close()
				return readErr
			}
		}
		_ = file.Close()
		result[relative] = fileFingerprint{size: info.Size(), hash: hex.EncodeToString(hash.Sum(nil)), binary: isBinary(first[:firstN])}
		return nil
	})
	return result, err
}

func isBinary(data []byte) bool {
	for _, value := range data {
		if value == 0 {
			return true
		}
	}
	return false
}

func countDiffLines(lines []model.DiffLine) (int, int) {
	add, del := 0, 0
	for _, line := range lines {
		if line.Kind == "add" {
			add++
		}
		if line.Kind == "del" {
			del++
		}
	}
	return add, del
}

func lineDiff(localPath, incomingPath string) ([]model.DiffLine, bool) {
	local, err := os.ReadFile(localPath)
	if err != nil {
		return nil, false
	}
	incoming, err := os.ReadFile(incomingPath)
	if err != nil {
		return nil, false
	}
	return lineDiffBytes(local, incoming)
}

func lineDiffBytes(local, incoming []byte) ([]model.DiffLine, bool) {
	a, b := splitLines(local), splitLines(incoming)
	if len(a) > 2000 || len(b) > 2000 {
		return nil, true
	}
	prefix := 0
	for prefix < len(a) && prefix < len(b) && a[prefix] == b[prefix] {
		prefix++
	}
	suffix := 0
	for suffix < len(a)-prefix && suffix < len(b)-prefix && a[len(a)-1-suffix] == b[len(b)-1-suffix] {
		suffix++
	}
	midA := a[prefix : len(a)-suffix]
	midB := b[prefix : len(b)-suffix]
	dp := make([][]int, len(midA)+1)
	for i := range dp {
		dp[i] = make([]int, len(midB)+1)
	}
	for i := 1; i <= len(midA); i++ {
		for j := 1; j <= len(midB); j++ {
			if midA[i-1] == midB[j-1] {
				dp[i][j] = dp[i-1][j-1] + 1
			} else if dp[i-1][j] >= dp[i][j-1] {
				dp[i][j] = dp[i-1][j]
			} else {
				dp[i][j] = dp[i][j-1]
			}
		}
	}
	lines := make([]model.DiffLine, 0, len(a)+len(b))
	for index := 0; index < prefix; index++ {
		lines = append(lines, model.DiffLine{Kind: "context", OldLine: index + 1, NewLine: index + 1, Text: a[index]})
	}
	i, j := len(midA), len(midB)
	midLines := make([]model.DiffLine, 0, len(midA)+len(midB))
	for i > 0 || j > 0 {
		switch {
		case i > 0 && j > 0 && midA[i-1] == midB[j-1]:
			midLines = append(midLines, model.DiffLine{Kind: "context", OldLine: prefix + i, NewLine: prefix + j, Text: midA[i-1]})
			i--
			j--
		case j > 0 && (i == 0 || dp[i][j-1] >= dp[i-1][j]):
			midLines = append(midLines, model.DiffLine{Kind: "add", NewLine: prefix + j, Text: midB[j-1]})
			j--
		default:
			midLines = append(midLines, model.DiffLine{Kind: "del", OldLine: prefix + i, Text: midA[i-1]})
			i--
		}
	}
	for left, right := 0, len(midLines)-1; left < right; left, right = left+1, right-1 {
		midLines[left], midLines[right] = midLines[right], midLines[left]
	}
	lines = append(lines, midLines...)
	for index := 0; index < suffix; index++ {
		oldLine := len(a) - suffix + index + 1
		newLine := len(b) - suffix + index + 1
		lines = append(lines, model.DiffLine{Kind: "context", OldLine: oldLine, NewLine: newLine, Text: a[len(a)-suffix+index]})
	}
	return lines, false
}

func splitLines(content []byte) []string {
	text := strings.ReplaceAll(string(content), "\r\n", "\n")
	if text == "" {
		return nil
	}
	return strings.Split(strings.TrimSuffix(text, "\n"), "\n")
}

// FileHashes returns a stable path→SHA-256 map for a folder. It is used to
// capture a new baseline after an install or update.
func FileHashes(root string) (map[string]string, error) {
	files, err := fingerprintTree(root)
	if err != nil {
		return nil, err
	}
	result := make(map[string]string, len(files))
	for path, file := range files {
		result[path] = file.hash
	}
	return result, nil
}

func WriteBaselineManifest(snapshotDir, sourceDir string) error {
	files, err := FileHashes(sourceDir)
	if err != nil {
		return err
	}
	raw, err := json.Marshal(files)
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(snapshotDir, "baseline.json"), raw, 0o600)
}

func LoadBaselineManifest(snapshotDir string) (map[string]string, error) {
	raw, err := os.ReadFile(filepath.Join(snapshotDir, "baseline.json"))
	if err != nil {
		return nil, err
	}
	var files map[string]string
	if err := json.Unmarshal(raw, &files); err != nil {
		return nil, err
	}
	return files, nil
}
