package backup

import (
	"archive/zip"
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

type File struct {
	Entry  string `json:"entry"`
	Target string `json:"target"`
	Kind   string `json:"kind"`
}

type Manifest struct {
	SchemaVersion    string    `json:"schemaVersion"`
	CreatedAt        time.Time `json:"createdAt"`
	Projects         []string  `json:"projects"`
	InstructionCount int       `json:"instructionCount"`
	Files            []File    `json:"files"`
}

type Info struct {
	Path         string    `json:"path"`
	CreatedAt    time.Time `json:"createdAt"`
	Total        int       `json:"total"`
	Configs      int       `json:"configs"`
	Instructions int       `json:"instructions"`
	Apps         int       `json:"apps"`
	Projects     []string  `json:"projects"`
}

type Result struct {
	Restored    int    `json:"restored"`
	Skipped     int    `json:"skipped"`
	SnapshotDir string `json:"snapshotDir"`
}

// Create writes a local zip with app state files, MCP configs and instruction
// files. It never uploads anything.
func Create(dataDir string, projects []string, instructions []model.InstructionFile) (string, error) {
	dir := filepath.Join(dataDir, "backups")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	path := filepath.Join(dir, "larder-backup-"+time.Now().Format("20060102-150405")+".zip")
	file, err := os.Create(path)
	if err != nil {
		return "", err
	}
	defer file.Close()
	writer := zip.NewWriter(file)
	manifest := Manifest{SchemaVersion: SchemaVersion, CreatedAt: time.Now(), Projects: append([]string(nil), projects...), InstructionCount: len(instructions)}
	targets := map[string]bool{}
	add := func(entryName, target, kind string) error {
		if targets[target] {
			return nil
		}
		raw, err := os.ReadFile(target)
		if err != nil {
			if os.IsNotExist(err) {
				return nil
			}
			return err
		}
		entry, err := writer.Create(entryName)
		if err != nil {
			return err
		}
		if _, err := entry.Write(raw); err != nil {
			return err
		}
		targets[target] = true
		manifest.Files = append(manifest.Files, File{Entry: entryName, Target: target, Kind: kind})
		return nil
	}
	for _, name := range []string{"catalog-installs.json", "disabled-mcp.json"} {
		if err := add("app/"+name, filepath.Join(dataDir, name), "app"); err != nil {
			return "", err
		}
	}
	for _, target := range append(coremcp.WritableTargets(), coremcp.ProjectTargets(projects)...) {
		if err := add(configEntryPath(target.Label, target.Path), target.Path, "mcp"); err != nil {
			return "", err
		}
	}
	for _, file := range instructions {
		if err := add(instructionEntryPath(file), file.Path, "instruction"); err != nil {
			return "", err
		}
	}
	raw, _ := json.MarshalIndent(manifest, "", "  ")
	entry, err := writer.Create("manifest.json")
	if err != nil {
		return "", err
	}
	if _, err := entry.Write(raw); err != nil {
		return "", err
	}
	if err := writer.Close(); err != nil {
		return "", err
	}
	return path, nil
}

func Inspect(path string) (Info, error) {
	reader, err := zip.OpenReader(path)
	if err != nil {
		return Info{}, err
	}
	defer reader.Close()
	manifest, err := readManifest(reader)
	if err != nil {
		return Info{}, err
	}
	info := Info{Path: path, CreatedAt: manifest.CreatedAt, Total: len(manifest.Files), Projects: manifest.Projects}
	for _, file := range manifest.Files {
		switch file.Kind {
		case "mcp":
			info.Configs++
		case "instruction":
			info.Instructions++
		case "app":
			info.Apps++
		}
	}
	return info, nil
}

func Restore(path, dataDir, home string, projects []string) (Result, error) {
	reader, err := zip.OpenReader(path)
	if err != nil {
		return Result{}, err
	}
	defer reader.Close()
	manifest, err := readManifest(reader)
	if err != nil {
		return Result{}, err
	}
	entries := map[string]*zip.File{}
	for _, file := range reader.File {
		entries[file.Name] = file
	}
	allowed := []string{dataDir, home}
	allowed = append(allowed, projects...)
	for _, file := range manifest.Files {
		if err := validateTarget(file, dataDir, allowed); err != nil {
			return Result{}, err
		}
	}
	snapshotDir := filepath.Join(dataDir, "restore-snapshots", time.Now().Format("20060102-150405"))
	if err := os.MkdirAll(snapshotDir, 0o700); err != nil {
		return Result{}, err
	}
	result := Result{SnapshotDir: snapshotDir}
	created := map[string]bool{}
	for index, file := range manifest.Files {
		entry := entries[file.Entry]
		if entry == nil {
			result.Skipped++
			continue
		}
		raw, err := readEntry(entry)
		if err != nil {
			return Result{}, err
		}
		if existing, err := os.ReadFile(file.Target); err == nil {
			backupName := fmt.Sprintf("%03d-%s", index, filepath.Base(file.Target))
			if err := os.WriteFile(filepath.Join(snapshotDir, backupName), existing, 0o600); err != nil {
				return Result{}, err
			}
		}
		if err := os.MkdirAll(filepath.Dir(file.Target), 0o700); err != nil {
			return Result{}, err
		}
		mode := os.FileMode(0o644)
		if file.Kind == "app" && filepath.Base(file.Target) == "disabled-mcp.json" {
			mode = 0o600
		}
		if err := os.WriteFile(file.Target, raw, mode); err != nil {
			return Result{}, err
		}
		created[file.Entry] = true
		result.Restored++
	}
	return result, nil
}

func validateTarget(file File, dataDir string, allowed []string) error {
	if filepath.Base(file.Target) == "." || strings.Contains(file.Target, "..") {
		return errors.New("invalid backup target")
	}
	clean := filepath.Clean(file.Target)
	switch file.Kind {
	case "app":
		base := filepath.Base(clean)
		if base != "catalog-installs.json" && base != "disabled-mcp.json" {
			return fmt.Errorf("invalid app backup target: %s", clean)
		}
		if filepath.Dir(clean) != filepath.Clean(dataDir) {
			return fmt.Errorf("invalid app backup target: %s", clean)
		}
	case "mcp":
		if !hasAllowedPrefix(clean, allowed) {
			return fmt.Errorf("mcp target outside workspace: %s", clean)
		}
		ext := strings.ToLower(filepath.Ext(clean))
		if ext != ".json" && ext != ".toml" {
			return fmt.Errorf("invalid mcp target: %s", clean)
		}
	case "instruction":
		if !hasAllowedPrefix(clean, allowed) {
			return fmt.Errorf("instruction target outside workspace: %s", clean)
		}
		switch strings.ToLower(filepath.Ext(clean)) {
		case ".md", ".mdc", ".markdown", ".txt":
		default:
			return fmt.Errorf("invalid instruction target: %s", clean)
		}
	default:
		return fmt.Errorf("unknown backup entry kind: %s", file.Kind)
	}
	return nil
}

func hasAllowedPrefix(path string, roots []string) bool {
	for _, root := range roots {
		root = filepath.Clean(root)
		if root != "" && (path == root || strings.HasPrefix(path, root+string(filepath.Separator))) {
			return true
		}
	}
	return false
}

func readManifest(reader *zip.ReadCloser) (Manifest, error) {
	var manifest Manifest
	for _, file := range reader.File {
		if file.Name != "manifest.json" {
			continue
		}
		raw, err := readEntry(file)
		if err != nil {
			return Manifest{}, err
		}
		if err := json.Unmarshal(raw, &manifest); err != nil {
			return Manifest{}, err
		}
		if manifest.SchemaVersion == "" {
			manifest.SchemaVersion = "1"
		}
		return manifest, nil
	}
	return Manifest{}, errors.New("backup manifest is missing")
}

func readEntry(file *zip.File) ([]byte, error) {
	handle, err := file.Open()
	if err != nil {
		return nil, err
	}
	defer handle.Close()
	return io.ReadAll(handle)
}

func configEntryPath(label, path string) string {
	name := strings.NewReplacer("/", "-", "\\", "-", " ", "_").Replace(label)
	return "mcp/" + name + "/" + filepath.Base(path)
}

func instructionEntryPath(file model.InstructionFile) string {
	prefix := "instructions/" + file.Scope + "/"
	if file.Project != "" {
		prefix += strings.NewReplacer("/", "-", " ", "_").Replace(file.Project) + "/"
	}
	return prefix + file.Name
}
