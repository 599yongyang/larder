package larder

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"
	"larder/internal/core/backup"
	"larder/internal/core/catalog"
	"larder/internal/core/clis"
	"larder/internal/core/commands"
	"larder/internal/core/instructions"
	"larder/internal/core/inventory"
	coremcp "larder/internal/core/mcp"
	"larder/internal/core/model"
	"larder/internal/core/ops"
	"larder/internal/core/sessions"
	"larder/internal/core/skills"
	"larder/internal/core/store"
	"larder/internal/core/trust"
	"larder/internal/core/xray"
	"larder/internal/platform"
)

// CatalogInstall records where a skill discovered on skills.sh was installed,
// so the market can tell an installed skill apart from a same-named one.
type CatalogInstall struct {
	Path        string    `json:"path"`
	Source      string    `json:"source"`
	SkillID     string    `json:"skillId"`
	InstalledAt time.Time `json:"installedAt"`
}

type AppState struct {
	SchemaVersion string             `json:"schemaVersion"`
	Snapshot      model.ScanResult   `json:"snapshot"`
	Updates       model.UpdateReport `json:"updates"`
	Cached        bool               `json:"cached"`
	Scanning      bool               `json:"scanning"`
	LastError     string             `json:"lastError,omitempty"`
	UpdatedAt     time.Time          `json:"updatedAt"`
}

var Version = "0.0.8"

type MCPTestResult struct {
	OK      bool   `json:"ok"`
	Message string `json:"message"`
}

type sessionCacheEntry struct {
	items []model.Session
	at    time.Time
}

type usageCacheEntry struct {
	items []model.ResourceUsage
	at    time.Time
}

type AppInfo struct {
	Version       string `json:"version"`
	SchemaVersion string `json:"schemaVersion"`
	GoVersion     string `json:"goVersion"`
	Platform      string `json:"platform"`
	DataDir       string `json:"dataDir"`
	HomeDir       string `json:"homeDir"`
}

type AppService struct {
	core          *inventory.Service
	catalog       *catalog.Client
	catalogByName map[string]catalog.Skill
	app           *application.App

	installsPath       string
	installs           map[string]CatalogInstall
	commandPlans       map[string]model.CommandWritePlan
	commandDeletePlans map[string]model.CommandDeletePlan
	skillEditPlans     map[string]model.SkillEditPlan
	skillCreatePlans   map[string]model.SkillCreatePlan
	skillRenamePlans   map[string]model.SkillRenamePlan
	trust              *trust.Client
	sessionMu          sync.Mutex
	sessionCache       map[string]sessionCacheEntry
	usageCache         map[string]usageCacheEntry
	disabledPath       string
	disabled           []ops.DetachedMCP
	skillSourcesPath   string
	skillSources       map[string]skills.SourceOverride
	dataDir            string
	homeDir            string
	locale             string
	localeListener     func(string)

	mu         sync.RWMutex
	ctx        context.Context
	cancel     context.CancelFunc
	scanCancel context.CancelFunc
	state      AppState
}

func NewAppService() (*AppService, error) {
	paths := platform.LocalPaths{}
	dataDir, err := paths.AppDataDir("Larder")
	if err != nil {
		return nil, err
	}
	db, err := store.Open(filepath.Join(dataDir, "ledger.sqlite"))
	if err != nil {
		return nil, err
	}
	core := inventory.NewService(paths, platform.OSFileSystem{}, db)
	core.SetSnapshotRoot(filepath.Join(dataDir, "snapshots"))
	homeDir, _ := paths.HomeDir()
	catalogClient := catalog.NewClient()
	catalogByName := map[string]catalog.Skill{}
	for _, item := range catalog.LocalPopular() {
		catalogByName[strings.ToLower(item.Name)] = item
	}
	service := &AppService{core: core, catalog: catalogClient, catalogByName: catalogByName, installsPath: filepath.Join(dataDir, "catalog-installs.json"), installs: map[string]CatalogInstall{}, commandPlans: map[string]model.CommandWritePlan{}, commandDeletePlans: map[string]model.CommandDeletePlan{}, skillEditPlans: map[string]model.SkillEditPlan{}, skillCreatePlans: map[string]model.SkillCreatePlan{}, skillRenamePlans: map[string]model.SkillRenamePlan{}, trust: trust.New(), sessionCache: map[string]sessionCacheEntry{}, usageCache: map[string]usageCacheEntry{}, disabledPath: filepath.Join(dataDir, "disabled-mcp.json"), skillSourcesPath: filepath.Join(dataDir, "skill-sources.json"), dataDir: dataDir, homeDir: homeDir}
	service.loadCatalogInstalls()
	service.loadDisabledMCPs()
	service.loadSkillSources()
	service.state = AppState{SchemaVersion: model.SchemaVersion, Snapshot: model.ScanResult{SchemaVersion: model.SchemaVersion}, UpdatedAt: time.Now()}
	return service, nil
}

// SetApp wires the Wails application after it is constructed. Keeping it
// optional lets service tests and non-GUI callers use the service without a
// desktop shell.
func (s *AppService) SetApp(app *application.App) {
	s.mu.Lock()
	s.app = app
	s.mu.Unlock()
}

func (s *AppService) Startup() {
	settings, _ := s.core.Settings(s.context())
	_ = s.applyGlobalShortcut(settings)
	retention := settings.SnapshotRetention
	if retention <= 0 {
		retention = 50
	}
	_ = s.core.PruneTrash(s.context(), settings.TrashRetentionDays)
	_ = s.core.PruneSnapshots(retention)
	_ = s.core.PruneTimeline(s.context())
	s.mu.Lock()
	if s.ctx != nil {
		s.mu.Unlock()
		return
	}
	s.ctx, s.cancel = context.WithCancel(context.Background())
	ctx := s.ctx
	s.mu.Unlock()
	if cached, ok, err := s.core.LoadCache(ctx); err == nil && ok {
		s.enrichSkillSources(&cached)
		s.updateState(func(state *AppState) {
			state.Snapshot = cached
			state.Updates = s.core.Updates()
			state.Cached = true
			state.LastError = ""
		})
	}
	scanCtx, cancel := context.WithCancel(ctx)
	s.mu.Lock()
	s.scanCancel = cancel
	s.mu.Unlock()
	go s.refresh(scanCtx)
}

func (s *AppService) refresh(parent context.Context) {
	s.mu.Lock()
	if s.state.Scanning {
		s.mu.Unlock()
		return
	}
	s.state.Scanning = true
	s.state.Snapshot.Status = model.ScanPartial
	s.mu.Unlock()
	s.emit()
	result, err := s.core.Scan(parent)
	s.mu.Lock()
	if s.scanCancel != nil {
		s.scanCancel = nil
	}
	s.mu.Unlock()
	s.mu.Lock()
	s.state.Scanning = false
	s.state.Snapshot = result
	s.state.Updates = s.core.Updates()
	s.state.Cached = false
	if err != nil && !errors.Is(err, context.Canceled) {
		s.state.LastError = err.Error()
	} else {
		s.state.LastError = ""
	}
	s.mu.Unlock()
	s.emit()
}

// State returns a snapshot suitable for the Wails binding. The result is
// explicitly marked cached or live so stale data is never presented as fresh.
func (s *AppService) State() AppState { s.mu.RLock(); defer s.mu.RUnlock(); return s.state }
func (s *AppService) PlanSkillCreate(name, content string) (model.SkillCreatePlan, error) {
	name, err := cleanSkillName(name)
	if err != nil {
		return model.SkillCreatePlan{}, err
	}
	root, err := s.sharedSkillRoot()
	if err != nil {
		return model.SkillCreatePlan{}, err
	}
	path := filepath.Join(root, name)
	if _, err := os.Stat(path); err == nil {
		return model.SkillCreatePlan{}, errors.New("a Skill with this name already exists")
	}
	if len(content) > 512*1024 {
		return model.SkillCreatePlan{}, errors.New("skill content is too large")
	}
	plan := model.SkillCreatePlan{ID: fmt.Sprintf("skill-create-%d", time.Now().UnixNano()), Name: name, Path: path, Content: content, Warnings: validateSkillMarkdown(content), CreatedAt: time.Now()}
	s.mu.Lock()
	if s.skillCreatePlans == nil {
		s.skillCreatePlans = map[string]model.SkillCreatePlan{}
	}
	s.skillCreatePlans[plan.ID] = plan
	s.mu.Unlock()
	return plan, nil
}

func (s *AppService) ApplySkillCreate(planID string) (model.Operation, error) {
	s.mu.Lock()
	plan, ok := s.skillCreatePlans[planID]
	s.mu.Unlock()
	if !ok {
		return model.Operation{}, errors.New("skill create plan not found or expired")
	}
	root, err := s.sharedSkillRoot()
	if err != nil {
		return model.Operation{}, err
	}
	if filepath.Dir(plan.Path) != root || filepath.Base(plan.Path) != plan.Name {
		return model.Operation{}, errors.New("skill create path is outside the shared library")
	}
	if _, err := os.Stat(plan.Path); err == nil {
		return model.Operation{}, errors.New("skill target already exists")
	}
	snapshotDir := filepath.Join(s.dataDir, "snapshots", "skill-create-"+plan.ID)
	if err := os.MkdirAll(snapshotDir, 0o700); err != nil {
		return model.Operation{}, err
	}
	manifest := ops.SnapshotManifest{ID: filepath.Base(snapshotDir), PlanID: plan.ID, CreatedAt: time.Now(), Targets: []ops.SnapshotTarget{{Target: plan.Path, Action: "install_skill"}}}
	payload, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return model.Operation{}, err
	}
	if err := os.WriteFile(filepath.Join(snapshotDir, "manifest.json"), payload, 0o600); err != nil {
		return model.Operation{}, err
	}
	if err := os.MkdirAll(plan.Path, 0o755); err != nil {
		return model.Operation{}, err
	}
	if err := writeFileAtomically(filepath.Join(plan.Path, "SKILL.md"), plan.Content); err != nil {
		_ = os.RemoveAll(plan.Path)
		return model.Operation{}, err
	}
	now := time.Now()
	op := model.Operation{ID: fmt.Sprintf("op-skill-create-%d", now.UnixNano()), PlanID: plan.ID, Operation: "skill_create", Status: "success", Message: "Skill created", Changes: []string{"skill:" + plan.Path}, SnapshotPath: snapshotDir, CreatedAt: now}
	if store := s.core.Store(); store != nil {
		_ = store.SaveOperation(s.context(), op)
	}
	s.mu.Lock()
	delete(s.skillCreatePlans, planID)
	s.mu.Unlock()
	s.rescanAfterSkillMutation()
	return op, nil
}

