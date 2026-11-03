package ops

import (
	"archive/zip"
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"larder/internal/platform"
)

func TestApplySkillUpdateWithResolutionsMergesZip(t *testing.T) {
	var archive bytes.Buffer
	writer := zip.NewWriter(&archive)
	for name, body := range map[string]string{"SKILL.md": "# new version", "new.md": "new"} {
		file, _ := writer.Create(name)
		_, _ = file.Write([]byte(body))
	}
	_ = writer.Close()
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) { _, _ = writer.Write(archive.Bytes()) }))
	defer server.Close()
	dir := t.TempDir()
	target := filepath.Join(dir, "skills", "demo")
	if err := os.MkdirAll(target, 0o755); err != nil { t.Fatal(err) }
	for name, body := range map[string]string{"SKILL.md": "# old version", "local.md": "keep me"} {
		if err := os.WriteFile(filepath.Join(target, name), []byte(body), 0o644); err != nil { t.Fatal(err) }
	}
	plan, err := PlanUpdateSkillArchive(server.URL, "demo", target)
	if err != nil { t.Fatal(err) }
	op, err := ApplySkillUpdateWithResolutions(context.Background(), plan, filepath.Join(dir, "snapshots"), platform.HTTPFetcher{}, platform.OSFileSystem{}, nil, nil)
	if err != nil { t.Fatal(err) }
	if op.Status != "success" { t.Fatalf("status = %s", op.Status) }
	if body, _ := os.ReadFile(filepath.Join(target, "local.md")); string(body) != "keep me" { t.Fatalf("local file = %q", body) }
	if _, err := os.Stat(filepath.Join(target, "new.md")); err != nil { t.Fatalf("new upstream file missing: %v", err) }
}
