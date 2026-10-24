package clis_test

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"larder/internal/core/clis"
	"larder/internal/platform"
)

func TestScannerFindsOwnersAndDuplicates(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("fixture uses Unix executable names without PATHEXT")
	}
	home := t.TempDir()
	brewDir := filepath.Join(home, "homebrew", "bin")
	npmDir := filepath.Join(home, "npm", "bin")
	for _, dir := range []string{brewDir, npmDir} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	writeExecutable(t, filepath.Join(brewDir, "jq"))
	writeExecutable(t, filepath.Join(npmDir, "jq"))
	writeExecutable(t, filepath.Join(brewDir, "rg"))

	result, issues := clis.NewScanner(platform.OSFileSystem{}).Scan(context.Background(), clis.Options{Dirs: []string{brewDir, npmDir}, Home: home})
	if len(issues) != 0 {
		t.Fatalf("issues = %#v", issues)
	}
	if len(result) != 2 {
		t.Fatalf("clis = %#v, want jq and rg", result)
	}
	var jq bool
	for _, item := range result {
		if item.Name == "jq" {
			jq = true
			if item.DuplicateCount != 1 || len(item.Paths) != 2 || item.Status != "duplicate" {
				t.Fatalf("jq = %#v", item)
			}
		}
	}
	if !jq {
		t.Fatal("jq not found")
	}
}

func writeExecutable(t *testing.T, path string) {
	t.Helper()
	if err := os.WriteFile(path, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatal(err)
	}
}

func TestScannerReadsVersionsFromMetadata(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink fixture is Unix-only")
	}
	home := t.TempDir()
	bin := filepath.Join(home, "homebrew", "bin")
	cellar := filepath.Join(home, "homebrew", "Cellar", "jq", "1.7.1", "bin")
	npmRoot := filepath.Join(home, ".npm-global", "lib", "node_modules", "foo")
	npmBin := filepath.Join(home, ".npm-global", "bin")
	for _, dir := range []string{bin, cellar, npmRoot, npmBin} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	writeExecutable(t, filepath.Join(cellar, "jq"))
	writeExecutable(t, filepath.Join(npmRoot, "foo"))
	if err := os.Symlink(filepath.Join(cellar, "jq"), filepath.Join(bin, "jq")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(npmRoot, "foo"), filepath.Join(npmBin, "foo")); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(npmRoot, "package.json"), []byte(`{"name":"foo","version":"2.0.0"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	result, _ := clis.NewScanner(platform.OSFileSystem{}).Scan(context.Background(), clis.Options{Dirs: []string{bin, npmBin}, Home: home})
	got := map[string]string{}
	for _, item := range result {
		got[item.Name] = item.Version
	}
	if got["jq"] != "1.7.1" {
		t.Fatalf("jq version = %q, want 1.7.1", got["jq"])
	}
	if got["foo"] != "2.0.0" {
		t.Fatalf("foo version = %q, want 2.0.0", got["foo"])
	}
}

func TestScannerReadsWingetPackageAndVersion(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink fixture is Unix-only")
	}
	home := t.TempDir()
	targetDir := filepath.Join(home, "WinGet", "Packages", "Git.Git_2.45.0", "cmd")
	linksDir := filepath.Join(home, "WinGet", "Links")
	for _, dir := range []string{targetDir, linksDir} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(targetDir, "git.exe"), []byte("x"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(targetDir, "git.exe"), filepath.Join(linksDir, "git.exe")); err != nil {
		t.Fatal(err)
	}
	result, _ := clis.NewScanner(platform.OSFileSystem{}).Scan(context.Background(), clis.Options{Dirs: []string{linksDir}, Home: home})
	if len(result) != 1 || result[0].Owner != "winget" || result[0].OwnerDetail != "Git.Git" || result[0].Version != "2.45.0" {
		t.Fatalf("result = %#v", result)
	}
}
