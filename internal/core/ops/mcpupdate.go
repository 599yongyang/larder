package ops

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"larder/internal/core/model"
	"larder/internal/platform"
)

// PlanMCPUpdate edits command/URL/name of an existing MCP server in selected
// agent configs. Environment values are preserved on disk and never read by
// the UI.
func PlanMCPUpdate(server model.MCPServer, newName, command string, args []string, url string, agentPaths []string) (model.Plan, error) {
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
		if err := copyFile(change.Target, backup); err != nil {
			return op, err
		}
		manifest.Targets = append(manifest.Targets, SnapshotTarget{RootID: change.RootID, Target: change.Target, Action: "update_mcp_entry", Exists: true, BackupPath: backup})
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
	if err != nil {
		return err
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
	servers, _ := root[key].(map[string]any)
	entry, _ := servers[plan.MCPName].(map[string]any)
	if entry == nil {
		return fmt.Errorf("MCP server %s not found in %s", plan.MCPName, path)
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
	if err != nil {
		return err
	}
	if err := json.Unmarshal(raw, &root); err != nil {
		return fmt.Errorf("parse %s: %w", path, err)
	}
	servers, _ := root["mcp"].(map[string]any)
	entry, _ := servers[plan.MCPName].(map[string]any)
	if entry == nil {
		return fmt.Errorf("MCP server %s not found in %s", plan.MCPName, path)
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
	if err != nil {
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
		return fmt.Errorf("MCP server %s not found in %s", plan.MCPName, path)
	}
	return writeMCPConfig(path, []byte(strings.Join(out, "\n")))
}
