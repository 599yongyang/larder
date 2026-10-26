package inventory

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"

	"larder/internal/core/clis"
	"larder/internal/core/doctor"
	"larder/internal/core/mcp"
	"larder/internal/core/model"
	"larder/internal/core/ops"
	"larder/internal/core/skills"
	"larder/internal/core/store"
	"larder/internal/core/updates"
	"larder/internal/core/usage"
	"larder/internal/platform"
)

type Service struct {
	paths        platform.LocalPaths
	fs           platform.FileSystem
	store        *store.Store
	scanner      *skills.Scanner
	cliScanner   *clis.Scanner
	runner       platform.CommandRunner
	fetcher      platform.Fetcher
	linker       platform.Linker
	snapshotRoot string
	pendingPlans map[string]model.Plan

	mu       sync.RWMutex
	latest   model.ScanResult
	updates  model.UpdateReport
	settings model.Settings
	scanMu   sync.Mutex
}

func NewService(paths platform.LocalPaths, fs platform.FileSystem, db *store.Store) *Service {
	if fs == nil {
		fs = platform.OSFileSystem{}
	}
	return &Service{paths: paths, fs: fs, store: db, scanner: skills.NewScanner(fs), cliScanner: clis.NewScanner(fs), runner: platform.OSCommandRunner{}, fetcher: platform.NewCachedFetcher(platform.HTTPFetcher{}, 2*time.Minute, 6), linker: platform.OSLinker{}, pendingPlans: map[string]model.Plan{}}
}

func (s *Service) LoadCache(ctx context.Context) (model.ScanResult, bool, error) {
	if s.store == nil {
		return model.ScanResult{}, false, nil
	}
	settings, err := s.store.LoadSettings(ctx)
	if err != nil {
		return model.ScanResult{}, false, err
	}
	s.mu.Lock()
	s.settings = settings
	s.mu.Unlock()
	result, ok, err := s.store.LoadLatest(ctx)
	if err != nil || !ok {
		return result, ok, err
	}
	updateReport, hasUpdates, err := s.store.LoadUpdates(ctx)
	if err != nil {
		return result, false, err
	}
	if hasUpdates && !validUpdateReport(updateReport) {
		_ = s.store.SaveUpdates(ctx, model.UpdateReport{})
		hasUpdates = false
	}
	s.mu.Lock()
	s.latest = result
	if hasUpdates {
		s.updates = updateReport
	}
	s.mu.Unlock()
	return result, true, nil
}

func (s *Service) Settings(ctx context.Context) (model.Settings, error) {
	s.mu.RLock()
	settings := s.settings
	s.mu.RUnlock()
	if s.store != nil {
		loaded, err := s.store.LoadSettings(ctx)
		if err != nil {
			return settings, err
		}
		settings = loaded
		s.mu.Lock()
		s.settings = settings
		s.mu.Unlock()
	}
	return settings, nil
}

func (s *Service) SaveSettings(ctx context.Context, settings model.Settings) (model.Settings, error) {
	settings.ProjectWorkspaces = uniquePaths(settings.ProjectWorkspaces)
	if s.store != nil {
		if err := s.store.SaveSettings(ctx, settings); err != nil {
			return settings, err
		}
	}
	s.mu.Lock()
	s.settings = settings
	s.mu.Unlock()
	return settings, nil
}

func (s *Service) IgnoreAgent(ctx context.Context, path, label, rootID string) (model.Settings, error) {
	settings, err := s.Settings(ctx)
	if err != nil {
		return model.Settings{}, err
	}
	path = filepath.Clean(path)
	for _, ignored := range settings.IgnoredAgents {
		if filepath.Clean(ignored.Path) == path {
			return settings, nil
		}
	}
	settings.IgnoredAgents = append(settings.IgnoredAgents, model.IgnoredAgent{RootID: rootID, Label: label, Path: path})
	return s.SaveSettings(ctx, settings)
}

func (s *Service) RevealSkill(ctx context.Context, skillID string) error {
	var path string
	for _, skill := range s.Snapshot().Skills {
		if skill.ID == skillID {
			path = skill.Path
			break
		}
	}
	if path == "" {
		return errors.New("skill not found")
	}
	if _, err := os.Stat(path); err != nil {
		return err
	}
	var command *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		command = exec.Command("open", "-R", path)
	case "windows":
		command = exec.Command("explorer", "/select,", path)
	default:
		command = exec.Command("xdg-open", filepath.Dir(path))
	}
	return command.Start()
}

func (s *Service) RevealCLI(ctx context.Context, cliID string) error {
	var path string
	for _, cli := range s.Snapshot().CLIs {
		if cli.ID == cliID {
			path = cli.Path
			break
		}
	}
	if path == "" {
		return errors.New("cli not found")
	}
	if _, err := os.Stat(path); err != nil {
		return err
	}
	var command *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		command = exec.Command("open", "-R", path)
	case "windows":
		command = exec.Command("explorer", "/select,", path)
	default:
		command = exec.Command("xdg-open", filepath.Dir(path))
	}
	return command.Start()
}

func (s *Service) TrashAgent(ctx context.Context, path string) (model.Settings, error) {
	settings, err := s.Settings(ctx)
	if err != nil {
		return model.Settings{}, err
	}
	path = filepath.Clean(path)
	found := false
	for _, agent := range settings.IgnoredAgents {
		if filepath.Clean(agent.Path) == path {
			found = true
			break
		}
	}
	if !found {
		return settings, errors.New("agent must be ignored before cleaning")
	}
	home, err := s.paths.HomeDir()
	if err != nil {
		return settings, err
	}
	rel, err := filepath.Rel(home, path)
	if err != nil || rel == "." || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return settings, errors.New("agent path is outside the home directory")
	}
	if filepath.Clean(path) == filepath.Join(home, ".agents", "skills") {
		return settings, errors.New("the shared skills library cannot be cleaned")
	}
	base := filepath.Base(path)
	if base != "skills" && base != "skills-cursor" {
		return settings, errors.New("cleanup only supports agent skills directories")
	}
	if runtime.GOOS != "darwin" {
		return settings, errors.New("trash cleanup is currently supported on macOS only")
	}
	if _, err := os.Lstat(path); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return s.removeIgnoredAgent(ctx, settings, path)
		}
		return settings, err
	}
	trash := filepath.Join(home, ".Trash")
	if err := os.MkdirAll(trash, 0o700); err != nil {
		return settings, err
	}
	destination := filepath.Join(trash, base)
	if _, err := os.Lstat(destination); err == nil {
		destination = filepath.Join(trash, fmt.Sprintf("%s-%d", base, time.Now().UnixNano()))
	}
	if err := os.Rename(path, destination); err != nil {
		return settings, err
	}
	if s.snapshotRoot != "" {
		if err := os.MkdirAll(s.snapshotRoot, 0o700); err == nil {
			manifest, _ := json.MarshalIndent(map[string]string{"original": path, "trashed": destination}, "", "  ")
			_ = os.WriteFile(filepath.Join(s.snapshotRoot, fmt.Sprintf("agent-cleanup-%d.json", time.Now().UnixNano())), manifest, 0o600)
		}
	}
	return s.removeIgnoredAgent(ctx, settings, path)
}

