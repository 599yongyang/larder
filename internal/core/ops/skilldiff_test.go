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
)

func tarWithFiles(t *testing.T, files map[string]string) []byte {
	t.Helper()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	for name, content := range files {
		if err := tw.WriteHeader(&tar.Header{Name: name, Mode: 0o644, Size: int64(len(content)), Typeflag: tar.TypeReg}); err != nil {
			t.Fatal(err)
		}
		if _, err := tw.Write([]byte(content)); err != nil {
			t.Fatal(err)
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestBuildSkillUpdateDiffClassifiesFiles(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, "alpha")
	if err := os.MkdirAll(target, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(target, "SKILL.md"), []byte("old\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(target, "local.md"), []byte("mine\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	skill := model.Skill{ID: "s1", Name: "alpha", Path: target, Locked: true, SourceType: "github", Source: "owner/repo", SourceURL: "https://github.com/owner/repo.git", SkillPath: "skills/alpha/SKILL.md"}
	data := tarWithFiles(t, map[string]string{
		"repo-HEAD/skills/alpha/SKILL.md": "new\n",
		"repo-HEAD/skills/alpha/new.md":   "upstream\n",
	})
	diff, err := ops.BuildSkillUpdateDiff(context.Background(), skill, "owner/repo", fakeFetcher{data: data}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if diff.Added != 1 || diff.Modified != 1 || diff.LocalOnly != 1 {
		t.Fatalf("unexpected diff: %+v", diff)
	}
	status := map[string]string{}
	for _, file := range diff.Files {
		status[file.Path] = file.Status
	}
	if status["SKILL.md"] != "modified" || status["new.md"] != "added" || status["local.md"] != "local" {
		t.Fatalf("unexpected statuses: %#v", status)
	}
}

func TestBuildSkillUpdateDiffUsesBaselineForConflict(t *testing.T) {
	root := t.TempDir()
	base := filepath.Join(root, "base")
	target := filepath.Join(root, "alpha")
	if err := os.MkdirAll(base, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(target, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(base, "SKILL.md"), []byte("old\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(target, "SKILL.md"), []byte("local\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	baseline, err := ops.FileHashes(base)
	if err != nil {
		t.Fatal(err)
	}
	skill := model.Skill{ID: "s1", Name: "alpha", Path: target, Locked: true, SourceType: "github", Source: "owner/repo", SourceURL: "https://github.com/owner/repo.git", SkillPath: "skills/alpha/SKILL.md"}
	data := tarWithFiles(t, map[string]string{"repo-HEAD/skills/alpha/SKILL.md": "upstream\n"})
	diff, err := ops.BuildSkillUpdateDiff(context.Background(), skill, "owner/repo", fakeFetcher{data: data}, baseline)
	if err != nil {
		t.Fatal(err)
	}
	if !diff.HasBaseline || diff.Conflicted != 1 || len(diff.Files) != 1 || diff.Files[0].Status != "conflict" || diff.Files[0].LocalChanged == false || diff.Files[0].UpstreamChanged == false {
		t.Fatalf("unexpected 3-way diff: %#v", diff)
	}
	if len(diff.Files[0].Lines) == 0 {
		t.Fatal("expected line diff for conflict")
	}
}
