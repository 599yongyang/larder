package ops

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"larder/internal/core/model"
)

func TestMCPUpdateJSONPreservesEnv(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "mcp.json")
	if err := os.WriteFile(path, []byte(`{"mcpServers":{"old":{"command":"npx","args":["a"],"env":{"TOKEN":"secret"}}}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	server := model.MCPServer{ID: "mcp:test", Name: "old", Agents: []model.MCPAgent{{Label: "Cursor", Path: path}}}
	plan, err := PlanMCPUpdate(server, "new", "node", []string{"server.js"}, "", nil, nil, []string{path})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ApplyMCPUpdate(context.Background(), plan, filepath.Join(dir, "snapshots"), nil); err != nil {
		t.Fatal(err)
	}
	body, _ := os.ReadFile(path)
	text := string(body)
	if !strings.Contains(text, `"new"`) || !strings.Contains(text, `"node"`) || !strings.Contains(text, `"TOKEN": "secret"`) {
		t.Fatalf("json = %s", text)
	}
}

func TestMCPUpdateTOMLPreservesEnv(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	initial := "[mcp_servers.old]\ncommand = \"npx\"\nargs = [\"a\"]\n\n[mcp_servers.old.env]\nTOKEN = \"secret\"\n"
	if err := os.WriteFile(path, []byte(initial), 0o600); err != nil {
		t.Fatal(err)
	}
	server := model.MCPServer{ID: "mcp:test", Name: "old", Agents: []model.MCPAgent{{Label: "Codex", Path: path}}}
	plan, err := PlanMCPUpdate(server, "new", "", nil, "https://example.com/mcp", nil, nil, []string{path})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ApplyMCPUpdate(context.Background(), plan, filepath.Join(dir, "snapshots"), nil); err != nil {
		t.Fatal(err)
	}
	body, _ := os.ReadFile(path)
	text := string(body)
	if !strings.Contains(text, `[mcp_servers.new]`) || !strings.Contains(text, `url = "https://example.com/mcp"`) || !strings.Contains(text, `[mcp_servers.new.env]`) || !strings.Contains(text, `TOKEN = "secret"`) {
		t.Fatalf("toml = %s", text)
	}
}

func TestMCPUpdateCreatesNewAgentConfig(t *testing.T) {
	dir := t.TempDir()
	existing := filepath.Join(dir, "existing.json")
	fresh := filepath.Join(dir, "fresh.json")
	if err := os.WriteFile(existing, []byte(`{"mcpServers":{"demo":{"command":"npx","args":["old"]}}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	server := model.MCPServer{ID: "mcp:demo", Name: "demo", Agents: []model.MCPAgent{{Label: "A", Path: existing}, {Label: "B", Path: fresh}}}
	plan, err := PlanMCPUpdate(server, "demo", "node", []string{"server.js"}, "", nil, nil, []string{existing, fresh})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ApplyMCPUpdate(context.Background(), plan, filepath.Join(dir, "snapshots"), nil); err != nil {
		t.Fatal(err)
	}
	body, err := os.ReadFile(fresh)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(body), `"demo"`) || !strings.Contains(string(body), `"node"`) {
		t.Fatalf("fresh = %s", body)
	}
}