func (s *AppService) CancelSkillCreate(planID string) {
	s.mu.Lock()
	delete(s.skillCreatePlans, planID)
	s.mu.Unlock()
}

func (s *AppService) PlanSkillRename(skillID, newName string) (model.SkillRenamePlan, error) {
	skill, err := s.findSkill(skillID)
	if err != nil {
		return model.SkillRenamePlan{}, err
	}
	newName, err = cleanSkillName(newName)
	if err != nil {
		return model.SkillRenamePlan{}, err
	}
	from := skill.Path
	to := filepath.Join(filepath.Dir(from), newName)
	if from == to {
		return model.SkillRenamePlan{}, errors.New("skill name is unchanged")
	}
	if _, err := os.Stat(to); err == nil {
		return model.SkillRenamePlan{}, errors.New("a Skill directory with this name already exists")
	}
	plan := model.SkillRenamePlan{ID: fmt.Sprintf("skill-rename-%d", time.Now().UnixNano()), SkillID: skill.ID, Name: newName, From: from, To: to, CreatedAt: time.Now()}
	s.mu.Lock()
	if s.skillRenamePlans == nil {
		s.skillRenamePlans = map[string]model.SkillRenamePlan{}
	}
	s.skillRenamePlans[plan.ID] = plan
	s.mu.Unlock()
	return plan, nil
}

func (s *AppService) ApplySkillRename(planID string) (model.Operation, error) {
	s.mu.Lock()
	plan, ok := s.skillRenamePlans[planID]
	s.mu.Unlock()
	if !ok {
		return model.Operation{}, errors.New("skill rename plan not found or expired")
	}
	if _, err := os.Stat(plan.To); err == nil {
		return model.Operation{}, errors.New("skill rename target already exists")
	}
	if _, err := os.Stat(plan.From); err != nil {
		return model.Operation{}, err
	}
	snapshotDir := filepath.Join(s.dataDir, "snapshots", "skill-rename-"+plan.ID)
	if err := os.MkdirAll(snapshotDir, 0o700); err != nil {
		return model.Operation{}, err
	}
	manifest := ops.SnapshotManifest{ID: filepath.Base(snapshotDir), PlanID: plan.ID, CreatedAt: time.Now()}
	payload, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return model.Operation{}, err
	}
	if err := os.WriteFile(filepath.Join(snapshotDir, "manifest.json"), payload, 0o600); err != nil {
		return model.Operation{}, err
	}
	if err := ops.SnapshotDirectoryRename(snapshotDir, plan.From, plan.To); err != nil {
		return model.Operation{}, err
	}
	if err := os.Rename(plan.From, plan.To); err != nil {
		return model.Operation{}, err
	}
	if err := renameSkillFrontmatter(filepath.Join(plan.To, "SKILL.md"), plan.Name); err != nil {
		_ = os.Rename(plan.To, plan.From)
		return model.Operation{}, err
	}
	now := time.Now()
	op := model.Operation{ID: fmt.Sprintf("op-skill-rename-%d", now.UnixNano()), PlanID: plan.ID, Operation: "skill_rename", Status: "success", Message: "Skill renamed", Changes: []string{"skill:" + plan.From, "skill:" + plan.To}, SnapshotPath: snapshotDir, CreatedAt: now}
	if store := s.core.Store(); store != nil {
		_ = store.SaveOperation(s.context(), op)
	}
	s.mu.Lock()
	delete(s.skillRenamePlans, planID)
	s.mu.Unlock()
	s.rescanAfterSkillMutation()
	return op, nil
}

func (s *AppService) CancelSkillRename(planID string) {
	s.mu.Lock()
	delete(s.skillRenamePlans, planID)
	s.mu.Unlock()
}

func (s *AppService) sharedSkillRoot() (string, error) {
	home, err := (platform.LocalPaths{}).HomeDir()
	if err != nil {
		return "", err
	}
	for _, root := range (platform.LocalPaths{}).DefaultRoots(home) {
		if root.ID == "global" {
			return root.Path, nil
		}
	}
	return filepath.Join(home, ".agents", "skills"), nil
}

func cleanSkillName(name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" || name == "." || name == ".." || strings.ContainsAny(name, "/\\") {
		return "", errors.New("invalid Skill name")
	}
	return name, nil
}

func (s *AppService) rescanAfterSkillMutation() {
	_, _ = s.Scan()
}

func (s *AppService) SkillContent(skillID string) (string, error) {
	skill, err := s.findSkill(skillID)
	if err != nil {
		return "", err
	}
	path := filepath.Join(skill.Path, "SKILL.md")
	raw, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	if len(raw) > 512*1024 {
		return "", errors.New("skill file is too large")
	}
	return string(raw), nil
}

func (s *AppService) PlanSkillEdit(skillID, content string) (model.SkillEditPlan, error) {
	skill, err := s.findSkill(skillID)
	if err != nil {
		return model.SkillEditPlan{}, err
	}
	if len(content) > 512*1024 {
		return model.SkillEditPlan{}, errors.New("skill content is too large")
	}
	path := filepath.Join(skill.Path, "SKILL.md")
	current, readErr := os.ReadFile(path)
	if readErr != nil && !errors.Is(readErr, os.ErrNotExist) {
		return model.SkillEditPlan{}, readErr
	}
	if readErr == nil && string(current) == content {
		return model.SkillEditPlan{}, errors.New("skill content has not changed")
	}
	plan := model.SkillEditPlan{ID: fmt.Sprintf("skill-edit-%d", time.Now().UnixNano()), SkillID: skill.ID, Path: path, Content: content, CurrentContent: string(current), Exists: readErr == nil, CreatedAt: time.Now()}
	plan.Warnings = validateSkillMarkdown(content)
	s.mu.Lock()
	if s.skillEditPlans == nil {
		s.skillEditPlans = map[string]model.SkillEditPlan{}
	}
	s.skillEditPlans[plan.ID] = plan
	s.mu.Unlock()
	return plan, nil
}

func (s *AppService) ApplySkillEdit(planID string) (model.Operation, error) {
	s.mu.Lock()
	plan, ok := s.skillEditPlans[planID]
	s.mu.Unlock()
	if !ok {
		return model.Operation{}, errors.New("skill edit plan not found or expired")
	}
	skill, err := s.findSkill(plan.SkillID)
	if err != nil {
		return model.Operation{}, err
	}
	expected := filepath.Join(skill.Path, "SKILL.md")
	if filepath.Clean(plan.Path) != filepath.Clean(expected) {
		return model.Operation{}, errors.New("skill edit path is outside the Skill directory")
	}
	snapshotDir := filepath.Join(s.dataDir, "snapshots", "skill-edit-"+plan.ID)
	if err := os.MkdirAll(snapshotDir, 0o700); err != nil {
		return model.Operation{}, err
	}
	manifest := ops.SnapshotManifest{ID: filepath.Base(snapshotDir), PlanID: plan.ID, CreatedAt: time.Now()}
	payload, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return model.Operation{}, err
	}
	if err := os.WriteFile(filepath.Join(snapshotDir, "manifest.json"), payload, 0o600); err != nil {
		return model.Operation{}, err
	}
	if err := ops.SnapshotFile(snapshotDir, plan.Path); err != nil {
		return model.Operation{}, err
	}
	if err := writeFileAtomically(plan.Path, plan.Content); err != nil {
		return model.Operation{}, err
	}
	now := time.Now()
	op := model.Operation{ID: fmt.Sprintf("op-skill-edit-%d", now.UnixNano()), PlanID: plan.ID, Operation: "skill_edit", Status: "success", Message: "Skill edited", Changes: []string{"skill:" + plan.Path}, SnapshotPath: snapshotDir, CreatedAt: now}
	if store := s.core.Store(); store != nil {
		if err := store.SaveOperation(s.context(), op); err != nil {
			return op, err
		}
	}
	s.mu.Lock()
	delete(s.skillEditPlans, planID)
	s.mu.Unlock()
	s.rescanAfterSkillMutation()
	return op, nil
}

func (s *AppService) CancelSkillEdit(planID string) {
	s.mu.Lock()
	delete(s.skillEditPlans, planID)
	s.mu.Unlock()
}

func (s *AppService) findSkill(skillID string) (*model.Skill, error) {
	state := s.State()
	for index := range state.Snapshot.Skills {
		if state.Snapshot.Skills[index].ID == skillID {
			return &state.Snapshot.Skills[index], nil
		}
	}
	return nil, messageError("skills.skillNotFound", "skill not found", nil)
}

func renameSkillFrontmatter(path, name string) error {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	normalized := strings.ReplaceAll(string(raw), "\r\n", "\n")
	if !strings.HasPrefix(normalized, "---\n") {
		return nil
	}
	end := strings.Index(normalized[4:], "\n---")
	if end < 0 {
		return nil
	}
	header := normalized[4 : 4+end]
	updated := regexp.MustCompile(`(?m)^name:.*$`).ReplaceAllString(header, "name: "+strconv.Quote(name))
	if updated == header {
		updated = strings.TrimRight(header, "\n") + "\nname: " + strconv.Quote(name)
	}
	content := "---\n" + updated + normalized[4+end:]
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		return err
	}
	return nil
}

func sameHashes(a, b map[string]string) bool {
	if len(a) != len(b) {
		return false
	}
	for path, hash := range a {
		if b[path] != hash {
			return false
		}
	}
	return true
}

func validateSkillMarkdown(content string) []string {
	normalized := strings.ReplaceAll(content, "\r\n", "\n")
	if !strings.HasPrefix(normalized, "---\n") {
		return []string{"SKILL.md should start with YAML frontmatter"}
	}
	end := strings.Index(normalized[4:], "\n---")
	if end < 0 {
		return []string{"frontmatter is not closed with ---"}
	}
	warnings := make([]string, 0)
	header := normalized[4 : 4+end]
	if !strings.Contains(header, "name:") {
		warnings = append(warnings, "frontmatter is missing name")
	}
	if !strings.Contains(header, "description:") {
		warnings = append(warnings, "frontmatter is missing description")
	}
	return warnings
}

