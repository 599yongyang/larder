import { lazy, Suspense, useEffect, useState } from 'react'
import { MotionConfig, motion } from 'framer-motion'
import { AlertTriangle, CheckCircle2, Coins, GitCompareArrows, History as HistoryIcon, Layers2, LayoutGrid, List, Plus, RefreshCw, RotateCcw, Sparkles, Trash2, Wrench } from 'lucide-react'
import { Badge } from '@/components/ui/badge'
import { AgentIcon } from '../../components/AgentIcon'
import { SkillIcon } from '../../components/SkillIcon'
import { AgentAvatarGroup } from '../../components/AgentAvatarGroup'
import { Button } from '@/components/ui/button'
import { PlanDialog } from '../../components/PlanDialog'
import { IssueRepairDialog } from '../../components/IssueRepairDialog'
import { BatchUpdateDialog, type BatchUpdateItem } from '../../components/BatchUpdateDialog'
import { SkillUpdateDialog } from '../../components/SkillUpdateDialog'
import { VersionDiffDialog } from '../../components/VersionDiffDialog'
const DiscoverSheet = lazy(() => import('../../components/DiscoverSheet').then((module) => ({ default: module.DiscoverSheet })))
import { AssetSearch } from '../../components/AssetSearch'
import { Sheet, SheetContent, SheetDescription, SheetHeader, SheetTitle } from '@/components/ui/sheet'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import styles from '../../App.module.css'
import { backend } from '../../backend'
import { formatTime, skillSourceLabel } from '../../lib/format'
import { toastError, toastSuccess } from '../../lib/toast'
import type { CLIInfo, Issue, Link, Operation, Plan, Root, Skill, UpdateItem, UpdateReport } from '../../types'
import { t, tValue } from '../../i18n'
import { getMotionMode } from '../../lib/motion'
import { backendErrorMessage } from '../../lib/backendError'
import { findSkillElement, playDeleteAnimation } from '../../lib/deleteAnimation'

const layoutTransition = { duration: 0.24, ease: [0.23, 1, 0.32, 1] as const }

