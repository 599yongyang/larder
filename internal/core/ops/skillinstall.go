package ops

import (
	"archive/tar"
	"archive/zip"
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
		TitleMessage:     planTitle("plan.installSkill.title", "安装 Skill"),
		SummaryMessage:   planMessage("plan.summary.install", "从 {repo} 安装 {name} 到共享库；会先确认目标不存在。", map[string]string{"repo": repo, "name": filepath.Base(target)}),
		SourceURL:        "https://codeload.github.com/" + repo + "/tar.gz/HEAD",
		SourcePath:       skillPath,
		TargetPath:       target,
		RequiresSnapshot: true,
		Executable:       true,
		CreatedAt:        time.Now(),
	}
	plan.Changes = []model.PlanChange{{Action: "install_skill", Source: repo, Target: target, Reason: "来源 " + repo + " · " + skillPath}}
	addWarning(&plan, "plan.warning.installFromHead", "安装会从来源 HEAD 下载并写入共享库；不会自动链接到 Agent 目录。", nil)
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
	if isZipArchive(tarball) {
		if err := extractZipSkill(tarball, plan.SkillSlug, staging); err != nil {
			return op, err
		}
	} else {
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
	op.Message = "已安装到共享库；未自动链接 Agent"
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
		TitleMessage:     planTitle("plan.installSkill.title", "安装 Skill"),
		SummaryMessage:   planMessage("plan.summary.install", "从 {repo} 安装 {name} 到共享库；会先确认目标不存在。", map[string]string{"repo": repo, "name": skillID}),
		SourceURL:        "https://codeload.github.com/" + repo + "/tar.gz/HEAD",
		TargetPath:       target,
		SkillSlug:        skillID,
		RequiresSnapshot: true,
		Executable:       true,
		CreatedAt:        time.Now(),
	}
	plan.Changes = []model.PlanChange{{Action: "install_skill", Source: repo, Target: target, Reason: "来源 " + repo + " · " + skillID}}
	addWarning(&plan, "plan.warning.installFromHead", "安装会从来源 HEAD 下载并写入共享库；不会自动链接到 Agent 目录。", nil)
	return plan, nil
}

func isZipArchive(data []byte) bool {
	return len(data) >= 4 && data[0] == 'P' && data[1] == 'K' && data[2] == 3 && data[3] == 4
}

func extractZipSkill(data []byte, slug, staging string) error {
	reader, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return fmt.Errorf("source is not a zip archive: %w", err)
	}
	folder := ""
	bestRank := 99
	for _, file := range reader.File {
		name := path.Clean(strings.TrimPrefix(file.Name, "./"))
		if path.Base(name) != "SKILL.md" {
			continue
		}
		dir := path.Dir(name)
		rank := 99
		switch {
		case dir == "skills/"+slug || dir == ".agents/skills/"+slug:
			rank = 0
		case dir == slug || path.Base(dir) == slug:
			rank = 1
		case dir == ".":
			rank = 2
		}
		if rank < bestRank {
			folder, bestRank = dir, rank
		}
	}
	if bestRank == 99 {
		return fmt.Errorf("could not find %s/SKILL.md in source archive", slug)
	}
	if err := os.MkdirAll(staging, 0o700); err != nil {
		return err
	}
	for _, file := range reader.File {
		name := path.Clean(strings.TrimPrefix(file.Name, "./"))
		if folder != "" && folder != "." {
			prefix := folder + "/"
			if !strings.HasPrefix(name, prefix) {
				continue
			}
			name = strings.TrimPrefix(name, prefix)
		}
		if name == "." || name == "" || strings.HasPrefix(name, "../") || strings.Contains(name, "/../") {
			continue
		}
		target := filepath.Join(staging, filepath.FromSlash(name))
		rel, err := filepath.Rel(staging, target)
		if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			continue
		}
		if file.FileInfo().IsDir() {
			if err := os.MkdirAll(target, 0o755); err != nil {
				return err
			}
			continue
		}
		if file.Mode()&os.ModeSymlink != 0 {
			continue
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		src, err := file.Open()
		if err != nil {
			return err
		}
		mode := file.Mode().Perm()
		if mode == 0 {
			mode = 0o644
		}
		out, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode)
		if err != nil {
			src.Close()
			return err
		}
		_, copyErr := io.Copy(out, src)
		closeErr := out.Close()
		src.Close()
		if copyErr != nil {
			return copyErr
		}
		if closeErr != nil {
			return closeErr
		}
	}
	return nil
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

