package model

import "time"

const SchemaVersion = "1"

type RootKind string

const (
	RootGlobal       RootKind = "global"
	RootKnownAgent   RootKind = "known_agent"
	RootUnknownAgent RootKind = "unknown_agent"
	RootProject      RootKind = "project"
)

type RootStatus string

const (
	RootAvailable        RootStatus = "available"
	RootUnavailable      RootStatus = "unavailable"
	RootNotInstalled     RootStatus = "not_installed"
	RootPermissionDenied RootStatus = "permission_denied"
)

type Root struct {
	ID           string     `json:"id"`
	Label        string     `json:"label"`
	LogicalAgent string     `json:"logicalAgent,omitempty"`
	Path         string     `json:"path"`
	Kind         RootKind   `json:"kind"`
	Known        bool       `json:"known"`
	Status       RootStatus `json:"status"`
	Error        string     `json:"error,omitempty"`
	EntryCount   int        `json:"entryCount"`
	SharedSkills bool       `json:"sharedSkills,omitempty"`
}

type Skill struct {
	ID           string    `json:"id"`
	Name         string    `json:"name"`
	Path         string    `json:"path"`
	RootID       string    `json:"rootId"`
	RootKind     RootKind  `json:"rootKind"`
	Source       string    `json:"source,omitempty"`
	SourceType   string    `json:"sourceType,omitempty"`
	SourceURL    string    `json:"sourceUrl,omitempty"`
	SkillPath    string    `json:"skillPath,omitempty"`
	Locked       bool      `json:"locked"`
	HasSkillMD   bool      `json:"hasSkillMd"`
	Description  string    `json:"description,omitempty"`
	DescChars    int       `json:"descChars"`
	DescTokens   int       `json:"descTokens"`
	FolderHash   string    `json:"folderHash,omitempty"`
	ContentHash  string    `json:"contentHash,omitempty"`
	ScanState    string    `json:"scanState"`
	ScannedAt    time.Time `json:"scannedAt"`
	IsAgentLocal bool      `json:"isAgentLocal"`
	IsProject    bool      `json:"isProject"`
	CopyDrift    bool      `json:"copyDrift"`
	Icon         string    `json:"icon,omitempty"`
	IconSource   string    `json:"iconSource,omitempty"`
	Bins         []string  `json:"bins,omitempty"`
	Origin       string    `json:"origin,omitempty"`
	OriginDetail string    `json:"originDetail,omitempty"`
}

type LinkMode string

const (
	LinkLinked      LinkMode = "linked"
	LinkCopied      LinkMode = "copied"
	LinkLocal       LinkMode = "local"
	LinkMissing     LinkMode = "missing"
	LinkBroken      LinkMode = "broken"
	LinkUnavailable LinkMode = "unavailable"
	LinkShared      LinkMode = "shared"
)

type AgentLink struct {
	SkillID     string   `json:"skillId"`
	RootID      string   `json:"rootId"`
	Mode        LinkMode `json:"mode"`
	Path        string   `json:"path,omitempty"`
	TargetPath  string   `json:"targetPath,omitempty"`
	ContentHash string   `json:"contentHash,omitempty"`
	Drift       bool     `json:"drift"`
	Visible     bool     `json:"visible"`
	Reason      string   `json:"reason,omitempty"`
}

type LockSkill struct {
	Name        string `json:"name"`
	Source      string `json:"source,omitempty"`
	SourceType  string `json:"sourceType,omitempty"`
	SourceURL   string `json:"sourceUrl,omitempty"`
	SkillPath   string `json:"skillPath,omitempty"`
	PluginName  string `json:"pluginName,omitempty"`
	InstalledAt string `json:"installedAt,omitempty"`
	UpdatedAt   string `json:"updatedAt,omitempty"`
}

type ScanIssue struct {
	Kind        string    `json:"kind"`
	Path        string    `json:"path,omitempty"`
	Message     string    `json:"message"`
	Recoverable bool      `json:"recoverable"`
	At          time.Time `json:"at"`
}

type ScanStatus string

const (
	ScanComplete  ScanStatus = "complete"
	ScanPartial   ScanStatus = "partial"
	ScanFailed    ScanStatus = "failed"
	ScanCancelled ScanStatus = "cancelled"
	ScanCached    ScanStatus = "cached"
)

type CLIPath struct {
	Path        string `json:"path"`
	TargetPath  string `json:"targetPath,omitempty"`
	Owner       string `json:"owner"`
	OwnerDetail string `json:"ownerDetail,omitempty"`
	Broken      bool   `json:"broken"`
	Effective   bool   `json:"effective"`
}

