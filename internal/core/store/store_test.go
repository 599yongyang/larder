package store_test

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"larder/internal/core/model"
	"larder/internal/core/store"
)

func TestStorePersistsScanAndSettings(t *testing.T) {
	db, err := store.Open(t.TempDir() + "/ledger.sqlite")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	now := time.Now().UTC().Truncate(time.Millisecond)
	want := model.ScanResult{
		SchemaVersion: model.SchemaVersion, StartedAt: now, FinishedAt: now.Add(time.Second), Status: model.ScanComplete,
		Roots:      []model.Root{{ID: "global", Label: "Global", Path: "/tmp/skills", Kind: model.RootGlobal, Known: true, Status: model.RootAvailable}},
		Skills:     []model.Skill{{ID: "s1", Name: "alpha", Path: "/tmp/skills/alpha", RootID: "global", RootKind: model.RootGlobal, Locked: true, HasSkillMD: true, DescTokens: 4, ScannedAt: now}},
		LockSkills: []model.LockSkill{{Name: "alpha", Source: "owner/repo"}},
		AgentLinks: []model.AgentLink{{SkillID: "s1", RootID: "global", Mode: model.LinkLinked, Visible: true}},
		CLIs:       []model.CLIInfo{{ID: "cli:jq", Name: "jq", Path: "/opt/homebrew/bin/jq", Owner: "Homebrew", Status: "duplicate", DuplicateCount: 1, Paths: []model.CLIPath{{Path: "/opt/homebrew/bin/jq", Owner: "Homebrew", Effective: true}, {Path: "/usr/local/bin/jq", Owner: "Homebrew"}}, ScannedAt: now}},
	}
	if err := db.SaveScan(context.Background(), want); err != nil {
		t.Fatal(err)
	}
	got, ok, err := db.LoadLatest(context.Background())
	if err != nil || !ok {
		t.Fatalf("load latest: ok=%v err=%v", ok, err)
	}
	if got.Status != want.Status || len(got.Skills) != 1 || got.Skills[0].Name != "alpha" || len(got.LockSkills) != 1 || len(got.CLIs) != 1 || got.CLIs[0].Name != "jq" || len(got.CLIs[0].Paths) != 2 {
		t.Fatalf("loaded scan = %#v", got)
	}
	updates := model.UpdateReport{CheckedAt: now, Items: []model.UpdateItem{{Kind: "cli", Name: "jq", State: "available"}}, Available: 1}
	if err := db.SaveUpdates(context.Background(), updates); err != nil {
		t.Fatal(err)
	}
	loadedUpdates, ok, err := db.LoadUpdates(context.Background())
	if err != nil || !ok || loadedUpdates.Available != 1 || len(loadedUpdates.Items) != 1 {
		t.Fatalf("loaded updates = %#v ok=%v err=%v", loadedUpdates, ok, err)
	}
	if err := db.SaveUpdates(context.Background(), model.UpdateReport{}); err != nil {
		t.Fatal(err)
	}
	if _, ok, err := db.LoadUpdates(context.Background()); err != nil || ok {
		t.Fatalf("empty updates should be absent: ok=%v err=%v", ok, err)
	}
	settings := model.Settings{ProjectWorkspaces: []string{"/tmp/project"}, ShowAgentBuiltins: true}
	if err := db.SaveSettings(context.Background(), settings); err != nil {
		t.Fatal(err)
	}
	gotSettings, err := db.LoadSettings(context.Background())
	if err != nil || len(gotSettings.ProjectWorkspaces) != 1 || !gotSettings.ShowAgentBuiltins {
		t.Fatalf("loaded settings = %#v err=%v", gotSettings, err)
	}
}

func TestLegacySettingsEnableAutomaticUpdateChecks(t *testing.T) {
	db, err := store.Open(filepath.Join(t.TempDir(), "ledger.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := db.SaveSettings(context.Background(), model.Settings{ProjectWorkspaces: []string{"/tmp/project"}}); err != nil {
		t.Fatal(err)
	}
	settings, err := db.LoadSettings(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !settings.AutoCheckUpdates || settings.UpdateCheckIntervalHr != 24 {
		t.Fatalf("legacy settings not migrated: %#v", settings)
	}
}