type nativeCLIAdapter struct {
	Package        string
	ReleaseRepo    string
	PyPI           string
	VersionURL     string
	VersionPattern string
	UpdateArgs     []string
}

var nativeCLIAdapters = map[string]nativeCLIAdapter{
	"codex":        {Package: "@openai/codex", UpdateArgs: []string{"update"}},
	"claude":       {Package: "@anthropic-ai/claude-code", UpdateArgs: []string{"update"}},
	"gemini":       {Package: "@google/gemini-cli"},
	"opencode":     {Package: "opencode-ai"},
	"cline":        {Package: "cline"},
	"copilot":      {Package: "@github/copilot"},
	"amp":          {Package: "@sourcegraph/amp"},
	"qwen":         {Package: "@qwen-code/qwen-code"},
	"cursor-agent": {VersionURL: "https://cursor.com/install", VersionPattern: `downloads\.cursor\.com/lab/([^/]+)/`, UpdateArgs: []string{"update"}},
	"omp":          {Package: "@oh-my-pi/pi-coding-agent", UpdateArgs: []string{"update"}},
	"openclaw":     {Package: "openclaw"},
	"goose":        {ReleaseRepo: "block/goose"},
	"crush":        {ReleaseRepo: "charmbracelet/crush"},
	"aider":        {PyPI: "aider-chat"},
}

func (s *AppService) NativeCLIUpdateCheck(cliID string) (model.NativeCLIUpdateInfo, error) {
	state := s.State()
	var cli *model.CLIInfo
	for index := range state.Snapshot.CLIs {
		if state.Snapshot.CLIs[index].ID == cliID {
			cli = &state.Snapshot.CLIs[index]
			break
		}
	}
	if cli == nil {
		return model.NativeCLIUpdateInfo{}, errors.New("CLI not found")
	}
	adapter, ok := nativeCLIAdapters[strings.ToLower(cli.Name)]
	if !ok || (adapter.Package == "" && adapter.VersionURL == "" && adapter.ReleaseRepo == "" && adapter.PyPI == "") {
		return model.NativeCLIUpdateInfo{}, errors.New("this CLI has no update source")
	}
	current := cli.Version
	if current == "" {
		current, _ = clis.ProbeVersion(s.context(), cli.Path)
	}
	ctx, cancel := context.WithTimeout(s.context(), 15*time.Second)
	defer cancel()
	latest, err := nativeLatestVersion(ctx, adapter)
	if err != nil {
		return model.NativeCLIUpdateInfo{CLIID: cli.ID, Current: current, CanUpdate: len(adapter.UpdateArgs) > 0, UpdateCommand: strings.TrimSpace(cli.Name + " " + strings.Join(adapter.UpdateArgs, " ")), Error: err.Error(), CheckedAt: time.Now()}, nil
	}
	available := current != "" && compareVersions(latest, current) > 0
	return model.NativeCLIUpdateInfo{CLIID: cli.ID, Current: current, Latest: latest, Available: available, CanUpdate: len(adapter.UpdateArgs) > 0, UpdateCommand: strings.TrimSpace(cli.Name + " " + strings.Join(adapter.UpdateArgs, " ")), CheckedAt: time.Now()}, nil
}

func nativeLatestVersion(ctx context.Context, adapter nativeCLIAdapter) (string, error) {
	if adapter.VersionURL != "" {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, adapter.VersionURL, nil)
		if err != nil {
			return "", err
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return "", err
		}
		defer resp.Body.Close()
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return "", fmt.Errorf("latest version: HTTP %d", resp.StatusCode)
		}
		body, err := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
		if err != nil {
			return "", err
		}
		pattern, err := regexp.Compile(adapter.VersionPattern)
		if err != nil {
			return "", err
		}
		match := pattern.FindSubmatch(body)
		if len(match) < 2 {
			return "", errors.New("latest version not found")
		}
		return string(match[1]), nil
	}
	if adapter.ReleaseRepo != "" {
		endpoint := "https://github.com/" + adapter.ReleaseRepo + "/releases/latest"
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
		if err != nil {
			return "", err
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return "", err
		}
		defer resp.Body.Close()
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return "", fmt.Errorf("latest release: HTTP %d", resp.StatusCode)
		}
		pattern := regexp.MustCompile(`/releases/tag/([^/?#]+)`)
		match := pattern.FindStringSubmatch(resp.Request.URL.String())
		if len(match) < 2 {
			return "", errors.New("latest release not found")
		}
		return match[1], nil
	}
	if adapter.PyPI != "" {
		endpoint := "https://pypi.org/pypi/" + url.PathEscape(adapter.PyPI) + "/json"
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
		if err != nil {
			return "", err
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return "", err
		}
		defer resp.Body.Close()
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return "", fmt.Errorf("latest version: HTTP %d", resp.StatusCode)
		}
		var payload struct {
			Info struct {
				Version string `json:"version"`
			} `json:"info"`
		}
		if err := json.NewDecoder(io.LimitReader(resp.Body, 2<<20)).Decode(&payload); err != nil {
			return "", err
		}
		return payload.Info.Version, nil
	}
	endpoint := "https://registry.npmjs.org/" + url.PathEscape(adapter.Package) + "/latest"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return "", err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("latest version: HTTP %d", resp.StatusCode)
	}
	var payload struct {
		Version string `json:"version"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&payload); err != nil {
		return "", err
	}
	return payload.Version, nil
}

func compareVersions(a, b string) int {
	parse := func(value string) []int {
		value = strings.TrimPrefix(strings.TrimSpace(value), "v")
		parts := strings.Split(value, ".")
		out := make([]int, 0, len(parts))
		for _, part := range parts {
			n := 0
			for _, r := range part {
				if r < '0' || r > '9' {
					break
				}
				n = n*10 + int(r-'0')
			}
			out = append(out, n)
		}
		return out
	}
	left, right := parse(a), parse(b)
	for index := 0; index < len(left) || index < len(right); index++ {
		l, r := 0, 0
		if index < len(left) {
			l = left[index]
		}
		if index < len(right) {
			r = right[index]
		}
		if l != r {
			if l > r {
				return 1
			}
			return -1
		}
	}
	return 0
}

func (s *AppService) NativeCLIUpdate(cliID string) (string, error) {
	state := s.State()
	var cli *model.CLIInfo
	for index := range state.Snapshot.CLIs {
		if state.Snapshot.CLIs[index].ID == cliID {
			cli = &state.Snapshot.CLIs[index]
			break
		}
	}
	if cli == nil {
		return "", errors.New("CLI not found")
	}
	if cli.Status == "broken" {
		return "", errors.New("CLI path is broken")
	}
	adapter, ok := nativeCLIAdapters[strings.ToLower(cli.Name)]
	if !ok || len(adapter.UpdateArgs) == 0 {
		return "", errors.New("this CLI has no native update adapter")
	}
	args := adapter.UpdateArgs
	if cli.Path == "" {
		return "", errors.New("CLI path is unavailable")
	}
	ctx, cancel := context.WithTimeout(s.context(), 10*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, cli.Path, args...)
	cmd.Stdin = nil
	cmd.Env = append(os.Environ(), "NO_COLOR=1")
	out, err := cmd.CombinedOutput()
	output := strings.TrimSpace(string(out))
	if len(output) > 16<<10 {
		output = output[:16<<10] + "…"
	}
	if ctx.Err() != nil {
		return output, errors.New("native CLI update timed out")
	}
	if err != nil {
		return output, fmt.Errorf("native CLI update failed: %w", err)
	}
	_, _ = s.Scan()
	return output, nil
}

func (s *AppService) CLIVersion(cliID string) (string, error) {
	state := s.State()
	for _, cli := range state.Snapshot.CLIs {
		if cli.ID != cliID {
			continue
		}
		if cli.Version != "" {
			return cli.Version, nil
		}
		if cli.Path == "" {
			return "", nil
		}
		return clis.ProbeVersion(s.context(), cli.Path)
	}
	return "", errors.New("CLI not found")
}

func (s *AppService) SkillAudit(skillID string) (model.SkillAudit, error) {
	var skill *model.Skill
	state := s.State()
	for index := range state.Snapshot.Skills {
		if state.Snapshot.Skills[index].ID == skillID {
			skill = &state.Snapshot.Skills[index]
			break
		}
	}
	if skill == nil {
		return model.SkillAudit{}, messageError("skills.skillNotFound", "skill not found", nil)
	}
	source := trust.NormalizeSource(skill.Source, skill.SourceURL)
	audit, err := s.trust.Lookup(s.context(), source, skill.Name)
	if err != nil {
		return model.SkillAudit{}, err
	}
	audit.SkillID = skill.ID
	audit.LocalHash = skill.FolderHash
	if baseline, ok, loadErr := s.core.Store().LoadSkillBaseline(s.context(), skill.Path); loadErr == nil && ok {
		audit.LocalBaselineAt = baseline.UpdatedAt
		if current, hashErr := ops.FileHashes(skill.Path); hashErr == nil {
			audit.LocalChanged = !sameHashes(current, baseline.Files)
		}
	}
	return audit, nil
}

func (s *AppService) SourceAudit(source, skill string) (model.SkillAudit, error) {
	audit, err := s.trust.Lookup(s.context(), trust.NormalizeSource(source, ""), skill)
	if err != nil {
		return model.SkillAudit{}, err
	}
	return audit, nil
}

func (s *AppService) applyGlobalShortcut(settings model.Settings) error {
	s.mu.RLock()
	app := s.app
	s.mu.RUnlock()
	if app == nil {
		return nil
	}
	_ = app.GlobalShortcut.UnregisterAll()
	if settings.GlobalShortcutEnabled != nil && !*settings.GlobalShortcutEnabled {
		return nil
	}
	accelerator := strings.TrimSpace(settings.GlobalShortcut)
	if accelerator == "" {
		accelerator = "CmdOrCtrl+Shift+L"
	}
	return app.GlobalShortcut.Register(accelerator, func() {
		s.emitWindowEvent("larder:open-command")
	})
}

func (s *AppService) AutostartEnabled() (bool, error) {
	if s.app == nil {
		return false, errors.New("app is not ready")
	}
	return s.app.Autostart.IsEnabled()
}

func (s *AppService) SetAutostart(enabled bool) error {
	if s.app == nil {
		return errors.New("app is not ready")
	}
	if enabled {
		return s.app.Autostart.Enable()
	}
	return s.app.Autostart.Disable()
}

func (s *AppService) SessionResumeCommand(session model.Session) (string, error) {
	id := strings.TrimSpace(session.ID)
	if id == "" {
		return "", errors.New("session id is missing")
	}
	var command string
	switch session.Agent {
	case "Claude Code":
		command = "claude --resume " + shellQuote(id)
	case "Codex":
		command = "codex resume " + shellQuote(id)
	default:
		return "", errors.New("resume is not supported for this agent yet")
	}
	if cwd := strings.TrimSpace(session.CWD); cwd != "" {
		command = "cd " + shellQuote(cwd) + " && " + command
	}
	return command, nil
}

func (s *AppService) OpenSessionInTerminal(session model.Session) error {
	command, err := s.SessionResumeCommand(session)
	if err != nil {
		return err
	}
	if runtime.GOOS != "darwin" {
		return errors.New("opening a terminal is only supported on macOS")
	}
	script := "tell application \"Terminal\"\nactivate\ndo script " + appleScriptString(command) + "\nend tell"
	return exec.Command("osascript", "-e", script).Run()
}

func appleScriptString(value string) string {
	value = strings.ReplaceAll(value, "\\", "\\\\")
	value = strings.ReplaceAll(value, "\"", "\\\"")
	return "\"" + value + "\""
}

func shellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\\''") + "'"
}

func (s *AppService) ExportSession(session model.Session) (string, error) {
	clean := filepath.Clean(strings.TrimSpace(session.Path))
	if clean == "" || clean == "." || !isSessionSourcePath(s.homeDir, clean) {
		return "", errors.New("session path is outside the supported session roots")
	}
	messages, err := sessions.ReadMessages(clean, 200)
	if err != nil {
		return "", err
	}
	if s.app == nil {
		return "", errors.New("app is not ready")
	}
	dialog := s.app.Dialog.SaveFile()
	dialog.SetFilename(safeSessionFilename(session.Title, session.ID) + ".md")
	dialog.AddFilter("Markdown", "*.md")
	if window := s.app.Window.Current(); window != nil {
		dialog.AttachToWindow(window)
	}
	path, err := dialog.PromptForSingleSelection()
	if err != nil || path == "" {
		return path, err
	}
	var body strings.Builder
	body.WriteString("# " + strings.TrimSpace(session.Title) + "\n\n")
	body.WriteString("## Metadata\n\n")
	body.WriteString("- Agent: " + session.Agent + "\n")
	body.WriteString("- Project: " + session.CWD + "\n")
	body.WriteString("- Model: " + session.Model + "\n")
	body.WriteString("- Messages: " + strconv.Itoa(session.Messages) + "\n")
	body.WriteString("- Started: " + session.StartedAt.Format(time.RFC3339) + "\n")
	body.WriteString("- Last active: " + session.LastAt.Format(time.RFC3339) + "\n\n")
	body.WriteString("## Recent messages\n\n")
	for _, message := range messages {
		role := "User"
		if message.Role == "assistant" {
			role = "Assistant"
		}
		if !message.At.IsZero() {
			body.WriteString("### " + role + " · " + message.At.Format(time.RFC3339) + "\n\n")
		} else {
			body.WriteString("### " + role + "\n\n")
		}
		body.WriteString(message.Text + "\n\n")
	}
	if err := os.WriteFile(path, []byte(body.String()), 0o600); err != nil {
		return "", err
	}
	return path, nil
}

func safeSessionFilename(title, id string) string {
	name := strings.TrimSpace(title)
	if name == "" {
		name = id
	}
	name = strings.Map(func(r rune) rune {
		if r < 32 || strings.ContainsRune(`/\:*?"<>|`, r) {
			return '-'
		}
		return r
	}, name)
	runes := []rune(name)
	if len(runes) > 80 {
		name = string(runes[:80])
	}
	return strings.TrimSpace(name) + "-" + id
}

