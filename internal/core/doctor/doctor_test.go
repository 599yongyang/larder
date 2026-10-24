package doctor_test

import (
	"testing"

	"larder/internal/core/doctor"
	"larder/internal/core/model"
)

func TestRunReportsFirstSliceRules(t *testing.T) {
	result := model.ScanResult{
		Skills:     []model.Skill{{ID: "s1", Name: "managed", RootKind: model.RootGlobal, Locked: true, HasSkillMD: true}, {ID: "s2", Name: "unmanaged", RootKind: model.RootGlobal, HasSkillMD: false}},
		LockSkills: []model.LockSkill{{Name: "orphan"}},
		AgentLinks: []model.AgentLink{{SkillID: "s1", RootID: "agent", Mode: model.LinkBroken, Path: "/agent/s1", TargetPath: "/gone"}, {SkillID: "s1", RootID: "agent2", Mode: model.LinkCopied, Drift: true}},
		Issues:     []model.ScanIssue{{Kind: "permission", Path: "/private", Message: "permission denied"}},
	}
	findings := doctor.Run(result)
	got := map[string]bool{}
	for _, finding := range findings {
		got[finding.Rule] = true
	}
	for _, rule := range []string{"broken_link", "copy_drift", "missing_skill_md", "unmanaged_skill", "orphan_skill", "permissions"} {
		if !got[rule] {
			t.Errorf("missing rule %q in %#v", rule, findings)
		}
	}
}

func TestBrokenLinksAreGrouped(t *testing.T) {
	result := model.ScanResult{AgentLinks: []model.AgentLink{
		{SkillID: "ghost", RootID: "a", Mode: model.LinkBroken, Path: "/agent-a/ghost"},
		{SkillID: "ghost", RootID: "b", Mode: model.LinkBroken, Path: "/agent-b/ghost"},
	}}
	findings := doctor.Run(result)
	var broken int
	for _, finding := range findings {
		if finding.Rule == "broken_link" {
			broken++
			if len(finding.Evidence) != 2 {
				t.Fatalf("evidence = %#v, want 2 locations", finding.Evidence)
			}
		}
	}
	if broken != 1 {
		t.Fatalf("broken findings = %d, want 1 grouped finding", broken)
	}
}

func TestRunReportsCLIInventoryIssues(t *testing.T) {
	result := model.ScanResult{CLIs: []model.CLIInfo{{ID: "cli:jq", Name: "jq", Path: "/opt/homebrew/bin/jq", Owner: "Homebrew", Status: "duplicate", DuplicateCount: 1, Paths: []model.CLIPath{{Path: "/opt/homebrew/bin/jq", Effective: true}, {Path: "/usr/local/bin/jq"}}}}}
	findings := doctor.Run(result)
	got := map[string]bool{}
	for _, finding := range findings {
		got[finding.Rule] = true
	}
	if !got["duplicate_cli"] {
		t.Fatalf("missing duplicate_cli in %#v", findings)
	}
}

func TestRunWithUpdatesAddsActionableFindings(t *testing.T) {
	result := model.ScanResult{Skills: []model.Skill{{ID: "s1", Name: "alpha", Path: "/skills/alpha"}}}
	report := model.UpdateReport{Items: []model.UpdateItem{
		{Kind: "cli", Owner: "Homebrew", Package: "jq", Name: "jq", CurrentVersion: "1.7", LatestVersion: "1.8", State: "available", Commands: []string{"jq"}},
		{Kind: "skill", Name: "alpha", Package: "owner/repo", Source: "owner/repo", State: "changed"},
	}}
	findings := doctor.RunWithUpdates(result, report)
	got := map[string]bool{}
	for _, finding := range findings {
		got[finding.Rule] = true
	}
	if !got["cli_update_available"] || !got["skill_source_changed"] {
		t.Fatalf("missing update findings in %#v", findings)
	}
}

func TestRunReportsOverlappingSkills(t *testing.T) {
	result := model.ScanResult{Skills: []model.Skill{
		{ID: "a", Name: "diagnose", Description: "Diagnose hard bugs and performance regressions with a structured loop", Path: "/skills/a"},
		{ID: "b", Name: "diagnosing-bugs", Description: "Diagnose hard bugs and performance regressions using a structured loop", Path: "/skills/b"},
	}}
	findings := doctor.Run(result)
	for _, finding := range findings {
		if finding.Rule == "overlap_skill" {
			return
		}
	}
	t.Fatalf("missing overlap_skill in %#v", findings)
}

func TestRunReportsSkillLint(t *testing.T) {
	result := model.ScanResult{Skills: []model.Skill{
		{ID: "empty", Name: "empty", HasSkillMD: true, Path: "/skills/empty"},
		{ID: "long", Name: "long", HasSkillMD: true, Path: "/skills/long", DescChars: 1200},
	}}
	findings := doctor.Run(result)
	got := map[string]bool{}
	for _, finding := range findings {
		got[finding.Rule] = true
	}
	if !got["description_missing"] || !got["description_too_long"] {
		t.Fatalf("missing lint findings in %#v", findings)
	}
}
