package ops

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"time"

	"larder/internal/core/model"
	"larder/internal/platform"
)

type SnapshotTarget struct {
	RootID     string `json:"rootId"`
	Target     string `json:"target"`
	Action     string `json:"action"`
	Exists     bool   `json:"exists"`
	LinkTarget string `json:"linkTarget,omitempty"`
	BackupPath string `json:"backupPath,omitempty"`
}

type SnapshotManifest struct {
	ID        string           `json:"id"`
	PlanID    string           `json:"planId"`
	CreatedAt time.Time        `json:"createdAt"`
	Targets   []SnapshotTarget `json:"targets"`
}

// ApplyLinkPlan writes only after it has persisted a snapshot manifest and
// re-validated that every target is still absent. A failure removes links it
// created in this operation; it never deletes pre-existing user data.
func ApplyLinkPlan(ctx context.Context, plan model.Plan, snapshotRoot string, filesystem platform.FileSystem, linker platform.Linker) (model.Operation, error) {
	now := time.Now()
	op := model.Operation{ID: fmt.Sprintf("op-%s-%d", plan.ID, now.UnixNano()), PlanID: plan.ID, Operation: plan.Operation, Status: "failed", CreatedAt: now}
	if filesystem == nil {
		filesystem = platform.OSFileSystem{}
	}
	if linker == nil {
		linker = platform.OSLinker{}
	}
	if plan.Operation != "link_skill" && plan.Operation != "replace_copy" && plan.Operation != "disable_link" && plan.Operation != "sync_copy" {
		return op, errors.New("unsupported operation")
	}
	if len(plan.Changes) == 0 {
		return op, errors.New("empty plan")
	}
	var creates []model.PlanChange
	var replacements []model.PlanChange
	var copies []model.PlanChange
	var syncs []model.PlanChange
	var disables []model.PlanChange
	for _, change := range plan.Changes {
		if change.Action == "create_symlink" || change.Action == "replace_link" || change.Action == "replace_copy" || change.Action == "remove_link" || change.Action == "sync_copy" {
			if info, err := filesystem.Stat(change.Source); err != nil || !info.IsDir() {
				return op, fmt.Errorf("skill source disappeared: %s", change.Source)
			}
		}
		switch change.Action {
		case "create_symlink":
			if _, err := filesystem.Lstat(change.Target); !errors.Is(err, fs.ErrNotExist) {
				if err == nil {
					return op, fmt.Errorf("target already exists: %s", change.Target)
				}
				return op, err
			}
			creates = append(creates, change)
		case "replace_link":
			info, err := filesystem.Lstat(change.Target)
			if err != nil || info.Mode()&fs.ModeSymlink == 0 {
				return op, fmt.Errorf("replace target is not a link: %s", change.Target)
			}
			old, err := filesystem.Readlink(change.Target)
			if err != nil {
				return op, err
			}
			if !filepath.IsAbs(old) {
				old = filepath.Join(filepath.Dir(change.Target), old)
			}
			if _, err := filesystem.Stat(filepath.Clean(old)); err == nil {
				return op, fmt.Errorf("refusing to replace a valid link: %s", change.Target)
			}
			replacements = append(replacements, change)
		case "replace_copy":
			info, err := filesystem.Lstat(change.Target)
			if err != nil || !info.IsDir() || info.Mode()&fs.ModeSymlink != 0 {
				return op, fmt.Errorf("replace_copy target is not a directory: %s", change.Target)
			}
			copies = append(copies, change)
		case "sync_copy":
			info, err := filesystem.Lstat(change.Target)
			if err != nil || !info.IsDir() || info.Mode()&fs.ModeSymlink != 0 {
				return op, fmt.Errorf("sync_copy target is not a directory: %s", change.Target)
			}
			syncs = append(syncs, change)
		case "remove_link":
			info, err := filesystem.Lstat(change.Target)
			if err != nil || info.Mode()&fs.ModeSymlink == 0 {
				return op, fmt.Errorf("remove target is not a symlink: %s", change.Target)
			}
			link, err := filesystem.Readlink(change.Target)
			if err != nil {
				return op, err
			}
			if !filepath.IsAbs(link) {
				link = filepath.Join(filepath.Dir(change.Target), link)
			}
			if filepath.Clean(link) != filepath.Clean(change.Source) {
				return op, fmt.Errorf("refusing to remove unrelated link: %s", change.Target)
			}
			disables = append(disables, change)
		case "already_linked":
		default:
			return op, fmt.Errorf("unsafe plan change: %s", change.Action)
		}
		if err := ctx.Err(); err != nil {
			return op, err
		}
	}

	if snapshotRoot == "" {
		return op, errors.New("snapshot root is not configured")
	}
	snapshotID := fmt.Sprintf("snapshot-%d", now.UnixNano())
	snapshotDir := filepath.Join(snapshotRoot, snapshotID)
	if err := os.MkdirAll(snapshotDir, 0o700); err != nil {
		return op, fmt.Errorf("create snapshot: %w", err)
	}
	manifest := SnapshotManifest{ID: snapshotID, PlanID: plan.ID, CreatedAt: time.Now()}
	for index, change := range plan.Changes {
		entry := SnapshotTarget{RootID: change.RootID, Target: change.Target, Action: change.Action}
		if info, err := filesystem.Lstat(change.Target); err == nil {
			entry.Exists = true
			if info.Mode()&fs.ModeSymlink != 0 {
				if link, linkErr := filesystem.Readlink(change.Target); linkErr == nil {
					entry.LinkTarget = link
				}
			}
		}
		if change.Action == "replace_copy" || change.Action == "sync_copy" {
			backup := filepath.Join(snapshotDir, "trees", fmt.Sprintf("%d", index))
			if err := copyTree(change.Target, backup); err != nil {
				return op, fmt.Errorf("snapshot copy %s: %w", change.Target, err)
			}
			entry.BackupPath = backup
		}
		manifest.Targets = append(manifest.Targets, entry)
	}
	raw, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return op, err
	}
	manifestPath := filepath.Join(snapshotDir, "manifest.json")
	if err := os.WriteFile(manifestPath, raw, 0o600); err != nil {
		return op, fmt.Errorf("write snapshot: %w", err)
	}
	op.SnapshotPath = snapshotDir

	created := make([]string, 0, len(creates))
	for _, change := range creates {
		if err := linker.Symlink(change.Source, change.Target); err != nil {
			cleanup(linker, created)
			return op, fmt.Errorf("create link %s: %w", change.Target, err)
		}
		created = append(created, change.Target)
		op.Changes = append(op.Changes, "created:"+change.Target)
	}
	replaced := make([]string, 0, len(replacements))
	oldTargets := map[string]string{}
	for _, change := range replacements {
		old, err := filesystem.Readlink(change.Target)
		if err != nil {
			restoreReplacements(linker, replacements, oldTargets)
			cleanup(linker, created)
			return op, fmt.Errorf("read replace target %s: %w", change.Target, err)
		}
		oldTargets[change.Target] = old
		if err := linker.Remove(change.Target); err != nil {
			restoreReplacements(linker, replacements, oldTargets)
			cleanup(linker, created)
			return op, err
		}
		if err := linker.Symlink(change.Source, change.Target); err != nil {
			_ = linker.Symlink(old, change.Target)
			restoreReplacements(linker, replacements, oldTargets)
			cleanup(linker, created)
			return op, fmt.Errorf("replace link %s: %w", change.Target, err)
		}
		replaced = append(replaced, change.Target)
		op.Changes = append(op.Changes, "replaced:"+change.Target)
	}
	for _, change := range copies {
		if err := os.RemoveAll(change.Target); err != nil {
			restoreReplacements(linker, replacements, oldTargets)
			cleanup(linker, created)
			return op, fmt.Errorf("remove copy %s: %w", change.Target, err)
		}
		if err := linker.Symlink(change.Source, change.Target); err != nil {
			restoreCopy(change.Target, manifest)
			restoreReplacements(linker, replacements, oldTargets)
			cleanup(linker, created)
			return op, fmt.Errorf("link copy %s: %w", change.Target, err)
		}
		op.Changes = append(op.Changes, "copy_to_link:"+change.Target)
	}
	removed := make([]string, 0, len(disables))
	removedLinks := map[string]string{}
	for _, change := range syncs {
		if err := os.RemoveAll(change.Target); err != nil {
			restoreCopies(manifest)
			restoreReplacements(linker, replacements, oldTargets)
			cleanup(linker, created)
			return op, fmt.Errorf("remove copy %s: %w", change.Target, err)
		}
		if err := copyTree(change.Source, change.Target); err != nil {
			restoreCopy(change.Target, manifest)
			restoreCopies(manifest)
			restoreReplacements(linker, replacements, oldTargets)
			cleanup(linker, created)
			return op, fmt.Errorf("sync copy %s: %w", change.Target, err)
		}
		op.Changes = append(op.Changes, "synced:"+change.Target)
	}
	for _, change := range disables {
		old, err := filesystem.Readlink(change.Target)
		if err != nil {
			restoreCopies(manifest)
			cleanup(linker, created)
			return op, err
		}
		removedLinks[change.Target] = old
		if err := linker.Remove(change.Target); err != nil {
			restoreRemoved(linker, removed, removedLinks)
			restoreCopies(manifest)
			cleanup(linker, created)
			return op, fmt.Errorf("remove link %s: %w", change.Target, err)
		}
		removed = append(removed, change.Target)
		op.Changes = append(op.Changes, "removed:"+change.Target)
	}
	for _, change := range append(append(append([]model.PlanChange{}, creates...), replacements...), copies...) {
		info, err := filesystem.Lstat(change.Target)
		if err != nil || info.Mode()&fs.ModeSymlink == 0 {
			restoreCopies(manifest)
			restoreReplacements(linker, replacements, oldTargets)
			cleanup(linker, created)
			return op, fmt.Errorf("verify link %s: %v", change.Target, err)
		}
		link, err := filesystem.Readlink(change.Target)
		if err != nil {
			restoreCopies(manifest)
			restoreReplacements(linker, replacements, oldTargets)
			cleanup(linker, created)
			return op, fmt.Errorf("verify link %s: %w", change.Target, err)
		}
		if !filepath.IsAbs(link) {
			link = filepath.Join(filepath.Dir(change.Target), link)
		}
		if filepath.Clean(link) != filepath.Clean(change.Source) {
			restoreCopies(manifest)
			restoreRemoved(linker, removed, removedLinks)
			restoreReplacements(linker, replacements, oldTargets)
			cleanup(linker, created)
			return op, fmt.Errorf("verify link %s: unexpected target", change.Target)
		}
	}
	for _, change := range disables {
		if _, err := filesystem.Lstat(change.Target); err == nil {
			restoreCopies(manifest)
			restoreRemoved(linker, removed, removedLinks)
			restoreReplacements(linker, replacements, oldTargets)
			cleanup(linker, created)
			return op, fmt.Errorf("verify removed link %s still exists", change.Target)
		}
	}
	for _, change := range syncs {
		if info, err := filesystem.Stat(change.Target); err != nil || !info.IsDir() {
			restoreCopies(manifest)
			restoreRemoved(linker, removed, removedLinks)
			restoreReplacements(linker, replacements, oldTargets)
			cleanup(linker, created)
			return op, fmt.Errorf("verify synced copy %s", change.Target)
		}
	}
	op.Status = "success"
	op.Message = fmt.Sprintf("新建 %d 个链接，修复 %d 个失效链接，同步 %d 个副本，移除 %d 个链接；快照已保存", len(created), len(replaced), len(syncs), len(disables))
	return op, nil
}

