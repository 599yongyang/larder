package skills

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"larder/internal/core/model"
	"larder/internal/platform"
)

type Scanner struct {
	FS platform.FileSystem
}

func NewScanner(fs platform.FileSystem) *Scanner {
	if fs == nil {
		fs = platform.OSFileSystem{}
	}
	return &Scanner{FS: fs}
}

type Options struct {
	Roots        []platform.RootSpec
	LockfilePath string
}

type lockFile struct {
	Version int                  `json:"version"`
	Skills  map[string]lockSkill `json:"skills"`
}

type lockSkill struct {
	Source      string `json:"source"`
	SourceType  string `json:"sourceType"`
	SourceURL   string `json:"sourceUrl"`
	SkillPath   string `json:"skillPath"`
	PluginName  string `json:"pluginName,omitempty"`
	InstalledAt string `json:"installedAt,omitempty"`
	UpdatedAt   string `json:"updatedAt,omitempty"`
}

func (s *Scanner) Scan(ctx context.Context, opts Options) (model.ScanResult, error) {
	started := time.Now()
	result := model.ScanResult{SchemaVersion: model.SchemaVersion, StartedAt: started, Status: model.ScanComplete}
	if len(opts.Roots) == 0 {
		result.Status = model.ScanFailed
		result.FinishedAt = time.Now()
		return result, errors.New("no scan roots configured")
	}

	locks, lockIssues := s.readLockfile(opts.LockfilePath)
	result.LockSkills = lockRecords(locks)
	result.Issues = append(result.Issues, lockIssues...)

	result.Roots = make([]model.Root, 0, len(opts.Roots))
	for _, root := range opts.Roots {
		if err := ctx.Err(); err != nil {
			result.Status = model.ScanCancelled
			result.Issues = append(result.Issues, issue("cancelled", root.Path, err.Error(), true))
			break
		}
		converted := model.Root{
			ID: root.ID, Label: root.Label, LogicalAgent: root.LogicalAgent, SharedSkills: root.SharedSkills,
			Path: root.Path, Kind: model.RootKind(root.Kind), Known: root.Known,
			Status: model.RootAvailable,
		}
		entries, err := s.FS.ReadDir(root.Path)
		if err != nil {
			converted.Status = model.RootUnavailable
			converted.Error = err.Error()
			if platform.IsPermissionError(err) {
				converted.Status = model.RootPermissionDenied
				result.Issues = append(result.Issues, issue("permission", root.Path, err.Error(), true))
			} else if errors.Is(err, fs.ErrNotExist) {
				// An optional agent that is not installed is a normal fact,
				// not a partial scan (ADR-0029). The UI can show it as
				// "not installed" instead of treating it as a failure.
				converted.Status = model.RootNotInstalled
				converted.Error = "目录不存在（未安装）"
			} else {
				result.Issues = append(result.Issues, issue("scan_failure", root.Path, err.Error(), true))
			}
			result.Roots = append(result.Roots, converted)
			continue
		}
		converted.EntryCount = len(entries)
		result.Roots = append(result.Roots, converted)
		if root.Kind == platform.RootGlobal || root.Kind == platform.RootProject {
			for _, entry := range entries {
				if err := ctx.Err(); err != nil {
					result.Status = model.ScanCancelled
					result.Issues = append(result.Issues, issue("cancelled", root.Path, err.Error(), true))
					break
				}
				if isHidden(entry.Name()) {
					continue
				}
				skill, scanIssue := s.scanSkill(root, entry.Name(), locks[entry.Name()])
				if skill != nil {
					result.Skills = append(result.Skills, *skill)
				}
				if scanIssue != nil {
					result.Issues = append(result.Issues, *scanIssue)
				}
			}
		}
	}

	// Build an index before visiting agent entries so links can point to the
	// canonical central/project asset when one exists.
	skillByName := map[string]model.Skill{}
	for _, skill := range result.Skills {
		if skill.RootKind == model.RootGlobal {
			skillByName[skill.Name] = skill
		}
	}
	for _, root := range opts.Roots {
		if root.Kind != platform.RootKnownAgent && root.Kind != platform.RootUnknownAgent {
			continue
		}
		convertedRoot := findRoot(result.Roots, root.ID)
		if convertedRoot == nil {
			continue
		}
		if convertedRoot.Status != model.RootAvailable {
			// A not-installed optional agent is not a matrix column at all.
			// Permission/read failures stay as "unavailable" cells so the UI
			// can explain that the boundary could not be read.
			if root.SharedSkills {
				for _, skill := range result.Skills {
					if skill.RootKind == model.RootGlobal {
						result.AgentLinks = append(result.AgentLinks, model.AgentLink{SkillID: skill.ID, RootID: root.ID, Mode: model.LinkShared, Path: skill.Path, Visible: true, Reason: "Agent 支持共享 .agents/skills"})
					}
				}
			} else if convertedRoot.Status != model.RootNotInstalled {
				for _, skill := range result.Skills {
					if skill.RootKind == model.RootGlobal {
						result.AgentLinks = append(result.AgentLinks, model.AgentLink{SkillID: skill.ID, RootID: root.ID, Mode: model.LinkUnavailable, Visible: false, Reason: convertedRoot.Error})
					}
				}
			}
			continue
		}
		entries, err := s.FS.ReadDir(root.Path)
		if err != nil {
			result.Issues = append(result.Issues, issue("scan_failure", root.Path, err.Error(), true))
			continue
		}
		seen := map[string]bool{}
		for _, entry := range entries {
			if err := ctx.Err(); err != nil {
				result.Status = model.ScanCancelled
				result.Issues = append(result.Issues, issue("cancelled", root.Path, err.Error(), true))
				break
			}
			if isHidden(entry.Name()) {
				continue
			}
			path := filepath.Join(root.Path, entry.Name())
			link := s.inspectAgentEntry(path, entry.Name(), root.ID, skillByName)
			if link.SkillID == "" {
				// Agent-local assets are independent assets. Keep them in the
				// ledger while still exposing their agent directory boundary.
				skill, scanIssue := s.scanSkill(root, entry.Name(), lockSkill{})
				if skill != nil {
					skill.IsAgentLocal = true
					if skill.ScanState != "broken" {
						skill.ScanState = "agent_local"
					}
					result.Skills = append(result.Skills, *skill)
					link.SkillID = skill.ID
				}
				if scanIssue != nil {
					result.Issues = append(result.Issues, *scanIssue)
				}
			}
			if link.SkillID != "" {
				result.AgentLinks = append(result.AgentLinks, link)
				seen[link.SkillID] = true
			}
		}
		extraCount := s.scanExtraAgentSkills(ctx, root, &result, seen)
		if extraCount > 0 {
			if convertedRoot := findRoot(result.Roots, root.ID); convertedRoot != nil {
				convertedRoot.EntryCount += extraCount
			}
		}
		for _, skill := range result.Skills {
			if skill.RootKind != model.RootGlobal || seen[skill.ID] {
				continue
			}
			if root.SharedSkills {
				result.AgentLinks = append(result.AgentLinks, model.AgentLink{SkillID: skill.ID, RootID: root.ID, Mode: model.LinkShared, Path: skill.Path, Visible: true, Reason: "Agent 支持共享 .agents/skills"})
				continue
			}
			result.AgentLinks = append(result.AgentLinks, model.AgentLink{
				SkillID: skill.ID, RootID: root.ID, Mode: model.LinkMissing, Visible: false,
				Reason: "agent 目录中未发现同名条目",
			})
		}
	}

	// Only incomplete reads (permission/scan/lockfile failures) make the scan
	// partial. Broken links, missing SKILL.md and invalid frontmatter are data
	// findings, not incomplete scans (ADR-0022, ADR-0029). Never translate an
	// unavailable/not-installed root into missing cells.
	dedupeBrokenAgentLocalSkills(&result)

	// Only incomplete reads make the scan partial. Broken links, missing
	// SKILL.md and invalid frontmatter are data findings, not incomplete
	// scans (ADR-0022, ADR-0029). Never translate an unavailable/not-installed
	// root into missing cells.
	if hasIncompleteIssue(result.Issues) && result.Status == model.ScanComplete {
		result.Status = model.ScanPartial
	}
	if result.Status == model.ScanCancelled {
		result.FinishedAt = time.Now()
		return result, context.Canceled
	}
	result.FinishedAt = time.Now()
	return result, nil
}

