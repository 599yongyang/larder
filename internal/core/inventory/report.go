package inventory

import (
	"context"
	"fmt"
	"sort"
	"strings"
)

// Report renders the current inventory as a local Markdown document. It does
// not write a file or send anything over the network.
func (s *Service) Report(ctx context.Context) (string, error) {
	snapshot := s.Snapshot()
	findings := s.Doctor()
	operations, _ := s.Operations(ctx)
	timeline, _ := s.Timeline(ctx)
	var b strings.Builder
	b.WriteString("# Skills & CLI 报告\n\n")
	fmt.Fprintf(&b, "- 生成时间：%s\n- 扫描状态：%s\n- Skills：%d\n- CLI：%d\n- 发现问题：%d\n\n", snapshot.FinishedAt.Format("2006-01-02 15:04:05"), snapshot.Status, len(snapshot.Skills), len(snapshot.CLIs), len(findings))
	b.WriteString("## Skills\n\n")
	names := make([]string, 0, len(snapshot.Skills))
	byName := map[string]string{}
	for _, skill := range snapshot.Skills {
		if skill.IsAgentLocal {
			continue
		}
		if _, ok := byName[skill.Name]; !ok {
			byName[skill.Name] = skill.Source
			names = append(names, skill.Name)
		}
	}
	sort.Strings(names)
	for _, name := range names {
		fmt.Fprintf(&b, "- %s — %s\n", name, byName[name])
	}
	b.WriteString("\n## 更新与问题\n\n")
	for _, finding := range findings {
		fmt.Fprintf(&b, "- [%s] %s：%s\n", finding.Severity, finding.Title, finding.Message)
	}
	b.WriteString("\n## 最近操作\n\n")
	for _, op := range operations {
		fmt.Fprintf(&b, "- %s %s %s：%s\n", op.CreatedAt.Format("2006-01-02 15:04"), op.Operation, op.Status, op.Message)
	}
	b.WriteString("\n## 时间线\n\n")
	for _, event := range timeline {
		fmt.Fprintf(&b, "- %s %s %s\n", event.At.Format("2006-01-02 15:04"), event.Kind, event.SkillName)
	}
	return b.String(), nil
}