func (s *Service) removeIgnoredAgent(ctx context.Context, settings model.Settings, path string) (model.Settings, error) {
	filtered := settings.IgnoredAgents[:0]
	for _, agent := range settings.IgnoredAgents {
		if filepath.Clean(agent.Path) != filepath.Clean(path) {
			filtered = append(filtered, agent)
		}
	}
	settings.IgnoredAgents = filtered
	return s.SaveSettings(ctx, settings)
}

func (s *Service) RestoreAgent(ctx context.Context, path string) (model.Settings, error) {
	settings, err := s.Settings(ctx)
	if err != nil {
		return model.Settings{}, err
	}
	path = filepath.Clean(path)
	filtered := settings.IgnoredAgents[:0]
	for _, ignored := range settings.IgnoredAgents {
		if filepath.Clean(ignored.Path) != path {
			filtered = append(filtered, ignored)
		}
	}
	settings.IgnoredAgents = filtered
	return s.SaveSettings(ctx, settings)
}

func (s *Service) Snapshot() model.ScanResult {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.latest
}

func (s *Service) Scan(ctx context.Context) (model.ScanResult, error) {
	s.scanMu.Lock()
	defer s.scanMu.Unlock()
	home, err := s.paths.HomeDir()
	if err != nil {
		return model.ScanResult{}, err
	}
	// M1 scope: only global + known, adapted agent directories are scanned.
	// Unknown agent directories are a fact, but treating them as matrix
	// columns creates thousands of misleading "missing" cells (ADR-0020,
	// ADR-0029). They will be surfaced in settings in a later slice instead.
	roots := s.paths.DefaultRoots(home)
	settings, _ := s.Settings(ctx)
	var projectRoots []string
	var projectIssues []model.ScanIssue
	projectIssue := func(kind, path string, err error) {
		projectIssues = append(projectIssues, model.ScanIssue{Kind: kind, Path: path, Message: err.Error(), Recoverable: true, At: time.Now()})
	}
	for _, workspace := range settings.ProjectWorkspaces {
		if _, err := os.ReadDir(workspace); err != nil {
			kind := "scan_failure"
			if platform.IsPermissionError(err) {
				kind = "permission"
			}
			projectIssue(kind, workspace, err)
		} else {
			for _, name := range []string{".agents", ".claude", ".codex", ".cursor"} {
				path := filepath.Join(workspace, name)
				if _, err := os.Stat(path); err != nil && !errors.Is(err, os.ErrNotExist) {
					kind := "scan_failure"
					if platform.IsPermissionError(err) {
						kind = "permission"
					}
					projectIssue(kind, path, err)
				}
			}
		}
		projectSpecs := s.paths.ProjectRoots(workspace)
		for _, spec := range projectSpecs {
			roots = append(roots, spec)
			projectRoots = append(projectRoots, spec.Path)
		}
	}
	// Discover additional conventional agent skill directories that are not in
	// the known registry. Only directories containing at least one SKILL.md
	// are kept, so an empty config folder never becomes a matrix column.
	if unknown, discoveryErr := s.paths.DiscoverUnknownRoots(home, roots); discoveryErr == nil {
		roots = append(roots, unknown...)
	}
	ignored := make(map[string]bool, len(settings.IgnoredAgents))
	for _, agent := range settings.IgnoredAgents {
		ignored[filepath.Clean(agent.Path)] = true
	}
	visibleRoots := make([]platform.RootSpec, 0, len(roots))
	for _, root := range roots {
		if (root.Kind == platform.RootKnownAgent || root.Kind == platform.RootUnknownAgent) && ignored[filepath.Clean(root.Path)] {
			continue
		}
		visibleRoots = append(visibleRoots, root)
	}
	result, scanErr := s.scanner.Scan(ctx, skills.Options{Roots: visibleRoots, LockfilePath: s.paths.LockfilePath(home)})
	result.ProjectRoots = projectRoots
	if len(projectIssues) > 0 {
		result.Issues = append(result.Issues, projectIssues...)
		if result.Status == model.ScanComplete {
			result.Status = model.ScanPartial
		}
	}
	cliItems, cliIssues := s.cliScanner.Scan(ctx, clis.Options{Dirs: s.paths.PathDirs(home), Home: home})
	result.CLIs = cliItems
	result.Issues = append(result.Issues, cliIssues...)
	s.mu.Lock()
	nextUpdates := s.updates
	s.mu.Unlock()
	if nextUpdates.CheckedAt.IsZero() && s.store != nil {
		if loaded, ok, err := s.store.LoadUpdates(ctx); err == nil && ok && validUpdateReport(loaded) {
			nextUpdates = loaded
			s.mu.Lock()
			s.updates = loaded
			s.mu.Unlock()
		}
	}
	if scanErr == nil {
		nextUpdates = s.pruneInstalledNewSkills(nextUpdates, result)
	}
	if scanErr == nil && s.store != nil {
		if err := s.store.SaveScan(ctx, result); err != nil {
			return result, err
		}
		if err := s.store.SaveUpdates(ctx, nextUpdates); err != nil {
			return result, err
		}
	}
	s.mu.Lock()
	s.latest = result
	s.updates = nextUpdates
	s.mu.Unlock()
	return result, scanErr
}

func (s *Service) pruneInstalledNewSkills(report model.UpdateReport, result model.ScanResult) model.UpdateReport {
	installed := make(map[string]bool, len(result.Skills))
	for _, skill := range result.Skills {
		if skill.HasSkillMD {
			installed[skill.Name] = true
		}
	}
	items := make([]model.UpdateItem, 0, len(report.Items))
	removed := 0
	for _, item := range report.Items {
		if item.Kind == "skill" && item.State == "new" && installed[item.Name] {
			removed++
			continue
		}
		items = append(items, item)
	}
	if removed == 0 {
		return report
	}
	report.Items = items
	report.SkillAdded -= removed
	if report.SkillAdded < 0 {
		report.SkillAdded = 0
	}
	return report
}

func (s *Service) Doctor() []model.Finding {
	return doctor.RunWithUpdates(s.Snapshot(), s.Updates())
}

// CheckUpdates asks the installed package managers for latest versions. It is
// explicit user action, read-only, and never installs or changes anything.
func (s *Service) CheckUpdates(ctx context.Context) (model.UpdateReport, error) {
	home, err := s.paths.HomeDir()
	if err != nil {
		return model.UpdateReport{}, err
	}
	snapshot := s.Snapshot()
	checkCtx, cancel := context.WithTimeout(ctx, 90*time.Second)
	defer cancel()
	npmCache := ""
	if cache, err := os.UserCacheDir(); err == nil {
		npmCache = filepath.Join(cache, "larder", "npm")
	}
	report := updates.Check(checkCtx, updates.Options{
		CLIs:        snapshot.CLIs,
		Skills:      snapshot.Skills,
		Dirs:        s.paths.PathDirs(home),
		Runner:      s.runner,
		FS:          s.fs,
		Fetcher:     s.fetcher,
		Timeout:     60 * time.Second,
		NpmCacheDir: npmCache,
	})
	if s.store != nil {
		if err := s.store.SaveUpdates(ctx, report); err != nil {
			return report, err
		}
	}
	s.mu.Lock()
	s.updates = report
	s.mu.Unlock()
	return report, nil
}

func (s *Service) SetSnapshotRoot(path string) { s.snapshotRoot = path }

