package clis

import (
	"context"
	"os/exec"
	"regexp"
	"time"
)

var versionOutputPattern = regexp.MustCompile(`(?:^|[^0-9])v?([0-9]+\.[0-9]+\.[0-9]+)(?:[^0-9]|$)`)

// ParseVersionOutput finds a semver in a CLI --version response.
func ParseVersionOutput(output string) string {
	if match := versionOutputPattern.FindStringSubmatch(output); len(match) == 2 {
		return match[1]
	}
	return ""
}

// ProbeVersion runs a CLI with --version, as a last resort when package
// metadata and standalone release paths cannot provide a version. It uses a
// short timeout, no stdin and disables known auto-update side effects.
func ProbeVersion(ctx context.Context, path string) (string, error) {
	probeCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	cmd := exec.CommandContext(probeCtx, path, "--version")
	cmd.Stdin = nil
	cmd.Env = append(cmd.Environ(), "NO_COLOR=1", "CLINE_NO_AUTO_UPDATE=1")
	out, err := cmd.CombinedOutput()
	version := ParseVersionOutput(string(out))
	if version != "" {
		return version, nil
	}
	return "", err
}
