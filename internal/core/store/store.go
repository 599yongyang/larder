package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"larder/internal/core/model"
	_ "modernc.org/sqlite"
)

type Store struct {
	db *sql.DB
}

func Open(path string) (*Store, error) {
	if path == "" {
		return nil, errors.New("empty sqlite path")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, fmt.Errorf("create app data directory: %w", err)
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	store := &Store{db: db}
	if err := store.configure(); err != nil {
		_ = db.Close()
		return nil, err
	}
	if err := store.migrate(); err != nil {
		_ = db.Close()
		return nil, err
	}
	return store, nil
}

func (s *Store) configure() error {
	for _, pragma := range []string{"PRAGMA journal_mode = WAL", "PRAGMA synchronous = NORMAL", "PRAGMA foreign_keys = ON", "PRAGMA busy_timeout = 5000"} {
		if _, err := s.db.Exec(pragma); err != nil {
			return fmt.Errorf("sqlite %s: %w", pragma, err)
		}
	}
	return nil
}

func (s *Store) migrate() error {
	_, err := s.db.Exec(`
CREATE TABLE IF NOT EXISTS schema_migrations (version INTEGER PRIMARY KEY, applied_at TEXT NOT NULL);
CREATE TABLE IF NOT EXISTS skills (
  id TEXT PRIMARY KEY,
  name TEXT NOT NULL,
  path TEXT NOT NULL,
  root_id TEXT NOT NULL,
  root_kind TEXT NOT NULL,
  source TEXT,
  source_type TEXT,
  source_url TEXT,
  skill_path TEXT,
  locked INTEGER NOT NULL DEFAULT 0,
  has_skill_md INTEGER NOT NULL DEFAULT 0,
  description TEXT,
  desc_chars INTEGER NOT NULL DEFAULT 0,
  desc_tokens INTEGER NOT NULL DEFAULT 0,
  folder_hash TEXT,
  content_hash TEXT,
  scan_state TEXT,
  scanned_at TEXT NOT NULL,
  is_agent_local INTEGER NOT NULL DEFAULT 0,
  is_project INTEGER NOT NULL DEFAULT 0,
  copy_drift INTEGER NOT NULL DEFAULT 0,
  icon TEXT,
  icon_source TEXT
);
CREATE TABLE IF NOT EXISTS agent_dirs (
  id TEXT PRIMARY KEY,
  label TEXT NOT NULL,
  logical_agent TEXT,
  path TEXT NOT NULL,
  kind TEXT NOT NULL,
  known INTEGER NOT NULL DEFAULT 0,
  status TEXT NOT NULL,
  error TEXT,
  entry_count INTEGER NOT NULL DEFAULT 0
);
CREATE TABLE IF NOT EXISTS agent_links (
  skill_id TEXT NOT NULL,
  root_id TEXT NOT NULL,
  mode TEXT NOT NULL,
  path TEXT,
  target_path TEXT,
  content_hash TEXT,
  drift INTEGER NOT NULL DEFAULT 0,
  visible INTEGER NOT NULL DEFAULT 0,
  reason TEXT,
  PRIMARY KEY (skill_id, root_id)
);
CREATE TABLE IF NOT EXISTS lock_skills (
  name TEXT PRIMARY KEY,
  source TEXT,
  source_type TEXT,
  source_url TEXT,
  skill_path TEXT,
  plugin_name TEXT,
  installed_at TEXT,
  updated_at TEXT
);
CREATE TABLE IF NOT EXISTS clis (
  id TEXT PRIMARY KEY,
  name TEXT NOT NULL,
  path TEXT NOT NULL,
  owner TEXT NOT NULL,
  owner_detail TEXT,
  version TEXT,
  status TEXT NOT NULL,
  duplicate_count INTEGER NOT NULL DEFAULT 0,
  paths_json TEXT,
  scanned_at TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS scan_runs (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  started_at TEXT NOT NULL,
  finished_at TEXT NOT NULL,
  status TEXT NOT NULL,
  root_count INTEGER NOT NULL,
  error_json TEXT
);
CREATE TABLE IF NOT EXISTS scan_issues (
  scan_id INTEGER NOT NULL,
  kind TEXT NOT NULL,
  path TEXT,
  message TEXT NOT NULL,
  recoverable INTEGER NOT NULL DEFAULT 0,
  at TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS settings (
  key TEXT PRIMARY KEY,
  value_json TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS timeline_events (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  scan_id INTEGER NOT NULL,
  kind TEXT NOT NULL,
  skill_name TEXT NOT NULL,
  path TEXT,
  old_hash TEXT,
  new_hash TEXT,
  at TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS skill_baselines (
  path TEXT PRIMARY KEY,
  files_json TEXT NOT NULL,
  updated_at TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS operations (
  id TEXT PRIMARY KEY,
  plan_id TEXT NOT NULL,
  operation TEXT NOT NULL,
  status TEXT NOT NULL,
  message TEXT,
  changes_json TEXT,
  snapshot_path TEXT,
  created_at TEXT NOT NULL
);
`)
	if err != nil {
		return err
	}
	for _, column := range []struct{ name, ddl string }{
		{"icon", "ALTER TABLE skills ADD COLUMN icon TEXT"},
		{"icon_source", "ALTER TABLE skills ADD COLUMN icon_source TEXT"},
	} {
		if _, err := s.db.Exec(column.ddl); err != nil && !strings.Contains(err.Error(), "duplicate column name") {
			return err
		}
	}
	_, err = s.db.Exec(`INSERT OR IGNORE INTO schema_migrations(version, applied_at) VALUES(1, ?)`, time.Now().UTC().Format(time.RFC3339Nano))
	return err
}

func (s *Store) SaveScan(ctx context.Context, result model.ScanResult) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	rollback := func(e error) error { _ = tx.Rollback(); return e }
	type priorSkill struct {
		name string
		hash string
	}
	previous := map[string]priorSkill{}
	prevRows, err := tx.QueryContext(ctx, `SELECT name, folder_hash, path FROM skills`)
	if err != nil {
		return rollback(err)
	}
	for prevRows.Next() {
		var name, hash, path string
		if err := prevRows.Scan(&name, &hash, &path); err != nil {
			_ = prevRows.Close()
			return rollback(err)
		}
		previous[path] = priorSkill{name: name, hash: hash}
	}
	_ = prevRows.Close()
	hadPrevious := len(previous) > 0
	suppressed := map[string]bool{}
	if rows, queryErr := tx.QueryContext(ctx, `SELECT changes_json FROM operations WHERE status='success' AND created_at >= ?`, time.Now().Add(-5*time.Minute).UTC().Format(time.RFC3339Nano)); queryErr == nil {
		for rows.Next() {
			var raw string
			if rows.Scan(&raw) == nil {
				var changes []string
				_ = json.Unmarshal([]byte(raw), &changes)
				for _, change := range changes {
					for _, prefix := range []string{"installed:", "uninstalled:", "updated:", "removed:"} {
						if strings.HasPrefix(change, prefix) {
							suppressed[strings.TrimPrefix(change, prefix)] = true
						}
					}
				}
			}
		}
		_ = rows.Close()
	}
	for _, table := range []string{"skills", "agent_dirs", "agent_links", "lock_skills", "clis", "scan_issues"} {
		if _, err := tx.ExecContext(ctx, "DELETE FROM "+table); err != nil {
			return rollback(err)
		}
	}
	issueJSON, _ := json.Marshal(result.Issues)
	res, err := tx.ExecContext(ctx, `INSERT INTO scan_runs(started_at, finished_at, status, root_count, error_json) VALUES(?,?,?,?,?)`, result.StartedAt.UTC().Format(time.RFC3339Nano), result.FinishedAt.UTC().Format(time.RFC3339Nano), result.Status, len(result.Roots), string(issueJSON))
	if err != nil {
		return rollback(err)
	}
	scanID, err := res.LastInsertId()
	if err != nil {
		return rollback(err)
	}
	current := map[string]priorSkill{}
	for _, skill := range result.Skills {
		// The same display name can exist in several agent folders. Path is the
		// stable identity; keying by name made duplicate Skills appear to change
		// on every scan.
		if _, seen := current[skill.Path]; seen {
			continue
		}
		current[skill.Path] = priorSkill{name: skill.Name, hash: skill.FolderHash}
	}
	for path, skill := range current {
		if suppressed[path] {
			continue
		}
		old, ok := previous[path]
		if !ok {
			// Do not seed the timeline with the entire initial inventory.
			if !hadPrevious {
				continue
			}
			if _, err := tx.ExecContext(ctx, `INSERT INTO timeline_events(scan_id,kind,skill_name,path,old_hash,new_hash,at) VALUES(?,?,?,?,?,?,?)`, scanID, "added", skill.name, path, "", skill.hash, time.Now().UTC().Format(time.RFC3339Nano)); err != nil {
				return rollback(err)
			}
			continue
		}
		if old.hash != skill.hash {
			if _, err := tx.ExecContext(ctx, `INSERT INTO timeline_events(scan_id,kind,skill_name,path,old_hash,new_hash,at) VALUES(?,?,?,?,?,?,?)`, scanID, "changed", skill.name, path, old.hash, skill.hash, time.Now().UTC().Format(time.RFC3339Nano)); err != nil {
				return rollback(err)
			}
		}
	}
	for path, skill := range previous {
		if suppressed[path] {
			continue
		}
		if _, ok := current[path]; !ok {
			if _, err := tx.ExecContext(ctx, `INSERT INTO timeline_events(scan_id,kind,skill_name,path,old_hash,new_hash,at) VALUES(?,?,?,?,?,?,?)`, scanID, "removed", skill.name, path, skill.hash, "", time.Now().UTC().Format(time.RFC3339Nano)); err != nil {
				return rollback(err)
			}
		}
	}
	for _, root := range result.Roots {
		_, err = tx.ExecContext(ctx, `INSERT INTO agent_dirs(id,label,logical_agent,path,kind,known,status,error,entry_count) VALUES(?,?,?,?,?,?,?,?,?)`, root.ID, root.Label, root.LogicalAgent, root.Path, root.Kind, boolInt(root.Known), root.Status, root.Error, root.EntryCount)
		if err != nil {
			return rollback(err)
		}
	}
	for _, skill := range result.Skills {
		_, err = tx.ExecContext(ctx, `INSERT INTO skills(id,name,path,root_id,root_kind,source,source_type,source_url,skill_path,locked,has_skill_md,description,desc_chars,desc_tokens,folder_hash,content_hash,scan_state,scanned_at,is_agent_local,is_project,copy_drift,icon,icon_source) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`, skill.ID, skill.Name, skill.Path, skill.RootID, skill.RootKind, skill.Source, skill.SourceType, skill.SourceURL, skill.SkillPath, boolInt(skill.Locked), boolInt(skill.HasSkillMD), skill.Description, skill.DescChars, skill.DescTokens, skill.FolderHash, skill.ContentHash, skill.ScanState, skill.ScannedAt.UTC().Format(time.RFC3339Nano), boolInt(skill.IsAgentLocal), boolInt(skill.IsProject), boolInt(skill.CopyDrift), skill.Icon, skill.IconSource)
		if err != nil {
			return rollback(err)
		}
	}
	for _, link := range result.AgentLinks {
		_, err = tx.ExecContext(ctx, `INSERT INTO agent_links(skill_id,root_id,mode,path,target_path,content_hash,drift,visible,reason) VALUES(?,?,?,?,?,?,?,?,?)`, link.SkillID, link.RootID, link.Mode, link.Path, link.TargetPath, link.ContentHash, boolInt(link.Drift), boolInt(link.Visible), link.Reason)
		if err != nil {
			return rollback(err)
		}
	}
	for _, lock := range result.LockSkills {
		_, err = tx.ExecContext(ctx, `INSERT INTO lock_skills(name,source,source_type,source_url,skill_path,plugin_name,installed_at,updated_at) VALUES(?,?,?,?,?,?,?,?)`, lock.Name, lock.Source, lock.SourceType, lock.SourceURL, lock.SkillPath, lock.PluginName, lock.InstalledAt, lock.UpdatedAt)
		if err != nil {
			return rollback(err)
		}
	}
	for _, cli := range result.CLIs {
		pathsJSON, _ := json.Marshal(cli.Paths)
		_, err = tx.ExecContext(ctx, `INSERT INTO clis(id,name,path,owner,owner_detail,version,status,duplicate_count,paths_json,scanned_at) VALUES(?,?,?,?,?,?,?,?,?,?)`, cli.ID, cli.Name, cli.Path, cli.Owner, cli.OwnerDetail, cli.Version, cli.Status, cli.DuplicateCount, string(pathsJSON), cli.ScannedAt.UTC().Format(time.RFC3339Nano))
		if err != nil {
			return rollback(err)
		}
	}
	for _, issue := range result.Issues {
		_, err = tx.ExecContext(ctx, `INSERT INTO scan_issues(scan_id,kind,path,message,recoverable,at) VALUES(?,?,?,?,?,?)`, scanID, issue.Kind, issue.Path, issue.Message, boolInt(issue.Recoverable), issue.At.UTC().Format(time.RFC3339Nano))
		if err != nil {
			return rollback(err)
		}
	}
	return tx.Commit()
}