// PlanInstallSkillArchive plans an install from a direct tar.gz or zip URL.
// The archive layout is inspected during apply so catalog providers can point
// at a versioned artifact without exposing its internal folder structure.
func PlanInstallSkillArchive(sourceURL, slug, target string) (model.Plan, error) {
	if sourceURL == "" || slug == "" || target == "" {
		return model.Plan{}, errors.New("source, skill and target are required")
	}
	plan := model.Plan{
		ID: "plan:install:" + slug + ":" + fmt.Sprint(time.Now().UnixNano()), Operation: "install_skill",
		Title: "安装 Skill", TitleMessage: planTitle("plan.installSkill.title", "安装 Skill"),
		SummaryMessage: planMessage("plan.summary.install", "从 {repo} 安装 {name} 到共享库；会先确认目标不存在。", map[string]string{"repo": slug, "name": slug}),
		SourceURL:      sourceURL, TargetPath: target, SkillSlug: slug, RequiresSnapshot: true, Executable: true, CreatedAt: time.Now(),
	}
	plan.Changes = []model.PlanChange{{Action: "install_skill", Source: sourceURL, Target: target, Reason: "来源 " + slug}}
	addWarning(&plan, "plan.warning.installFromHead", "安装会从来源下载并写入共享库；不会自动链接到 Agent 目录。", nil)
	return plan, nil
}

// PlanUpdateSkillArchive replaces an existing skill directory from a direct
// archive URL. The previous directory is snapshotted before replacement.
func PlanUpdateSkillArchive(sourceURL, slug, target string) (model.Plan, error) {
	if sourceURL == "" || slug == "" || target == "" {
		return model.Plan{}, errors.New("source, skill and target are required")
	}
	plan := model.Plan{
		ID: "plan:update-skill:" + slug + ":" + fmt.Sprint(time.Now().UnixNano()), Operation: "skill_update",
		Title: "更新 Skill", TitleMessage: planTitle("plan.updateSkill.title", "更新 Skill"),
		SummaryMessage: planMessage("plan.summary.update", "更新 {name}；会先快照旧目录并保留本地修改。", map[string]string{"name": slug}),
		SourceURL:      sourceURL, TargetPath: target, SkillSlug: slug, SkillName: slug, RequiresSnapshot: true, Executable: true, CreatedAt: time.Now(),
	}
	plan.Changes = []model.PlanChange{{Action: "update_skill", Source: sourceURL, Target: target, Reason: "更新 " + slug}}
	addWarning(&plan, "plan.warning.updateSkill", "更新前会快照旧目录，失败时自动恢复。", nil)
	return plan, nil
}

func PlanUninstallSkill(skill model.Skill) (model.Plan, error) {
	if skill.RootKind != model.RootGlobal {
		return model.Plan{}, errors.New("only central library skills can be uninstalled here")
	}
	plan := model.Plan{
		ID:               "plan:uninstall:" + skill.ID,
		Operation:        "uninstall_skill",
		Title:            "卸载 Skill",
		TitleMessage:     planTitle("plan.uninstallSkill.title", "卸载 Skill"),
		SummaryMessage:   planMessage("plan.summary.uninstall", "从共享库移除 {name}；会先快照整个文件夹。", map[string]string{"name": skill.Name}),
		SkillID:          skill.ID,
		SkillName:        skill.Name,
		TargetPath:       skill.Path,
		RequiresSnapshot: true,
		Executable:       true,
		CreatedAt:        time.Now(),
		Impact:           []string{},
	}
	plan.Changes = []model.PlanChange{{Action: "uninstall_skill", Target: skill.Path, Reason: "删除前快照，可回滚"}}
	addWarning(&plan, "plan.warning.removeSharedVersion", "会移除共享版本目录；已链接的 Agent 会变成失效链接，请先确认影响面。", nil)
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
	op.Message = "已从共享库移除；快照可回滚"
	return op, nil
}