// PruneSnapshots keeps the newest max snapshot directories. Operations keep
// only their last 50 records, so older snapshots can no longer be reached from
// History and would otherwise grow without bound.
func (s *Service) PruneSnapshots(max int) error {
	if s.snapshotRoot == "" || max <= 0 {
		return nil
	}
	entries, err := os.ReadDir(s.snapshotRoot)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	trashSnapshots := map[string]bool{}
	if s.store != nil {
		if operations, err := s.store.LoadOperations(context.Background(), 500); err == nil {
			for _, operation := range operations {
				if operation.Status == "success" && (operation.Operation == "uninstall_skill" || operation.Operation == "mcp_remove") && operation.SnapshotPath != "" {
					trashSnapshots[filepath.Clean(operation.SnapshotPath)] = true
				}
			}
		}
	}
	type snapshot struct {
		path    string
		modTime time.Time
	}
	snapshots := make([]snapshot, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}
		path := filepath.Join(s.snapshotRoot, entry.Name())
		if trashSnapshots[filepath.Clean(path)] {
			// Trash retention controls these, not the version snapshot count.
			continue
		}
		snapshots = append(snapshots, snapshot{path: path, modTime: info.ModTime()})
	}
	sort.Slice(snapshots, func(i, j int) bool { return snapshots[i].modTime.After(snapshots[j].modTime) })
	if len(snapshots) <= max {
		return nil
	}
	for _, snapshot := range snapshots[max:] {
		_ = os.RemoveAll(snapshot.path)
	}
	return nil
}

func (s *Service) SetCommandRunner(runner platform.CommandRunner) { s.runner = runner }

func (s *Service) PlanLink(ctx context.Context, skillID string, rootIDs []string) (model.Plan, error) {
	if err := ctx.Err(); err != nil {
		return model.Plan{}, err
	}
	var skill *model.Skill
	for _, item := range s.Snapshot().Skills {
		if item.ID == skillID {
			skill = &item
			break
		}
	}
	if skill == nil {
		return model.Plan{}, errors.New("skill not found")
	}
	roots := map[string]model.Root{}
	for _, root := range s.Snapshot().Roots {
		roots[root.ID] = root
	}
	targets := make([]ops.LinkTarget, 0, len(rootIDs))
	for _, rootID := range rootIDs {
		root, ok := roots[rootID]
		if !ok {
			return model.Plan{}, fmt.Errorf("agent root not found: %s", rootID)
		}
		if root.Kind != model.RootKnownAgent && root.Kind != model.RootUnknownAgent {
			return model.Plan{}, fmt.Errorf("root is not an agent directory: %s", rootID)
		}
		if root.Status != model.RootAvailable {
			return model.Plan{}, fmt.Errorf("agent root is not available: %s", rootID)
		}
		targets = append(targets, ops.LinkTarget{RootID: root.ID, RootLabel: root.Label, Path: filepath.Join(root.Path, skill.Name)})
	}
	plan, err := ops.PlanLink(ops.LinkRequest{SkillID: skill.ID, SkillName: skill.Name, SourcePath: skill.Path, Targets: targets}, s.fs)
	if err != nil {
		return model.Plan{}, err
	}
	s.mu.Lock()
	s.pendingPlans[plan.ID] = plan
	s.mu.Unlock()
	return plan, nil
}

func (s *Service) PlanInstallSkill(ctx context.Context, repo, skillPath, name string) (model.Plan, error) {
	if repo == "" || skillPath == "" || name == "" {
		return model.Plan{}, errors.New("repo, skill path and name are required")
	}
	if strings.ContainsAny(name, "/\\") || name == "." || name == ".." {
		return model.Plan{}, errors.New("invalid skill name")
	}
	home, err := s.paths.HomeDir()
	if err != nil {
		return model.Plan{}, err
	}
	var globalPath string
	for _, root := range s.paths.DefaultRoots(home) {
		if root.ID == "global" {
			globalPath = root.Path
		}
	}
	if globalPath == "" {
		return model.Plan{}, errors.New("global skill root not found")
	}
	target := filepath.Join(globalPath, name)
	if _, err := s.fs.Lstat(target); err == nil {
		return model.Plan{}, errors.New("target already exists")
	}
	plan, err := ops.PlanInstallSkill(repo, skillPath, target)
	if err != nil {
		return model.Plan{}, err
	}
	s.mu.Lock()
	s.pendingPlans[plan.ID] = plan
	s.mu.Unlock()
	return plan, nil
}

func (s *Service) InstallUpstreamSkills(ctx context.Context, repo string, items []model.UpstreamSkillInstall, progress func(done, total int, name string)) ([]model.Operation, error) {
	if repo == "" || len(items) == 0 {
		return nil, errors.New("repository and Skills are required")
	}
	home, err := s.paths.HomeDir()
	if err != nil {
		return nil, err
	}
	var globalPath string
	for _, root := range s.paths.DefaultRoots(home) {
		if root.ID == "global" {
			globalPath = root.Path
			break
		}
	}
	if globalPath == "" {
		return nil, errors.New("global skill root not found")
	}
	pending := make([]model.UpstreamSkillInstall, 0, len(items))
	for _, item := range items {
		if strings.ContainsAny(item.Name, "/\\") || item.Name == "." || item.Name == ".." || strings.TrimSpace(item.SkillPath) == "" {
			return nil, errors.New("invalid Skill target")
		}
		if _, err := s.fs.Lstat(filepath.Join(globalPath, item.Name)); err == nil {
			_ = s.registerInstalledSource(repo, item.Name, item.SkillPath)
			continue
		}
		pending = append(pending, item)
	}
	items = pending
	if len(items) == 0 {
		_, _ = s.Scan(ctx)
		return nil, nil
	}
	results := make([]model.Operation, 0, len(items))
	if progress != nil {
		progress(0, len(items), "")
	}
	defer func() { _, _ = s.Scan(ctx) }()
	for _, item := range items {
		plan, planErr := ops.PlanInstallSkill(repo, item.SkillPath, filepath.Join(globalPath, item.Name))
		if planErr != nil {
			return results, planErr
		}
		op, applyErr := ops.ApplyInstallSkill(ctx, plan, s.snapshotRoot, s.fetcher, s.fs)
		if applyErr != nil || op.Status != "success" {
			if s.store != nil {
				_ = s.store.SaveOperation(ctx, op)
			}
			if applyErr != nil {
				return results, applyErr
			}
			return results, fmt.Errorf("install failed: %s", item.Name)
		}
		if files, hashErr := ops.FileHashes(plan.TargetPath); hashErr == nil {
			s.saveSkillBaseline(ctx, plan.TargetPath, files)
		}
		if home, homeErr := s.paths.HomeDir(); homeErr == nil {
			_ = ops.SnapshotLockfile(op.SnapshotPath, s.paths.LockfilePath(home))
		}
		if sourceErr := s.registerInstalledSource(repo, item.Name, item.SkillPath); sourceErr != nil {
			op.Message = strings.TrimSpace(op.Message + "；来源未登记")
		}
		s.markNewSkillInstalled(ctx, plan.SourcePath)
		if s.store != nil {
			_ = s.store.SaveOperation(ctx, op)
		}
		results = append(results, op)
		if progress != nil {
			progress(len(results), len(items), item.Name)
		}
	}
	return results, nil
}

