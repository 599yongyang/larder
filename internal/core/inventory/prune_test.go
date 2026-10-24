package inventory

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestPruneSnapshotsKeepsNewest(t *testing.T) {
	root := t.TempDir()
	for i := 0; i < 6; i++ {
		dir := filepath.Join(root, "snap-"+string(rune('a'+i)))
		if err := os.MkdirAll(dir, 0o700); err != nil {
			t.Fatal(err)
		}
		mod := time.Now().Add(time.Duration(i-10) * time.Hour)
		if err := os.Chtimes(dir, mod, mod); err != nil {
			t.Fatal(err)
		}
	}
	service := &Service{snapshotRoot: root}
	if err := service.PruneSnapshots(3); err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 3 {
		t.Fatalf("snapshots = %d, want 3", len(entries))
	}
}
