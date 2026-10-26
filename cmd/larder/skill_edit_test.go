package larder

import (
	"os"
	"path/filepath"
	"testing"

	"larder/internal/core/model"
)

func TestPlanAndApplySkillEditCanBeRolledBack(t *testing.T) {
	home := t.TempDir()
	data := t.TempDir()
	t.Setenv("LARDER_HOME", home)
	t.Setenv("LARDER_DATA_DIR", data)
	service, err := NewAppService()
	if err != nil {
		t.Fatal(err)
	}
	skillDir := filepath.Join(home, ".agents", "skills", "alpha")
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(skillDir, "SKILL.md")
	if err := os.WriteFile(path, []byte("---\nname: alpha\ndescription: before\n---\n\nold\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	service.state.Snapshot.Skills = []model.Skill{{ID: "skill-alpha", Name: "alpha", Path: skillDir, HasSkillMD: true, ScanState: "ok"}}
	plan, err := service.PlanSkillEdit("skill-alpha", "---\nname: alpha\ndescription: after\n---\n\nnew\n")
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Warnings) != 0 {
		t.Fatalf("warnings = %v", plan.Warnings)
	}
	op, err := service.ApplySkillEdit(plan.ID)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(raw) != "---\nname: alpha\ndescription: after\n---\n\nnew\n" {
		t.Fatalf("SKILL.md = %q", raw)
	}
	if _, err := service.core.RollbackOperation(service.context(), op.ID); err != nil {
		t.Fatal(err)
	}
	raw, err = os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(raw) != "---\nname: alpha\ndescription: before\n---\n\nold\n" {
		t.Fatalf("restored SKILL.md = %q", raw)
	}
}

func TestSkillCreateAndRenameCanBeRolledBack(t *testing.T) {
	home := t.TempDir()
	data := t.TempDir()
	t.Setenv("LARDER_HOME", home)
	t.Setenv("LARDER_DATA_DIR", data)
	service, err := NewAppService()
	if err != nil {
		t.Fatal(err)
	}
	content := "---\nname: new-skill\ndescription: created\n---\n\nbody\n"
	createPlan, err := service.PlanSkillCreate("new-skill", content)
	if err != nil {
		t.Fatal(err)
	}
	createOp, err := service.ApplySkillCreate(createPlan.ID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(createPlan.Path); err != nil {
		t.Fatal(err)
	}
	foundCreated := false
	for _, skill := range service.state.Snapshot.Skills {
		if skill.Name == "new-skill" {
			foundCreated = true
			break
		}
	}
	if !foundCreated {
		t.Fatal("created Skill was not rescanned into state")
	}
	if _, err := service.core.RollbackOperation(service.context(), createOp.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(createPlan.Path); !os.IsNotExist(err) {
		t.Fatalf("created Skill still exists after rollback: %v", err)
	}

	oldDir := filepath.Join(home, ".agents", "skills", "old-name")
	if err := os.MkdirAll(oldDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(oldDir, "SKILL.md"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	service.state.Snapshot.Skills = []model.Skill{{ID: "skill-old", Name: "old-name", Path: oldDir, HasSkillMD: true, ScanState: "ok"}}
	renamePlan, err := service.PlanSkillRename("skill-old", "new-name")
	if err != nil {
		t.Fatal(err)
	}
	renameOp, err := service.ApplySkillRename(renamePlan.ID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(renamePlan.From); !os.IsNotExist(err) {
		t.Fatalf("old path still exists: %v", err)
	}
	if _, err := os.Stat(renamePlan.To); err != nil {
		t.Fatal(err)
	}
	foundRenamed := false
	for _, skill := range service.state.Snapshot.Skills {
		if skill.Name == "new-name" {
			foundRenamed = true
			break
		}
	}
	if !foundRenamed {
		t.Fatal("renamed Skill was not rescanned into state")
	}
	if _, err := service.core.RollbackOperation(service.context(), renameOp.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(renamePlan.From); err != nil {
		t.Fatalf("old path was not restored: %v", err)
	}
	if _, err := os.Stat(renamePlan.To); !os.IsNotExist(err) {
		t.Fatalf("new path still exists after rollback: %v", err)
	}
}

func TestPlanSkillEditRejectsUnchangedContent(t *testing.T) {
	home := t.TempDir()
	data := t.TempDir()
	t.Setenv("LARDER_HOME", home)
	t.Setenv("LARDER_DATA_DIR", data)
	service, err := NewAppService()
	if err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(home, ".agents", "skills", "alpha")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	content := "---\nname: alpha\ndescription: same\n---\n\nbody\n"
	if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	service.state.Snapshot.Skills = []model.Skill{{ID: "skill-alpha", Name: "alpha", Path: dir, HasSkillMD: true, ScanState: "ok"}}
	if _, err := service.PlanSkillEdit("skill-alpha", content); err == nil {
		t.Fatal("expected unchanged content to be rejected")
	}
}