type CLIInfo struct {
	ID             string    `json:"id"`
	Name           string    `json:"name"`
	Path           string    `json:"path"`
	Owner          string    `json:"owner"`
	OwnerDetail    string    `json:"ownerDetail,omitempty"`
	Version        string    `json:"version,omitempty"`
	Status         string    `json:"status"`
	DuplicateCount int       `json:"duplicateCount"`
	Paths          []CLIPath `json:"paths,omitempty"`
	ScannedAt      time.Time `json:"scannedAt"`
}

type ScanResult struct {
	SchemaVersion string      `json:"schemaVersion"`
	StartedAt     time.Time   `json:"startedAt"`
	FinishedAt    time.Time   `json:"finishedAt"`
	Status        ScanStatus  `json:"status"`
	Roots         []Root      `json:"roots"`
	Skills        []Skill     `json:"skills"`
	AgentLinks    []AgentLink `json:"agentLinks"`
	LockSkills    []LockSkill `json:"lockSkills"`
	CLIs          []CLIInfo   `json:"clis"`
	Issues        []ScanIssue `json:"issues"`
	ProjectRoots  []string    `json:"projectRoots,omitempty"`
	FromCache     bool        `json:"fromCache"`
}

type Severity string

const (
	SeverityInfo  Severity = "info"
	SeverityWarn  Severity = "warn"
	SeverityError Severity = "error"
)

type UpdateItem struct {
	Kind           string   `json:"kind"`
	CLIID          string   `json:"cliId,omitempty"`
	Owner          string   `json:"owner"`
	Package        string   `json:"package,omitempty"`
	Name           string   `json:"name"`
	Commands       []string `json:"commands,omitempty"`
	CurrentVersion string   `json:"currentVersion,omitempty"`
	LatestVersion  string   `json:"latestVersion,omitempty"`
	Source         string   `json:"source,omitempty"`
	State          string   `json:"state"`
	Message        string   `json:"message,omitempty"`
}

type UpdateReport struct {
	CheckedAt    time.Time    `json:"checkedAt"`
	Items        []UpdateItem `json:"items"`
	Errors       []string     `json:"errors,omitempty"`
	Scanned      int          `json:"scanned"`
	Available    int          `json:"available"`
	Current      int          `json:"current"`
	SkillScanned int          `json:"skillScanned"`
	SkillChanged int          `json:"skillChanged"`
	SkillRemoved int          `json:"skillRemoved"`
	SkillCurrent int          `json:"skillCurrent"`
	SkillErrors  int          `json:"skillErrors"`
}

type Finding struct {
	ID          string   `json:"id"`
	Rule        string   `json:"rule"`
	Severity    Severity `json:"severity"`
	Title       string   `json:"title"`
	Message     string   `json:"message"`
	Path        string   `json:"path,omitempty"`
	SkillID     string   `json:"skillId,omitempty"`
	RootID      string   `json:"rootId,omitempty"`
	Evidence    []string `json:"evidence,omitempty"`
	Remediation string   `json:"remediation,omitempty"`
}

type TimelineEvent struct {
	ID        int64     `json:"id"`
	ScanID    int64     `json:"scanId"`
	Kind      string    `json:"kind"`
	SkillName string    `json:"skillName"`
	Path      string    `json:"path,omitempty"`
	OldHash   string    `json:"oldHash,omitempty"`
	NewHash   string    `json:"newHash,omitempty"`
	At        time.Time `json:"at"`
}

type Operation struct {
	ID           string    `json:"id"`
	PlanID       string    `json:"planId"`
	Operation    string    `json:"operation"`
	Status       string    `json:"status"`
	Message      string    `json:"message,omitempty"`
	Changes      []string  `json:"changes,omitempty"`
	SnapshotPath string    `json:"snapshotPath,omitempty"`
	CreatedAt    time.Time `json:"createdAt"`
}

type SkillUpdateFile struct {
	Path            string     `json:"path"`
	Status          string     `json:"status"` // added, modified, local (only in local folder)
	Size            int64      `json:"size,omitempty"`
	LocalSize       int64      `json:"localSize,omitempty"`
	IncomingSize    int64      `json:"incomingSize,omitempty"`
	Binary          bool       `json:"binary,omitempty"`
	Additions       int        `json:"additions,omitempty"`
	Deletions       int        `json:"deletions,omitempty"`
	LocalChanged    bool       `json:"localChanged,omitempty"`
	UpstreamChanged bool       `json:"upstreamChanged,omitempty"`
	Lines           []DiffLine `json:"lines,omitempty"`
	Truncated       bool       `json:"truncated,omitempty"`
}

type DiffLine struct {
	Kind    string `json:"kind"` // context, add, del
	OldLine int    `json:"oldLine,omitempty"`
	NewLine int    `json:"newLine,omitempty"`
	Text    string `json:"text"`
}

