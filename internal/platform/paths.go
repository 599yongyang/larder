package platform

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
)

// RootKind is kept in the platform package so path discovery does not make the
// business core depend on a specific desktop shell.
type RootKind string

const (
	RootGlobal       RootKind = "global"
	RootKnownAgent   RootKind = "known_agent"
	RootUnknownAgent RootKind = "unknown_agent"
	RootProject      RootKind = "project"
)

// RootSpec describes one directory that the read-only scanner may inspect.
type RootSpec struct {
	ID           string   `json:"id"`
	Label        string   `json:"label"`
	LogicalAgent string   `json:"logicalAgent,omitempty"`
	Path         string   `json:"path"`
	Kind         RootKind `json:"kind"`
	Known        bool     `json:"known"`
	SharedSkills bool     `json:"sharedSkills,omitempty"`
	ExtraPaths   []string `json:"extraPaths,omitempty"`
}

// LocalPaths owns platform-dependent path conventions. It never writes to any
// user asset; AppDataDir is only used for the application's private cache.
type LocalPaths struct{}

func (LocalPaths) HomeDir() (string, error) {
	if override := strings.TrimSpace(firstEnv("LARDER_HOME", "AAM_HOME")); override != "" {
		return filepath.Clean(override), nil
	}
	return os.UserHomeDir()
}

func (LocalPaths) LockfilePath(home string) string {
	return filepath.Join(home, ".agents", ".skill-lock.json")
}

func (LocalPaths) AppDataDir(appName string) (string, error) {
	if override := strings.TrimSpace(firstEnv("LARDER_DATA_DIR", "AAM_DATA_DIR")); override != "" {
		return filepath.Clean(override), nil
	}
	base, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, appName), nil
}

