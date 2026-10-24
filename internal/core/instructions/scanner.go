package instructions

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"larder/internal/core/model"
)

var projectFiles = []string{"AGENTS.md", "CLAUDE.md", "GEMINI.md", ".cursorrules"}

// Scan finds user-level and project-level agent instruction files. It is
// read-only: only names, size, mtime, a hash and a short preview are read.
func Scan(ctx context.Context, home string, projects []string) []model.InstructionFile {
	byPath := map[string]model.InstructionFile{}
	add := func(path, agent, scope, project string) {
		if ctx.Err() != nil {
			return
		}
		info, err := os.Stat(path)
		if err != nil || info.IsDir() {
			return
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			return
		}
		sum := sha256.Sum256(raw)
		excerpt := strings.TrimSpace(string(raw))
		if runes := []rune(excerpt); len(runes) > 600 {
			excerpt = string(runes[:600]) + "…"
		}
		byPath[path] = model.InstructionFile{
			ID: "instruction:" + hex.EncodeToString(sum[:8]), Name: filepath.Base(path), Path: path,
			Agent: agent, Scope: scope, Project: project, Size: info.Size(), ModifiedAt: info.ModTime(),
			Hash: hex.EncodeToString(sum[:]), Excerpt: excerpt,
		}
	}
	if home != "" {
		add(filepath.Join(home, ".claude", "CLAUDE.md"), "Claude Code", "global", "")
		add(filepath.Join(home, ".codex", "AGENTS.md"), "Codex", "global", "")
		add(filepath.Join(home, ".gemini", "GEMINI.md"), "Gemini CLI", "global", "")
	}
	for _, project := range projects {
		project = strings.TrimSpace(project)
		if project == "" {
			continue
		}
		label := filepath.Base(project)
		for _, name := range projectFiles {
			add(filepath.Join(project, name), agentForName(name), "project", label)
		}
		add(filepath.Join(project, ".claude", "CLAUDE.md"), "Claude Code", "project", label)
		add(filepath.Join(project, ".github", "copilot-instructions.md"), "GitHub Copilot", "project", label)
		walkRules(ctx, filepath.Join(project, ".cursor", "rules"), "Cursor", label, &byPath)
		walkRules(ctx, filepath.Join(project, ".github", "instructions"), "GitHub Copilot", label, &byPath)
	}
	files := make([]model.InstructionFile, 0, len(byPath))
	seenHash := map[string]bool{}
	for _, file := range byPath {
		file.Duplicate = seenHash[file.Hash]
		seenHash[file.Hash] = true
		files = append(files, file)
	}
	sort.Slice(files, func(i, j int) bool {
		if files[i].Scope != files[j].Scope {
			return files[i].Scope < files[j].Scope
		}
		if files[i].Project != files[j].Project {
			return files[i].Project < files[j].Project
		}
		return files[i].Path < files[j].Path
	})
	return files
}

func walkRules(ctx context.Context, root, agent, project string, out *map[string]model.InstructionFile) {
	_ = filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil || ctx.Err() != nil {
			return nil
		}
		if entry.IsDir() || !strings.HasSuffix(strings.ToLower(entry.Name()), ".md") && !strings.HasSuffix(strings.ToLower(entry.Name()), ".mdc") {
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return nil
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			return nil
		}
		sum := sha256.Sum256(raw)
		excerpt := strings.TrimSpace(string(raw))
		if runes := []rune(excerpt); len(runes) > 600 {
			excerpt = string(runes[:600]) + "…"
		}
		(*out)[path] = model.InstructionFile{ID: "instruction:" + hex.EncodeToString(sum[:8]), Name: entry.Name(), Path: path, Agent: agent, Scope: "project", Project: project, Size: info.Size(), ModifiedAt: info.ModTime(), Hash: hex.EncodeToString(sum[:]), Excerpt: excerpt}
		return nil
	})
}

func agentForName(name string) string {
	switch name {
	case "CLAUDE.md":
		return "Claude Code"
	case "GEMINI.md":
		return "Gemini CLI"
	case ".cursorrules":
		return "Cursor"
	default:
		return "通用"
	}
}
