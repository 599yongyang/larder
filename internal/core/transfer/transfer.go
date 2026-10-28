package transfer

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	coremcp "larder/internal/core/mcp"
	"larder/internal/core/model"
)

const SchemaVersion = "1"
const maxPackageBytes = 64 << 20

// Selection controls which parts of the local Larder state are exported,
// inspected or restored. Zero values mean "not selected".
type Selection struct {
	Settings     bool `json:"settings"`
	AppState     bool `json:"appState"`
	MCP          bool `json:"mcp"`
	Instructions bool `json:"instructions"`
	Commands     bool `json:"commands"`
	Skills       bool `json:"skills"`
}

func (s Selection) Empty() bool {
	return !s.Settings && !s.AppState && !s.MCP && !s.Instructions && !s.Commands && !s.Skills
}

func (s Selection) Allows(kind string) bool {
	switch kind {
	case "settings":
		return s.Settings
	case "app":
		return s.AppState
	case "mcp":
		return s.MCP
	case "instruction":
		return s.Instructions
	case "command":
		return s.Commands
	case "skill":
		return s.Skills
	}
	return false
}

type File struct {
	Entry  string `json:"entry"`
	Target string `json:"target,omitempty"`
	Kind   string `json:"kind"`
	Hash   string `json:"hash,omitempty"`
}

type Roots struct {
	Home     string   `json:"home,omitempty"`
	Data     string   `json:"data,omitempty"`
	Projects []string `json:"projects,omitempty"`
}

type Manifest struct {
	SchemaVersion string    `json:"schemaVersion"`
	CreatedAt     time.Time `json:"createdAt"`
	AppVersion    string    `json:"appVersion,omitempty"`
	Selection     Selection `json:"selection"`
	Roots         Roots     `json:"roots,omitempty"`
	Files         []File    `json:"files"`
}

type Info struct {
	CreatedAt  time.Time       `json:"createdAt"`
	Selection  Selection       `json:"selection"`
	Total      int             `json:"total"`
	Categories map[string]int  `json:"categories"`
	Settings   *model.Settings `json:"settings,omitempty"`
}

type Result struct {
	Restored    int             `json:"restored"`
	Skipped     int             `json:"skipped"`
	SnapshotDir string          `json:"snapshotDir"`
	Settings    *model.Settings `json:"settings,omitempty"`
}

type Options struct {
	DataDir      string
	HomeDir      string
	Projects     []string
	Settings     model.Settings
	Instructions []model.InstructionFile
	Commands     []model.Command
	Skills       []model.Skill
	Selection    Selection
	AppVersion   string
}

