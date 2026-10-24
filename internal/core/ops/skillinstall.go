package ops

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"larder/internal/core/model"
	"larder/internal/platform"
)

func PlanInstallSkill(repo, skillPath, target string) (model.Plan, error) {
	if repo == "" || skillPath == "" || target == "" {
		return model.Plan{}, errors.New("repo, skill path and target are required")
	}
	plan := model.Plan{
		ID:               "plan:install:" + repo + ":" + skillPath,
		Operation:        "install_skill",
		Title:            "安装 Skill",
		Summary:          fmt.Sprintf("从 %s 安装 %s 到中央库；会先确认目标不存在。", repo, filepath.Base(target)),
		SourceURL:        "https://codeload.github.com/" + repo + "/tar.gz/HEAD",
		SourcePath:       skillPath,
		TargetPath:       target,
		RequiresSnapshot: true,
		Executable:       true,
		CreatedAt:        time.Now(),
	}
	plan.Changes = []model.PlanChange{{Action: "install_skill", Source: repo, Target: target, Reason: "来源 " + repo + " · " + skillPath}}
	plan.Warnings = append(plan.Warnings, "安装会从来源 HEAD 下载并写入中央库；不会自动链接到 Agent 目录。")
	return plan, nil
}

func ApplyInstallSkill(ctx context.Context, plan model.Plan, snapshotRoot string, fetcher platform.Fetcher, filesystem platform.FileSystem) (model.Operation, error) {
	now := time.Now()
	op := model.Operation{ID: fmt.Sprintf("op-%s-%d", plan.ID, now.UnixNano()), PlanID: plan.ID, Operation: plan.Operation, Status: "failed", CreatedAt: now}
	if filesystem == nil {
		filesystem = platform.OSFileSystem{}
	}
	if fetcher == nil {
		fetcher = platform.HTTPFetcher{}
	}
	if snapshotRoot == "" || plan.TargetPath == "" || plan.SourceURL == "" {
		return op, errors.New("invalid install plan")
	}
	if _, err := filesystem.Lstat(plan.TargetPath); !errors.Is(err, os.ErrNotExist) {
		return op, fmt.Errorf("install target already exists: %s", plan.TargetPath)
	}
	snapshotDir := filepath.Join(snapshotRoot, fmt.Sprintf("install-%d", now.UnixNano()))
	staging := filepath.Join(snapshotDir, "incoming")
	if err := os.MkdirAll(snapshotDir, 0o700); err != nil {
		return op, err
	}
	manifest := SnapshotManifest{ID: filepath.Base(snapshotDir), PlanID: plan.ID, CreatedAt: now, Targets: []SnapshotTarget{{RootID: "skill", Target: plan.TargetPath, Action: "install_skill"}}}
	raw, _ := json.MarshalIndent(manifest, "", "  ")
	if err := os.WriteFile(filepath.Join(snapshotDir, "manifest.json"), raw, 0o600); err != nil {
		return op, err
	}
	op.SnapshotPath = snapshotDir
	tarball, err := fetcher.Fetch(ctx, plan.SourceURL)
	if err != nil {
		return op, fmt.Errorf("download source: %w", err)
	}
	sourcePath := plan.SourcePath
	if sourcePath == "" && plan.SkillSlug != "" {
		dir, err := locateSkillFolder(tarball, plan.SkillSlug)
		if err != nil {
			return op, err
		}
		sourcePath = path.Join(dir, "SKILL.md")
	}
	if err := extractSkillFolder(tarball, sourcePath, staging); err != nil {
		return op, err
	}
	if _, err := os.Stat(filepath.Join(staging, "SKILL.md")); err != nil {
		return op, errors.New("source has no SKILL.md")
	}
	if err := copyTree(staging, plan.TargetPath); err != nil {
		_ = os.RemoveAll(plan.TargetPath)
		return op, err
	}
	if info, err := filesystem.Stat(plan.TargetPath); err != nil || !info.IsDir() {
		_ = os.RemoveAll(plan.TargetPath)
		return op, errors.New("installed folder is missing")
	}
	_ = os.RemoveAll(staging)
	op.Status = "success"
	op.Changes = []string{"installed:" + plan.TargetPath}
	op.Message = "已安装到中央库；未自动链接 Agent"
	return op, nil
}

// PlanInstallCatalogSkill plans an install from a skills.sh catalog entry.
// The exact SKILL.md path is not known until the repository archive is
// downloaded, so the archive is searched for the skill's folder on apply.
func PlanInstallCatalogSkill(repo, skillID, target string) (model.Plan, error) {
	if repo == "" || skillID == "" || target == "" {
		return model.Plan{}, errors.New("repo, skill and target are required")
	}
	plan := model.Plan{
		ID:               "plan:install:" + repo + ":" + skillID,
		Operation:        "install_skill",
		Title:            "安装 Skill",
		Summary:          fmt.Sprintf("从 %s 安装 %s 到中央库；会先确认目标不存在。", repo, skillID),
		SourceURL:        "https://codeload.github.com/" + repo + "/tar.gz/HEAD",
		TargetPath:       target,
		SkillSlug:        skillID,
		RequiresSnapshot: true,
		Executable:       true,
		CreatedAt:        time.Now(),
	}
	plan.Changes = []model.PlanChange{{Action: "install_skill", Source: repo, Target: target, Reason: "来源 " + repo + " · " + skillID}}
	plan.Warnings = append(plan.Warnings, "安装会从来源 HEAD 下载并写入中央库；不会自动链接到 Agent 目录。")
	return plan, nil
}

