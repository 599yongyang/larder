export type View = 'skills' | 'cli' | 'mcp' | 'instructions' | 'commands' | 'project' | 'history' | 'settings' | 'xray' | 'agent-sessions'
export type LinkMode = 'linked' | 'copied' | 'local' | 'missing' | 'broken' | 'unavailable' | 'shared'
export type Root = { id: string; label: string; logicalAgent?: string; path: string; kind: string; known: boolean; status: string; error?: string; entryCount: number; sharedSkills?: boolean }
export type Skill = { id: string; name: string; path: string; rootId: string; rootKind: string; source?: string; sourceUrl?: string; skillPath?: string; locked: boolean; hasSkillMd: boolean; description?: string; descTokens: number; folderHash?: string; contentHash?: string; scanState: string; isAgentLocal: boolean; isProject: boolean; copyDrift: boolean; icon?: string; iconRoot?: string; iconSource?: string; bins?: string[]; origin?: string; originDetail?: string }
export type Link = { skillId: string; rootId: string; mode: LinkMode; path?: string; targetPath?: string; drift: boolean; visible: boolean; reason?: string }
export type Issue = { kind: string; path?: string; message: string }
export type Snapshot = { schemaVersion: string; startedAt?: string; finishedAt?: string; status: string; roots: Root[]; skills: Skill[]; agentLinks: Link[]; clis: CLIInfo[]; issues: Issue[]; fromCache: boolean }
export type Finding = { id: string; rule: string; severity: 'info' | 'warn' | 'error'; title: string; message: string; path?: string; skillId?: string; cliId?: string; rootId?: string; evidence?: string[]; remediation?: string }
export type BackupInfo = { path: string; createdAt: string; total: number; configs: number; instructions: number; apps: number; projects: string[] }
export type BackupResult = { restored: number; skipped: number; snapshotDir: string }
export type AppInfo = { version: string; schemaVersion: string; goVersion: string; platform: string; dataDir: string; homeDir: string }
export type InstructionFile = { id: string; name: string; path: string; agent: string; scope: 'global' | 'project' | string; project?: string; size: number; modifiedAt: string; hash: string; excerpt?: string; duplicate: boolean }
export type MCPAgent = { label: string; path: string; scope?: string; enabled: boolean }
export type MCPServer = { id: string; name: string; transport: string; command?: string; args?: string[]; url?: string; envKeys?: string[]; headerKeys?: string[]; status: string; agents: MCPAgent[] }
export type CLIPath = { path: string; owner: string; ownerDetail?: string; broken: boolean; effective: boolean }
export type CLIInfo = { id: string; name: string; path: string; owner: string; ownerDetail?: string; version?: string; status: string; duplicateCount: number; paths?: CLIPath[]; scannedAt: string }
export type TimelineEvent = { id: number; scanId: number; kind: 'added' | 'changed' | 'removed' | string; skillName: string; path?: string; oldHash?: string; newHash?: string; at: string }
export type Operation = { id: string; planId: string; operation: string; status: string; message?: string; changes?: string[]; snapshotPath?: string; createdAt: string }
export type PlanChange = { action: string; rootId: string; rootLabel: string; source: string; target: string; exists: boolean; reason?: string }
export type DiffLine = { kind: 'context' | 'add' | 'del'; oldLine?: number; newLine?: number; text: string }
export type SkillUpdateFile = { path: string; status: 'added' | 'modified' | 'local' | 'conflict' | 'local_modified' | 'removed'; size?: number; localSize?: number; incomingSize?: number; binary?: boolean; additions?: number; deletions?: number; localChanged?: boolean; upstreamChanged?: boolean; lines?: DiffLine[]; truncated?: boolean }
export type SkillUpdateDiff = { skillId: string; skillName: string; repo: string; files: SkillUpdateFile[]; added: number; modified: number; localOnly: number; conflicted: number; hasBaseline: boolean; scannedAt: string }
export type Message = { code: string; params?: Record<string, string>; fallback?: string }
export type Plan = { id: string; operation: string; title: string; titleMessage?: Message; summaryMessage?: Message; warningMessages?: Message[]; summary: string; skillId?: string; skillName?: string; skillSlug?: string; mcpName?: string; targetPath?: string; changes: PlanChange[]; impact: string[]; command?: string; args?: string[]; warnings?: string[]; requiresSnapshot: boolean; executable: boolean; createdAt: string }
export type UpdateItem = { kind: 'cli' | 'skill'; cliId?: string; owner: string; package?: string; name: string; description?: string; commands?: string[]; currentVersion?: string; latestVersion?: string; source?: string; state: 'available' | 'current' | 'changed' | 'removed' | 'new' | 'unknown' | 'error'; skillPath?: string; copies?: number; message?: string }
export type UpdateReport = { checkedAt: string; items: UpdateItem[]; errors?: string[]; scanned: number; available: number; current: number; skillScanned: number; skillAdded?: number; skillChanged: number; skillRemoved?: number; skillCurrent: number; skillErrors: number }
export type IgnoredAgent = { rootId?: string; label: string; path: string }
export type Settings = { projectWorkspaces: string[]; ignoredAgents?: IgnoredAgent[]; showAgentBuiltins: boolean; usageStatsEnabled: boolean; autoCheckUpdates: boolean; updateCheckIntervalHours: number; snapshotRetention: number; trashRetentionDays?: number; globalShortcutEnabled?: boolean; globalShortcut?: string }
export type UsageStat = { name: string; count: number }

