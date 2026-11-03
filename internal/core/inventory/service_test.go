package inventory_test

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"larder/internal/core/inventory"
	"larder/internal/core/model"
	"larder/internal/core/store"
	"larder/internal/platform"
)

func TestScanPersistsAndReloadsCacheWithoutTouchingUserAssets(t *testing.T) {
	home := t.TempDir()
	t.Setenv("LARDER_HOME", home)
	if err := os.MkdirAll(filepath.Join(home, ".agents", "skills", "alpha"), 0o755); err != nil {
		t.Fatal(err)
	}
	// A conventional agent directory outside the registry is now auto-discovered
	// when it contains at least one SKILL.md (ADR-0033).
	if err := os.MkdirAll(filepath.Join(home, "unadapted", "skills", "noise"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(home, "unadapted", "skills", "noise", "SKILL.md"), []byte("---\nname: noise\ndescription: unknown agent fixture\n---\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(home, ".agents", "skills", "alpha", "SKILL.md"), []byte("---\nname: alpha\ndescription: fixture skill\n---\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(filepath.Join(home, ".agents", "skills", "alpha", "SKILL.md"))
	if err != nil {
		t.Fatal(err)
	}
	db, err := store.Open(filepath.Join(t.TempDir(), "ledger.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	service := inventory.NewService(platform.LocalPaths{}, platform.OSFileSystem{}, db)
	result, err := service.Scan(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Skills) != 2 {
		t.Fatalf("skills = %#v", result.Skills)
	}
	names := map[string]bool{}
	for _, skill := range result.Skills {
		names[skill.Name] = true
	}
	if !names["alpha"] || !names["noise"] {
		t.Fatalf("skills = %#v", result.Skills)
	}
	if result.Status != "complete" {
		t.Fatalf("status = %s, want complete; absent known roots are not installed, not partial", result.Status)
	}
	for _, root := range result.Roots {
		if root.ID == "unknown-" || root.Kind != "unknown_agent" {
			continue
		}
		if root.EntryCount != 1 {
			t.Fatalf("unknown root = %#v", root)
		}
	}
	cached, ok, err := service.LoadCache(context.Background())
	if err != nil || !ok {
		t.Fatalf("cache = ok:%v err:%v", ok, err)
	}
	if !cached.FromCache || len(cached.Skills) != 2 {
		t.Fatalf("cached result = %#v", cached)
	}
	after, err := os.ReadFile(filepath.Join(home, ".agents", "skills", "alpha", "SKILL.md"))
	if err != nil {
		t.Fatal(err)
	}
	if string(before) != string(after) {
		t.Fatal("scanner modified a user skill")
	}
}

func TestPlanAndApplyLinkOperationUsesSnapshot(t *testing.T) {
	home := t.TempDir()
	t.Setenv("LARDER_HOME", home)
	skillDir := filepath.Join(home, ".agents", "skills", "alpha")
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte("---\nname: alpha\ndescription: fixture\n---\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	agentRoot := filepath.Join(home, ".claude", "skills")
	if err := os.MkdirAll(agentRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	db, err := store.Open(filepath.Join(t.TempDir(), "ledger.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	service := inventory.NewService(platform.LocalPaths{}, platform.OSFileSystem{}, db)
	service.SetSnapshotRoot(filepath.Join(t.TempDir(), "snapshots"))
	result, err := service.Scan(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	var skillID string
	for _, skill := range result.Skills {
		if skill.Name == "alpha" && skill.RootID == "global" {
			skillID = skill.ID
		}
	}
	if skillID == "" {
		t.Fatal("fixture skill not found")
	}
	plan, err := service.PlanLink(context.Background(), skillID, []string{"claude-code"})
	if err != nil {
		t.Fatal(err)
	}
	op, err := service.ApplyLinkPlan(context.Background(), plan.ID)
	if err != nil {
		t.Fatal(err)
	}
	if op.Status != "success" || op.SnapshotPath == "" {
		t.Fatalf("operation = %#v", op)
	}
	target := filepath.Join(agentRoot, "alpha")
	link, err := os.Readlink(target)
	if err != nil || filepath.Clean(link) != filepath.Clean(skillDir) {
		t.Fatalf("target not linked: link=%q err=%v", link, err)
	}
	operations, err := service.Operations(context.Background())
	if err != nil || len(operations) != 1 {
		t.Fatalf("operations = %#v err=%v", operations, err)
	}
	if _, err := service.RollbackOperation(context.Background(), op.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Lstat(target); !os.IsNotExist(err) {
		t.Fatal("rollback did not remove the link")
	}
	operations, err = service.Operations(context.Background())
	if err != nil || len(operations) != 2 {
		t.Fatalf("operations after rollback = %#v err=%v", operations, err)
	}
}

func TestPlanCopyToLinkAndRollbackRestoresCopy(t *testing.T) {
	home := t.TempDir()
	t.Setenv("LARDER_HOME", home)
	global := filepath.Join(home, ".agents", "skills", "alpha")
	copyDir := filepath.Join(home, ".claude", "skills", "alpha")
	for _, dir := range []string{global, copyDir} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(global, "SKILL.md"), []byte("---\nname: alpha\ndescription: canonical\n---\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(copyDir, "SKILL.md"), []byte("---\nname: alpha\ndescription: local copy\n---\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	db, err := store.Open(filepath.Join(t.TempDir(), "ledger.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	service := inventory.NewService(platform.LocalPaths{}, platform.OSFileSystem{}, db)
	service.SetSnapshotRoot(filepath.Join(t.TempDir(), "snapshots"))
	result, err := service.Scan(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	var skillID string
	for _, skill := range result.Skills {
		if skill.Name == "alpha" && skill.RootID == "global" {
			skillID = skill.ID
		}
	}
	if skillID == "" {
		t.Fatal("fixture skill not found")
	}
	plan, err := service.PlanCopyToLink(context.Background(), skillID, []string{"claude-code"})
	if err != nil {
		t.Fatal(err)
	}
	op, err := service.ApplyLinkPlan(context.Background(), plan.ID)
	if err != nil {
		t.Fatal(err)
	}
	link, err := os.Readlink(copyDir)
	if err != nil || filepath.Clean(link) != filepath.Clean(global) {
		t.Fatalf("copy not converted to link: link=%q err=%v", link, err)
	}
	if _, err := service.RollbackOperation(context.Background(), op.ID); err != nil {
		t.Fatal(err)
	}
	info, err := os.Lstat(copyDir)
	if err != nil || !info.IsDir() {
		t.Fatalf("copy not restored: %v", err)
	}
	content, err := os.ReadFile(filepath.Join(copyDir, "SKILL.md"))
	if err != nil || string(content) != "---\nname: alpha\ndescription: local copy\n---\n" {
		t.Fatalf("restored content = %q err=%v", content, err)
	}
}

type fakeCLIRunner struct{ path string }

func (f fakeCLIRunner) Find(string, []string) (string, bool) { return f.path, true }
func (f fakeCLIRunner) Run(context.Context, string, []string, []string) ([]byte, []byte, error) {
	return []byte("updated\n"), nil, nil
}

func TestCLIPlanExecutionUsesInjectedRunner(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("fixture uses a Unix symlink and executable")
	}
	home := t.TempDir()
	t.Setenv("LARDER_HOME", home)
	cliDir := filepath.Join(home, "brewbin")
	target := filepath.Join(home, "homebrew", "Cellar", "jq", "1.7.1", "bin", "jq")
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(cliDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(target, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, filepath.Join(cliDir, "jq")); err != nil {
		t.Fatal(err)
	}
	t.Setenv("LARDER_CLI_PATH", cliDir)
	db, err := store.Open(filepath.Join(t.TempDir(), "ledger.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	service := inventory.NewService(platform.LocalPaths{}, platform.OSFileSystem{}, db)
	service.SetSnapshotRoot(filepath.Join(t.TempDir(), "snapshots"))
	service.SetCommandRunner(fakeCLIRunner{path: "/fake/brew"})
	if _, err := service.Scan(context.Background()); err != nil {
		t.Fatal(err)
	}
	var cliID string
	for _, cli := range service.Snapshot().CLIs {
		if cli.Name == "jq" {
			cliID = cli.ID
		}
	}
	if cliID == "" {
		t.Fatal("fixture CLI not found")
	}
	plan, err := service.PlanCLI(context.Background(), cliID, false, "1.8.0")
	if err != nil {
		t.Fatal(err)
	}
	if plan.Command != "brew" || len(plan.Args) == 0 {
		t.Fatalf("plan command = %#v", plan)
	}
	op, err := service.ApplyPlan(context.Background(), plan.ID)
	if err != nil {
		t.Fatal(err)
	}
	if op.Status != "success" || op.SnapshotPath == "" {
		t.Fatalf("operation = %#v", op)
	}
}

func TestIgnoredAgentRootIsSkippedAndRestored(t *testing.T) {
	home := t.TempDir()
	t.Setenv("LARDER_HOME", home)
	root := filepath.Join(home, "future-agent", "skills")
	if err := os.MkdirAll(filepath.Join(root, "noise"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "noise", "SKILL.md"), []byte("---\nname: noise\ndescription: fixture\n---\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	db, err := store.Open(filepath.Join(t.TempDir(), "ledger.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	service := inventory.NewService(platform.LocalPaths{}, platform.OSFileSystem{}, db)
	if _, err := service.Scan(context.Background()); err != nil {
		t.Fatal(err)
	}
	settings, err := service.IgnoreAgent(context.Background(), root, "Future Agent", "unknown-test")
	if err != nil {
		t.Fatal(err)
	}
	if len(settings.IgnoredAgents) != 1 {
		t.Fatalf("ignored = %#v", settings.IgnoredAgents)
	}
	result, err := service.Scan(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Skills) != 0 {
		t.Fatalf("ignored agent was scanned: %#v", result.Skills)
	}
	if _, err := service.RestoreAgent(context.Background(), root); err != nil {
		t.Fatal(err)
	}
	result, err = service.Scan(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Skills) != 1 || result.Skills[0].Name != "noise" {
		t.Fatalf("restored skills = %#v", result.Skills)
	}
}

func TestTrashAgentMovesIgnoredRootToTrash(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("trash cleanup is macOS only")
	}
	home := t.TempDir()
	t.Setenv("LARDER_HOME", home)
	root := filepath.Join(home, ".future-agent", "skills")
	if err := os.MkdirAll(filepath.Join(root, "noise"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "noise", "SKILL.md"), []byte("---\nname: noise\ndescription: fixture\n---\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	db, err := store.Open(filepath.Join(t.TempDir(), "ledger.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	service := inventory.NewService(platform.LocalPaths{}, platform.OSFileSystem{}, db)
	if _, err := service.Scan(context.Background()); err != nil {
		t.Fatal(err)
	}
	if _, err := service.IgnoreAgent(context.Background(), root, "Future Agent", "future"); err != nil {
		t.Fatal(err)
	}
	settings, err := service.TrashAgent(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	if len(settings.IgnoredAgents) != 0 {
		t.Fatalf("ignored = %#v", settings.IgnoredAgents)
	}
	if _, err := os.Stat(root); !os.IsNotExist(err) {
		t.Fatalf("root still exists: %v", err)
	}
	trashed := filepath.Join(home, ".Trash", "skills")
	if _, err := os.Stat(filepath.Join(trashed, "noise", "SKILL.md")); err != nil {
		t.Fatalf("trashed skill missing: %v", err)
	}
}

func TestScanKeepsPersistedUpdateReport(t *testing.T) {
	home := t.TempDir()
	t.Setenv("LARDER_HOME", home)
	if err := os.MkdirAll(filepath.Join(home, ".agents", "skills", "alpha"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(home, ".agents", "skills", "alpha", "SKILL.md"), []byte("---\nname: alpha\ndescription: fixture\n---\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	db, err := store.Open(filepath.Join(t.TempDir(), "ledger.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	ctx := context.Background()
	report := model.UpdateReport{CheckedAt: time.Now(), Items: []model.UpdateItem{{Kind: "skill", Name: "alpha", State: "changed"}}, Available: 1, SkillChanged: 1}
	if err := db.SaveUpdates(ctx, report); err != nil {
		t.Fatal(err)
	}
	service := inventory.NewService(platform.LocalPaths{}, platform.OSFileSystem{}, db)
	if _, _, err := service.LoadCache(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Scan(ctx); err != nil {
		t.Fatal(err)
	}
	if got := service.Updates(); got.CheckedAt.IsZero() || len(got.Items) != 1 || got.Items[0].Name != "alpha" {
		t.Fatalf("update report was lost by scan: %#v", got)
	}
}

func TestEmptyUpdatePlaceholderDoesNotSuppressChecks(t *testing.T) {
	home := t.TempDir()
	t.Setenv("LARDER_HOME", home)
	db, err := store.Open(filepath.Join(t.TempDir(), "ledger.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	ctx := context.Background()
	if err := db.SaveUpdates(ctx, model.UpdateReport{CheckedAt: time.Now()}); err != nil {
		t.Fatal(err)
	}
	service := inventory.NewService(platform.LocalPaths{}, platform.OSFileSystem{}, db)
	if _, _, err := service.LoadCache(ctx); err != nil {
		t.Fatal(err)
	}
	if got := service.Updates(); !got.CheckedAt.IsZero() {
		t.Fatalf("empty placeholder should be discarded: %#v", got)
	}
}
