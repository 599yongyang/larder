package ops

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSnapshotFileRestoresCommandOnRollback(t *testing.T) {
	root := t.TempDir()
	snapshotDir := filepath.Join(root, "snapshot")
	if err := os.MkdirAll(snapshotDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(snapshotDir, "manifest.json"), []byte(`{"id":"snapshot","targets":[]}`), 0o600); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(root, "commands", "review.md")
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(target, []byte("before"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := SnapshotFile(snapshotDir, target); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(target, []byte("after"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := RollbackSnapshot(snapshotDir, nil, nil); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	if string(raw) != "before" {
		t.Fatalf("target = %q, want before", raw)
	}
}