func (s *Store) LoadLatest(ctx context.Context) (model.ScanResult, bool, error) {
	var result model.ScanResult
	var start, finish, status, issueJSON string
	var rootCount int
	err := s.db.QueryRowContext(ctx, `SELECT started_at, finished_at, status, root_count, error_json FROM scan_runs ORDER BY id DESC LIMIT 1`).Scan(&start, &finish, &status, &rootCount, &issueJSON)
	if errors.Is(err, sql.ErrNoRows) {
		return model.ScanResult{}, false, nil
	}
	if err != nil {
		return result, false, err
	}
	result.SchemaVersion = model.SchemaVersion
	result.StartedAt, _ = time.Parse(time.RFC3339Nano, start)
	result.FinishedAt, _ = time.Parse(time.RFC3339Nano, finish)
	result.Status = model.ScanStatus(status)
	result.FromCache = true
	if issueJSON != "" {
		_ = json.Unmarshal([]byte(issueJSON), &result.Issues)
	}
	if err := s.loadRoots(ctx, &result); err != nil {
		return result, false, err
	}
	if err := s.loadSkills(ctx, &result); err != nil {
		return result, false, err
	}
	if err := s.loadLinks(ctx, &result); err != nil {
		return result, false, err
	}
	if err := s.loadLocks(ctx, &result); err != nil {
		return result, false, err
	}
	if err := s.loadCLIs(ctx, &result); err != nil {
		return result, false, err
	}
	_ = rootCount
	return result, true, nil
}