type SkillBaseline struct {
	Path      string            `json:"path"`
	Files     map[string]string `json:"files"`
	UpdatedAt time.Time         `json:"updatedAt"`
}

type SkillUpdateDiff struct {
	SkillID     string            `json:"skillId"`
	SkillName   string            `json:"skillName"`
	Repo        string            `json:"repo"`
	Files       []SkillUpdateFile `json:"files"`
	Added       int               `json:"added"`
	Modified    int               `json:"modified"`
	LocalOnly   int               `json:"localOnly"`
	Conflicted  int               `json:"conflicted"`
	HasBaseline bool              `json:"hasBaseline"`
	ScannedAt   time.Time         `json:"scannedAt"`
}

type PlanChange struct {
	Action    string `json:"action"`
	RootID    string `json:"rootId"`
	RootLabel string `json:"rootLabel"`
	Source    string `json:"source"`
	Target    string `json:"target"`
	Exists    bool   `json:"exists"`
	Reason    string `json:"reason,omitempty"`
}

type Message struct {
	Code     string            `json:"code,omitempty"`
	Params   map[string]string `json:"params,omitempty"`
	Fallback string            `json:"fallback,omitempty"`
}

type Plan struct {
	ID               string            `json:"id"`
	Operation        string            `json:"operation"`
	Title            string            `json:"title"`
	TitleMessage     Message           `json:"titleMessage,omitempty"`
	SummaryMessage   Message           `json:"summaryMessage,omitempty"`
	WarningMessages  []Message         `json:"warningMessages,omitempty"`
	Summary          string            `json:"summary"`
	SkillID          string            `json:"skillId,omitempty"`
	SkillName        string            `json:"skillName,omitempty"`
	SkillSlug        string            `json:"skillSlug,omitempty"`
	MCPName          string            `json:"mcpName,omitempty"`
	MCPNewName       string            `json:"mcpNewName,omitempty"`
	Changes          []PlanChange      `json:"changes"`
	Impact           []string          `json:"impact"`
	Warnings         []string          `json:"warnings,omitempty"`
	Command          string            `json:"command,omitempty"`
	Args             []string          `json:"args,omitempty"`
	MCPURL           string            `json:"mcpUrl,omitempty"`
	MCPEnv           map[string]string `json:"mcpEnv,omitempty"`
	SourceURL        string            `json:"sourceUrl,omitempty"`
	SourcePath       string            `json:"sourcePath,omitempty"`
	TargetPath       string            `json:"targetPath,omitempty"`
	RequiresSnapshot bool              `json:"requiresSnapshot"`
	Executable       bool              `json:"executable"`
	CreatedAt        time.Time         `json:"createdAt"`
}

type UsageStat struct {
	Name  string `json:"name"`
	Count int    `json:"count"`
}

type IgnoredAgent struct {
	RootID string `json:"rootId,omitempty"`
	Label  string `json:"label"`
	Path   string `json:"path"`
}

type Settings struct {
	ProjectWorkspaces     []string       `json:"projectWorkspaces"`
	IgnoredAgents         []IgnoredAgent `json:"ignoredAgents"`
	ShowAgentBuiltins     bool           `json:"showAgentBuiltins"`
	UsageStatsEnabled     bool           `json:"usageStatsEnabled"`
	AutoCheckUpdates      bool           `json:"autoCheckUpdates"`
	UpdateCheckIntervalHr int            `json:"updateCheckIntervalHours"`
	SnapshotRetention     int            `json:"snapshotRetention"`
	TrashRetentionDays    int            `json:"trashRetentionDays"`
}

type MCPAgent struct {
	Label   string `json:"label"`
	Path    string `json:"path"`
	Scope   string `json:"scope,omitempty"`
	Enabled bool   `json:"enabled"`
}

type MCPServer struct {
	ID         string     `json:"id"`
	Name       string     `json:"name"`
	Transport  string     `json:"transport"`
	Command    string     `json:"command,omitempty"`
	Args       []string   `json:"args,omitempty"`
	URL        string     `json:"url,omitempty"`
	EnvKeys    []string   `json:"envKeys,omitempty"`
	HeaderKeys []string   `json:"headerKeys,omitempty"`
	Status     string     `json:"status"`
	Agents     []MCPAgent `json:"agents"`
}

type InstructionFile struct {
	ID         string    `json:"id"`
	Name       string    `json:"name"`
	Path       string    `json:"path"`
	Agent      string    `json:"agent"`
	Scope      string    `json:"scope"`
	Project    string    `json:"project,omitempty"`
	Size       int64     `json:"size"`
	ModifiedAt time.Time `json:"modifiedAt"`
	Hash       string    `json:"hash"`
	Excerpt    string    `json:"excerpt,omitempty"`
	Duplicate  bool      `json:"duplicate"`
}
