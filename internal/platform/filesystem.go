package platform

import (
	"io/fs"
	"os"
)

// FileSystem is the small read-only surface used by core scanners. Keeping it
// here makes fixture tests independent from the host filesystem and leaves a
// future Windows-specific implementation behind the same interface.
type FileSystem interface {
	ReadDir(name string) ([]os.DirEntry, error)
	Lstat(name string) (os.FileInfo, error)
	Stat(name string) (os.FileInfo, error)
	ReadFile(name string) ([]byte, error)
	Readlink(name string) (string, error)
	IsDirectoryLink(name string) (bool, error)
}

type OSFileSystem struct{}

func (OSFileSystem) ReadDir(name string) ([]os.DirEntry, error) { return os.ReadDir(name) }
func (OSFileSystem) Lstat(name string) (os.FileInfo, error)     { return os.Lstat(name) }
func (OSFileSystem) Stat(name string) (os.FileInfo, error)      { return os.Stat(name) }
func (OSFileSystem) ReadFile(name string) ([]byte, error)       { return os.ReadFile(name) }
func (OSFileSystem) Readlink(name string) (string, error)       { return readlink(name) }
func (OSFileSystem) IsDirectoryLink(name string) (bool, error)  { return isDirectoryLink(name) }

func IsPermissionError(err error) bool {
	return err != nil && (os.IsPermission(err) || err == fs.ErrPermission)
}