// Build returns a portable ZIP containing only the selected categories. MCP
// config files may contain environment values or headers; callers should warn
// the user before uploading a package to a remote service.
func Build(opts Options) ([]byte, Manifest, error) {
	if opts.Selection.Empty() {
		return nil, Manifest{}, errors.New("select at least one category")
	}
	var buf bytes.Buffer
	writer := zip.NewWriter(&buf)
	manifest := Manifest{SchemaVersion: SchemaVersion, CreatedAt: time.Now(), AppVersion: opts.AppVersion, Selection: opts.Selection, Roots: Roots{Home: filepath.Clean(opts.HomeDir), Data: filepath.Clean(opts.DataDir), Projects: append([]string(nil), opts.Projects...)}}
	add := func(entry, target, kind string, raw []byte) error {
		handle, err := writer.Create(entry)
		if err != nil {
			return err
		}
		if _, err := handle.Write(raw); err != nil {
			return err
		}
		manifest.Files = append(manifest.Files, File{Entry: entry, Target: target, Kind: kind})
		return nil
	}
	if opts.Selection.Skills {
		seen := map[string]bool{}
		for index, skill := range opts.Skills {
			if skill.Path == "" || skill.ScanState == "broken" || !skill.HasSkillMD || seen[filepath.Clean(skill.Path)] {
				continue
			}
			seen[filepath.Clean(skill.Path)] = true
			info, err := os.Stat(skill.Path)
			if err != nil {
				continue
			}
			addFile := func(path, rel string) error {
				stat, err := os.Stat(path)
				if err != nil || stat.IsDir() || stat.Size() > 8<<20 {
					return nil
				}
				raw, err := os.ReadFile(path)
				if err != nil {
					return nil
				}
				entry := "skills/" + safeName(skill.Name) + "-" + fmt.Sprint(index) + "/" + filepath.ToSlash(rel)
				if seen[entry] {
					return nil
				}
				seen[entry] = true
				return add(entry, path, "skill", raw)
			}
			if !info.IsDir() {
				if err := addFile(skill.Path, filepath.Base(skill.Path)); err != nil {
					return nil, Manifest{}, err
				}
				continue
			}
			err = filepath.WalkDir(skill.Path, func(path string, entry os.DirEntry, walkErr error) error {
				if walkErr != nil {
					return nil
				}
				if path == skill.Path {
					return nil
				}
				if entry.Type()&os.ModeSymlink != 0 {
					if entry.IsDir() {
						return filepath.SkipDir
					}
					return nil
				}
				if entry.IsDir() {
					return nil
				}
				rel, err := filepath.Rel(skill.Path, path)
				if err != nil {
					return nil
				}
				return addFile(path, rel)
			})
			if err != nil {
				return nil, Manifest{}, err
			}
		}
	}
	if opts.Selection.Settings {
		raw, err := json.MarshalIndent(opts.Settings, "", "  ")
		if err != nil {
			return nil, Manifest{}, err
		}
		if err := add("settings.json", "", "settings", raw); err != nil {
			return nil, Manifest{}, err
		}
	}
	if opts.Selection.AppState {
		for _, name := range []string{"catalog-installs.json", "disabled-mcp.json"} {
			path := filepath.Join(opts.DataDir, name)
			raw, err := os.ReadFile(path)
			if errors.Is(err, os.ErrNotExist) {
				continue
			}
			if err != nil {
				return nil, Manifest{}, err
			}
			if err := add("app/"+name, path, "app", raw); err != nil {
				return nil, Manifest{}, err
			}
		}
	}
	if opts.Selection.MCP {
		targets := append(coremcp.WritableTargets(), coremcp.ProjectTargets(opts.Projects)...)
		seen := map[string]bool{}
		for _, target := range targets {
			if seen[target.Path] {
				continue
			}
			seen[target.Path] = true
			raw, err := os.ReadFile(target.Path)
			if errors.Is(err, os.ErrNotExist) {
				continue
			}
			if err != nil {
				return nil, Manifest{}, err
			}
			entry := "mcp/" + safeName(target.Label) + "/" + filepath.Base(target.Path)
			if err := add(entry, target.Path, "mcp", raw); err != nil {
				return nil, Manifest{}, err
			}
		}
	}
	if opts.Selection.Instructions {
		for _, file := range opts.Instructions {
			raw, err := os.ReadFile(file.Path)
			if errors.Is(err, os.ErrNotExist) {
				continue
			}
			if err != nil {
				return nil, Manifest{}, err
			}
			entry := "instructions/" + safeName(file.Scope)
			if file.Project != "" {
				entry += "/" + safeName(file.Project)
			}
			entry += "/" + file.Name
			if err := add(entry, file.Path, "instruction", raw); err != nil {
				return nil, Manifest{}, err
			}
		}
	}
	if opts.Selection.Commands {
		for _, command := range opts.Commands {
			raw, err := os.ReadFile(command.Path)
			if errors.Is(err, os.ErrNotExist) {
				continue
			}
			if err != nil {
				return nil, Manifest{}, err
			}
			entry := "commands/" + safeName(command.Agent) + "/" + safeName(command.Scope)
			if command.Project != "" {
				entry += "/" + safeName(command.Project)
			}
			entry += "/" + command.Name + filepath.Ext(command.Path)
			if err := add(entry, command.Path, "command", raw); err != nil {
				return nil, Manifest{}, err
			}
		}
	}
	raw, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return nil, Manifest{}, err
	}
	handle, err := writer.Create("manifest.json")
	if err != nil {
		return nil, Manifest{}, err
	}
	if _, err := handle.Write(raw); err != nil {
		return nil, Manifest{}, err
	}
	if err := writer.Close(); err != nil {
		return nil, Manifest{}, err
	}
	if buf.Len() > maxPackageBytes {
		return nil, Manifest{}, errors.New("data package is too large")
	}
	return buf.Bytes(), manifest, nil
}

