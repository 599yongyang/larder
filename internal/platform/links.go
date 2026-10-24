package platform

import "fmt"

type LinkKind string

const (
	LinkSymlink  LinkKind = "symlink"
	LinkJunction LinkKind = "junction"
)

// CreateDirectoryLink is intentionally a platform adapter. Core never invokes
// a shell or mutates a user asset; M2 will decide when this adapter is allowed
// to run. Tests use only temporary directories.
func CreateDirectoryLink(target, link string) (LinkKind, error) {
	if target == "" || link == "" {
		return "", fmt.Errorf("target and link are required")
	}
	return createDirectoryLink(target, link)
}

func ResolveDirectoryLink(path string) (target string, kind LinkKind, err error) {
	return resolveDirectoryLink(path)
}
