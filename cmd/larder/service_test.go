package larder

import (
	"os"
	"path/filepath"
	"testing"
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