func (s *Service) PlanInstallCatalogSkill(ctx context.Context, repo, skillID, name string) (model.Plan, error) {
	if repo == "" || skillID == "" || name == "" {
		return model.Plan{}, errors.New("repo, skill and name are required")
	}
	if strings.ContainsAny(name, "/\\") || name == "." || name == ".." {
		return model.Plan{}, errors.New("invalid skill name")
	}
	home, err := s.paths.HomeDir()
	if err != nil {
		return model.Plan{}, err
	}
	var globalPath string
	for _, root := range s.paths.DefaultRoots(home) {
		if root.ID == "global" {
			globalPath = root.Path
		}
	}
	if globalPath == "" {
		return model.Plan{}, errors.New("global skill root not found")
	}
	target := filepath.Join(globalPath, name)
	if _, err := s.fs.Lstat(target); err == nil {
		return model.Plan{}, errors.New("target already exists")
	}
	plan, err := ops.PlanInstallCatalogSkill(repo, skillID, target)
	if err != nil {
		return model.Plan{}, err
	}
	s.mu.Lock()
	s.pendingPlans[plan.ID] = plan
	s.mu.Unlock()
	return plan, nil
}

func (s *Service) PlanUninstallSkill(ctx context.Context, skillID string) (model.Plan, error) {
	if err := ctx.Err(); err != nil {
		return model.Plan{}, err
	}
	var skill *model.Skill
	for i := range s.Snapshot().Skills {
		if s.Snapshot().Skills[i].ID == skillID {
			item := s.Snapshot().Skills[i]
			skill = &item
			break
		}
	}
	if skill == nil {
		return model.Plan{}, errors.New("skill not found")
	}
	plan, err := ops.PlanUninstallSkill(*skill)
	if err != nil {
		return model.Plan{}, err
	}
	s.mu.Lock()
	s.pendingPlans[plan.ID] = plan
	s.mu.Unlock()
	return plan, nil
}

func (s *Service) MCPTargets() []model.MCPAgent {
	s.mu.RLock()
	projects := append([]string(nil), s.settings.ProjectWorkspaces...)
	s.mu.RUnlock()
	targets := append(mcp.WritableTargets(), mcp.ProjectTargets(projects)...)
	agents := make([]model.MCPAgent, 0, len(targets))
	for _, target := range targets {
		agents = append(agents, model.MCPAgent{Label: target.Label, Path: target.Path, Enabled: true})
	}
	return agents
}

func (s *Service) PlanMCPAdd(ctx context.Context, name, transport, command string, args []string, url string, env map[string]string, agentPaths []string) (model.Plan, error) {
	if err := ctx.Err(); err != nil {
		return model.Plan{}, err
	}
	targets := map[string]model.MCPAgent{}
	for _, target := range s.MCPTargets() {
		targets[filepath.Clean(target.Path)] = target
	}
	agents := make([]model.MCPAgent, 0, len(agentPaths))
	for _, path := range agentPaths {
		target, ok := targets[filepath.Clean(path)]
		if !ok {
			return model.Plan{}, fmt.Errorf("unknown MCP config: %s", path)
		}
		agents = append(agents, target)
	}
	plan, err := ops.PlanMCPAdd(name, transport, command, args, url, env, agents)
	if err != nil {
		return model.Plan{}, err
	}
	s.mu.Lock()
	s.pendingPlans[plan.ID] = plan
	s.mu.Unlock()
	return plan, nil
}

func (s *Service) DetachMCPAgent(ctx context.Context, agentPath, name, label string) (ops.DetachedMCP, model.Operation, error) {
	detached, op, err := ops.DetachMCPEntry(ctx, agentPath, name, label, s.snapshotRoot)
	if s.store != nil {
		_ = s.store.SaveOperation(ctx, op)
	}
	if err == nil {
		_, _ = s.Scan(ctx)
	}
	return detached, op, err
}

func (s *Service) AttachMCPAgent(ctx context.Context, detached ops.DetachedMCP) (model.Operation, error) {
	op, err := ops.AttachMCPEntry(ctx, detached, s.snapshotRoot)
	if s.store != nil {
		_ = s.store.SaveOperation(ctx, op)
	}
	if err == nil {
		_, _ = s.Scan(ctx)
	}
	return op, err
}

func (s *Service) PlanMCPUpdate(ctx context.Context, serverID, name, command string, args []string, url string, agentPaths []string) (model.Plan, error) {
	if err := ctx.Err(); err != nil {
		return model.Plan{}, err
	}
	var server *model.MCPServer
	for _, item := range mcp.Scan(ctx) {
		if item.ID == serverID {
			copy := item
			server = &copy
			break
		}
	}
	if server == nil {
		return model.Plan{}, errors.New("mcp server not found")
	}
	plan, err := ops.PlanMCPUpdate(*server, name, command, args, url, agentPaths)
	if err != nil {
		return model.Plan{}, err
	}
	s.mu.Lock()
	s.pendingPlans[plan.ID] = plan
	s.mu.Unlock()
	return plan, nil
}

func (s *Service) PlanMCPRemove(ctx context.Context, serverID string, agentPaths []string) (model.Plan, error) {
	if err := ctx.Err(); err != nil {
		return model.Plan{}, err
	}
	var server *model.MCPServer
	for _, item := range mcp.Scan(ctx) {
		if item.ID == serverID {
			copy := item
			server = &copy
			break
		}
	}
	if server == nil {
		return model.Plan{}, errors.New("mcp server not found")
	}
	plan, err := ops.PlanMCPRemove(*server, agentPaths)
	if err != nil {
		return model.Plan{}, err
	}
	s.mu.Lock()
	s.pendingPlans[plan.ID] = plan
	s.mu.Unlock()
	return plan, nil
}

func (s *Service) PlanSkillUpdate(ctx context.Context, skillID string) (model.Plan, error) {
	if err := ctx.Err(); err != nil {
		return model.Plan{}, err
	}
	var skill *model.Skill
	for i := range s.Snapshot().Skills {
		if s.Snapshot().Skills[i].ID == skillID || s.Snapshot().Skills[i].Name == skillID {
			item := s.Snapshot().Skills[i]
			skill = &item
			break
		}
	}
	if skill == nil {
		return model.Plan{}, errors.New("skill not found")
	}
	plan, err := ops.PlanSkillUpdate(*skill, ops.GitHubRepo(*skill))
	if err != nil {
		return model.Plan{}, err
	}
	s.mu.Lock()
	s.pendingPlans[plan.ID] = plan
	s.mu.Unlock()
	return plan, nil
}

func (s *Service) PlanCLI(ctx context.Context, cliID string, uninstall bool, latest string) (model.Plan, error) {
	if err := ctx.Err(); err != nil {
		return model.Plan{}, err
	}
	var cli *model.CLIInfo
	for i := range s.Snapshot().CLIs {
		if s.Snapshot().CLIs[i].ID == cliID {
			item := s.Snapshot().CLIs[i]
			cli = &item
			break
		}
	}
	if cli == nil {
		return model.Plan{}, errors.New("CLI not found")
	}
	plan, err := ops.PlanCLICommand(*cli, uninstall, latest)
	if err != nil {
		return model.Plan{}, err
	}
	s.mu.Lock()
	s.pendingPlans[plan.ID] = plan
	s.mu.Unlock()
	return plan, nil
}