type AppUpdateInfo struct {
	Available bool   `json:"available"`
	Version   string `json:"version"`
	Name      string `json:"name"`
	Notes     string `json:"notes"`
	Published string `json:"publishedAt"`
	Size      int64  `json:"size"`
}

func (s *AppService) CheckAppUpdate() (*AppUpdateInfo, error) {
	if s.app == nil || s.app.Updater == nil {
		return nil, errors.New("app updater is unavailable")
	}
	release, err := s.app.Updater.Check(s.context())
	if err != nil {
		return nil, err
	}
	if release == nil {
		return &AppUpdateInfo{Available: false}, nil
	}
	info := &AppUpdateInfo{
		Available: true,
		Version:   release.Version,
		Name:      release.Name,
		Notes:     release.Notes,
		Size:      release.Artifact.Size,
	}
	if !release.PublishedAt.IsZero() {
		info.Published = release.PublishedAt.Format(time.RFC3339)
	}
	return info, nil
}

func (s *AppService) DownloadAppUpdate() error {
	if s.app == nil || s.app.Updater == nil {
		return errors.New("app updater is unavailable")
	}
	return s.app.Updater.DownloadAndInstall(s.context())
}

func (s *AppService) RestartAppUpdate() error {
	if s.app == nil || s.app.Updater == nil {
		return errors.New("app updater is unavailable")
	}
	return s.app.Updater.Restart(s.context())
}

func (s *AppService) AgentUsage(agent string) []model.ResourceUsage {
	agent = strings.TrimSpace(agent)
	if agent == "" {
		return nil
	}
	key := "usage:agent:" + agent
	s.sessionMu.Lock()
	if cached, ok := s.usageCache[key]; ok && time.Since(cached.at) < 20*time.Second {
		items := append([]model.ResourceUsage(nil), cached.items...)
		s.sessionMu.Unlock()
		return items
	}
	s.sessionMu.Unlock()
	items := s.usageFromSessions(s.AgentSessions(agent))
	s.sessionMu.Lock()
	s.usageCache[key] = usageCacheEntry{items: append([]model.ResourceUsage(nil), items...), at: time.Now()}
	s.sessionMu.Unlock()
	return items
}

func (s *AppService) ProjectUsage(projectPath string) []model.ResourceUsage {
	if strings.TrimSpace(projectPath) == "" {
		return nil
	}
	key := "usage:project:" + filepath.Clean(projectPath)
	s.sessionMu.Lock()
	if cached, ok := s.usageCache[key]; ok && time.Since(cached.at) < 20*time.Second {
		items := append([]model.ResourceUsage(nil), cached.items...)
		s.sessionMu.Unlock()
		return items
	}
	s.sessionMu.Unlock()
	usage := s.usageFromSessions(s.Sessions(projectPath))
	s.sessionMu.Lock()
	s.usageCache[key] = usageCacheEntry{items: append([]model.ResourceUsage(nil), usage...), at: time.Now()}
	s.sessionMu.Unlock()
	return usage
}

func (s *AppService) usageFromSessions(items []model.Session) []model.ResourceUsage {
	if len(items) > 100 {
		items = items[:100]
	}
	type aggregate struct {
		kind   string
		name   string
		count  int
		lastAt time.Time
	}
	usage := map[string]*aggregate{}
	for _, session := range items {
		parsed, err := sessions.ReadToolUsage(session.Path)
		if err != nil {
			continue
		}
		for name, count := range parsed.Skills {
			key := "skill\x00" + name
			if usage[key] == nil {
				usage[key] = &aggregate{kind: "skill", name: name}
			}
			usage[key].count += count
			if parsed.LastAt.After(usage[key].lastAt) {
				usage[key].lastAt = parsed.LastAt
			}
		}
		for name, count := range parsed.MCP {
			key := "mcp\x00" + name
			if usage[key] == nil {
				usage[key] = &aggregate{kind: "mcp", name: name}
			}
			usage[key].count += count
			if parsed.LastAt.After(usage[key].lastAt) {
				usage[key].lastAt = parsed.LastAt
			}
		}
	}
	out := make([]model.ResourceUsage, 0, len(usage))
	for _, item := range usage {
		out = append(out, model.ResourceUsage{Kind: item.kind, Name: item.name, Count: item.count, LastAt: item.lastAt})
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Count != out[j].Count {
			return out[i].Count > out[j].Count
		}
		return out[i].LastAt.After(out[j].LastAt)
	})
	if len(out) > 20 {
		out = out[:20]
	}
	return out
}

func (s *AppService) RecentSessions(limit int) []model.Session {
	if limit <= 0 || limit > 200 {
		limit = 100
	}
	key := "recent"
	s.sessionMu.Lock()
	if cached, ok := s.sessionCache[key]; ok && time.Since(cached.at) < 20*time.Second {
		items := append([]model.Session(nil), cached.items...)
		s.sessionMu.Unlock()
		if len(items) > limit {
			items = items[:limit]
		}
		return items
	}
	s.sessionMu.Unlock()
	agents := []string{"Claude Code", "Codex", "DSH", "OpenCode", "Cursor", "Cline", "Goose", "OMP", "WorkBuddy", "Qoder", "CodeBuddy", "Gemini CLI"}
	var found []model.Session
	var mu sync.Mutex
	var wg sync.WaitGroup
	for _, agent := range agents {
		wg.Add(1)
		go func(agent string) {
			defer wg.Done()
			items := sessions.ScanAgent(s.context(), s.homeDir, agent)
			mu.Lock()
			found = append(found, items...)
			mu.Unlock()
		}(agent)
	}
	wg.Wait()
	sort.SliceStable(found, func(i, j int) bool { return found[i].LastAt.After(found[j].LastAt) })
	seen := map[string]bool{}
	unique := found[:0]
	for _, item := range found {
		key := item.Agent + "\x00" + item.ID
		if seen[key] {
			continue
		}
		seen[key] = true
		unique = append(unique, item)
	}
	s.sessionMu.Lock()
	s.sessionCache[key] = sessionCacheEntry{items: append([]model.Session(nil), unique...), at: time.Now()}
	s.sessionMu.Unlock()
	if len(unique) > limit {
		unique = unique[:limit]
	}
	return unique
}

