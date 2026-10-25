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
	if err := os.WriteFile(filepath.Join(target, "my-template.md"), []byte("mine\n"), 0o644); err != nil {
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
	if content, err := os.ReadFile(filepath.Join(target, "my-template.md")); err != nil || string(content) != "mine\n" {
		t.Fatalf("local file was not preserved: %q err=%v", content, err)
	}
	if baseline, err := ops.LoadBaselineManifest(op.SnapshotPath); err != nil || baseline["SKILL.md"] == "" {
		t.Fatalf("baseline manifest missing: %#v err=%v", baseline, err)
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

func TestSkillUpdatePreservesLocallyModifiedManagedFile(t *testing.T) {
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
	plan, err := ops.PlanSkillUpdate(skill, "owner/repo")
	if err != nil {
		t.Fatal(err)
	}
	_, err = ops.ApplySkillUpdate(context.Background(), plan, filepath.Join(root, "snapshots"), fakeFetcher{data: tarWithFile(t, "repo-HEAD/skills/alpha/SKILL.md", "upstream\n")}, platform.OSFileSystem{}, baseline)
	if err != nil {
		t.Fatal(err)
	}
	content, err := os.ReadFile(filepath.Join(target, "SKILL.md"))
	if err != nil || string(content) != "local\n" {
		t.Fatalf("local edit was overwritten: %q err=%v", content, err)
	}
}

func TestSkillUpdateConflictResolutionUpstreamAndBoth(t *testing.T) {
	for _, resolution := range []string{"upstream", "both"} {
		t.Run(resolution, func(t *testing.T) {
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
			plan, err := ops.PlanSkillUpdate(skill, "owner/repo")
			if err != nil {
				t.Fatal(err)
			}
			_, err = ops.ApplySkillUpdateWithResolutions(context.Background(), plan, filepath.Join(root, "snapshots"), fakeFetcher{data: tarWithFile(t, "repo-HEAD/skills/alpha/SKILL.md", "upstream\n")}, platform.OSFileSystem{}, baseline, map[string]string{"SKILL.md": resolution})
			if err != nil {
				t.Fatal(err)
			}
			content, _ := os.ReadFile(filepath.Join(target, "SKILL.md"))
			if resolution == "upstream" && string(content) != "upstream\n" {
				t.Fatalf("upstream = %q", content)
			}
			if resolution == "both" {
				if string(content) != "local\n" {
					t.Fatalf("local = %q", content)
				}
				extra, readErr := os.ReadFile(filepath.Join(target, "SKILL.md.upstream"))
				if readErr != nil || string(extra) != "upstream\n" {
					t.Fatalf("both upstream = %q err=%v", extra, readErr)
				}
			}
		})
	}
}

func TestSkillUpdateCanRemoveLocalOnlyFileWhenSyncingOfficial(t *testing.T) {
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
	if err := os.WriteFile(filepath.Join(target, "SKILL.md"), []byte("old\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(target, "local.md"), []byte("mine\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	baseline, err := ops.FileHashes(base)
	if err != nil {
		t.Fatal(err)
	}
	skill := model.Skill{ID: "s1", Name: "alpha", Path: target, Locked: true, SourceType: "github", Source: "owner/repo", SourceURL: "https://github.com/owner/repo.git", SkillPath: "skills/alpha/SKILL.md"}
	plan, err := ops.PlanSkillUpdate(skill, "owner/repo")
	if err != nil {
		t.Fatal(err)
	}
	_, err = ops.ApplySkillUpdateWithResolutions(context.Background(), plan, filepath.Join(root, "snapshots"), fakeFetcher{data: tarWithFile(t, "repo-HEAD/skills/alpha/SKILL.md", "upstream\n")}, platform.OSFileSystem{}, baseline, map[string]string{"local.md": "upstream"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(target, "local.md")); !os.IsNotExist(err) {
		t.Fatalf("local-only file was not removed: %v", err)
	}
}

func TestSkillUpdateNoBaselineCanKeepLocal(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, "alpha")
	if err := os.MkdirAll(target, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(target, "SKILL.md"), []byte("local\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	skill := model.Skill{ID: "s1", Name: "alpha", Path: target, Locked: true, SourceType: "github", Source: "owner/repo", SourceURL: "https://github.com/owner/repo.git", SkillPath: "skills/alpha/SKILL.md"}
	plan, err := ops.PlanSkillUpdate(skill, "owner/repo")
	if err != nil {
		t.Fatal(err)
	}
	_, err = ops.ApplySkillUpdateWithResolutions(context.Background(), plan, filepath.Join(root, "snapshots"), fakeFetcher{data: tarWithFile(t, "repo-HEAD/skills/alpha/SKILL.md", "upstream\n")}, platform.OSFileSystem{}, nil, map[string]string{"SKILL.md": "local"})
	if err != nil {
		t.Fatal(err)
	}
	content, err := os.ReadFile(filepath.Join(target, "SKILL.md"))
	if err != nil || string(content) != "local\n" {
		t.Fatalf("local = %q err=%v", content, err)
	}
}
