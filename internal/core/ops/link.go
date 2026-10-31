// Package ops builds write-operation plans without performing any writes.
package ops

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	iofs "io/fs"
	"path/filepath"
	"time"

	"larder/internal/core/model"
	"larder/internal/platform"
)

type LinkTarget struct {
	RootID    string
	RootLabel string
	Path      string
}

type LinkRequest struct {
	SkillID    string
	SkillName  string
	SourcePath string
	Targets    []LinkTarget
}

// PlanLink is a dry-run only. It resolves target state and produces the exact
// changes a future executor would need; it never creates, removes or modifies.
func PlanLink(req LinkRequest, filesystem platform.FileSystem) (model.Plan, error) {
	if filesystem == nil {
		filesystem = platform.OSFileSystem{}
	}
	if req.SkillName == "" || req.SourcePath == "" {
		return model.Plan{}, errors.New("skill source is required")
	}
	info, err := filesystem.Stat(req.SourcePath)
	if err != nil || !info.IsDir() {
		return model.Plan{}, fmt.Errorf("skill source is not a directory: %s", req.SourcePath)
	}
	if len(req.Targets) == 0 {
		return model.Plan{}, errors.New("no target agent directories")
	}
	plan := model.Plan{
		ID:               planID(req),
		Operation:        "link_skill",
		Title:            "修复链接",
		TitleMessage:     planTitle("plan.repairLinks.title", "修复链接"),
		SkillID:          req.SkillID,
		SkillName:        req.SkillName,
		RequiresSnapshot: true,
		Executable:       true,
		CreatedAt:        time.Now(),
	}
	for _, target := range req.Targets {
		change := model.PlanChange{
			Action: "create_symlink", RootID: target.RootID, RootLabel: target.RootLabel,
			Source: req.SourcePath, Target: target.Path,
		}
		_, statErr := filesystem.Lstat(target.Path)
		switch {
		case errors.Is(statErr, iofs.ErrNotExist):
			change.Reason = "目标不存在，可创建链接"
		case statErr != nil:
			change.Action = "error"
			change.Reason = statErr.Error()
			addWarning(&plan, "plan.warning.targetUnreadable", fmt.Sprintf("%s：无法读取目标 %s", target.RootLabel, statErr), map[string]string{"root": target.RootLabel})
		default:
			change.Exists = true
			if link, isLink := linkTargetOf(filesystem, target.Path); isLink {
				if !filepath.IsAbs(link) {
					link = filepath.Join(filepath.Dir(target.Path), link)
				}
				if filepath.Clean(link) == filepath.Clean(req.SourcePath) {
					change.Action = "already_linked"
					change.Reason = "已经指向该 Skill"
				} else if _, statErr := filesystem.Stat(filepath.Clean(link)); statErr != nil {
					change.Action = "replace_link"
					change.Reason = "现有链接已失效，可以修复"
					addWarning(&plan, "plan.warning.existingLinkBroken", fmt.Sprintf("%s：现有链接已失效，修复时会替换它", target.RootLabel), map[string]string{"root": target.RootLabel})
				} else {
					change.Action = "conflict"
					change.Reason = "已有有效链接指向其他位置，不自动替换"
					addWarning(&plan, "plan.warning.validLinkKept", fmt.Sprintf("%s：目标已有有效链接，不会覆盖", target.RootLabel), map[string]string{"root": target.RootLabel})
				}
			} else {
				change.Action = "conflict"
				change.Reason = "目标路径已存在且不是链接"
				addWarning(&plan, "plan.warning.targetExists", fmt.Sprintf("%s：目标已存在，不会覆盖", target.RootLabel), map[string]string{"root": target.RootLabel})
			}
		}
		plan.Changes = append(plan.Changes, change)
		plan.Impact = append(plan.Impact, target.RootLabel+" · "+target.Path)
	}
	created, repaired := 0, 0
	for _, change := range plan.Changes {
		switch change.Action {
		case "create_symlink":
			created++
		case "replace_link":
			repaired++
		}
	}
	switch {
	case created > 0 && repaired == 0:
		plan.Title = "添加可见"
		plan.Summary = fmt.Sprintf("让 %s 对 %d 个 Agent 可见；确认后才会创建链接。", req.SkillName, created)
	case repaired > 0 && created == 0:
		plan.Title = "修复链接"
		plan.Summary = fmt.Sprintf("修复 %s 在 %d 个 Agent 中的失效链接；确认后才会执行。", req.SkillName, repaired)
	case created > 0 || repaired > 0:
		plan.Title = "调整可见性"
		plan.Summary = fmt.Sprintf("为 %s 添加 %d 个链接、修复 %d 个失效链接；确认后才会执行。", req.SkillName, created, repaired)
	}
	addWarning(&plan, "plan.warning.snapshotRepairOnly", "执行前会先写快照；只创建缺失链接或修复失效链接，不覆盖正常链接。", nil)
	return plan, nil
}

