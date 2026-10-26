// Package appupdate downloads a published Larder installer and verifies it
// against the checksum sidecar from the same GitHub release.
package appupdate

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"time"
)

const releaseBaseURL = "https://github.com/599yongyang/larder/releases/download"

var versionPattern = regexp.MustCompile(`^[0-9]+\.[0-9]+\.[0-9]+$`)

const maxInstallerSize = 200 << 20

// Artifact describes the installer for one running platform.
type Artifact struct {
	Filename string
	Checksum string
}

type options struct {
	baseURL string
	client  *http.Client
	goos    string
	goarch  string
	destDir string
	open    func(string) error
}

func defaultOptions() options {
	return options{
		baseURL: releaseBaseURL,
		client:  &http.Client{Timeout: 2 * time.Minute},
		goos:    runtime.GOOS,
		goarch:  runtime.GOARCH,
		open:    openPath,
	}
}

// Download fetches the published installer for version, verifies its SHA-256
// checksum and opens the downloaded file. It returns the local path.
func Download(ctx context.Context, version string) (string, error) {
	opts := defaultOptions()
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	opts.destDir = filepath.Join(home, "Downloads")
	return download(ctx, version, opts)
}

func download(ctx context.Context, version string, opts options) (string, error) {
	version = strings.TrimPrefix(strings.TrimSpace(version), "v")
	if !versionPattern.MatchString(version) {
		return "", errors.New("invalid app version")
	}
	artifact, err := artifactFor(opts.goos, opts.goarch)
	if err != nil {
		return "", err
	}
	checksums, err := fetchChecksums(ctx, opts.client, opts.baseURL, version)
	if err != nil {
		return "", err
	}
	expected, ok := checksums[artifact.Filename]
	if !ok || expected == "" {
		return "", fmt.Errorf("checksums file does not contain %s", artifact.Filename)
	}
	artifact.Checksum = expected
	return downloadAndOpen(ctx, version, artifact, opts)
}

func artifactFor(goos, goarch string) (Artifact, error) {
	switch goos {
	case "darwin":
		if goarch != "arm64" {
			return Artifact{}, errors.New("macOS updates are only available for arm64")
		}
		return Artifact{Filename: "Larder.dmg"}, nil
	case "windows":
		if goarch != "amd64" {
			return Artifact{}, errors.New("Windows updates are only available for amd64")
		}
		return Artifact{Filename: "larder-amd64-installer.exe"}, nil
	default:
		return Artifact{}, fmt.Errorf("app updates are not available on %s", goos)
	}
}

func fetchChecksums(ctx context.Context, client *http.Client, baseURL, version string) (map[string]string, error) {
	url := fmt.Sprintf("%s/v%s/checksums.txt", baseURL, version)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "larder-app-update")
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("download checksums: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("download checksums: HTTP %d", resp.StatusCode)
	}
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, err
	}
	return parseChecksums(string(raw))
}

func parseChecksums(content string) (map[string]string, error) {
	out := map[string]string{}
	for _, line := range strings.Split(strings.ReplaceAll(content, "\r\n", "\n"), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		sum := strings.ToLower(fields[0])
		if len(sum) != 64 {
			continue
		}
		if _, err := hex.DecodeString(sum); err != nil {
			continue
		}
		name := strings.TrimPrefix(fields[len(fields)-1], "*")
		out[name] = sum
	}
	if len(out) == 0 {
		return nil, errors.New("checksums file is empty or invalid")
	}
	return out, nil
}

func downloadAndOpen(ctx context.Context, version string, artifact Artifact, opts options) (string, error) {
	if err := os.MkdirAll(opts.destDir, 0o755); err != nil {
		return "", err
	}
	ext := filepath.Ext(artifact.Filename)
	base := strings.TrimSuffix(filepath.Base(artifact.Filename), ext)
	target := filepath.Join(opts.destDir, fmt.Sprintf("%s-%s%s", base, version, ext))
	tmp, err := os.CreateTemp(opts.destDir, ".larder-update-*.tmp")
	if err != nil {
		return "", err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	url := fmt.Sprintf("%s/v%s/%s", opts.baseURL, version, artifact.Filename)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		_ = tmp.Close()
		return "", err
	}
	req.Header.Set("User-Agent", "larder-app-update")
	resp, err := opts.client.Do(req)
	if err != nil {
		_ = tmp.Close()
		return "", fmt.Errorf("download installer: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		_ = tmp.Close()
		return "", fmt.Errorf("download installer: HTTP %d", resp.StatusCode)
	}
	if resp.ContentLength > maxInstallerSize {
		_ = tmp.Close()
		return "", errors.New("installer is too large")
	}
	hash := sha256.New()
	written, err := io.Copy(io.MultiWriter(tmp, hash), io.LimitReader(resp.Body, maxInstallerSize+1))
	if err != nil {
		_ = tmp.Close()
		return "", err
	}
	if written > maxInstallerSize {
		_ = tmp.Close()
		return "", errors.New("installer is too large")
	}
	if err := tmp.Close(); err != nil {
		return "", err
	}
	if got := hex.EncodeToString(hash.Sum(nil)); got != artifact.Checksum {
		return "", errors.New("download checksum mismatch")
	}
	if err := os.Rename(tmpName, target); err != nil {
		return "", err
	}
	if err := opts.open(target); err != nil {
		return target, fmt.Errorf("open downloaded installer: %w", err)
	}
	return target, nil
}

func openPath(path string) error {
	switch runtime.GOOS {
	case "darwin":
		return exec.Command("open", path).Start()
	case "windows":
		return exec.Command("explorer.exe", path).Start()
	default:
		return fmt.Errorf("opening installers is not supported on %s", runtime.GOOS)
	}
}