func (s *AppService) RefreshSessions() {
	s.sessionMu.Lock()
	s.sessionCache = map[string]sessionCacheEntry{}
	s.sessionMu.Unlock()
}

func (s *AppService) AgentSessions(agent string) []model.Session {
	if strings.TrimSpace(agent) == "" {
		return nil
	}
	key := "agent:" + agent
	s.sessionMu.Lock()
	if cached, ok := s.sessionCache[key]; ok && time.Since(cached.at) < 20*time.Second {
		items := append([]model.Session(nil), cached.items...)
		s.sessionMu.Unlock()
		return items
	}
	s.sessionMu.Unlock()
	items := sessions.ScanAgent(s.context(), s.homeDir, agent)
	s.sessionMu.Lock()
	s.sessionCache[key] = sessionCacheEntry{items: items, at: time.Now()}
	s.sessionMu.Unlock()
	return items
}

func (s *AppService) Sessions(projectPath string) []model.Session {
	if strings.TrimSpace(projectPath) == "" {
		return nil
	}
	s.sessionMu.Lock()
	if cached, ok := s.sessionCache[projectPath]; ok && time.Since(cached.at) < 20*time.Second {
		items := append([]model.Session(nil), cached.items...)
		s.sessionMu.Unlock()
		return items
	}
	s.sessionMu.Unlock()
	items := sessions.Scan(s.context(), s.homeDir, projectPath)
	s.sessionMu.Lock()
	s.sessionCache[projectPath] = sessionCacheEntry{items: items, at: time.Now()}
	s.sessionMu.Unlock()
	return items
}

func (s *AppService) Commands() []model.Command {
	settings, _ := s.core.Settings(s.context())
	home, _ := platform.LocalPaths{}.HomeDir()
	return commands.Scan(s.context(), home, settings.ProjectWorkspaces)
}

func (s *AppService) CommandTargets() []model.CommandTarget {
	settings, _ := s.core.Settings(s.context())
	home, _ := platform.LocalPaths{}.HomeDir()
	return commands.Targets(home, settings.ProjectWorkspaces)
}

func (s *AppService) PlanCommandWrite(req model.CommandWriteRequest) (model.CommandWritePlan, error) {
	if len(req.Content) > 2<<20 {
		return model.CommandWritePlan{}, errors.New("command content is too large")
	}
	targets := s.CommandTargets()
	scanned := s.Commands()
	var target model.CommandTarget
	var source *model.Command
	var name, content, operation, format string

	if req.SourcePath != "" {
		for index := range scanned {
			if scanned[index].Path == req.SourcePath {
				source = &scanned[index]
				break
			}
		}
		if source == nil {
			return model.CommandWritePlan{}, errors.New("source command not found")
		}
		var err error
		target, err = findCommandTarget(targets, req)
		if err != nil {
			return model.CommandWritePlan{}, err
		}
		raw, err := os.ReadFile(source.Path)
		if err != nil {
			return model.CommandWritePlan{}, err
		}
		name = strings.TrimSpace(req.Name)
		if name == "" {
			name = source.Name
		}
		format = target.Format
		sourceFormat := source.Format
		if sourceFormat == "" {
			sourceFormat = "markdown"
		}
		var warnings []string
		content, warnings = commands.Convert(string(raw), sourceFormat, format, target.Agent)
		operation = "command_convert"
		if strings.TrimSpace(req.Content) != "" {
			content = req.Content
		}
		plan := s.buildCommandPlan(target, name, content, operation)
		plan.Warnings = append(plan.Warnings, warnings...)
		return s.storeCommandPlan(plan)
	}

	if req.Path != "" {
		for index := range scanned {
			if scanned[index].Path == req.Path {
				source = &scanned[index]
				break
			}
		}
		if source == nil {
			return model.CommandWritePlan{}, errors.New("command not found")
		}
		target = model.CommandTarget{Agent: source.Agent, Scope: source.Scope, Project: source.Project, RootPath: source.RootPath, Format: source.Format}
		name = source.Name
		if strings.TrimSpace(req.Name) != "" {
			name = strings.TrimSpace(req.Name)
		}
		content = req.Content
		if content == "" {
			raw, err := os.ReadFile(source.Path)
			if err != nil {
				return model.CommandWritePlan{}, err
			}
			content = string(raw)
		}
		operation = "command_update"
	} else {
		var err error
		target, err = findCommandTarget(targets, req)
		if err != nil {
			return model.CommandWritePlan{}, err
		}
		name = strings.TrimSpace(req.Name)
		content = req.Content
		operation = "command_create"
	}
	plan := s.buildCommandPlan(target, name, content, operation)
	return s.storeCommandPlan(plan)
}

func findCommandTarget(targets []model.CommandTarget, req model.CommandWriteRequest) (model.CommandTarget, error) {
	for _, target := range targets {
		if req.Agent != "" && target.Agent != req.Agent {
			continue
		}
		if req.Scope != "" && target.Scope != req.Scope {
			continue
		}
		if req.Project != "" && target.Project != req.Project {
			continue
		}
		if req.RootPath != "" && target.RootPath != req.RootPath {
			continue
		}
		return target, nil
	}
	return model.CommandTarget{}, errors.New("command target not found")
}

func (s *AppService) buildCommandPlan(target model.CommandTarget, name, content, operation string) model.CommandWritePlan {
	name = cleanCommandName(name)
	format := target.Format
	if format == "" {
		format = "markdown"
	}
	ext := ".md"
	if format == "toml" {
		ext = ".toml"
	}
	path := filepath.Join(target.RootPath, filepath.FromSlash(name)+ext)
	current := ""
	exists := false
	if raw, err := os.ReadFile(path); err == nil {
		current = string(raw)
		exists = true
	}
	plan := model.CommandWritePlan{ID: fmt.Sprintf("command-plan-%d", time.Now().UnixNano()), Operation: operation, Target: target, Name: name, Path: path, Content: content, CurrentContent: current, Exists: exists, CreatedAt: time.Now()}
	if exists {
		plan.Warnings = append(plan.Warnings, "target already exists; applying will overwrite it")
	}
	if name == "" {
		plan.Warnings = append(plan.Warnings, "command name is empty")
	}
	return plan
}

func cleanCommandName(name string) string {
	name = strings.TrimSpace(strings.ReplaceAll(name, "\\", "/"))
	name = strings.TrimPrefix(name, "/")
	for strings.Contains(name, "//") {
		name = strings.ReplaceAll(name, "//", "/")
	}
	parts := strings.Split(name, "/")
	clean := make([]string, 0, len(parts))
	for _, part := range parts {
		if part == "" || part == "." || part == ".." {
			continue
		}
		clean = append(clean, part)
	}
	return strings.Join(clean, "/")
}

func (s *AppService) storeCommandPlan(plan model.CommandWritePlan) (model.CommandWritePlan, error) {
	if plan.Name == "" {
		return model.CommandWritePlan{}, errors.New("command name is required")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.commandPlans == nil {
		s.commandPlans = map[string]model.CommandWritePlan{}
	}
	s.commandPlans[plan.ID] = plan
	return plan, nil
}

func (s *AppService) PlanCommandDelete(path string) (model.CommandDeletePlan, error) {
	for _, command := range s.Commands() {
		if command.Path != path {
			continue
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			return model.CommandDeletePlan{}, err
		}
		target := model.CommandTarget{Agent: command.Agent, Scope: command.Scope, Project: command.Project, RootPath: command.RootPath, Format: command.Format}
		plan := model.CommandDeletePlan{ID: fmt.Sprintf("command-delete-%d", time.Now().UnixNano()), Target: target, Name: command.Name, Path: command.Path, CurrentContent: string(raw), CreatedAt: time.Now()}
		s.mu.Lock()
		if s.commandDeletePlans == nil {
			s.commandDeletePlans = map[string]model.CommandDeletePlan{}
		}
		s.commandDeletePlans[plan.ID] = plan
		s.mu.Unlock()
		return plan, nil
	}
	return model.CommandDeletePlan{}, errors.New("command not found")
}

func (s *AppService) ApplyCommandDelete(planID string) (model.Operation, error) {
	s.mu.Lock()
	plan, ok := s.commandDeletePlans[planID]
	s.mu.Unlock()
	if !ok {
		return model.Operation{}, errors.New("command delete plan not found or expired")
	}
	if plan.Target.RootPath == "" || !strings.HasPrefix(filepath.Clean(plan.Path), filepath.Clean(plan.Target.RootPath)+string(filepath.Separator)) {
		return model.Operation{}, errors.New("command path is outside target root")
	}
	snapshotDir := filepath.Join(s.dataDir, "snapshots", "command-delete-"+plan.ID)
	if err := os.MkdirAll(snapshotDir, 0o700); err != nil {
		return model.Operation{}, err
	}
	manifest := ops.SnapshotManifest{ID: filepath.Base(snapshotDir), PlanID: plan.ID, CreatedAt: time.Now()}
	payload, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return model.Operation{}, err
	}
	if err := os.WriteFile(filepath.Join(snapshotDir, "manifest.json"), payload, 0o600); err != nil {
		return model.Operation{}, err
	}
	if err := ops.SnapshotFile(snapshotDir, plan.Path); err != nil {
		return model.Operation{}, err
	}
	if err := os.Remove(plan.Path); err != nil {
		return model.Operation{}, err
	}
	now := time.Now()
	op := model.Operation{ID: fmt.Sprintf("op-command-delete-%d", now.UnixNano()), PlanID: plan.ID, Operation: "command_delete", Status: "success", Message: "Command deleted", Changes: []string{"command:" + plan.Path}, SnapshotPath: snapshotDir, CreatedAt: now}
	if store := s.core.Store(); store != nil {
		if err := store.SaveOperation(s.context(), op); err != nil {
			return op, err
		}
	}
	s.mu.Lock()
	delete(s.commandDeletePlans, planID)
	s.mu.Unlock()
	return op, nil
}

func (s *AppService) CancelCommandDelete(planID string) {
	s.mu.Lock()
	delete(s.commandDeletePlans, planID)
	s.mu.Unlock()
}

