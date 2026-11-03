package ops

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"larder/internal/core/model"
	"larder/internal/platform"
)

// PlanMCPUpdate edits command/URL/name of an existing MCP server in selected
// agent configs. Environment values are preserved on disk and never read by
// the UI.
func PlanMCPUpdate(server model.MCPServer, newName, command string, args []string, url string, env map[string]string, headers map[string]string, agentPaths []string) (model.Plan, error) {
	newName = strings.TrimSpace(newName)
	if !mcpNamePattern.MatchString(newName) {
		return model.Plan{}, errors.New("invalid MCP server name")
	}
	if command == "" && url == "" {
		return model.Plan{}, errors.New("command or URL is required")
	}
	if len(agentPaths) == 0 {
		return model.Plan{}, errors.New("select at least one agent config")
	}
	byPath := map[string]model.MCPAgent{}
	for _, agent := range server.Agents {
		byPath[filepath.Clean(agent.Path)] = agent
	}
	plan := model.Plan{
		ID:               "plan:mcp-update:" + server.ID + ":" + fmt.Sprint(time.Now().UnixNano()),
		Operation:        "mcp_update",
		Title:            "编辑 MCP Server",
		TitleMessage:     planTitle("plan.editMcp.title", "编辑 MCP Server"),
		SummaryMessage:   planMessage("plan.summary.mcpUpdate", "更新 {name} 的命令或 URL；会先备份配置文件。", map[string]string{"name": server.Name}),
		MCPName:          server.Name,
		MCPNewName:       newName,
		Command:          command,
		Args:             args,
		MCPURL:           url,
		MCPEnv:           env,
		MCPHeaders:       headers,
		RequiresSnapshot: true,
		Executable:       true,
		CreatedAt:        time.Now(),
		Impact:           []string{},
	}
	for _, path := range agentPaths {
		agent, ok := byPath[filepath.Clean(path)]
		if !ok {
			return model.Plan{}, fmt.Errorf("agent config is not part of this MCP server: %s", path)
		}
		plan.Changes = append(plan.Changes, model.PlanChange{Action: "update_mcp_entry", RootID: agent.Label, RootLabel: agent.Label, Target: agent.Path, Reason: "更新该 Agent 的 MCP 配置"})
	}
	addWarning(&plan, "plan.warning.mcpConfigOnly", "只修改 MCP 配置，不会安装命令或启动 Server。", nil)
	addWarning(&plan, "plan.warning.envValuesKept", "现有环境变量的值会保留；执行前会备份配置。", nil)
	return plan, nil
}

func ApplyMCPUpdate(ctx context.Context, plan model.Plan, snapshotRoot string, _ platform.FileSystem) (model.Operation, error) {
	now := time.Now()
	op := model.Operation{ID: fmt.Sprintf("op-%s-%d", plan.ID, now.UnixNano()), PlanID: plan.ID, Operation: plan.Operation, Status: "failed", CreatedAt: now}
	if snapshotRoot == "" || plan.MCPName == "" || plan.MCPNewName == "" || len(plan.Changes) == 0 {
		return op, errors.New("invalid MCP update plan")
	}
	snapshotDir := filepath.Join(snapshotRoot, fmt.Sprintf("mcp-update-%d", now.UnixNano()))
	if err := os.MkdirAll(snapshotDir, 0o700); err != nil {
		return op, err
	}
	manifest := SnapshotManifest{ID: filepath.Base(snapshotDir), PlanID: plan.ID, CreatedAt: now}
	for i, change := range plan.Changes {
		backup := filepath.Join(snapshotDir, fmt.Sprintf("config-%d.bak", i))
		exists := true
		if _, statErr := os.Stat(change.Target); os.IsNotExist(statErr) {
			exists = false
		}
		if exists {
			if err := copyFile(change.Target, backup); err != nil {
				return op, err
			}
		}
		manifest.Targets = append(manifest.Targets, SnapshotTarget{RootID: change.RootID, Target: change.Target, Action: "update_mcp_entry", Exists: exists, BackupPath: backup})
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
		if err := updateMCPEntry(change.Target, plan); err != nil {
			restoreMCPBackups(manifest)
			return op, err
		}
	}
	op.Status = "success"
	op.Message = "已更新 " + plan.MCPName + " 的 MCP 配置；配置文件快照可回滚"
	for _, change := range plan.Changes {
		op.Changes = append(op.Changes, "updated:"+change.Target)
	}
	return op, nil
}

