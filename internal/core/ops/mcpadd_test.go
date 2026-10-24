package ops

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"larder/internal/core/model"
)

func TestMCPAddJSONAndTOML(t *testing.T) {
	dir := t.TempDir()
	snapshot := filepath.Join(dir, "snapshots")
	jsonPath := filepath.Join(dir, "mcp.json")
	tomlPath := filepath.Join(dir, "config.toml")
	if err := os.WriteFile(jsonPath, []byte("{\n  \"mcpServers\": {}\n}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	plan, err := PlanMCPAdd("playwright", "stdio", "npx", []string{"-y", "@playwright/mcp@latest"}, "", nil, []model.MCPAgent{{Label: "Cursor", Path: jsonPath}, {Label: "Codex", Path: tomlPath}})
	if err != nil {
		t.Fatal(err)
	}
	op, err := ApplyMCPAdd(context.Background(), plan, snapshot, nil)
	if err != nil {
		t.Fatal(err)
	}
	if op.Status != "success" {
		t.Fatalf("status = %s", op.Status)
	}
	jsonBody, _ := os.ReadFile(jsonPath)
	if !strings.Contains(string(jsonBody), "playwright") || !strings.Contains(string(jsonBody), "@playwright/mcp@latest") {
		t.Fatalf("json = %s", jsonBody)
	}
	tomlBody, _ := os.ReadFile(tomlPath)
	if !strings.Contains(string(tomlBody), "[mcp_servers.playwright]") || !strings.Contains(string(tomlBody), "command = \"npx\"") {
		t.Fatalf("toml = %s", tomlBody)
	}
}

func TestMCPAddEnvironment(t *testing.T) {
	dir := t.TempDir()
	jsonPath := filepath.Join(dir, "mcp.json")
	tomlPath := filepath.Join(dir, "config.toml")
	plan, err := PlanMCPAdd("custom", "stdio", "node", []string{"server.js"}, "", map[string]string{"API_KEY": "secret"}, []model.MCPAgent{{Label: "Cursor", Path: jsonPath}, {Label: "Codex", Path: tomlPath}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ApplyMCPAdd(context.Background(), plan, filepath.Join(dir, "snapshots"), nil); err != nil {
		t.Fatal(err)
	}
	jsonBody, _ := os.ReadFile(jsonPath)
	if !strings.Contains(string(jsonBody), "\"API_KEY\": \"secret\"") {
		t.Fatalf("json = %s", jsonBody)
	}
	tomlBody, _ := os.ReadFile(tomlPath)
	if !strings.Contains(string(tomlBody), "[mcp_servers.custom.env]") || !strings.Contains(string(tomlBody), "API_KEY = \"secret\"") {
		t.Fatalf("toml = %s", tomlBody)
	}
}

func TestMCPAddTOMLDottedName(t *testing.T) {
	dir := t.TempDir()
	tomlPath := filepath.Join(dir, "config.toml")
	plan, err := PlanMCPAdd("io.github.demo", "stdio", "node", nil, "", nil, []model.MCPAgent{{Label: "Codex", Path: tomlPath}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ApplyMCPAdd(context.Background(), plan, filepath.Join(dir, "snapshots"), nil); err != nil {
		t.Fatal(err)
	}
	body, _ := os.ReadFile(tomlPath)
	if !strings.Contains(string(body), `[mcp_servers."io.github.demo"]`) {
		t.Fatalf("toml = %s", body)
	}
}

func TestMCPAddVSCodeFormat(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "Library", "Application Support", "Code", "User", "mcp.json")
	plan, err := PlanMCPAdd("demo", "http", "", nil, "https://example.com/mcp", nil, []model.MCPAgent{{Label: "VS Code", Path: path}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ApplyMCPAdd(context.Background(), plan, filepath.Join(dir, "snapshots"), nil); err != nil {
		t.Fatal(err)
	}
	body, _ := os.ReadFile(path)
	if !strings.Contains(string(body), `"servers"`) || !strings.Contains(string(body), `"type": "http"`) {
		t.Fatalf("vscode = %s", body)
	}
}
