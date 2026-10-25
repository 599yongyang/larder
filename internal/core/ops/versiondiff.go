package ops

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"time"

	"larder/internal/core/model"
)

// BuildVersionDiff compares the snapshot taken before an operation with the
// Skill folder as it exists now. It is used by version history before a
// restore, so users can see what changes the restore will introduce.
func BuildVersionDiff(ctx context.Context, operation model.Operation, skill model.Skill) (model.SkillUpdateDiff, error) {
	if err := ctx.Err(); err != nil {
		return model.SkillUpdateDiff{}, err
	}
	if operation.Operation != "skill_update" && operation.Operation != "uninstall_skill" {
		return model.SkillUpdateDiff{}, errors.New("operation has no version snapshot")
	}
	raw, err := os.ReadFile(filepath.Join(operation.SnapshotPath, "manifest.json"))
	if err != nil {
		return model.SkillUpdateDiff{}, err
	}
	var manifest SnapshotManifest
	if err := json.Unmarshal(raw, &manifest); err != nil {
		return model.SkillUpdateDiff{}, err
	}
	targetPath := operationPath(operation)
	var backup string
	for _, target := range manifest.Targets {
		if target.Target == targetPath || target.Target == skill.Path {
			backup = target.BackupPath
			break
		}
	}
	if backup == "" {
		return model.SkillUpdateDiff{}, errors.New("snapshot has no version backup")
	}
	oldFiles, err := fingerprintTree(backup)
	if err != nil {
		return model.SkillUpdateDiff{}, err
	}
	currentFiles, err := fingerprintTree(skill.Path)
	if err != nil && !os.IsNotExist(err) {
		return model.SkillUpdateDiff{}, err
	}
	if currentFiles == nil {
		currentFiles = map[string]fileFingerprint{}
	}
	diff := model.SkillUpdateDiff{SkillID: skill.ID, SkillName: skill.Name, Files: []model.SkillUpdateFile{}, HasBaseline: true, ScannedAt: time.Now()}
	paths := map[string]bool{}
	for path := range oldFiles {
		paths[path] = true
	}
	for path := range currentFiles {
		paths[path] = true
	}
	for path := range paths {
		oldFile, oldOK := oldFiles[path]
		currentFile, currentOK := currentFiles[path]
		file := model.SkillUpdateFile{Path: path}
		switch {
		case !currentOK:
			file.Status = "removed"
			file.LocalSize = oldFile.size
			file.Binary = oldFile.binary
			if !file.Binary {
				content, _ := os.ReadFile(filepath.Join(backup, filepath.FromSlash(path)))
				file.Lines, file.Truncated = lineDiffBytes(content, nil)
				file.Additions, file.Deletions = countDiffLines(file.Lines)
			}
			diff.Files = append(diff.Files, file)
		case !oldOK:
			file.Status = "added"
			file.Size = currentFile.size
			file.Binary = currentFile.binary
			diff.Added++
			if !file.Binary {
				content, _ := os.ReadFile(filepath.Join(skill.Path, filepath.FromSlash(path)))
				file.Lines, file.Truncated = lineDiffBytes(nil, content)
				file.Additions, file.Deletions = countDiffLines(file.Lines)
			}
			diff.Files = append(diff.Files, file)
		case oldFile.hash == currentFile.hash:
			continue
		default:
			file.Status = "modified"
			file.Size = currentFile.size
			file.LocalSize = oldFile.size
			file.Binary = oldFile.binary || currentFile.binary
			diff.Modified++
			if !file.Binary {
				file.Lines, file.Truncated = lineDiff(filepath.Join(backup, filepath.FromSlash(path)), filepath.Join(skill.Path, filepath.FromSlash(path)))
				file.Additions, file.Deletions = countDiffLines(file.Lines)
			}
			diff.Files = append(diff.Files, file)
		}
	}
	sort.Slice(diff.Files, func(i, j int) bool {
		rank := map[string]int{"added": 0, "modified": 1, "removed": 2}
		if rank[diff.Files[i].Status] != rank[diff.Files[j].Status] {
			return rank[diff.Files[i].Status] < rank[diff.Files[j].Status]
		}
		return diff.Files[i].Path < diff.Files[j].Path
	})
	return diff, nil
}

func operationPath(operation model.Operation) string {
	for _, change := range operation.Changes {
		for _, prefix := range []string{"updated:", "uninstalled:"} {
			if len(change) > len(prefix) && change[:len(prefix)] == prefix {
				return change[len(prefix):]
			}
		}
	}
	return ""
}