func updateMCPEntry(path string, plan model.Plan) error {
	if strings.HasSuffix(strings.ToLower(path), ".toml") {
		return updateMCPTOML(path, plan)
	}
	if strings.EqualFold(filepath.Base(path), "opencode.json") {
		return updateMCPOpenCode(path, plan)
	}
	if isVSCodeMCPConfig(path) {
		return updateMCPJSON(path, plan, "servers")
	}
	return updateMCPJSON(path, plan, "")
}

func updateMCPJSON(path string, plan model.Plan, fixedKey string) error {
	var root map[string]any
	raw, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	if len(strings.TrimSpace(string(raw))) == 0 {
		raw = []byte("{}")
	}
	if err := json.Unmarshal(raw, &root); err != nil {
		return fmt.Errorf("parse %s: %w", path, err)
	}
	key := fixedKey
	if key == "" {
		for _, candidate := range []string{"mcpServers", "servers", "mcp"} {
			if _, ok := root[candidate].(map[string]any); ok {
				key = candidate
				break
			}
		}
	}
	if key == "" {
		key = "mcpServers"
	}
	servers, _ := root[key].(map[string]any)
	if servers == nil {
		servers = map[string]any{}
	}
	entry, _ := servers[plan.MCPName].(map[string]any)
	if entry == nil {
		entry = map[string]any{}
	}
	delete(servers, plan.MCPName)
	if plan.MCPURL != "" {
		entry["url"] = plan.MCPURL
		entry["type"] = "http"
		delete(entry, "command")
		delete(entry, "args")
	} else {
		entry["command"] = plan.Command
		if len(plan.Args) > 0 {
			entry["args"] = plan.Args
		} else {
			delete(entry, "args")
		}
		delete(entry, "url")
		entry["type"] = "stdio"
	}
	if plan.MCPEnv != nil {
		if len(plan.MCPEnv) > 0 {
			entry["env"] = plan.MCPEnv
		} else {
			delete(entry, "env")
		}
	}
	if plan.MCPHeaders != nil {
		if len(plan.MCPHeaders) > 0 {
			entry["headers"] = plan.MCPHeaders
		} else {
			delete(entry, "headers")
		}
	}
	servers[plan.MCPNewName] = entry
	root[key] = servers
	output, err := json.MarshalIndent(root, "", "  ")
	if err != nil {
		return err
	}
	return writeMCPConfig(path, append(output, '\n'))
}

func updateMCPOpenCode(path string, plan model.Plan) error {
	var root map[string]any
	raw, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	if len(strings.TrimSpace(string(raw))) == 0 {
		raw = []byte("{}")
	}
	if err := json.Unmarshal(raw, &root); err != nil {
		return fmt.Errorf("parse %s: %w", path, err)
	}
	servers, _ := root["mcp"].(map[string]any)
	if servers == nil {
		servers = map[string]any{}
	}
	entry, _ := servers[plan.MCPName].(map[string]any)
	if entry == nil {
		entry = map[string]any{}
	}
	delete(servers, plan.MCPName)
	if plan.MCPURL != "" {
		entry["type"] = "remote"
		entry["url"] = plan.MCPURL
		delete(entry, "command")
		delete(entry, "environment")
	} else {
		entry["type"] = "local"
		entry["command"] = append([]string{plan.Command}, plan.Args...)
		delete(entry, "url")
	}
	if plan.MCPEnv != nil {
		if len(plan.MCPEnv) > 0 {
			entry["environment"] = plan.MCPEnv
		} else {
			delete(entry, "environment")
		}
	}
	if plan.MCPHeaders != nil {
		if len(plan.MCPHeaders) > 0 {
			entry["headers"] = plan.MCPHeaders
		} else {
			delete(entry, "headers")
		}
	}
	servers[plan.MCPNewName] = entry
	root["mcp"] = servers
	output, err := json.MarshalIndent(root, "", "  ")
	if err != nil {
		return err
	}
	return writeMCPConfig(path, append(output, '\n'))
}