function SkillsView({ skills, allSkills, roots, links, operations, selected, onSelect, onCloseDetail, source, sources, onSource, onRefresh, issues, updates, updatesLoading, onCheckUpdates, clis, onOpenCLI }: { skills: Skill[]; allSkills: Skill[]; roots: Root[]; links: Link[]; operations: Operation[]; selected?: Skill; onSelect: (id: string) => void; onCloseDetail: () => void; source: string; sources: string[]; onSource: (value: string) => void; onRefresh: () => Promise<void>; issues: Issue[]; updates?: UpdateReport; updatesLoading: boolean; onCheckUpdates: () => void; clis: CLIInfo[]; onOpenCLI?: (id: string) => void }) {
  const issueSkills = allSkills.filter((skill) => hasIssue(skill, links, issues))
  const issueCount = issueSkills.length
  const [repairOpen, setRepairOpen] = useState(false)
  const [visibleCount, setVisibleCount] = useState(120)
  const issueItems = issueSkills.map((skill) => {
    const own = links.filter((link) => link.skillId === skill.id)
    const copyLinks = own.filter((link) => link.mode === 'copied' && link.drift)
    const brokenLinks = own.filter((link) => link.mode === 'broken')
    const label = (rootId: string) => roots.find((root) => root.id === rootId)?.label ?? rootId
    return { skill, reasons: issueReasons(skill, links, issues), copyRoots: copyLinks.map((link) => link.rootId), copyLabels: copyLinks.map((link) => label(link.rootId)), brokenRoots: brokenLinks.map((link) => link.rootId), brokenLabels: brokenLinks.map((link) => label(link.rootId)) }
  })
  useEffect(() => {
    if (!selected) return
    const frame = requestAnimationFrame(() => {
      const id = typeof CSS !== 'undefined' && CSS.escape ? CSS.escape(selected.id) : selected.id
      document.querySelector<HTMLElement>(`[data-skill-id="${id}"]`)?.scrollIntoView({ block: 'center', behavior: 'smooth' })
    })
    return () => cancelAnimationFrame(frame)
  }, [selected?.id, visibleCount])
  const sourceItems = [{ label: t('common.allSources'), value: 'all' }, ...sources.map((item) => ({ label: item, value: item }))]
  const selectedVisibleCount = selected ? links.filter((link) => link.skillId === selected.id && link.visible).length : 0

    const [plan, setPlan] = useState<Plan>()
  const [planning, setPlanning] = useState(false)
  const [planError, setPlanError] = useState('')
  const [syncPlanning, setSyncPlanning] = useState(false)
  const [syncError, setSyncError] = useState('')
  const [discoverOpen, setDiscoverOpen] = useState(false)
  const [display, setDisplay] = useState<'list' | 'grid'>(() => { try { return (localStorage.getItem('larder.skills.display') ?? localStorage.getItem('aam.skills.display')) === 'grid' ? 'grid' : 'list' } catch { return 'list' } })
  const [grouped, setGrouped] = useState(() => { try { return (localStorage.getItem('larder.skills.grouped') ?? localStorage.getItem('aam.skills.grouped')) !== 'false' } catch { return true } })
  useEffect(() => { try { localStorage.setItem('larder.skills.display', display) } catch {} }, [display])
  useEffect(() => { try { localStorage.setItem('larder.skills.grouped', String(grouped)) } catch {} }, [grouped])
  const [statusFilter, setStatusFilter] = useState<'all' | 'issues' | 'updates'>(() => {
    try { const saved = localStorage.getItem('larder.skills.status'); return saved === 'issues' || saved === 'updates' ? saved : 'all' } catch { return 'all' }
  })
  const [query, setQuery] = useState(() => { try { return localStorage.getItem('larder.skills.query') ?? '' } catch { return '' } })
  useEffect(() => { try { localStorage.setItem('larder.skills.status', statusFilter) } catch {} }, [statusFilter])
  useEffect(() => { try { localStorage.setItem('larder.skills.query', query) } catch {} }, [query])
  const [batchItems, setBatchItems] = useState<BatchUpdateItem[]>([])
  const [batchProgress, setBatchProgress] = useState<{ done: number; total: number }>()
  const [updateTarget, setUpdateTarget] = useState<Skill>()
  const updateItems = (updates?.items ?? []).filter((item) => item.kind === 'skill')
  const updateFor = (skill: Skill) => updateItems.find((item) => item.name === skill.name)
  const updateBySkill = new Map(allSkills.map((skill) => [skill.id, updateFor(skill)]).filter(([, item]) => Boolean(item)) as [string, UpdateItem][])
  const hasUpdate = (skill: Skill) => { const state = updateBySkill.get(skill.id)?.state; return state === 'changed' || state === 'available' }
  const queryText = query.trim().toLowerCase()
  const displayedSkills = [...skills].filter((skill) => statusFilter === 'all' || (statusFilter === 'issues' ? hasIssue(skill, links, issues) : hasUpdate(skill))).filter((skill) => !queryText || [skill.name, skill.description ?? '', skillSourceLabel(skill), skill.path].some((value) => value.toLowerCase().includes(queryText))).sort((a, b) => a.name.localeCompare(b.name, 'zh-CN'))
  const groups = Array.from(displayedSkills.reduce((map, skill) => { const key = skillSourceLabel(skill); if (!map.has(key)) map.set(key, []); map.get(key)!.push(skill); return map }, new Map<string, Skill[]>())).sort(([a], [b]) => a.localeCompare(b))
  const visibleSkills = displayedSkills.slice(0, visibleCount)
  let groupBudget = visibleCount
  const visibleGroups = groups.map(([label, items]) => { const shown = items.slice(0, Math.max(0, groupBudget)); groupBudget -= shown.length; return [label, items, shown] as const }).filter(([, , shown]) => shown.length > 0)
  const remainingSkills = Math.max(0, displayedSkills.length - visibleCount)
  useEffect(() => { setVisibleCount(120) }, [query, statusFilter, source, display, grouped])
  useEffect(() => {
    if (!selected) return
    const index = displayedSkills.findIndex((skill) => skill.id === selected.id)
    if (index >= visibleCount) setVisibleCount(index + 1)
  }, [selected?.id, displayedSkills.length, visibleCount])
  const brokenRoots = selected ? roots.filter((root) => links.find((item) => item.skillId === selected.id && item.rootId === root.id)?.mode === 'broken') : []
  const copiedRoots = selected ? roots.filter((root) => links.find((item) => item.skillId === selected.id && item.rootId === root.id)?.mode === 'copied') : []
  const requestLinkPlan = async (targetRoots: Root[]) => {
    if (!selected || targetRoots.length === 0) return
    setPlanning(true)
    setPlanError('')
    try {
      setPlan(await backend.planLink(selected.id, targetRoots.map((root) => root.id)))
    } catch (error) {
      setPlanError(backendErrorMessage(error, t('common.previewFailed')))
    } finally {
      setPlanning(false)
    }
  }
  const undoDelete = async (operationID: string, name: string) => {
    try {
      await backend.rollbackOperation(operationID)
      toastSuccess(t('skills.versionRestored'), name)
      await onRefresh()
    } catch (error) {
      toastError(error, t('skills.undoFailed'))
    }
  }
  const deleteSkillNow = async (skill: Skill) => {
    setPlanError('')
    try {
      const plan = await backend.planUninstallSkill(skill.id)
      const operation = await backend.applyPlan(plan.id) as Operation
      onCloseDetail()
      toastSuccess(t('app.movedToTrash'), skill.name, { label: t('skills.undo'), onClick: () => void undoDelete(operation.id, skill.name) })
      const animation = playDeleteAnimation(findSkillElement(skill.id))
      const mode = getMotionMode()
      await new Promise((resolve) => setTimeout(resolve, mode === 'off' ? 0 : mode === 'subtle' ? 180 : 700))
      await onRefresh()
      void animation
    } catch (error) {
      toastError(error, t('skills.failedToPreviewUninstall'))
    }
  }
  const undoAddLink = async (skillID: string, rootID: string) => {
    try {
      const plan = await backend.planDisableLink(skillID, [rootID])
      await backend.applyPlan(plan.id)
      await onRefresh()
      toastSuccess(t('skills.addedVisibilityRevoked'))
    } catch (error) {
      toastError(error, t('skills.undoFailed'))
    }
  }
  const requestAddLink = async (rootID: string) => {
    const root = roots.find((item) => item.id === rootID)
    if (!selected || !root) return
    const skillID = selected.id
    setPlanning(true)
    setPlanError('')
    try {
      const plan = await backend.planLink(skillID, [root.id])
      await backend.applyPlan(plan.id)
      await onRefresh()
      toastSuccess(t('skills.visibilityAdded'), `${selected.name} → ${root.label}`, { label: t('skills.undo'), onClick: () => void undoAddLink(skillID, root.id) })
    } catch (error) {
      toastError(error, t('skills.failedToAddVisibility'))
    } finally {
      setPlanning(false)
    }
  }
  const undoDisableLink = async (skillID: string, rootID: string) => {
    try {
      const plan = await backend.planLink(skillID, [rootID])
      await backend.applyPlan(plan.id)
      await onRefresh()
      toastSuccess(t('skills.visibilityRestored'))
    } catch (error) {
      toastError(error, t('skills.restoreFailed'))
    }
  }
  const requestDisable = async (rootID: string) => {
    if (!selected) return
    const skillID = selected.id
    setPlanning(true)
    setPlanError('')
    try {
      const plan = await backend.planDisableLink(skillID, [rootID])
      await backend.applyPlan(plan.id)
      await onRefresh()
      toastSuccess(t('skills.visibilityDisabled'), undefined, { label: t('skills.undo'), onClick: () => void undoDisableLink(skillID, rootID) })
    } catch (error) {
      toastError(error, t('skills.failedToDisableVisibility'))
    } finally {
      setPlanning(false)
    }
  }
  const requestSyncPlan = async () => {
    if (!selected || copiedRoots.length === 0) return
    setSyncPlanning(true)
    setSyncError('')
    try {
      setPlan(await backend.planSyncCopy(selected.id, copiedRoots.map((root) => root.id)))
    } catch (error) {
      setSyncError(backendErrorMessage(error, t('skills.failedToPreviewSync')))
    } finally {
      setSyncPlanning(false)
    }
  }
  const requestGroupUpdate = async (items: Skill[]) => {
    const targets = items.filter((skill) => { const state = updateBySkill.get(skill.id)?.state; return state === 'changed' || state === 'available' })
    if (targets.length === 0 || batchProgress) return
    setPlanError('')
    setBatchProgress({ done: 0, total: targets.length })
    try {
      const plans: Array<{ skill: Skill; plan: Plan }> = []
      for (const skill of targets) plans.push({ skill, plan: await backend.planSkillUpdate(skill.id) })
      const results: BatchUpdateItem[] = new Array(plans.length)
      let next = 0
      let done = 0
      const worker = async () => {
        while (true) {
          const index = next++
          if (index >= plans.length) return
          const { skill, plan } = plans[index]
          try {
            const diff = await backend.previewSkillUpdate(skill.id) as import('../../types').SkillUpdateDiff
            results[index] = { plan, diff }
          } catch (error) {
            results[index] = { plan, error: backendErrorMessage(error, t('skills.updatePreviewFailed')) }
          }
          done += 1
          setBatchProgress({ done, total: plans.length })
        }
      }
      await Promise.all(Array.from({ length: Math.min(3, plans.length) }, () => worker()))
      setBatchItems(results.filter(Boolean))
    } catch (error) {
      setPlanError(backendErrorMessage(error, t('skills.failedToCreateBatchUpdatePlan')))
    } finally {
      setBatchProgress(undefined)
    }
  }
  const updateSelected = () => { if (selected) setUpdateTarget(selected) }
  const copyPath = async () => { if (!selected) return; try { await navigator.clipboard.writeText(selected.path); toastSuccess(t('skills.pathCopied')) } catch (error) { toastError(error, t('skills.failedToCopyPath')) } }
  const revealPath = async () => { if (!selected) return; try { await backend.revealSkill(selected.id) } catch (error) { toastError(error, t('common.failedToOpenLocation')) } }
  return <MotionConfig reducedMotion="user" transition={{ duration: getMotionMode() === 'off' ? 0 : 0.24, ease: [0.23, 1, 0.32, 1] }}><section className={styles.ledgerPanel}>
    <div className={styles.metrics}>
      <Metric label={t('common.skills')} value={allSkills.length} detail={t('skills.skillsScanned')} />
      <Metric label={t('skills.agentDirectories')} value={roots.length} detail={t('skills.actualDirectoryRoots')} />
      <Metric label={t('common.contextCost')} value={`${allSkills.reduce((sum, skill) => sum + skill.descTokens, 0).toLocaleString()} tokens`} detail={t('skills.estimatedPerTurn')} />
      <Metric label={t('skills.needsAttention')} value={issueCount} detail={issueCount ? t('skills.contentDriftBrokenLinksOrMissingFilesBatchFix') : t('skills.contentDriftBrokenLinksOrMissingFiles')} tone={issueCount ? 'warn' : undefined} onClick={issueCount > 0 ? () => setRepairOpen(true) : undefined} />
    </div>
    <div className={styles.filterBar}>
      <div className={styles.filterLeft}>
        <AssetSearch value={query} onChange={setQuery} placeholder={t('skills.searchLabel')} label={t('skills.searchLabel')} />
        <Select value={source} onValueChange={onSource}>
          <SelectTrigger size="sm" className="w-[120px]"><SelectValue /></SelectTrigger>
          <SelectContent>{sourceItems.map((item) => <SelectItem key={item.value} value={item.value}>{item.label}</SelectItem>)}</SelectContent>
        </Select>
        <Select value={statusFilter} onValueChange={(value) => setStatusFilter(value as 'all' | 'issues' | 'updates')}>
          <SelectTrigger size="sm" className="w-[110px]"><SelectValue /></SelectTrigger>
          <SelectContent><SelectItem value="all">{t('common.allStates')}</SelectItem><SelectItem value="issues">{t('skills.issuesOnly')}</SelectItem><SelectItem value="updates">{t('skills.updates')}</SelectItem></SelectContent>
        </Select>
      </div>
      <div className={styles.filterRight}>
        <div className={`${styles.segmentedControl} ${styles.iconSegmented}`} role="group" aria-label={t('skills.displayMode')}>
          <Button variant={display === 'list' ? 'secondary' : 'ghost'} size="icon-sm" aria-pressed={display === 'list'} title={t('skills.listView')} aria-label={t('skills.listView')} onClick={() => setDisplay('list')}><List size={14} /></Button>
          <Button variant={display === 'grid' ? 'secondary' : 'ghost'} size="icon-sm" aria-pressed={display === 'grid'} title={t('skills.cardView')} aria-label={t('skills.cardView')} onClick={() => setDisplay('grid')}><LayoutGrid size={14} /></Button>
        </div>
        <Button variant={grouped ? 'secondary' : 'ghost'} size="icon-sm" aria-pressed={grouped} title={grouped ? t('skills.ungroupBySource') : t('skills.groupBySource')} aria-label={grouped ? t('skills.ungroupBySource') : t('skills.groupBySource')} onClick={() => setGrouped(!grouped)}><Layers2 size={14} /></Button>
        <Button variant="ghost" size="icon-sm" onClick={onCheckUpdates} disabled={updatesLoading} title={updatesLoading ? t('skills.checkingForUpdates') : t('common.checkForUpdates')} aria-label={updatesLoading ? t('skills.checkingForUpdates') : t('common.checkForUpdates')}><RefreshCw size={14} className={updatesLoading ? 'animate-spin' : undefined} /></Button>
        <Button variant="outline" size="sm" onClick={() => setDiscoverOpen(true)}><Plus size={14} />{t('common.installSkill')}</Button>
      </div>
    </div>
    {batchProgress && <div className={styles.batchPreparing} role="status"><span className={styles.batchPreparingDot} />{t('skills.batchPreparing', { done: batchProgress.done, total: batchProgress.total })}</div>}
    {displayedSkills.length === 0 ? <div className={styles.skillsEmpty}><span className={styles.emptyIcon}><Sparkles size={20} aria-hidden="true" /></span>{skills.length === 0 ? <><strong>{t('skills.noScanResultsYet')}</strong><p>{t('skills.scanThisMachineOrInstallDirectlyFromADirectory')}</p><div className={styles.emptyActions}><Button size="sm" onClick={() => void onRefresh()}><RefreshCw size={14} />{t('skills.scanThisMachine')}</Button><Button size="sm" variant="outline" onClick={() => setDiscoverOpen(true)}><Plus size={14} />{t('common.installSkill')}</Button></div></> : <><strong>{t('skills.noMatchingSkills')}</strong><p>{t('skills.noResultsUnderTheCurrentFilters')}</p><div className={styles.emptyActions}><Button size="sm" variant="outline" onClick={() => { setQuery(''); setStatusFilter('all'); onSource('all') }}>{t('skills.clearFilters')}</Button></div></>}</div> : display === 'list' ? (grouped ? <div className={styles.groupedList}><div className={styles.groupedColumns}><span>{t('common.name')}</span><span>{t('common.sourceLabel')}</span><span>{t('skills.agentVisibility')}</span><span>Tokens</span><span>{t('common.actions')}</span></div>{visibleGroups.map(([label, items, visibleItems]) => <section key={label} className={styles.groupedSection}>
      <header className={styles.groupedGroupHeader}><SkillIcon skill={items[0]} className="size-5" /><span>{label}</span><small>{t('common.skillCount', { n: items.length })}</small>{items.some((skill) => hasUpdate(skill)) && <Button size="xs" variant="outline" className={styles.groupUpdateButton} disabled={Boolean(batchProgress)} onClick={() => void requestGroupUpdate(items)}>{t('skills.updateAll')}</Button>}</header>
      {visibleItems.map((skill) => <SkillListRow key={skill.id} skill={skill} roots={roots} links={links} issues={issues} update={updateBySkill.get(skill.id)} selected={selected?.id === skill.id} onSelect={() => onSelect(skill.id)} onUpdate={() => setUpdateTarget(skill)} onUninstall={skill.rootKind === 'global' ? () => void deleteSkillNow(skill) : undefined} />)}
    </section>)}</div> : <div className={`${styles.tableWrap} ${styles.skillsTable}`}><table>
      <thead><tr><th>{t('common.name')}</th><th>{t('common.sourceLabel')}</th><th>{t('skills.agentVisibility')}</th><th>Tokens</th><th>{t('common.actions')}</th></tr></thead>
      <tbody>{visibleSkills.map((skill) => <SkillRow key={skill.id} skill={skill} roots={roots} links={links} issues={issues} update={updateBySkill.get(skill.id)} selected={selected?.id === skill.id} onSelect={() => onSelect(skill.id)} onUpdate={() => setUpdateTarget(skill)} onUninstall={skill.rootKind === 'global' ? () => void deleteSkillNow(skill) : undefined} />)}</tbody>
    </table></div>) : grouped ? <div className={styles.skillGroups}>{visibleGroups.map(([label, items, visibleItems]) => <section key={label} className={styles.skillGroup}><header className={styles.skillGroupHeader}><div className={styles.skillGroupTitle}><SkillIcon skill={items[0]} className="size-5" /><strong>{label}</strong></div><div className={styles.skillGroupMeta}><span>{t('common.skillCount', { n: items.length })}</span>{items.some((skill) => hasUpdate(skill)) && <Button size="xs" variant="outline" disabled={Boolean(batchProgress)} onClick={() => void requestGroupUpdate(items)}>{batchProgress ? t('common.preparing') : t('skills.updateAll')}</Button>}</div></header><div className={styles.skillGrid}>{visibleItems.map((skill) => <SkillCard key={skill.id} skill={skill} roots={roots} links={links} issues={issues} update={updateBySkill.get(skill.id)} selected={selected?.id === skill.id} onSelect={() => onSelect(skill.id)} showIcon={false} showSource={false} onUpdate={() => setUpdateTarget(skill)} onUninstall={skill.rootKind === 'global' ? () => void deleteSkillNow(skill) : undefined} />)}</div></section>)}</div> : <div className={styles.skillGrid}>{visibleSkills.map((skill) => <SkillCard key={skill.id} skill={skill} roots={roots} links={links} issues={issues} update={updateBySkill.get(skill.id)} selected={selected?.id === skill.id} onSelect={() => onSelect(skill.id)} onUpdate={() => setUpdateTarget(skill)} onUninstall={skill.rootKind === 'global' ? () => void deleteSkillNow(skill) : undefined} />)}</div>}
    {remainingSkills > 0 && <div className={styles.skillsMore}><Button variant="outline" size="sm" onClick={() => setVisibleCount((count) => count + 120)}>{t('skills.showMoreRemaining', { n: remainingSkills })}</Button></div>}
    {selected && <Sheet open onOpenChange={(open) => { if (!open) onCloseDetail() }}>
      <SheetContent side="right" className="w-[440px] sm:max-w-[440px]">
        <SheetHeader className="gap-0 border-b px-5 py-4">
          <div className="flex items-start gap-3 pr-9">
            <SkillIcon skill={selected} className="size-9" />
            <div className="min-w-0 flex-1">
              <div className="flex items-center gap-2"><SheetTitle className="truncate text-[15px]">{selected.name}</SheetTitle><UpdateBadge update={updateBySkill.get(selected.id)} onClick={updateSelected} /><StatusBadge skill={selected} issue={issueReasons(selected, links, issues).length > 0} reasons={issueReasons(selected, links, issues)} /></div>
              <SheetDescription className="mt-1 flex flex-wrap items-center gap-x-1.5 gap-y-0.5 text-[11px]">
                <span>{skillSourceLabel(selected)}</span><span>·</span><span>{t('skills.agentsCanDiscover', { visible: selectedVisibleCount, total: roots.length })}</span><span>·</span><span>{t('skills.tokensTurn', { n: selected.descTokens.toLocaleString() })}</span>
              </SheetDescription>
            </div>
          </div>
          {selected.description && <p className={styles.detailHeroDescription}>{selected.description}</p>}
        </SheetHeader>
        <div className="min-h-0 flex-1 overflow-y-auto p-0"><SkillDetail skill={selected} roots={roots} clis={clis} onOpenCLI={onOpenCLI} links={links} issues={issues} update={updateBySkill.get(selected.id)} onUpdate={updateSelected} onCopyPath={copyPath} onReveal={revealPath} onRepairLinks={brokenRoots.length > 0 ? () => void requestLinkPlan(brokenRoots) : undefined} onAddLink={(rootID) => void requestAddLink(rootID)} planning={planning} planError={planError} onSyncPlan={copiedRoots.length > 0 ? () => void requestSyncPlan() : undefined} syncPlanning={syncPlanning} syncError={syncError} onDisable={(rootID) => void requestDisable(rootID)} disabling={planning} onUninstall={selected.rootKind === 'global' ? () => void deleteSkillNow(selected) : undefined} operations={operations} onRestored={() => void onRefresh()} /></div>
      </SheetContent>
    </Sheet>}
    {discoverOpen && <Suspense fallback={null}><DiscoverSheet open={discoverOpen} roots={roots} installedSkills={allSkills} onOpenInstalled={(id) => { setDiscoverOpen(false); onSelect(id) }} onClose={() => setDiscoverOpen(false)} onInstalled={() => { setDiscoverOpen(false); void onRefresh() }} /></Suspense>}
    {repairOpen && <IssueRepairDialog issues={issueItems} onClose={() => setRepairOpen(false)} onLocate={(id) => { setRepairOpen(false); onSelect(id) }} onApplied={() => { setRepairOpen(false); void onRefresh() }} />}
    {updateTarget && <SkillUpdateDialog skillID={updateTarget.id} skillName={updateTarget.name} source={updateTarget.source} onClose={() => setUpdateTarget(undefined)} onApplied={() => { setUpdateTarget(undefined); onCloseDetail(); void onRefresh() }} />}
    {batchItems.length > 0 && <BatchUpdateDialog items={batchItems} onClose={() => setBatchItems([])} onApplied={() => { setBatchItems([]); void onRefresh() }} />}
    {plan && <PlanDialog plan={plan} onClose={() => setPlan(undefined)} onApplied={() => { setPlan(undefined); onCloseDetail(); void onRefresh() }} />}
  </section></MotionConfig>
}

