package platform

import (
	"context"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

// NewCommand and NewCommandContext wrap Windows batch shims (.cmd/.bat) in
// cmd.exe. CreateProcess cannot launch a batch file directly, but npm, npx and
// several Agent CLIs are installed as .cmd shims.
func NewCommand(name string, args ...string) *exec.Cmd {
	if runtime.GOOS == "windows" && isWindowsBatch(name) {
		wrapped := append([]string{"/d", "/s", "/c", name}, args...)
		return exec.Command("cmd.exe", wrapped...)
	}
	return exec.Command(name, args...)
}

func NewCommandContext(ctx context.Context, name string, args ...string) *exec.Cmd {
	if runtime.GOOS == "windows" && isWindowsBatch(name) {
		wrapped := append([]string{"/d", "/s", "/c", name}, args...)
		return exec.CommandContext(ctx, "cmd.exe", wrapped...)
	}
	return exec.CommandContext(ctx, name, args...)
}

func isWindowsBatch(name string) bool {
	ext := strings.ToLower(filepath.Ext(name))
	return ext == ".cmd" || ext == ".bat"
}
