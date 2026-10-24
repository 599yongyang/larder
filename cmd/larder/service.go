package larder

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"
	"larder/internal/core/backup"
	"larder/internal/core/catalog"
	"larder/internal/core/instructions"
	"larder/internal/core/inventory"
	coremcp "larder/internal/core/mcp"
	"larder/internal/core/model"
	"larder/internal/core/ops"
	"larder/internal/core/store"
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

var Version = "0.0.1"

type MCPTestResult struct {
	OK      bool   `json:"ok"`
	Message string `json:"message"`
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
	core    *inventory.Service
	catalog *catalog.Client
	app     *application.App

	installsPath string
	installs     map[string]CatalogInstall
	disabledPath string
	disabled     []ops.DetachedMCP
	dataDir      string
	homeDir      string

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
	service := &AppService{core: core, catalog: catalog.NewClient(), installsPath: filepath.Join(dataDir, "catalog-installs.json"), installs: map[string]CatalogInstall{}, disabledPath: filepath.Join(dataDir, "disabled-mcp.json"), dataDir: dataDir, homeDir: homeDir}
	service.loadCatalogInstalls()
	service.loadDisabledMCPs()
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
	retention := settings.SnapshotRetention
	if retention <= 0 {
		retention = 50
	}
	_ = s.core.PruneSnapshots(retention)
	s.mu.Lock()
	if s.ctx != nil {
		s.mu.Unlock()
		return
	}
	s.ctx, s.cancel = context.WithCancel(context.Background())
	ctx := s.ctx
	s.mu.Unlock()
	if cached, ok, err := s.core.LoadCache(ctx); err == nil && ok {
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
	s.state.Updates = model.UpdateReport{CheckedAt: s.state.Updates.CheckedAt}
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
	cancel()
	s.mu.Lock()
	s.scanCancel = nil
	s.state.Scanning = false
	s.state.Snapshot = result
	s.state.Updates = model.UpdateReport{CheckedAt: s.state.Updates.CheckedAt}
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
		s.syncFromCore()
	}
	return op, err
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

func (s *AppService) RevealSkill(skillID string) error {
	return s.core.RevealSkill(s.context(), skillID)
}

func (s *AppService) RevealCLI(cliID string) error {
	return s.core.RevealCLI(s.context(), cliID)
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
	return s.core.SaveSettings(ctx, settings)
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