func (s *AppService) ApplyCommandWrite(planID string) (model.Operation, error) {
	s.mu.Lock()
	plan, ok := s.commandPlans[planID]
	s.mu.Unlock()
	if !ok {
		return model.Operation{}, errors.New("command plan not found or expired")
	}
	if plan.Target.RootPath == "" || !strings.HasPrefix(filepath.Clean(plan.Path), filepath.Clean(plan.Target.RootPath)+string(filepath.Separator)) {
		return model.Operation{}, errors.New("command path is outside target root")
	}
	snapshotDir := filepath.Join(s.dataDir, "snapshots", "command-"+plan.ID)
	if err := os.MkdirAll(snapshotDir, 0o700); err != nil {
		return model.Operation{}, err
	}
	manifest := ops.SnapshotManifest{ID: filepath.Base(snapshotDir), PlanID: plan.ID, CreatedAt: time.Now()}
	payload, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return model.Operation{}, err
	}
	if err := os.WriteFile(filepath.Join(snapshotDir, "manifest.json"), payload, 0o600); err != nil {
		return model.Operation{}, err
	}
	if err := ops.SnapshotFile(snapshotDir, plan.Path); err != nil {
		return model.Operation{}, err
	}
	if err := writeFileAtomically(plan.Path, plan.Content); err != nil {
		return model.Operation{}, err
	}
	now := time.Now()
	op := model.Operation{ID: fmt.Sprintf("op-command-%d", now.UnixNano()), PlanID: plan.ID, Operation: plan.Operation, Status: "success", Message: "Command written", Changes: []string{"command:" + plan.Path}, SnapshotPath: snapshotDir, CreatedAt: now}
	if store := s.core.Store(); store != nil {
		if err := store.SaveOperation(s.context(), op); err != nil {
			return op, err
		}
	}
	s.mu.Lock()
	delete(s.commandPlans, planID)
	s.mu.Unlock()
	return op, nil
}

func (s *AppService) CancelCommandWrite(planID string) {
	s.mu.Lock()
	delete(s.commandPlans, planID)
	s.mu.Unlock()
}

func writeFileAtomically(path, content string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".command-*.tmp")
	if err != nil {
		return err
	}
	name := tmp.Name()
	defer os.Remove(name)
	if err := tmp.Chmod(0o644); err != nil {
		_ = tmp.Close()
		return err
	}
	if _, err := tmp.WriteString(content); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(name, path)
}

func (s *AppService) CommandContent(path string) (string, error) {
	for _, command := range s.Commands() {
		if command.Path != path {
			continue
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			return "", err
		}
		if len(raw) > 2<<20 {
			return "", errors.New("command file is too large")
		}
		return string(raw), nil
	}
	return "", errors.New("command not found")
}

func (s *AppService) Xray(agentID string, project string, compareAgentIDs []string) (model.XrayReport, error) {
	settings, _ := s.core.Settings(s.context())
	return xray.Build(xray.Input{
		Scan:            s.core.Snapshot(),
		Instructions:    s.Instructions(),
		MCPServers:      s.MCP(),
		Settings:        settings,
		AgentID:         agentID,
		Project:         project,
		CompareAgentIDs: compareAgentIDs,
	})
}

// enrichSkillSources fills unresolved local skills from the bundled official
// catalog snapshot. It never performs a network request.
func (s *AppService) enrichSkillSources(result *model.ScanResult) {
	if result == nil || len(s.catalogByName) == 0 {
		return
	}
	for index := range result.Skills {
		skill := &result.Skills[index]
		if skill.Source != "" {
			continue
		}
		item, ok := s.catalogByName[strings.ToLower(skill.Name)]
		if !ok || item.Source == "" {
			continue
		}
		skill.Source = item.Source
		skill.SourceType = "catalog"
		skill.Locked = true
		skill.Origin = "catalog"
		if strings.Contains(item.Source, "/") {
			skill.SourceURL = "https://github.com/" + item.Source
		}
	}
}

func (s *AppService) Scan() (AppState, error) {
	s.mu.Lock()
	if s.state.Scanning {
		state := s.state
		s.mu.Unlock()
		return state, messageError("common.scanAlreadyRunning", "scan already running", nil)
	}
	parent := s.ctx
	if parent == nil {
		parent = context.Background()
	}
	ctx, cancel := context.WithCancel(parent)
	s.scanCancel = cancel
	s.state.Scanning = true
	s.state.Cached = false
	s.mu.Unlock()
	s.emit()
	result, err := s.core.Scan(ctx)
	s.enrichSkillSources(&result)
	cancel()
	s.mu.Lock()
	s.scanCancel = nil
	s.state.Scanning = false
	s.state.Snapshot = result
	s.state.Updates = s.core.Updates()
	if err != nil && !errors.Is(err, context.Canceled) {
		s.state.LastError = err.Error()
	} else {
		s.state.LastError = ""
	}
	state := s.state
	s.mu.Unlock()
	s.emit()
	return state, err
}

func (s *AppService) CancelScan() bool {
	s.mu.Lock()
	cancel := s.scanCancel
	s.mu.Unlock()
	if cancel == nil {
		return false
	}
	cancel()
	return true
}

func (s *AppService) Doctor() []model.Finding { return s.core.Doctor() }

// CheckUpdates is explicit: it runs only the fixed brew/npm read commands and
// returns a report; it never installs, upgrades or removes anything.
func (s *AppService) PlanLink(skillID string, rootIDs []string) (model.Plan, error) {
	return s.core.PlanLink(s.context(), skillID, rootIDs)
}

func (s *AppService) PlanDisableLink(skillID string, rootIDs []string) (model.Plan, error) {
	return s.core.PlanDisableLink(s.context(), skillID, rootIDs)
}

func (s *AppService) PlanInstallSkill(repo string, skillPath string, name string) (model.Plan, error) {
	return s.core.PlanInstallSkill(s.context(), repo, skillPath, name)
}

func (s *AppService) InstallUpstreamSkills(repo string, items []model.UpstreamSkillInstall) ([]model.Operation, error) {
	return s.core.InstallUpstreamSkills(s.context(), repo, items, func(done, total int, name string) {
		if s.app != nil {
			s.app.Event.Emit("larder:install-progress", map[string]any{"done": done, "total": total, "name": name})
		}
	})
}

func (s *AppService) PlanInstallCatalogSkill(repo, skillID, name string) (model.Plan, error) {
	return s.core.PlanInstallCatalogSkill(s.context(), repo, skillID, name)
}

func (s *AppService) PlanUninstallSkill(skillID string) (model.Plan, error) {
	return s.core.PlanUninstallSkill(s.context(), skillID)
}

func (s *AppService) PlanSkillUpdate(skillID string) (model.Plan, error) {
	return s.core.PlanSkillUpdate(s.context(), skillID)
}

func (s *AppService) PlanCLI(cliID string, uninstall bool, latest string) (model.Plan, error) {
	return s.core.PlanCLI(s.context(), cliID, uninstall, latest)
}

func (s *AppService) PlanSyncCopy(skillID string, rootIDs []string) (model.Plan, error) {
	return s.core.PlanSyncCopy(s.context(), skillID, rootIDs)
}

func (s *AppService) PlanCopyToLink(skillID string, rootIDs []string) (model.Plan, error) {
	return s.core.PlanCopyToLink(s.context(), skillID, rootIDs)
}

func (s *AppService) ApplyPlanWithResolutions(planID string, resolutions map[string]string) (model.Operation, error) {
	op, err := s.core.ApplyPlanWithResolutions(s.context(), planID, resolutions)
	if err == nil {
		settings, _ := s.core.Settings(s.context())
		retention := settings.SnapshotRetention
		if retention <= 0 {
			retention = 50
		}
		_ = s.core.PruneSnapshots(retention)
		s.syncFromCore()
	}
	return op, err
}

func (s *AppService) ApplyPlan(planID string) (model.Operation, error) {
	op, err := s.core.ApplyPlan(s.context(), planID)
	if err != nil {
		return op, messageError("common.executionFailed", err.Error(), nil)
	}
	if err == nil {
		settings, _ := s.core.Settings(s.context())
		retention := settings.SnapshotRetention
		if retention <= 0 {
			retention = 50
		}
		_ = s.core.PruneSnapshots(retention)
		s.syncFromCore()
	}
	return op, err
}

func (s *AppService) ApplyLinkPlan(planID string) (model.Operation, error) {
	op, err := s.core.ApplyLinkPlan(s.context(), planID)
	if err == nil {
		settings, _ := s.core.Settings(s.context())
		retention := settings.SnapshotRetention
		if retention <= 0 {
			retention = 50
		}
		_ = s.core.PruneSnapshots(retention)
		s.syncFromCore()
	}
	return op, err
}

// syncFromCore republishes the inventory snapshot after a plan has mutated the
// filesystem. The core apply paths rescan internally, but AppService.state
// would otherwise stay on the pre-operation snapshot until the next manual scan.
func (s *AppService) syncFromCore() {
	snapshot := s.core.Snapshot()
	s.updateState(func(state *AppState) {
		state.Snapshot = snapshot
		state.Updates = s.core.Updates()
		state.Cached = false
		state.Scanning = false
		state.LastError = ""
	})
}

func (s *AppService) ExportReport() (string, error) {
	return s.core.Report(s.context())
}

func (s *AppService) SkillIcon(skillID string) (string, error) {
	return s.core.SkillIcon(s.context(), skillID)
}

// ToggleWindowMaximised is used by the custom chrome: double-clicking the
// top bar toggles the native maximised state.
func (s *AppService) ToggleWindowMaximised() bool {
	s.mu.RLock()
	app := s.app
	s.mu.RUnlock()
	if app == nil {
		return false
	}
	window := app.Window.Current()
	if window == nil {
		return false
	}
	if window.IsMaximised() {
		window.UnMaximise()
	} else {
		window.Maximise()
	}
	return window.IsMaximised()
}

func (s *AppService) UsageStats() ([]model.UsageStat, error) {
	return s.core.UsageStats(s.context())
}

func (s *AppService) Timeline() ([]model.TimelineEvent, error) {
	return s.core.Timeline(s.context())
}

func (s *AppService) Operations() ([]model.Operation, error) {
	return s.core.Operations(s.context())
}

func (s *AppService) RollbackOperation(operationID string) (model.Operation, error) {
	op, err := s.core.RollbackOperation(s.context(), operationID)
	if err == nil {
		settings, _ := s.core.Settings(s.context())
		retention := settings.SnapshotRetention
		if retention <= 0 {
			retention = 50
		}
		_ = s.core.PruneSnapshots(retention)
		s.syncFromCore()
	}
	return op, err
}

