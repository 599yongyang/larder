// Package mcp reads MCP server definitions from common agent config files.
// It never starts a server, executes a tool or returns secret values.
package mcp

import (
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"larder/internal/core/model"
)

type configSpec struct {
	label  string
	path   string
	format string
	scope  string
}

func home() string {
	if value, err := os.UserHomeDir(); err == nil {
		return value
	}
	return ""
}

func configs() []configSpec {
	h := home()
	appSupport := filepath.Join(h, "Library", "Application Support")
	return []configSpec{
		{label: "Claude Code", path: filepath.Join(h, ".claude.json"), format: "claude-json"},
		{label: "Claude Desktop", path: filepath.Join(appSupport, "Claude", "claude_desktop_config.json"), format: "json"},
		{label: "Cursor", path: filepath.Join(h, ".cursor", "mcp.json"), format: "json"},
		{label: "Windsurf", path: filepath.Join(h, ".codeium", "windsurf", "mcp_config.json"), format: "json"},
		{label: "Gemini CLI", path: filepath.Join(h, ".gemini", "settings.json"), format: "json"},
		{label: "Continue", path: filepath.Join(h, ".continue", "config.json"), format: "json"},
		{label: "OpenCode", path: filepath.Join(h, ".config", "opencode", "opencode.json"), format: "opencode"},
		{label: "Codex", path: filepath.Join(h, ".codex", "config.toml"), format: "codex"},
		{label: "VS Code", path: filepath.Join(appSupport, "Code", "User", "mcp.json"), format: "json"},
		{label: "VS Code Insiders", path: filepath.Join(appSupport, "Code - Insiders", "User", "mcp.json"), format: "json"},
		{label: "Trae", path: filepath.Join(appSupport, "Trae", "User", "mcp.json"), format: "json"},
		{label: "Trae CN", path: filepath.Join(appSupport, "Trae CN", "User", "mcp.json"), format: "json"},
		{label: "Qoder", path: filepath.Join(appSupport, "Qoder", "SharedClientCache", "mcp.json"), format: "json"},
	}
}

type client struct {
	name       string
	transport  string
	command    string
	args       []string
	url        string
	envKeys    []string
	headerKeys []string
	enabled    bool
	envSection bool
	agent      model.MCPAgent
}

// Scan returns a deduplicated, read-only view of MCP servers found in agent
// configuration files. Files that are absent or unreadable are skipped.
func Scan(ctx context.Context) []model.MCPServer {
	return scanConfigs(ctx, configs())
}

// ProjectConfigs are project-scoped MCP config files an installer can write.
func projectConfigs(project string) []configSpec {
	name := filepath.Base(project)
	label := "项目 · " + name
	return []configSpec{
		{label: label, path: filepath.Join(project, ".mcp.json"), format: "json", scope: project},
		{label: "Cursor · " + name, path: filepath.Join(project, ".cursor", "mcp.json"), format: "json", scope: project},
		{label: "VS Code · " + name, path: filepath.Join(project, ".vscode", "mcp.json"), format: "json", scope: project},
		{label: "Codex · " + name, path: filepath.Join(project, ".codex", "config.toml"), format: "codex", scope: project},
	}
}

// ScanProjects includes MCP configs from project workspaces alongside global
// agent configs.
func ScanProjects(ctx context.Context, projects []string) []model.MCPServer {
	specs := configs()
	for _, project := range projects {
		if strings.TrimSpace(project) != "" {
			specs = append(specs, projectConfigs(project)...)
		}
	}
	return scanConfigs(ctx, specs)
}

// ProjectTargets lists writable project MCP configs.
func ProjectTargets(projects []string) []Target {
	var targets []Target
	for _, project := range projects {
		for _, spec := range projectConfigs(project) {
			targets = append(targets, Target{Label: spec.label, Path: spec.path, Format: spec.format})
		}
	}
	return targets
}