// PlanDisableLink removes only a valid symlink that points at the canonical
// Skill. It never removes a real directory, file or foreign link.
func PlanDisableLink(req LinkRequest, filesystem platform.FileSystem) (model.Plan, error) {
	if filesystem == nil {
		filesystem = platform.OSFileSystem{}
	}
	if req.SkillName == "" || req.SourcePath == "" {
		return model.Plan{}, errors.New("skill source is required")
	}
	if info, err := filesystem.Stat(req.SourcePath); err != nil || !info.IsDir() {
		return model.Plan{}, fmt.Errorf("skill source is not a directory: %s", req.SourcePath)
	}
	if len(req.Targets) == 0 {
		return model.Plan{}, errors.New("no target agent directories")
	}
	plan := model.Plan{
		ID:               planID(LinkRequest{SkillID: "disable:" + req.SkillID, SkillName: req.SkillName, SourcePath: req.SourcePath, Targets: req.Targets}),
		Operation:        "disable_link",
		Title:            "停用可见性",
		TitleMessage:     planTitle("plan.disableVisibility.title", "停用可见性"),
		SkillID:          req.SkillID,
		SkillName:        req.SkillName,
		RequiresSnapshot: true,
		Executable:       true,
		CreatedAt:        time.Now(),
	}
	for _, target := range req.Targets {
		if _, err := filesystem.Lstat(target.Path); err != nil {
			return model.Plan{}, fmt.Errorf("target is missing: %s", target.Path)
		}
		link, isLink := linkTargetOf(filesystem, target.Path)
		if !isLink {
			return model.Plan{}, fmt.Errorf("target is not a link: %s", target.Path)
		}
		if !filepath.IsAbs(link) {
			link = filepath.Join(filepath.Dir(target.Path), link)
		}
		if filepath.Clean(link) != filepath.Clean(req.SourcePath) {
			return model.Plan{}, fmt.Errorf("link does not point at this skill: %s", target.Path)
		}
		plan.Changes = append(plan.Changes, model.PlanChange{Action: "remove_link", RootID: target.RootID, RootLabel: target.RootLabel, Source: req.SourcePath, Target: target.Path, Exists: true, Reason: "只移除这个指向共享版本的 symlink"})
		plan.Impact = append(plan.Impact, target.RootLabel+" · "+target.Path)
	}
	plan.Summary = fmt.Sprintf("从 %d 个 Agent 目录移除 %s 的链接；不会删除共享版本或副本。", len(req.Targets), req.SkillName)
	addWarning(&plan, "plan.warning.snapshotRollback", "执行前会快照现有链接；回滚可以恢复。", nil)
	return plan, nil
}

// PlanSyncCopy replaces an agent-local copy with the canonical source content.
// The existing copy is snapshotted before it is removed.
func PlanSyncCopy(req LinkRequest, filesystem platform.FileSystem) (model.Plan, error) {
	if filesystem == nil {
		filesystem = platform.OSFileSystem{}
	}
	if req.SkillName == "" || req.SourcePath == "" {
		return model.Plan{}, errors.New("skill source is required")
	}
	if info, err := filesystem.Stat(req.SourcePath); err != nil || !info.IsDir() {
		return model.Plan{}, fmt.Errorf("skill source is not a directory: %s", req.SourcePath)
	}
	if len(req.Targets) == 0 {
		return model.Plan{}, errors.New("no target agent directories")
	}
	plan := model.Plan{
		ID:               planID(LinkRequest{SkillID: "sync:" + req.SkillID, SkillName: req.SkillName, SourcePath: req.SourcePath, Targets: req.Targets}),
		Operation:        "sync_copy",
		Title:            "同步内容",
		TitleMessage:     planTitle("plan.syncContent.title", "同步内容"),
		SkillID:          req.SkillID,
		SkillName:        req.SkillName,
		RequiresSnapshot: true,
		Executable:       true,
		CreatedAt:        time.Now(),
	}
	for _, target := range req.Targets {
		info, err := filesystem.Lstat(target.Path)
		if err != nil || !info.IsDir() || info.Mode()&iofs.ModeSymlink != 0 {
			return model.Plan{}, fmt.Errorf("target is not a directory copy: %s", target.Path)
		}
		plan.Changes = append(plan.Changes, model.PlanChange{Action: "sync_copy", RootID: target.RootID, RootLabel: target.RootLabel, Source: req.SourcePath, Target: target.Path, Exists: true, Reason: "复制一份快照，然后用共享版本覆盖副本"})
		plan.Impact = append(plan.Impact, target.RootLabel+" · "+target.Path)
		addWarning(&plan, "plan.warning.copyReplaced", fmt.Sprintf("%s：副本会被共享版本内容替换", target.RootLabel), map[string]string{"root": target.RootLabel})
	}
	plan.Summary = fmt.Sprintf("把 %s 的 %d 个副本同步为共享版本内容；确认后才会执行。", req.SkillName, len(req.Targets))
	addWarning(&plan, "plan.warning.snapshotNoShared", "执行前会先写快照；不会创建或删除共享版本。", nil)
	return plan, nil
}

