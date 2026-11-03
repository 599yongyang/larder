package ops_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"larder/internal/core/ops"
	"larder/internal/platform"
)

func TestPlanLinkIsDryRunAndReportsConflicts(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "source", "alpha")
	if err := os.MkdirAll(source, 0o755); err != nil {
		t.Fatal(err)
	}
	conflict := filepath.Join(root, "agent", "alpha")
	if err := os.MkdirAll(conflict, 0o755); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(root, "agent2", "alpha")
	plan, err := ops.PlanLink(ops.LinkRequest{
		SkillID: "s1", SkillName: "alpha", SourcePath: source,
		Targets: []ops.LinkTarget{
			{RootID: "a", RootLabel: "Agent", Path: conflict},
			{RootID: "b", RootLabel: "Agent 2", Path: target},
		},
	}, platform.OSFileSystem{})
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Changes) != 2 || plan.Changes[0].Action != "conflict" || plan.Changes[1].Action != "create_symlink" {
		t.Fatalf("changes = %#v", plan.Changes)
	}
	if !plan.RequiresSnapshot || !plan.Executable {
		t.Fatalf("plan safety flags = %#v", plan)
	}
	if _, err := os.Stat(target); !os.IsNotExist(err) {
		t.Fatal("planner created a target")
	}
}

func TestApplyLinkPlanSnapshotsExecutesAndRollsBack(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "source", "alpha")
	if err := os.MkdirAll(source, 0o755); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(root, "agent", "alpha")
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		t.Fatal(err)
	}
	plan, err := ops.PlanLink(ops.LinkRequest{SkillID: "s1", SkillName: "alpha", SourcePath: source, Targets: []ops.LinkTarget{{RootID: "a", RootLabel: "Agent", Path: target}}}, platform.OSFileSystem{})
	if err != nil {
		t.Fatal(err)
	}
	snapshotRoot := filepath.Join(root, "snapshots")
	op, err := ops.ApplyLinkPlan(context.Background(), plan, snapshotRoot, platform.OSFileSystem{}, platform.OSLinker{})
	if err != nil {
		t.Fatal(err)
	}
	if op.Status != "success" || op.SnapshotPath == "" {
		t.Fatalf("op = %#v", op)
	}
	link, err := os.Readlink(target)
	if err != nil || filepath.Clean(link) != filepath.Clean(source) {
		t.Fatalf("link = %q err=%v", link, err)
	}
	if err := ops.RollbackSnapshot(op.SnapshotPath, platform.OSFileSystem{}, platform.OSLinker{}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Lstat(target); !os.IsNotExist(err) {
		t.Fatal("rollback did not remove the created link")
	}
}

func TestRepairBrokenLinkAndRollback(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "source", "alpha")
	if err := os.MkdirAll(source, 0o755); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(root, "agent", "alpha")
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(root, "gone"), target); err != nil {
		t.Fatal(err)
	}
	plan, err := ops.PlanLink(ops.LinkRequest{SkillID: "s1", SkillName: "alpha", SourcePath: source, Targets: []ops.LinkTarget{{RootID: "a", RootLabel: "Agent", Path: target}}}, platform.OSFileSystem{})
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Changes) != 1 || plan.Changes[0].Action != "replace_link" {
		t.Fatalf("plan changes = %#v", plan.Changes)
	}
	op, err := ops.ApplyLinkPlan(context.Background(), plan, filepath.Join(root, "snapshots"), platform.OSFileSystem{}, platform.OSLinker{})
	if err != nil {
		t.Fatal(err)
	}
	if op.Status != "success" {
		t.Fatalf("op = %#v", op)
	}
	link, err := os.Readlink(target)
	if err != nil || filepath.Clean(link) != filepath.Clean(source) {
		t.Fatalf("link = %q err=%v", link, err)
	}
	if err := ops.RollbackSnapshot(op.SnapshotPath, platform.OSFileSystem{}, platform.OSLinker{}); err != nil {
		t.Fatal(err)
	}
	restored, err := os.Readlink(target)
	if err != nil || filepath.Clean(restored) != filepath.Join(root, "gone") {
		t.Fatalf("restored = %q err=%v", restored, err)
	}
}