function Metric({ label, value, detail, tone, onClick }: { label: string; value: string | number; detail: string; tone?: 'warn'; onClick?: () => void }) {
  const content = <><span>{label}</span><strong data-tone={tone}>{value}</strong><small>{detail}</small></>
  if (onClick) return <button type="button" className={`${styles.metric} ${styles.metricClickable}`} onClick={onClick} title={t('skills.batchFixIssues')}>{content}</button>
  return <div className={styles.metric}>{content}</div>
}

function issueReasons(skill: Skill, links: Link[], issues: Issue[] = []) {
  const reasons: string[] = []
  const own = links.filter((link) => link.skillId === skill.id)
  if (skill.scanState === 'broken' || own.some((link) => link.mode === 'broken')) reasons.push('链接失效')
  if (!skill.hasSkillMd) reasons.push('缺少 SKILL.md')
  if (own.some((link) => link.drift)) reasons.push('内容不一致')
  if (issues.some((issue) => issue.kind === 'invalid_frontmatter' && issue.path === `${skill.path}/SKILL.md`)) reasons.push('SKILL.md 格式无效')
  return reasons
}

function hasIssue(skill: Skill, links: Link[], issues: Issue[] = []) {
  return issueReasons(skill, links, issues).length > 0
}

function modeLabel(mode: string) {
  return ({ linked: t('skills.sharedLink'), copied: t('skills.copy'), local: t('common.local'), shared: t('common.sharedDiscovery'), missing: t('skills.missing'), broken: t('common.brokenLink'), unavailable: t('skills.unreadable') } as Record<string, string>)[mode] ?? mode
}