// PlanCopyToLink replaces an agent-local copy with a symlink to the canonical
// source. The copy is snapshotted before any removal.
func PlanCopyToLink(req LinkRequest, filesystem platform.FileSystem) (model.Plan, error) {
	if filesystem == nil {
		filesystem = platform.OSFileSystem{}
	}
	if req.SkillName == "" || req.SourcePath == "" {
		return model.Plan{}, errors.New("skill source is required")
	}
	info, err := filesystem.Stat(req.SourcePath)
	if err != nil || !info.IsDir() {
		return model.Plan{}, fmt.Errorf("skill source is not a directory: %s", req.SourcePath)
	}
	if len(req.Targets) == 0 {
		return model.Plan{}, errors.New("no target agent directories")
	}
	plan := model.Plan{
		ID:               planID(LinkRequest{SkillID: "copy:" + req.SkillID, SkillName: req.SkillName, SourcePath: req.SourcePath, Targets: req.Targets}),
		Operation:        "replace_copy",
		Title:            "改用共享版本",
		TitleMessage:     planTitle("plan.useSharedVersion.title", "改用共享版本"),
		SkillID:          req.SkillID,
		SkillName:        req.SkillName,
		RequiresSnapshot: true,
		Executable:       true,
		CreatedAt:        time.Now(),
	}
	for _, target := range req.Targets {
		change := model.PlanChange{Action: "replace_copy", RootID: target.RootID, RootLabel: target.RootLabel, Source: req.SourcePath, Target: target.Path}
		info, err := filesystem.Lstat(target.Path)
		_, isLink := linkTargetOf(filesystem, target.Path)
		switch {
		case err != nil:
			return model.Plan{}, fmt.Errorf("target copy is missing: %s", target.Path)
		case isLink:
			return model.Plan{}, fmt.Errorf("target is already a link: %s", target.Path)
		case !info.IsDir():
			return model.Plan{}, fmt.Errorf("target is not a directory copy: %s", target.Path)
		}
		change.Exists = true
		change.Reason = "目录副本将被快照，然后替换为 symlink"
		plan.Changes = append(plan.Changes, change)
		plan.Impact = append(plan.Impact, target.RootLabel+" · "+target.Path)
		addWarning(&plan, "plan.warning.copyReplacedWithLink", fmt.Sprintf("%s：副本会被替换为链接，执行前会快照", target.RootLabel), map[string]string{"root": target.RootLabel})
	}
	plan.Summary = fmt.Sprintf("把 %s 的 %d 个副本替换为指向共享版本的链接；确认后才会执行写入。", req.SkillName, len(req.Targets))
	addWarning(&plan, "plan.warning.snapshotReplaceCopy", "执行前必须生成快照；只会替换目录副本，不覆盖有效链接。", nil)
	return plan, nil
}

func planID(req LinkRequest) string {
	h := sha256.New()
	fmt.Fprintf(h, "%s\x00%s\x00%s", req.SkillID, req.SkillName, req.SourcePath)
	for _, target := range req.Targets {
		fmt.Fprintf(h, "\x00%s\x00%s", target.RootID, target.Path)
	}
	return "plan:" + hex.EncodeToString(h.Sum(nil))[:16]
}