// DefaultRoots contains the central library and the agent directories with
// known discovery rules. Missing roots are retained so the UI can explain the
// boundary instead of silently turning an unreadable root into missing cells.
func (LocalPaths) DefaultRoots(home string) []RootSpec {
	roots := []RootSpec{
		{ID: "global", Label: "Global skills", Path: filepath.Join(home, ".agents", "skills"), Kind: RootGlobal, Known: true},
		{ID: "claude-code", Label: "Claude Code", LogicalAgent: "Claude Code", Path: filepath.Join(home, ".claude", "skills"), Kind: RootKnownAgent, Known: true},
		{ID: "codex", Label: "Codex", LogicalAgent: "Codex", Path: filepath.Join(home, ".codex", "skills"), Kind: RootKnownAgent, Known: true, SharedSkills: true, ExtraPaths: []string{filepath.Join(home, ".codex", "skills", ".system"), filepath.Join(home, ".codex", "plugins", "cache")}},
		{ID: "kiro", Label: "Kiro", LogicalAgent: "Kiro", Path: filepath.Join(home, ".kiro", "skills"), Kind: RootKnownAgent, Known: true},
		{ID: "windsurf", Label: "Windsurf", LogicalAgent: "Windsurf", Path: filepath.Join(home, ".codeium", "windsurf", "skills"), Kind: RootKnownAgent, Known: true},
		{ID: "cursor-legacy", Label: "Cursor (legacy)", LogicalAgent: "Cursor", Path: filepath.Join(home, ".cursor", "skills-cursor"), Kind: RootKnownAgent, Known: true},
		{ID: "cursor", Label: "Cursor", LogicalAgent: "Cursor", Path: filepath.Join(home, ".cursor", "skills"), Kind: RootKnownAgent, Known: true},
		{ID: "gemini-cli", Label: "Gemini CLI", LogicalAgent: "Gemini CLI", Path: filepath.Join(home, ".gemini", "skills"), Kind: RootKnownAgent, Known: true},
		{ID: "cline", Label: "Cline", LogicalAgent: "Cline", Path: filepath.Join(home, ".cline", "skills"), Kind: RootKnownAgent, Known: true},
		{ID: "dsh", Label: "DSH", LogicalAgent: "DSH", Path: filepath.Join(home, ".dsh", "skills"), Kind: RootKnownAgent, Known: true, SharedSkills: true},
		{ID: "opencode", Label: "OpenCode", LogicalAgent: "OpenCode", Path: filepath.Join(home, ".config", "opencode", "skills"), Kind: RootKnownAgent, Known: true},
		{ID: "codebuddy", Label: "CodeBuddy", LogicalAgent: "CodeBuddy", Path: filepath.Join(home, ".codebuddy", "skills"), Kind: RootKnownAgent, Known: true},
		{ID: "comate", Label: "Comate", LogicalAgent: "Comate", Path: filepath.Join(home, ".comate", "skills"), Kind: RootKnownAgent, Known: true},
		{ID: "openclaw", Label: "OpenClaw", LogicalAgent: "OpenClaw", Path: filepath.Join(home, ".openclaw", "skills"), Kind: RootKnownAgent, Known: true},
		{ID: "qoder", Label: "Qoder", LogicalAgent: "Qoder", Path: filepath.Join(home, ".qoder", "skills"), Kind: RootKnownAgent, Known: true},
		{ID: "qoderwork", Label: "QoderWork", LogicalAgent: "QoderWork", Path: filepath.Join(home, ".qoderwork", "skills"), Kind: RootKnownAgent, Known: true},
		{ID: "qwenwork", Label: "QwenWork", LogicalAgent: "QwenWork", Path: filepath.Join(home, ".qwenworkcn", "skills"), Kind: RootKnownAgent, Known: true},
		{ID: "trae", Label: "Trae", LogicalAgent: "Trae", Path: filepath.Join(home, ".trae", "skills"), Kind: RootKnownAgent, Known: true},
		{ID: "trae-cn", Label: "Trae CN", LogicalAgent: "Trae", Path: filepath.Join(home, ".trae-cn", "skills"), Kind: RootKnownAgent, Known: true},
		{ID: "workbuddy", Label: "WorkBuddy", LogicalAgent: "WorkBuddy", Path: filepath.Join(home, ".workbuddy", "skills"), Kind: RootKnownAgent, Known: true},
		{ID: "browseros", Label: "BrowserOS", LogicalAgent: "BrowserOS", Path: filepath.Join(home, ".browseros", "skills"), Kind: RootKnownAgent, Known: true},
		{ID: "commandcode", Label: "CommandCode", LogicalAgent: "CommandCode", Path: filepath.Join(home, ".commandcode", "skills"), Kind: RootKnownAgent, Known: true},
		{ID: "continue", Label: "Continue", LogicalAgent: "Continue", Path: filepath.Join(home, ".continue", "skills"), Kind: RootKnownAgent, Known: true},
		{ID: "copilot", Label: "GitHub Copilot", LogicalAgent: "GitHub Copilot", Path: filepath.Join(home, ".copilot", "skills"), Kind: RootKnownAgent, Known: true},
		{ID: "goose", Label: "Goose", LogicalAgent: "Goose", Path: filepath.Join(home, ".config", "goose", "skills"), Kind: RootKnownAgent, Known: true},
		{ID: "crush", Label: "Crush", LogicalAgent: "Crush", Path: filepath.Join(home, ".config", "crush", "skills"), Kind: RootKnownAgent, Known: true},
		{ID: "factory", Label: "Factory", LogicalAgent: "Factory", Path: filepath.Join(home, ".factory", "skills"), Kind: RootKnownAgent, Known: true},
		{ID: "junie", Label: "Junie", LogicalAgent: "Junie", Path: filepath.Join(home, ".junie", "skills"), Kind: RootKnownAgent, Known: true},
		{ID: "kilocode", Label: "Kilo Code", LogicalAgent: "Kilo Code", Path: filepath.Join(home, ".kilocode", "skills"), Kind: RootKnownAgent, Known: true},
		{ID: "kode", Label: "Kode", LogicalAgent: "Kode", Path: filepath.Join(home, ".kode", "skills"), Kind: RootKnownAgent, Known: true},
		{ID: "lingma", Label: "Lingma", LogicalAgent: "Lingma", Path: filepath.Join(home, ".lingma", "skills"), Kind: RootKnownAgent, Known: true},
		{ID: "mcpjam", Label: "MCPJam", LogicalAgent: "MCPJam", Path: filepath.Join(home, ".mcpjam", "skills"), Kind: RootKnownAgent, Known: true},
		{ID: "moltbot", Label: "Moltbot", LogicalAgent: "Moltbot", Path: filepath.Join(home, ".moltbot", "skills"), Kind: RootKnownAgent, Known: true},
		{ID: "mux", Label: "Mux", LogicalAgent: "Mux", Path: filepath.Join(home, ".mux", "skills"), Kind: RootKnownAgent, Known: true},
		{ID: "neovate", Label: "Neovate", LogicalAgent: "Neovate", Path: filepath.Join(home, ".neovate", "skills"), Kind: RootKnownAgent, Known: true},
		{ID: "openhands", Label: "OpenHands", LogicalAgent: "OpenHands", Path: filepath.Join(home, ".openhands", "skills"), Kind: RootKnownAgent, Known: true},
		{ID: "pochi", Label: "Pochi", LogicalAgent: "Pochi", Path: filepath.Join(home, ".pochi", "skills"), Kind: RootKnownAgent, Known: true},
		{ID: "qwen", Label: "Qwen", LogicalAgent: "Qwen", Path: filepath.Join(home, ".qwen", "skills"), Kind: RootKnownAgent, Known: true},
		{ID: "roo", Label: "Roo Code", LogicalAgent: "Roo Code", Path: filepath.Join(home, ".roo", "skills"), Kind: RootKnownAgent, Known: true},
		{ID: "zencoder", Label: "Zencoder", LogicalAgent: "Zencoder", Path: filepath.Join(home, ".zencoder", "skills"), Kind: RootKnownAgent, Known: true},
		{ID: "omp", Label: "OMP", LogicalAgent: "OMP", Path: filepath.Join(home, ".omp", "agent", "managed-skills"), Kind: RootKnownAgent, Known: true, SharedSkills: true},
	}
	return roots
}

