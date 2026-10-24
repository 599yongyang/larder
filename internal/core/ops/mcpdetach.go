package ops

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"larder/internal/core/model"
)

// DetachedMCP is a removed per-agent MCP entry kept so it can be restored.
type DetachedMCP struct {
	Name       string   `json:"name"`
	AgentLabel string   `json:"agentLabel"`
	AgentPath  string   `json:"agentPath"`
	Format     string   `json:"format"`
	Container  string   `json:"container,omitempty"`
	Raw        string   `json:"raw"`
	Transport  string   `json:"transport"`
	Command    string   `json:"command,omitempty"`
	Args       []string `json:"args,omitempty"`
	URL        string   `json:"url,omitempty"`
}

// DetachMCPEntry snapshots a config, removes one server entry and returns the
// removed definition. Used by per-agent disable; the caller persists it.
func DetachMCPEntry(ctx context.Context, path, name, agentLabel, snapshotRoot string) (DetachedMCP, model.Operation, error) {
	now := time.Now()
	op := model.Operation{ID: fmt.Sprintf("op-mcp-detach-%d", now.UnixNano()), Operation: "mcp_detach", Status: "failed", CreatedAt: now}
	if ctx.Err() != nil {
		return DetachedMCP{}, op, ctx.Err()
	}
	rawFile, err := os.ReadFile(path)
	if err != nil {
		return DetachedMCP{}, op, err
	}
	snapshotDir, err := snapshotMCPConfig(path, snapshotRoot, "mcp-detach", now)
	if err != nil {
		return DetachedMCP{}, op, err
	}
	op.SnapshotPath = snapshotDir
	format := mcpConfigKind(path)
	detached := DetachedMCP{Name: name, AgentLabel: agentLabel, AgentPath: path, Format: format}
	switch format {
	case "toml":
		raw, rest, err := detachTOML(string(rawFile), name)
		if err != nil {
			return DetachedMCP{}, op, err
		}
		detached.Raw = raw
		if err := writeMCPConfig(path, []byte(rest)); err != nil {
			return DetachedMCP{}, op, err
		}
	case "opencode":
		container, raw, rest, entry, err := detachJSON(rawFile, name, "mcp")
		if err != nil {
			return DetachedMCP{}, op, err
		}
		detached.Container, detached.Raw = container, raw
		if err := writeMCPConfig(path, rest); err != nil {
			return DetachedMCP{}, op, err
		}
		detached.Transport, detached.Command, detached.Args, detached.URL = entry.Transport, entry.Command, entry.Args, entry.URL
	default:
		container, raw, rest, entry, err := detachJSON(rawFile, name, "")
		if err != nil {
			return DetachedMCP{}, op, err
		}
		detached.Container, detached.Raw = container, raw
		if err := writeMCPConfig(path, rest); err != nil {
			return DetachedMCP{}, op, err
		}
		detached.Transport, detached.Command, detached.Args, detached.URL = entry.Transport, entry.Command, entry.Args, entry.URL
	}
	op.Status = "success"
	op.Message = "已停用 " + name + " 的 Agent 配置；可用于恢复"
	op.Changes = []string{"detached:" + path}
	return detached, op, nil
}

// AttachMCPEntry restores a previously detached entry into its config.
func AttachMCPEntry(ctx context.Context, detached DetachedMCP, snapshotRoot string) (model.Operation, error) {
	now := time.Now()
	op := model.Operation{ID: fmt.Sprintf("op-mcp-attach-%d", now.UnixNano()), Operation: "mcp_attach", Status: "failed", CreatedAt: now}
	if ctx.Err() != nil {
		return op, ctx.Err()
	}
	path := detached.AgentPath
	raw, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return op, err
	}
	snapshotDir, err := snapshotMCPConfig(path, snapshotRoot, "mcp-attach", now)
	if err != nil {
		return op, err
	}
	op.SnapshotPath = snapshotDir
	switch detached.Format {
	case "toml":
		content := string(raw)
		if content != "" && !strings.HasSuffix(content, "\n") {
			content += "\n"
		}
		if err := writeMCPConfig(path, []byte(content+"\n"+detached.Raw)); err != nil {
			return op, err
		}
	default:
		root := map[string]any{}
		if len(strings.TrimSpace(string(raw))) > 0 {
			if err := json.Unmarshal(raw, &root); err != nil {
				return op, err
			}
		}
		container := detached.Container
		if container == "" {
			container = "mcpServers"
		}
		servers, _ := root[container].(map[string]any)
		if servers == nil {
			servers = map[string]any{}
		}
		var entry map[string]any
		if err := json.Unmarshal([]byte(detached.Raw), &entry); err != nil {
			return op, err
		}
		servers[detached.Name] = entry
		root[container] = servers
		output, _ := json.MarshalIndent(root, "", "  ")
		if err := writeMCPConfig(path, append(output, '\n')); err != nil {
			return op, err
		}
	}
	op.Status = "success"
	op.Message = "已启用 " + detached.Name + " 的 Agent 配置"
	op.Changes = []string{"attached:" + path}
	return op, nil
}

