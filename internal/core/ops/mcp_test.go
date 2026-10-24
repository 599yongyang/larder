package ops

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"larder/internal/core/model"
	"larder/internal/platform"
)

func TestPlanMCPRemoveJSONAndTOML(t *testing.T) {
	dir := t.TempDir()
	jsonPath := filepath.Join(dir, "mcp.json")
	tomlPath := filepath.Join(dir, "config.toml")
	jsonOriginal := `{"mcpServers":{"demo":{"command":"npx"}},"other":true}`
	tomlOriginal := "[mcp_servers.demo]\ncommand = \"npx\"\n\n[other]\nvalue = 1\n"
	if err := os.WriteFile(jsonPath, []byte(jsonOriginal), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(tomlPath, []byte(tomlOriginal), 0o600); err != nil {
		t.Fatal(err)
	}
	server := model.MCPServer{ID: "mcp:test", Name: "demo", Agents: []model.MCPAgent{{Label: "A", Path: jsonPath}, {Label: "B", Path: tomlPath}}}
	plan, err := PlanMCPRemove(server, []string{jsonPath, tomlPath})
	if err != nil {
		t.Fatal(err)
	}
	op, err := ApplyMCPRemove(context.Background(), plan, filepath.Join(dir, "snapshots"), platform.OSFileSystem{})
	if err != nil {
		t.Fatal(err)
	}
	if op.Status != "success" {
		t.Fatalf("op = %#v", op)
	}
	raw, _ := os.ReadFile(jsonPath)
	var root map[string]any
	if err := json.Unmarshal(raw, &root); err != nil {
		t.Fatal(err)
	}
	if servers := root["mcpServers"].(map[string]any); len(servers) != 0 {
		t.Fatalf("json servers = %#v", servers)
	}
	toml, _ := os.ReadFile(tomlPath)
	if strings.Contains(string(toml), "mcp_servers.demo") || !strings.Contains(string(toml), "[other]") {
		t.Fatalf("toml = %s", toml)
	}
	if err := RollbackSnapshot(op.SnapshotPath, platform.OSFileSystem{}, platform.OSLinker{}); err != nil {
		t.Fatal(err)
	}
	raw, _ = os.ReadFile(jsonPath)
	var restored map[string]any
	if err := json.Unmarshal(raw, &restored); err != nil {
		t.Fatal(err)
	}
	if len(restored["mcpServers"].(map[string]any)) != 1 {
		t.Fatalf("restored = %#v", restored)
	}
	toml, _ = os.ReadFile(tomlPath)
	if string(toml) != tomlOriginal {
		t.Fatalf("toml restored = %s", toml)
	}
}