func (s *AppService) PurgeTrash(operationID string) error {
	return s.core.PurgeTrash(s.context(), operationID)
}

func (s *AppService) PreviewSkillFileDiff(skillID, path string) (model.SkillUpdateFile, error) {
	return s.core.PreviewSkillFileDiff(s.context(), skillID, path)
}

func (s *AppService) PreviewVersionDiff(operationID string) (model.SkillUpdateDiff, error) {
	return s.core.PreviewVersionDiff(s.context(), operationID)
}

func (s *AppService) PreviewSkillUpdate(skillID string) (model.SkillUpdateDiff, error) {
	return s.core.PreviewSkillUpdate(s.context(), skillID)
}

func (s *AppService) CheckUpdates() (model.UpdateReport, error) {
	report, err := s.core.CheckUpdates(s.context())
	if err != nil {
		return model.UpdateReport{}, err
	}
	s.mu.Lock()
	s.state.Updates = report
	s.mu.Unlock()
	s.emit()
	return report, nil
}

func (s *AppService) Settings() (model.Settings, error) {
	ctx := s.context()
	return s.core.Settings(ctx)
}

func (s *AppService) IgnoreAgent(path, label, rootID string) (model.Settings, error) {
	return s.core.IgnoreAgent(s.context(), path, label, rootID)
}

func (s *AppService) RestoreAgent(path string) (model.Settings, error) {
	return s.core.RestoreAgent(s.context(), path)
}

func (s *AppService) TrashAgent(path string) (model.Settings, error) {
	return s.core.TrashAgent(s.context(), path)
}

func (s *AppService) SessionMessages(path string, limit int) ([]model.SessionMessage, error) {
	clean := filepath.Clean(strings.TrimSpace(path))
	if clean == "" || clean == "." || !isSessionSourcePath(s.homeDir, clean) {
		return nil, errors.New("session path is outside the supported session roots")
	}
	return sessions.ReadMessages(clean, limit)
}

func (s *AppService) revealPath(path string) error {
	if s.app == nil {
		return errors.New("app is not ready")
	}
	if _, err := os.Stat(path); err != nil {
		return err
	}
	return s.app.Env.OpenFileManager(path, true)
}

func (s *AppService) RevealSession(path string) error {
	clean := filepath.Clean(strings.TrimSpace(path))
	if clean == "" || clean == "." || !isSessionSourcePath(s.homeDir, clean) {
		return errors.New("session path is outside the supported session roots")
	}
	return s.revealPath(clean)
}

func isSessionSourcePath(home, path string) bool {
	if home == "" {
		return false
	}
	roots := []string{
		filepath.Join(home, ".claude", "projects"),
		filepath.Join(home, ".codex", "sessions"),
		filepath.Join(home, ".codex", "archived_sessions"),
		filepath.Join(home, ".local", "share", "opencode", "storage"),
		filepath.Join(home, ".cursor", "projects"),
		filepath.Join(home, ".cline", "data", "sessions"),
		filepath.Join(home, ".local", "share", "goose", "sessions"),
		filepath.Join(home, ".omp", "agent", "sessions"),
		filepath.Join(home, ".workbuddy", "projects"),
		filepath.Join(home, ".qoder", "projects"),
		filepath.Join(home, ".qoder-cn", "projects"),
		filepath.Join(home, ".codebuddy", "projects"),
		filepath.Join(home, ".gemini", "tmp"),
		filepath.Join(home, ".dsh", "sessions"),
	}
	for _, root := range roots {
		rel, err := filepath.Rel(root, path)
		if err != nil || rel == "." || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			continue
		}
		return true
	}
	return false
}

func (s *AppService) RevealSkill(skillID string) error {
	for _, skill := range s.core.Snapshot().Skills {
		if skill.ID == skillID {
			return s.revealPath(skill.Path)
		}
	}
	return errors.New("skill not found")
}

func (s *AppService) RevealCLI(cliID string) error {
	for _, cli := range s.core.Snapshot().CLIs {
		if cli.ID == cliID {
			return s.revealPath(cli.Path)
		}
	}
	return errors.New("CLI not found")
}

func (s *AppService) MCP() []model.MCPServer {
	settings, _ := s.core.Settings(s.context())
	servers := coremcp.ScanProjects(s.context(), settings.ProjectWorkspaces)
	s.mu.RLock()
	disabled := append([]ops.DetachedMCP(nil), s.disabled...)
	s.mu.RUnlock()
	for _, entry := range disabled {
		found := false
		for index := range servers {
			if servers[index].Name != entry.Name {
				continue
			}
			if entry.Command != "" && servers[index].Command != entry.Command {
				continue
			}
			if entry.URL != "" && servers[index].URL != entry.URL {
				continue
			}
			servers[index].Agents = append(servers[index].Agents, model.MCPAgent{Label: entry.AgentLabel, Path: entry.AgentPath, Enabled: false})
			found = true
			break
		}
		if found {
			continue
		}
		server := model.MCPServer{
			ID:        "disabled:" + entry.AgentPath + ":" + entry.Name,
			Name:      entry.Name,
			Transport: entry.Transport,
			Command:   entry.Command,
			Args:      entry.Args,
			URL:       entry.URL,
			Status:    "disabled",
		}
		server.Agents = append(server.Agents, model.MCPAgent{Label: entry.AgentLabel, Path: entry.AgentPath, Enabled: false})
		servers = append(servers, server)
	}
	return servers
}

func (s *AppService) DisableMCPAgent(serverID, agentPath string) error {
	servers := s.MCP()
	var target *model.MCPServer
	for index := range servers {
		if servers[index].ID == serverID {
			copy := servers[index]
			target = &copy
			break
		}
	}
	if target == nil {
		return messageError("mcp.serverNotFound", "mcp server not found", nil)
	}
	var label string
	for _, agent := range target.Agents {
		if filepath.Clean(agent.Path) == filepath.Clean(agentPath) && agent.Enabled {
			label = agent.Label
		}
	}
	if label == "" {
		return errors.New("enabled mcp agent not found")
	}
	detached, _, err := s.core.DetachMCPAgent(s.context(), agentPath, target.Name, label)
	if err != nil {
		return err
	}
	s.mu.Lock()
	s.disabled = append(s.disabled, detached)
	s.mu.Unlock()
	if err := s.saveDisabledMCPs(); err != nil {
		return err
	}
	s.syncFromCore()
	return nil
}

func (s *AppService) EnableMCPAgent(name, agentPath string) error {
	s.mu.RLock()
	var entry ops.DetachedMCP
	found := false
	for _, item := range s.disabled {
		if item.Name == name && filepath.Clean(item.AgentPath) == filepath.Clean(agentPath) {
			entry, found = item, true
			break
		}
	}
	s.mu.RUnlock()
	if !found {
		return errors.New("disabled mcp entry not found")
	}
	if _, err := s.core.AttachMCPAgent(s.context(), entry); err != nil {
		return err
	}
	s.mu.Lock()
	for index, item := range s.disabled {
		if item.Name == name && filepath.Clean(item.AgentPath) == filepath.Clean(agentPath) {
			s.disabled = append(s.disabled[:index], s.disabled[index+1:]...)
			break
		}
	}
	s.mu.Unlock()
	if err := s.saveDisabledMCPs(); err != nil {
		return err
	}
	s.syncFromCore()
	return nil
}

func (s *AppService) loadDisabledMCPs() {
	body, err := os.ReadFile(s.disabledPath)
	if err != nil {
		return
	}
	_ = json.Unmarshal(body, &s.disabled)
}

func (s *AppService) saveDisabledMCPs() error {
	s.mu.RLock()
	body, err := json.Marshal(s.disabled)
	s.mu.RUnlock()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(s.disabledPath), 0o700); err != nil {
		return err
	}
	return os.WriteFile(s.disabledPath, body, 0o600)
}

func (s *AppService) ExportBackup() (string, error) {
	settings, _ := s.core.Settings(s.context())
	return backup.Create(s.dataDir, settings.ProjectWorkspaces, s.Instructions())
}

func (s *AppService) PickBackupFile() (string, error) {
	s.mu.RLock()
	app := s.app
	s.mu.RUnlock()
	if app == nil {
		return "", messageError("common.appNotReady", "app is not ready", nil)
	}
	dialog := app.Dialog.OpenFile().SetTitle("选择备份文件")
	dialog.CanChooseFiles(true)
	dialog.CanChooseDirectories(false)
	dialog.AddFilter("Larder 备份", "*.zip")
	if window := app.Window.Current(); window != nil {
		dialog.AttachToWindow(window)
	}
	return dialog.PromptForSingleSelection()
}

func (s *AppService) InspectBackup(path string) (backup.Info, error) {
	return backup.Inspect(path)
}

func (s *AppService) RestoreBackup(path string) (backup.Result, error) {
	settings, _ := s.core.Settings(s.context())
	result, err := backup.Restore(path, s.dataDir, s.homeDir, settings.ProjectWorkspaces)
	if err != nil {
		return backup.Result{}, err
	}
	s.loadDisabledMCPs()
	s.loadCatalogInstalls()
	s.syncFromCore()
	return result, nil
}

func (s *AppService) AppInfo() AppInfo {
	return AppInfo{Version: Version, SchemaVersion: model.SchemaVersion, GoVersion: runtime.Version(), Platform: runtime.GOOS + "/" + runtime.GOARCH, DataDir: s.dataDir, HomeDir: s.homeDir}
}

func (s *AppService) SaveInstruction(path, content string) error {
	settings, _ := s.core.Settings(s.context())
	roots := append([]string{s.homeDir}, settings.ProjectWorkspaces...)
	if err := validateInstructionPath(path, roots); err != nil {
		return err
	}
	if len(content) > 512*1024 {
		return messageError("instructions.fileTooLarge", "instruction file is too large", nil)
	}
	if existing, err := os.ReadFile(path); err == nil {
		backupDir := filepath.Join(s.dataDir, "instruction-backups")
		if err := os.MkdirAll(backupDir, 0o700); err != nil {
			return err
		}
		name := fmt.Sprintf("%s-%s", time.Now().Format("20060102-150405"), filepath.Base(path))
		if err := os.WriteFile(filepath.Join(backupDir, name), existing, 0o600); err != nil {
			return err
		}
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(content), 0o644)
}

