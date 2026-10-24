package mcp

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeConfig(t *testing.T, dir, name, body string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestScanJSONAndTOML(t *testing.T) {
	dir := t.TempDir()
	jsonPath := writeConfig(t, dir, "mcp.json", `{"mcpServers":{"demo":{"command":"npx","args":["-y","demo-mcp"],"env":{"TOKEN":"secret-value"}},"off":{"command":"npx","args":["off"],"enabled":false}}}`)
	tomlPath := writeConfig(t, dir, "config.toml", "[mcp_servers.codex]\ncommand = \"/does/not/exist\"\nargs = [\"--stdio\"]\n[mcp_servers.codex.env]\nCODEX_KEY = \"hidden-value\"\n")
	servers := scanConfigs(context.Background(), []configSpec{
		{label: "A", path: jsonPath, format: "json"},
		{label: "B", path: tomlPath, format: "codex"},
	})
	if len(servers) != 3 {
		t.Fatalf("servers = %#v", servers)
	}
	byName := map[string]int{}
	for i, server := range servers {
		byName[server.Name] = i
		if strings.Contains(strings.Join(server.EnvKeys, ","), "secret") {
			t.Fatalf("env values must not be returned: %#v", server)
		}
	}
	codex := servers[byName["codex"]]
	if codex.Status != "missing" || len(codex.EnvKeys) != 1 || codex.EnvKeys[0] != "CODEX_KEY" {
		t.Fatalf("codex = %#v", codex)
	}
	demo := servers[byName["demo"]]
	if demo.Status != "ok" || len(demo.EnvKeys) != 1 || demo.EnvKeys[0] != "TOKEN" {
		t.Fatalf("demo = %#v", demo)
	}
	if servers[byName["off"]].Status != "disabled" {
		t.Fatalf("off = %#v", servers[byName["off"]])
	}
}

func TestScanDeduplicatesAcrossAgents(t *testing.T) {
	dir := t.TempDir()
	a := writeConfig(t, dir, "a.json", `{"mcpServers":{"shared":{"command":"npx","args":["-y","shared"],"env":{"A":"1"}}}}`)
	b := writeConfig(t, dir, "b.json", `{"mcpServers":{"shared":{"command":"npx","args":["-y","shared"],"env":{"B":"2"}}}}`)
	servers := scanConfigs(context.Background(), []configSpec{
		{label: "A", path: a, format: "json"},
		{label: "B", path: b, format: "json"},
	})
	if len(servers) != 1 || len(servers[0].Agents) != 2 {
		t.Fatalf("servers = %#v", servers)
	}
	if strings.Join(servers[0].EnvKeys, ",") != "A,B" {
		t.Fatalf("env keys = %#v", servers[0].EnvKeys)
	}
}

func TestProjectConfigsAreScoped(t *testing.T) {
	project := t.TempDir()
	if err := os.WriteFile(filepath.Join(project, ".mcp.json"), []byte(`{"mcpServers":{"project-only":{"command":"npx","args":["-y","demo"]}}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	servers := scanConfigs(context.Background(), projectConfigs(project))
	if len(servers) != 1 {
		t.Fatalf("servers = %d", len(servers))
	}
	if servers[0].Name != "project-only" || servers[0].Agents[0].Scope != project {
		t.Fatalf("server = %+v", servers[0])
	}
}
