//go:build windows

package platform

import "os"

// Symlink creates a directory junction instead of a symbolic link.
//
// Windows only grants SE_CREATE_SYMBOLIC_LINK_PRIVILEGE to administrators and
// to users running with Developer Mode enabled. larder is a desktop program
// started by an ordinary user account, so os.Symlink fails there with "A
// required privilege is not held by the client". A junction needs no privilege
// at all, and it matches the way skillstash distributes skills into the agent
// directories.
func (OSLinker) Symlink(source, target string) error {
	if info, err := os.Stat(source); err == nil && info.IsDir() {
		_, err := createDirectoryLink(source, target)
		return err
	}
	return os.Symlink(source, target)
}
