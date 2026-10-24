package instructions

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestScanFindsProjectAndGlobalFiles(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()
	if err := os.MkdirAll(filepath.Join(home, ".claude"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(home, ".claude", "CLAUDE.md"), []byte("global rules"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(project, "AGENTS.md"), []byte("project rules"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(project, ".cursor", "rules"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(project, ".cursor", "rules", "ui.mdc"), []byte("ui rules"), 0o600); err != nil {
		t.Fatal(err)
	}
	files := Scan(context.Background(), home, []string{project})
	if len(files) != 3 {
		t.Fatalf("files = %d: %+v", len(files), files)
	}
	scopes := map[string]int{}
	for _, file := range files {
		scopes[file.Scope]++
	}
	if scopes["global"] != 1 || scopes["project"] != 2 {
		t.Fatalf("scopes = %v", scopes)
	}
}
