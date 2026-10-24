package updates_test

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"testing"

	"larder/internal/core/model"
	"larder/internal/core/updates"
	"larder/internal/platform"
)

type fakeRunner struct {
	commands map[string]string
}

func (f fakeRunner) Find(name string, _ []string) (string, bool) {
	_, ok := f.commands[name]
	return "/fake/" + name, ok
}

func (f fakeRunner) Run(_ context.Context, path string, _ []string, _ []string) ([]byte, []byte, error) {
	return []byte(f.commands[path[len("/fake/"):]]), nil, nil
}

func TestCheckMapsOutdatedPackages(t *testing.T) {
	runner := fakeRunner{commands: map[string]string{
		"brew": `{"formulae":[{"name":"jq","current_version":"1.8.0"}]}`,
		"npm":  `{"@scope/tool":{"current":"1.0.0","latest":"2.0.0"}}`,
	}}
	clis := []model.CLIInfo{
		{ID: "cli:jq", Name: "jq", Owner: "Homebrew", OwnerDetail: "jq", Version: "1.7.1"},
		{ID: "cli:foo", Name: "foo", Owner: "Homebrew", OwnerDetail: "foo", Version: "1.0.0"},
		{ID: "cli:tool", Name: "tool", Owner: "npm", OwnerDetail: "@scope/tool", Version: "1.0.0"},
	}
	report := updates.Check(context.Background(), updates.Options{CLIs: clis, Runner: runner, Dirs: []string{"/fake"}})
	if report.Available != 2 || report.Current != 1 {
		t.Fatalf("report = %#v, want 2 available and 1 current", report)
	}
	got := map[string]string{}
	for _, item := range report.Items {
		got[item.Name+":"+item.State] = item.LatestVersion
	}
	if got["jq:available"] != "1.8.0" || got["@scope/tool:available"] != "2.0.0" {
		t.Fatalf("items = %#v", got)
	}
}

type fakeFetcher struct {
	content map[string][]byte
}

func (f fakeFetcher) Fetch(_ context.Context, url string) ([]byte, error) {
	value, ok := f.content[url]
	if !ok {
		return nil, context.Canceled
	}
	return value, nil
}

func TestCheckComparesWholeSkillFoldersWithSourceHead(t *testing.T) {
	root := t.TempDir()
	makeSkill := func(name, content string) model.Skill {
		dir := filepath.Join(root, name)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "notes.txt"), []byte("same\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		return model.Skill{
			Name: name, Path: dir, FolderHash: folderHash(t, dir), Locked: true, SourceType: "github",
			Source: "owner/repo", SourceURL: "https://github.com/owner/repo.git", SkillPath: "skills/" + name + "/SKILL.md",
		}
	}
	alpha := makeSkill("alpha", "local alpha\n")
	beta := makeSkill("beta", "local beta\n")
	fetcher := fakeFetcher{content: map[string][]byte{
		"https://codeload.github.com/owner/repo/tar.gz/HEAD": makeTar(t, "repo-HEAD", map[string]string{
			"skills/alpha/SKILL.md":  "local alpha\n",
			"skills/alpha/notes.txt": "same\n",
			"skills/beta/SKILL.md":   "remote beta\n",
			"skills/beta/notes.txt":  "same\n",
		}),
	}}
	report := updates.Check(context.Background(), updates.Options{Skills: []model.Skill{alpha, beta}, FS: platform.OSFileSystem{}, Fetcher: fetcher})
	if report.SkillScanned != 2 || report.SkillChanged != 1 || report.SkillCurrent != 1 {
		t.Fatalf("report = %#v", report)
	}
	if len(report.Items) != 2 || report.Items[0].Kind != "skill" {
		t.Fatalf("items = %#v", report.Items)
	}
}

func makeTar(t *testing.T, root string, files map[string]string) []byte {
	t.Helper()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	names := make([]string, 0, len(files))
	for name := range files {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		content := []byte(files[name])
		if err := tw.WriteHeader(&tar.Header{Name: root + "/" + name, Mode: 0o644, Size: int64(len(content)), Typeflag: tar.TypeReg}); err != nil {
			t.Fatal(err)
		}
		if _, err := tw.Write(content); err != nil {
			t.Fatal(err)
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func folderHash(t *testing.T, root string) string {
	t.Helper()
	var entries []struct{ path, hash string }
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		sum := sha256.Sum256(content)
		entries = append(entries, struct{ path, hash string }{path: rel, hash: hex.EncodeToString(sum[:])})
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].path < entries[j].path })
	h := sha256.New()
	for _, entry := range entries {
		fmt.Fprintf(h, "%s\x00%s\n", entry.path, entry.hash)
	}
	return hex.EncodeToString(h.Sum(nil))
}

func TestCheckMapsWingetUpgrades(t *testing.T) {
	runner := fakeRunner{commands: map[string]string{
		"winget": "Name    Id              Version   Available   Source\n-----------------------------------------------------------\nGit     Git.Git         2.44.0    2.45.0      winget\n",
	}}
	clis := []model.CLIInfo{{ID: "cli:git", Name: "git", Owner: "winget", OwnerDetail: "Git.Git", Version: "2.44.0"}}
	report := updates.Check(context.Background(), updates.Options{CLIs: clis, Runner: runner, Dirs: []string{"/fake"}})
	if report.Available != 1 || report.Current != 0 || len(report.Items) != 1 {
		t.Fatalf("report = %#v", report)
	}
	if report.Items[0].LatestVersion != "2.45.0" || report.Items[0].Owner != "winget" {
		t.Fatalf("item = %#v", report.Items[0])
	}
}