// Inspect reads and validates a package without writing anything.
func Inspect(data []byte) (Info, error) {
	manifest, _, entries, err := read(data)
	if err != nil {
		return Info{}, err
	}
	info := Info{CreatedAt: manifest.CreatedAt, Selection: manifest.Selection, Total: len(manifest.Files), Categories: map[string]int{}}
	for _, file := range manifest.Files {
		info.Categories[file.Kind]++
		if file.Kind == "settings" {
			entry := entries[file.Entry]
			if entry == nil {
				continue
			}
			raw, err := readEntry(entry)
			if err != nil {
				return Info{}, err
			}
			var settings model.Settings
			if json.Unmarshal(raw, &settings) == nil {
				info.Settings = &settings
			}
		}
	}
	return info, nil
}

// Restore writes the selected categories and returns parsed settings for the
// caller to apply through the normal settings save path.
type writeItem struct {
	target   string
	data     []byte
	mode     os.FileMode
	existed  bool
	original []byte
}

func Restore(data []byte, selection Selection, applySettings func(*model.Settings) error, allowedRoots ...string) (Result, error) {
	manifest, _, entries, err := read(data)
	if err != nil {
		return Result{}, err
	}
	if selection.Empty() {
		return Result{}, errors.New("select at least one category")
	}
	dataDir := ""
	if len(allowedRoots) > 0 {
		dataDir = allowedRoots[0]
	}
	allowed := append([]string(nil), allowedRoots...)
	home := ""
	if len(allowed) > 1 {
		home = allowed[1]
	}
	projects := []string{}
	if len(allowed) > 2 {
		projects = allowed[2:]
	}
	result := Result{}
	var importedSettings *model.Settings
	var writes []writeItem
	seen := map[string]bool{}
	for _, file := range manifest.Files {
		if !selection.Allows(file.Kind) {
			continue
		}
		entry := entries[file.Entry]
		if entry == nil {
			result.Skipped++
			continue
		}
		raw, err := readEntry(entry)
		if err != nil {
			return Result{}, err
		}
		if file.Kind == "settings" {
			var settings model.Settings
			if err := json.Unmarshal(raw, &settings); err != nil {
				return Result{}, err
			}
			importedSettings = &settings
			result.Restored++
			continue
		}
		target, ok := mapTarget(file.Target, manifest.Roots, dataDir, home, projects)
		if !ok || seen[target] {
			result.Skipped++
			continue
		}
		seen[target] = true
		mapped := file
		mapped.Target = target
		if err := validateTarget(mapped, dataDir, allowed); err != nil {
			return Result{}, err
		}
		item := writeItem{target: target, data: raw, mode: 0o644}
		if file.Kind == "app" && filepath.Base(target) == "disabled-mcp.json" {
			item.mode = 0o600
		}
		if existing, err := os.ReadFile(target); err == nil {
			if bytes.Equal(existing, raw) {
				result.Skipped++
				continue
			}
			item.existed = true
			item.original = existing
		}
		writes = append(writes, item)
	}
	if len(writes) == 0 && importedSettings == nil {
		return result, nil
	}
	snapshotDir := filepath.Join(dataDir, "restore-snapshots", time.Now().Format("20060102-150405"))
	if len(writes) > 0 {
		if err := os.MkdirAll(snapshotDir, 0o700); err != nil {
			return Result{}, err
		}
	}
	result.SnapshotDir = snapshotDir
	rollback := func() {
		for i := len(writes) - 1; i >= 0; i-- {
			item := writes[i]
			if item.existed {
				_ = os.MkdirAll(filepath.Dir(item.target), 0o700)
				_ = os.WriteFile(item.target, item.original, item.mode)
			} else {
				_ = os.Remove(item.target)
			}
		}
	}
	written := 0
	for index, item := range writes {
		if item.existed {
			backupName := fmt.Sprintf("%03d-%s", index, filepath.Base(item.target))
			if err := os.WriteFile(filepath.Join(snapshotDir, backupName), item.original, 0o600); err != nil {
				rollback()
				return Result{}, err
			}
		}
		if err := os.MkdirAll(filepath.Dir(item.target), 0o700); err != nil {
			rollback()
			return Result{}, err
		}
		if err := os.WriteFile(item.target, item.data, item.mode); err != nil {
			rollback()
			return Result{}, err
		}
		written++
		result.Restored++
	}
	result.Settings = importedSettings
	if importedSettings != nil && applySettings != nil {
		if err := applySettings(importedSettings); err != nil {
			rollback()
			return Result{}, err
		}
	}
	return result, nil
}