function summarize(skill: Skill, roots: Root[], links: Link[], issues: Issue[]) {
  const skillLinks = links.filter((link) => link.skillId === skill.id)
  const visibleLinks = skillLinks.filter((link) => link.visible)
  return {
    skillLinks,
    visible: visibleLinks.length,
    discoverableRoots: roots.filter((root) => visibleLinks.some((link) => link.rootId === root.id)),
    issue: hasIssue(skill, links, issues),
    reasons: issueReasons(skill, links),
  }
}

function OriginBadge({ origin }: { origin?: string }) {
  if (origin !== 'plugin' && origin !== 'system') return null
  return <Badge className={styles.originBadge} variant={origin === 'plugin' ? 'info' : 'outline'} title={origin === 'plugin' ? t('skills.fromAgentPlugInCache') : t('skills.agentBuiltInSkill')}>{origin === 'plugin' ? t('common.plugIn') : t('common.system')}</Badge>
}

function SkillHealthText({ issue, reasons }: { issue: boolean; reasons: string[] }) {
  if (!issue) return null
  const title = reasons.map((reason) => tValue(reason)).join('、') || t('common.needsAttention')
  return <span className={`${styles.statusText} ${styles.statusTextIssue}`} title={title} aria-label={title}>{t('skills.issue')}</span>
}