func (s *AppService) CreateInstruction(projectPath, name, content string) (string, error) {
	settings, _ := s.core.Settings(s.context())
	allowedProject := false
	for _, project := range settings.ProjectWorkspaces {
		if filepath.Clean(project) == filepath.Clean(projectPath) {
			allowedProject = true
			break
		}
	}
	if !allowedProject {
		return "", messageError("instructions.projectNotConfigured", "project is not in settings", nil)
	}
	names := map[string]string{
		"AGENTS.md":                       "AGENTS.md",
		"CLAUDE.md":                       "CLAUDE.md",
		"GEMINI.md":                       "GEMINI.md",
		".cursorrules":                    ".cursorrules",
		".claude/CLAUDE.md":               ".claude/CLAUDE.md",
		".cursor/rules/project.mdc":       ".cursor/rules/project.mdc",
		".github/copilot-instructions.md": ".github/copilot-instructions.md",
	}
	relative, ok := names[name]
	if !ok {
		return "", messageError("instructions.unsupportedFileName", "unsupported instruction file name", nil)
	}
	path := filepath.Join(projectPath, filepath.FromSlash(relative))
	if _, err := os.Stat(path); err == nil {
		return "", messageError("instructions.fileAlreadyExists", "instruction file already exists", nil)
	}
	return path, s.SaveInstruction(path, content)
}

func validateInstructionPath(path string, roots []string) error {
	if !filepath.IsAbs(path) {
		return messageError("instructions.pathMustBeAbsolute", "instruction path must be absolute", nil)
	}
	clean := filepath.Clean(path)
	allowed := false
	for _, root := range roots {
		root = filepath.Clean(root)
		if root != "" && (clean == root || strings.HasPrefix(clean, root+string(filepath.Separator))) {
			allowed = true
			break
		}
	}
	if !allowed {
		return messageError("instructions.pathOutsideWorkspaces", "instruction path is outside configured workspaces", nil)
	}
	switch strings.ToLower(filepath.Ext(clean)) {
	case ".md", ".mdc", ".markdown", ".txt", ".cursorrules":
		return nil
	default:
		return messageError("instructions.unsupportedFileType", "unsupported instruction file type", nil)
	}
}

func (s *AppService) InstructionContent(path string) (string, error) {
	ext := strings.ToLower(filepath.Ext(path))
	if ext != ".md" && ext != ".mdc" && ext != ".markdown" && ext != ".txt" && ext != ".cursorrules" {
		return "", messageError("instructions.unsupportedFileType", "unsupported instruction file type", nil)
	}
	info, err := os.Stat(path)
	if err != nil {
		return "", err
	}
	if info.Size() > 512*1024 {
		return "", messageError("instructions.fileTooLargeToPreview", "instruction file is too large to preview", nil)
	}
	raw, err := os.ReadFile(path)
	return string(raw), err
}

func (s *AppService) Instructions() []model.InstructionFile {
	settings, _ := s.core.Settings(s.context())
	home, _ := platform.LocalPaths{}.HomeDir()
	return instructions.Scan(s.context(), home, settings.ProjectWorkspaces)
}

func (s *AppService) TestMCP(serverID string) (MCPTestResult, error) {
	var server *model.MCPServer
	for _, item := range s.MCP() {
		if item.ID == serverID {
			copy := item
			server = &copy
			break
		}
	}
	if server == nil {
		return MCPTestResult{}, messageError("mcp.serverNotFound", "mcp server not found", nil)
	}
	if server.URL != "" {
		client := &http.Client{Timeout: 6 * time.Second}
		request, _ := http.NewRequest(http.MethodHead, server.URL, nil)
		response, err := client.Do(request)
		if err == nil {
			response.Body.Close()
			if response.StatusCode == http.StatusMethodNotAllowed || response.StatusCode == http.StatusNotImplemented {
				request, _ = http.NewRequest(http.MethodGet, server.URL, nil)
				response, err = client.Do(request)
				if err == nil {
					response.Body.Close()
				}
			}
		}
		if err != nil {
			return MCPTestResult{OK: false, Message: "无法连接：" + err.Error()}, nil
		}
		if response.StatusCode >= 500 {
			return MCPTestResult{OK: false, Message: fmt.Sprintf("服务返回 %d", response.StatusCode)}, nil
		}
		return MCPTestResult{OK: true, Message: fmt.Sprintf("已连通（HTTP %d）", response.StatusCode)}, nil
	}
	if server.Command == "" {
		return MCPTestResult{OK: false, Message: "配置缺少启动命令"}, nil
	}
	command := server.Command
	if !filepath.IsAbs(command) {
		found, err := exec.LookPath(command)
		if err != nil {
			return MCPTestResult{OK: false, Message: "命令不存在：" + command}, nil
		}
		command = found
	}
	info, err := os.Stat(command)
	if err != nil || info.IsDir() || info.Mode()&0o111 == 0 {
		return MCPTestResult{OK: false, Message: "命令不可执行：" + server.Command}, nil
	}
	return MCPTestResult{OK: true, Message: "命令可执行"}, nil
}

func (s *AppService) MCPTargets() []model.MCPAgent {
	return s.core.MCPTargets()
}

func (s *AppService) SearchMCPRegistry(query string) ([]coremcp.RegistryServer, error) {
	return coremcp.SearchRegistry(s.context(), query)
}

func (s *AppService) PlanMCPAdd(name, transport, command string, args []string, url string, env map[string]string, agentPaths []string) (model.Plan, error) {
	return s.core.PlanMCPAdd(s.context(), name, transport, command, args, url, env, agentPaths)
}

func (s *AppService) PlanMCPUpdate(serverID, name, command string, args []string, url string, agentPaths []string) (model.Plan, error) {
	return s.core.PlanMCPUpdate(s.context(), serverID, name, command, args, url, agentPaths)
}

func (s *AppService) PlanMCPRemove(serverID string, agentPaths []string) (model.Plan, error) {
	return s.core.PlanMCPRemove(s.context(), serverID, agentPaths)
}

// PickFolder opens the native directory picker for project workspaces.
func (s *AppService) CatalogSkills(view string) ([]catalog.Skill, error) {
	if view == "popular" {
		return s.catalog.Popular(s.context())
	}
	return s.catalog.Top(s.context(), view)
}

func (s *AppService) SearchCatalogSkills(query string) ([]catalog.Skill, error) {
	return s.catalog.Search(s.context(), query)
}

func (s *AppService) ResolveCatalogSkill(source, skillID string) (string, error) {
	return s.catalog.Resolve(s.context(), source, skillID)
}

func (s *AppService) CatalogDescription(source, skillID string) (string, error) {
	return s.catalog.Description(s.context(), source, skillID)
}

func (s *AppService) RecordCatalogInstall(path, source, skillID string) error {
	if path == "" || source == "" || skillID == "" {
		return messageError("skills.installFieldsRequired", "path, source and skill are required", nil)
	}
	s.mu.Lock()
	s.installs[path] = CatalogInstall{Path: path, Source: source, SkillID: skillID, InstalledAt: time.Now()}
	s.mu.Unlock()
	return s.saveCatalogInstalls()
}

func (s *AppService) CatalogInstalls() []CatalogInstall {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]CatalogInstall, 0, len(s.installs))
	for _, install := range s.installs {
		if _, err := os.Stat(install.Path); err == nil {
			out = append(out, install)
		}
	}
	return out
}

func (s *AppService) loadCatalogInstalls() {
	body, err := os.ReadFile(s.installsPath)
	if err != nil {
		return
	}
	var installs map[string]CatalogInstall
	if json.Unmarshal(body, &installs) != nil {
		return
	}
	for path, install := range installs {
		install.Path = path
		s.installs[path] = install
	}
}

func (s *AppService) saveCatalogInstalls() error {
	s.mu.RLock()
	body, err := json.Marshal(s.installs)
	s.mu.RUnlock()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(s.installsPath), 0o700); err != nil {
		return err
	}
	return os.WriteFile(s.installsPath, body, 0o600)
}

func (s *AppService) PickFolder() (string, error) {
	s.mu.RLock()
	app := s.app
	s.mu.RUnlock()
	if app == nil {
		return "", messageError("common.appNotReady", "app is not ready", nil)
	}
	dialog := app.Dialog.OpenFile().SetTitle("选择项目文件夹")
	dialog.CanChooseFiles(false)
	dialog.CanChooseDirectories(true)
	dialog.CanCreateDirectories(false)
	if window := app.Window.Current(); window != nil {
		dialog.AttachToWindow(window)
	}
	return dialog.PromptForSingleSelection()
}

func (s *AppService) SaveSettings(settings model.Settings) (model.Settings, error) {
	ctx := s.context()
	previous, _ := s.core.Settings(ctx)
	saved, err := s.core.SaveSettings(ctx, settings)
	if err != nil {
		return saved, err
	}
	if shortcutErr := s.applyGlobalShortcut(saved); shortcutErr != nil {
		_, _ = s.core.SaveSettings(ctx, previous)
		_ = s.applyGlobalShortcut(previous)
		return previous, fmt.Errorf("apply global shortcut: %w", shortcutErr)
	}
	_ = s.core.PruneTrash(ctx, saved.TrashRetentionDays)
	return saved, nil
}

func (s *AppService) Shutdown() {
	s.mu.Lock()
	if s.cancel != nil {
		s.cancel()
	}
	if s.scanCancel != nil {
		s.scanCancel()
	}
	s.mu.Unlock()
	if s.core.Store() != nil {
		_ = s.core.Store().Close()
	}
}

func (s *AppService) context() context.Context {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.ctx != nil {
		return s.ctx
	}
	return context.Background()
}

func (s *AppService) updateState(fn func(*AppState)) {
	s.mu.Lock()
	fn(&s.state)
	s.mu.Unlock()
	s.emit()
}

func (s *AppService) emit() {
	s.mu.Lock()
	s.state.UpdatedAt = time.Now()
	snapshot := s.state
	app := s.app
	s.mu.Unlock()
	if app != nil {
		app.Event.Emit("larder:state", snapshot)
	}
}
