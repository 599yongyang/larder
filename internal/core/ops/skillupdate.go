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
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"larder/internal/core/model"
	"larder/internal/platform"
)

// PlanSkillUpdate prepares a source-based Skill folder replacement. The
// snapshot and the actual replacement happen only after confirmation.
func PlanSkillUpdate(skill model.Skill, repo string) (model.Plan, error) {
	if !skill.Locked || skill.SourceType != "github" || repo == "" {
		return model.Plan{}, errors.New("skill has no GitHub source")
	}
	skillPath := strings.TrimSpace(skill.SkillPath)
	if skillPath == "" {
		skillPath = "SKILL.md"
	}
	plan := model.Plan{
		ID:               "plan:skill-update:" + skill.ID,
		Operation:        "skill_update",
		Title:            "更新 Skill",
		TitleMessage:     planTitle("plan.updateSkill.title", "更新 Skill"),
		SummaryMessage:   planMessage("plan.summary.skillUpdate", "用 {repo} 的来源 HEAD 替换本地 {name}；会先快照。", map[string]string{"repo": repo, "name": skill.Name}),
		SkillID:          skill.ID,
		SkillName:        skill.Name,
		SourceURL:        "https://codeload.github.com/" + repo + "/tar.gz/HEAD",
		SourcePath:       skillPath,
		TargetPath:       skill.Path,
		RequiresSnapshot: true,
		Executable:       true,
		CreatedAt:        time.Now(),
	}
	addWarning(&plan, "plan.warning.overwriteLocalSkill", "会用来源 HEAD 覆盖本地 Skill 文件夹；本地未提交修改请先确认。", nil)
	plan.Changes = append(plan.Changes, model.PlanChange{Action: "update_skill", Source: repo, Target: skill.Path, Reason: "来源 " + repo + " · " + skillPath})
	return plan, nil
}

// ApplySkillUpdate downloads the pinned source URL, snapshots the local folder,
// then atomically replaces its files. A failure restores the snapshot.
func ApplySkillUpdate(ctx context.Context, plan model.Plan, snapshotRoot string, fetcher platform.Fetcher, filesystem platform.FileSystem) (model.Operation, error) {
	now := time.Now()
	op := model.Operation{ID: fmt.Sprintf("op-%s-%d", plan.ID, now.UnixNano()), PlanID: plan.ID, Operation: plan.Operation, Status: "failed", CreatedAt: now}
	if filesystem == nil {
		filesystem = platform.OSFileSystem{}
	}
	if fetcher == nil {
		fetcher = platform.HTTPFetcher{}
	}
	if snapshotRoot == "" {
		return op, errors.New("snapshot root is not configured")
	}
	if plan.SourceURL == "" || plan.TargetPath == "" {
		return op, errors.New("invalid skill update plan")
	}
	info, err := filesystem.Stat(plan.TargetPath)
	if err != nil || !info.IsDir() {
		return op, errors.New("local skill folder is missing")
	}
	snapshotDir := filepath.Join(snapshotRoot, fmt.Sprintf("skill-%d", now.UnixNano()))
	staging := filepath.Join(snapshotDir, "incoming")
	backup := filepath.Join(snapshotDir, "backup")
	if err := os.MkdirAll(snapshotDir, 0o700); err != nil {
		return op, err
	}
	if err := copyTree(plan.TargetPath, backup); err != nil {
		return op, fmt.Errorf("snapshot skill: %w", err)
	}
	manifest := SnapshotManifest{ID: filepath.Base(snapshotDir), PlanID: plan.ID, CreatedAt: now, Targets: []SnapshotTarget{{RootID: "skill", Target: plan.TargetPath, Action: "update_skill", Exists: true, BackupPath: backup}}}
	raw, _ := json.MarshalIndent(manifest, "", "  ")
	if err := os.WriteFile(filepath.Join(snapshotDir, "manifest.json"), raw, 0o600); err != nil {
		return op, err
	}
	op.SnapshotPath = snapshotDir
	tarball, err := fetcher.Fetch(ctx, plan.SourceURL)
	if err != nil {
		return op, fmt.Errorf("download source: %w", err)
	}
	if err := extractSkillFolder(tarball, plan.SourcePath, staging); err != nil {
		return op, err
	}
	if err := os.RemoveAll(plan.TargetPath); err != nil {
		_ = copyTree(backup, plan.TargetPath)
		return op, err
	}
	if err := copyTree(staging, plan.TargetPath); err != nil {
		_ = os.RemoveAll(plan.TargetPath)
		_ = copyTree(backup, plan.TargetPath)
		return op, err
	}
	if info, err := filesystem.Stat(plan.TargetPath); err != nil || !info.IsDir() {
		_ = os.RemoveAll(plan.TargetPath)
		_ = copyTree(backup, plan.TargetPath)
		return op, errors.New("updated skill folder is missing")
	}
	_ = os.RemoveAll(staging)
	op.Status = "success"
	op.Changes = []string{"updated:" + plan.TargetPath}
	op.Message = "已从来源 HEAD 更新 Skill；快照已保存"
	return op, nil
}

func GitHubRepo(skill model.Skill) string {
	value := strings.TrimSpace(skill.SourceURL)
	value = strings.TrimPrefix(value, "https://github.com/")
	value = strings.TrimPrefix(value, "http://github.com/")
	value = strings.TrimSuffix(value, ".git")
	if value == "" {
		value = strings.TrimSpace(skill.Source)
	}
	if !strings.Contains(value, "/") {
		return ""
	}
	return value
}

func extractSkillFolder(tarball []byte, skillPath, target string) error {
	gz, err := gzip.NewReader(bytes.NewReader(tarball))
	if err != nil {
		return fmt.Errorf("source is not a gzip archive: %w", err)
	}
	defer gz.Close()
	tr := tar.NewReader(gz)
	root := ""
	dir := path.Dir(filepath.ToSlash(skillPath))
	if dir == "." {
		dir = ""
	}
	found := false
	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		if header.Typeflag != tar.TypeReg && header.Typeflag != tar.TypeRegA {
			continue
		}
		name := path.Clean(strings.TrimPrefix(header.Name, "./"))
		if root == "" {
			root = strings.SplitN(name, "/", 2)[0]
		}
		if !strings.HasPrefix(name, root+"/") {
			continue
		}
		rel := strings.TrimPrefix(name, root+"/")
		if dir != "" && !strings.HasPrefix(rel, dir+"/") {
			continue
		}
		inside := rel
		if dir != "" {
			inside = strings.TrimPrefix(rel, dir+"/")
		}
		if inside == "" {
			continue
		}
		cleanInside := path.Clean(inside)
		if cleanInside == ".." || strings.HasPrefix(cleanInside, "../") || path.IsAbs(cleanInside) {
			return fmt.Errorf("unsafe path in source archive: %s", header.Name)
		}
		inside = cleanInside
		found = true
		destination := filepath.Join(target, filepath.FromSlash(inside))
		if err := os.MkdirAll(filepath.Dir(destination), 0o700); err != nil {
			return err
		}
		content, err := io.ReadAll(io.LimitReader(tr, 16<<20))
		if err != nil {
			return err
		}
		if err := os.WriteFile(destination, content, fs.FileMode(header.Mode)&0o777); err != nil {
			return err
		}
	}
	if !found {
		return errors.New("skill folder not found in source archive")
	}
	return nil
}
