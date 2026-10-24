package platform_test

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"larder/internal/platform"
)

func TestCreateAndResolveDirectoryLink(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, "target")
	link := filepath.Join(root, "links", "skill")
	if err := os.MkdirAll(target, 0o755); err != nil {
		t.Fatal(err)
	}
	kind, err := platform.CreateDirectoryLink(target, link)
	if err != nil {
		t.Fatal(err)
	}
	want := platform.LinkSymlink
	if runtime.GOOS == "windows" {
		want = platform.LinkJunction
	}
	if kind != want {
		t.Fatalf("link kind = %q, want %q", kind, want)
	}
	resolved, resolvedKind, err := platform.ResolveDirectoryLink(link)
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Clean(resolved) != filepath.Clean(target) {
		t.Fatalf("resolved target = %q, want %q", resolved, target)
	}
	if resolvedKind != want {
		t.Fatalf("resolved kind = %q, want %q", resolvedKind, want)
	}
}