func read(data []byte) (Manifest, *zip.Reader, map[string]*zip.File, error) {
	if len(data) > maxPackageBytes {
		return Manifest{}, nil, nil, errors.New("data package is too large")
	}
	reader, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return Manifest{}, nil, nil, err
	}
	entries := map[string]*zip.File{}
	var manifest Manifest
	for _, file := range reader.File {
		entries[file.Name] = file
		if file.Name == "manifest.json" {
			raw, err := readEntry(file)
			if err != nil {
				return Manifest{}, nil, nil, err
			}
			if err := json.Unmarshal(raw, &manifest); err != nil {
				return Manifest{}, nil, nil, err
			}
		}
	}
	if manifest.SchemaVersion == "" {
		return Manifest{}, nil, nil, errors.New("manifest is missing")
	}
	if manifest.SchemaVersion != SchemaVersion {
		return Manifest{}, nil, nil, fmt.Errorf("unsupported package schema %s", manifest.SchemaVersion)
	}
	return manifest, reader, entries, nil
}

func readEntry(file *zip.File) ([]byte, error) {
	handle, err := file.Open()
	if err != nil {
		return nil, err
	}
	defer handle.Close()
	return io.ReadAll(io.LimitReader(handle, maxPackageBytes))
}

// mapTarget rewrites a package target that was captured on another machine
// onto the current home/data/project roots.
func mapTarget(target string, roots Roots, dataDir, home string, projects []string) (string, bool) {
	clean := filepath.Clean(target)
	if roots.Data != "" && roots.Data != "." && (clean == roots.Data || strings.HasPrefix(clean, roots.Data+string(filepath.Separator))) {
		return filepath.Join(dataDir, strings.TrimPrefix(clean, roots.Data+string(filepath.Separator))), true
	}
	for _, source := range roots.Projects {
		source = filepath.Clean(source)
		if source == "" || source == "." || !(clean == source || strings.HasPrefix(clean, source+string(filepath.Separator))) {
			continue
		}
		base := filepath.Base(source)
		for _, destination := range projects {
			if filepath.Base(destination) != base {
				continue
			}
			return filepath.Join(destination, strings.TrimPrefix(clean, source+string(filepath.Separator))), true
		}
		return "", false
	}
	if roots.Home != "" && roots.Home != "." && (clean == roots.Home || strings.HasPrefix(clean, roots.Home+string(filepath.Separator))) {
		return filepath.Join(home, strings.TrimPrefix(clean, roots.Home+string(filepath.Separator))), true
	}
	return clean, true
}

func validateTarget(file File, dataDir string, allowed []string) error {
	if file.Target == "" || filepath.Base(file.Target) == "." || strings.Contains(file.Target, "..") {
		return errors.New("invalid target path")
	}
	clean := filepath.Clean(file.Target)
	if file.Kind == "app" {
		if filepath.Dir(clean) != filepath.Clean(dataDir) {
			return fmt.Errorf("invalid app target: %s", clean)
		}
		base := filepath.Base(clean)
		if base != "catalog-installs.json" && base != "disabled-mcp.json" {
			return fmt.Errorf("invalid app target: %s", clean)
		}
		return nil
	}
	if !hasAllowedPrefix(clean, allowed) {
		return fmt.Errorf("target outside workspace: %s", clean)
	}
	switch file.Kind {
	case "mcp":
		switch strings.ToLower(filepath.Ext(clean)) {
		case ".json", ".toml":
			return nil
		}
	case "instruction":
		switch strings.ToLower(filepath.Ext(clean)) {
		case ".md", ".mdc", ".markdown", ".txt":
			return nil
		}
	case "command":
		if strings.ToLower(filepath.Ext(clean)) == ".md" {
			return nil
		}
	case "skill":
		return nil
	}
	return fmt.Errorf("invalid target kind: %s", file.Kind)
}

func hasAllowedPrefix(path string, roots []string) bool {
	for _, root := range roots {
		root = filepath.Clean(root)
		if root != "" && root != "." && (path == root || strings.HasPrefix(path, root+string(filepath.Separator))) {
			return true
		}
	}
	return false
}

func safeName(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "unknown"
	}
	return strings.NewReplacer("/", "-", "\\", "-", " ", "_", ":", "-").Replace(value)
}
