import { useCallback, useEffect, useRef, useState } from 'react'
import { Events } from '@wailsio/runtime'
import { Monitor, Moon, RefreshCw, Search, Sun } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { CommandPalette } from './components/CommandPalette'
import { Kbd } from '@/components/ui/kbd'
import { GooeyToaster, gooeyToast } from 'goey-toast'
import { toastError, toastSuccess } from './lib/toast'
import { AgentManagerDialog } from './components/AgentManagerDialog'

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
import { formatTime, skillSourceLabel } from './lib/format'
import { emptySnapshot } from './types'
import type { AppInfo, InstructionFile, Issue, Operation, TimelineEvent, UpdateReport, UsageStat, View } from './types'
import styles from './App.module.css'
import { backend } from './backend'
import { useAppStore } from './store'
import { applyTheme, getThemeMode, nextThemeMode, resolveTheme, systemTheme } from './lib/theme'
import type { ThemeMode } from './lib/theme'
import { switchTheme } from './lib/themeTransition'
import { pageTransition } from './lib/pageTransition'

function App() {
  const [view, setView] = useState<View>('skills')
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
  const [operations, setOperations] = useState<Operation[]>([])
  const [timeline, setTimeline] = useState<TimelineEvent[]>([])
  const [usage, setUsage] = useState<UsageStat[]>([])
  const [scope, setScope] = useState<{ kind: 'library'; label: string } | { kind: 'agent'; id: string; label: string } | { kind: 'project'; path: string; label: string }>()
  const [source, setSource] = useState('all')
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
  const refresh = useCallback(async () => { try { const next = await backend.state(); setState(next); setUpdates(next.updates?.checkedAt ? next.updates : undefined); setSettings(await backend.settings()); setOperations(await backend.operations()); setTimeline(await backend.timeline()); setUsage(await backend.usageStats()); setMcps(await backend.mcp()); setInstructions(await backend.instructions()); setAppInfo(await backend.appInfo()) } catch (error) { toastError(error, '无法读取应用状态') } }, [])
  useEffect(() => { void refresh() }, [refresh])
  const lastFocusRefresh = useRef(0)
  useEffect(() => {
    const onFocus = () => { const now = Date.now(); if (now - lastFocusRefresh.current < 60_000) return; lastFocusRefresh.current = now; void refresh() }
    window.addEventListener('focus', onFocus)
    return () => window.removeEventListener('focus', onFocus)
  }, [refresh])
  // The Go service emits larder:state on scan start/finish/failure. Refresh only
  // on that signal; idle ledgers never poll a full snapshot.
  useEffect(() => {
    const off = Events.On(LarderState, () => { void refresh() })
    return () => { off?.() }
  }, [refresh])
  const scan = async () => { setUpdates(undefined); const toast = gooeyToast('正在扫描…', { duration: 60000 }); try { await backend.scan(); await refresh(); gooeyToast.update(toast, { title: '扫描完成', type: 'success' }) } catch (error) { gooeyToast.update(toast, { title: '扫描失败', type: 'error', description: error instanceof Error ? error.message : '未知错误' }); await refresh() } }
  const snapshot = state.snapshot ?? emptySnapshot
  const partialIssues = snapshot.issues.filter((issue: Issue) => ['permission', 'scan_failure', 'lockfile_error', 'scan_unavailable'].includes(issue.kind))
  const cacheTime = state.updatedAt ? formatTime(state.updatedAt) : ''
  const ignoredAgents = settings.ignoredAgents ?? []
  const ignoredAgentPaths = new Set(ignoredAgents.map((agent) => agent.path))
  const hasGlobalSkills = snapshot.skills.some((skill) => skill.rootKind === 'global')
  const agentRoots = snapshot.roots.filter((root) => (root.kind === 'known_agent' || root.kind === 'unknown_agent') && !ignoredAgentPaths.has(root.path) && ((root.status === 'available' && root.entryCount > 0) || (root.sharedSkills && hasGlobalSkills)))
  const clis = snapshot.clis ?? []
  const visibleClis = clis
  const ignoreAgent = async (root: { id: string; label: string; path: string }) => {
    try {
      setSettings(await backend.ignoreAgent(root.path, root.label, root.id))
      toastSuccess('已忽略 Agent', root.label)
      await backend.scan()
      await refresh()
    } catch (error) {
      toastError(error, '忽略 Agent 失败')
    }
  }
  const restoreAgent = async (path: string) => {
    try {
      setSettings(await backend.restoreAgent(path))
      toastSuccess('已恢复 Agent', '正在更新扫描结果')
      await backend.scan()
      await refresh()
    } catch (error) {
      toastError(error, '恢复 Agent 失败')
    }
  }
  const cleanupAgent = async (path: string) => {
    try {
      setSettings(await backend.trashAgent(path))
      toastSuccess('已移到废纸篓', '可从 Finder 恢复')
      await backend.scan()
      await refresh()
    } catch (error) {
      toastError(error, '清理 Agent 失败')
    }
  }
  const checkUpdates = async (notify = false) => { setUpdatesLoading(true); try { const next = await backend.checkUpdates(); setUpdates(next); if (notify) toastSuccess('更新检查完成', `${next.skillChanged} 个 Skill · ${next.available} 个 CLI 可更新`) } catch (error) { toastError(error, '检查更新失败') } finally { setUpdatesLoading(false) } }
  useEffect(() => {
    if (!settings.autoCheckUpdates || updatesLoading) return
    const interval = Math.max(1, settings.updateCheckIntervalHours || 24) * 60 * 60 * 1000
    const last = updates?.checkedAt ? Date.parse(updates.checkedAt) : 0
    if (last && Date.now() - last < interval) return
    void checkUpdates()
  }, [settings.autoCheckUpdates, settings.updateCheckIntervalHours, updates?.checkedAt])
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
  const addProject = async (value: string) => { const path = value.trim(); if (!path) return; const next = await backend.saveSettings({ ...settings, projectWorkspaces: [...new Set([...settings.projectWorkspaces, path])] }); setSettings(next); await backend.scan(); await refresh(); toastSuccess('已添加项目工作区', '已重新扫描') }
  const removeProject = async (path: string) => { const next = await backend.saveSettings({ ...settings, projectWorkspaces: settings.projectWorkspaces.filter((item) => item !== path) }); setSettings(next); await backend.scan(); await refresh() }
  return <SidebarProvider className={styles.appShell} style={{ "--sidebar-width": "232px" } as React.CSSProperties}>
    <AppSidebar view={view} scope={scope} onView={goView} onScope={goScope} onManageAgents={() => setAgentManagerOpen(true)} skillsCount={snapshot.skills.filter((skill) => !skill.isAgentLocal).length} cliCount={clis.length} mcpCount={mcps.length}
      instructionCount={instructions.length} onManageProjects={() => setProjectManagerOpen(true)} agents={agentItems} projects={projectItems} centralCount={snapshot.skills.filter((skill) => skill.rootKind === 'global').length} />
    <SidebarInset className={styles.main}><header className={styles.topbar} title="双击最大化窗口" onDoubleClick={(event) => { const target = event.target as HTMLElement; if (target.closest('button, a, input, [role="button"]')) return; event.preventDefault(); void backend.toggleWindowMaximised() }}><h1>{view === 'skills' && scope ? (scope.kind === 'library' ? '共享 Skills' : `${scope.label} · Skills`) : view === 'project' && scope?.kind === 'project' ? `${scope.label} · 项目` : viewTitle(view)}</h1><div className={styles.topActions}><Button variant="outline" size="sm" onClick={() => setCommandOpen(true)} className="w-[260px] justify-start gap-2 font-normal text-muted-foreground"><Search size={14} aria-hidden="true" /><span>搜索或跳转…</span><Kbd className="ml-auto">⌘K</Kbd></Button><Button variant="outline" size="icon-sm" title={state.scanning ? '取消扫描' : '重新扫描'} aria-label={state.scanning ? '取消扫描' : '重新扫描'} onClick={() => state.scanning ? void backend.cancel() : void scan()}><RefreshCw size={14} aria-hidden="true" className={state.scanning ? 'animate-spin' : undefined} /></Button><Button variant="ghost" size="icon-sm" aria-label={`外观：${themeMode === 'system' ? '跟随系统' : themeMode === 'light' ? '明亮' : '暗黑'}，点击切换`} title={`外观：${themeMode === 'system' ? '跟随系统' : themeMode === 'light' ? '明亮' : '暗黑'}，点击切换`} onClick={(event) => changeTheme(nextThemeMode(themeMode), event)}>{themeMode === 'system' ? <Monitor size={15} aria-hidden="true" /> : themeMode === 'dark' ? <Moon size={15} aria-hidden="true" /> : <Sun size={15} aria-hidden="true" />}</Button></div></header>
      <div className={`${styles.scanProgress} ${state.scanning ? styles.scanProgressActive : ''}`} aria-hidden={!state.scanning}><span /></div>
      {state.lastError && <div className={`${styles.banner} ${styles.bannerError}`} role="alert">读取失败：{state.lastError}</div>}
      {!state.lastError && snapshot.status === 'partial' && <div className={`${styles.banner} ${styles.bannerWarn}`}>扫描不完整：{partialIssues.length || snapshot.issues.length} 项读取失败，部分状态为未知</div>}
      {!state.lastError && state.cached && <div className={`${styles.banner} ${styles.bannerCache}`}>缓存数据 · 上次扫描 {cacheTime || '时间未知'}，后台刷新中…</div>}
      <div className={styles.viewBody}>
        {view === 'skills' && <SkillsView skills={visibleSkills} allSkills={scopedSkills} roots={agentRoots} links={snapshot.agentLinks} selected={selectedSkill} onSelect={setSelectedId} onCloseDetail={() => setSelectedId(undefined)} source={source} sources={sources} onSource={setSource} onRefresh={refresh} issues={snapshot.issues} updates={updates} updatesLoading={updatesLoading} onCheckUpdates={() => void checkUpdates(true)} clis={clis} onOpenCLI={(id) => pageTransition(() => { setScope(undefined); setCliFocus(id); setView('cli') })} />}
        {view === 'mcp' && <MCPView servers={mcps} clis={clis} onRefresh={refresh} focusMCP={mcpFocus} onFocusConsumed={() => setMcpFocus(undefined)} onOpenCLI={(id) => pageTransition(() => { setScope(undefined); setCliFocus(id); setView('cli') })} />}
        {view === 'instructions' && <InstructionsView files={instructions} projects={settings.projectWorkspaces} onRefresh={refresh} />}
        {view === 'project' && scope?.kind === 'project' && <ProjectView path={scope.path} skills={snapshot.skills} servers={mcps} instructions={instructions} projects={settings.projectWorkspaces} onRefresh={refresh} />}
        {view === 'cli' && <CLIView clis={visibleClis} skills={snapshot.skills} mcps={mcps} onOpenSkill={(id) => pageTransition(() => { setScope(undefined); setView('skills'); setSelectedId(id) })} onOpenMCP={() => pageTransition(() => { setScope(undefined); setView('mcp') })} onRefresh={refresh} updates={updates} updatesLoading={updatesLoading} onCheckUpdates={() => void checkUpdates(true)} focusCLI={cliFocus} onFocusConsumed={() => setCliFocus(undefined)} />}
        {view === 'history' && <HistoryView events={timeline} usage={usage} operations={operations} onRollback={async (id) => { await backend.rollbackOperation(id); await refresh() }} />}
        {view === 'settings' && <SettingsView settings={settings} onToggle={(value) => void backend.saveSettings({ ...settings, showAgentBuiltins: value }).then(setSettings).catch((error) => toastError(error, '保存失败'))} onUsageToggle={(value) => void backend.saveSettings({ ...settings, usageStatsEnabled: value }).then(setSettings).catch((error) => toastError(error, '保存失败'))} onAutoCheckToggle={(value) => void backend.saveSettings({ ...settings, autoCheckUpdates: value }).then(setSettings).catch((error) => toastError(error, '保存失败'))} onInterval={(value) => void backend.saveSettings({ ...settings, updateCheckIntervalHours: value }).then(setSettings).catch((error) => toastError(error, '保存失败'))} onRetention={(value) => void backend.saveSettings({ ...settings, snapshotRetention: value }).then(setSettings).catch((error) => toastError(error, '保存失败'))} themeMode={themeMode} onThemeMode={setThemeMode} info={appInfo} onBackup={() => void backend.exportBackup().then((path) => toastSuccess('备份已导出', path)).catch((error) => toastError(error, '导出备份失败'))} onRestored={() => void refresh()} />}
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
      <CommandPalette open={commandOpen} onOpenChange={setCommandOpen} skills={snapshot.skills.filter((skill) => settings.showAgentBuiltins || !skill.isAgentLocal)} clis={clis} mcps={mcps} instructions={instructions} onMCP={(mcp) => pageTransition(() => { setScope(undefined); setView('mcp'); setMcpFocus(mcp.id) })} onInstruction={() => pageTransition(() => { setScope(undefined); setView('instructions') })} onView={(next) => pageTransition(() => { setScope(undefined); setSource('all'); setView(next) })} onSkill={(skill) => pageTransition(() => { setScope(undefined); setSource('all'); setView('skills'); setSelectedId(skill.id) })} onCLI={(cli) => pageTransition(() => { setCliFocus(cli.id); setView('cli') })} />
    </SidebarInset>
    <GooeyToaster position="bottom-right" theme={resolvedTheme} />
  </SidebarProvider>
}

function viewTitle(view: View) { return view === 'skills' ? 'Skills' : view === 'cli' ? 'CLI' : view === 'mcp' ? 'MCP' : view === 'instructions' ? '指令' : view === 'project' ? '项目' : view === 'history' ? '历史' : '设置' }

export default App
