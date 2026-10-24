//go:build !windows

package platform

import "os"

func readlink(name string) (string, error) { return os.Readlink(name) }
