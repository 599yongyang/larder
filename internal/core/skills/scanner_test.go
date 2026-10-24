package skills_test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"larder/internal/core/model"
	"larder/internal/core/skills"
	"larder/internal/platform"
)

func TestScannerBuildsLedgerAndMatrixFacts(t *testing.T) {
	root := t.TempDir()
	central := filepath.Join(root, "central")
	agent := filepath.Join(root, "agent")
	if err := os.MkdirAll(central, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(agent, 0o755); err != nil {
		t.Fatal(err)
	}
	writeSkill(t, filepath.Join(central, "alpha"), "Alpha", "Read files quickly.")
	writeSkill(t, filepath.Join(central, "drift"), "Drift", "Central copy.")
	writeSkill(t, filepath.Join(central, "broken"), "Broken", "Broken target.")
	if err := os.MkdirAll(filepath.Join(central, "missing-md"), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := platform.CreateDirectoryLink(filepath.Join(central, "alpha"), filepath.Join(agent, "alpha")); err != nil {
		t.Fatal(err)
	}
	brokenTarget := filepath.Join(root, "broken-target")
	if err := os.MkdirAll(brokenTarget, 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := platform.CreateDirectoryLink(brokenTarget, filepath.Join(agent, "broken")); err != nil {
		t.Fatal(err)
	}
	if err := os.RemoveAll(brokenTarget); err != nil {
		t.Fatal(err)
	}
	writeSkill(t, filepath.Join(agent, "drift"), "Drift", "Agent changed.")
	lock := map[string]any{"version": 1, "skills": map[string]any{
		"alpha": map[string]any{"source": "owner/repo", "sourceUrl": "https://github.com/owner/repo", "skillPath": "skills/alpha"},
	}}
	lockBytes, _ := json.Marshal(lock)
	lockPath := filepath.Join(root, "lock.json")
	if err := os.WriteFile(lockPath, lockBytes, 0o644); err != nil {
		t.Fatal(err)
	}

	result, err := skills.NewScanner(platform.OSFileSystem{}).Scan(context.Background(), skills.Options{Roots: []platform.RootSpec{
		{ID: "global", Label: "Global", Path: central, Kind: platform.RootGlobal, Known: true},
		{ID: "agent", Label: "Agent", LogicalAgent: "Agent", Path: agent, Kind: platform.RootKnownAgent, Known: true},
		{ID: "absent", Label: "Absent agent", LogicalAgent: "Absent", Path: filepath.Join(root, "absent"), Kind: platform.RootKnownAgent, Known: true},
	}, LockfilePath: lockPath})
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != "complete" {
		t.Fatalf("status = %s, issues = %#v", result.Status, result.Issues)
	}
	if len(result.Skills) != 4 {
		t.Fatalf("skills = %d, want 4", len(result.Skills))
	}
	if len(result.AgentLinks) != 4 {
		t.Fatalf("links = %d, want 4 (3 observed + 1 missing matrix cell)", len(result.AgentLinks))
	}
	var absent *model.Root
	for i := range result.Roots {
		if result.Roots[i].ID == "absent" {
			absent = &result.Roots[i]
		}
	}
	if absent == nil || absent.Status != "not_installed" {
		t.Fatalf("absent root status = %#v", absent)
	}
	var linked, broken, drift bool
	for _, link := range result.AgentLinks {
		if link.Mode == "linked" {
			linked = true
		}
		if link.Mode == "broken" {
			broken = true
		}
		if link.Mode == "copied" && link.Drift {
			drift = true
		}
	}
	if !linked || !broken || !drift {
		t.Fatalf("link facts missing: linked=%v broken=%v drift=%v", linked, broken, drift)
	}
}

func writeSkill(t *testing.T, dir, name, description string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	content := "---\nname: " + name + "\ndescription: " + description + "\n---\n\n# " + name + "\n"
	if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestScannerDedupesBrokenAgentTargets(t *testing.T) {
	root := t.TempDir()
	central := filepath.Join(root, "central")
	agentA := filepath.Join(root, "agent-a")
	agentB := filepath.Join(root, "agent-b")
	if err := os.MkdirAll(central, 0o755); err != nil {
		t.Fatal(err)
	}
	writeSkill(t, filepath.Join(central, "alpha"), "Alpha", "Fixture.")
	for _, agent := range []string{agentA, agentB} {
		if err := os.MkdirAll(agent, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(filepath.Join(root, "missing-target"), filepath.Join(agent, "ghost")); err != nil {
			t.Fatal(err)
		}
	}
	result, err := skills.NewScanner(platform.OSFileSystem{}).Scan(context.Background(), skills.Options{Roots: []platform.RootSpec{
		{ID: "global", Label: "Global", Path: central, Kind: platform.RootGlobal, Known: true},
		{ID: "a", Label: "A", Path: agentA, Kind: platform.RootKnownAgent, Known: true},
		{ID: "b", Label: "B", Path: agentB, Kind: platform.RootKnownAgent, Known: true},
	}, LockfilePath: filepath.Join(root, "no-lock.json")})
	if err != nil {
		t.Fatal(err)
	}
	var ghosts int
	for _, skill := range result.Skills {
		if skill.Name == "ghost" {
			ghosts++
		}
	}
	if ghosts != 1 {
		t.Fatalf("ghost rows = %d, want 1 (deduped by name)", ghosts)
	}
	var broken []model.AgentLink
	for _, link := range result.AgentLinks {
		if link.Mode == model.LinkBroken {
			broken = append(broken, link)
		}
	}
	if len(broken) != 2 || broken[0].SkillID != broken[1].SkillID {
		t.Fatalf("broken links = %#v, want 2 with the same deduped skill id", broken)
	}
}

func TestSharedSkillRootsAreVisible(t *testing.T) {
	root := t.TempDir()
	central := filepath.Join(root, "central")
	if err := os.MkdirAll(filepath.Join(central, "shared"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(central, "shared", "SKILL.md"), []byte("---\nname: shared\ndescription: shared skill\n---\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	result, err := skills.NewScanner(platform.OSFileSystem{}).Scan(context.Background(), skills.Options{Roots: []platform.RootSpec{
		{ID: "global", Label: "Global", Path: central, Kind: platform.RootGlobal, Known: true},
		{ID: "codex", Label: "Codex", LogicalAgent: "Codex", Path: filepath.Join(root, "missing-codex"), Kind: platform.RootKnownAgent, Known: true, SharedSkills: true},
	}, LockfilePath: filepath.Join(root, "no-lock.json")})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.AgentLinks) != 1 || result.AgentLinks[0].Mode != model.LinkShared || !result.AgentLinks[0].Visible {
		t.Fatalf("agent links = %#v", result.AgentLinks)
	}
}

func TestExtraAgentSkillRoots(t *testing.T) {
	root := t.TempDir()
	central := filepath.Join(root, "central")
	codex := filepath.Join(root, "codex")
	extra := filepath.Join(root, "plugins", "cache", "market", "plugin", "1.0.0", "skills", "plugin-skill")
	for _, dir := range []string{filepath.Join(central, "central-skill"), codex, extra} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte("---\nname: "+filepath.Base(dir)+"\ndescription: test\n---\n"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	result, err := skills.NewScanner(platform.OSFileSystem{}).Scan(context.Background(), skills.Options{Roots: []platform.RootSpec{
		{ID: "global", Label: "Global", Path: central, Kind: platform.RootGlobal, Known: true},
		{ID: "codex", Label: "Codex", LogicalAgent: "Codex", Path: codex, Kind: platform.RootKnownAgent, Known: true, SharedSkills: true, ExtraPaths: []string{filepath.Join(root, "plugins", "cache")}},
	}, LockfilePath: filepath.Join(root, "no-lock.json")})
	if err != nil {
		t.Fatal(err)
	}
	var found bool
	for _, skill := range result.Skills {
		if skill.Name == "plugin-skill" && skill.RootID == "codex" && skill.IsAgentLocal {
			found = true
			if skill.Origin != "plugin" {
				t.Fatalf("plugin origin = %q", skill.Origin)
			}
		}
	}
	if !found {
		t.Fatalf("plugin skill missing from %#v", result.Skills)
	}
	for _, root := range result.Roots {
		if root.ID == "codex" && root.EntryCount < 2 {
			t.Fatalf("codex entry count = %d", root.EntryCount)
		}
	}
}
