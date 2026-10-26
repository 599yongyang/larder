package commands

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/BurntSushi/toml"
)

func write(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestScanFindsGlobalAndProjectCommands(t *testing.T) {
	home := t.TempDir()
	project := filepath.Join(t.TempDir(), "shop")
	write(t, filepath.Join(home, ".claude", "commands", "engineering", "review.md"), "---\ndescription: Review a change\nargument-hint: [path]\nallowed-tools:\n  - Read\n---\nReview this change.\n")
	write(t, filepath.Join(project, ".github", "prompts", "review.prompt.md"), "Review the pull request.\n")
	write(t, filepath.Join(home, ".cursor", "commands", "fix.md"), "Fix the selected issue.\n")
	write(t, filepath.Join(project, ".gemini", "commands", "qa.toml"), "description = \"Run QA checks\"\nprompt = \"Run the QA checklist.\"\n")
	got := Scan(context.Background(), home, []string{project})
	if len(got) != 4 {
		t.Fatalf("commands = %d, want 3: %#v", len(got), got)
	}
	byName := map[string]struct{ agent, scope, project, description string }{
		"engineering/review": {"Claude Code", "global", "", "Review a change"},
		"review":             {"GitHub Copilot", "project", "shop", "Review the pull request."},
		"fix":                {"Cursor", "global", "", "Fix the selected issue."},
		"qa":                 {"Gemini CLI", "project", "shop", "Run QA checks"},
	}
	for _, command := range got {
		want, ok := byName[command.Name]
		if !ok {
			t.Fatalf("unexpected command %#v", command)
		}
		if command.Agent != want.agent || command.Scope != want.scope || command.Project != want.project || command.Description != want.description {
			t.Fatalf("command = %#v, want %#v", command, want)
		}
	}
}

func TestConvertCommandFormats(t *testing.T) {
	markdown := "---\ndescription: Review code\nargument-hint: [path]\n---\n\nReview this change.\n"
	converted, warnings := Convert(markdown, "markdown", "toml", "Gemini CLI")
	if len(warnings) == 0 {
		t.Fatal("expected argument-hint warning")
	}
	var command geminiCommand
	if err := toml.Unmarshal([]byte(converted), &command); err != nil {
		t.Fatal(err)
	}
	if command.Description != "Review code" || command.Prompt != "Review this change." {
		t.Fatalf("converted = %#v", command)
	}
	back, warnings := Convert(converted, "toml", "markdown", "Claude Code")
	if len(warnings) != 0 {
		t.Fatalf("unexpected warnings: %v", warnings)
	}
	if !strings.Contains(back, "description: \"Review code\"") || !strings.Contains(back, "Review this change.") {
		t.Fatalf("markdown = %q", back)
	}
}
