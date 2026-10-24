//go:build !windows

package platform

import (
	"io/fs"
	"os"
)

func isDirectoryLink(name string) (bool, error) {
	info, err := os.Lstat(name)
	if err != nil {
		return false, err
	}
	return info.Mode()&fs.ModeSymlink != 0, nil
}