func restoreRemoved(linker platform.Linker, removed []string, old map[string]string) {
	for _, path := range removed {
		if target := old[path]; target != "" {
			_ = linker.Symlink(target, path)
		}
	}
}

func restoreCopies(manifest SnapshotManifest) {
	for _, target := range manifest.Targets {
		if target.Action == "replace_copy" && target.BackupPath != "" {
			_ = os.RemoveAll(target.Target)
			_ = copyTree(target.BackupPath, target.Target)
		}
	}
}

func restoreCopy(target string, manifest SnapshotManifest) {
	for _, entry := range manifest.Targets {
		if entry.Target == target && entry.BackupPath != "" {
			_ = os.RemoveAll(target)
			_ = copyTree(entry.BackupPath, target)
		}
	}
}

func copyTree(source, target string) error {
	info, err := os.Lstat(source)
	if err != nil {
		return err
	}
	if info.Mode()&fs.ModeSymlink != 0 {
		link, err := os.Readlink(source)
		if err != nil {
			return err
		}
		return os.Symlink(link, target)
	}
	if info.IsDir() {
		if err := os.MkdirAll(target, info.Mode().Perm()); err != nil {
			return err
		}
		entries, err := os.ReadDir(source)
		if err != nil {
			return err
		}
		for _, entry := range entries {
			if err := copyTree(filepath.Join(source, entry.Name()), filepath.Join(target, entry.Name())); err != nil {
				return err
			}
		}
		return nil
	}
	content, err := os.ReadFile(source)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
		return err
	}
	return os.WriteFile(target, content, info.Mode().Perm())
}