func locateSkillFolder(tarball []byte, skillID string) (string, error) {
	gz, err := gzip.NewReader(bytes.NewReader(tarball))
	if err != nil {
		return "", fmt.Errorf("source is not a gzip archive: %w", err)
	}
	defer gz.Close()
	tr := tar.NewReader(gz)
	best := ""
	bestRank := 99
	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", err
		}
		if header.Typeflag != tar.TypeReg && header.Typeflag != tar.TypeRegA {
			continue
		}
		name := path.Clean(strings.TrimPrefix(header.Name, "./"))
		parts := strings.SplitN(name, "/", 2)
		if len(parts) < 2 {
			continue
		}
		relative := parts[1]
		if path.Base(relative) != "SKILL.md" {
			continue
		}
		dir := path.Dir(relative)
		rank := 99
		switch {
		case dir == "skills/"+skillID:
			rank = 0
		case dir == ".agents/skills/"+skillID:
			rank = 1
		case dir == skillID:
			rank = 2
		case path.Base(dir) == skillID:
			rank = 3
		case dir == ".":
			rank = 4
		}
		if rank < bestRank {
			best, bestRank = dir, rank
		}
	}
	if bestRank == 99 {
		return "", fmt.Errorf("could not find %s/SKILL.md in source archive", skillID)
	}
	if best == "." {
		return "", nil
	}
	return best, nil
}

func PlanUninstallSkill(skill model.Skill) (model.Plan, error) {
	if skill.RootKind != model.RootGlobal {
		return model.Plan{}, errors.New("only central library skills can be uninstalled here")
	}
	plan := model.Plan{
		ID:               "plan:uninstall:" + skill.ID,
		Operation:        "uninstall_skill",
		Title:            "卸载 Skill",
		Summary:          fmt.Sprintf("从中央库移除 %s；会先快照整个文件夹。", skill.Name),
		SkillID:          skill.ID,
		SkillName:        skill.Name,
		TargetPath:       skill.Path,
		RequiresSnapshot: true,
		Executable:       true,
		CreatedAt:        time.Now(),
		Impact:           []string{},
	}
	plan.Changes = []model.PlanChange{{Action: "uninstall_skill", Target: skill.Path, Reason: "删除前快照，可回滚"}}
	plan.Warnings = append(plan.Warnings, "会移除共享版本目录；已链接的 Agent 会变成失效链接，请先确认影响面。")
	return plan, nil
}

func ApplyUninstallSkill(ctx context.Context, plan model.Plan, snapshotRoot string, filesystem platform.FileSystem) (model.Operation, error) {
	now := time.Now()
	op := model.Operation{ID: fmt.Sprintf("op-%s-%d", plan.ID, now.UnixNano()), PlanID: plan.ID, Operation: plan.Operation, Status: "failed", CreatedAt: now}
	if filesystem == nil {
		filesystem = platform.OSFileSystem{}
	}
	if snapshotRoot == "" || plan.TargetPath == "" {
		return op, errors.New("invalid uninstall plan")
	}
	info, err := filesystem.Stat(plan.TargetPath)
	if err != nil || !info.IsDir() {
		return op, errors.New("skill folder is missing")
	}
	snapshotDir := filepath.Join(snapshotRoot, fmt.Sprintf("uninstall-%d", now.UnixNano()))
	backup := filepath.Join(snapshotDir, "backup")
	if err := os.MkdirAll(snapshotDir, 0o700); err != nil {
		return op, err
	}
	if err := copyTree(plan.TargetPath, backup); err != nil {
		return op, err
	}
	manifest := SnapshotManifest{ID: filepath.Base(snapshotDir), PlanID: plan.ID, CreatedAt: now, Targets: []SnapshotTarget{{RootID: "skill", Target: plan.TargetPath, Action: "uninstall_skill", Exists: true, BackupPath: backup}}}
	raw, _ := json.MarshalIndent(manifest, "", "  ")
	if err := os.WriteFile(filepath.Join(snapshotDir, "manifest.json"), raw, 0o600); err != nil {
		return op, err
	}
	op.SnapshotPath = snapshotDir
	if err := os.RemoveAll(plan.TargetPath); err != nil {
		return op, err
	}
	if _, err := filesystem.Lstat(plan.TargetPath); !errors.Is(err, os.ErrNotExist) {
		return op, errors.New("uninstall did not remove the folder")
	}
	op.Status = "success"
	op.Changes = []string{"uninstalled:" + plan.TargetPath}
	op.Message = "已从中央库移除；快照可回滚"
	return op, nil
}
