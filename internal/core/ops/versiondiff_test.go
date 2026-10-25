package ops_test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"larder/internal/core/model"
	"larder/internal/core/ops"
)

func TestBuildVersionDiffComparesSnapshotWithCurrent(t *testing.T) {
	root := t.TempDir()
	oldDir := filepath.Join(root, "old")
	current := filepath.Join(root, "skill")
	snapshot := filepath.Join(root, "snapshot")
	for _, dir := range []string{oldDir, current, snapshot} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(oldDir, "SKILL.md"), []byte("old\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(oldDir, "removed.md"), []byte("gone\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(current, "SKILL.md"), []byte("new\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(current, "added.md"), []byte("added\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	manifest := ops.SnapshotManifest{ID: "snap", CreatedAt: time.Now(), Targets: []ops.SnapshotTarget{{Target: current, Action: "update_skill", BackupPath: oldDir, Exists: true}}}
	raw, _ := json.Marshal(manifest)
	if err := os.WriteFile(filepath.Join(snapshot, "manifest.json"), raw, 0o644); err != nil {
		t.Fatal(err)
	}
	diff, err := ops.BuildVersionDiff(context.Background(), model.Operation{Operation: "skill_update", SnapshotPath: snapshot, Changes: []string{"updated:" + current}}, model.Skill{ID: "s1", Name: "alpha", Path: current})
	if err != nil {
		t.Fatal(err)
	}
	if diff.Added != 1 || diff.Modified != 1 || len(diff.Files) != 3 {
		t.Fatalf("diff = %#v", diff)
	}
	status := map[string]string{}
	for _, file := range diff.Files {
		status[file.Path] = file.Status
	}
	if status["added.md"] != "added" || status["SKILL.md"] != "modified" || status["removed.md"] != "removed" {
		t.Fatalf("status = %#v", status)
	}
}