func (s *Service) PlanDisableLink(ctx context.Context, skillID string, rootIDs []string) (model.Plan, error) {
	if err := ctx.Err(); err != nil {
		return model.Plan{}, err
	}
	snapshot := s.Snapshot()
	var skill *model.Skill
	for i := range snapshot.Skills {
		if snapshot.Skills[i].ID == skillID {
			skill = &snapshot.Skills[i]
			break
		}
	}
	if skill == nil {
		return model.Plan{}, errors.New("skill not found")
	}
	roots := map[string]model.Root{}
	for _, root := range snapshot.Roots {
		roots[root.ID] = root
	}
	links := map[string]model.AgentLink{}
	for _, link := range snapshot.AgentLinks {
		if link.SkillID == skillID {
			links[link.RootID] = link
		}
	}
	targets := make([]ops.LinkTarget, 0, len(rootIDs))
	for _, rootID := range rootIDs {
		root, ok := roots[rootID]
		if !ok || (root.Kind != model.RootKnownAgent && root.Kind != model.RootUnknownAgent) || root.Status != model.RootAvailable {
			return model.Plan{}, fmt.Errorf("agent root is not available: %s", rootID)
		}
		link, ok := links[rootID]
		if !ok || link.Mode != model.LinkLinked || link.Path == "" {
			return model.Plan{}, fmt.Errorf("root is not a linked skill: %s", rootID)
		}
		targets = append(targets, ops.LinkTarget{RootID: root.ID, RootLabel: root.Label, Path: link.Path})
	}
	plan, err := ops.PlanDisableLink(ops.LinkRequest{SkillID: skill.ID, SkillName: skill.Name, SourcePath: skill.Path, Targets: targets}, s.fs)
	if err != nil {
		return model.Plan{}, err
	}
	s.mu.Lock()
	s.pendingPlans[plan.ID] = plan
	s.mu.Unlock()
	return plan, nil
}

func (s *Service) PlanSyncCopy(ctx context.Context, skillID string, rootIDs []string) (model.Plan, error) {
	if err := ctx.Err(); err != nil {
		return model.Plan{}, err
	}
	snapshot := s.Snapshot()
	var skill *model.Skill
	for i := range snapshot.Skills {
		if snapshot.Skills[i].ID == skillID {
			skill = &snapshot.Skills[i]
			break
		}
	}
	if skill == nil {
		return model.Plan{}, errors.New("skill not found")
	}
	roots := map[string]model.Root{}
	for _, root := range snapshot.Roots {
		roots[root.ID] = root
	}
	links := map[string]model.AgentLink{}
	for _, link := range snapshot.AgentLinks {
		if link.SkillID == skillID {
			links[link.RootID] = link
		}
	}
	targets := make([]ops.LinkTarget, 0, len(rootIDs))
	for _, rootID := range rootIDs {
		root, ok := roots[rootID]
		if !ok || (root.Kind != model.RootKnownAgent && root.Kind != model.RootUnknownAgent) || root.Status != model.RootAvailable {
			return model.Plan{}, fmt.Errorf("agent root is not available: %s", rootID)
		}
		link, ok := links[rootID]
		if !ok || link.Mode != model.LinkCopied || link.Path == "" {
			return model.Plan{}, fmt.Errorf("root is not a copied skill: %s", rootID)
		}
		targets = append(targets, ops.LinkTarget{RootID: root.ID, RootLabel: root.Label, Path: link.Path})
	}
	plan, err := ops.PlanSyncCopy(ops.LinkRequest{SkillID: skill.ID, SkillName: skill.Name, SourcePath: skill.Path, Targets: targets}, s.fs)
	if err != nil {
		return model.Plan{}, err
	}
	s.mu.Lock()
	s.pendingPlans[plan.ID] = plan
	s.mu.Unlock()
	return plan, nil
}

func (s *Service) PlanCopyToLink(ctx context.Context, skillID string, rootIDs []string) (model.Plan, error) {
	if err := ctx.Err(); err != nil {
		return model.Plan{}, err
	}
	snapshot := s.Snapshot()
	var skill *model.Skill
	for i := range snapshot.Skills {
		if snapshot.Skills[i].ID == skillID {
			skill = &snapshot.Skills[i]
			break
		}
	}
	if skill == nil {
		return model.Plan{}, errors.New("skill not found")
	}
	roots := map[string]model.Root{}
	for _, root := range snapshot.Roots {
		roots[root.ID] = root
	}
	links := map[string]model.AgentLink{}
	for _, link := range snapshot.AgentLinks {
		if link.SkillID == skillID {
			links[link.RootID] = link
		}
	}
	targets := make([]ops.LinkTarget, 0, len(rootIDs))
	for _, rootID := range rootIDs {
		root, ok := roots[rootID]
		if !ok || (root.Kind != model.RootKnownAgent && root.Kind != model.RootUnknownAgent) || root.Status != model.RootAvailable {
			return model.Plan{}, fmt.Errorf("agent root is not available: %s", rootID)
		}
		link, ok := links[rootID]
		if !ok || link.Mode != model.LinkCopied || link.Path == "" {
			return model.Plan{}, fmt.Errorf("root is not a copied skill: %s", rootID)
		}
		targets = append(targets, ops.LinkTarget{RootID: root.ID, RootLabel: root.Label, Path: link.Path})
	}
	plan, err := ops.PlanCopyToLink(ops.LinkRequest{SkillID: skill.ID, SkillName: skill.Name, SourcePath: skill.Path, Targets: targets}, s.fs)
	if err != nil {
		return model.Plan{}, err
	}
	s.mu.Lock()
	s.pendingPlans[plan.ID] = plan
	s.mu.Unlock()
	return plan, nil
}

// ApplyPlan dispatches a confirmed plan to the matching executor.
func (s *Service) ApplyPlan(ctx context.Context, planID string) (model.Operation, error) {
	return s.ApplyPlanWithResolutions(ctx, planID, nil)
}