function SkillUpdateText({ update, onClick }: { update?: UpdateItem; onClick?: () => void }) {
  const canUpdate = update?.state === 'changed' || update?.state === 'available'
  if (!canUpdate) return null
  const content = <><RefreshCw size={12} aria-hidden="true" />{t('common.updateAvailable')}</>
  if (onClick) return <button type="button" className={styles.skillUpdateTextButton} title={update?.message || t('common.updateAvailable')} onClick={(event) => { event.stopPropagation(); onClick() }}>{content}</button>
  return <span className={styles.skillUpdateText} title={update?.message || t('common.updateAvailable')}>{content}</span>
}

function UpdateBadge({ update, onClick }: { update?: UpdateItem; onClick?: () => void }) {
  if (!update || (update.state !== 'changed' && update.state !== 'available')) return null
  const content = <><RefreshCw size={12} aria-hidden="true" />{t('common.updateAvailable')}</>
  if (onClick) return <button type="button" className={styles.updateInlineButton} title={update.message || t('skills.sourceChanged')} onClick={onClick}>{content}</button>
  return <span className={styles.updateInline} title={update.message || t('skills.sourceChanged')}>{content}</span>
}

function StatusBadge({ issue, reasons = [] }: { skill?: Skill; issue: boolean; reasons?: string[] }) {
  if (issue) {
    const label = reasons.length <= 1 ? (reasons[0] ?? t('skills.hasIssue')) : t('skills.issues', { n: reasons.length })
    return <Badge variant="warning" title={reasons.join(' · ')}>{label}</Badge>
  }
  return <Badge variant="success" title={t('skills.noBrokenLinksContentDriftOrMissingSkillMd')}>{t('common.healthy')}</Badge>
}

