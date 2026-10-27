// Package clis builds a read-only inventory of commands on PATH.
package clis

import (
	"context"
	"errors"
	"io/fs"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"

	"larder/internal/core/model"
	"larder/internal/platform"
)

type Options struct {
	Dirs []string
	Home string
}

type Scanner struct {
	fs       platform.FileSystem
	resolver *metadataResolver
}

func NewScanner(fs platform.FileSystem) *Scanner {
	if fs == nil {
		fs = platform.OSFileSystem{}
	}
	return &Scanner{fs: fs}
}

func (s *Scanner) Scan(ctx context.Context, opts Options) ([]model.CLIInfo, []model.ScanIssue) {
	type entry struct {
		info model.CLIInfo
		seen map[string]bool
	}
	byName := map[string]*entry{}
	resolver := newMetadataResolver(s.fs, opts.Home)
	var issues []model.ScanIssue

	for _, dir := range opts.Dirs {
		if err := ctx.Err(); err != nil {
			return nil, nil
		}
		dir = filepath.Clean(dir)
		entries, err := s.fs.ReadDir(dir)
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				continue
			}
			kind := "scan_failure"
			if platform.IsPermissionError(err) {
				kind = "permission"
			}
			issues = append(issues, model.ScanIssue{Kind: kind, Path: dir, Message: err.Error(), Recoverable: true, At: time.Now()})
			continue
		}
		for _, dirEntry := range entries {
			if err := ctx.Err(); err != nil {
				return nil, nil
			}
			name := dirEntry.Name()
			if name == "" || strings.HasPrefix(name, ".") {
				continue
			}
			path := filepath.Join(dir, name)
			linfo, err := s.fs.Lstat(path)
			if err != nil || linfo.IsDir() {
				continue
			}
			if !isExecutable(name, linfo) {
				continue
			}
			broken := false
			target := path
			if linfo.Mode()&fs.ModeSymlink != 0 {
				link, readErr := s.fs.Readlink(path)
				if readErr != nil {
					broken = true
				} else {
					if !filepath.IsAbs(link) {
						link = filepath.Join(filepath.Dir(path), link)
					}
					target = filepath.Clean(link)
					if stat, statErr := s.fs.Stat(target); statErr != nil || stat.IsDir() {
						broken = true
					}
				}
			}
			owner, ownerDetail := classifyOwner(path, target, opts.Home)
			current, ok := byName[name]
			if !ok {
				current = &entry{
					info: model.CLIInfo{
						ID:          "cli:" + name,
						Name:        name,
						Path:        path,
						Owner:       owner,
						OwnerDetail: ownerDetail,
						Status:      "ok",
						ScannedAt:   time.Now(),
					},
					seen: map[string]bool{},
				}
				byName[name] = current
			}
			if current.seen[path] {
				continue
			}
			current.seen[path] = true
			current.info.Paths = append(current.info.Paths, model.CLIPath{Path: path, TargetPath: target, Owner: owner, OwnerDetail: ownerDetail, Broken: broken})
		}
	}

	result := make([]model.CLIInfo, 0, len(byName))
	for _, current := range byName {
		info := current.info
		info.DuplicateCount = len(info.Paths) - 1
		effectiveIndex := 0
		hasWorking := false
		for index := range info.Paths {
			if !info.Paths[index].Broken {
				effectiveIndex = index
				hasWorking = true
				break
			}
		}
		for index := range info.Paths {
			info.Paths[index].Effective = index == effectiveIndex
		}
		switch {
		case !hasWorking:
			info.Status = "broken"
		case info.DuplicateCount > 0:
			info.Status = "duplicate"
		default:
			info.Status = "ok"
		}
		if len(info.Paths) > 0 {
			effective := info.Paths[effectiveIndex]
			info.Path = effective.Path
			info.Owner = effective.Owner
			info.OwnerDetail = effective.OwnerDetail
			info.Version, info.OwnerDetail = resolver.version(info.Name, effective.Path, effective.TargetPath, effective.Owner)
		}
		result = append(result, info)
	}
	sort.Slice(result, func(i, j int) bool { return strings.ToLower(result[i].Name) < strings.ToLower(result[j].Name) })
	return result, issues
}

func isExecutable(name string, info fs.FileInfo) bool {
	if runtime.GOOS == "windows" {
		switch strings.ToLower(filepath.Ext(name)) {
		case ".exe", ".cmd", ".bat", ".ps1", ".com":
			return true
		default:
			return false
		}
	}
	return info.Mode().Perm()&0o111 != 0
}

func packageAfterNodeModules(value string) string {
	index := strings.Index(value, "/node_modules/")
	if index < 0 {
		return ""
	}
	parts := strings.Split(strings.Trim(value[index+len("/node_modules/"):], "/"), "/")
	if len(parts) == 0 {
		return ""
	}
	if strings.HasPrefix(parts[0], "@") && len(parts) > 1 {
		return parts[0] + "/" + parts[1]
	}
	return parts[0]
}

func segmentAfter(value, marker string) string {
	index := strings.Index(value, marker)
	if index < 0 {
		return ""
	}
	rest := value[index+len(marker):]
	if slash := strings.Index(rest, "/"); slash >= 0 {
		rest = rest[:slash]
	}
	return strings.TrimSpace(rest)
}

func classifyOwner(path, target, home string) (string, string) {
	value := filepath.ToSlash(strings.ToLower(target))
	pathValue := filepath.ToSlash(strings.ToLower(path))
	check := func(needle string) bool {
		return strings.Contains(value, needle) || strings.Contains(pathValue, needle)
	}
	switch {
	case check("/cellar/") || check("/homebrew/") || check("/opt/homebrew/") || check("/linuxbrew/"):
		return "Homebrew", ""
	case check("/node_modules/") || check("/.npm-global/") || check("/.nvm/") || check("/.volta/"):
		return "npm", ""
	case check("/winget/"):
		return "winget", ""
	case check("/scoop/"):
		return "Scoop", ""
	case check("/chocolatey/"):
		return "Chocolatey", ""
	case check("/.bun/bin/") || check("/bun/bin/"):
		return "Bun", packageAfterNodeModules(value)
	case check("/.cargo/bin/"):
		return "Cargo", filepath.Base(target)
	case check("/go/bin/"):
		return "Go", ""
	case check("/miniconda") || check("/anaconda"):
		return "Conda", filepath.Base(target)
	case check("/pipx/") || check("/.local/pipx/"):
		return "pipx", segmentAfter(value, "/pipx/venvs/")
	case check("/uv/tools/"):
		return "uv", segmentAfter(value, "/uv/tools/")
	case check(".app/contents/"):
		return "App 内置", ""
	case strings.HasPrefix(value, "/system/") || strings.HasPrefix(value, "/windows/system32/") ||
		strings.HasPrefix(pathValue, "/usr/bin/") || strings.HasPrefix(pathValue, "/bin/") ||
		strings.HasPrefix(pathValue, "/usr/sbin/") || strings.HasPrefix(pathValue, "/sbin/"):
		return "系统", ""
	default:
		if home != "" && strings.HasPrefix(path, filepath.Join(home, ".local", "bin")) {
			return "Standalone", "~/.local/bin"
		}
		return "Standalone", ""
	}
}
