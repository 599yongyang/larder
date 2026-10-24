package doctor

import (
	"fmt"
	"sort"

	"larder/internal/core/insights"
	"larder/internal/core/model"
)

// Run evaluates the first-slice Skill Doctor rules. It only reports facts from
// a scan and never attempts a repair.
func Run(result model.ScanResult) []model.Finding {
	return RunWithUpdates(result, model.UpdateReport{})
}

// RunWithUpdates adds findings from the explicit update check; it still only
// reports facts and never installs or changes anything.
func RunWithUpdates(result model.ScanResult, updateReport model.UpdateReport) []model.Finding {
	findings := make([]model.Finding, 0)

	// One broken target can appear in several agent directories. Group by the
	// (deduped) skill ID so Doctor reports one finding with all locations
	// instead of N near-identical cards (ADR-0029).
	brokenBySkill := map[string]*model.Finding{}
	var brokenOrder []string
	for _, link := range result.AgentLinks {
		if link.Mode != model.LinkBroken {
			continue
		}
		finding, ok := brokenBySkill[link.SkillID]
		if !ok {
			finding = &model.Finding{
				ID: "broken-link-" + link.SkillID, Rule: "broken_link", Severity: model.SeverityError,
				Title: "坏链", Path: link.Path, SkillID: link.SkillID, RootID: link.RootID,
				Remediation: "检查目标路径；M1 只提供报告，不会修改链接。",
			}
			brokenBySkill[link.SkillID] = finding
			brokenOrder = append(brokenOrder, link.SkillID)
		}
		location := link.Path
		if location == "" {
			location = link.TargetPath
		}
		if location != "" {
			finding.Evidence = append(finding.Evidence, location)
		}
		if finding.Path == "" {
			finding.Path = location
		}
	}
	for _, id := range brokenOrder {
		finding := brokenBySkill[id]
		finding.Message = fmt.Sprintf("Agent 目录中有 %d 处链接目标不存在。", len(finding.Evidence))
		findings = append(findings, *finding)
	}

	for _, link := range result.AgentLinks {
		switch link.Mode {
		case model.LinkBroken:
			// already grouped above
		case model.LinkCopied:
			if link.Drift {
				findings = append(findings, model.Finding{
					ID: "copy-drift-" + link.SkillID + "-" + link.RootID, Rule: "copy_drift", Severity: model.SeverityWarn,
					Title: "副本漂移", Message: "Agent 目录中的副本与共享库内容不同。", Path: link.Path, SkillID: link.SkillID, RootID: link.RootID,
					Evidence: []string{link.ContentHash}, Remediation: "M2 再由用户选择保留副本、覆盖副本或回收副本。",
				})
			}
		}
	}

	locked := map[string]model.LockSkill{}
	for _, lock := range result.LockSkills {
		locked[lock.Name] = lock
	}
	seenCentral := map[string]model.Skill{}
	for _, skill := range result.Skills {
		if skill.RootKind == model.RootGlobal {
			seenCentral[skill.Name] = skill
			if !skill.HasSkillMD {
				findings = append(findings, model.Finding{
					ID: "missing-skill-md-" + skill.ID, Rule: "missing_skill_md", Severity: model.SeverityError,
					Title: "缺少 SKILL.md", Message: "Skill 目录没有可读取的 SKILL.md。", Path: skill.Path, SkillID: skill.ID,
					Remediation: "补充 SKILL.md 后重新扫描。",
				})
			}
			if !skill.Locked {
				findings = append(findings, model.Finding{
					ID: "unmanaged-skill-" + skill.ID, Rule: "unmanaged_skill", Severity: model.SeverityWarn,
					Title: "无主 Skill", Message: "Skill 没有对应的 lockfile 来源记录。", Path: skill.Path, SkillID: skill.ID,
					Remediation: "可以在应用元数据中登记来源；登记不会修改 lockfile。",
				})
			}
		}
	}
	for name, lock := range locked {
		if _, ok := seenCentral[name]; !ok {
			findings = append(findings, model.Finding{
				ID: "orphan-lock-" + name, Rule: "orphan_skill", Severity: model.SeverityWarn,
				Title: "lockfile 孤儿", Message: fmt.Sprintf("lockfile 记录 %q，但共享库中没有对应目录。", name), SkillID: name,
				Evidence: []string{lock.SourceURL, lock.SkillPath}, Remediation: "确认 Skill 是否被移除或 lockfile 是否过期。",
			})
		}
	}
	for _, issue := range result.Issues {
		if issue.Kind == "missing_skill_md" {
			// The direct skill rule above carries the stable Skill ID and a
			// focused remediation, so do not show the same fact twice.
			continue
		}
		severity := model.SeverityWarn
		if issue.Kind == "permission" || issue.Kind == "scan_failure" || issue.Kind == "lockfile_error" {
			severity = model.SeverityError
		}
		findings = append(findings, model.Finding{
			ID: "scan-" + issue.Kind + "-" + issue.Path, Rule: ruleForIssue(issue.Kind), Severity: severity,
			Title: titleForIssue(issue.Kind), Message: issue.Message, Path: issue.Path,
			Evidence: []string{issue.Path}, Remediation: "检查路径权限或配置后重新扫描。",
		})
	}
	for _, cli := range result.CLIs {
		paths := make([]string, 0, len(cli.Paths))
		for _, item := range cli.Paths {
			if item.Path != "" {
				paths = append(paths, item.Path)
			}
		}
		if cli.Status == "broken" {
			findings = append(findings, model.Finding{
				ID: "cli-broken-" + cli.ID, Rule: "broken_cli", Severity: model.SeverityError,
				Title: "CLI 坏链", Message: fmt.Sprintf("%s 的命令路径指向不存在的目标。", cli.Name),
				Path: cli.Path, Evidence: paths, Remediation: "检查命令文件是否已被删除；M1 只报告，不会修改或删除。",
			})
			continue
		}
		if cli.DuplicateCount > 0 {
			findings = append(findings, model.Finding{
				ID: "cli-duplicate-" + cli.ID, Rule: "duplicate_cli", Severity: model.SeverityWarn,
				Title: "CLI 重复安装", Message: fmt.Sprintf("%s 在 PATH 中有 %d 个位置；当前生效的是 %s。", cli.Name, len(cli.Paths), cli.Path),
				Path: cli.Path, Evidence: paths, Remediation: "确认 PATH 优先级与来源；M1 只报告，不会卸载。",
			})
		}
	}
	skillsByName := map[string]model.Skill{}
	for _, skill := range result.Skills {
		if _, ok := skillsByName[skill.Name]; !ok {
			skillsByName[skill.Name] = skill
		}
	}
	for _, item := range updateReport.Items {
		switch {
		case item.Kind == "cli" && item.State == "available":
			findings = append(findings, model.Finding{
				ID: "update-" + item.Owner + "-" + item.Package, Rule: "cli_update_available", Severity: model.SeverityInfo,
				Title: "CLI 可更新", Message: fmt.Sprintf("%s：%s → %s", item.Name, item.CurrentVersion, item.LatestVersion),
				Evidence: item.Commands, Remediation: "打开“更新”页查看；M1 不会自动升级。",
			})
		case item.Kind == "skill" && item.State == "changed":
			skill := skillsByName[item.Name]
			findings = append(findings, model.Finding{
				ID: "update-skill-" + item.Name, Rule: "skill_source_changed", Severity: model.SeverityWarn,
				Title: "Skill 来源有变化", Message: fmt.Sprintf("%s 的 SKILL.md 与来源 HEAD 不一致；可能是上游更新或本地修改。", item.Name),
				Path: skill.Path, SkillID: skill.ID, Evidence: []string{item.Package, item.Source},
				Remediation: "打开“更新”页查看；确认来源与本地修改后再决定是否更新。",
			})
		}
	}
	for _, skill := range result.Skills {
		if skill.IsAgentLocal || !skill.HasSkillMD {
			continue
		}
		if skill.DescChars == 0 {
			findings = append(findings, model.Finding{
				ID: "description-missing-" + skill.ID, Rule: "description_missing", Severity: model.SeverityInfo,
				Title: "描述为空", Message: fmt.Sprintf("%s 的 SKILL.md 没有可读取的 frontmatter description。", skill.Name),
				Path: skill.Path, SkillID: skill.ID, Remediation: "补充 name/description，Agent 才能正确发现这个 Skill。",
			})
		} else if skill.DescChars > 1024 {
			findings = append(findings, model.Finding{
				ID: "description-long-" + skill.ID, Rule: "description_too_long", Severity: model.SeverityInfo,
				Title: "描述过长", Message: fmt.Sprintf("%s 的描述约 %d 字符，可能增加每轮上下文成本。", skill.Name, skill.DescChars),
				Path: skill.Path, SkillID: skill.ID, Remediation: "压缩 description，把详细说明放到 SKILL.md 正文。",
			})
		}
	}
	for _, overlap := range insights.Overlaps(result.Skills, 10) {
		findings = append(findings, model.Finding{
			ID: "overlap-" + overlap.A.ID + "-" + overlap.B.ID, Rule: "overlap_skill", Severity: model.SeverityInfo,
			Title: "相似 Skill", Message: fmt.Sprintf("%s 与 %s 的描述相似度约 %.0f%%；确认是否重复安装。", overlap.A.Name, overlap.B.Name, overlap.Score*100),
			Path: overlap.A.Path, SkillID: overlap.A.ID, Evidence: []string{overlap.A.Path, overlap.B.Path},
			Remediation: "对比两个 Skill 的用途，确认后只停用其中一个；M3 不会自动删除。",
		})
	}
	sort.Slice(findings, func(i, j int) bool {
		if findings[i].Severity != findings[j].Severity {
			return severityRank(findings[i].Severity) > severityRank(findings[j].Severity)
		}
		return findings[i].Title < findings[j].Title
	})
	return findings
}

func ruleForIssue(kind string) string {
	if kind == "permission" {
		return "permissions"
	}
	if kind == "scan_unavailable" {
		return "scan_failure"
	}
	return kind
}

func titleForIssue(kind string) string {
	switch kind {
	case "permission":
		return "权限问题"
	case "scan_unavailable":
		return "扫描根不可用"
	case "scan_failure":
		return "扫描失败"
	case "lockfile_error":
		return "lockfile 读取失败"
	case "cancelled":
		return "扫描已取消"
	default:
		return "扫描问题"
	}
}

func severityRank(value model.Severity) int {
	switch value {
	case model.SeverityError:
		return 3
	case model.SeverityWarn:
		return 2
	default:
		return 1
	}
}