func (s *Service) ApplyPlanWithResolutions(ctx context.Context, planID string, resolutions map[string]string) (model.Operation, error) {
	s.mu.RLock()
	plan, ok := s.pendingPlans[planID]
	s.mu.RUnlock()
	if !ok {
		return model.Operation{}, errors.New("plan not found or already used")
	}
	if plan.Operation == "cli_update" || plan.Operation == "cli_uninstall" {
		return s.applyCLIPlan(ctx, plan)
	}
	if plan.Operation == "skill_update" {
		return s.applySkillPlan(ctx, plan, resolutions)
	}
	if plan.Operation == "install_skill" {
		op, err := ops.ApplyInstallSkill(ctx, plan, s.snapshotRoot, s.fetcher, s.fs)
		if err == nil && op.Status == "success" {
			if files, hashErr := ops.FileHashes(plan.TargetPath); hashErr == nil {
				s.saveSkillBaseline(ctx, plan.TargetPath, files)
			}
			repo := ""
			if len(plan.Changes) > 0 {
				repo = plan.Changes[0].Source
			}
			if home, homeErr := s.paths.HomeDir(); homeErr == nil {
				_ = ops.SnapshotLockfile(op.SnapshotPath, s.paths.LockfilePath(home))
			}
			if sourceErr := s.registerInstalledSource(repo, filepath.Base(plan.TargetPath), plan.SourcePath); sourceErr != nil && !strings.Contains(op.Message, "来源未登记") {
				op.Message = strings.TrimSpace(op.Message + "；来源未登记")
			}
			s.markNewSkillInstalled(ctx, plan.SourcePath)
		}
		if s.store != nil {
			_ = s.store.SaveOperation(ctx, op)
		}
		s.mu.Lock()
		delete(s.pendingPlans, plan.ID)
		s.mu.Unlock()
		_, _ = s.Scan(ctx)
		return op, err
	}
	if plan.Operation == "mcp_remove" {
		op, err := ops.ApplyMCPRemove(ctx, plan, s.snapshotRoot, s.fs)
		if s.store != nil {
			_ = s.store.SaveOperation(ctx, op)
		}
		s.mu.Lock()
		delete(s.pendingPlans, plan.ID)
		s.mu.Unlock()
		_, _ = s.Scan(ctx)
		return op, err
	}
	if plan.Operation == "mcp_add" {
		op, err := ops.ApplyMCPAdd(ctx, plan, s.snapshotRoot, s.fs)
		if s.store != nil {
			_ = s.store.SaveOperation(ctx, op)
		}
		s.mu.Lock()
		delete(s.pendingPlans, plan.ID)
		s.mu.Unlock()
		_, _ = s.Scan(ctx)
		return op, err
	}
	if plan.Operation == "mcp_update" {
		op, err := ops.ApplyMCPUpdate(ctx, plan, s.snapshotRoot, s.fs)
		if s.store != nil {
			_ = s.store.SaveOperation(ctx, op)
		}
		s.mu.Lock()
		delete(s.pendingPlans, plan.ID)
		s.mu.Unlock()
		_, _ = s.Scan(ctx)
		return op, err
	}
	if plan.Operation == "uninstall_skill" {
		op, err := ops.ApplyUninstallSkill(ctx, plan, s.snapshotRoot, s.fs)
		if s.store != nil {
			_ = s.store.SaveOperation(ctx, op)
		}
		s.mu.Lock()
		delete(s.pendingPlans, plan.ID)
		s.mu.Unlock()
		_, _ = s.Scan(ctx)
		return op, err
	}
	return s.ApplyLinkPlan(ctx, planID)
}

func (s *Service) markNewSkillInstalled(ctx context.Context, skillPath string) {
	if skillPath == "" {
		return
	}
	s.mu.Lock()
	report := s.updates
	items := make([]model.UpdateItem, 0, len(report.Items))
	removed := 0
	normalizedPath := strings.TrimSuffix(skillPath, "/SKILL.md")
	for _, item := range report.Items {
		if item.Kind == "skill" && item.State == "new" && (item.SkillPath == skillPath || strings.TrimSuffix(item.SkillPath, "/SKILL.md") == normalizedPath) {
			removed++
			continue
		}
		items = append(items, item)
	}
	if removed == 0 {
		s.mu.Unlock()
		return
	}
	report.Items = items
	report.SkillAdded -= removed
	if report.SkillAdded < 0 {
		report.SkillAdded = 0
	}
	s.updates = report
	s.mu.Unlock()
	if s.store != nil {
		_ = s.store.SaveUpdates(ctx, report)
	}
}

func (s *Service) markSkillUpdateApplied(ctx context.Context, name string) {
	if name == "" {
		return
	}
	s.mu.Lock()
	report := s.updates
	items := make([]model.UpdateItem, 0, len(report.Items))
	removed := 0
	for _, item := range report.Items {
		if item.Kind == "skill" && item.Name == name {
			removed++
			continue
		}
		items = append(items, item)
	}
	if removed == 0 {
		s.mu.Unlock()
		return
	}
	report.Items = items
	report.SkillChanged -= removed
	if report.SkillChanged < 0 {
		report.SkillChanged = 0
	}
	report.Available -= removed
	if report.Available < 0 {
		report.Available = 0
	}
	s.updates = report
	s.mu.Unlock()
	if s.store != nil {
		_ = s.store.SaveUpdates(ctx, report)
	}
}

func (s *Service) applySkillPlan(ctx context.Context, plan model.Plan, resolutions map[string]string) (model.Operation, error) {
	baseline := map[string]string{}
	if s.store != nil {
		if saved, ok, loadErr := s.store.LoadSkillBaseline(ctx, plan.TargetPath); loadErr == nil && ok {
			baseline = saved.Files
		}
	}
	op, err := ops.ApplySkillUpdateWithResolutions(ctx, plan, s.snapshotRoot, s.fetcher, s.fs, baseline, resolutions)
	if err == nil && op.Status == "success" {
		s.markSkillUpdateApplied(ctx, plan.SkillName)
		files := map[string]string{}
		if saved, loadErr := ops.LoadBaselineManifest(op.SnapshotPath); loadErr == nil {
			files = saved
		} else if hashes, hashErr := ops.FileHashes(plan.TargetPath); hashErr == nil {
			files = hashes
		}
		s.saveSkillBaseline(ctx, plan.TargetPath, files)
	}
	if s.store != nil {
		_ = s.store.SaveOperation(ctx, op)
	}
	s.mu.Lock()
	delete(s.pendingPlans, plan.ID)
	s.mu.Unlock()
	_, _ = s.Scan(ctx)
	return op, err
}

func (s *Service) applyCLIPlan(ctx context.Context, plan model.Plan) (model.Operation, error) {
	now := time.Now()
	op := model.Operation{ID: fmt.Sprintf("op-%s-%d", plan.ID, now.UnixNano()), PlanID: plan.ID, Operation: plan.Operation, Status: "failed", CreatedAt: now}
	home, err := s.paths.HomeDir()
	if err != nil {
		return op, err
	}
	executable, ok := s.runner.Find(plan.Command, s.paths.PathDirs(home))
	if !ok {
		return op, fmt.Errorf("package manager not found: %s", plan.Command)
	}
	env := []string{"HOMEBREW_NO_AUTO_UPDATE=1", "HOMEBREW_NO_INSTALL_CLEANUP=1", "NPM_CONFIG_UPDATE_NOTIFIER=false"}
	if cache, err := os.UserCacheDir(); err == nil {
		env = append(env, "NPM_CONFIG_CACHE="+filepath.Join(cache, "larder", "npm"))
	}
	if s.snapshotRoot != "" {
		dir := filepath.Join(s.snapshotRoot, fmt.Sprintf("cli-%d", now.UnixNano()))
		if err := os.MkdirAll(dir, 0o700); err != nil {
			return op, err
		}
		raw, _ := json.MarshalIndent(plan, "", "  ")
		if err := os.WriteFile(filepath.Join(dir, "plan.json"), raw, 0o600); err != nil {
			return op, err
		}
		op.SnapshotPath = dir
	}
	runCtx, cancel := context.WithTimeout(ctx, 90*time.Second)
	stdout, stderr, runErr := s.runner.Run(runCtx, executable, plan.Args, env)
	cancel()
	op.Changes = append(op.Changes, plan.Command+" "+strings.Join(plan.Args, " "))
	if runErr != nil {
		op.Message = strings.TrimSpace(string(stderr))
		if op.Message == "" {
			op.Message = runErr.Error()
		}
	} else {
		op.Status = "success"
		op.Message = strings.TrimSpace(string(stdout))
		if len(op.Message) > 2000 {
			op.Message = op.Message[:2000]
		}
	}
	if s.store != nil {
		_ = s.store.SaveOperation(ctx, op)
	}
	s.mu.Lock()
	delete(s.pendingPlans, plan.ID)
	s.mu.Unlock()
	_, _ = s.Scan(ctx)
	if runErr != nil {
		return op, fmt.Errorf("package manager failed: %w", runErr)
	}
	return op, nil
}