func restoreReplacements(linker platform.Linker, replacements []model.PlanChange, oldTargets map[string]string) {
	for _, change := range replacements {
		_ = linker.Remove(change.Target)
		if old := oldTargets[change.Target]; old != "" {
			_ = linker.Symlink(old, change.Target)
		}
	}
}

func cleanup(linker platform.Linker, paths []string) {
	for _, path := range paths {
		_ = linker.Remove(path)
	}
}

func SnapshotLockfile(snapshotDir, lockfilePath string) error {
	if snapshotDir == "" || lockfilePath == "" {
		return errors.New("snapshot and lockfile paths are required")
	}
	raw, err := os.ReadFile(filepath.Join(snapshotDir, "manifest.json"))
	if err != nil {
		return err
	}
	var manifest SnapshotManifest
	if err := json.Unmarshal(raw, &manifest); err != nil {
		return err
	}
	target := SnapshotTarget{RootID: "meta", Target: lockfilePath, Action: "lockfile"}
	if current, readErr := os.ReadFile(lockfilePath); readErr == nil {
		backup := filepath.Join(snapshotDir, "lockfile-backup.json")
		if err := os.WriteFile(backup, current, 0o600); err != nil {
			return err
		}
		target.Exists = true
		target.BackupPath = backup
	} else if !errors.Is(readErr, os.ErrNotExist) {
		return readErr
	}
	manifest.Targets = append(manifest.Targets, target)
	payload, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(snapshotDir, "manifest.json"), payload, 0o600)
}

