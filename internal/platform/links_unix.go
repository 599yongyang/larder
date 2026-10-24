//go:build !windows

package platform

import (
	"os"
	"path/filepath"
)

func createDirectoryLink(target, link string) (LinkKind, error) {
	if _, err := os.Stat(target); err != nil {
		return "", err
	}
	if err := os.MkdirAll(filepath.Dir(link), 0o755); err != nil {
		return "", err
	}
	if err := os.Symlink(target, link); err != nil {
		return "", err
	}
	return LinkSymlink, nil
}

func resolveDirectoryLink(path string) (string, LinkKind, error) {
	target, err := os.Readlink(path)
	if err != nil {
		return "", "", err
	}
	if !filepath.IsAbs(target) {
		target = filepath.Join(filepath.Dir(path), target)
	}
	return filepath.Clean(target), LinkSymlink, nil
}
