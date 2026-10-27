import { useEffect, useMemo, useState } from 'react'
import type { KeyboardEvent } from 'react'
import { ChevronRight, Command as CommandIcon, Folder, FolderLock, History as HistoryIcon, RefreshCw } from 'lucide-react'
import { Badge } from '@/components/ui/badge'
import { Sheet, SheetContent, SheetDescription, SheetHeader, SheetTitle } from '@/components/ui/sheet'
import { AgentIcon } from '../../components/AgentIcon'
import { InstructionDetailSheet } from '../../components/InstructionDetailSheet'
import { SessionDetailSheet } from '../../components/SessionDetailSheet'
import { InstructionEditorDialog } from '../../components/InstructionEditorDialog'
import { MCPIcon } from '../../components/MCPIcon'
import { AssetSearch } from '../../components/AssetSearch'
import { SkillIcon } from '../../components/SkillIcon'
import styles from '../../App.module.css'
import { t, tValue } from '../../i18n'
import { formatCompactNumber, formatRelativeTime } from '../../lib/format'
import { operationKeys } from '../history/HistoryFeed'
import projectStyles from './ProjectView.module.css'
import type { Command, InstructionFile, Issue, MCPServer, Operation, Session, Skill } from '../../types'
import { Button } from '@/components/ui/button'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'

type ProjectTab = 'sessions' | 'resources' | 'activity'
type ResourceFilter = 'all' | 'skills' | 'mcp' | 'instructions' | 'commands'

function activateRow(event: KeyboardEvent<HTMLTableRowElement>, action: () => void) {
  if (event.key !== 'Enter' && event.key !== ' ') return
  event.preventDefault()
  action()
}