func TestCopyToLinkSnapshotsAndRollsBack(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "source", "alpha")
	target := filepath.Join(root, "agent", "alpha")
	if err := os.MkdirAll(source, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(target, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(source, "SKILL.md"), []byte("source\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(target, "SKILL.md"), []byte("local copy\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	plan, err := ops.PlanCopyToLink(ops.LinkRequest{SkillID: "s1", SkillName: "alpha", SourcePath: source, Targets: []ops.LinkTarget{{RootID: "a", RootLabel: "Agent", Path: target}}}, platform.OSFileSystem{})
	if err != nil {
		t.Fatal(err)
	}
	if plan.Operation != "replace_copy" || plan.Changes[0].Action != "replace_copy" {
		t.Fatalf("plan = %#v", plan)
	}
	op, err := ops.ApplyLinkPlan(context.Background(), plan, filepath.Join(root, "snapshots"), platform.OSFileSystem{}, platform.OSLinker{})
	if err != nil {
		t.Fatal(err)
	}
	link, err := os.Readlink(target)
	if err != nil || filepath.Clean(link) != filepath.Clean(source) {
		t.Fatalf("link = %q err=%v", link, err)
	}
	if err := ops.RollbackSnapshot(op.SnapshotPath, platform.OSFileSystem{}, platform.OSLinker{}); err != nil {
		t.Fatal(err)
	}
	content, err := os.ReadFile(filepath.Join(target, "SKILL.md"))
	if err != nil || string(content) != "local copy\n" {
		t.Fatalf("restored copy = %q err=%v", content, err)
	}
}

func TestDisableLinkAndRollback(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "source", "alpha")
	target := filepath.Join(root, "agent", "alpha")
	if err := os.MkdirAll(source, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(source, target); err != nil {
		t.Fatal(err)
	}
	plan, err := ops.PlanDisableLink(ops.LinkRequest{SkillID: "s1", SkillName: "alpha", SourcePath: source, Targets: []ops.LinkTarget{{RootID: "a", RootLabel: "Agent", Path: target}}}, platform.OSFileSystem{})
	if err != nil {
		t.Fatal(err)
	}
	op, err := ops.ApplyLinkPlan(context.Background(), plan, filepath.Join(root, "snapshots"), platform.OSFileSystem{}, platform.OSLinker{})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Lstat(target); !os.IsNotExist(err) {
		t.Fatal("disable did not remove the link")
	}
	if err := ops.RollbackSnapshot(op.SnapshotPath, platform.OSFileSystem{}, platform.OSLinker{}); err != nil {
		t.Fatal(err)
	}
	link, err := os.Readlink(target)
	if err != nil || filepath.Clean(link) != filepath.Clean(source) {
		t.Fatalf("restored link = %q err=%v", link, err)
	}
}

func TestSyncCopyReplacesContentAndRollsBack(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "source", "alpha")
	target := filepath.Join(root, "agent", "alpha")
	for _, dir := range []string{source, target} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(source, "SKILL.md"), []byte("source\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(target, "SKILL.md"), []byte("old copy\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	plan, err := ops.PlanSyncCopy(ops.LinkRequest{SkillID: "s1", SkillName: "alpha", SourcePath: source, Targets: []ops.LinkTarget{{RootID: "a", RootLabel: "Agent", Path: target}}}, platform.OSFileSystem{})
	if err != nil {
		t.Fatal(err)
	}
	op, err := ops.ApplyLinkPlan(context.Background(), plan, filepath.Join(root, "snapshots"), platform.OSFileSystem{}, platform.OSLinker{})
	if err != nil {
		t.Fatal(err)
	}
	content, err := os.ReadFile(filepath.Join(target, "SKILL.md"))
	if err != nil || string(content) != "source\n" {
		t.Fatalf("synced content = %q err=%v", content, err)
	}
	if err := ops.RollbackSnapshot(op.SnapshotPath, platform.OSFileSystem{}, platform.OSLinker{}); err != nil {
		t.Fatal(err)
	}
	content, err = os.ReadFile(filepath.Join(target, "SKILL.md"))
	if err != nil || string(content) != "old copy\n" {
		t.Fatalf("restored content = %q err=%v", content, err)
	}
}
