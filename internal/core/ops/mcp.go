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
	"larder/internal/platform"
)

// PlanMCPRemove removes one MCP server definition from selected agent config
// files. It edits only those files and snapshots each one before writing.
func PlanMCPRemove(server model.MCPServer, agentPaths []string) (model.Plan, error) {
	if server.Name == "" || len(agentPaths) == 0 {
		return model.Plan{}, errors.New("mcp server and target configs are required")
	}
	plan := model.Plan{
		ID:               "plan:mcp-remove:" + server.ID,
		Operation:        "mcp_remove",
		Title:            "移除 MCP Server",
		TitleMessage:     planTitle("plan.removeMcp.title", "移除 MCP Server"),
		SummaryMessage:   planMessage("plan.summary.mcpRemove", "从选中的 Agent 配置中移除 {name}；会先备份配置文件。", map[string]string{"name": server.Name}),
		MCPName:          server.Name,
		RequiresSnapshot: true,
		Executable:       true,
		CreatedAt:        time.Now(),
		Impact:           []string{},
	}
	byPath := map[string]model.MCPAgent{}
	for _, agent := range server.Agents {
		byPath[filepath.Clean(agent.Path)] = agent
	}
	for _, path := range agentPaths {
		agent, ok := byPath[filepath.Clean(path)]
		if !ok {
			return model.Plan{}, fmt.Errorf("agent config is not part of this MCP server: %s", path)
		}
		plan.Changes = append(plan.Changes, model.PlanChange{Action: "remove_mcp_entry", RootID: agent.Label, RootLabel: agent.Label, Target: agent.Path, Reason: "从该 Agent 配置中移除 MCP Server"})
	}
	addWarning(&plan, "plan.warning.mcpConfigOnly", "只修改 MCP 配置，不会删除任何已安装的命令或文件。", nil)
	addWarning(&plan, "plan.warning.jsonReformat", "JSON 配置会重新排版；执行前会备份原文件，可在历史中回滚。", nil)
	return plan, nil
}

// ApplyMCPRemove snapshots each config and removes the named server entry.
func ApplyMCPRemove(ctx context.Context, plan model.Plan, snapshotRoot string, filesystem platform.FileSystem) (model.Operation, error) {
	now := time.Now()
	op := model.Operation{ID: fmt.Sprintf("op-%s-%d", plan.ID, now.UnixNano()), PlanID: plan.ID, Operation: plan.Operation, Status: "failed", CreatedAt: now}
	if snapshotRoot == "" || plan.MCPName == "" || len(plan.Changes) == 0 {
		return op, errors.New("invalid MCP removal plan")
	}
	snapshotDir := filepath.Join(snapshotRoot, fmt.Sprintf("mcp-remove-%d", now.UnixNano()))
	if err := os.MkdirAll(snapshotDir, 0o700); err != nil {
		return op, err
	}
	manifest := SnapshotManifest{ID: filepath.Base(snapshotDir), PlanID: plan.ID, CreatedAt: now}
	for i, change := range plan.Changes {
		backup := filepath.Join(snapshotDir, fmt.Sprintf("config-%d.bak", i))
		if err := copyFile(change.Target, backup); err != nil {
			return op, err
		}
		manifest.Targets = append(manifest.Targets, SnapshotTarget{RootID: change.RootID, Target: change.Target, Action: "remove_mcp_entry", Exists: true, BackupPath: backup})
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
		if err := removeMCPEntry(change.Target, plan.MCPName); err != nil {
			restoreMCPBackups(manifest)
			return op, err
		}
	}
	op.Status = "success"
	op.Message = "已从选中的 Agent 配置中移除 " + plan.MCPName + "；配置文件快照可回滚"
	for _, change := range plan.Changes {
		op.Changes = append(op.Changes, "removed:"+change.Target)
	}
	return op, nil
}

func restoreMCPBackups(manifest SnapshotManifest) {
	for _, target := range manifest.Targets {
		if target.BackupPath != "" {
			_ = copyFile(target.BackupPath, target.Target)
		}
	}
}

func copyFile(source, target string) error {
	data, err := os.ReadFile(source)
	if err != nil {
		return err
	}
	info, err := os.Stat(source)
	if err != nil {
		return err
	}
	if err := os.WriteFile(target, data, info.Mode().Perm()); err != nil {
		return err
	}
	return nil
}

func removeMCPEntry(path, name string) error {
	if strings.HasSuffix(strings.ToLower(path), ".toml") {
		return removeMCPFromTOML(path, name)
	}
	var root map[string]any
	raw, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(raw, &root); err != nil {
		return fmt.Errorf("parse %s: %w", path, err)
	}
	changed := false
	for _, key := range []string{"mcpServers", "servers", "mcp"} {
		if servers, ok := root[key].(map[string]any); ok {
			if _, exists := servers[name]; exists {
				delete(servers, name)
				changed = true
			}
		}
	}
	if projects, ok := root["projects"].(map[string]any); ok {
		for _, value := range projects {
			if project, ok := value.(map[string]any); ok {
				if servers, ok := project["mcpServers"].(map[string]any); ok {
					if _, exists := servers[name]; exists {
						delete(servers, name)
						changed = true
					}
				}
			}
		}
	}
	if !changed {
		return fmt.Errorf("mcp server %s not found in %s", name, path)
	}
	output, err := json.MarshalIndent(root, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(output, '\n'), 0o600)
}

func removeMCPFromTOML(path, name string) error {
	raw, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	lines := strings.Split(string(raw), "\n")
	prefix := "[mcp_servers." + name
	var out []string
	skip := false
	removed := false
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "[") && strings.HasSuffix(trimmed, "]") {
			section := strings.Trim(trimmed, "[]")
			skip = section == strings.TrimPrefix(prefix, "[") || strings.HasPrefix(section, strings.TrimPrefix(prefix, "[")+".")
			if skip {
				removed = true
				continue
			}
		}
		if skip {
			continue
		}
		out = append(out, line)
	}
	if !removed {
		return fmt.Errorf("mcp server %s not found in %s", name, path)
	}
	return os.WriteFile(path, []byte(strings.Join(out, "\n")), 0o600)
}