func (s *Store) loadRoots(ctx context.Context, result *model.ScanResult) error {
	rows, err := s.db.QueryContext(ctx, `SELECT id,label,logical_agent,path,kind,known,status,error,entry_count FROM agent_dirs ORDER BY id`)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var root model.Root
		var known int
		if err := rows.Scan(&root.ID, &root.Label, &root.LogicalAgent, &root.Path, &root.Kind, &known, &root.Status, &root.Error, &root.EntryCount); err != nil {
			return err
		}
		root.Known = known != 0
		result.Roots = append(result.Roots, root)
	}
	return rows.Err()
}

func (s *Store) loadSkills(ctx context.Context, result *model.ScanResult) error {
	rows, err := s.db.QueryContext(ctx, `SELECT id,name,path,root_id,root_kind,source,source_type,source_url,skill_path,locked,has_skill_md,description,desc_chars,desc_tokens,folder_hash,content_hash,scan_state,scanned_at,is_agent_local,is_project,copy_drift,icon,icon_source FROM skills ORDER BY name,path`)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var skill model.Skill
		var locked, hasMD, local, project, drift int
		var scanned string
		if err := rows.Scan(&skill.ID, &skill.Name, &skill.Path, &skill.RootID, &skill.RootKind, &skill.Source, &skill.SourceType, &skill.SourceURL, &skill.SkillPath, &locked, &hasMD, &skill.Description, &skill.DescChars, &skill.DescTokens, &skill.FolderHash, &skill.ContentHash, &skill.ScanState, &scanned, &local, &project, &drift, &skill.Icon, &skill.IconSource); err != nil {
			return err
		}
		skill.Locked = locked != 0
		skill.HasSkillMD = hasMD != 0
		skill.IsAgentLocal = local != 0
		skill.IsProject = project != 0
		skill.CopyDrift = drift != 0
		skill.ScannedAt, _ = time.Parse(time.RFC3339Nano, scanned)
		result.Skills = append(result.Skills, skill)
	}
	return rows.Err()
}

