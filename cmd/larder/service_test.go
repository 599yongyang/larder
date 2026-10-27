package larder

import (
	"os"
	"path/filepath"
	"testing"

	"larder/internal/core/model"
)

func TestValidateInstructionPath(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "AGENTS.md"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := validateInstructionPath(filepath.Join(root, "AGENTS.md"), []string{root}); err != nil {
		t.Fatalf("valid path rejected: %v", err)
	}
	if err := validateInstructionPath(filepath.Join(root, "notes.txt"), []string{root}); err != nil {
		t.Fatalf("valid txt rejected: %v", err)
	}
	if err := validateInstructionPath(filepath.Join(root, "script.sh"), []string{root}); err == nil {
		t.Fatal("unsupported extension accepted")
	}
	if err := validateInstructionPath(filepath.Join(t.TempDir(), "AGENTS.md"), []string{root}); err == nil {
		t.Fatal("outside root accepted")
	}
}

func TestSessionResumeCommand(t *testing.T) {
	service := &AppService{}
	claude := model.Session{Agent: "Claude Code", ID: "abc-123", CWD: "/Users/me/My Project"}
	command, err := service.SessionResumeCommand(claude)
	if err != nil {
		t.Fatal(err)
	}
	want := "cd '/Users/me/My Project' && claude --resume 'abc-123'"
	if command != want {
		t.Fatalf("command = %q, want %q", command, want)
	}
	codex := model.Session{Agent: "Codex", ID: "0199f3"}
	command, err = service.SessionResumeCommand(codex)
	if err != nil || command != "codex resume '0199f3'" {
		t.Fatalf("codex command = %q, err = %v", command, err)
	}
	if _, err := service.SessionResumeCommand(model.Session{Agent: "DSH", ID: "x"}); err == nil {
		t.Fatal("expected unsupported agent error")
	}
}
