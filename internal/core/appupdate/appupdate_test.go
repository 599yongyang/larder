package appupdate

import (
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestArtifactForSupportedPlatforms(t *testing.T) {
	mac, err := artifactFor("darwin", "arm64")
	if err != nil || mac.Filename != "Larder.dmg" {
		t.Fatalf("mac artifact = %#v, %v", mac, err)
	}
	win, err := artifactFor("windows", "amd64")
	if err != nil || win.Filename != "larder-amd64-installer.exe" {
		t.Fatalf("windows artifact = %#v, %v", win, err)
	}
	if _, err := artifactFor("linux", "amd64"); err == nil {
		t.Fatal("linux should be unsupported")
	}
}

func TestDownloadVerifiesChecksumAndOpensFile(t *testing.T) {
	payload := []byte("installer bytes")
	sum := sha256.Sum256(payload)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v0.0.5/checksums.txt":
			_, _ = w.Write([]byte(hex.EncodeToString(sum[:]) + "  Larder.dmg\n"))
		case "/v0.0.5/Larder.dmg":
			_, _ = w.Write(payload)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	dir := t.TempDir()
	var opened string
	opts := options{baseURL: server.URL, client: server.Client(), goos: "darwin", goarch: "arm64", destDir: dir, open: func(path string) error { opened = path; return nil }}
	path, err := download(t.Context(), "v0.0.5", opts)
	if err != nil {
		t.Fatal(err)
	}
	if opened != path {
		t.Fatalf("opened = %q, path = %q", opened, path)
	}
	raw, err := os.ReadFile(filepath.Join(dir, "Larder-0.0.5.dmg"))
	if err != nil {
		t.Fatal(err)
	}
	if string(raw) != string(payload) {
		t.Fatalf("downloaded = %q", raw)
	}
}

func TestDownloadRejectsChecksumMismatch(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v0.0.5/checksums.txt" {
			_, _ = w.Write([]byte(hex.EncodeToString(make([]byte, 32)) + "  Larder.dmg\n"))
			return
		}
		_, _ = w.Write([]byte("wrong"))
	}))
	defer server.Close()
	opts := options{baseURL: server.URL, client: server.Client(), goos: "darwin", goarch: "arm64", destDir: t.TempDir(), open: func(string) error { return nil }}
	if _, err := download(t.Context(), "0.0.5", opts); err == nil {
		t.Fatal("expected checksum failure")
	}
}