// DiscoverUnknownRoots looks only at shallow, conventional config locations.
// It deliberately avoids walking the whole home directory. A discovered root
// is a fact about a directory, not a claim that its agent runtime is supported.
func containsSkill(dir string) bool {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return false
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		if info, err := os.Stat(filepath.Join(dir, entry.Name(), "SKILL.md")); err == nil && !info.IsDir() {
			return true
		}
	}
	return false
}

func (LocalPaths) DiscoverUnknownRoots(home string, known []RootSpec) ([]RootSpec, error) {
	knownPaths := make(map[string]struct{}, len(known))
	for _, root := range known {
		knownPaths[cleanPath(root.Path)] = struct{}{}
	}

	parents := []string{home, filepath.Join(home, ".config"), filepath.Join(home, ".local", "share")}
	seen := map[string]struct{}{}
	var candidates []string
	for _, parent := range parents {
		entries, err := os.ReadDir(parent)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) || errors.Is(err, os.ErrPermission) {
				continue
			}
			return nil, err
		}
		for _, entry := range entries {
			if !entry.IsDir() || strings.HasPrefix(entry.Name(), ".") && parent != home {
				continue
			}
			candidate := filepath.Join(parent, entry.Name(), "skills")
			if parent == home {
				candidate = filepath.Join(parent, entry.Name(), "skills")
			}
			candidate = cleanPath(candidate)
			if _, ok := knownPaths[candidate]; ok {
				continue
			}
			if _, ok := seen[candidate]; ok {
				continue
			}
			if info, err := os.Stat(candidate); err == nil && info.IsDir() && containsSkill(candidate) {
				seen[candidate] = struct{}{}
				candidates = append(candidates, candidate)
			}
		}
	}
	sort.Strings(candidates)
	roots := make([]RootSpec, 0, len(candidates))
	for i, path := range candidates {
		label := filepath.Base(filepath.Dir(path))
		roots = append(roots, RootSpec{
			ID:           "unknown-" + stableSlug(path, i),
			Label:        label,
			LogicalAgent: label,
			Path:         path,
			Kind:         RootUnknownAgent,
			Known:        false,
		})
	}
	return roots, nil
}

func (LocalPaths) ProjectRoots(workspace string) []RootSpec {
	workspace = cleanPath(workspace)
	if workspace == "" {
		return nil
	}
	conventions := []struct {
		id   string
		path string
	}{
		{"agents", filepath.Join(workspace, ".agents", "skills")},
		{"claude", filepath.Join(workspace, ".claude", "skills")},
		{"codex", filepath.Join(workspace, ".codex", "skills")},
		{"cursor", filepath.Join(workspace, ".cursor", "skills")},
	}
	var roots []RootSpec
	for _, convention := range conventions {
		if info, err := os.Stat(convention.path); err == nil && info.IsDir() {
			roots = append(roots, RootSpec{
				ID:           convention.id + "-project-" + stableSlug(workspace, len(roots)),
				Label:        "Project " + convention.id,
				LogicalAgent: convention.id,
				Path:         convention.path,
				Kind:         RootProject,
				Known:        true,
			})
		}
	}
	return roots
}

