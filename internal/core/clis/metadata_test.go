package clis

import (
	"os"
	"path/filepath"
	"testing"
)

func TestStandaloneVersionFromReleasePath(t *testing.T) {
	version, detail := standaloneVersion("/Users/me/.codex/packages/standalone/releases/0.164.0-aarch64-apple-darwin/bin/codex")
	if version != "0.164.0" || detail != "standalone" {
		t.Fatalf("version=%q detail=%q", version, detail)
	}
	if version, _ := standaloneVersion("/usr/local/bin/codex"); version != "" {
		t.Fatalf("unexpected version %q", version)
	}
}

func TestStandaloneVersionFollowsCurrentSymlink(t *testing.T) {
	root := t.TempDir()
	release := filepath.Join(root, "releases", "0.164.0-aarch64-apple-darwin", "bin")
	if err := os.MkdirAll(release, 0o755); err != nil {
		t.Fatal(err)
	}
	binary := filepath.Join(release, "codex")
	if err := os.WriteFile(binary, []byte("x"), 0o755); err != nil {
		t.Fatal(err)
	}
	current := filepath.Join(root, "current")
	if err := os.Symlink(filepath.Join(root, "releases", "0.164.0-aarch64-apple-darwin"), current); err != nil {
		t.Skip(err)
	}
	version, detail := standaloneVersion(filepath.Join(current, "bin", "codex"))
	if version != "0.164.0" || detail != "standalone" {
		t.Fatalf("version=%q detail=%q", version, detail)
	}
}