func (s *Scanner) readLockfile(path string) (map[string]lockSkill, []model.ScanIssue) {
	if strings.TrimSpace(path) == "" {
		return map[string]lockSkill{}, nil
	}
	b, err := s.FS.ReadFile(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return map[string]lockSkill{}, nil
		}
		kind := "lockfile_error"
		if platform.IsPermissionError(err) {
			kind = "permission"
		}
		return map[string]lockSkill{}, []model.ScanIssue{issue(kind, path, err.Error(), true)}
	}
	var lf lockFile
	if err := json.Unmarshal(b, &lf); err != nil {
		return map[string]lockSkill{}, []model.ScanIssue{issue("lockfile_error", path, err.Error(), true)}
	}
	if lf.Skills == nil {
		lf.Skills = map[string]lockSkill{}
	}
	return lf.Skills, nil
}

func lockRecords(locks map[string]lockSkill) []model.LockSkill {
	keys := make([]string, 0, len(locks))
	for key := range locks {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	result := make([]model.LockSkill, 0, len(keys))
	for _, key := range keys {
		value := locks[key]
		result = append(result, model.LockSkill{Name: key, Source: value.Source, SourceType: value.SourceType, SourceURL: value.SourceURL, SkillPath: value.SkillPath, PluginName: value.PluginName, InstalledAt: value.InstalledAt, UpdatedAt: value.UpdatedAt})
	}
	return result
}

func (s *Scanner) scanSkill(root platform.RootSpec, name string, locked lockSkill) (*model.Skill, *model.ScanIssue) {
	path := filepath.Join(root.Path, name)
	info, err := s.FS.Lstat(path)
	if err != nil {
		return nil, ptrIssue(issue("scan_failure", path, err.Error(), true))
	}
	resolvedPath := path
	linkTarget, linkErr := s.FS.Readlink(path)
	reparse, _ := s.FS.IsDirectoryLink(path)
	isLink := info.Mode()&fs.ModeSymlink != 0 || reparse || linkErr == nil
	if isLink {
		target, readErr := linkTarget, linkErr
		if readErr == nil && !filepath.IsAbs(target) {
			target = filepath.Join(filepath.Dir(path), target)
		}
		if readErr != nil {
			return &model.Skill{ID: skillID(root.ID, path), Name: name, Path: path, RootID: root.ID, RootKind: model.RootKind(root.Kind), ScanState: "broken", ScannedAt: time.Now()}, ptrIssue(issue("scan_failure", path, readErr.Error(), true))
		}
		resolvedPath = filepath.Clean(target)
		if targetInfo, statErr := s.FS.Stat(resolvedPath); statErr != nil || !targetInfo.IsDir() {
			return &model.Skill{ID: skillID(root.ID, path), Name: name, Path: path, RootID: root.ID, RootKind: model.RootKind(root.Kind), ScanState: "broken", ScannedAt: time.Now()}, ptrIssue(issue("broken_link", path, "链接目标不存在", true))
		}
	}
	if !info.IsDir() && !isLink {
		// Agent config directories often contain metadata files next to Skill
		// folders. They are not failed Skills; ignore them instead of making
		// the whole scan partial.
		return nil, nil
	}
	content, desc, hasMD, mdIssue := s.readSkillMarkdown(filepath.Join(resolvedPath, "SKILL.md"))
	bins := parseRequiredBins(content)
	contentHash, folderHash, hashIssue := s.hashDirectory(resolvedPath)
	icon, iconSource := s.findSkillIcon(path, content)
	skill := &model.Skill{
		ID: skillID(root.ID, path), Name: name, Path: path, RootID: root.ID, RootKind: model.RootKind(root.Kind),
		Source: locked.Source, SourceType: locked.SourceType, SourceURL: locked.SourceURL, SkillPath: locked.SkillPath,
		Locked: locked.Source != "" || locked.SourceURL != "" || locked.SkillPath != "", HasSkillMD: hasMD,
		Description: desc, DescChars: len([]rune(desc)), DescTokens: estimateTokens(desc), FolderHash: folderHash, ContentHash: contentHash,
		ScanState: "ok", ScannedAt: time.Now(), IsProject: root.Kind == platform.RootProject,
		Icon: icon, IconSource: iconSource, Bins: bins,
	}
	if mdIssue != nil {
		return skill, mdIssue
	}
	if hashIssue != nil {
		return skill, hashIssue
	}
	_ = content
	return skill, nil
}

func (s *Scanner) findSkillIcon(dir, content string) (string, string) {
	candidates := []string{
		"icon.svg", "icon.png", "icon.webp", "icon.jpg", "icon.jpeg",
		"logo.svg", "logo.png", "logo.webp", "logo.jpg", "logo.jpeg",
		"assets/icon.svg", "assets/icon.png", "assets/icon.webp",
		"assets/logo.svg", "assets/logo.png", "assets/logo.webp",
	}
	for _, candidate := range candidates {
		path := filepath.Join(dir, candidate)
		if info, err := s.FS.Stat(path); err == nil && !info.IsDir() {
			return path, "local"
		}
	}
	value := frontmatterIcon(content)
	if value == "" {
		return "", ""
	}
	lower := strings.ToLower(value)
	if strings.HasPrefix(lower, "http://") || strings.HasPrefix(lower, "https://") || strings.HasPrefix(lower, "data:") {
		return value, "frontmatter"
	}
	if !filepath.IsAbs(value) {
		path := filepath.Join(dir, filepath.FromSlash(value))
		if info, err := s.FS.Stat(path); err == nil && !info.IsDir() {
			return path, "frontmatter"
		}
	}
	return value, "frontmatter"
}

func frontmatterIcon(content string) string {
	if !strings.HasPrefix(content, "---") {
		return ""
	}
	rest := content[3:]
	end := strings.Index(rest, "\n---")
	if end < 0 {
		return ""
	}
	for _, line := range strings.Split(rest[:end], "\n") {
		trimmed := strings.TrimSpace(line)
		lower := strings.ToLower(trimmed)
		for _, key := range []string{"icon:", "iconurl:", "logo:"} {
			if strings.HasPrefix(lower, key) {
				return strings.Trim(strings.TrimSpace(trimmed[len(key):]), "\"'")
			}
		}
	}
	return ""
}

func (s *Scanner) scanExtraAgentSkills(ctx context.Context, root platform.RootSpec, result *model.ScanResult, seen map[string]bool) int {
	if len(root.ExtraPaths) == 0 {
		return 0
	}
	knownNames := map[string]bool{}
	for _, skill := range result.Skills {
		knownNames[skill.Name] = true
	}
	added := 0
	for _, extra := range root.ExtraPaths {
		dirs, err := s.findSkillDirs(extra)
		if err != nil {
			continue
		}
		for _, dir := range dirs {
			if ctx.Err() != nil {
				return added
			}
			name := filepath.Base(dir)
			if knownNames[name] {
				continue
			}
			parent := filepath.Dir(dir)
			skill, scanIssue := s.scanSkill(platform.RootSpec{ID: root.ID, Label: root.Label, LogicalAgent: root.LogicalAgent, Path: parent, Kind: root.Kind, Known: root.Known}, name, lockSkill{})
			if skill != nil {
				skill.IsAgentLocal = true
				skill.ScanState = "agent_local"
				normalized := filepath.ToSlash(dir)
				if strings.Contains(normalized, "/skills/.system/") || strings.Contains(normalized, "/.system/") {
					skill.Origin = "system"
					skill.OriginDetail = "系统内置"
				} else if strings.Contains(normalized, "/plugins/cache/") {
					skill.Origin = "plugin"
					skill.OriginDetail = pluginName(normalized)
				}
				result.Skills = append(result.Skills, *skill)
				result.AgentLinks = append(result.AgentLinks, model.AgentLink{SkillID: skill.ID, RootID: root.ID, Mode: model.LinkLocal, Path: skill.Path, Visible: true, Reason: "Agent 插件/系统 Skill"})
				knownNames[name] = true
				seen[skill.ID] = true
				added++
			}
			if scanIssue != nil {
				result.Issues = append(result.Issues, *scanIssue)
			}
		}
	}
	return added
}

func pluginName(path string) string {
	parts := strings.Split(path, "/")
	for i, part := range parts {
		if part == "cache" && i >= 2 && parts[i-1] == "plugins" && i+2 < len(parts) {
			return parts[i+2]
		}
	}
	return "插件"
}

func (s *Scanner) findSkillDirs(root string) ([]string, error) {
	sorted := []string{}
	var walk func(string, int) error
	walk = func(dir string, depth int) error {
		if depth > 8 {
			return nil
		}
		entries, err := s.FS.ReadDir(dir)
		if err != nil {
			return err
		}
		for _, entry := range entries {
			if !entry.IsDir() || isHidden(entry.Name()) || entry.Name() == "node_modules" || entry.Name() == ".git" {
				continue
			}
			child := filepath.Join(dir, entry.Name())
			if _, err := s.FS.ReadFile(filepath.Join(child, "SKILL.md")); err == nil {
				sorted = append(sorted, child)
				continue
			}
			if err := walk(child, depth+1); err != nil {
				continue
			}
		}
		return nil
	}
	if err := walk(root, 0); err != nil {
		return nil, err
	}
	return sorted, nil
}

func (s *Scanner) inspectAgentEntry(path, name, rootID string, canonical map[string]model.Skill) model.AgentLink {
	info, err := s.FS.Lstat(path)
	if err != nil {
		return model.AgentLink{RootID: rootID, Mode: model.LinkBroken, Reason: err.Error()}
	}
	canonicalSkill, hasCanonical := canonical[name]
	linkTarget, linkErr := s.FS.Readlink(path)
	reparse, _ := s.FS.IsDirectoryLink(path)
	isLink := info.Mode()&fs.ModeSymlink != 0 || reparse || linkErr == nil
	if isLink {
		target, readErr := linkTarget, linkErr
		if readErr != nil {
			if hasCanonical {
				return model.AgentLink{SkillID: canonicalSkill.ID, RootID: rootID, Mode: model.LinkBroken, Path: path, Visible: false, Reason: readErr.Error()}
			}
			return model.AgentLink{RootID: rootID, Mode: model.LinkBroken, Path: path, Visible: false, Reason: readErr.Error()}
		}
		if !filepath.IsAbs(target) {
			target = filepath.Join(filepath.Dir(path), target)
		}
		if targetInfo, statErr := s.FS.Stat(filepath.Clean(target)); statErr != nil || !targetInfo.IsDir() {
			if hasCanonical {
				return model.AgentLink{SkillID: canonicalSkill.ID, RootID: rootID, Mode: model.LinkBroken, Path: path, TargetPath: filepath.Clean(target), Visible: false, Reason: "链接目标不存在"}
			}
			return model.AgentLink{RootID: rootID, Mode: model.LinkBroken, Path: path, TargetPath: filepath.Clean(target), Visible: false, Reason: "链接目标不存在"}
		}
		return model.AgentLink{SkillID: canonicalSkill.ID, RootID: rootID, Mode: model.LinkLinked, Path: path, TargetPath: filepath.Clean(target), Visible: true}
	}
	if !info.IsDir() {
		return model.AgentLink{RootID: rootID, Mode: model.LinkLocal, Path: path, Visible: true, Reason: "条目不是目录"}
	}
	if hasCanonical {
		contentHash, _, _ := s.hashDirectory(path)
		drift := canonicalSkill.ContentHash != "" && contentHash != "" && canonicalSkill.ContentHash != contentHash
		return model.AgentLink{SkillID: canonicalSkill.ID, RootID: rootID, Mode: model.LinkCopied, Path: path, ContentHash: contentHash, Drift: drift, Visible: true, Reason: copyReason(drift)}
	}
	return model.AgentLink{RootID: rootID, Mode: model.LinkLocal, Path: path, Visible: true, Reason: "仅存在于该 Agent 目录"}
}

func (s *Scanner) readSkillMarkdown(path string) (string, string, bool, *model.ScanIssue) {
	b, err := s.FS.ReadFile(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return "", "", false, ptrIssue(issue("missing_skill_md", path, "缺少 SKILL.md", true))
		}
		kind := "scan_failure"
		if platform.IsPermissionError(err) {
			kind = "permission"
		}
		return "", "", false, ptrIssue(issue(kind, path, err.Error(), true))
	}
	name, description, ok := parseFrontmatter(string(b))
	if !ok {
		return "", "", true, ptrIssue(issue("invalid_frontmatter", path, "SKILL.md 缺少可解析的 frontmatter", true))
	}
	_ = name
	return string(b), description, true, nil
}

