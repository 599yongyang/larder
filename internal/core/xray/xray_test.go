package xray

import (
	"testing"

	"larder/internal/core/model"
)

func root(id, label string) model.Root {
	return model.Root{ID: id, Label: label, LogicalAgent: label, Path: "/" + id, Kind: model.RootKnownAgent, Known: true, Status: model.RootAvailable, EntryCount: 2}
}

func skill(id, name, rootID string, description string) model.Skill {
	return model.Skill{ID: id, Name: name, Path: "/" + rootID + "/" + name, RootID: rootID, RootKind: model.RootKnownAgent, HasSkillMD: true, Description: description, DescTokens: 10, ScanState: "ok", IsAgentLocal: true}
}

func input(skills []model.Skill, links []model.AgentLink) Input {
	return Input{
		Scan:     model.ScanResult{Roots: []model.Root{root("claude", "Claude Code"), root("codex", "Codex")}, Skills: skills, AgentLinks: links},
		Settings: model.Settings{},
		AgentID:  "claude",
	}
}

func TestBuildFlagsCandidatesMissingAndBroken(t *testing.T) {
	skills := []model.Skill{skill("a", "alpha", "claude", "desc"), skill("b", "beta", "claude", ""), skill("c", "alpha", "codex", "desc"), skill("d", "gamma", "claude", "desc")}
	links := []model.AgentLink{{SkillID: "c", RootID: "claude", Mode: model.LinkLinked, Visible: true}, {SkillID: "d", RootID: "claude", Mode: model.LinkBroken, Visible: true}}
	report, err := Build(input(skills, links))
	if err != nil {
		t.Fatal(err)
	}
	byName := map[string]model.XrayEntry{}
	for _, entry := range report.Entries {
		byName[entry.Skill.Name] = entry
	}
	if byName["alpha"].Status != model.XrayStatusCandidate {
		t.Fatalf("alpha = %s", byName["alpha"].Status)
	}
	if byName["beta"].Status != model.XrayStatusMissing || byName["beta"].Reason != model.XrayReasonMissingDescription {
		t.Fatalf("beta = %#v", byName["beta"])
	}
	if byName["gamma"].Status != model.XrayStatusBroken || byName["gamma"].Reason != model.XrayReasonBrokenLink {
		t.Fatalf("gamma = %#v", byName["gamma"])
	}
}

func TestBuildUsesScanIssuesAndAddsOtherAgentOnlyResources(t *testing.T) {
	skills := []model.Skill{skill("a", "alpha", "claude", "desc"), skill("b", "beta", "codex", "desc")}
	in := input(skills, nil)
	in.Scan.Issues = []model.ScanIssue{{Kind: "scan_failure", Path: "/claude/alpha", Message: "cannot read"}}
	report, err := Build(in)
	if err != nil {
		t.Fatal(err)
	}
	if report.Entries[0].Status != model.XrayStatusBroken || report.Entries[0].Reason != model.XrayReasonScanFailure {
		t.Fatalf("entry = %#v", report.Entries[0])
	}
	foundBeta := false
	for _, row := range report.Compare.Rows {
		if row.Name == "beta" {
			foundBeta = true
			if row.Cells[1].Status != model.XrayStatusActive {
				t.Fatalf("beta cell = %s", row.Cells[1].Status)
			}
		}
	}
	if !foundBeta {
		t.Fatal("beta missing from comparison rows")
	}
}

func TestBuildIncludesProjectResourcesWhenSelected(t *testing.T) {
	projectSkill := skill("p", "project-skill", "project", "desc")
	projectSkill.IsProject = true
	projectSkill.Path = "/work/demo/.claude/skills/project-skill"
	in := input([]model.Skill{projectSkill}, nil)
	report, err := Build(in)
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Entries) != 0 {
		t.Fatalf("entries without project = %d", len(report.Entries))
	}
	in.Project = "/work/demo"
	report, err = Build(in)
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Entries) != 1 || report.Entries[0].Skill.Name != "project-skill" {
		t.Fatalf("entries with project = %#v", report.Entries)
	}
}

func TestBuildHonoursRequestedCompareAgents(t *testing.T) {
	in := input([]model.Skill{skill("a", "alpha", "claude", "desc")}, nil)
	in.Scan.Roots = append(in.Scan.Roots, root("cursor", "Cursor"))
	in.CompareAgentIDs = []string{"cursor"}
	report, err := Build(in)
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Compare.Columns) != 2 {
		t.Fatalf("columns = %d", len(report.Compare.Columns))
	}
	if report.Compare.Columns[0].Root.ID != "claude" || report.Compare.Columns[1].Root.ID != "cursor" {
		t.Fatalf("columns = %#v", report.Compare.Columns)
	}
}

func TestBuildDefaultComparisonStaysWithinPreferredAgents(t *testing.T) {
	in := input([]model.Skill{skill("a", "alpha", "claude", "desc")}, nil)
	in.Scan.Roots = append(in.Scan.Roots, root("cursor", "Cursor"), root("opencode", "OpenCode"))
	report, err := Build(in)
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Compare.Columns) != 3 {
		t.Fatalf("columns = %d, want current + 2 preferred", len(report.Compare.Columns))
	}
	for _, column := range report.Compare.Columns {
		if column.Root.ID == "opencode" {
			t.Fatal("non-preferred Agent should not be added by default")
		}
	}
}
