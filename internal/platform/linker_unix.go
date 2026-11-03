//go:build !windows

package platform

import "os"

// Symlink creates a symbolic link. Unix has no privilege requirement for it.
func (OSLinker) Symlink(source, target string) error { return os.Symlink(source, target) }
