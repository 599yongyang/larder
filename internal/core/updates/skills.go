package updates

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"larder/internal/core/model"
	"larder/internal/platform"
)

type skillCandidate struct {
	index int
	skill model.Skill
	dir   string
	state string
	msg   string
}

type fileHash struct {
	path string
	hash string
}

// checkSkills compares the whole installed Skill folder with the source
// repository at HEAD. The repository tarball is downloaded once per repo; no
// skill is executed and nothing is written.
func checkSkills(ctx context.Context, opts Options, report *model.UpdateReport) {
	if len(opts.Skills) == 0 {
		return
	}
	fetcher := opts.Fetcher
	if fetcher == nil {
		fetcher = platform.HTTPFetcher{}
	}
	seen := map[string]bool{}
	var candidates []*skillCandidate
	type repoGroup struct {
		repo       string
		candidates []*skillCandidate
	}
	groups := map[string]*repoGroup{}
	var groupOrder []string
	for _, skill := range opts.Skills {
		if !skill.Locked || skill.SourceType != "github" {
			continue
		}
		repo := githubRepo(skill)
		dir := skillDir(skill.SkillPath)
		key := repo + "\x00" + dir
		if repo == "" || seen[key] {
			continue
		}
		seen[key] = true
		candidate := &skillCandidate{
			index: len(report.Items),
			skill: skill,
			dir:   dir,
			state: "unknown",
			msg:   "尚未检查",
		}
		candidates = append(candidates, candidate)
		report.Items = append(report.Items, model.UpdateItem{
			Kind: "skill", Owner: "github", Package: repo, Name: skill.Name,
			Source: skill.Source, State: "unknown", Message: "尚未检查",
		})
		if groups[repo] == nil {
			groups[repo] = &repoGroup{repo: repo}
			groupOrder = append(groupOrder, repo)
		}
		groups[repo].candidates = append(groups[repo].candidates, candidate)
	}
	if len(candidates) == 0 {
		return
	}
	report.SkillScanned = len(candidates)

	sem := make(chan struct{}, 3)
	var wg sync.WaitGroup
	for _, repo := range groupOrder {
		group := groups[repo]
		wg.Add(1)
		go func(group *repoGroup) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			for _, candidate := range group.candidates {
				candidate.state = "error"
				candidate.msg = "来源仓库下载失败"
			}
			if ctx.Err() != nil {
				return
			}
			data, err := fetcher.Fetch(ctx, "https://codeload.github.com/"+group.repo+"/tar.gz/HEAD")
			if err != nil {
				for _, candidate := range group.candidates {
					candidate.msg = "来源仓库下载失败: " + err.Error()
				}
				return
			}
			dirs := make([]string, 0, len(group.candidates))
			for _, candidate := range group.candidates {
				dirs = append(dirs, candidate.dir)
			}
			hashes, missing, err := folderHashesFromTar(data, dirs)
			if err != nil {
				for _, candidate := range group.candidates {
					candidate.msg = "来源仓库解析失败: " + err.Error()
				}
				return
			}
			for _, candidate := range group.candidates {
				remote, ok := hashes[candidate.dir]
				if !ok {
					if err := missing[candidate.dir]; err != "" {
						candidate.msg = err
					} else {
						candidate.msg = "来源中找不到 Skill 目录"
					}
					continue
				}
				if candidate.skill.FolderHash == "" {
					candidate.msg = "本地没有文件夹哈希"
					continue
				}
				if candidate.skill.FolderHash == remote {
					candidate.state = "current"
					candidate.msg = "文件夹与来源 HEAD 一致"
				} else {
					candidate.state = "changed"
					candidate.msg = "文件夹与来源 HEAD 不一致"
				}
			}
		}(group)
	}
	wg.Wait()
	for _, candidate := range candidates {
		item := &report.Items[candidate.index]
		item.State = candidate.state
		item.Message = candidate.msg
		switch candidate.state {
		case "changed":
			report.SkillChanged++
		case "current":
			report.SkillCurrent++
		default:
			report.SkillErrors++
		}
	}
}

func skillDir(skillPath string) string {
	value := strings.TrimSpace(filepath.ToSlash(skillPath))
	if value == "" || value == "." {
		return ""
	}
	dir := path.Dir(value)
	if dir == "." || dir == "/" {
		return ""
	}
	return strings.TrimPrefix(dir, "/")
}

func folderHashesFromTar(data []byte, dirs []string) (map[string]string, map[string]string, error) {
	gz, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, nil, err
	}
	defer gz.Close()
	tr := tar.NewReader(gz)
	files := make(map[string][]fileHash, len(dirs))
	missing := map[string]string{}
	root := ""
	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, nil, err
		}
		if header.Typeflag != tar.TypeReg && header.Typeflag != tar.TypeRegA {
			continue
		}
		name := path.Clean(strings.TrimPrefix(header.Name, "./"))
		if root == "" {
			parts := strings.SplitN(name, "/", 2)
			root = parts[0]
		}
		if !strings.HasPrefix(name, root+"/") {
			continue
		}
		rel := strings.TrimPrefix(name, root+"/")
		matches := make([]struct{ dir, inside string }, 0, 1)
		for _, dir := range dirs {
			if dir != "" && !strings.HasPrefix(rel, dir+"/") && rel != dir {
				continue
			}
			inside := rel
			if dir != "" {
				inside = strings.TrimPrefix(rel, dir+"/")
			}
			if inside == "" || skipRemoteFile(inside) {
				continue
			}
			matches = append(matches, struct{ dir, inside string }{dir: dir, inside: inside})
		}
		if len(matches) == 0 {
			continue
		}
		content, err := io.ReadAll(io.LimitReader(tr, 8<<20))
		if err != nil {
			return nil, nil, err
		}
		sum := sha256.Sum256(content)
		for _, match := range matches {
			files[match.dir] = append(files[match.dir], fileHash{path: match.inside, hash: hex.EncodeToString(sum[:])})
		}
	}
	hashes := map[string]string{}
	for _, dir := range dirs {
		entries := files[dir]
		if len(entries) == 0 {
			if _, ok := missing[dir]; !ok {
				missing[dir] = "来源中找不到 Skill 目录"
			}
			continue
		}
		sort.Slice(entries, func(i, j int) bool { return entries[i].path < entries[j].path })
		h := sha256.New()
		for _, entry := range entries {
			fmt.Fprintf(h, "%s\x00%s\n", entry.path, entry.hash)
		}
		hashes[dir] = hex.EncodeToString(h.Sum(nil))
	}
	return hashes, missing, nil
}

func skipRemoteFile(rel string) bool {
	for _, part := range strings.Split(filepath.ToSlash(rel), "/") {
		if part == ".git" || part == "node_modules" || strings.HasPrefix(part, ".") {
			return true
		}
	}
	return false
}

func githubRepo(skill model.Skill) string {
	value := strings.TrimSpace(skill.SourceURL)
	value = strings.TrimPrefix(value, "https://github.com/")
	value = strings.TrimPrefix(value, "http://github.com/")
	value = strings.TrimSuffix(value, ".git")
	if value == "" {
		value = strings.TrimSpace(skill.Source)
	}
	if !strings.Contains(value, "/") {
		return ""
	}
	return value
}