func scanConfigs(ctx context.Context, specs []configSpec) []model.MCPServer {
	byKey := map[string]*model.MCPServer{}
	order := []string{}
	for _, spec := range specs {
		if ctx.Err() != nil {
			break
		}
		raw, err := os.ReadFile(spec.path)
		if err != nil {
			continue
		}
		var entries []client
		switch spec.format {
		case "claude-json":
			entries = parseClaudeJSON(raw, spec)
		case "opencode":
			entries = parseOpenCodeJSON(raw, spec)
		case "codex":
			entries = parseCodexTOML(raw, spec)
		default:
			entries = parseJSONConfig(raw, spec)
		}
		for _, entry := range entries {
			key := serverKey(entry)
			server := byKey[key]
			if server == nil {
				server = &model.MCPServer{
					ID: hashedID(key), Name: entry.name, Transport: entry.transport,
					Command: entry.command, Args: entry.args, URL: entry.url,
					EnvKeys: entry.envKeys, HeaderKeys: entry.headerKeys,
				}
				byKey[key] = server
				order = append(order, key)
			}
			server.EnvKeys = union(server.EnvKeys, entry.envKeys)
			server.HeaderKeys = union(server.HeaderKeys, entry.headerKeys)
			server.Agents = append(server.Agents, entry.agent)
		}
	}
	result := make([]model.MCPServer, 0, len(order))
	for _, key := range order {
		server := byKey[key]
		sort.Slice(server.Agents, func(i, j int) bool { return server.Agents[i].Label < server.Agents[j].Label })
		server.Status = statusForServer(server)
		result = append(result, *server)
	}
	sort.Slice(result, func(i, j int) bool {
		if strings.EqualFold(result[i].Name, result[j].Name) {
			return result[i].ID < result[j].ID
		}
		return strings.ToLower(result[i].Name) < strings.ToLower(result[j].Name)
	})
	return result
}

func hashedID(value string) string {
	sum := sha256.Sum256([]byte(value))
	return "mcp:" + hex.EncodeToString(sum[:8])
}

func serverKey(entry client) string {
	scope := entry.agent.Scope
	if entry.command != "" {
		return "stdio:" + scope + "\x00" + entry.command + "\x00" + strings.Join(entry.args, "\x00")
	}
	if entry.url != "" {
		return "http:" + scope + "\x00" + strings.TrimRight(entry.url, "/")
	}
	return "name:" + scope + ":" + entry.name + ":" + entry.agent.Label
}

func statusForServer(server *model.MCPServer) string {
	if server.Command == "" && server.URL == "" {
		return "invalid"
	}
	enabled := false
	for _, agent := range server.Agents {
		if agent.Enabled {
			enabled = true
			break
		}
	}
	if !enabled {
		return "disabled"
	}
	if server.Command != "" && !commandExists(server.Command) {
		return "missing"
	}
	return "ok"
}

func union(left, right []string) []string {
	if len(right) == 0 {
		return left
	}
	seen := map[string]bool{}
	for _, item := range left {
		seen[item] = true
	}
	for _, item := range right {
		if !seen[item] {
			left = append(left, item)
			seen[item] = true
		}
	}
	sort.Strings(left)
	return left
}

func commandExists(command string) bool {
	if command == "" {
		return false
	}
	if strings.Contains(command, "/") {
		info, err := os.Stat(command)
		return err == nil && !info.IsDir()
	}
	_, err := exec.LookPath(command)
	return err == nil
}

func readStrings(value any) []string {
	raw, ok := value.([]any)
	if !ok {
		return nil
	}
	items := make([]string, 0, len(raw))
	for _, item := range raw {
		if text, ok := item.(string); ok {
			items = append(items, text)
		}
	}
	return items
}

