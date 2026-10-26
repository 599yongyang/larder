package ops

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSnapshotLockfileRestoresRolledBackInstall(t *testing.T) {
	root := t.TempDir()
	snapshotDir := filepath.Join(root, "snapshot")
	if err := os.MkdirAll(snapshotDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(snapshotDir, "manifest.json"), []byte(`{"id":"snapshot","targets":[]}`), 0o600); err != nil {
		t.Fatal(err)
	}
	lockfile := filepath.Join(root, "lock.json")
	if err := os.WriteFile(lockfile, []byte("before"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := SnapshotLockfile(snapshotDir, lockfile); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(lockfile, []byte("after"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := RollbackSnapshot(snapshotDir, nil, nil); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(lockfile)
	if err != nil {
		t.Fatal(err)
	}
	if string(raw) != "before" {
		t.Fatalf("lockfile = %q, want before", raw)
	}
}