function SkillRow({ skill, roots, links, issues, update, selected, onSelect, onUpdate, onUninstall, showIcon = true }: { skill: Skill; roots: Root[]; links: Link[]; issues: Issue[]; update?: UpdateItem; selected: boolean; onSelect: () => void; onUpdate?: () => void; onUninstall?: () => void; showIcon?: boolean }) {
  const { discoverableRoots, issue, reasons } = summarize(skill, roots, links, issues)
  return <motion.tr layout="position" transition={{ layout: layoutTransition }} data-skill-id={skill.id} className={`${selected ? styles.selectedRow : ''} ${styles.clickableRow}`} onClick={onSelect}>
    <td><div className={styles.skillNameRow}><div className={styles.skillName}>{showIcon && <SkillIcon skill={skill} className="size-7" />}<span>{skill.name}</span>{skill.isProject && <Badge variant="info">{t('common.projects')}</Badge>}<OriginBadge origin={skill.origin} /><SkillHealthText issue={issue} reasons={reasons} /></div><SkillUpdateText update={update} onClick={onUpdate} /></div><p className={styles.skillDescription}>{skill.description || t('skills.noDescriptionAvailable')}</p></td>
    <td><span className={styles.sourceChip}>{skillSourceLabel(skill)}</span></td>
    <td><AgentAvatarGroup roots={discoverableRoots} /></td>
    <td><span className={styles.tokenValue} title={`${t('skills.tokensPerTurn', { n: skill.descTokens.toLocaleString() })} · ${t('skills.tokenCostHint')}`} aria-label={t('skills.tokensPerTurn', { n: skill.descTokens.toLocaleString() })}><Coins size={12} aria-hidden="true" />{skill.descTokens.toLocaleString()}</span></td>
    <td><div className={styles.quickActions}>{onUninstall && <Button variant="ghost" size="xs" className="text-destructive" title={t('common.uninstallSkill')} aria-label={t('common.uninstallSkill')} onClick={(event) => { event.stopPropagation(); onUninstall() }}><Trash2 size={14} /></Button>}</div></td>
  </motion.tr>
}

function SkillListRow({ skill, roots, links, issues, update, selected, onSelect, onUpdate, onUninstall }: { skill: Skill; roots: Root[]; links: Link[]; issues: Issue[]; update?: UpdateItem; selected: boolean; onSelect: () => void; onUpdate?: () => void; onUninstall?: () => void }) {
  const { discoverableRoots, issue, reasons } = summarize(skill, roots, links, issues)
  return <motion.div layout="position" transition={{ layout: layoutTransition }} data-skill-id={skill.id} className={`${styles.groupedRow} ${selected ? styles.groupedRowSelected : ''} ${styles.clickableRow}`} onClick={onSelect}>
    <div className={styles.groupedName}><div className={styles.skillNameRow}><div className={styles.skillName}><span>{skill.name}</span>{skill.isProject && <Badge variant="info">{t('common.projects')}</Badge>}<OriginBadge origin={skill.origin} /><SkillHealthText issue={issue} reasons={reasons} /></div><SkillUpdateText update={update} onClick={onUpdate} /></div><p className={styles.skillDescription}>{skill.description || t('skills.noDescriptionAvailable')}</p></div>
    <div><span className={styles.sourceChip}>{skillSourceLabel(skill)}</span></div>
    <div><AgentAvatarGroup roots={discoverableRoots} /></div>
    <div><span className={styles.tokenValue} title={`${t('skills.tokensPerTurn', { n: skill.descTokens.toLocaleString() })} · ${t('skills.tokenCostHint')}`} aria-label={t('skills.tokensPerTurn', { n: skill.descTokens.toLocaleString() })}><Coins size={12} aria-hidden="true" />{skill.descTokens.toLocaleString()}</span></div>
    <div><div className={styles.quickActions}>{onUninstall && <Button variant="ghost" size="xs" className="text-destructive" title={t('common.uninstallSkill')} aria-label={t('common.uninstallSkill')} onClick={(event) => { event.stopPropagation(); onUninstall() }}><Trash2 size={14} /></Button>}</div></div>
  </motion.div>
}

function SkillCard({ skill, roots, links, issues, update, selected, onSelect, onUpdate, onUninstall, showIcon = true, showSource = true }: { skill: Skill; roots: Root[]; links: Link[]; issues: Issue[]; update?: UpdateItem; selected: boolean; onSelect: () => void; onUpdate?: () => void; onUninstall?: () => void; showIcon?: boolean; showSource?: boolean }) {
  const { discoverableRoots, issue, reasons } = summarize(skill, roots, links, issues)
  return <motion.div layout="position" transition={{ layout: layoutTransition }} role="button" tabIndex={0} data-skill-id={skill.id} onClick={onSelect} onKeyDown={(event) => { if (event.key === 'Enter' || event.key === ' ') { event.preventDefault(); onSelect() } }} className={`${styles.skillCard} ${selected ? styles.skillCardSelected : ''}`}>
    <div className={styles.skillCardHead}><div className={styles.skillCardTitle}>{showIcon && <SkillIcon skill={skill} className="size-6" />}<strong>{skill.name}</strong>{skill.isProject && <Badge variant="info">{t('common.projects')}</Badge>}<OriginBadge origin={skill.origin} /><SkillHealthText issue={issue} reasons={reasons} /><SkillUpdateText update={update} onClick={onUpdate} /></div><span className={styles.skillCardToken} title={`${t('skills.tokensPerTurn', { n: skill.descTokens.toLocaleString() })} · ${t('skills.tokenCostHint')}`}><Coins size={11} aria-hidden="true" />{skill.descTokens.toLocaleString()}</span></div>
    <p className={styles.skillCardDescription}>{skill.description || t('skills.noDescriptionAvailable')}</p>
    {showSource && <div className={styles.skillCardSource}><span className={styles.sourceChip}>{skillSourceLabel(skill)}</span></div>}
    <div className={styles.skillCardFoot}><div className={styles.skillCardAgents}><AgentAvatarGroup roots={discoverableRoots} /></div>{onUninstall && <Button variant="ghost" size="xs" className={styles.skillCardDelete} title={t('common.uninstallSkill')} aria-label={t('common.uninstallSkill')} onClick={(event) => { event.stopPropagation(); onUninstall() }}><Trash2 size={13} /></Button>}</div>
  </motion.div>
}