// PathDirs returns the directories used for the read-only CLI inventory.
// The desktop process may inherit a minimal PATH (especially when launched
// from Finder), so common package-manager and user bin directories are added
// when they exist. This never executes a command or reads shell profiles.
func (LocalPaths) PathDirs(home string) []string {
	// Fixture mode intentionally disables host PATH scanning so tests never
	// depend on the developer machine's installed commands.
	if strings.TrimSpace(firstEnv("LARDER_HOME", "AAM_HOME")) != "" {
		if override := strings.TrimSpace(firstEnv("LARDER_CLI_PATH", "AAM_CLI_PATH")); override != "" {
			var dirs []string
			for _, dir := range filepath.SplitList(override) {
				if info, err := os.Stat(dir); err == nil && info.IsDir() {
					dirs = append(dirs, filepath.Clean(dir))
				}
			}
			return dirs
		}
		return nil
	}
	var raw []string
	raw = append(raw, filepath.SplitList(os.Getenv("PATH"))...)
	if runtime.GOOS == "windows" {
		localAppData := os.Getenv("LOCALAPPDATA")
		appData := os.Getenv("APPDATA")
		raw = append(raw,
			filepath.Join(localAppData, "Programs"),
			filepath.Join(localAppData, "Microsoft", "WindowsApps"),
			filepath.Join(localAppData, "Microsoft", "WinGet", "Links"),
			filepath.Join(appData, "npm"),
			filepath.Join(home, "scoop", "shims"),
			filepath.Join(home, ".bun", "bin"),
			filepath.Join(home, ".cargo", "bin"),
			filepath.Join(home, "go", "bin"),
			`C:\ProgramData\chocolatey\bin`,
		)
	} else {
		raw = append(raw,
			"/opt/homebrew/bin",
			"/opt/homebrew/sbin",
			"/usr/local/bin",
			"/usr/local/sbin",
			filepath.Join(home, ".local", "bin"),
			filepath.Join(home, "bin"),
			filepath.Join(home, "go", "bin"),
			filepath.Join(home, ".cargo", "bin"),
			filepath.Join(home, ".bun", "bin"),
			filepath.Join(home, ".npm-global", "bin"),
			filepath.Join(home, ".volta", "bin"),
			filepath.Join(home, ".asdf", "shims"),
			filepath.Join(home, ".local", "share", "pnpm"),
		)
	}
	seen := map[string]bool{}
	dirs := make([]string, 0, len(raw))
	for _, dir := range raw {
		dir = cleanPath(dir)
		if dir == "" || dir == "." || seen[dir] {
			continue
		}
		if isSystemBin(dir) {
			continue
		}
		if info, err := os.Stat(dir); err != nil || !info.IsDir() {
			continue
		}
		seen[dir] = true
		dirs = append(dirs, dir)
	}
	return dirs
}

func isSystemBin(dir string) bool {
	clean := filepath.Clean(dir)
	if runtime.GOOS == "windows" {
		lower := strings.ToLower(filepath.ToSlash(clean))
		return strings.Contains(lower, "/windows/system32") || strings.HasPrefix(lower, "c:/windows/")
	}
	switch clean {
	case "/bin", "/sbin", "/usr/bin", "/usr/sbin", "/usr/local/opt":
		return true
	}
	return strings.HasPrefix(clean, "/System/") || strings.HasPrefix(clean, "/usr/libexec")
}

func cleanPath(path string) string {
	if path == "" {
		return ""
	}
	if abs, err := filepath.Abs(path); err == nil {
		return filepath.Clean(abs)
	}
	return filepath.Clean(path)
}

func stableSlug(value string, salt int) string {
	value = strings.ReplaceAll(value, string(filepath.Separator), "-")
	value = strings.Trim(value, "-.")
	if len(value) > 32 {
		value = value[len(value)-32:]
	}
	if value == "" {
		value = "root"
	}
	return value + "-" + string(rune('a'+(salt%26)))
}

func PlatformName() string { return runtime.GOOS }

func firstEnv(keys ...string) string {
	for _, key := range keys {
		if value := strings.TrimSpace(os.Getenv(key)); value != "" {
			return value
		}
	}
	return ""
}