export const emptySnapshot: Snapshot = { schemaVersion: '1', status: 'idle', roots: [], skills: [], agentLinks: [], clis: [], issues: [], fromCache: false }

export type Command = { id: string; name: string; description?: string; argumentHint?: string; allowedTools?: string[]; model?: string; agent: string; rootPath: string; path: string; format?: string; scope: 'global' | 'project' | string; project?: string; size: number; modifiedAt: string; hash: string }

export type CommandTarget = { agent: string; scope: 'global' | 'project' | string; project?: string; rootPath: string; format: string }
export type CommandWriteRequest = { path?: string; agent?: string; scope?: string; project?: string; rootPath?: string; name?: string; content?: string; sourcePath?: string }
export type CommandWritePlan = { id: string; operation: string; target: CommandTarget; name: string; path: string; content: string; currentContent?: string; exists: boolean; warnings?: string[]; createdAt: string; targetDeprecated?: boolean }

export type SkillAuditEntry = { provider: string; slug?: string; status: string; summary?: string; riskLevel?: string; auditedAt?: string; categories?: string[] }
export type SkillAudit = { skillId: string; source?: string; skill?: string; available: boolean; verdict: 'pass' | 'warn' | 'fail' | 'unreviewed' | string; riskLevel?: string; entries?: SkillAuditEntry[]; checkedAt?: string; reason?: string; localHash?: string; localChanged?: boolean; localBaselineAt?: string }

export type SkillEditPlan = { id: string; skillId: string; path: string; content: string; currentContent?: string; exists: boolean; warnings?: string[]; createdAt: string }

export type SkillCreatePlan = { id: string; name: string; path: string; content: string; warnings?: string[]; createdAt: string }
export type SkillRenamePlan = { id: string; skillId: string; name: string; from: string; to: string; createdAt: string }

export type CommandDeletePlan = { id: string; target: CommandTarget; name: string; path: string; currentContent?: string; createdAt: string }

export type NativeCLIUpdateInfo = { cliId: string; current?: string; latest?: string; available: boolean; canUpdate: boolean; updateCommand?: string; error?: string; checkedAt: string }

export type Session = { id: string; agent: string; cwd: string; title?: string; model?: string; path: string; startedAt: string; lastAt: string; messages: number; tokensIn: number; tokensOut: number }

export type SessionMessage = { role: string; text: string; at: string }

export type ResourceUsage = { kind: string; name: string; count: number; lastAt: string }

export type AppUpdateInfo = { available: boolean; version: string; name: string; notes: string; publishedAt: string; size: number }

export type DataPackageSelection = { settings: boolean; appState: boolean; mcp: boolean; instructions: boolean; commands: boolean; skills: boolean }
export type DataPackageInfo = { createdAt: string; selection: DataPackageSelection; total: number; categories: Record<string, number>; settings?: Settings }
export type DataPackageResult = { restored: number; skipped: number; snapshotDir: string }

export type SyncConfig = { provider: string; url: string; username: string; endpoint: string; region: string; bucket: string; prefix: string; accessKey: string; filename: string; passwordSet: boolean; lastSyncAt: string; lastSyncKind: string; encryptionEnabled: boolean; encryptionPassphraseSet: boolean }
