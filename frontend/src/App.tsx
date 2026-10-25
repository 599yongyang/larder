import { useCallback, useEffect, useRef, useState } from 'react'
import { Events } from '@wailsio/runtime'
import { Monitor, Moon, RefreshCw, Search, Sun } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { CommandPalette } from './components/CommandPalette'
import { Kbd } from '@/components/ui/kbd'
import { GooeyToaster, gooeyToast } from 'goey-toast'
import { toastError, toastSuccess } from './lib/toast'
import { AgentManagerDialog } from './components/AgentManagerDialog'
import { InitialSkeleton } from './components/InitialSkeleton'

import { LarderState } from '../bindings/events'
import { AppSidebar } from './components/AppSidebar'
import { SidebarInset, SidebarProvider } from '@/components/ui/sidebar'
import { CLIView } from './features/cli/CLIView'
import { MCPView } from './features/mcp/MCPView'
import { ProjectManagerDialog } from './components/ProjectManagerDialog'
import { HistoryView } from './features/history/HistoryView'
import { SkillsView } from './features/ledger/LedgerView'
import { SettingsView } from './features/settings/SettingsView'
import { InstructionsView } from './features/instructions/InstructionsView'
import { ProjectView } from './features/project/ProjectView'
import { formatRelativeTime, skillSourceLabel } from './lib/format'
import { emptySnapshot } from './types'
import type { AppInfo, InstructionFile, Issue, Operation, TimelineEvent, UpdateReport, UsageStat, View } from './types'
import styles from './App.module.css'
import { backend } from './backend'
import { useAppStore } from './store'
import { applyTheme, getThemeMode, nextThemeMode, resolveTheme, systemTheme } from './lib/theme'
import type { ThemeMode } from './lib/theme'
import { switchTheme } from './lib/themeTransition'
import { pageTransition } from './lib/pageTransition'
import { t } from './i18n'
import { backendErrorMessage } from './lib/backendError'
import { cycleZoom, resetZoom } from './lib/zoom'
import { ariaShortcut, shortcutLabel } from './lib/platform'

