package ops

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMCPDetachAttachJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "mcp.json")
	if err := os.WriteFile(path, []byte(`{"mcpServers":{"demo":{"command":"npx","args":["demo"],"env":{"TOKEN":"secret"}}}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	detached, _, err := DetachMCPEntry(context.Background(), path, "demo", "Cursor", filepath.Join(dir, "snapshots"))
	if err != nil {
		t.Fatal(err)
	}
	body, _ := os.ReadFile(path)
	if strings.Contains(string(body), "demo") {
		t.Fatalf("not removed: %s", body)
	}
	if !strings.Contains(detached.Raw, "TOKEN") {
		t.Fatalf("raw = %s", detached.Raw)
	}
	if _, err := AttachMCPEntry(context.Background(), detached, filepath.Join(dir, "snapshots")); err != nil {
		t.Fatal(err)
	}
	body, _ = os.ReadFile(path)
	if !strings.Contains(string(body), "demo") || !strings.Contains(string(body), "TOKEN") {
		t.Fatalf("not restored: %s", body)
	}
}

func TestMCPDetachAttachTOML(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	initial := "[mcp_servers.demo]\ncommand = \"npx\"\nargs = [\"demo\"]\n\n[mcp_servers.demo.env]\nTOKEN = \"secret\"\n"
	if err := os.WriteFile(path, []byte(initial), 0o600); err != nil {
		t.Fatal(err)
	}
	detached, _, err := DetachMCPEntry(context.Background(), path, "demo", "Codex", filepath.Join(dir, "snapshots"))
	if err != nil {
		t.Fatal(err)
	}
	body, _ := os.ReadFile(path)
	if strings.Contains(string(body), "[mcp_servers.demo]") {
		t.Fatalf("not removed: %s", body)
	}
	if _, err := AttachMCPEntry(context.Background(), detached, filepath.Join(dir, "snapshots")); err != nil {
		t.Fatal(err)
	}
	body, _ = os.ReadFile(path)
	if !strings.Contains(string(body), "[mcp_servers.demo]") || !strings.Contains(string(body), "TOKEN") {
		t.Fatalf("not restored: %s", body)
	}
}
