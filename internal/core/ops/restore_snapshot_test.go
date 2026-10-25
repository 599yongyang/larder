package ops_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"larder/internal/core/ops"
	"larder/internal/platform"
)

func copyTestTree(t *testing.T, src, dst string) {
	t.Helper()
	if err := os.MkdirAll(dst, 0o755); err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(src)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		from, to := filepath.Join(src, entry.Name()), filepath.Join(dst, entry.Name())
		if entry.IsDir() {
			copyTestTree(t, from, to)
			continue
		}
		data, err := os.ReadFile(from)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(to, data, 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

func TestCreateRestoreSnapshotCapturesCurrentFiles(t *testing.T) {
	root := t.TempDir()
	sourceBackup := filepath.Join(root, "backup")
	current := filepath.Join(root, "skill")
	if err := os.MkdirAll(sourceBackup, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(current, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sourceBackup, "SKILL.md"), []byte("old\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(current, "SKILL.md"), []byte("current\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	sourceSnapshot := filepath.Join(root, "source-snapshot")
	if err := os.MkdirAll(sourceSnapshot, 0o755); err != nil {
		t.Fatal(err)
	}
	manifest := ops.SnapshotManifest{ID: "source", CreatedAt: time.Now(), Targets: []ops.SnapshotTarget{{Target: current, Action: "uninstall_skill", Exists: true, BackupPath: sourceBackup}}}
	raw, _ := json.Marshal(manifest)
	if err := os.WriteFile(filepath.Join(sourceSnapshot, "manifest.json"), raw, 0o644); err != nil {
		t.Fatal(err)
	}
	safety, err := ops.CreateRestoreSnapshot(sourceSnapshot, filepath.Join(root, "snapshots"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(current, "SKILL.md"), []byte("changed\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := ops.RollbackSnapshot(safety, platform.OSFileSystem{}, platform.OSLinker{}); err != nil {
		t.Fatal(err)
	}
	content, err := os.ReadFile(filepath.Join(current, "SKILL.md"))
	if err != nil || string(content) != "current\n" {
		t.Fatalf("safety restore = %q err=%v", content, err)
	}
}
