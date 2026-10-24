//go:build windows

package platform

import (
	"errors"
	"os"
	"path/filepath"
	"syscall"
)

func readlink(name string) (string, error) {
	info, err := os.Lstat(name)
	if err != nil {
		return "", err
	}
	attrs, ok := info.Sys().(*syscall.Win32FileAttributeData)
	if !ok || attrs.FileAttributes&0x400 == 0 { // FILE_ATTRIBUTE_REPARSE_POINT
		return "", errors.New("path is not a Windows reparse point")
	}
	if target, err := os.Readlink(name); err == nil {
		return target, nil
	}
	// Directory junctions are reparse points. EvalSymlinks follows both
	// junctions and symbolic links when os.Readlink cannot expose the target.
	return filepath.EvalSymlinks(name)
}
