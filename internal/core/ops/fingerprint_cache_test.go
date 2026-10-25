package ops

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestFingerprintCacheInvalidatesChangedFiles(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "SKILL.md")
	if err := os.WriteFile(path, []byte("old\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	first, err := fingerprintTree(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("new\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	future := time.Now().Add(time.Second)
	if err := os.Chtimes(path, future, future); err != nil {
		t.Fatal(err)
	}
	second, err := fingerprintTree(root)
	if err != nil {
		t.Fatal(err)
	}
	if first["SKILL.md"].hash == second["SKILL.md"].hash {
		t.Fatal("fingerprint cache did not invalidate after file change")
	}
}