// RollbackSnapshot removes links recorded as created by a snapshot. It never
// removes a path that currently exists as a real file or directory.
func RollbackSnapshot(snapshotDir string, filesystem platform.FileSystem, linker platform.Linker) error {
	raw, err := os.ReadFile(filepath.Join(snapshotDir, "manifest.json"))
	if err != nil {
		return err
	}
	var manifest SnapshotManifest
	if err := json.Unmarshal(raw, &manifest); err != nil {
		return err
	}
	if filesystem == nil {
		filesystem = platform.OSFileSystem{}
	}
	if linker == nil {
		linker = platform.OSLinker{}
	}
	for _, target := range manifest.Targets {
		if target.Action == "lockfile" || target.Action == "lockfile_restore" {
			if target.Exists && target.BackupPath != "" {
				if err := copyFile(target.BackupPath, target.Target); err != nil {
					return err
				}
			} else if !target.Exists {
				if err := os.Remove(target.Target); err != nil && !errors.Is(err, os.ErrNotExist) {
					return err
				}
			}
			continue
		}
		if target.Action == "install_skill" {
			_ = os.RemoveAll(target.Target)
			continue
		}
		if target.Action == "uninstall_skill" && target.BackupPath != "" {
			_ = os.RemoveAll(target.Target)
			if err := copyTree(target.BackupPath, target.Target); err != nil {
				return err
			}
			continue
		}
		if target.Action == "update_skill" && target.BackupPath != "" {
			_ = os.RemoveAll(target.Target)
			if err := copyTree(target.BackupPath, target.Target); err != nil {
				return err
			}
			continue
		}
		if target.Action == "sync_copy" && target.BackupPath != "" {
			_ = os.RemoveAll(target.Target)
			if err := copyTree(target.BackupPath, target.Target); err != nil {
				return err
			}
			continue
		}
		if target.Action == "remove_mcp_entry" && target.BackupPath != "" {
			if err := copyFile(target.BackupPath, target.Target); err != nil {
				return err
			}
			continue
		}
		if target.Action == "remove_link" {
			if _, err := filesystem.Lstat(target.Target); err == nil {
				return fmt.Errorf("refusing to restore over existing path %s", target.Target)
			}
			if target.LinkTarget != "" {
				if err := linker.Symlink(target.LinkTarget, target.Target); err != nil {
					return err
				}
			}
			continue
		}
		info, err := filesystem.Lstat(target.Target)
		if err != nil {
			continue
		}
		if info.Mode()&fs.ModeSymlink == 0 {
			return fmt.Errorf("refusing to remove non-link %s", target.Target)
		}
		if err := linker.Remove(target.Target); err != nil {
			return err
		}
		if target.Action == "replace_link" && target.LinkTarget != "" {
			if err := linker.Symlink(target.LinkTarget, target.Target); err != nil {
				return err
			}
		}
		if (target.Action == "replace_copy" || target.Action == "sync_copy") && target.BackupPath != "" {
			_ = os.RemoveAll(target.Target)
			if err := copyTree(target.BackupPath, target.Target); err != nil {
				return err
			}
		}
	}
	return nil
}

