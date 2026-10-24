package ops_test

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"os"
	"path/filepath"
	"testing"

	"larder/internal/core/model"
	"larder/internal/core/ops"
	"larder/internal/platform"
)

type fakeFetcher struct{ data []byte }

func (f fakeFetcher) Fetch(context.Context, string) ([]byte, error) { return f.data, nil }

func TestSkillUpdateSnapshotsAndRollsBack(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, "skills", "alpha")
	if err := os.MkdirAll(target, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(target, "SKILL.md"), []byte("old\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	skill := model.Skill{ID: "s1", Name: "alpha", Path: target, Locked: true, SourceType: "github", Source: "owner/repo", SourceURL: "https://github.com/owner/repo.git", SkillPath: "skills/alpha/SKILL.md"}
	plan, err := ops.PlanSkillUpdate(skill, "owner/repo")
	if err != nil {
		t.Fatal(err)
	}
	op, err := ops.ApplySkillUpdate(context.Background(), plan, filepath.Join(root, "snapshots"), fakeFetcher{data: tarWithFile(t, "repo-HEAD/skills/alpha/SKILL.md", "new\n")}, platform.OSFileSystem{})
	if err != nil {
		t.Fatal(err)
	}
	content, err := os.ReadFile(filepath.Join(target, "SKILL.md"))
	if err != nil || string(content) != "new\n" {
		t.Fatalf("updated = %q err=%v", content, err)
	}
	if err := ops.RollbackSnapshot(op.SnapshotPath, platform.OSFileSystem{}, platform.OSLinker{}); err != nil {
		t.Fatal(err)
	}
	content, err = os.ReadFile(filepath.Join(target, "SKILL.md"))
	if err != nil || string(content) != "old\n" {
		t.Fatalf("rolled back = %q err=%v", content, err)
	}
}

func tarWithFile(t *testing.T, name, content string) []byte {
	t.Helper()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	if err := tw.WriteHeader(&tar.Header{Name: name, Mode: 0o644, Size: int64(len(content)), Typeflag: tar.TypeReg}); err != nil {
		t.Fatal(err)
	}
	if _, err := tw.Write([]byte(content)); err != nil {
		t.Fatal(err)
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}