function ProjectView({ path, skills, servers, instructions, commands, operations, sessions, sessionsLoading, sessionsError, onRetrySessions, projects, issues, onRefresh, onOpenCommands }: { path: string; skills: Skill[]; servers: MCPServer[]; instructions: InstructionFile[]; commands: Command[]; operations: Operation[]; sessions: Session[]; sessionsLoading: boolean; sessionsError?: boolean; onRetrySessions?: () => void; projects: string[]; issues?: Issue[]; onRefresh: () => Promise<void>; onOpenCommands: () => void }) {
  const projectSkills = skills.filter((skill) => skill.isProject && (skill.path === path || skill.path.startsWith(path + '/')))
  const projectMCPs = servers.filter((server) => server.agents.some((agent) => agent.scope === path))
  const projectInstructions = instructions.filter((file) => file.path.startsWith(path + '/'))
  const isProjectPath = (value: string) => value === path || value.startsWith(path + '/')
  const projectCommands = commands.filter((command) => command.scope === 'project' && (command.project === path || command.path.startsWith(path + '/') || command.rootPath.startsWith(path + '/')))
  const projectOperations = operations.filter((operation) => (operation.changes ?? []).some((change) => isProjectPath(change.slice(change.indexOf(':') + 1))))
  const [selected, setSelected] = useState<{ kind: 'skill'; item: Skill } | { kind: 'mcp'; item: MCPServer } | { kind: 'instruction'; item: InstructionFile }>()
  const [editingInstruction, setEditingInstruction] = useState<InstructionFile>()
  const [selectedSession, setSelectedSession] = useState<Session>()
  const [tab, setTab] = useState<ProjectTab>('sessions')
  const [sessionQuery, setSessionQuery] = useState('')
  const [sessionRange, setSessionRange] = useState<'all' | 'today' | '7d' | '30d'>('all')
  const [sessionLimit, setSessionLimit] = useState(20)
  const [resourceQuery, setResourceQuery] = useState('')
  const [resourceFilter, setResourceFilter] = useState<ResourceFilter>('all')
  const filteredSessions = useMemo(() => {
    const now = Date.now()
    const cutoff = sessionRange === 'today' ? new Date(new Date().setHours(0, 0, 0, 0)).getTime() : sessionRange === '7d' ? now - 7 * 86400000 : sessionRange === '30d' ? now - 30 * 86400000 : 0
    const needle = sessionQuery.trim().toLowerCase()
    return sessions.filter((session) => {
      if (cutoff && Date.parse(session.lastAt) < cutoff) return false
      if (!needle) return true
      return [session.title, session.id, session.agent, session.model].some((value) => (value ?? '').toLowerCase().includes(needle))
    })
  }, [sessions, sessionQuery, sessionRange])
  const visibleSessions = filteredSessions.slice(0, sessionLimit)
  const sessionCountLabel = sessionsLoading ? '…' : sessions.length >= 50 ? '50+' : String(sessions.length)
  useEffect(() => setSessionLimit(20), [sessionQuery, sessionRange])
  const resourceNeedle = resourceQuery.trim().toLowerCase()
  const matchesResource = (values: (string | undefined)[]) => !resourceNeedle || values.some((value) => (value ?? '').toLowerCase().includes(resourceNeedle))
  const filteredSkills = (resourceFilter === 'all' || resourceFilter === 'skills') ? projectSkills.filter((skill) => matchesResource([skill.name, skill.source])) : []
  const filteredMCPs = (resourceFilter === 'all' || resourceFilter === 'mcp') ? projectMCPs.filter((server) => matchesResource([server.name, server.transport])) : []
  const filteredInstructions = (resourceFilter === 'all' || resourceFilter === 'instructions') ? projectInstructions.filter((file) => matchesResource([file.name, file.agent, file.path])) : []
  const filteredCommands = (resourceFilter === 'all' || resourceFilter === 'commands') ? projectCommands.filter((command) => matchesResource([command.name, command.agent, command.path])) : []
  const visibleResourceCount = filteredSkills.length + filteredMCPs.length + filteredInstructions.length + filteredCommands.length
  const resourceCount = (filter: ResourceFilter) => filter === 'skills' ? projectSkills.length : filter === 'mcp' ? projectMCPs.length : filter === 'instructions' ? projectInstructions.length : filter === 'commands' ? projectCommands.length : projectSkills.length + projectMCPs.length + projectInstructions.length + projectCommands.length
  const hasResources = projectSkills.length + projectMCPs.length + projectInstructions.length + projectCommands.length > 0
  const hasAny = hasResources || sessions.length > 0 || sessionsLoading || projectOperations.length > 0
  const accessIssue = (issues ?? []).find((issue) => Boolean(issue.path) && (issue.path === path || issue.path!.startsWith(path + '/')) && ['permission', 'scan_unavailable', 'scan_failure'].includes(issue.kind))
  const tabOrder: ProjectTab[] = ['sessions', 'resources', 'activity']
  const activateTab = (next: ProjectTab) => {
    setTab(next)
    requestAnimationFrame(() => document.getElementById(`project-tab-${next}`)?.focus())
  }
  const tabKeyDown = (event: KeyboardEvent<HTMLButtonElement>, current: ProjectTab) => {
    const index = tabOrder.indexOf(current)
    if (event.key !== 'ArrowRight' && event.key !== 'ArrowLeft' && event.key !== 'Home' && event.key !== 'End') return
    event.preventDefault()
    const next = event.key === 'Home' ? tabOrder[0] : event.key === 'End' ? tabOrder[tabOrder.length - 1] : tabOrder[(index + (event.key === 'ArrowRight' ? 1 : -1) + tabOrder.length) % tabOrder.length]
    activateTab(next)
  }
  const resourceFilters: { value: ResourceFilter; label: string }[] = [
    { value: 'all', label: t('common.all') },
    { value: 'skills', label: t('common.skills') },
    { value: 'mcp', label: 'MCP' },
    { value: 'instructions', label: t('common.instructions') },
    { value: 'commands', label: t('common.commands') },
  ]
  return <section className={styles.ledgerPanel}>
    <div className={projectStyles.pathBar}>
      <Folder size={14} aria-hidden="true" />
      <code title={path}>{path}</code>
      <div className={projectStyles.pathStats}>
        <span><b>{projectSkills.length}</b>{t('common.skills')}</span>
        <span><b>{projectMCPs.length}</b>MCP</span>
        <span><b>{projectInstructions.length}</b>{t('common.instructions')}</span>
        <span><b>{projectCommands.length}</b>{t('common.commands')}</span>
        <span><b>{sessionCountLabel}</b>{t('common.sessions')}</span>
      </div>
      <Button size="xs" variant="ghost" onClick={() => void onRefresh()}><RefreshCw size={12} />{t('common.rescan')}</Button>
    </div>
    {!hasAny && accessIssue ? <div className={`${projectStyles.emptyState} ${projectStyles.accessState}`}><FolderLock size={20} aria-hidden="true" /><p>{accessIssue.kind === 'permission' ? t('project.cannotAccessProjectFolder') : t('project.cannotReadProjectFolder')}</p><span>{t('project.grantFolderAccessThenRescan')}</span><code>{accessIssue.path ?? path}</code><Button size="sm" variant="outline" onClick={() => void onRefresh()}>{t('common.rescan')}</Button></div> : !hasAny ? <div className={projectStyles.emptyState}><p>{t('project.thisProjectHasNoProjectLevelResourcesYet')}</p><span>{t('project.skillsMcpConfigsAndInstructionFilesAddedToTheProjectWillBeSummarizedHere')}</span></div> : <>
      <div className={projectStyles.tabs} role="tablist" aria-label={t('project.projectTabs')}>
        <button type="button" role="tab" id="project-tab-sessions" aria-controls="project-panel-sessions" aria-selected={tab === 'sessions'} tabIndex={tab === 'sessions' ? 0 : -1} className={tab === 'sessions' ? projectStyles.tabActive : projectStyles.tab} onClick={() => setTab('sessions')} onKeyDown={(event) => tabKeyDown(event, 'sessions')}>{t('common.sessions')}<b>{sessionCountLabel}</b></button>
        <button type="button" role="tab" id="project-tab-resources" aria-controls="project-panel-resources" aria-selected={tab === 'resources'} tabIndex={tab === 'resources' ? 0 : -1} className={tab === 'resources' ? projectStyles.tabActive : projectStyles.tab} onClick={() => setTab('resources')} onKeyDown={(event) => tabKeyDown(event, 'resources')}>{t('project.resources')}<b>{resourceCount('all')}</b></button>
        <button type="button" role="tab" id="project-tab-activity" aria-controls="project-panel-activity" aria-selected={tab === 'activity'} tabIndex={tab === 'activity' ? 0 : -1} className={tab === 'activity' ? projectStyles.tabActive : projectStyles.tab} onClick={() => setTab('activity')} onKeyDown={(event) => tabKeyDown(event, 'activity')}>{t('project.recentActivity')}<b>{projectOperations.length}</b></button>
      </div>
      {tab === 'sessions' && <div role="tabpanel" id="project-panel-sessions" aria-labelledby="project-tab-sessions">
        <div className={`${styles.filterBar} ${projectStyles.filters}`}>
          <div className={styles.filterLeft}>
            <AssetSearch value={sessionQuery} onChange={setSessionQuery} placeholder={t('sessions.searchPlaceholder')} label={t('sessions.searchPlaceholder')} />
            <Select value={sessionRange} onValueChange={(value) => setSessionRange(value as 'all' | 'today' | '7d' | '30d')}><SelectTrigger size="sm" className="w-[125px]"><SelectValue /></SelectTrigger><SelectContent><SelectItem value="all">{t('sessions.allTime')}</SelectItem><SelectItem value="today">{t('sessions.today')}</SelectItem><SelectItem value="7d">{t('sessions.last7Days')}</SelectItem><SelectItem value="30d">{t('sessions.last30Days')}</SelectItem></SelectContent></Select>
          </div>
          <div className={styles.filterRight}><span className={styles.resultCount}>{sessionsLoading ? '…' : t('sessions.resultCount', { shown: visibleSessions.length, total: filteredSessions.length })}</span></div>
        </div>
        {sessionsError ? <div className={projectStyles.tableEmpty} role="alert"><span><strong>{t('sessions.loadFailed')}</strong><small>{t('sessions.loadFailedHint')}</small></span>{onRetrySessions && <Button size="sm" variant="outline" onClick={onRetrySessions}>{t('common.retry')}</Button>}</div> : sessionsLoading ? <div className={styles.tableWrap}><table className={`${projectStyles.table} ${projectStyles.sessionTable}`}><thead><tr><th>{t('sessions.title')}</th><th>{t('common.agent')}</th><th>{t('sessions.messages')}</th><th>{t('sessions.tokens')}</th><th>{t('sessions.lastActive')}</th></tr></thead><tbody>{Array.from({ length: 7 }).map((_, index) => <tr key={index} className={projectStyles.skeletonRow}><td><span className={projectStyles.skeletonLine} style={{ width: `${52 + (index % 3) * 9}%` }} /><span className={projectStyles.skeletonLineSmall} style={{ width: `${22 + (index % 2) * 8}%` }} /></td><td><span className={projectStyles.skeletonLineSmall} style={{ width: '72%' }} /></td><td><span className={projectStyles.skeletonLineSmall} style={{ width: '54%' }} /></td><td><span className={projectStyles.skeletonLineSmall} style={{ width: '46%' }} /></td><td><span className={projectStyles.skeletonLineSmall} style={{ width: '64%' }} /></td></tr>)}</tbody></table></div> : filteredSessions.length === 0 ? <div className={projectStyles.tableEmpty}><span>{sessions.length === 0 ? <><strong>{t('agent.noSessionsTitle')}</strong><small>{t('agent.noSessionsHint')}</small></> : t('sessions.noMatchingSessions')}</span></div> : <>
          <div className={styles.tableWrap}><table className={`${projectStyles.table} ${projectStyles.sessionTable}`}><thead><tr><th>{t('sessions.title')}</th><th>{t('common.agent')}</th><th>{t('sessions.messages')}</th><th>{t('sessions.tokens')}</th><th>{t('sessions.lastActive')}</th></tr></thead><tbody>
            {visibleSessions.map((session) => <tr key={`${session.agent}:${session.id}`} tabIndex={0} onClick={() => setSelectedSession(session)} onKeyDown={(event) => activateRow(event, () => setSelectedSession(session))}><td><span className={projectStyles.sessionTitle}>{session.title || session.id}</span><small>{session.model || session.path}</small></td><td><span className={projectStyles.agentCell}><AgentIcon name={session.agent} className="size-4" />{session.agent}</span></td><td>{t('project.sessionMessages', { n: formatCompactNumber(session.messages) })}</td><td>{session.tokensIn + session.tokensOut > 0 ? formatCompactNumber(session.tokensIn + session.tokensOut) : '—'}</td><td>{formatRelativeTime(session.lastAt)}</td></tr>)}
          </tbody></table></div>
          {filteredSessions.length > visibleSessions.length && <div className={projectStyles.tableMore}><Button variant="outline" size="sm" onClick={() => setSessionLimit((value) => value + 20)}>{t('sessions.loadMore', { n: Math.min(20, filteredSessions.length - visibleSessions.length) })}</Button></div>}
        </>}
      </div>}
      {tab === 'resources' && <div role="tabpanel" id="project-panel-resources" aria-labelledby="project-tab-resources">
        <div className={`${styles.filterBar} ${projectStyles.filters}`}>
          <div className={styles.filterLeft}>
            <AssetSearch value={resourceQuery} onChange={setResourceQuery} placeholder={t('project.searchResources')} label={t('project.searchResources')} />
            <Select value={resourceFilter} onValueChange={(value) => setResourceFilter(value as ResourceFilter)}><SelectTrigger size="sm" className="w-[140px]"><SelectValue /></SelectTrigger><SelectContent>{resourceFilters.map((item) => <SelectItem key={item.value} value={item.value}>{item.label} ({resourceCount(item.value)})</SelectItem>)}</SelectContent></Select>
          </div>
          <div className={styles.filterRight}><span className={styles.resultCount}>{visibleResourceCount}</span></div>
        </div>
        {visibleResourceCount === 0 ? <div className={projectStyles.tableEmpty}>{hasResources ? t('sessions.noMatchingSessions') : t('project.noResourcesInThisCategory')}</div> : <div className={styles.tableWrap}><table className={`${projectStyles.table} ${projectStyles.resourceTable}`}><thead><tr><th>{t('project.resourceType')}</th><th>{t('common.name')}</th><th>{t('common.sourceLabel')}</th><th /></tr></thead><tbody>
          {filteredSkills.map((skill) => <tr key={skill.id} tabIndex={0} onClick={() => setSelected({ kind: 'skill', item: skill })} onKeyDown={(event) => activateRow(event, () => setSelected({ kind: 'skill', item: skill }))}><td><span className={projectStyles.typeChip}>{t('common.skills')}</span></td><td><span className={projectStyles.resourceName}><SkillIcon skill={skill} className="size-4" />{skill.name}</span></td><td>{skill.source || t('common.local')}</td><td><ChevronRight size={12} className={projectStyles.rowEnd} /></td></tr>)}
          {filteredMCPs.map((server) => <tr key={server.id} tabIndex={0} onClick={() => setSelected({ kind: 'mcp', item: server })} onKeyDown={(event) => activateRow(event, () => setSelected({ kind: 'mcp', item: server }))}><td><span className={projectStyles.typeChip}>MCP</span></td><td><span className={projectStyles.resourceName}><MCPIcon name={server.name} command={server.command} url={server.url} className="size-4" />{server.name}</span></td><td>{server.transport} · {t('common.agents', { n: server.agents.length })}</td><td><ChevronRight size={12} className={projectStyles.rowEnd} /></td></tr>)}
          {filteredInstructions.map((file) => <tr key={file.id} tabIndex={0} onClick={() => setSelected({ kind: 'instruction', item: file })} onKeyDown={(event) => activateRow(event, () => setSelected({ kind: 'instruction', item: file }))}><td><span className={projectStyles.typeChip}>{t('common.instructions')}</span></td><td><span className={projectStyles.resourceName}><AgentIcon name={file.agent} className="size-4" />{file.name}</span></td><td>{tValue(file.agent)} · {file.path.startsWith(path + '/') ? file.path.slice(path.length + 1) : file.path}</td><td><ChevronRight size={12} className={projectStyles.rowEnd} /></td></tr>)}
          {filteredCommands.map((command) => <tr key={command.id} tabIndex={0} onClick={onOpenCommands} onKeyDown={(event) => activateRow(event, onOpenCommands)}><td><span className={projectStyles.typeChip}>{t('common.commands')}</span></td><td><span className={projectStyles.resourceName}><CommandIcon size={14} className={projectStyles.rowIcon} />{command.name}</span></td><td>{tValue(command.agent)} · {command.path.slice(path.length + 1)}</td><td><ChevronRight size={12} className={projectStyles.rowEnd} /></td></tr>)}
        </tbody></table></div>}
      </div>}
      {tab === 'activity' && <div role="tabpanel" id="project-panel-activity" aria-labelledby="project-tab-activity">
        {projectOperations.length === 0 ? <div className={projectStyles.tableEmpty}>{t('project.noProjectActivity')}</div> : <div className={styles.tableWrap}><table className={`${projectStyles.table} ${projectStyles.activityTable}`}><thead><tr><th>{t('project.operation')}</th><th>{t('common.path')}</th><th>{t('sessions.lastActive')}</th></tr></thead><tbody>
          {projectOperations.map((operation) => <tr key={operation.id}><td><span className={projectStyles.activityName}><HistoryIcon size={13} />{t(operationKeys[operation.operation] ?? operation.operation)}</span></td><td><code className={projectStyles.activityPath}>{(operation.changes ?? []).map((change) => change.slice(change.indexOf(':') + 1)).find(isProjectPath)}</code></td><td>{formatRelativeTime(operation.createdAt)}</td></tr>)}
        </tbody></table></div>}
      </div>}
    </>}
    {selectedSession && <SessionDetailSheet session={selectedSession} onClose={() => setSelectedSession(undefined)} />}
    {selected?.kind === 'instruction' && <InstructionDetailSheet file={selected.item} onClose={() => setSelected(undefined)} onEdit={() => setEditingInstruction(selected.item)} />}
    {editingInstruction && <InstructionEditorDialog file={editingInstruction} projects={projects} onClose={() => setEditingInstruction(undefined)} onSaved={() => { setSelected(undefined); void onRefresh() }} />}
    {selected && selected.kind !== 'instruction' && <ProjectAssetSheet selected={selected} onClose={() => setSelected(undefined)} />}
  </section>
}