func (s *Scanner) hashDirectory(root string) (contentHash, folderHash string, scanIssue *model.ScanIssue) {
	type fileHash struct{ path, hash string }
	var files []fileHash
	var walk func(string, string) error
	walk = func(current, relative string) error {
		entries, err := s.FS.ReadDir(current)
		if err != nil {
			return err
		}
		sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })
		for _, entry := range entries {
			if isHidden(entry.Name()) || entry.Name() == ".git" || entry.Name() == "node_modules" {
				continue
			}
			path := filepath.Join(current, entry.Name())
			rel := filepath.Join(relative, entry.Name())
			info, err := s.FS.Lstat(path)
			if err != nil {
				return err
			}
			if info.IsDir() {
				if err := walk(path, rel); err != nil {
					return err
				}
				continue
			}
			b, err := s.FS.ReadFile(path)
			if err != nil {
				return err
			}
			digest := sha256.Sum256(b)
			files = append(files, fileHash{path: filepath.ToSlash(rel), hash: hex.EncodeToString(digest[:])})
		}
		return nil
	}
	if err := walk(root, ""); err != nil {
		kind := "scan_failure"
		if platform.IsPermissionError(err) {
			kind = "permission"
		}
		return "", "", ptrIssue(issue(kind, root, err.Error(), true))
	}
	h := sha256.New()
	for _, file := range files {
		fmt.Fprintf(h, "%s\x00%s\n", file.path, file.hash)
	}
	folder := h.Sum(nil)
	content := sha256.New()
	for _, file := range files {
		content.Write([]byte(file.hash))
	}
	return hex.EncodeToString(content.Sum(nil)), hex.EncodeToString(folder), nil
}

