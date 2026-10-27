package larder

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"larder/internal/core/model"
)

func TestPlanAndApplyCommandWriteCanBeRolledBack(t *testing.T) {
	home := t.TempDir()
	data := t.TempDir()
	t.Setenv("LARDER_HOME", home)
	t.Setenv("LARDER_DATA_DIR", data)
	service, err := NewAppService()
	if err != nil {
		t.Fatal(err)
	}
	var target model.CommandTarget
	for _, candidate := range service.CommandTargets() {
		if candidate.Agent == "Claude Code" && candidate.Scope == "global" {
			target = candidate
			break
		}
	}
	if target.RootPath == "" {
		t.Fatal("Claude Code global target not found")
	}
	plan, err := service.PlanCommandWrite(model.CommandWriteRequest{Agent: target.Agent, Scope: target.Scope, RootPath: target.RootPath, Name: "review", Content: "# Review\n"})
	if err != nil {
		t.Fatal(err)
	}
	op, err := service.ApplyCommandWrite(plan.ID)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(target.RootPath, "review.md")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(raw) != "# Review\n" {
		t.Fatalf("content = %q", raw)
	}
	if _, err := service.core.RollbackOperation(service.context(), op.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("command still exists after rollback: %v", err)
	}
}

func TestPlanCommandConversionToGeminiTOML(t *testing.T) {
	home := t.TempDir()
	data := t.TempDir()
	t.Setenv("LARDER_HOME", home)
	t.Setenv("LARDER_DATA_DIR", data)
	service, err := NewAppService()
	if err != nil {
		t.Fatal(err)
	}
	var claude, gemini model.CommandTarget
	for _, target := range service.CommandTargets() {
		if target.Scope != "global" {
			continue
		}
		if target.Agent == "Claude Code" {
			claude = target
		}
		if target.Agent == "Gemini CLI" {
			gemini = target
		}
	}
	if claude.RootPath == "" || gemini.RootPath == "" {
		t.Fatal("expected global Claude and Gemini targets")
	}
	sourcePlan, err := service.PlanCommandWrite(model.CommandWriteRequest{Agent: claude.Agent, Scope: claude.Scope, RootPath: claude.RootPath, Name: "review", Content: "---\ndescription: Review code\nargument-hint: [path]\n---\n\nReview this change.\n"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.ApplyCommandWrite(sourcePlan.ID); err != nil {
		t.Fatal(err)
	}
	sourcePath := filepath.Join(claude.RootPath, "review.md")
	plan, err := service.PlanCommandWrite(model.CommandWriteRequest{SourcePath: sourcePath, Agent: gemini.Agent, Scope: gemini.Scope, RootPath: gemini.RootPath, Name: "review"})
	if err != nil {
		t.Fatal(err)
	}
	if plan.Operation != "command_convert" || len(plan.Warnings) == 0 {
		t.Fatalf("plan = %#v", plan)
	}
	if _, err := service.ApplyCommandWrite(plan.ID); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(filepath.Join(gemini.RootPath, "review.toml"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), "description = \"Review code\"") || !strings.Contains(string(raw), "Review this change.") {
		t.Fatalf("converted = %q", raw)
	}
}

func TestPlanAndApplyCommandDeleteCanBeRolledBack(t *testing.T) {
	home := t.TempDir()
	data := t.TempDir()
	t.Setenv("LARDER_HOME", home)
	t.Setenv("LARDER_DATA_DIR", data)
	service, err := NewAppService()
	if err != nil {
		t.Fatal(err)
	}
	var target model.CommandTarget
	for _, candidate := range service.CommandTargets() {
		if candidate.Agent == "Claude Code" && candidate.Scope == "global" {
			target = candidate
			break
		}
	}
	if target.RootPath == "" {
		t.Fatal("Claude Code global target not found")
	}
	path := filepath.Join(target.RootPath, "delete-me.md")
	createPlan, err := service.PlanCommandWrite(model.CommandWriteRequest{Agent: target.Agent, Scope: target.Scope, RootPath: target.RootPath, Name: "delete-me", Content: "# Delete me\n"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.ApplyCommandWrite(createPlan.ID); err != nil {
		t.Fatal(err)
	}
	deletePlan, err := service.PlanCommandDelete(path)
	if err != nil {
		t.Fatal(err)
	}
	deleteOp, err := service.ApplyCommandDelete(deletePlan.ID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("command still exists: %v", err)
	}
	if _, err := service.core.RollbackOperation(service.context(), deleteOp.ID); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(raw) != "# Delete me\n" {
		t.Fatalf("restored content = %q", raw)
	}
}
