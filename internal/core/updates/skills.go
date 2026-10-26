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
		additions  []model.UpdateItem
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
			if discovered, parseErr := discoverSkillsFromTar(data); parseErr == nil {
				installed := map[string]bool{}
				for _, candidate := range group.candidates {
					installed[candidate.dir] = true
				}
				for _, skill := range discovered {
					if skill.Dir == "" || skill.Name == "" || installed[skill.Dir] {
						continue
					}
					nested := false
					for dir := range installed {
						if strings.HasPrefix(skill.Dir, dir+"/") {
							nested = true
							break
						}
					}
					if nested {
						continue
					}
					group.additions = append(group.additions, model.UpdateItem{Kind: "skill", Owner: "github", Package: group.repo, Name: skill.Name, Description: skill.Description, Source: group.repo, SkillPath: skill.Dir, State: "new", Message: "上游新增，可安装"})
				}
			}
			for _, candidate := range group.candidates {
				remote, ok := hashes[candidate.dir]
				if !ok {
					candidate.state = "removed"
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
	for _, repo := range groupOrder {
		report.Items = append(report.Items, groups[repo].additions...)
		report.SkillAdded += len(groups[repo].additions)
	}
	for _, candidate := range candidates {
		item := &report.Items[candidate.index]
		item.State = candidate.state
		item.Message = candidate.msg
		switch candidate.state {
		case "changed":
			report.SkillChanged++
		case "current":
			report.SkillCurrent++
		case "removed":
			report.SkillRemoved++
		default:
			report.SkillErrors++
		}
	}
}

type discoveredSkill struct {
	Name        string
	Description string
	Dir         string
}

func discoverSkillsFromTar(data []byte) ([]discoveredSkill, error) {
	gz, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	defer gz.Close()
	tr := tar.NewReader(gz)
	root := ""
	seen := map[string]bool{}
	var result []discoveredSkill
	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		if header.Typeflag != tar.TypeReg && header.Typeflag != tar.TypeRegA {
			continue
		}
		name := path.Clean(strings.TrimPrefix(header.Name, "./"))
		if root == "" {
			parts := strings.SplitN(name, "/", 2)
			root = parts[0]
		}
		if !strings.HasPrefix(name, root+"/") || path.Base(name) != "SKILL.md" {
			continue
		}
		relative := strings.TrimPrefix(name, root+"/")
		dir := path.Dir(relative)
		if dir == "." || dir == "" || seen[dir] || !discoverableSkillDir(dir) {
			continue
		}
		content, err := io.ReadAll(io.LimitReader(tr, 1<<20))
		if err != nil {
			return nil, err
		}
		skillName, skillDescription, ok := parseUpdateFrontmatter(string(content))
		if !ok || strings.TrimSpace(skillDescription) == "" {
			continue
		}
		if strings.TrimSpace(skillName) == "" {
			skillName = path.Base(dir)
		}
		seen[dir] = true
		result = append(result, discoveredSkill{Name: skillName, Description: skillDescription, Dir: dir})
	}
	return result, nil
}

func discoverableSkillDir(dir string) bool {
	for _, part := range strings.Split(strings.ToLower(dir), "/") {
		switch part {
		case ".git", ".github", "node_modules", "vendor", "dist", "build", "test", "tests", "__tests__", "fixtures", "examples", "example", "templates", "template":
			return false
		}
	}
	return true
}

func parseUpdateFrontmatter(content string) (name, description string, ok bool) {
	trimmed := strings.TrimPrefix(content, "\ufeff")
	if !strings.HasPrefix(trimmed, "---\n") && !strings.HasPrefix(trimmed, "---\r\n") {
		return "", "", false
	}
	rest := trimmed[3:]
	end := strings.Index(rest, "\n---")
	if end < 0 {
		return "", "", false
	}
	lines := strings.Split(rest[:end], "\n")
	for index := 0; index < len(lines); index++ {
		line := strings.TrimSpace(strings.TrimSuffix(lines[index], "\r"))
		key, value, found := strings.Cut(line, ":")
		if !found {
			continue
		}
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		switch key {
		case "name":
			name = strings.Trim(value, `"'`)
		case "description":
			if value == "|" || value == ">" || value == "|-" || value == ">-" || value == "" {
				block := make([]string, 0, 4)
				for index+1 < len(lines) {
					next := strings.TrimSuffix(lines[index+1], "\r")
					if strings.TrimSpace(next) == "" {
						block = append(block, "")
						index++
						continue
					}
					if next[0] != ' ' && next[0] != '\t' {
						break
					}
					block = append(block, strings.TrimSpace(next))
					index++
				}
				separator := "\n"
				if strings.HasPrefix(value, ">") {
					separator = " "
				}
				description = strings.Join(block, separator)
			} else {
				description = strings.Trim(value, `"'`)
			}
		}
	}
	return name, description, true
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