func mcpConfigKind(path string) string {
	if strings.HasSuffix(strings.ToLower(path), ".toml") {
		return "toml"
	}
	if strings.EqualFold(filepath.Base(path), "opencode.json") {
		return "opencode"
	}
	return "json"
}

type detachedJSONEntry struct {
	Transport string
	Command   string
	Args      []string
	URL       string
}

func detachJSON(raw []byte, name, fixedContainer string) (string, string, []byte, detachedJSONEntry, error) {
	var root map[string]any
	if err := json.Unmarshal(raw, &root); err != nil {
		return "", "", nil, detachedJSONEntry{}, err
	}
	container := fixedContainer
	if container == "" {
		for _, candidate := range []string{"mcpServers", "servers", "mcp"} {
			if _, ok := root[candidate].(map[string]any); ok {
				container = candidate
				break
			}
		}
	}
	servers, _ := root[container].(map[string]any)
	entry, _ := servers[name].(map[string]any)
	if entry == nil {
		return "", "", nil, detachedJSONEntry{}, fmt.Errorf("MCP server %s not found", name)
	}
	entryRaw, _ := json.Marshal(entry)
	delete(servers, name)
	root[container] = servers
	out, _ := json.MarshalIndent(root, "", "  ")
	info := detachedJSONEntry{}
	info.Transport = "stdio"
	if value, ok := entry["url"].(string); ok && value != "" {
		info.URL = value
		info.Transport = "http"
	}
	if value, ok := entry["command"].(string); ok {
		info.Command = value
	}
	if list, ok := entry["command"].([]any); ok && len(list) > 0 {
		if text, ok := list[0].(string); ok {
			info.Command = text
		}
		for _, item := range list[1:] {
			if text, ok := item.(string); ok {
				info.Args = append(info.Args, text)
			}
		}
	}
	if list, ok := entry["args"].([]any); ok {
		for _, item := range list {
			if text, ok := item.(string); ok {
				info.Args = append(info.Args, text)
			}
		}
	}
	return container, string(entryRaw), append(out, '\n'), info, nil
}

func detachTOML(content, name string) (string, string, error) {
	key := tomlMCPKey(name)
	mainSection := "[mcp_servers." + key + "]"
	envSection := mainSection[:len(mainSection)-1] + ".env]"
	lines := strings.Split(content, "\n")
	var kept, removed []string
	inSection := false
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "[") && strings.HasSuffix(trimmed, "]") {
			section := strings.Trim(trimmed, "[]")
			inSection = section == strings.Trim(mainSection, "[]") || section == strings.Trim(envSection, "[]")
		}
		if inSection {
			removed = append(removed, line)
		} else {
			kept = append(kept, line)
		}
	}
	if len(removed) == 0 {
		return "", "", fmt.Errorf("MCP server %s not found", name)
	}
	return strings.Join(removed, "\n"), strings.Join(kept, "\n"), nil
}

func snapshotMCPConfig(path, snapshotRoot, prefix string, now time.Time) (string, error) {
	if snapshotRoot == "" {
		return "", errors.New("snapshot root is required")
	}
	dir := filepath.Join(snapshotRoot, fmt.Sprintf("%s-%d", prefix, now.UnixNano()))
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	manifest := SnapshotManifest{ID: filepath.Base(dir), CreatedAt: now}
	target := SnapshotTarget{Target: path}
	if raw, err := os.ReadFile(path); err == nil {
		backup := filepath.Join(dir, "config.bak")
		if err := os.WriteFile(backup, raw, 0o600); err != nil {
			return "", err
		}
		target.Exists = true
		target.BackupPath = backup
	} else if !os.IsNotExist(err) {
		return "", err
	}
	manifest.Targets = []SnapshotTarget{target}
	raw, _ := json.MarshalIndent(manifest, "", "  ")
	if err := os.WriteFile(filepath.Join(dir, "manifest.json"), raw, 0o600); err != nil {
		return "", err
	}
	return dir, nil
}