func (s *Store) loadCLIs(ctx context.Context, result *model.ScanResult) error {
	rows, err := s.db.QueryContext(ctx, `SELECT id,name,path,owner,owner_detail,version,status,duplicate_count,paths_json,scanned_at FROM clis ORDER BY name`)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var cli model.CLIInfo
		var pathsJSON, scanned string
		if err := rows.Scan(&cli.ID, &cli.Name, &cli.Path, &cli.Owner, &cli.OwnerDetail, &cli.Version, &cli.Status, &cli.DuplicateCount, &pathsJSON, &scanned); err != nil {
			return err
		}
		_ = json.Unmarshal([]byte(pathsJSON), &cli.Paths)
		cli.ScannedAt, _ = time.Parse(time.RFC3339Nano, scanned)
		result.CLIs = append(result.CLIs, cli)
	}
	return rows.Err()
}

func (s *Store) loadLinks(ctx context.Context, result *model.ScanResult) error {
	rows, err := s.db.QueryContext(ctx, `SELECT skill_id,root_id,mode,path,target_path,content_hash,drift,visible,reason FROM agent_links ORDER BY root_id,skill_id`)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var link model.AgentLink
		var drift, visible int
		if err := rows.Scan(&link.SkillID, &link.RootID, &link.Mode, &link.Path, &link.TargetPath, &link.ContentHash, &drift, &visible, &link.Reason); err != nil {
			return err
		}
		link.Drift = drift != 0
		link.Visible = visible != 0
		result.AgentLinks = append(result.AgentLinks, link)
	}
	return rows.Err()
}