// ApplyLinkPlan executes a previously generated plan after re-validating it,
// writing a snapshot first. Callers must have explicit human confirmation.
func (s *Service) ApplyLinkPlan(ctx context.Context, planID string) (model.Operation, error) {
	s.mu.RLock()
	plan, ok := s.pendingPlans[planID]
	s.mu.RUnlock()
	if !ok {
		return model.Operation{}, errors.New("plan not found or already used")
	}
	op, err := ops.ApplyLinkPlan(ctx, plan, s.snapshotRoot, s.fs, s.linker)
	if s.store != nil {
		_ = s.store.SaveOperation(ctx, op)
	}
	s.mu.Lock()
	delete(s.pendingPlans, planID)
	s.mu.Unlock()
	if err != nil {
		return op, err
	}
	_, _ = s.Scan(ctx)
	return op, nil
}

func (s *Service) UsageStats(ctx context.Context) ([]model.UsageStat, error) {
	settings, err := s.Settings(ctx)
	if err != nil || !settings.UsageStatsEnabled {
		return nil, err
	}
	home, err := s.paths.HomeDir()
	if err != nil {
		return nil, err
	}
	data, err := s.fs.ReadFile(filepath.Join(home, ".zsh_history"))
	if err != nil {
		return nil, nil
	}
	known := map[string]bool{}
	for _, cli := range s.Snapshot().CLIs {
		known[cli.Name] = true
	}
	return usage.ParseZshHistory(data, known, 50), nil
}

func (s *Service) SkillIcon(ctx context.Context, skillID string) (string, error) {
	var skill *model.Skill
	for i := range s.Snapshot().Skills {
		if s.Snapshot().Skills[i].ID == skillID {
			item := s.Snapshot().Skills[i]
			skill = &item
			break
		}
	}
	if skill == nil || skill.Icon == "" {
		return "", nil
	}
	if strings.HasPrefix(skill.Icon, "http://") || strings.HasPrefix(skill.Icon, "https://") || strings.HasPrefix(skill.Icon, "data:") {
		return skill.Icon, nil
	}
	rel, err := filepath.Rel(skill.Path, skill.Icon)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", errors.New("icon is outside the skill folder")
	}
	raw, err := s.fs.ReadFile(skill.Icon)
	if err != nil || len(raw) > 4<<20 {
		return "", err
	}
	ext := strings.ToLower(filepath.Ext(skill.Icon))
	mime := map[string]string{".svg": "image/svg+xml", ".png": "image/png", ".webp": "image/webp", ".jpg": "image/jpeg", ".jpeg": "image/jpeg"}[ext]
	if mime == "" {
		return "", nil
	}
	return "data:" + mime + ";base64," + base64.StdEncoding.EncodeToString(raw), nil
}

func (s *Service) PruneTimeline(ctx context.Context) error {
	if s.store == nil {
		return nil
	}
	return s.store.PruneTimeline(ctx)
}

func (s *Service) Timeline(ctx context.Context) ([]model.TimelineEvent, error) {
	if s.store == nil {
		return nil, nil
	}
	return s.store.LoadTimeline(ctx, 100)
}

func (s *Service) Operations(ctx context.Context) ([]model.Operation, error) {
	if s.store == nil {
		return nil, nil
	}
	return s.store.LoadOperations(ctx, 50)
}

func (s *Service) RollbackOperation(ctx context.Context, operationID string) (model.Operation, error) {
	if s.store == nil {
		return model.Operation{}, errors.New("operation store is unavailable")
	}
	original, ok, err := s.store.GetOperation(ctx, operationID)
	if err != nil {
		return model.Operation{}, err
	}
	if !ok {
		return model.Operation{}, errors.New("operation not found")
	}
	if (original.Operation != "link_skill" && original.Operation != "replace_copy" && original.Operation != "sync_copy" && original.Operation != "disable_link" && original.Operation != "skill_update" && original.Operation != "install_skill" && original.Operation != "uninstall_skill" && original.Operation != "mcp_remove" && original.Operation != "rollback" && original.Operation != "command_create" && original.Operation != "command_update" && original.Operation != "command_convert" && original.Operation != "skill_edit" && original.Operation != "skill_create" && original.Operation != "skill_rename" && original.Operation != "command_delete") || original.Status != "success" {
		return model.Operation{}, errors.New("operation is not rollback-able")
	}
	safetySnapshot, err := ops.CreateRestoreSnapshot(original.SnapshotPath, s.snapshotRoot)
	if err != nil {
		return model.Operation{}, fmt.Errorf("back up current files before restore: %w", err)
	}
	if err := s.Rollback(ctx, original.SnapshotPath); err != nil {
		return model.Operation{}, err
	}
	// Restore the upstream baseline for the restored Skill so the next update
	// comparison starts from the version the user just restored.
	if path := skillPathForOperation(original); path != "" {
		if files, hashErr := ops.FileHashes(path); hashErr == nil {
			s.saveSkillBaseline(ctx, path, files)
		}
	}
	original.Status = "rolled_back"
	_ = s.store.SaveOperation(ctx, original)
	now := time.Now()
	rollback := model.Operation{
		ID: fmt.Sprintf("op-rollback-%d", now.UnixNano()), PlanID: original.PlanID, Operation: "rollback", Status: "success",
		Message: "已回滚 " + original.ID + "；恢复前状态已保存", SnapshotPath: safetySnapshot, CreatedAt: now,
	}
	_ = s.store.SaveOperation(ctx, rollback)
	_, _ = s.Scan(ctx)
	return rollback, nil
}

func (s *Service) Rollback(ctx context.Context, snapshotPath string) error {
	if s.snapshotRoot == "" || !strings.HasPrefix(filepath.Clean(snapshotPath), filepath.Clean(s.snapshotRoot)+string(filepath.Separator)) {
		return errors.New("snapshot path is outside the snapshot directory")
	}
	return ops.RollbackSnapshot(snapshotPath, s.fs, s.linker)
}

func (s *Service) PruneTrash(ctx context.Context, days int) error {
	if s.store == nil || days <= 0 {
		return nil
	}
	cutoff := time.Now().AddDate(0, 0, -days)
	operations, err := s.store.LoadOperations(ctx, 500)
	if err != nil {
		return err
	}
	for _, operation := range operations {
		if operation.Status != "success" || (operation.Operation != "uninstall_skill" && operation.Operation != "mcp_remove") || operation.CreatedAt.After(cutoff) {
			continue
		}
		if operation.SnapshotPath != "" && s.snapshotRoot != "" {
			clean := filepath.Clean(operation.SnapshotPath)
			if strings.HasPrefix(clean, filepath.Clean(s.snapshotRoot)+string(filepath.Separator)) {
				_ = os.RemoveAll(clean)
			}
		}
		operation.Status = "deleted"
		_ = s.store.SaveOperation(ctx, operation)
	}
	return nil
}

