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

func TestPathDirsIncludesNVMNodeBins(t *testing.T) {
	home := t.TempDir()
	bin := filepath.Join(home, ".nvm", "versions", "node", "v22.0.0", "bin")
	if err := os.MkdirAll(bin, 0o755); err != nil {
		t.Fatal(err)
	}
	got := (LocalPaths{}).PathDirs(home)
	for _, dir := range got {
		if filepath.Clean(dir) == filepath.Clean(bin) {
			return
		}
	}
	t.Fatalf("nvm bin not discovered: %v", got)
}

func TestNodeVersionOrdering(t *testing.T) {
	if nodeVersionAt("/home/u/.nvm/versions/node/v22.1.0/bin") <= nodeVersionAt("/home/u/.nvm/versions/node/v22.0.1/bin") {
		t.Fatal("major/minor/patch ordering is wrong")
	}
}
