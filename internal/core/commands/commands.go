// Package commands scans read-only custom Agent slash commands.
package commands

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/BurntSushi/toml"
	"gopkg.in/yaml.v3"

	"larder/internal/core/model"
)

type format string

const (
	formatMarkdown format = "markdown"
	formatTOML     format = "toml"
)

type source struct {
	agent   string
	root    string
	scope   string
	project string
	format  format
}

var globalSources = func(home string) []source {
	return []source{
		{agent: "Claude Code", root: filepath.Join(home, ".claude", "commands"), scope: "global"},
		{agent: "OpenCode", root: filepath.Join(home, ".config", "opencode", "command"), scope: "global"},
		{agent: "Cursor", root: filepath.Join(home, ".cursor", "commands"), scope: "global"},
		{agent: "Gemini CLI", root: filepath.Join(home, ".gemini", "commands"), scope: "global", format: formatTOML},
	}
}

var projectSources = func(project string) []source {
	return []source{
		{agent: "Claude Code", root: filepath.Join(project, ".claude", "commands"), scope: "project", project: project},
		{agent: "OpenCode", root: filepath.Join(project, ".opencode", "command"), scope: "project", project: project},
		{agent: "GitHub Copilot", root: filepath.Join(project, ".github", "prompts"), scope: "project", project: project},
		{agent: "Cursor", root: filepath.Join(project, ".cursor", "commands"), scope: "project", project: project},
		{agent: "Gemini CLI", root: filepath.Join(project, ".gemini", "commands"), scope: "project", project: project, format: formatTOML},
	}
}

// Targets returns writable command roots for the supported Agents.
func Targets(home string, projects []string) []model.CommandTarget {
	sources := make([]source, 0)
	if home != "" {
		sources = append(sources, globalSources(home)...)
	}
	for _, project := range projects {
		project = strings.TrimSpace(project)
		if project != "" {
			sources = append(sources, projectSources(project)...)
		}
	}
	seen := map[string]bool{}
	targets := make([]model.CommandTarget, 0, len(sources))
	for _, src := range sources {
		fileFormat := src.format
		if fileFormat == "" {
			fileFormat = formatMarkdown
		}
		key := src.agent + "|" + src.scope + "|" + src.project + "|" + src.root
		if seen[key] {
			continue
		}
		seen[key] = true
		project := ""
		if src.scope == "project" {
			project = filepath.Base(src.project)
		}
		targets = append(targets, model.CommandTarget{Agent: src.agent, Scope: src.scope, Project: project, RootPath: src.root, Format: string(fileFormat)})
	}
	return targets
}

func Extension(format string) string {
	if format == string(formatTOML) {
		return ".toml"
	}
	return ".md"
}

// Convert renders a command for another Agent format. It is deliberately
// explicit: unsupported metadata is reported instead of silently dropped.
func Convert(content, sourceFormat, targetFormat, targetAgent string) (string, []string) {
	if sourceFormat == targetFormat {
		return content, nil
	}
	if sourceFormat == string(formatMarkdown) && targetFormat == string(formatTOML) {
		meta := metadata{}
		if text, ok := frontmatter(content); ok {
			_ = yaml.Unmarshal([]byte(text), &meta)
		}
		body := commandBody(content)
		warnings := make([]string, 0)
		if argumentHint := stringValue(meta.ArgumentHint); argumentHint != "" {
			warnings = append(warnings, "argument-hint is not supported by "+targetAgent+" TOML commands")
		}
		if len(stringList(meta.AllowedTools)) > 0 {
			warnings = append(warnings, "allowed-tools is not supported by "+targetAgent+" TOML commands")
		}
		if strings.TrimSpace(meta.Model) != "" {
			warnings = append(warnings, "model is not supported by "+targetAgent+" TOML commands")
		}
		var out strings.Builder
		out.WriteString("description = ")
		out.WriteString(strconv.Quote(meta.Description))
		out.WriteString("\nprompt = ")
		out.WriteString(strconv.Quote(body))
		out.WriteString("\n")
		return out.String(), warnings
	}
	if sourceFormat == string(formatTOML) && targetFormat == string(formatMarkdown) {
		var command geminiCommand
		_ = toml.Unmarshal([]byte(content), &command)
		var out strings.Builder
		out.WriteString("---\n")
		if strings.TrimSpace(command.Description) != "" {
			out.WriteString("description: ")
			out.WriteString(strconv.Quote(command.Description))
			out.WriteString("\n")
		}
		out.WriteString("---\n\n")
		out.WriteString(command.Prompt)
		out.WriteString("\n")
		return out.String(), nil
	}
	return content, []string{"conversion between " + sourceFormat + " and " + targetFormat + " is not implemented"}
}

func commandBody(content string) string {
	normalized := strings.ReplaceAll(content, "\r\n", "\n")
	if !strings.HasPrefix(normalized, "---\n") {
		return strings.TrimSpace(normalized)
	}
	rest := normalized[4:]
	if end := strings.Index(rest, "\n---"); end >= 0 {
		return strings.TrimSpace(rest[end+4:])
	}
	return strings.TrimSpace(normalized)
}

