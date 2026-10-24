//go:build windows

package platform

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

func createDirectoryLink(target, link string) (LinkKind, error) {
	if info, err := os.Stat(target); err != nil || !info.IsDir() {
		if err != nil {
			return "", err
		}
		return "", fmt.Errorf("target is not a directory: %s", target)
	}
	if err := os.MkdirAll(filepath.Dir(link), 0o755); err != nil {
		return "", err
	}
	// mklink is a cmd.exe built-in. Pass each argument separately so paths are
	// never interpolated into a shell command string.
	cmd := exec.Command("cmd.exe", "/d", "/c", "mklink", "/J", link, target)
	if output, err := cmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("mklink /J failed: %w: %s", err, string(output))
	}
	return LinkJunction, nil
}

func resolveDirectoryLink(path string) (string, LinkKind, error) {
	target, err := filepath.EvalSymlinks(path)
	if err != nil {
		return "", "", err
	}
	return filepath.Clean(target), LinkJunction, nil
}
