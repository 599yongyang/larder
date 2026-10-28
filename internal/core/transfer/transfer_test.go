package transfer

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"larder/internal/core/model"
)

func TestBuildInspectRestoreSelection(t *testing.T) {
	root := t.TempDir()
	dataDir := filepath.Join(root, "app")
	home := filepath.Join(root, "home")
	project := filepath.Join(home, "project")
	if err := os.MkdirAll(dataDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(project, 0o700); err != nil {
		t.Fatal(err)
	}
	instruction := filepath.Join(home, "AGENTS.md")
	command := filepath.Join(project, "review.md")
	if err := os.WriteFile(instruction, []byte("# instructions"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(command, []byte("# command"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dataDir, "catalog-installs.json"), []byte(`{"a":1}`), 0o600); err != nil {
		t.Fatal(err)
	}
	settings := model.Settings{ProjectWorkspaces: []string{project}, ShowAgentBuiltins: true}
	data, manifest, err := Build(Options{
		DataDir: dataDir, HomeDir: home, Projects: []string{project}, Settings: settings,
		Instructions: []model.InstructionFile{{Name: "AGENTS.md", Path: instruction, Scope: "global"}},
		Commands:     []model.Command{{Name: "review", Path: command, Agent: "Claude Code", Scope: "project", Project: project}},
		Selection:    Selection{Settings: true, AppState: true, Instructions: true, Commands: true},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(manifest.Files) != 4 {
		t.Fatalf("files = %d, want 4", len(manifest.Files))
	}
	info, err := Inspect(data)
	if err != nil {
		t.Fatal(err)
	}
	if info.Settings == nil || info.Settings.ShowAgentBuiltins != true {
		t.Fatalf("settings = %#v", info.Settings)
	}
	if info.Categories["mcp"] != 0 || info.Categories["settings"] != 1 {
		t.Fatalf("categories = %#v", info.Categories)
	}
	// Remove all targets then restore only instructions and settings.
	if err := os.Remove(instruction); err != nil {
		t.Fatal(err)
	}
	result, err := Restore(data, Selection{Settings: true, Instructions: true}, nil, dataDir, home, project)
	if err != nil {
		t.Fatal(err)
	}
	if result.Settings == nil || !result.Settings.ShowAgentBuiltins {
		t.Fatalf("restored settings = %#v", result.Settings)
	}
	if _, err := os.Stat(instruction); err != nil {
		t.Fatalf("instruction not restored: %v", err)
	}
	if _, err := os.Stat(command); err != nil {
		t.Fatalf("command should have been filtered out: %v", err)
	}
}

func TestBuildRequiresSelection(t *testing.T) {
	if _, _, err := Build(Options{}); err == nil {
		t.Fatal("expected selection error")
	}
}

func TestRestoreMapsHomeAndProjectRoots(t *testing.T) {
	sourceRoot := t.TempDir()
	sourceHome := filepath.Join(sourceRoot, "home")
	sourceProject := filepath.Join(sourceHome, "project")
	sourceData := filepath.Join(sourceRoot, "app")
	if err := os.MkdirAll(sourceProject, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(sourceData, 0o700); err != nil {
		t.Fatal(err)
	}
	instruction := filepath.Join(sourceHome, "AGENTS.md")
	command := filepath.Join(sourceProject, "review.md")
	if err := os.WriteFile(instruction, []byte("instructions"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(command, []byte("command"), 0o644); err != nil {
		t.Fatal(err)
	}
	data, _, err := Build(Options{DataDir: sourceData, HomeDir: sourceHome, Projects: []string{sourceProject}, Instructions: []model.InstructionFile{{Name: "AGENTS.md", Path: instruction, Scope: "global"}}, Commands: []model.Command{{Name: "review", Path: command, Agent: "Claude Code", Scope: "project", Project: sourceProject}}, Selection: Selection{Instructions: true, Commands: true}})
	if err != nil {
		t.Fatal(err)
	}
	destRoot := t.TempDir()
	destHome := filepath.Join(destRoot, "home")
	destProject := filepath.Join(destHome, "project")
	destData := filepath.Join(destRoot, "app")
	if err := os.MkdirAll(destProject, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(destData, 0o700); err != nil {
		t.Fatal(err)
	}
	if _, err := Restore(data, Selection{Instructions: true, Commands: true}, nil, destData, destHome, destProject); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(destHome, "AGENTS.md")); err != nil {
		t.Fatalf("mapped instruction missing: %v", err)
	}
	if _, err := os.Stat(filepath.Join(destProject, "review.md")); err != nil {
		t.Fatalf("mapped command missing: %v", err)
	}
}

func TestRestoreRollsBackWhenSettingsFail(t *testing.T) {
	root := t.TempDir()
	dataDir := filepath.Join(root, "app")
	home := filepath.Join(root, "home")
	if err := os.MkdirAll(dataDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(home, 0o700); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(home, "AGENTS.md")
	if err := os.WriteFile(target, []byte("old"), 0o644); err != nil {
		t.Fatal(err)
	}
	data, _, err := Build(Options{DataDir: dataDir, HomeDir: home, Settings: model.Settings{ShowAgentBuiltins: true}, Instructions: []model.InstructionFile{{Name: "AGENTS.md", Path: target, Scope: "global"}}, Selection: Selection{Settings: true, Instructions: true}})
	if err != nil {
		t.Fatal(err)
	}
	_, err = Restore(data, Selection{Settings: true, Instructions: true}, func(*model.Settings) error { return errors.New("settings failed") }, dataDir, home)
	if err == nil {
		t.Fatal("expected settings error")
	}
	raw, readErr := os.ReadFile(target)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if string(raw) != "old" {
		t.Fatalf("target = %q, want rollback", raw)
	}
}

func TestBuildAndRestoreSkillFolders(t *testing.T) {
	root := t.TempDir()
	home := filepath.Join(root, "home")
	dataDir := filepath.Join(root, "app")
	skillDir := filepath.Join(home, ".agents", "skills", "demo")
	for _, dir := range []string{skillDir, dataDir} {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte("# demo"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(skillDir, "scripts"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(skillDir, "scripts", "run.sh"), []byte("echo hi"), 0o755); err != nil {
		t.Fatal(err)
	}
	skill := model.Skill{Name: "demo", Path: skillDir, HasSkillMD: true, ScanState: "ok"}
	data, info, err := Build(Options{DataDir: dataDir, HomeDir: home, Skills: []model.Skill{skill}, Selection: Selection{Skills: true}})
	if err != nil {
		t.Fatal(err)
	}
	if info.Selection.Skills != true {
		t.Fatal("skills not selected")
	}
	destRoot := t.TempDir()
	destHome := filepath.Join(destRoot, "home")
	destData := filepath.Join(destRoot, "app")
	if err := os.MkdirAll(destHome, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(destData, 0o700); err != nil {
		t.Fatal(err)
	}
	if _, err := Restore(data, Selection{Skills: true}, nil, destData, destHome); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(destHome, ".agents", "skills", "demo", "scripts", "run.sh")); err != nil {
		t.Fatalf("skill file missing: %v", err)
	}
}
