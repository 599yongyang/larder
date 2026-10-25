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
	"sync"
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
func BuildSkillUpdateDiff(ctx context.Context, skill model.Skill, repo string, fetcher platform.Fetcher, baseline map[string]string, includeLines ...bool) (model.SkillUpdateDiff, error) {
	withLines := true
	if len(includeLines) > 0 {
		withLines = includeLines[0]
	}
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
	incomingDir, err := upstreamTree(ctx, plan, fetcher)
	if err != nil {
		return model.SkillUpdateDiff{}, err
	}
	if !cacheableFetcher(fetcher) {
		defer os.RemoveAll(incomingDir)
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
			if withLines && !file.Binary {
				content, _ := os.ReadFile(filepath.Join(skill.Path, filepath.FromSlash(path)))
				file.Lines, file.Truncated = lineDiffBytes(content, nil)
				file.Additions, file.Deletions = countDiffLines(file.Lines)
			}
		case incomingOK && !localOK:
			file.UpstreamChanged = true
			file.IncomingSize = incomingFile.size
			file.Binary = incomingFile.binary
			if withLines && !file.Binary {
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
			if withLines && !file.Binary {
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

type fingerprintCacheEntry struct {
	size   int64
	mod    time.Time
	binary bool
	hash   string
}

var fingerprintCache = struct {
	sync.Mutex
	entries map[string]fingerprintCacheEntry
}{entries: map[string]fingerprintCacheEntry{}}

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
		fingerprintCache.Lock()
		cached, cachedOK := fingerprintCache.entries[path]
		fingerprintCache.Unlock()
		if cachedOK && cached.size == info.Size() && cached.mod.Equal(info.ModTime()) {
			result[relative] = fileFingerprint{size: cached.size, hash: cached.hash, binary: cached.binary}
			return nil
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
		fingerprint := fileFingerprint{size: info.Size(), hash: hex.EncodeToString(hash.Sum(nil)), binary: isBinary(first[:firstN])}
		fingerprintCache.Lock()
		if len(fingerprintCache.entries) > 8000 {
			fingerprintCache.entries = map[string]fingerprintCacheEntry{}
		}
		fingerprintCache.entries[path] = fingerprintCacheEntry{size: info.Size(), mod: info.ModTime(), binary: fingerprint.binary, hash: fingerprint.hash}
		fingerprintCache.Unlock()
		result[relative] = fingerprint
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

type cachedUpstream struct {
	dir string
	at  time.Time
}

var upstreamCache = struct {
	sync.Mutex
	entries  map[string]cachedUpstream
	inflight map[string]chan struct{}
}{entries: map[string]cachedUpstream{}, inflight: map[string]chan struct{}{}}

func cacheableFetcher(fetcher platform.Fetcher) bool {
	switch fetcher.(type) {
	case platform.HTTPFetcher, *platform.CachedFetcher:
		return true
	default:
		return false
	}
}

func finishUpstream(key string) {
	upstreamCache.Lock()
	if wait, ok := upstreamCache.inflight[key]; ok {
		delete(upstreamCache.inflight, key)
		close(wait)
	}
	upstreamCache.Unlock()
}

func upstreamTree(ctx context.Context, plan model.Plan, fetcher platform.Fetcher) (string, error) {
	key := plan.SourceURL + "\x00" + plan.SourcePath
	if cacheableFetcher(fetcher) {
		upstreamCache.Lock()
		entry, ok := upstreamCache.entries[key]
		if ok && time.Since(entry.at) < 2*time.Minute {
			if _, err := os.Stat(entry.dir); err == nil {
				upstreamCache.Unlock()
				return entry.dir, nil
			}
		}
		if wait, waiting := upstreamCache.inflight[key]; waiting {
			upstreamCache.Unlock()
			<-wait
			return upstreamTree(ctx, plan, fetcher)
		}
		upstreamCache.inflight[key] = make(chan struct{})
		delete(upstreamCache.entries, key)
		upstreamCache.Unlock()
	}
	tarball, err := fetcher.Fetch(ctx, plan.SourceURL)
	if err != nil {
		finishUpstream(key)
		return "", err
	}
	dir, err := os.MkdirTemp("", "larder-upstream-")
	if err != nil {
		finishUpstream(key)
		return "", err
	}
	if err := extractSkillFolder(tarball, plan.SourcePath, dir); err != nil {
		_ = os.RemoveAll(dir)
		finishUpstream(key)
		return "", err
	}
	if !cacheableFetcher(fetcher) {
		return dir, nil
	}
	upstreamCache.Lock()
	if old, ok := upstreamCache.entries[key]; ok && old.dir != dir {
		_ = os.RemoveAll(old.dir)
	}
	upstreamCache.entries[key] = cachedUpstream{dir: dir, at: time.Now()}
	// Keep the cache small; previews are interactive, not a general cache.
	if len(upstreamCache.entries) > 6 {
		var oldestKey string
		var oldest time.Time
		for currentKey, entry := range upstreamCache.entries {
			if oldestKey == "" || entry.at.Before(oldest) {
				oldestKey, oldest = currentKey, entry.at
			}
		}
		if oldestKey != "" && oldestKey != key {
			_ = os.RemoveAll(upstreamCache.entries[oldestKey].dir)
			delete(upstreamCache.entries, oldestKey)
		}
	}
	upstreamCache.Unlock()
	finishUpstream(key)
	return dir, nil
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

// BuildSkillUpdateFileDiff computes the line diff for one file only. The
// update preview can therefore stay small and only load code when a user
// expands a file.
func BuildSkillUpdateFileDiff(ctx context.Context, skill model.Skill, repo, path string, fetcher platform.Fetcher, baseline map[string]string) (model.SkillUpdateFile, error) {
	plan, err := PlanSkillUpdate(skill, repo)
	if err != nil {
		return model.SkillUpdateFile{}, err
	}
	clean := filepath.Clean(filepath.FromSlash(path))
	if clean == "." || filepath.IsAbs(clean) || strings.HasPrefix(clean, ".."+string(filepath.Separator)) || clean == ".." {
		return model.SkillUpdateFile{}, os.ErrInvalid
	}
	if fetcher == nil {
		fetcher = platform.HTTPFetcher{}
	}
	incomingDir, err := upstreamTree(ctx, plan, fetcher)
	if err != nil {
		return model.SkillUpdateFile{}, err
	}
	if !cacheableFetcher(fetcher) {
		defer os.RemoveAll(incomingDir)
	}
	localPath := filepath.Join(skill.Path, clean)
	incomingPath := filepath.Join(incomingDir, clean)
	local, localErr := os.ReadFile(localPath)
	incoming, incomingErr := os.ReadFile(incomingPath)
	if localErr != nil && incomingErr != nil {
		return model.SkillUpdateFile{}, os.ErrNotExist
	}
	file := model.SkillUpdateFile{Path: filepath.ToSlash(clean)}
	oldContent, newContent := local, incoming
	if localErr != nil {
		oldContent = nil
		file.Status = "added"
	}
	if incomingErr != nil {
		newContent = nil
		file.Status = "removed"
	}
	if file.Status == "" {
		file.Status = "modified"
	}
	file.Binary = isBinary(oldContent) || isBinary(newContent)
	if !file.Binary {
		file.Lines, file.Truncated = lineDiffBytes(oldContent, newContent)
		file.Additions, file.Deletions = countDiffLines(file.Lines)
	}
	return file, nil
}
