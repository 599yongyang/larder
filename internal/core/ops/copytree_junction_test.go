package ops

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"

	"larder/internal/core/model"
	"larder/internal/platform"
)

// Windows junction 的 os.Lstat 返回 ModeIrregular，既不带 ModeSymlink 也不满足
// IsDir。copyTree 过去只看 Mode 位，于是把 junction 当普通文件用 os.ReadFile 读，
// 报 "Incorrect function"，导致卸载预览失败。
func TestCopyTreePreservesWindowsJunction(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("junction 只存在于 Windows")
	}

	base := t.TempDir()
	real := filepath.Join(base, "real-skill")
	if err := os.MkdirAll(filepath.Join(real, "sub"), 0o755); err != nil {
		t.Fatalf("建目录失败: %v", err)
	}
	if err := os.WriteFile(filepath.Join(real, "SKILL.md"), []byte("---\nname: x\n---\n"), 0o644); err != nil {
		t.Fatalf("写文件失败: %v", err)
	}

	junction := filepath.Join(base, "junction-skill")
	if out, err := exec.Command("cmd", "/d", "/c", "mklink", "/J", junction, real).CombinedOutput(); err != nil {
		t.Skipf("无法创建 junction，跳过: %v %s", err, out)
	}

	backup := filepath.Join(base, "backup")
	if err := copyTree(junction, backup); err != nil {
		t.Fatalf("copyTree 处理 junction 失败: %v", err)
	}

	link, err := os.Readlink(backup)
	if err != nil {
		t.Fatalf("备份不是链接: %v", err)
	}
	if filepath.Clean(link) != filepath.Clean(real) {
		t.Fatalf("链接目标不符: got %q, want %q", link, real)
	}
	if _, err := os.Stat(filepath.Join(real, "SKILL.md")); err != nil {
		t.Fatalf("真实目录受损: %v", err)
	}
}

// RemoveAll 作用于 junction 时只应移除链接本身，不得删除链接指向的真实目录。
// 这是卸载路径的最后一步，必须保证 skillstash hub 实体不会被连带删除。
func TestRemoveAllOnJunctionKeepsTarget(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("junction 只存在于 Windows")
	}

	base := t.TempDir()
	real := filepath.Join(base, "real-skill")
	if err := os.MkdirAll(real, 0o755); err != nil {
		t.Fatalf("建目录失败: %v", err)
	}
	if err := os.WriteFile(filepath.Join(real, "SKILL.md"), []byte("x"), 0o644); err != nil {
		t.Fatalf("写文件失败: %v", err)
	}

	junction := filepath.Join(base, "junction-skill")
	if out, err := exec.Command("cmd", "/d", "/c", "mklink", "/J", junction, real).CombinedOutput(); err != nil {
		t.Skipf("无法创建 junction，跳过: %v %s", err, out)
	}

	if err := os.RemoveAll(junction); err != nil {
		t.Fatalf("RemoveAll 失败: %v", err)
	}
	if _, err := os.Lstat(junction); err == nil {
		t.Fatal("junction 应当已被移除")
	}
	if _, err := os.Stat(filepath.Join(real, "SKILL.md")); err != nil {
		t.Fatalf("真实目录被连带删除: %v", err)
	}
}

// 端到端复现用户在 larder 里点「卸载」时的完整链路：
// PlanUninstallSkill -> ApplyUninstallSkill（快照 -> copyTree -> RemoveAll）。
// 修复前 copyTree 对 junction 报 "Incorrect function"，表现为「预览卸载失败」。
func TestApplyUninstallSkillOnJunctionSucceedsAndKeepsTarget(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("junction 只存在于 Windows")
	}

	base := t.TempDir()
	// 模拟 skillstash hub 里的实体目录
	entity := filepath.Join(base, "hub", "skills", "demo-skill")
	if err := os.MkdirAll(entity, 0o755); err != nil {
		t.Fatalf("建实体目录失败: %v", err)
	}
	if err := os.WriteFile(filepath.Join(entity, "SKILL.md"), []byte("---\nname: demo-skill\n---\n"), 0o644); err != nil {
		t.Fatalf("写 SKILL.md 失败: %v", err)
	}
	// 模拟 agent 目录里的 junction
	agentDir := filepath.Join(base, "agent", "skills")
	if err := os.MkdirAll(agentDir, 0o755); err != nil {
		t.Fatalf("建 agent 目录失败: %v", err)
	}
	junction := filepath.Join(agentDir, "demo-skill")
	if out, err := exec.Command("cmd", "/d", "/c", "mklink", "/J", junction, entity).CombinedOutput(); err != nil {
		t.Skipf("无法创建 junction，跳过: %v %s", err, out)
	}

	plan := model.Plan{
		ID:               "plan:uninstall:test",
		Operation:        "uninstall_skill",
		SkillID:          "demo-skill",
		SkillName:        "demo-skill",
		TargetPath:       junction,
		RequiresSnapshot: true,
		Executable:       true,
	}

	op, err := ApplyUninstallSkill(context.Background(), plan, filepath.Join(base, "snapshots"), platform.OSFileSystem{})
	if err != nil {
		t.Fatalf("ApplyUninstallSkill 失败（这正是用户看到的「预览卸载失败」）: %v", err)
	}
	if op.Status != "success" {
		t.Fatalf("操作状态 = %q, 期望 success", op.Status)
	}
	if op.SnapshotPath == "" {
		t.Fatal("未记录快照路径")
	}
	if _, err := os.Stat(filepath.Join(op.SnapshotPath, "manifest.json")); err != nil {
		t.Fatalf("快照缺少 manifest.json: %v", err)
	}

	// junction 应当被移除
	if _, err := os.Lstat(junction); err == nil {
		t.Fatal("junction 应当已被移除")
	}
	// 关键：hub 实体必须完好
	if _, err := os.Stat(filepath.Join(entity, "SKILL.md")); err != nil {
		t.Fatalf("hub 实体被连带删除: %v", err)
	}
}