func parseRequiredBins(content string) []string {
	if !strings.HasPrefix(content, "---") {
		return nil
	}
	rest := content[3:]
	end := strings.Index(rest, "\n---")
	if end < 0 {
		return nil
	}
	var bins []string
	for _, line := range strings.Split(rest[:end], "\n") {
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, "bins:") {
			continue
		}
		value := strings.TrimSpace(strings.TrimPrefix(trimmed, "bins:"))
		value = strings.Trim(value, "[]")
		for _, part := range strings.Split(value, ",") {
			part = strings.Trim(strings.TrimSpace(part), "\"'")
			if part != "" {
				bins = append(bins, part)
			}
		}
	}
	return bins
}

func parseFrontmatter(content string) (name, description string, ok bool) {
	if !strings.HasPrefix(content, "---") {
		return "", "", false
	}
	rest := content[3:]
	end := strings.Index(rest, "\n---")
	if end < 0 {
		return "", "", false
	}
	lines := strings.Split(rest[:end], "\n")
	for i := 0; i < len(lines); i++ {
		trimmed := strings.TrimSpace(lines[i])
		switch {
		case strings.HasPrefix(trimmed, "name:") && name == "":
			name = strings.Trim(strings.TrimSpace(strings.TrimPrefix(trimmed, "name:")), "\"'")
		case strings.HasPrefix(trimmed, "description:"):
			value := strings.TrimSpace(strings.TrimPrefix(trimmed, "description:"))
			if value == "" || value == ">" || value == "|" || value == ">-" || value == "|-" {
				var parts []string
				for j := i + 1; j < len(lines); j++ {
					if strings.TrimSpace(lines[j]) == "" || (!strings.HasPrefix(lines[j], " ") && !strings.HasPrefix(lines[j], "\t")) {
						break
					}
					parts = append(parts, strings.TrimSpace(lines[j]))
				}
				value = strings.Join(parts, " ")
			}
			description = strings.Trim(strings.TrimSpace(value), "\"'")
		}
	}
	return name, description, true
}