func (s *Store) loadLocks(ctx context.Context, result *model.ScanResult) error {
	rows, err := s.db.QueryContext(ctx, `SELECT name,source,source_type,source_url,skill_path,plugin_name,installed_at,updated_at FROM lock_skills ORDER BY name`)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var lock model.LockSkill
		if err := rows.Scan(&lock.Name, &lock.Source, &lock.SourceType, &lock.SourceURL, &lock.SkillPath, &lock.PluginName, &lock.InstalledAt, &lock.UpdatedAt); err != nil {
			return err
		}
		result.LockSkills = append(result.LockSkills, lock)
	}
	return rows.Err()
}

func (s *Store) LoadSettings(ctx context.Context) (model.Settings, error) {
	var raw string
	err := s.db.QueryRowContext(ctx, `SELECT value_json FROM settings WHERE key='settings'`).Scan(&raw)
	if errors.Is(err, sql.ErrNoRows) {
		return model.Settings{AutoCheckUpdates: true, UpdateCheckIntervalHr: 24, SnapshotRetention: 50}, nil
	}
	if err != nil {
		return model.Settings{}, err
	}
	var settings model.Settings
	if err := json.Unmarshal([]byte(raw), &settings); err != nil {
		return model.Settings{}, err
	}
	// Older settings stored zero for both fields. Treat that as "not
	// configured yet" and opt into the default background check.
	if settings.UpdateCheckIntervalHr <= 0 {
		settings.UpdateCheckIntervalHr = 24
		settings.AutoCheckUpdates = true
	}
	if settings.SnapshotRetention <= 0 {
		settings.SnapshotRetention = 50
	}
	return settings, nil
}

