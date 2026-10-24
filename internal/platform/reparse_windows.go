//go:build windows

package platform

import (
	"os"
	"syscall"
)

func isDirectoryLink(name string) (bool, error) {
	info, err := os.Lstat(name)
	if err != nil {
		return false, err
	}
	attrs, ok := info.Sys().(*syscall.Win32FileAttributeData)
	if !ok {
		return false, nil
	}
	return attrs.FileAttributes&0x400 != 0, nil // FILE_ATTRIBUTE_REPARSE_POINT
}