// Scan walks the known custom-command directories. It never writes or invokes
// an Agent; only Markdown files are read.
func Scan(ctx context.Context, home string, projects []string) []model.Command {
	var sources []source
	if home != "" {
		sources = append(sources, globalSources(home)...)
	}
	for _, project := range projects {
		project = strings.TrimSpace(project)
		if project != "" {
			sources = append(sources, projectSources(project)...)
		}
	}
	byPath := map[string]model.Command{}
	for _, src := range sources {
		if ctx.Err() != nil {
			break
		}
		scanDir(ctx, src, byPath)
	}
	out := make([]model.Command, 0, len(byPath))
	for _, command := range byPath {
		out = append(out, command)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Agent != out[j].Agent {
			return out[i].Agent < out[j].Agent
		}
		if out[i].Scope != out[j].Scope {
			return out[i].Scope < out[j].Scope
		}
		if out[i].Project != out[j].Project {
			return out[i].Project < out[j].Project
		}
		return out[i].Name < out[j].Name
	})
	return out
}

func scanDir(ctx context.Context, src source, out map[string]model.Command) {
	_ = filepath.WalkDir(src.root, func(path string, entry os.DirEntry, err error) error {
		if err != nil || ctx.Err() != nil {
			return nil
		}
		if entry.IsDir() {
			return nil
		}
		fileFormat := src.format
		if fileFormat == "" {
			fileFormat = formatMarkdown
		}
		ext := strings.ToLower(filepath.Ext(entry.Name()))
		if fileFormat == formatMarkdown && ext != ".md" && ext != ".markdown" {
			return nil
		}
		if fileFormat == formatTOML && ext != ".toml" {
			return nil
		}
		info, err := entry.Info()
		if err != nil || info.Size() > 2<<20 {
			return nil
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			return nil
		}
		rel, err := filepath.Rel(src.root, path)
		if err != nil {
			return nil
		}
		name := strings.TrimSuffix(filepath.ToSlash(rel), filepath.Ext(rel))
		name = strings.TrimSuffix(name, ".prompt")
		name = strings.TrimSpace(name)
		if name == "" {
			return nil
		}
		sum := sha256.Sum256(raw)
		description, argumentHint, modelName, allowedTools := parseMetadata(string(raw))
		if fileFormat == formatTOML {
			description, argumentHint, modelName, allowedTools = parseTOMLMetadata(string(raw))
		}
		project := ""
		if src.scope == "project" {
			project = filepath.Base(src.project)
		}
		out[path] = model.Command{
			ID:           "command:" + hashText(path),
			Name:         name,
			Description:  description,
			ArgumentHint: argumentHint,
			AllowedTools: allowedTools,
			Model:        modelName,
			Agent:        src.agent,
			RootPath:     src.root,
			Path:         path,
			Format:       string(fileFormat),
			Scope:        src.scope,
			Project:      project,
			Size:         info.Size(),
			ModifiedAt:   info.ModTime(),
			Hash:         hex.EncodeToString(sum[:]),
		}
		return nil
	})
}

type metadata struct {
	Description  string `yaml:"description"`
	ArgumentHint any    `yaml:"argument-hint"`
	Model        string `yaml:"model"`
	AllowedTools any    `yaml:"allowed-tools"`
}

func parseMetadata(content string) (string, string, string, []string) {
	meta := metadata{}
	if text, ok := frontmatter(content); ok {
		_ = yaml.Unmarshal([]byte(text), &meta)
	}
	description := strings.TrimSpace(meta.Description)
	if description == "" {
		description = firstBodyLine(content)
	}
	return description, stringValue(meta.ArgumentHint), strings.TrimSpace(meta.Model), stringList(meta.AllowedTools)
}

type geminiCommand struct {
	Description string `toml:"description"`
	Prompt      string `toml:"prompt"`
}

func parseTOMLMetadata(content string) (string, string, string, []string) {
	var command geminiCommand
	_ = toml.Unmarshal([]byte(content), &command)
	description := strings.TrimSpace(command.Description)
	if description == "" {
		description = firstBodyLine(command.Prompt)
	}
	return description, "", "", nil
}

func frontmatter(content string) (string, bool) {
	normalized := strings.ReplaceAll(content, "\r\n", "\n")
	if !strings.HasPrefix(normalized, "---\n") {
		return "", false
	}
	rest := normalized[4:]
	end := strings.Index(rest, "\n---")
	if end < 0 {
		return "", false
	}
	return rest[:end], true
}

func firstBodyLine(content string) string {
	normalized := strings.ReplaceAll(content, "\r\n", "\n")
	if strings.HasPrefix(normalized, "---\n") {
		if end := strings.Index(normalized[4:], "\n---"); end >= 0 {
			normalized = normalized[4+end+4:]
		}
	}
	for _, line := range strings.Split(normalized, "\n") {
		line = strings.TrimSpace(strings.TrimLeft(line, "#>-*` "))
		if line != "" {
			runes := []rune(line)
			if len(runes) > 160 {
				line = string(runes[:160]) + "…"
			}
			return line
		}
	}
	return ""
}

func stringValue(value any) string {
	switch item := value.(type) {
	case string:
		return strings.TrimSpace(item)
	case []any:
		parts := make([]string, 0, len(item))
		for _, entry := range item {
			if text, ok := entry.(string); ok {
				parts = append(parts, text)
			}
		}
		return strings.TrimSpace(strings.Join(parts, " "))
	}
	return ""
}

func stringList(value any) []string {
	switch item := value.(type) {
	case string:
		if strings.TrimSpace(item) != "" {
			return []string{strings.TrimSpace(item)}
		}
	case []any:
		out := make([]string, 0, len(item))
		for _, entry := range item {
			if text, ok := entry.(string); ok && strings.TrimSpace(text) != "" {
				out = append(out, strings.TrimSpace(text))
			}
		}
		return out
	case []string:
		return item
	}
	return nil
}

func hashText(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:8])
}