function ProjectAssetSheet({ selected, onClose }: { selected: { kind: 'skill'; item: Skill } | { kind: 'mcp'; item: MCPServer }; onClose: () => void }) {
  const item = selected.item
  return <Sheet open onOpenChange={(open) => { if (!open) onClose() }}>
    <SheetContent side="right" className="w-[440px] sm:max-w-[440px]">
      <SheetHeader className="gap-0 border-b px-5 py-4"><div className="flex items-start gap-3">{selected.kind === 'skill' ? <SkillIcon skill={item as Skill} className="size-9" /> : <span className={styles.mcpHero}><MCPIcon name={item.name} command={(item as MCPServer).command} url={(item as MCPServer).url} className="size-5" /></span>}<div className="min-w-0 flex-1"><SheetTitle className="truncate text-[15px]">{item.name}</SheetTitle><SheetDescription className="mt-1 flex flex-wrap items-center gap-x-1.5 gap-y-0.5 text-[11px]"><span>{selected.kind === 'skill' ? t('project.projectSkill') : (item as MCPServer).transport}</span>{selected.kind === 'mcp' && <><span>·</span><span>{t('common.agents', { n: (item as MCPServer).agents.length })}</span></>}</SheetDescription></div></div></SheetHeader>
      <div className="min-h-0 flex-1 overflow-y-auto p-0">
        {selected.kind === 'skill' ? <SkillSections skill={item as Skill} /> : <MCPSections mcp={item as MCPServer} />}
      </div>
    </SheetContent>
  </Sheet>
}

