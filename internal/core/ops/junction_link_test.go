package ops

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"

	"larder/internal/platform"
)

// makeJunction 用 mklink /J 创建目录联接。junction 不需要管理员权限，
// 与 skillstash 在各 agent 目录里的分发方式一致。
func makeJunction(t *testing.T, target, link string) {
	t.Helper()
	out, err := exec.Command("cmd.exe", "/d", "/c", "mklink", "/J", link, target).CombinedOutput()
	if err != nil {
		t.Fatalf("mklink /J 失败: %v %s", err, out)
	}
}

func newSkillDir(t *testing.T, base, name string) string {
	t.Helper()
	dir := filepath.Join(base, name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte("---\nname: "+name+"\n---\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

func linkReq(source, target string) LinkRequest {
	return LinkRequest{
		SkillID:    "skill-demo",
		SkillName:  "demo",
		SourcePath: source,
		Targets:    []LinkTarget{{RootID: "codebuddy", RootLabel: "CodeBuddy", Path: target}},
	}
}

// Windows 上 os.Lstat 把目录联接报成 ModeIrregular：既不带 ModeSymlink 位，
// 也不满足 IsDir。只看 ModeSymlink 的判断会把 junction 误认成「已存在但不是
// 链接」，于是 PlanLink 判 conflict、PlanDisableLink 直接报错。
func TestPlanLinkTreatsWindowsJunctionAsLinked(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("仅 Windows 适用")
	}
	base := t.TempDir()
	source := newSkillDir(t, base, "source")
	target := filepath.Join(base, "agent-skills", "demo")
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		t.Fatal(err)
	}
	makeJunction(t, source, target)

	plan, err := PlanLink(linkReq(source, target), platform.OSFileSystem{})
	if err != nil {
		t.Fatalf("PlanLink: %v", err)
	}
	if len(plan.Changes) != 1 {
		t.Fatalf("changes = %d, want 1", len(plan.Changes))
	}
	if plan.Changes[0].Action != "already_linked" {
		t.Fatalf("action = %q, want already_linked；reason = %q", plan.Changes[0].Action, plan.Changes[0].Reason)
	}
}

func TestPlanDisableLinkAcceptsWindowsJunction(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("仅 Windows 适用")
	}
	base := t.TempDir()
	source := newSkillDir(t, base, "source")
	target := filepath.Join(base, "agent-skills", "demo")
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		t.Fatal(err)
	}
	makeJunction(t, source, target)

	plan, err := PlanDisableLink(linkReq(source, target), platform.OSFileSystem{})
	if err != nil {
		t.Fatalf("PlanDisableLink 拒绝 junction: %v", err)
	}
	if len(plan.Changes) != 1 || plan.Changes[0].Action != "remove_link" {
		t.Fatalf("changes = %#v, want 一条 remove_link", plan.Changes)
	}
}

// 端到端：停用可见性应当只摘掉 junction，hub 实体必须毫发无损。
func TestApplyDisableLinkRemovesJunctionAndKeepsHubEntity(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("仅 Windows 适用")
	}
	base := t.TempDir()
	source := newSkillDir(t, base, "source")
	target := filepath.Join(base, "agent-skills", "demo")
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		t.Fatal(err)
	}
	makeJunction(t, source, target)

	plan, err := PlanDisableLink(linkReq(source, target), platform.OSFileSystem{})
	if err != nil {
		t.Fatalf("PlanDisableLink: %v", err)
	}
	op, err := ApplyLinkPlan(context.Background(), plan, filepath.Join(base, "snapshots"), platform.OSFileSystem{}, platform.OSLinker{})
	if err != nil {
		t.Fatalf("ApplyLinkPlan: %v", err)
	}
	if op.Status != "success" {
		t.Fatalf("status = %q, message = %q", op.Status, op.Message)
	}
	if _, err := os.Lstat(target); !os.IsNotExist(err) {
		t.Fatalf("junction 没被摘掉: %v", err)
	}
	if _, err := os.Stat(filepath.Join(source, "SKILL.md")); err != nil {
		t.Fatalf("hub 实体被误删: %v", err)
	}
}

// 快照必须记住 junction 的真实目标，否则回滚时 LinkTarget 为空、链接恢复不了。
func TestRollbackRestoresRemovedJunction(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("仅 Windows 适用")
	}
	base := t.TempDir()
	source := newSkillDir(t, base, "source")
	target := filepath.Join(base, "agent-skills", "demo")
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		t.Fatal(err)
	}
	makeJunction(t, source, target)

	plan, err := PlanDisableLink(linkReq(source, target), platform.OSFileSystem{})
	if err != nil {
		t.Fatalf("PlanDisableLink: %v", err)
	}
	op, err := ApplyLinkPlan(context.Background(), plan, filepath.Join(base, "snapshots"), platform.OSFileSystem{}, platform.OSLinker{})
	if err != nil {
		t.Fatalf("ApplyLinkPlan: %v", err)
	}
	if op.Status != "success" {
		t.Fatalf("status = %q", op.Status)
	}
	if err := RollbackSnapshot(op.SnapshotPath, platform.OSFileSystem{}, platform.OSLinker{}); err != nil {
		t.Fatalf("RollbackSnapshot: %v", err)
	}
	link, err := os.Readlink(target)
	if err != nil {
		t.Fatalf("回滚后链接没恢复: %v", err)
	}
	if filepath.Clean(link) != filepath.Clean(source) {
		t.Fatalf("回滚后指向 %q, want %q", link, source)
	}
}

// 新建链接必须用 junction：Windows 上创建符号链接需要管理员或开发者模式，
// 而 larder 是普通用户启动的桌面程序；junction 无需任何特权。
func TestApplyLinkPlanCreatesWindowsJunction(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("仅 Windows 适用")
	}
	base := t.TempDir()
	source := newSkillDir(t, base, "source")
	target := filepath.Join(base, "agent-skills", "demo")
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		t.Fatal(err)
	}

	plan, err := PlanLink(linkReq(source, target), platform.OSFileSystem{})
	if err != nil {
		t.Fatalf("PlanLink: %v", err)
	}
	op, err := ApplyLinkPlan(context.Background(), plan, filepath.Join(base, "snapshots"), platform.OSFileSystem{}, platform.OSLinker{})
	if err != nil {
		t.Fatalf("ApplyLinkPlan: %v", err)
	}
	if op.Status != "success" {
		t.Fatalf("status = %q, message = %q", op.Status, op.Message)
	}
	info, err := os.Lstat(target)
	if err != nil {
		t.Fatalf("链接没建出来: %v", err)
	}
	if info.Mode()&os.ModeIrregular == 0 {
		t.Fatalf("建出来的是 %v，不是 junction；普通用户建符号链接需要开发者模式", info.Mode())
	}
	link, err := os.Readlink(target)
	if err != nil {
		t.Fatalf("读不出链接目标: %v", err)
	}
	if filepath.Clean(link) != filepath.Clean(source) {
		t.Fatalf("指向 %q, want %q", link, source)
	}
}