func mapKeys(value any) []string {
	raw, ok := value.(map[string]any)
	if !ok {
		return nil
	}
	keys := make([]string, 0, len(raw))
	for key := range raw {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func firstString(values ...any) string {
	for _, value := range values {
		if text, ok := value.(string); ok && strings.TrimSpace(text) != "" {
			return text
		}
	}
	return ""
}

func transportOf(entry map[string]any, url string) string {
	if value := strings.ToLower(firstString(entry["type"])); strings.Contains(value, "sse") {
		return "sse"
	}
	if value := strings.ToLower(firstString(entry["type"])); strings.Contains(value, "http") || strings.Contains(value, "remote") {
		if url != "" {
			return "http"
		}
	}
	if url != "" {
		return "http"
	}
	if value := strings.ToLower(firstString(entry["type"])); value != "" && value != "stdio" && value != "local" {
		return value
	}
	return "stdio"
}

func entryEnabled(entry map[string]any) bool {
	if value, ok := entry["enabled"].(bool); ok && !value {
		return false
	}
	if value, ok := entry["disabled"].(bool); ok && value {
		return false
	}
	return true
}

func entryClients(name string, entry map[string]any, spec configSpec, scope string, enabled bool) client {
	env := entry["env"]
	if env == nil {
		env = entry["environment"]
	}
	command := firstString(entry["command"])
	args := readStrings(entry["args"])
	if list, ok := entry["command"].([]any); ok {
		command = firstString(list...)
		args = nil
		for _, item := range list[1:] {
			if text, ok := item.(string); ok {
				args = append(args, text)
			}
		}
	}
	url := firstString(entry["url"], entry["endpoint"])
	return client{
		name:       name,
		transport:  transportOf(entry, url),
		command:    command,
		args:       args,
		url:        url,
		envKeys:    mapKeys(env),
		headerKeys: mapKeys(entry["headers"]),
		enabled:    enabled,
		agent:      model.MCPAgent{Label: spec.label, Path: spec.path, Scope: scope, Enabled: enabled},
	}
}

func parseJSONConfig(raw []byte, spec configSpec) []client {
	var root map[string]any
	if err := json.Unmarshal(raw, &root); err != nil {
		return nil
	}
	var entries []client
	for _, key := range []string{"mcpServers", "mcp", "servers"} {
		servers, ok := root[key].(map[string]any)
		if !ok {
			continue
		}
		for name, value := range servers {
			entry, ok := value.(map[string]any)
			if !ok {
				continue
			}
			entries = append(entries, entryClients(name, entry, spec, spec.scope, entryEnabled(entry)))
		}
	}
	return entries
}

func parseClaudeJSON(raw []byte, spec configSpec) []client {
	var root map[string]any
	if err := json.Unmarshal(raw, &root); err != nil {
		return nil
	}
	entries := parseJSONConfig(raw, spec)
	projects, _ := root["projects"].(map[string]any)
	for path, value := range projects {
		project, ok := value.(map[string]any)
		if !ok {
			continue
		}
		servers, ok := project["mcpServers"].(map[string]any)
		if !ok {
			continue
		}
		for name, value := range servers {
			entry, ok := value.(map[string]any)
			if !ok {
				continue
			}
			entries = append(entries, entryClients(name, entry, spec, path, entryEnabled(entry)))
		}
	}
	return entries
}

func parseOpenCodeJSON(raw []byte, spec configSpec) []client {
	var root map[string]any
	if err := json.Unmarshal(raw, &root); err != nil {
		return nil
	}
	servers, ok := root["mcp"].(map[string]any)
	if !ok {
		return nil
	}
	var entries []client
	for name, value := range servers {
		entry, ok := value.(map[string]any)
		if !ok {
			continue
		}
		enabled := entryEnabled(entry)
		items := entryClients(name, entry, spec, spec.scope, enabled)
		if transport := firstString(entry["type"]); transport != "" {
			items.transport = transport
		}
		entries = append(entries, items)
	}
	return entries
}

func parseCodexTOML(raw []byte, spec configSpec) []client {
	byName := map[string]*client{}
	var order []string
	var current *client
	scanner := bufio.NewScanner(strings.NewReader(string(raw)))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			section := strings.Trim(line, "[]")
			if strings.HasPrefix(section, "mcp_servers.") {
				rest := strings.TrimPrefix(section, "mcp_servers.")
				if strings.HasSuffix(rest, ".env") || strings.HasSuffix(rest, ".env_vars") {
					current = byName[strings.TrimSuffix(strings.TrimSuffix(rest, ".env_vars"), ".env")]
					if current != nil {
						current.envSection = true
					}
					continue
				}
				name := strings.Trim(rest, "\"'")
				if byName[name] == nil {
					byName[name] = &client{name: name, transport: "stdio", enabled: true, agent: model.MCPAgent{Label: spec.label, Path: spec.path, Scope: spec.scope, Enabled: true}}
					order = append(order, name)
				}
				current = byName[name]
				current.envSection = false
				continue
			}
			current = nil
			continue
		}
		if current == nil {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		switch key {
		case "command":
			current.command = parseTOMLString(value)
		case "args":
			current.args = parseTOMLArray(value)
		case "url":
			current.url = parseTOMLString(value)
			current.transport = "http"
		case "enabled":
			current.enabled = value != "false"
			current.agent.Enabled = current.enabled
		case "env_vars":
			for _, item := range parseTOMLArray(value) {
				current.envKeys = append(current.envKeys, item)
			}
		default:
			if current.envSection {
				current.envKeys = append(current.envKeys, key)
			}
		}
	}
	entries := make([]client, 0, len(order))
	for _, name := range order {
		item := byName[name]
		if item.envKeys == nil {
			item.envKeys = []string{}
		}
		entries = append(entries, *item)
	}
	return entries
}

func parseTOMLString(value string) string {
	value = strings.TrimSpace(value)
	if unquoted, err := strconv.Unquote(value); err == nil {
		return unquoted
	}
	return strings.Trim(value, "\"'")
}

func parseTOMLArray(value string) []string {
	value = strings.TrimSpace(value)
	if !strings.HasPrefix(value, "[") || !strings.HasSuffix(value, "]") {
		return nil
	}
	value = strings.TrimSuffix(strings.TrimPrefix(value, "["), "]")
	var items []string
	for _, part := range strings.Split(value, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		items = append(items, parseTOMLString(part))
	}
	return items
}