function SkillDetail({ skill, roots, links, operations, issues, clis, update, onUpdate, onCopyPath, onReveal, onRepairLinks, onAddLink, planning, planError, onSyncPlan, syncPlanning, syncError, onDisable, disabling, onUninstall, onOpenCLI, onRestored }: { skill: Skill; roots: Root[]; links: Link[]; operations: Operation[]; issues: Issue[]; clis: CLIInfo[]; update?: UpdateItem; onCopyPath?: () => void; onReveal?: () => void; onUpdate?: () => void; onRepairLinks?: () => void; onAddLink?: (rootID: string) => void; planning?: boolean; planError?: string; onSyncPlan?: () => void; syncPlanning?: boolean; syncError?: string; onDisable?: (rootID: string) => void; disabling?: boolean; onUninstall?: () => void; onOpenCLI?: (id: string) => void; onRestored: () => void }) {
  const skillLinks = links.filter((link) => link.skillId === skill.id)
  const visibleCount = skillLinks.filter((link) => link.visible).length
  const linkForRoot = (rootID: string) => skillLinks.find((link) => link.rootId === rootID)
  const rootRank = (root: Root) => {
    const mode = linkForRoot(root.id)?.mode
    if (mode === 'linked') return 0
    if (mode === 'copied') return 1
    if (mode === 'shared') return 2
    if (mode === 'local') return 3
    if (mode === 'broken') return 4
    if (mode === 'unavailable') return 5
    return 6
  }
  const sortedRoots = [...roots].sort((a, b) => rootRank(a) - rootRank(b) || (a.logicalAgent || a.label).localeCompare(b.logicalAgent || b.label, 'zh-CN'))
  const reasons = issueReasons(skill, links, issues)
  const hasDrift = reasons.includes('内容不一致')
  const hasBroken = reasons.includes('链接失效')
  const primary = hasDrift && onSyncPlan ? 'sync' : hasBroken && onRepairLinks ? 'repair' : undefined
  const driftCount = skillLinks.filter((link) => link.drift).length
  const brokenCount = skillLinks.filter((link) => link.mode === 'broken').length
  const healthTitle = primary === 'sync' ? t('common.contentDrift') : primary === 'repair' ? t('common.brokenLink') : reasons.length > 0 ? t('skills.needsManualRepair') : t('skills.allGood')
  const healthText = primary === 'sync' ? t('skills.copiesInAgentDirectoriesDifferFromTheSharedVersion') : primary === 'repair' ? t('skills.linksInAgentDirectoriesAreBroken') : reasons.length > 0 ? (reasons.includes('SKILL.md 格式无效') ? t('skills.skillMdFrontmatterCannotBeParsedFixTheFileOrReinstall') : t('skills.theDirectoryIsMissingSkillMdReinstallOrUpdateFromTheSource')) : t('skills.noContentDriftOrBrokenLinksFound')
  const healthMeta = primary === 'sync' ? t('skills.agentCopiesToSync', { n: driftCount }) : primary === 'repair' ? t('skills.agentLinksToRepair', { n: brokenCount }) : undefined
  const HealthIcon = primary ? AlertTriangle : reasons.length > 0 ? Wrench : CheckCircle2
  const hasAdvanced = (onRepairLinks && primary !== 'repair') || (onSyncPlan && primary !== 'sync')
  return <div>
    {reasons.length > 0 && <section className={styles.detailSection}>
      <h3>{t('skills.health')}</h3>
      <div className={`${styles.healthCard} ${primary ? styles.healthCardWarn : styles.healthCardManual}`}>
        <span className={styles.healthIcon}><HealthIcon size={16} aria-hidden="true" /></span>
        <div className={styles.healthBody}><strong>{healthTitle}</strong><p>{healthText}</p>{healthMeta && <small>{healthMeta}</small>}</div>
        {primary && <Button size="xs" className={styles.healthAction} onClick={primary === 'sync' ? onSyncPlan : onRepairLinks} disabled={planning || syncPlanning}>{planning || syncPlanning ? t('common.preparing') : primary === 'sync' ? t('common.syncContent') : t('common.repairLinks')}</Button>}
      </div>
      {planError && <p className={styles.planError} role="alert">{planError}</p>}
      {syncError && <p className={styles.planError} role="alert">{syncError}</p>}
    </section>}
    <section className={styles.detailSection}>
      <h3>{t('skills.agentVisibility')} <span className={styles.sectionCount}>{visibleCount}/{roots.length}</span></h3>
      <div className={styles.visibilityList}>{sortedRoots.map((root) => {
        const link = linkForRoot(root.id)
        const mode = link?.mode ?? 'missing'
        const tone: 'error' | 'outline' | 'info' | 'success' = mode === 'broken' ? 'error' : mode === 'missing' || mode === 'unavailable' ? 'outline' : mode === 'copied' ? 'info' : 'success'
        return <div key={root.id} className={styles.visibilityRow}><span className={styles.visibilityAgent}><AgentIcon name={root.logicalAgent || root.label} className="size-4" /><span>{root.label}</span></span><span className={styles.visibilityActions}><Badge variant={tone}>{modeLabel(mode)}</Badge>{onAddLink && mode === 'missing' && <Button variant="outline" size="xs" onClick={() => onAddLink(root.id)} disabled={planning}>{t('skills.addVisibility')}</Button>}{onDisable && mode === 'linked' && <Button variant="ghost" size="xs" onClick={() => onDisable(root.id)} disabled={disabling}>{t('common.disableAction')}</Button>}</span></div>
      })}</div>
    </section>
    {(hasAdvanced || onUninstall) && <section className={styles.detailSection}>
      <h3>{t('common.actions')}</h3>
      <div className={styles.actionRow}>
        {onSyncPlan && primary !== 'sync' && <Button variant="outline" size="xs" onClick={onSyncPlan} disabled={syncPlanning}>{syncPlanning ? t('common.preparing') : t('common.syncContent')}</Button>}
        {onRepairLinks && primary !== 'repair' && <Button variant="outline" size="xs" onClick={onRepairLinks} disabled={planning}>{planning ? t('common.preparing') : t('common.repairLinks')}</Button>}
        {update && (update.state === 'changed' || update.state === 'available') && <Button variant="outline" size="xs" onClick={onUpdate}>{t('skills.generateUpdatePlan')}</Button>}
        {onUninstall && <Button variant="outline" size="xs" className={`${styles.dangerAction} ml-auto`} onClick={onUninstall}><Trash2 size={12} />{t('common.uninstallSkill')}</Button>}
      </div>
    </section>}
    {skill.bins && skill.bins.length > 0 && <section className={styles.detailSection}>
      <h3>{t('common.cliDependencies')}</h3>
      <div className={styles.dependencyList}>{skill.bins.map((name) => {
        const cli = clis.find((item) => item.name === name)
        return <div key={name} className={styles.dependencyRow}><code>{name}</code>{cli ? <><Badge variant="success">{cli.version || t('common.installed')}</Badge><Button size="xs" variant="ghost" onClick={() => onOpenCLI?.(cli.id)}>{t('common.viewCli')}</Button></> : <Badge variant="outline">{t('common.notDetected')}</Badge>}</div>
      })}</div>
    </section>}

    <SkillVersionHistory skill={skill} operations={operations} onRestored={onRestored} />
    <section className={styles.detailSection}>
      <h3>{t('skills.sourceBaseline')}</h3>
      <dl className={styles.definition}><dt>{t('skills.localPath')}</dt><dd><code className={styles.detailPath}>{skill.path}</code></dd><dt>{t('common.sourceLabel')}</dt><dd>{skillSourceLabel(skill)}</dd><dt>{t('skills.upstreamPath')}</dt><dd>{skill.skillPath || '—'}</dd><dt>{t('skills.contentBaseline')}</dt><dd><code>{skill.folderHash ? skill.folderHash.slice(0, 16) : t('skills.notCalculated')}</code></dd></dl>
      {(onCopyPath || onReveal) && <div className={styles.detailPathActions}>{onCopyPath && <Button size="xs" variant="outline" onClick={onCopyPath}>{t('common.copyPath')}</Button>}{onReveal && <Button size="xs" variant="ghost" onClick={onReveal}>{t('common.revealInFinder')}</Button>}</div>}
    </section>
  </div>
}