// CreateRestoreSnapshot records the current state of the paths touched by an
// operation before rolling it back. The snapshot is itself restorable, so a
// version/trash restore can always be undone.
func CreateRestoreSnapshot(sourceSnapshotDir, snapshotRoot string) (string, error) {
	if snapshotRoot == "" {
		return "", errors.New("snapshot root is not configured")
	}
	raw, err := os.ReadFile(filepath.Join(sourceSnapshotDir, "manifest.json"))
	if err != nil {
		return "", err
	}
	var source SnapshotManifest
	if err := json.Unmarshal(raw, &source); err != nil {
		return "", err
	}
	now := time.Now()
	id := fmt.Sprintf("restore-%d", now.UnixNano())
	dir := filepath.Join(snapshotRoot, id)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	manifest := SnapshotManifest{ID: id, CreatedAt: now}
	for index, target := range source.Targets {
		if target.Action == "lockfile" || target.Action == "lockfile_restore" {
			entry := SnapshotTarget{Target: target.Target, Action: "lockfile_restore"}
			if current, readErr := os.ReadFile(target.Target); readErr == nil {
				backup := filepath.Join(dir, "files", fmt.Sprintf("%d", index))
				if err := os.MkdirAll(filepath.Dir(backup), 0o700); err != nil {
					return "", err
				}
				if err := os.WriteFile(backup, current, 0o600); err != nil {
					return "", err
				}
				entry.Exists = true
				entry.BackupPath = backup
			} else if !errors.Is(readErr, os.ErrNotExist) {
				return "", readErr
			}
			manifest.Targets = append(manifest.Targets, entry)
			continue
		}
		info, statErr := os.Lstat(target.Target)
		if statErr != nil {
			continue
		}
		entry := SnapshotTarget{Target: target.Target, Exists: true}
		switch {
		case info.Mode()&fs.ModeSymlink != 0:
			link, readErr := os.Readlink(target.Target)
			if readErr != nil {
				return "", readErr
			}
			entry.Action = "replace_link"
			entry.LinkTarget = link
		case info.IsDir():
			backup := filepath.Join(dir, "trees", fmt.Sprintf("%d", index))
			if err := copyTree(target.Target, backup); err != nil {
				return "", err
			}
			entry.Action = "uninstall_skill"
			entry.BackupPath = backup
		default:
			backup := filepath.Join(dir, "files", fmt.Sprintf("%d", index))
			if err := os.MkdirAll(filepath.Dir(backup), 0o700); err != nil {
				return "", err
			}
			if err := copyFile(target.Target, backup); err != nil {
				return "", err
			}
			entry.Action = "remove_mcp_entry"
			entry.BackupPath = backup
		}
		manifest.Targets = append(manifest.Targets, entry)
	}
	payload, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return "", err
	}
	if err := os.WriteFile(filepath.Join(dir, "manifest.json"), payload, 0o600); err != nil {
		return "", err
	}
	return dir, nil
}