func (s *Service) PurgeTrash(ctx context.Context, operationID string) error {
	if s.store == nil {
		return errors.New("operation store is unavailable")
	}
	op, ok, err := s.store.GetOperation(ctx, operationID)
	if err != nil {
		return err
	}
	if !ok {
		return errors.New("operation not found")
	}
	if op.Status != "success" || (op.Operation != "uninstall_skill" && op.Operation != "mcp_remove") {
		return errors.New("operation is not a trash item")
	}
	if op.SnapshotPath != "" && s.snapshotRoot != "" {
		clean := filepath.Clean(op.SnapshotPath)
		root := filepath.Clean(s.snapshotRoot) + string(filepath.Separator)
		if !strings.HasPrefix(clean, root) {
			return errors.New("snapshot path is outside the snapshot directory")
		}
		if err := os.RemoveAll(clean); err != nil {
			return err
		}
	}
	op.Status = "deleted"
	if err := s.store.SaveOperation(ctx, op); err != nil {
		return err
	}
	return nil
}

func skillPathForOperation(operation model.Operation) string {
	for _, change := range operation.Changes {
		for _, prefix := range []string{"updated:", "uninstalled:"} {
			if strings.HasPrefix(change, prefix) {
				return strings.TrimPrefix(change, prefix)
			}
		}
	}
	return ""
}

func (s *Service) registerInstalledSource(repo, name, skillPath string) error {
	repo = strings.TrimSpace(repo)
	name = strings.TrimSpace(name)
	if repo == "" || name == "" {
		return nil
	}
	home, err := s.paths.HomeDir()
	if err != nil {
		return err
	}
	path := s.paths.LockfilePath(home)
	data := map[string]any{}
	if raw, readErr := os.ReadFile(path); readErr == nil {
		if err := json.Unmarshal(raw, &data); err != nil {
			return err
		}
	} else if !errors.Is(readErr, os.ErrNotExist) {
		return readErr
	}
	skills, _ := data["skills"].(map[string]any)
	if skills == nil {
		skills = map[string]any{}
	}
	entry, _ := skills[name].(map[string]any)
	if entry == nil {
		entry = map[string]any{}
	}
	now := time.Now().UTC().Format(time.RFC3339)
	if _, ok := entry["installedAt"]; !ok {
		entry["installedAt"] = now
	}
	entry["source"] = repo
	entry["sourceType"] = "github"
	entry["sourceUrl"] = "https://github.com/" + repo
	entry["skillPath"] = skillPath
	entry["updatedAt"] = now
	skills[name] = entry
	data["skills"] = skills
	if _, ok := data["version"]; !ok {
		data["version"] = 1
	}
	raw, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".skill-lock-*.tmp")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if err := tmp.Chmod(0o600); err != nil {
		_ = tmp.Close()
		return err
	}
	if _, err := tmp.Write(raw); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpName, path)
}

func (s *Service) saveSkillBaseline(ctx context.Context, path string, files map[string]string) {
	if s.store == nil || path == "" || len(files) == 0 {
		return
	}
	_ = s.store.SaveSkillBaseline(ctx, model.SkillBaseline{Path: path, Files: files, UpdatedAt: time.Now()})
}

func (s *Service) PreviewSkillFileDiff(ctx context.Context, skillID, path string) (model.SkillUpdateFile, error) {
	var skill *model.Skill
	for i := range s.Snapshot().Skills {
		if s.Snapshot().Skills[i].ID == skillID || s.Snapshot().Skills[i].Name == skillID {
			item := s.Snapshot().Skills[i]
			skill = &item
			break
		}
	}
	if skill == nil {
		return model.SkillUpdateFile{}, errors.New("skill not found")
	}
	repo := ops.GitHubRepo(*skill)
	if repo == "" {
		return model.SkillUpdateFile{}, errors.New("skill has no GitHub source")
	}
	baseline := map[string]string{}
	if s.store != nil {
		if saved, ok, err := s.store.LoadSkillBaseline(ctx, skill.Path); err == nil && ok {
			baseline = saved.Files
		}
	}
	return ops.BuildSkillUpdateFileDiff(ctx, *skill, repo, path, s.fetcher, baseline)
}

func (s *Service) PreviewVersionDiff(ctx context.Context, operationID string) (model.SkillUpdateDiff, error) {
	if s.store == nil {
		return model.SkillUpdateDiff{}, errors.New("operation store is unavailable")
	}
	operation, ok, err := s.store.GetOperation(ctx, operationID)
	if err != nil {
		return model.SkillUpdateDiff{}, err
	}
	if !ok {
		return model.SkillUpdateDiff{}, errors.New("operation not found")
	}
	path := ""
	for _, change := range operation.Changes {
		for _, prefix := range []string{"updated:", "uninstalled:"} {
			if strings.HasPrefix(change, prefix) {
				path = strings.TrimPrefix(change, prefix)
			}
		}
	}
	if path == "" {
		return model.SkillUpdateDiff{}, errors.New("operation has no skill path")
	}
	var skill model.Skill
	for _, item := range s.Snapshot().Skills {
		if item.Path == path {
			skill = item
			break
		}
	}
	if skill.Path == "" {
		skill = model.Skill{ID: path, Name: filepath.Base(path), Path: path}
	}
	return ops.BuildVersionDiff(ctx, operation, skill)
}

func (s *Service) PreviewSkillUpdate(ctx context.Context, skillID string) (model.SkillUpdateDiff, error) {
	var skill *model.Skill
	snapshot := s.Snapshot()
	for i := range snapshot.Skills {
		if snapshot.Skills[i].ID == skillID || snapshot.Skills[i].Name == skillID {
			item := snapshot.Skills[i]
			skill = &item
			break
		}
	}
	if skill == nil {
		return model.SkillUpdateDiff{}, errors.New("skill not found")
	}
	repo := ops.GitHubRepo(*skill)
	if repo == "" {
		return model.SkillUpdateDiff{}, errors.New("skill has no GitHub source")
	}
	baseline := map[string]string{}
	if s.store != nil {
		if saved, ok, err := s.store.LoadSkillBaseline(ctx, skill.Path); err == nil && ok {
			baseline = saved.Files
		}
	}
	return ops.BuildSkillUpdateDiff(ctx, *skill, repo, s.fetcher, baseline, false)
}

func (s *Service) Updates() model.UpdateReport {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.updates
}

func (s *Service) Store() *store.Store { return s.store }

func validUpdateReport(report model.UpdateReport) bool {
	if report.CheckedAt.IsZero() {
		return false
	}
	// Older scans wrote an empty placeholder report with only checkedAt. It
	// must not suppress the next automatic check or hide real updates.
	return len(report.Items) > 0 || report.Scanned > 0
}

func uniquePaths(values []string) []string {
	seen := map[string]bool{}
	result := make([]string, 0, len(values))
	for _, value := range values {
		value = filepath.Clean(value)
		if value == "." || value == "" || seen[value] {
			continue
		}
		seen[value] = true
		result = append(result, value)
	}
	return result
}

var ErrNoCache = errors.New("no cached scan")