function SkillVersionHistory({ skill, operations, onRestored }: { skill: Skill; operations: Operation[]; onRestored: () => void }) {
  const [target, setTarget] = useState<Operation>()
  const [diffTarget, setDiffTarget] = useState<Operation>()
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')
  const versions = operations.filter((operation) => (operation.operation === 'skill_update' || operation.operation === 'uninstall_skill') && operation.status === 'success' && (operation.changes ?? []).some((change) => change.endsWith(':' + skill.path)))
  const restore = async () => {
    if (!target || busy) return
    setBusy(true); setError('')
    try { await backend.rollbackOperation(target.id); toastSuccess(t('skills.versionRestored')); setTarget(undefined); onRestored() } catch (cause) { setError(backendErrorMessage(cause, t('skills.versionRestoreFailed'))) } finally { setBusy(false) }
  }
  return <section className={styles.detailSection}>
    <h3>{t('skills.versionHistory')} <span className={styles.sectionCount}>{versions.length}</span></h3>
    {versions.length === 0 ? <p className={styles.versionEmpty}>{t('skills.noVersionHistoryYet')}</p> : <div className={styles.versionList}>{versions.slice(0, 8).map((operation) => <div key={operation.id} className={styles.versionRow}>
      <span className={styles.versionIcon}><HistoryIcon size={13} aria-hidden="true" /></span>
      <span className={styles.versionMain}><strong>{operation.operation === 'uninstall_skill' ? t('skills.versionBeforeDelete') : t('skills.versionBeforeUpdate')}</strong><small>{formatTime(operation.createdAt) || '—'}</small></span>
      <Button variant="ghost" size="xs" onClick={() => setDiffTarget(operation)}><GitCompareArrows size={12} />{t('skills.viewDiff')}</Button>
      <Button variant="outline" size="xs" onClick={() => { setError(''); setTarget(operation) }}><RotateCcw size={12} />{t('history.restore')}</Button>
    </div>)}</div>}
    {error && <p className={styles.planError} role="alert">{error}</p>}
    {target && <Dialog open onOpenChange={(open) => { if (!open) setTarget(undefined) }}>
      <DialogContent><DialogHeader><DialogTitle>{t('skills.restoreVersion')}</DialogTitle><DialogDescription>{t('skills.restoreVersionDescription')}</DialogDescription></DialogHeader><div className="p-6"><p className={styles.updateMessage}>{target.operation === 'uninstall_skill' ? t('skills.versionBeforeDelete') : t('skills.versionBeforeUpdate')} · {formatTime(target.createdAt) || '—'}</p>{error && <p className={styles.planError} role="alert">{error}</p>}</div><DialogFooter><Button size="sm" variant="outline" onClick={() => setTarget(undefined)} disabled={busy}>{t('common.cancel')}</Button><Button size="sm" disabled={busy} onClick={() => void restore()}>{busy ? t('common.restoring') : t('history.restore')}</Button></DialogFooter></DialogContent>
    </Dialog>}
    {diffTarget && <VersionDiffDialog operation={diffTarget} onClose={() => setDiffTarget(undefined)} />}
  </section>
}

export { SkillsView }
