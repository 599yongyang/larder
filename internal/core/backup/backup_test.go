package backup

import (
	"archive/zip"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"larder/internal/core/model"
)

func TestCreateIncludesStateFiles(t *testing.T) {
	dataDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dataDir, "catalog-installs.json"), []byte(`[]`), 0o600); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "AGENTS.md")
	if err := os.WriteFile(path, []byte("rules"), 0o600); err != nil {
		t.Fatal(err)
	}
	backup, err := Create(dataDir, nil, []model.InstructionFile{{Name: "AGENTS.md", Path: path, Scope: "project", Project: "demo"}})
	if err != nil {
		t.Fatal(err)
	}
	reader, err := zip.OpenReader(backup)
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	found := map[string]bool{}
	for _, entry := range reader.File {
		found[entry.Name] = true
	}
	if !found["app/catalog-installs.json"] || !found["instructions/project/demo/AGENTS.md"] {
		t.Fatalf("entries = %v", found)
	}
}

func TestRestoreRoundTrip(t *testing.T) {
	dataDir := t.TempDir()
	home := t.TempDir()
	project := filepath.Join(home, "demo")
	if err := os.MkdirAll(project, 0o755); err != nil {
		t.Fatal(err)
	}
	instruction := filepath.Join(project, "AGENTS.md")
	if err := os.WriteFile(instruction, []byte("after"), 0o644); err != nil {
		t.Fatal(err)
	}
	config := filepath.Join(project, ".mcp.json")
	if err := os.WriteFile(config, []byte(`{"mcpServers":{}}`), 0o644); err != nil {
		t.Fatal(err)
	}

	archive := filepath.Join(t.TempDir(), "backup.zip")
	file, err := os.Create(archive)
	if err != nil {
		t.Fatal(err)
	}
	writer := zip.NewWriter(file)
	manifest := Manifest{SchemaVersion: SchemaVersion, Files: []File{
		{Entry: "app/catalog-installs.json", Target: filepath.Join(dataDir, "catalog-installs.json"), Kind: "app"},
		{Entry: "instructions/project/demo/AGENTS.md", Target: instruction, Kind: "instruction"},
		{Entry: "mcp/demo/.mcp.json", Target: config, Kind: "mcp"},
	}}
	raw, _ := json.Marshal(manifest)
	entry, _ := writer.Create("manifest.json")
	_, _ = entry.Write(raw)
	for name, body := range map[string]string{
		"app/catalog-installs.json":           `{"a":1}`,
		"instructions/project/demo/AGENTS.md": "before",
		"mcp/demo/.mcp.json":                  `{"mcpServers":{"demo":{"command":"npx"}}}`,
	} {
		entry, _ := writer.Create(name)
		_, _ = entry.Write([]byte(body))
	}
	_ = writer.Close()
	_ = file.Close()

	info, err := Inspect(archive)
	if err != nil {
		t.Fatal(err)
	}
	if info.Instructions != 1 || info.Apps != 1 || info.Configs != 1 {
		t.Fatalf("info = %+v", info)
	}
	result, err := Restore(archive, dataDir, home, []string{project})
	if err != nil {
		t.Fatal(err)
	}
	if result.Restored != 3 {
		t.Fatalf("result = %+v", result)
	}
	body, _ := os.ReadFile(instruction)
	if string(body) != "before" {
		t.Fatalf("instruction = %q", body)
	}
}
