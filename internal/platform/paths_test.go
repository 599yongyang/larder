package platform

import (
	"os"
	"path/filepath"
	"testing"
)

func TestPathDirsIgnoresAmbientPATH(t *testing.T) {
	home := t.TempDir()
	extra := filepath.Join(t.TempDir(), "project-tools")
	if err := os.MkdirAll(extra, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", extra+string(os.PathListSeparator)+"/usr/bin:/bin")
	got := (LocalPaths{}).PathDirs(home)
	for _, dir := range got {
		if filepath.Clean(dir) == filepath.Clean(extra) {
			t.Fatalf("ambient PATH leaked into CLI inventory: %s", extra)
		}
	}
}