function SkillSections({ skill }: { skill: Skill }) {
  return <>
    <section className={styles.detailSection}><h3>{t('common.sourceLabel')}</h3><dl className={styles.definition}><dt>{t('common.path')}</dt><dd><code className={styles.detailPath}>{skill.path}</code></dd><dt>{t('common.sourceLabel')}</dt><dd>{skill.source || t('common.local')}</dd><dt>{t('skills.descriptionTokens')}</dt><dd>{skill.descTokens.toLocaleString()} tokens</dd><dt>{t('common.contentHash')}</dt><dd><code>{skill.folderHash?.slice(0, 16) || '—'}</code></dd></dl></section>
    {skill.description && <section className={styles.detailSection}><h3>{t('project.description')}</h3><p className={styles.assetDescription}>{skill.description}</p></section>}
    {skill.bins && skill.bins.length > 0 && <section className={styles.detailSection}><h3>{t('common.cliDependencies')}</h3><div className={styles.mcpKeys}>{skill.bins.map((bin) => <Badge key={bin} variant="outline">{bin}</Badge>)}</div></section>}
  </>
}

function MCPSections({ mcp }: { mcp: MCPServer }) {
  return <>
    <section className={styles.detailSection}><h3>{t('common.launch')}</h3><dl className={styles.definition}>{mcp.command && <><dt>{t('common.commandLabel')}</dt><dd><code>{mcp.command}</code></dd></>}{mcp.args && mcp.args.length > 0 && <><dt>{t('common.arguments')}</dt><dd><code>{mcp.args.join(' ')}</code></dd></>}{mcp.url && <><dt>URL</dt><dd><code>{mcp.url}</code></dd></>}<dt>{t('common.transport')}</dt><dd>{mcp.transport}</dd></dl></section>
    <section className={styles.detailSection}><h3>{t('common.agentConfiguration')}</h3><div className={styles.mcpAgents}>{mcp.agents.map((agent) => <div key={agent.path} className={styles.mcpAgentRow}><AgentIcon name={agent.label} className="size-4" /><div><strong>{tValue(agent.label)}</strong><small>{agent.path}</small></div><Badge variant={agent.enabled ? 'success' : 'outline'}>{agent.enabled ? t('common.enabled') : t('common.disabled')}</Badge></div>)}</div></section>
    {(mcp.envKeys?.length || mcp.headerKeys?.length) ? <section className={styles.detailSection}><h3>{t('common.sensitiveFields')}</h3><div className={styles.mcpKeys}>{(mcp.envKeys ?? []).map((key) => <Badge key={key} variant="outline">{key}</Badge>)}{(mcp.headerKeys ?? []).map((key) => <Badge key={key} variant="outline">{key}</Badge>)}</div></section> : null}
  </>
}

export { ProjectView }
