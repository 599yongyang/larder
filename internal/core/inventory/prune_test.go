package inventory

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"larder/internal/core/model"
)

func TestPruneSnapshotsKeepsNewest(t *testing.T) {
	root := t.TempDir()
	for i := 0; i < 6; i++ {
		dir := filepath.Join(root, "snap-"+string(rune('a'+i)))
		if err := os.MkdirAll(dir, 0o700); err != nil {
			t.Fatal(err)
		}
		mod := time.Now().Add(time.Duration(i-10) * time.Hour)
		if err := os.Chtimes(dir, mod, mod); err != nil {
			t.Fatal(err)
		}
	}
	service := &Service{snapshotRoot: root}
	if err := service.PruneSnapshots(3); err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 3 {
		t.Fatalf("snapshots = %d, want 3", len(entries))
	}
}

func TestPruneInstalledNewSkillsRemovesLocalMatches(t *testing.T) {
	service := &Service{}
	report := model.UpdateReport{SkillAdded: 2, Items: []model.UpdateItem{
		{Kind: "skill", Name: "alpha", State: "new"},
		{Kind: "skill", Name: "beta", State: "new"},
		{Kind: "skill", Name: "alpha", State: "changed"},
	}}
	result := model.ScanResult{Skills: []model.Skill{{Name: "alpha", HasSkillMD: true}}}
	got := service.pruneInstalledNewSkills(report, result)
	if got.SkillAdded != 1 || len(got.Items) != 2 {
		t.Fatalf("report = %#v", got)
	}
	if got.Items[0].Name != "beta" || got.Items[1].State != "changed" {
		t.Fatalf("items = %#v", got.Items)
	}
}

func TestRegisterInstalledSourceWritesLockfile(t *testing.T) {
	home := t.TempDir()
	t.Setenv("LARDER_HOME", home)
	service := &Service{}
	service.registerInstalledSource("owner/repo", "alpha", "skills/alpha/SKILL.md")
	raw, err := os.ReadFile(filepath.Join(home, ".agents", ".skill-lock.json"))
	if err != nil {
		t.Fatal(err)
	}
	var data struct {
		Skills map[string]struct {
			Source    string `json:"source"`
			SourceURL string `json:"sourceUrl"`
			SkillPath string `json:"skillPath"`
		} `json:"skills"`
	}
	if err := json.Unmarshal(raw, &data); err != nil {
		t.Fatal(err)
	}
	entry := data.Skills["alpha"]
	if entry.Source != "owner/repo" || entry.SourceURL != "https://github.com/owner/repo" || entry.SkillPath != "skills/alpha/SKILL.md" {
		t.Fatalf("entry = %#v", entry)
	}
}