function App() {
  const [view, setView] = useState<View>(() => {
    try {
      const saved = localStorage.getItem('larder.view')
      return saved === 'skills' || saved === 'cli' || saved === 'mcp' || saved === 'instructions' || saved === 'history' || saved === 'settings' ? saved : 'skills'
    } catch { return 'skills' }
  })
  const { state, settings, setState, setSettings } = useAppStore()
  const [selectedId, setSelectedId] = useState<string>()
  const [commandOpen, setCommandOpen] = useState(false)
  const [agentManagerOpen, setAgentManagerOpen] = useState(false)
  const [projectManagerOpen, setProjectManagerOpen] = useState(false)
  const [cliFocus, setCliFocus] = useState<string>()
  const [mcpFocus, setMcpFocus] = useState<string>()
  const [mcps, setMcps] = useState<any[]>([])
  const [instructions, setInstructions] = useState<InstructionFile[]>([])
  const [appInfo, setAppInfo] = useState<AppInfo>()
  const [updates, setUpdates] = useState<UpdateReport>()
  const [updatesLoading, setUpdatesLoading] = useState(false)
  const [ready, setReady] = useState(false)
  const [operations, setOperations] = useState<Operation[]>([])
  const [timeline, setTimeline] = useState<TimelineEvent[]>([])
  const [usage, setUsage] = useState<UsageStat[]>([])
  const viewBodyRef = useRef<HTMLDivElement>(null)
  const scrollSaveTimer = useRef<number>()
  const [scope, setScope] = useState<{ kind: 'library'; label: string } | { kind: 'agent'; id: string; label: string } | { kind: 'project'; path: string; label: string }>()
  const [source, setSource] = useState(() => { try { return localStorage.getItem('larder.skills.source') ?? 'all' } catch { return 'all' } })
  const [themeMode, setThemeMode] = useState<ThemeMode>(() => getThemeMode())
  const [systemDark, setSystemDark] = useState(() => systemTheme())
  const resolvedTheme = resolveTheme(themeMode, systemDark)
  const changeTheme = (mode: ThemeMode, event?: React.MouseEvent<HTMLElement>) => switchTheme(mode, event ? { x: event.clientX, y: event.clientY } : undefined, systemDark, setThemeMode)
  const goView = (next: View) => pageTransition(() => { setScope(undefined); setView(next) })
  const goScope = (next: typeof scope) => pageTransition(() => { setScope(next); setView(next?.kind === 'project' ? 'project' : 'skills'); setSource('all'); setSelectedId(undefined) })
  useEffect(() => { applyTheme(themeMode, systemDark) }, [themeMode, systemDark])
  useEffect(() => {
    if (themeMode !== 'system' || typeof window.matchMedia !== 'function') return
    const media = window.matchMedia('(prefers-color-scheme: dark)')
    const onChange = () => setSystemDark(media.matches ? 'dark' : 'light')
    onChange()
    media.addEventListener('change', onChange)
    return () => media.removeEventListener('change', onChange)
  }, [themeMode])
  const refreshInFlight = useRef<Promise<void> | null>(null)
  const refresh = useCallback(() => {
    if (refreshInFlight.current) return refreshInFlight.current
    const request = (async () => {
      const results = await Promise.allSettled([
        backend.state(),
        backend.settings(),
        backend.operations(),
        backend.timeline(),
        backend.usageStats(),
        backend.mcp(),
        backend.instructions(),
        backend.appInfo(),
      ])
      const [next, nextSettings, nextOperations, nextTimeline, nextUsage, nextMcps, nextInstructions, nextAppInfo] = results
      if (next.status === 'fulfilled') {
        setState(next.value)
        setUpdates(next.value.updates?.checkedAt ? next.value.updates : undefined)
      }
      if (nextSettings.status === 'fulfilled') setSettings(nextSettings.value)
      if (nextOperations.status === 'fulfilled') setOperations(nextOperations.value)
      if (nextTimeline.status === 'fulfilled') setTimeline(nextTimeline.value)
      if (nextUsage.status === 'fulfilled') setUsage(nextUsage.value)
      if (nextMcps.status === 'fulfilled') setMcps(nextMcps.value)
      if (nextInstructions.status === 'fulfilled') setInstructions(nextInstructions.value)
      if (nextAppInfo.status === 'fulfilled') setAppInfo(nextAppInfo.value)
      const failure = results.find((result): result is PromiseRejectedResult => result.status === 'rejected')
      if (failure) toastError(failure.reason, t('app.someDataCouldNotBeRead'))
    })()
    refreshInFlight.current = request
    void request.finally(() => {
      if (refreshInFlight.current === request) refreshInFlight.current = null
    })
    return request
  }, [])
  useEffect(() => { try { localStorage.setItem('larder.view', view) } catch {} }, [view])
  useEffect(() => { try { localStorage.setItem('larder.skills.source', source) } catch {} }, [source])
  useEffect(() => { void refresh().finally(() => setReady(true)) }, [refresh])
  const refreshVisibleView = useCallback(async () => {
    try {
      const results = await Promise.allSettled([backend.state(), backend.settings()])
      const [next, nextSettings] = results
      if (next.status === 'fulfilled') {
        setState(next.value)
        setUpdates(next.value.updates?.checkedAt ? next.value.updates : undefined)
      }
      if (nextSettings.status === 'fulfilled') setSettings(nextSettings.value)
      if (view === 'mcp' || view === 'cli' || view === 'project') {
        const result = await Promise.allSettled([backend.mcp()])
        if (result[0].status === 'fulfilled') setMcps(result[0].value)
      }
      if (view === 'instructions' || view === 'project') {
        const result = await Promise.allSettled([backend.instructions()])
        if (result[0].status === 'fulfilled') setInstructions(result[0].value)
      }
      if (view === 'history') {
        const [nextOperations, nextTimeline, nextUsage] = await Promise.allSettled([backend.operations(), backend.timeline(), backend.usageStats()])
        if (nextOperations.status === 'fulfilled') setOperations(nextOperations.value)
        if (nextTimeline.status === 'fulfilled') setTimeline(nextTimeline.value)
        if (nextUsage.status === 'fulfilled') setUsage(nextUsage.value)
      }
      if (view === 'settings') {
        const [nextUsage, nextAppInfo] = await Promise.allSettled([backend.usageStats(), backend.appInfo()])
        if (nextUsage.status === 'fulfilled') setUsage(nextUsage.value)
        if (nextAppInfo.status === 'fulfilled') setAppInfo(nextAppInfo.value)
      }
    } catch (error) {
      toastError(error, t('app.unableToRefreshThisPage'))
    }
  }, [view])
  const lastFocusRefresh = useRef(0)
  useEffect(() => {
    const onFocus = () => { const now = Date.now(); if (now - lastFocusRefresh.current < 60_000) return; lastFocusRefresh.current = now; void refreshVisibleView() }
    window.addEventListener('focus', onFocus)
    return () => window.removeEventListener('focus', onFocus)
  }, [refreshVisibleView])
  // The Go service emits larder:state on scan start/finish/failure. Refresh only
  // on that signal; idle ledgers never poll a full snapshot.
  useEffect(() => {
    const off = Events.On(LarderState, () => { void refresh() })
    return () => { off?.() }
  }, [refresh])
  useEffect(() => {
    if (!ready) return
    const element = viewBodyRef.current
    if (!element) return
    let top = 0
    try { top = Number(localStorage.getItem(`larder.scroll.${view}`) ?? 0) || 0 } catch {}
    const frame = requestAnimationFrame(() => { element.scrollTop = top })
    return () => cancelAnimationFrame(frame)
  }, [ready, view])
  const saveScroll = (event: React.UIEvent<HTMLDivElement>) => {
    const top = event.currentTarget.scrollTop
    window.clearTimeout(scrollSaveTimer.current)
    scrollSaveTimer.current = window.setTimeout(() => { try { localStorage.setItem(`larder.scroll.${view}`, String(top)) } catch {} }, 120)
  }
  const scan = async () => { setUpdates(undefined); const toast = gooeyToast(t('app.scanning'), { duration: 60000 }); try { await backend.scan(); await refresh(); gooeyToast.update(toast, { title: t('app.scanComplete'), type: 'success' }) } catch (error) { gooeyToast.update(toast, { title: t('app.scanFailed'), type: 'error', description: backendErrorMessage(error, t('app.unknownError')) }); await refresh() } }
  const snapshot = state.snapshot ?? emptySnapshot
  const partialIssues = snapshot.issues.filter((issue: Issue) => ['permission', 'scan_failure', 'lockfile_error', 'scan_unavailable'].includes(issue.kind))
  const cacheRelative = state.updatedAt ? formatRelativeTime(state.updatedAt) : ''
  useEffect(() => {
    if (state.lastError) {
      gooeyToast.dismiss('scan-partial')
      gooeyToast.error(t('app.readFailed', { message: backendErrorMessage(state.lastError, t('app.unknownError')) }), {
        id: 'scan-error',
        duration: Infinity,
        description: t('app.scanFailureDescription'),
      })
      return
    }
    gooeyToast.dismiss('scan-error')
  }, [state.lastError])

  useEffect(() => {
    if (state.lastError || !state.cached || snapshot.status === 'partial') {
      gooeyToast.dismiss('cache-stale')
      return
    }
    gooeyToast.info(t('app.cachedDataLastScanRefreshingInBackground', { time: cacheRelative || t('app.unknownTime') }), {
      id: 'cache-stale',
      duration: Infinity,
      description: t('app.cachedRefreshDescription'),
    })
  }, [state.lastError, state.cached, snapshot.status, cacheRelative])

  useEffect(() => {
    if (state.lastError || state.cached || snapshot.status !== 'partial') {
      gooeyToast.dismiss('scan-partial')
      return
    }
    gooeyToast.warning(t('app.incompleteScanFailedSomeStatesAreUnknown', { count: partialIssues.length || snapshot.issues.length }), {
      id: 'scan-partial',
      duration: Infinity,
      description: t('app.scanPartialDescription'),
    })
  }, [snapshot.status, snapshot.updatedAt, state.lastError, state.cached, partialIssues.length, snapshot.issues.length])
  const ignoredAgents = settings.ignoredAgents ?? []
  const ignoredAgentPaths = new Set(ignoredAgents.map((agent) => agent.path))
  const hasGlobalSkills = snapshot.skills.some((skill) => skill.rootKind === 'global')
  const agentRoots = snapshot.roots.filter((root) => (root.kind === 'known_agent' || root.kind === 'unknown_agent') && !ignoredAgentPaths.has(root.path) && ((root.status === 'available' && root.entryCount > 0) || (root.sharedSkills && hasGlobalSkills)))
  const clis = snapshot.clis ?? []
  const visibleClis = clis
  const ignoreAgent = async (root: { id: string; label: string; path: string }) => {
    try {
      setSettings(await backend.ignoreAgent(root.path, root.label, root.id))
      toastSuccess(t('app.agentIgnored'), root.label)
      await backend.scan()
      await refresh()
    } catch (error) {
      toastError(error, t('app.failedToIgnoreAgent'))
    }
  }
  const restoreAgent = async (path: string) => {
    try {
      setSettings(await backend.restoreAgent(path))
      toastSuccess(t('app.agentRestored'), t('app.updatingScanResults'))
      await backend.scan()
      await refresh()
    } catch (error) {
      toastError(error, t('app.failedToRestoreAgent'))
    }
  }
  const cleanupAgent = async (path: string) => {
    try {
      setSettings(await backend.trashAgent(path))
      toastSuccess(t('app.movedToTrash'), t('app.youCanRestoreItFromFinder'))
      await backend.scan()
      await refresh()
    } catch (error) {
      toastError(error, t('app.failedToCleanUpAgent'))
    }
  }
  const checkUpdates = async (notify = false) => { setUpdatesLoading(true); try { const next = await backend.checkUpdates(); setUpdates(next); if (notify) toastSuccess(t('app.updateCheckComplete'), t('app.skillsClisCanUpdate', { skills: next.skillChanged, clis: next.available })) } catch (error) { toastError(error, t('app.updateCheckFailed')) } finally { setUpdatesLoading(false) } }
  useEffect(() => {
    if (!ready || !settings.autoCheckUpdates || updatesLoading) return
    const interval = Math.max(1, settings.updateCheckIntervalHours || 24) * 60 * 60 * 1000
    const last = updates?.checkedAt ? Date.parse(updates.checkedAt) : 0
    if (last && Date.now() - last < interval) return
    void checkUpdates()
  }, [ready, settings.autoCheckUpdates, settings.updateCheckIntervalHours, updates?.checkedAt])
  useEffect(() => {
    const views: View[] = ['skills', 'mcp', 'cli', 'instructions', 'history', 'settings']
    const onKey = (event: KeyboardEvent) => {
      if (!(event.metaKey || event.ctrlKey) || event.altKey) return
      const target = event.target as HTMLElement | null
      const typing = Boolean(target?.closest('input, textarea, select, [contenteditable="true"]'))
      if (event.key.toLowerCase() === 'f') {
        const search = document.querySelector<HTMLInputElement>('[data-asset-search]')
        if (search) { event.preventDefault(); search.focus(); search.select(); return }
        if (!typing) { event.preventDefault(); setCommandOpen(true) }
        return
      }
      if (event.key === '0') { event.preventDefault(); resetZoom(); return }
      if (event.key === '=' || event.key === '+') { event.preventDefault(); cycleZoom(1); return }
      if (event.key === '-') { event.preventDefault(); cycleZoom(-1); return }
      if (!typing && event.key.toLowerCase() === 'r') { event.preventDefault(); void scan(); return }
      if (!typing && event.key === ',') { event.preventDefault(); goView('settings'); return }
      if (!typing && /^[1-6]$/.test(event.key) && !event.shiftKey) { event.preventDefault(); goView(views[Number(event.key) - 1]) }
    }
    window.addEventListener('keydown', onKey)
    return () => window.removeEventListener('keydown', onKey)
  }, [scan, goView])
  const scopedSkills = snapshot.skills.filter((skill) => {
    if (scope?.kind === 'library') return skill.rootKind === 'global'
    if (scope?.kind === 'agent') {
      const local = skill.rootId === scope.id
      const linked = snapshot.agentLinks.some((link) => link.skillId === skill.id && link.rootId === scope.id && link.visible)
      return local || linked
    }
    if (scope?.kind === 'project') return skill.isProject && (skill.path === scope.path || skill.path.startsWith(scope.path + '/'))
    return settings.showAgentBuiltins || !skill.isAgentLocal
  })
  const visibleSkills = scopedSkills.filter((skill) => source === 'all' || skillSourceLabel(skill) === source)
  const sources: string[] = Array.from(new Set<string>(visibleSkills.map(skillSourceLabel))).sort()
  const selectedSkill = selectedId ? visibleSkills.find((skill) => skill.id === selectedId) : undefined
  const agentItems = agentRoots.map((root) => {
    const ids = new Set<string>()
    for (const skill of snapshot.skills) if (skill.rootId === root.id) ids.add(skill.id)
    for (const link of snapshot.agentLinks) if (link.rootId === root.id && link.visible) ids.add(link.skillId)
    return { id: root.id, label: root.logicalAgent || root.label, count: ids.size }
  })
  const projectItems = settings.projectWorkspaces.map((workspace) => ({ path: workspace, label: workspace.split('/').filter(Boolean).pop() || workspace, count: snapshot.skills.filter((skill) => skill.isProject && (skill.path === workspace || skill.path.startsWith(workspace + '/'))).length }))
  const addProject = async (value: string) => { const path = value.trim(); if (!path) return; const next = await backend.saveSettings({ ...settings, projectWorkspaces: [...new Set([...settings.projectWorkspaces, path])] }); setSettings(next); await backend.scan(); await refresh(); toastSuccess(t('app.projectWorkspaceAdded'), t('app.scanningAgain')) }
  const removeProject = async (path: string) => { const next = await backend.saveSettings({ ...settings, projectWorkspaces: settings.projectWorkspaces.filter((item) => item !== path) }); setSettings(next); await backend.scan(); await refresh() }
  return <SidebarProvider className={styles.appShell} style={{ "--sidebar-width": "232px" } as React.CSSProperties}>
    <AppSidebar view={view} scope={scope} onView={goView} onScope={goScope} onManageAgents={() => setAgentManagerOpen(true)} skillsCount={snapshot.skills.filter((skill) => !skill.isAgentLocal).length} cliCount={clis.length} mcpCount={mcps.length}
      instructionCount={instructions.length} onManageProjects={() => setProjectManagerOpen(true)} agents={agentItems} projects={projectItems} centralCount={snapshot.skills.filter((skill) => skill.rootKind === 'global').length} />
    <SidebarInset className={styles.main}><header className={styles.topbar} title={t('app.doubleClickToMaximize')} onDoubleClick={(event) => { const target = event.target as HTMLElement; if (target.closest('button, a, input, [role="button"]')) return; event.preventDefault(); void backend.toggleWindowMaximised() }}><h1>{view === 'skills' && scope ? (scope.kind === 'library' ? t('common.sharedLibrary') : `${scope.label} · ${t('common.skills')}`) : view === 'project' && scope?.kind === 'project' ? `${scope.label} · ${t('common.projects')}` : viewTitle(view)}</h1><div className={styles.topActions}><Button variant="outline" size="sm" onClick={() => setCommandOpen(true)} className="w-[260px] justify-start gap-2 font-normal text-muted-foreground"><Search size={14} aria-hidden="true" /><span>{t('app.searchOrJump')}</span><Kbd className="ml-auto">{shortcutLabel('K')}</Kbd></Button><Button variant="outline" size="icon-sm" title={state.scanning ? t('app.cancelScan') : t('common.rescan')} aria-label={state.scanning ? t('app.cancelScan') : t('common.rescan')} aria-keyshortcuts={ariaShortcut('R')} onClick={() => state.scanning ? void backend.cancel() : void scan()}><RefreshCw size={14} aria-hidden="true" className={state.scanning ? 'animate-spin' : undefined} /></Button><Button variant="ghost" size="icon-sm" aria-label={t('app.appearanceClickToSwitch', { mode: themeMode === 'system' ? t('app.system') : themeMode === 'light' ? t('common.light') : t('common.dark') })} title={t('app.appearanceClickToSwitch', { mode: themeMode === 'system' ? t('app.system') : themeMode === 'light' ? t('common.light') : t('common.dark') })} onClick={(event) => changeTheme(nextThemeMode(themeMode), event)}>{themeMode === 'system' ? <Monitor size={15} aria-hidden="true" /> : themeMode === 'dark' ? <Moon size={15} aria-hidden="true" /> : <Sun size={15} aria-hidden="true" />}</Button></div></header>
      <div className={`${styles.scanProgress} ${state.scanning ? styles.scanProgressActive : ''}`} aria-hidden={!state.scanning} role="status" aria-live="polite" aria-label={state.scanning ? t('common.scanning') : undefined}><span /></div>
    
      <div ref={viewBodyRef} className={styles.viewBody} onScroll={saveScroll}>
        {!ready ? <InitialSkeleton /> : <>
        {view === 'skills' && <SkillsView skills={visibleSkills} allSkills={scopedSkills} roots={agentRoots} links={snapshot.agentLinks} operations={operations} selected={selectedSkill} onSelect={setSelectedId} onCloseDetail={() => setSelectedId(undefined)} source={source} sources={sources} onSource={setSource} onRefresh={refresh} issues={snapshot.issues} updates={updates} updatesLoading={updatesLoading} onCheckUpdates={() => void checkUpdates(true)} clis={clis} onOpenCLI={(id) => pageTransition(() => { setScope(undefined); setCliFocus(id); setView('cli') })} />}
        {view === 'mcp' && <MCPView servers={mcps} clis={clis} onRefresh={refresh} focusMCP={mcpFocus} onFocusConsumed={() => setMcpFocus(undefined)} onOpenCLI={(id) => pageTransition(() => { setScope(undefined); setCliFocus(id); setView('cli') })} />}
        {view === 'instructions' && <InstructionsView files={instructions} projects={settings.projectWorkspaces} onRefresh={refresh} />}
        {view === 'project' && scope?.kind === 'project' && <ProjectView path={scope.path} skills={snapshot.skills} servers={mcps} instructions={instructions} projects={settings.projectWorkspaces} issues={snapshot.issues} onRefresh={refresh} />}
        {view === 'cli' && <CLIView clis={visibleClis} skills={snapshot.skills} mcps={mcps} onOpenSkill={(id) => pageTransition(() => { setScope(undefined); setView('skills'); setSelectedId(id) })} onOpenMCP={() => pageTransition(() => { setScope(undefined); setView('mcp') })} onRefresh={refresh} updates={updates} updatesLoading={updatesLoading} onCheckUpdates={() => void checkUpdates(true)} focusCLI={cliFocus} onFocusConsumed={() => setCliFocus(undefined)} />}
        {view === 'history' && <HistoryView events={timeline} usage={usage} operations={operations} onRefresh={refresh} onRollback={async (id) => { await backend.rollbackOperation(id); await refresh() }} />}
        {view === 'settings' && <SettingsView settings={settings} onToggle={(value) => void backend.saveSettings({ ...settings, showAgentBuiltins: value }).then(setSettings).catch((error) => toastError(error, t('common.failedToSave')))} onUsageToggle={(value) => void backend.saveSettings({ ...settings, usageStatsEnabled: value }).then(setSettings).catch((error) => toastError(error, t('common.failedToSave')))} onAutoCheckToggle={(value) => void backend.saveSettings({ ...settings, autoCheckUpdates: value }).then(setSettings).catch((error) => toastError(error, t('common.failedToSave')))} onInterval={(value) => void backend.saveSettings({ ...settings, updateCheckIntervalHours: value }).then(setSettings).catch((error) => toastError(error, t('common.failedToSave')))} onRetention={(value) => void backend.saveSettings({ ...settings, snapshotRetention: value }).then(setSettings).catch((error) => toastError(error, t('common.failedToSave')))} themeMode={themeMode} onThemeMode={changeTheme} info={appInfo} onBackup={() => void backend.exportBackup().then((path) => toastSuccess(t('app.backupExported'), path)).catch((error) => toastError(error, t('app.failedToExportBackup')))} onRestored={() => void refresh()} />}
        </>}
      </div>
      {projectManagerOpen && <ProjectManagerDialog projects={projectItems} onClose={() => setProjectManagerOpen(false)} onAdd={addProject} onRemove={removeProject} />}
      {agentManagerOpen && <AgentManagerDialog
        agents={snapshot.roots.filter((root) => (root.kind === 'known_agent' || root.kind === 'unknown_agent') && !ignoredAgentPaths.has(root.path) && ((root.status === 'available') || (root.sharedSkills && hasGlobalSkills)))}
        ignored={ignoredAgents}
        onClose={() => setAgentManagerOpen(false)}
        onIgnore={ignoreAgent}
        onRestore={restoreAgent}
        onCleanup={cleanupAgent}
      />}
      <CommandPalette open={commandOpen} onOpenChange={setCommandOpen} skills={snapshot.skills.filter((skill) => settings.showAgentBuiltins || !skill.isAgentLocal)} clis={clis} mcps={mcps} instructions={instructions} onMCP={(mcp) => pageTransition(() => { setScope(undefined); setView('mcp'); setMcpFocus(mcp.id) })} onInstruction={() => pageTransition(() => { setScope(undefined); setView('instructions') })} onView={(next) => pageTransition(() => { setScope(undefined); setSource('all'); setView(next) })} onSkill={(skill) => pageTransition(() => { setScope(undefined); setSource('all'); setView('skills'); setSelectedId(skill.id) })} onCLI={(cli) => pageTransition(() => { setCliFocus(cli.id); setView('cli') })} onScan={() => void scan()} onCheckUpdates={() => void checkUpdates(true)} onToggleAppearance={() => changeTheme(nextThemeMode(themeMode))} />
    </SidebarInset>
    <GooeyToaster position="bottom-right" theme={resolvedTheme} closeButton />
  </SidebarProvider>
}

function viewTitle(view: View) { return view === 'skills' ? t('common.skills') : view === 'cli' ? 'CLI' : view === 'mcp' ? 'MCP' : view === 'instructions' ? t('common.instructions') : view === 'project' ? t('common.projects') : view === 'history' ? t('common.history') : t('common.settings') }

export default App