func (s *Store) SaveUpdates(ctx context.Context, report model.UpdateReport) error {
	if report.CheckedAt.IsZero() {
		_, err := s.db.ExecContext(ctx, `DELETE FROM settings WHERE key='update_report'`)
		return err
	}
	raw, err := json.Marshal(report)
	if err != nil {
		return err
	}
	_, err = s.db.ExecContext(ctx, `INSERT INTO settings(key,value_json) VALUES('update_report',?) ON CONFLICT(key) DO UPDATE SET value_json=excluded.value_json`, string(raw))
	return err
}

func (s *Store) LoadUpdates(ctx context.Context) (model.UpdateReport, bool, error) {
	var raw string
	err := s.db.QueryRowContext(ctx, `SELECT value_json FROM settings WHERE key='update_report'`).Scan(&raw)
	if errors.Is(err, sql.ErrNoRows) {
		return model.UpdateReport{}, false, nil
	}
	if err != nil {
		return model.UpdateReport{}, false, err
	}
	var report model.UpdateReport
	if err := json.Unmarshal([]byte(raw), &report); err != nil {
		return model.UpdateReport{}, false, err
	}
	return report, !report.CheckedAt.IsZero(), nil
}

func (s *Store) SaveSkillBaseline(ctx context.Context, baseline model.SkillBaseline) error {
	if baseline.Path == "" || baseline.Files == nil {
		return errors.New("invalid skill baseline")
	}
	raw, err := json.Marshal(baseline.Files)
	if err != nil {
		return err
	}
	updated := baseline.UpdatedAt
	if updated.IsZero() {
		updated = time.Now()
	}
	_, err = s.db.ExecContext(ctx, `INSERT INTO skill_baselines(path,files_json,updated_at) VALUES(?,?,?) ON CONFLICT(path) DO UPDATE SET files_json=excluded.files_json,updated_at=excluded.updated_at`, baseline.Path, string(raw), updated.UTC().Format(time.RFC3339Nano))
	return err
}

func (s *Store) LoadSkillBaseline(ctx context.Context, path string) (model.SkillBaseline, bool, error) {
	var raw, updated string
	err := s.db.QueryRowContext(ctx, `SELECT files_json,updated_at FROM skill_baselines WHERE path=?`, path).Scan(&raw, &updated)
	if errors.Is(err, sql.ErrNoRows) {
		return model.SkillBaseline{}, false, nil
	}
	if err != nil {
		return model.SkillBaseline{}, false, err
	}
	var files map[string]string
	if err := json.Unmarshal([]byte(raw), &files); err != nil {
		return model.SkillBaseline{}, false, err
	}
	at, _ := time.Parse(time.RFC3339Nano, updated)
	return model.SkillBaseline{Path: path, Files: files, UpdatedAt: at}, true, nil
}