func updateMCPTOML(path string, plan model.Plan) error {
	raw, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	lines := strings.Split(string(raw), "\n")
	oldKey, newKey := tomlMCPKey(plan.MCPName), tomlMCPKey(plan.MCPNewName)
	var out []string
	inMain := false
	inEnv := false
	found := false
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "[") && strings.HasSuffix(trimmed, "]") {
			section := strings.Trim(trimmed, "[]")
			mainOld := "mcp_servers." + oldKey
			mainNew := "mcp_servers." + newKey
			switch {
			case section == mainOld:
				inMain, inEnv, found = true, false, true
				out = append(out, "["+mainNew+"]")
				if plan.MCPURL != "" {
					out = append(out, "url = "+strconv.Quote(plan.MCPURL))
				} else {
					out = append(out, "command = "+strconv.Quote(plan.Command))
					if len(plan.Args) > 0 {
						quoted := make([]string, 0, len(plan.Args))
						for _, arg := range plan.Args {
							quoted = append(quoted, strconv.Quote(arg))
						}
						out = append(out, "args = ["+strings.Join(quoted, ", ")+"]")
					}
				}
				continue
			case section == mainOld+".env":
				inMain, inEnv = false, true
				out = append(out, "["+mainNew+".env]")
				continue
			default:
				inMain, inEnv = false, false
			}
		}
		if inMain {
			key, _, _ := strings.Cut(trimmed, "=")
			key = strings.TrimSpace(key)
			if key == "command" || key == "args" || key == "url" {
				continue
			}
		}
		_ = inEnv
		out = append(out, line)
	}
	if !found {
		content := strings.Join(out, "\n")
		if content != "" && !strings.HasSuffix(content, "\n") {
			content += "\n"
		}
		content += "\n[mcp_servers." + newKey + "]\n"
		if plan.MCPURL != "" {
			content += "url = " + strconv.Quote(plan.MCPURL) + "\n"
		} else {
			content += "command = " + strconv.Quote(plan.Command) + "\n"
			if len(plan.Args) > 0 {
				quoted := make([]string, 0, len(plan.Args))
				for _, arg := range plan.Args {
					quoted = append(quoted, strconv.Quote(arg))
				}
				content += "args = [" + strings.Join(quoted, ", ") + "]\n"
			}
		}
		if plan.MCPEnv != nil {
			content = replaceTOMLSection(content, "mcp_servers."+newKey+".env", plan.MCPEnv)
		}
		if plan.MCPHeaders != nil {
			content = replaceTOMLSection(content, "mcp_servers."+newKey+".http_headers", plan.MCPHeaders)
		}
		return writeMCPConfig(path, []byte(content))
	}
	content := strings.Join(out, "\n")
	if plan.MCPEnv != nil {
		content = replaceTOMLSection(content, "mcp_servers."+newKey+".env", plan.MCPEnv)
	}
	if plan.MCPHeaders != nil {
		content = replaceTOMLSection(content, "mcp_servers."+newKey+".http_headers", plan.MCPHeaders)
	}
	return writeMCPConfig(path, []byte(content))
}

func replaceTOMLSection(content, section string, values map[string]string) string {
	lines := strings.Split(content, "\n")
	out := make([]string, 0, len(lines)+len(values)+2)
	skipping := false
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "[") && strings.HasSuffix(trimmed, "]") {
			if strings.Trim(trimmed, "[]") == section {
				skipping = true
				continue
			}
			skipping = false
		}
		if !skipping {
			out = append(out, line)
		}
	}
	if len(values) == 0 {
		return strings.Join(out, "\n")
	}
	if len(out) > 0 && strings.TrimSpace(out[len(out)-1]) != "" {
		out = append(out, "")
	}
	out = append(out, "["+section+"]")
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		out = append(out, key+" = "+strconv.Quote(values[key]))
	}
	return strings.Join(out, "\n")
}
