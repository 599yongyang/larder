package platform

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
)

// TestProbeJunctionSemantics 是一次性探针：打印 Windows 上 junction 与 symlink
// 在 os.Lstat / os.Readlink / EvalSymlinks / OSFileSystem.Readlink 下的真实行为。
// 用来决定补丁该依赖哪种判别方式，跑完可保留作为回归基线。
func TestProbeJunctionSemantics(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("仅 Windows 适用")
	}
	base := t.TempDir()
	real := filepath.Join(base, "real")
	if err := os.MkdirAll(real, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(real, "SKILL.md"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	junction := filepath.Join(base, "junction")
	out, err := exec.Command("cmd.exe", "/d", "/c", "mklink", "/J", junction, real).CombinedOutput()
	t.Logf("mklink /J err=%v out=%s", err, out)
	if err != nil {
		t.Fatalf("mklink 失败: %v", err)
	}

	info, err := os.Lstat(junction)
	if err != nil {
		t.Fatalf("Lstat: %v", err)
	}
	t.Logf("junction Lstat: mode=%v err=%v IsDir=%v ModeSymlink=%v ModeIrregular=%v",
		info.Mode(), err, info.IsDir(),
		info.Mode()&os.ModeSymlink != 0,
		info.Mode()&os.ModeIrregular != 0)

	rl, rlErr := os.Readlink(junction)
	t.Logf("junction os.Readlink=%q err=%v", rl, rlErr)

	ev, evErr := filepath.EvalSymlinks(junction)
	t.Logf("junction EvalSymlinks=%q err=%v", ev, evErr)

	fsRead, fsErr := (OSFileSystem{}).Readlink(junction)
	t.Logf("junction OSFileSystem.Readlink=%q err=%v", fsRead, fsErr)

	// os.Symlink 创建目录链接在非管理员/无开发者模式下会失败
	sym := filepath.Join(base, "symlink")
	symErr := os.Symlink(real, sym)
	t.Logf("os.Symlink(dir) err=%v", symErr)
	if symErr == nil {
		li, _ := os.Lstat(sym)
		t.Logf("symlink Lstat: mode=%v IsDir=%v ModeSymlink=%v", li.Mode(), li.IsDir(), li.Mode()&os.ModeSymlink != 0)
	}

	// os.Remove 作用于 junction：应当只摘链接、保留目标内容
	rmErr := os.Remove(junction)
	_, stillErr := os.Stat(real)
	t.Logf("os.Remove(junction) err=%v ; 目标实体仍存在=%v", rmErr, stillErr == nil)
}
