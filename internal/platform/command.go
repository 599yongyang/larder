package platform

import (
	"bytes"
	"context"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
)

// CommandRunner is the only place core update checks may execute a process.
// It never goes through a shell and callers must pass fixed argv templates.
type CommandRunner interface {
	Find(name string, dirs []string) (string, bool)
	Run(ctx context.Context, path string, args []string, env []string) (stdout, stderr []byte, err error)
}

type OSCommandRunner struct{}

func (OSCommandRunner) Find(name string, dirs []string) (string, bool) {
	for _, dir := range dirs {
		path := filepath.Join(dir, name)
		if runtime.GOOS == "windows" {
			for _, ext := range []string{".exe", ".cmd", ".bat", ".ps1"} {
				if info, err := os.Stat(path + ext); err == nil && !info.IsDir() {
					return path + ext, true
				}
			}
			continue
		}
		info, err := os.Lstat(path)
		if err != nil || info.IsDir() {
			continue
		}
		if info.Mode()&fs.ModeSymlink != 0 {
			if target, err := os.Stat(path); err == nil && !target.IsDir() {
				return path, true
			}
			continue
		}
		if info.Mode().Perm()&0o111 != 0 {
			return path, true
		}
	}
	return "", false
}

func (OSCommandRunner) Run(ctx context.Context, path string, args []string, env []string) ([]byte, []byte, error) {
	cmd := exec.CommandContext(ctx, path, args...)
	if len(env) > 0 {
		cmd.Env = append(os.Environ(), env...)
	}
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	return stdout.Bytes(), stderr.Bytes(), err
}