func estimateTokens(value string) int {
	if value == "" {
		return 0
	}
	return int(float64(len([]rune(value)))/3.5 + 0.5)
}

func skillID(rootID, path string) string {
	digest := sha256.Sum256([]byte(rootID + "\x00" + filepath.Clean(path)))
	return hex.EncodeToString(digest[:])[:24]
}

func issue(kind, path, message string, recoverable bool) model.ScanIssue {
	return model.ScanIssue{Kind: kind, Path: path, Message: message, Recoverable: recoverable, At: time.Now()}
}

func ptrIssue(value model.ScanIssue) *model.ScanIssue { return &value }

func copyReason(drift bool) string {
	if drift {
		return "副本内容与中央库不同"
	}
	return "副本内容与中央库一致"
}

// dedupeBrokenAgentLocalSkills collapses the same broken target reported by
// multiple agent directories into one synthetic agent-local Skill row, while
// keeping every location as an AgentLink. Without this, an orphan such as
// find-skills produces one row per agent directory and floods Doctor.
func dedupeBrokenAgentLocalSkills(result *model.ScanResult) {
	firstByID := map[string]string{}  // duplicate skill ID -> kept skill ID
	keptByName := map[string]string{} // name -> kept skill ID
	kept := result.Skills[:0]
	for _, skill := range result.Skills {
		if skill.IsAgentLocal && skill.ScanState == "broken" {
			if keptID, ok := keptByName[skill.Name]; ok {
				firstByID[skill.ID] = keptID
				continue
			}
			keptByName[skill.Name] = skill.ID
		}
		kept = append(kept, skill)
	}
	result.Skills = kept
	if len(firstByID) == 0 {
		return
	}
	for i := range result.AgentLinks {
		if keptID, ok := firstByID[result.AgentLinks[i].SkillID]; ok {
			result.AgentLinks[i].SkillID = keptID
		}
	}
}

func hasIncompleteIssue(issues []model.ScanIssue) bool {
	for _, issue := range issues {
		switch issue.Kind {
		case "permission", "scan_failure", "lockfile_error", "scan_unavailable":
			return true
		}
	}
	return false
}

func findRoot(roots []model.Root, id string) *model.Root {
	for i := range roots {
		if roots[i].ID == id {
			return &roots[i]
		}
	}
	return nil
}

func isHidden(name string) bool { return name == "" || strings.HasPrefix(name, ".") }
