package ops

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"larder/internal/core/model"
	"larder/internal/platform"
)

var mcpNamePattern = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9._-]{0,63}$`)
var envKeyPattern = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

// PlanMCPAdd writes a server definition into the selected agent config files.
func PlanMCPAdd(name, transport, command string, args []string, url string, env map[string]string, agents []model.MCPAgent) (model.Plan, error) {
	if !mcpNamePattern.MatchString(name) {
		return model.Plan{}, errors.New("invalid MCP server name")
	}
	if transport != "stdio" && transport != "http" {
		return model.Plan{}, errors.New("transport must be stdio or http")
	}
	if transport == "stdio" && command == "" {
		return model.Plan{}, errors.New("stdio MCP server needs a command")
	}
	if transport == "http" && url == "" {
		return model.Plan{}, errors.New("http MCP server needs a URL")
	}
	if len(agents) == 0 {
		return model.Plan{}, errors.New("select at least one agent config")
	}
	for key := range env {
		if !envKeyPattern.MatchString(key) {
			return model.Plan{}, fmt.Errorf("invalid environment key: %s", key)
		}
	}
	plan := model.Plan{
		ID:               "plan:mcp-add:" + name + ":" + fmt.Sprint(time.Now().UnixNano()),
		Operation:        "mcp_add",
		Title:            "添加 MCP Server",
		TitleMessage:     planTitle("plan.addMcp.title", "添加 MCP Server"),
		SummaryMessage:   planMessage("plan.summary.mcpAdd", "向选中的 Agent 写入 {name}；写入前会备份配置文件。", map[string]string{"name": name}),
		MCPName:          name,
		Command:          command,
		Args:             args,
		MCPURL:           url,
		MCPEnv:           env,
		RequiresSnapshot: true,
		Executable:       true,
		CreatedAt:        time.Now(),
		Impact:           []string{},
	}
	for _, agent := range agents {
		plan.Changes = append(plan.Changes, model.PlanChange{Action: "add_mcp_entry", RootID: agent.Label, RootLabel: agent.Label, Target: agent.Path, Reason: "写入该 Agent 的 MCP 配置"})
	}
	addWarning(&plan, "plan.warning.mcpConfigOnly", "只写入 MCP 配置，不会安装命令或启动 Server。", nil)
	addWarning(&plan, "plan.warning.configBackedUp", "执行前会备份原配置，可在历史中回滚。", nil)
	return plan, nil
}

// ApplyMCPAdd snapshots each selected config and writes the server entry.
func ApplyMCPAdd(ctx context.Context, plan model.Plan, snapshotRoot string, _ platform.FileSystem) (model.Operation, error) {
	now := time.Now()
	op := model.Operation{ID: fmt.Sprintf("op-%s-%d", plan.ID, now.UnixNano()), PlanID: plan.ID, Operation: plan.Operation, Status: "failed", CreatedAt: now}
	if snapshotRoot == "" || plan.MCPName == "" || len(plan.Changes) == 0 {
		return op, errors.New("invalid MCP add plan")
	}
	snapshotDir := filepath.Join(snapshotRoot, fmt.Sprintf("mcp-add-%d", now.UnixNano()))
	if err := os.MkdirAll(snapshotDir, 0o700); err != nil {
		return op, err
	}
	manifest := SnapshotManifest{ID: filepath.Base(snapshotDir), PlanID: plan.ID, CreatedAt: now}
	existed := map[string]bool{}
	for i, change := range plan.Changes {
		raw, err := os.ReadFile(change.Target)
		if err != nil && !os.IsNotExist(err) {
			return op, err
		}
		exists := err == nil
		existed[change.Target] = exists
		backup := ""
		if exists {
			backup = filepath.Join(snapshotDir, fmt.Sprintf("config-%d.bak", i))
			if err := os.WriteFile(backup, raw, 0o600); err != nil {
				return op, err
			}
		}
		manifest.Targets = append(manifest.Targets, SnapshotTarget{RootID: change.RootID, Target: change.Target, Action: "add_mcp_entry", Exists: exists, BackupPath: backup})
	}
	raw, _ := json.MarshalIndent(manifest, "", "  ")
	if err := os.WriteFile(filepath.Join(snapshotDir, "manifest.json"), raw, 0o600); err != nil {
		return op, err
	}
	op.SnapshotPath = snapshotDir
	for _, change := range plan.Changes {
		if err := ctx.Err(); err != nil {
			return op, err
		}
		if err := addMCPEntry(change.Target, plan); err != nil {
			restoreMCPAddBackups(manifest, existed)
			return op, err
		}
	}
	op.Status = "success"
	op.Message = "已写入 " + plan.MCPName + " 的 MCP 配置；服务器快照可回滚"
	for _, change := range plan.Changes {
		op.Changes = append(op.Changes, "added:"+change.Target)
	}
	return op, nil
}

func restoreMCPAddBackups(manifest SnapshotManifest, existed map[string]bool) {
	for _, target := range manifest.Targets {
		if target.BackupPath != "" {
			_ = copyFile(target.BackupPath, target.Target)
		} else if !existed[target.Target] {
			_ = os.Remove(target.Target)
		}
	}
}

func addMCPEntry(path string, plan model.Plan) error {
	if strings.HasSuffix(strings.ToLower(path), ".toml") {
		return addMCPToTOML(path, plan)
	}
	if strings.EqualFold(filepath.Base(path), "opencode.json") {
		return addMCPToOpenCode(path, plan)
	}
	if isVSCodeMCPConfig(path) {
		return addMCPToVSCode(path, plan)
	}
	return addMCPToJSON(path, plan)
}

func isVSCodeMCPConfig(path string) bool {
	normalized := filepath.ToSlash(path)
	return (strings.Contains(normalized, "/Application Support/Code/") ||
		strings.Contains(normalized, "/Application Support/Code - Insiders/") ||
		strings.Contains(normalized, "/.vscode/")) && strings.HasSuffix(normalized, "/mcp.json")
}

func addMCPToVSCode(path string, plan model.Plan) error {
	root := map[string]any{}
	if raw, err := os.ReadFile(path); err == nil && len(strings.TrimSpace(string(raw))) > 0 {
		if err := json.Unmarshal(raw, &root); err != nil {
			return fmt.Errorf("parse %s: %w", path, err)
		}
	} else if err != nil && !os.IsNotExist(err) {
		return err
	}
	servers, _ := root["servers"].(map[string]any)
	if servers == nil {
		servers = map[string]any{}
	}
	if _, exists := servers[plan.MCPName]; exists {
		return fmt.Errorf("MCP server %s already exists in %s", plan.MCPName, path)
	}
	entry := map[string]any{"type": "stdio"}
	if plan.MCPURL != "" {
		entry["type"] = "http"
	}
	if plan.MCPURL != "" {
		entry["url"] = plan.MCPURL
	} else {
		entry["command"] = plan.Command
		if len(plan.Args) > 0 {
			entry["args"] = plan.Args
		}
		if len(plan.MCPEnv) > 0 {
			entry["env"] = plan.MCPEnv
		}
	}
	servers[plan.MCPName] = entry
	root["servers"] = servers
	output, err := json.MarshalIndent(root, "", "  ")
	if err != nil {
		return err
	}
	return writeMCPConfig(path, append(output, '\n'))
}

func addMCPToJSON(path string, plan model.Plan) error {
	root := map[string]any{}
	if raw, err := os.ReadFile(path); err == nil && len(strings.TrimSpace(string(raw))) > 0 {
		if err := json.Unmarshal(raw, &root); err != nil {
			return fmt.Errorf("parse %s: %w", path, err)
		}
	} else if err != nil && !os.IsNotExist(err) {
		return err
	}
	key := "mcpServers"
	for _, candidate := range []string{"mcpServers", "servers", "mcp"} {
		if _, ok := root[candidate].(map[string]any); ok {
			key = candidate
			break
		}
	}
	servers, _ := root[key].(map[string]any)
	if servers == nil {
		servers = map[string]any{}
	}
	if _, exists := servers[plan.MCPName]; exists {
		return fmt.Errorf("MCP server %s already exists in %s", plan.MCPName, path)
	}
	entry := map[string]any{}
	if plan.MCPURL != "" {
		entry["type"] = "http"
		entry["url"] = plan.MCPURL
	} else {
		entry["command"] = plan.Command
		if len(plan.Args) > 0 {
			entry["args"] = plan.Args
		}
		if len(plan.MCPEnv) > 0 {
			entry["env"] = plan.MCPEnv
		}
	}
	servers[plan.MCPName] = entry
	root[key] = servers
	output, err := json.MarshalIndent(root, "", "  ")
	if err != nil {
		return err
	}
	return writeMCPConfig(path, append(output, '\n'))
}

func addMCPToOpenCode(path string, plan model.Plan) error {
	root := map[string]any{}
	if raw, err := os.ReadFile(path); err == nil && len(strings.TrimSpace(string(raw))) > 0 {
		if err := json.Unmarshal(raw, &root); err != nil {
			return fmt.Errorf("parse %s: %w", path, err)
		}
	} else if err != nil && !os.IsNotExist(err) {
		return err
	}
	servers, _ := root["mcp"].(map[string]any)
	if servers == nil {
		servers = map[string]any{}
	}
	if _, exists := servers[plan.MCPName]; exists {
		return fmt.Errorf("MCP server %s already exists in %s", plan.MCPName, path)
	}
	entry := map[string]any{"enabled": true}
	if plan.MCPURL != "" {
		entry["type"] = "remote"
		entry["url"] = plan.MCPURL
	} else {
		entry["type"] = "local"
		entry["command"] = append([]string{plan.Command}, plan.Args...)
		if len(plan.MCPEnv) > 0 {
			entry["environment"] = plan.MCPEnv
		}
	}
	servers[plan.MCPName] = entry
	root["mcp"] = servers
	output, err := json.MarshalIndent(root, "", "  ")
	if err != nil {
		return err
	}
	return writeMCPConfig(path, append(output, '\n'))
}

func writeMCPConfig(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o600)
}

func tomlMCPKey(name string) string {
	if regexp.MustCompile(`^[A-Za-z0-9_-]+$`).MatchString(name) {
		return name
	}
	return strconv.Quote(name)
}

func addMCPToTOML(path string, plan model.Plan) error {
	raw, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	content := string(raw)
	key := tomlMCPKey(plan.MCPName)
	section := "[mcp_servers." + key + "]"
	unquotedSection := "[mcp_servers." + plan.MCPName + "]"
	if strings.Contains(content, section) || (key != plan.MCPName && strings.Contains(content, unquotedSection)) {
		return fmt.Errorf("MCP server %s already exists in %s", plan.MCPName, path)
	}
	var block strings.Builder
	if content != "" && !strings.HasSuffix(content, "\n") {
		block.WriteString("\n")
	}
	block.WriteString("\n" + section + "\n")
	if plan.MCPURL != "" {
		block.WriteString("url = " + strconv.Quote(plan.MCPURL) + "\n")
	} else {
		block.WriteString("command = " + strconv.Quote(plan.Command) + "\n")
		if len(plan.Args) > 0 {
			quoted := make([]string, 0, len(plan.Args))
			for _, arg := range plan.Args {
				quoted = append(quoted, strconv.Quote(arg))
			}
			block.WriteString("args = [" + strings.Join(quoted, ", ") + "]\n")
		}
		if len(plan.MCPEnv) > 0 {
			block.WriteString("\n[mcp_servers." + key + ".env]\n")
			keys := make([]string, 0, len(plan.MCPEnv))
			for key := range plan.MCPEnv {
				keys = append(keys, key)
			}
			sort.Strings(keys)
			for _, key := range keys {
				block.WriteString(key + " = " + strconv.Quote(plan.MCPEnv[key]) + "\n")
			}
		}
	}
	return writeMCPConfig(path, []byte(content+block.String()))
}