func (s *Store) DeleteSkillBaseline(ctx context.Context, path string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM skill_baselines WHERE path=?`, path)
	return err
}

func (s *Store) SaveOperation(ctx context.Context, op model.Operation) error {
	changes, _ := json.Marshal(op.Changes)
	_, err := s.db.ExecContext(ctx, `INSERT INTO operations(id,plan_id,operation,status,message,changes_json,snapshot_path,created_at) VALUES(?,?,?,?,?,?,?,?) ON CONFLICT(id) DO UPDATE SET status=excluded.status,message=excluded.message,changes_json=excluded.changes_json,snapshot_path=excluded.snapshot_path`, op.ID, op.PlanID, op.Operation, op.Status, op.Message, string(changes), op.SnapshotPath, op.CreatedAt.UTC().Format(time.RFC3339Nano))
	return err
}

func (s *Store) LoadTimeline(ctx context.Context, limit int) ([]model.TimelineEvent, error) {
	if limit <= 0 {
		limit = 100
	}
	rows, err := s.db.QueryContext(ctx, `SELECT id,scan_id,kind,skill_name,path,old_hash,new_hash,at FROM timeline_events ORDER BY id DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []model.TimelineEvent
	for rows.Next() {
		var event model.TimelineEvent
		var at string
		if err := rows.Scan(&event.ID, &event.ScanID, &event.Kind, &event.SkillName, &event.Path, &event.OldHash, &event.NewHash, &at); err != nil {
			return nil, err
		}
		event.At, _ = time.Parse(time.RFC3339Nano, at)
		result = append(result, event)
	}
	return result, rows.Err()
}

func (s *Store) PruneTimeline(ctx context.Context) error {
	// The first scan is the inventory baseline, not a user-visible change.
	_, err := s.db.ExecContext(ctx, `DELETE FROM timeline_events WHERE kind = 'added' AND scan_id = (SELECT MIN(id) FROM scan_runs)`)
	if err != nil {
		return err
	}
	_, err = s.db.ExecContext(ctx, `DELETE FROM timeline_events WHERE id NOT IN (SELECT MAX(id) FROM timeline_events GROUP BY kind, skill_name, path, old_hash, new_hash)`)
	if err != nil {
		return err
	}
	_, err = s.db.ExecContext(ctx, `DELETE FROM timeline_events WHERE id NOT IN (SELECT id FROM timeline_events ORDER BY id DESC LIMIT 1000)`)
	return err
}

func (s *Store) GetOperation(ctx context.Context, id string) (model.Operation, bool, error) {
	row := s.db.QueryRowContext(ctx, `SELECT id,plan_id,operation,status,message,changes_json,snapshot_path,created_at FROM operations WHERE id=?`, id)
	var op model.Operation
	var changesJSON, created string
	if err := row.Scan(&op.ID, &op.PlanID, &op.Operation, &op.Status, &op.Message, &changesJSON, &op.SnapshotPath, &created); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return model.Operation{}, false, nil
		}
		return model.Operation{}, false, err
	}
	_ = json.Unmarshal([]byte(changesJSON), &op.Changes)
	op.CreatedAt, _ = time.Parse(time.RFC3339Nano, created)
	return op, true, nil
}

func (s *Store) LoadOperations(ctx context.Context, limit int) ([]model.Operation, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := s.db.QueryContext(ctx, `SELECT id,plan_id,operation,status,message,changes_json,snapshot_path,created_at FROM operations ORDER BY created_at DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []model.Operation
	for rows.Next() {
		var op model.Operation
		var changesJSON, created string
		if err := rows.Scan(&op.ID, &op.PlanID, &op.Operation, &op.Status, &op.Message, &changesJSON, &op.SnapshotPath, &created); err != nil {
			return nil, err
		}
		_ = json.Unmarshal([]byte(changesJSON), &op.Changes)
		op.CreatedAt, _ = time.Parse(time.RFC3339Nano, created)
		result = append(result, op)
	}
	return result, rows.Err()
}

func (s *Store) SaveSettings(ctx context.Context, settings model.Settings) error {
	raw, err := json.Marshal(settings)
	if err != nil {
		return err
	}
	_, err = s.db.ExecContext(ctx, `INSERT INTO settings(key,value_json) VALUES('settings',?) ON CONFLICT(key) DO UPDATE SET value_json=excluded.value_json`, string(raw))
	return err
}

func (s *Store) Close() error { return s.db.Close() }

func boolInt(value bool) int {
	if value {
		return 1
	}
	return 0
}
