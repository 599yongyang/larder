import { lazy, Suspense, useEffect, useState } from 'react'
import { AlertTriangle, CheckCircle2, Coins, Layers2, LayoutGrid, List, Plus, RefreshCw, Sparkles, Trash2, Wrench } from 'lucide-react'
import { Badge } from '@/components/ui/badge'
import { AgentIcon } from '../../components/AgentIcon'
import { SkillIcon, preloadSkillIcons } from '../../components/SkillIcon'
import { AgentAvatarGroup } from '../../components/AgentAvatarGroup'
import { Button } from '@/components/ui/button'
import { PlanDialog } from '../../components/PlanDialog'
import { IssueRepairDialog } from '../../components/IssueRepairDialog'
import { BatchPlanDialog } from '../../components/BatchPlanDialog'
import { SkillUpdateDialog } from '../../components/SkillUpdateDialog'
const DiscoverSheet = lazy(() => import('../../components/DiscoverSheet').then((module) => ({ default: module.DiscoverSheet })))
import { AssetSearch } from '../../components/AssetSearch'
import { Sheet, SheetContent, SheetDescription, SheetHeader, SheetTitle } from '@/components/ui/sheet'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import styles from '../../App.module.css'
import { backend } from '../../backend'
import { skillSourceLabel } from '../../lib/format'
import { toastError, toastSuccess } from '../../lib/toast'
import type { CLIInfo, Issue, Link, Plan, Root, Skill, UpdateItem, UpdateReport } from '../../types'

function SkillsView({ skills, allSkills, roots, links, selected, onSelect, onCloseDetail, source, sources, onSource, onRefresh, issues, updates, updatesLoading, onCheckUpdates, clis, onOpenCLI }: { skills: Skill[]; allSkills: Skill[]; roots: Root[]; links: Link[]; selected?: Skill; onSelect: (id: string) => void; onCloseDetail: () => void; source: string; sources: string[]; onSource: (value: string) => void; onRefresh: () => Promise<void>; issues: Issue[]; updates?: UpdateReport; updatesLoading: boolean; onCheckUpdates: () => void; clis: CLIInfo[]; onOpenCLI?: (id: string) => void }) {
  const issueSkills = allSkills.filter((skill) => hasIssue(skill, links, issues))
  const issueCount = issueSkills.length
  const [repairOpen, setRepairOpen] = useState(false)
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
  }, [selected?.id])
  const sourceItems = [{ label: '全部来源', value: 'all' }, ...sources.map((item) => ({ label: item, value: item }))]
  useEffect(() => { preloadSkillIcons(allSkills) }, [allSkills])
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
  const [statusFilter, setStatusFilter] = useState<'all' | 'issues' | 'updates'>('all')
  const [query, setQuery] = useState('')
  const [batchPlans, setBatchPlans] = useState<Plan[]>([])
  const [updateTarget, setUpdateTarget] = useState<Skill>()
  const updateItems = (updates?.items ?? []).filter((item) => item.kind === 'skill')
  const updateFor = (skill: Skill) => updateItems.find((item) => item.name === skill.name)
  const updateBySkill = new Map(allSkills.map((skill) => [skill.id, updateFor(skill)]).filter(([, item]) => Boolean(item)) as [string, UpdateItem][])
  const hasUpdate = (skill: Skill) => { const state = updateBySkill.get(skill.id)?.state; return state === 'changed' || state === 'available' }
  const queryText = query.trim().toLowerCase()
  const displayedSkills = [...skills].filter((skill) => statusFilter === 'all' || (statusFilter === 'issues' ? hasIssue(skill, links, issues) : hasUpdate(skill))).filter((skill) => !queryText || [skill.name, skill.description ?? '', skillSourceLabel(skill), skill.path].some((value) => value.toLowerCase().includes(queryText))).sort((a, b) => a.name.localeCompare(b.name, 'zh-CN'))
  const groups = Array.from(displayedSkills.reduce((map, skill) => { const key = skillSourceLabel(skill); if (!map.has(key)) map.set(key, []); map.get(key)!.push(skill); return map }, new Map<string, Skill[]>())).sort(([a], [b]) => a.localeCompare(b))
  const brokenRoots = selected ? roots.filter((root) => links.find((item) => item.skillId === selected.id && item.rootId === root.id)?.mode === 'broken') : []
  const copiedRoots = selected ? roots.filter((root) => links.find((item) => item.skillId === selected.id && item.rootId === root.id)?.mode === 'copied') : []
  const requestLinkPlan = async (targetRoots: Root[]) => {
    if (!selected || targetRoots.length === 0) return
    setPlanning(true)
    setPlanError('')
    try {
      setPlan(await backend.planLink(selected.id, targetRoots.map((root) => root.id)))
    } catch (error) {
      setPlanError(error instanceof Error ? error.message : '预览失败')
    } finally {
      setPlanning(false)
    }
  }
  const requestUninstall = async () => {
    if (!selected) return
    setPlanError('')
    try {
      setPlan(await backend.planUninstallSkill(selected.id))
    } catch (error) {
      setPlanError(error instanceof Error ? error.message : '预览卸载失败')
    }
  }
  const requestUninstallSkill = async (skill: Skill) => {
    setPlanError('')
    try { setPlan(await backend.planUninstallSkill(skill.id)) } catch (error) { const message = error instanceof Error ? error.message : '预览卸载失败'; setPlanError(message); toastError(error, message) }
  }
  const undoAddLink = async (skillID: string, rootID: string) => {
    try {
      const plan = await backend.planDisableLink(skillID, [rootID])
      await backend.applyPlan(plan.id)
      await onRefresh()
      toastSuccess('已撤销添加')
    } catch (error) {
      toastError(error, '撤销失败')
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
      toastSuccess('已添加可见性', `${selected.name} → ${root.label}`, { label: '撤销', onClick: () => void undoAddLink(skillID, root.id) })
    } catch (error) {
      toastError(error, '添加可见性失败')
    } finally {
      setPlanning(false)
    }
  }
  const undoDisableLink = async (skillID: string, rootID: string) => {
    try {
      const plan = await backend.planLink(skillID, [rootID])
      await backend.applyPlan(plan.id)
      await onRefresh()
      toastSuccess('已恢复可见性')
    } catch (error) {
      toastError(error, '恢复失败')
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
      toastSuccess('已停用可见性', undefined, { label: '撤销', onClick: () => void undoDisableLink(skillID, rootID) })
    } catch (error) {
      toastError(error, '停用可见性失败')
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
      setSyncError(error instanceof Error ? error.message : '预览同步失败')
    } finally {
      setSyncPlanning(false)
    }
  }
  const requestGroupUpdate = async (items: Skill[]) => {
    const targets = items.filter((skill) => { const state = updateBySkill.get(skill.id)?.state; return state === 'changed' || state === 'available' })
    if (targets.length === 0) return
    setPlanError('')
    try {
      const plans: Plan[] = []
      for (const skill of targets) plans.push(await backend.planSkillUpdate(skill.id))
      setBatchPlans(plans)
    } catch (error) {
      setPlanError(error instanceof Error ? error.message : '生成批量更新计划失败')
    }
  }
  const updateSelected = () => { if (selected) setUpdateTarget(selected) }
  const copyPath = async () => { if (!selected) return; try { await navigator.clipboard.writeText(selected.path); toastSuccess('已复制路径') } catch (error) { toastError(error, '复制路径失败') } }
  const revealPath = async () => { if (!selected) return; try { await backend.revealSkill(selected.id) } catch (error) { toastError(error, '打开位置失败') } }
  return <section className={styles.ledgerPanel}>
    <div className={styles.metrics}>
      <Metric label="Skills" value={allSkills.length} detail="扫描到的 Skill" />
      <Metric label="Agent 目录" value={roots.length} detail="实际目录边界" />
      <Metric label="上下文成本" value={`${allSkills.reduce((sum, skill) => sum + skill.descTokens, 0).toLocaleString()} tokens`} detail="每轮上下文估算" />
      <Metric label="需关注" value={issueCount} detail={issueCount ? "内容不一致、链接失效或文件缺失 · 批量处理" : "内容不一致、链接失效或文件缺失"} tone={issueCount ? 'warn' : undefined} onClick={issueCount > 0 ? () => setRepairOpen(true) : undefined} />
    </div>
    <div className={styles.filterBar}>
      <div className={styles.filterLeft}>
        <AssetSearch value={query} onChange={setQuery} placeholder="搜索 Skill" label="搜索 Skill" />
        <Select value={source} onValueChange={onSource}>
          <SelectTrigger size="sm" className="w-[120px]"><SelectValue /></SelectTrigger>
          <SelectContent>{sourceItems.map((item) => <SelectItem key={item.value} value={item.value}>{item.label}</SelectItem>)}</SelectContent>
        </Select>
        <Select value={statusFilter} onValueChange={(value) => setStatusFilter(value as 'all' | 'issues' | 'updates')}>
          <SelectTrigger size="sm" className="w-[110px]"><SelectValue /></SelectTrigger>
          <SelectContent><SelectItem value="all">全部状态</SelectItem><SelectItem value="issues">只看问题</SelectItem><SelectItem value="updates">有更新</SelectItem></SelectContent>
        </Select>
      </div>
      <div className={styles.filterRight}>
        <div className={`${styles.segmentedControl} ${styles.iconSegmented}`} role="group" aria-label="展示方式">
          <Button variant={display === 'list' ? 'secondary' : 'ghost'} size="icon-sm" aria-pressed={display === 'list'} title="列表视图" aria-label="列表视图" onClick={() => setDisplay('list')}><List size={14} /></Button>
          <Button variant={display === 'grid' ? 'secondary' : 'ghost'} size="icon-sm" aria-pressed={display === 'grid'} title="卡片视图" aria-label="卡片视图" onClick={() => setDisplay('grid')}><LayoutGrid size={14} /></Button>
        </div>
        <Button variant={grouped ? 'secondary' : 'ghost'} size="icon-sm" aria-pressed={grouped} title={grouped ? '关闭来源分组' : '按来源分组'} aria-label={grouped ? '关闭来源分组' : '按来源分组'} onClick={() => setGrouped(!grouped)}><Layers2 size={14} /></Button>
        <Button variant="ghost" size="icon-sm" onClick={onCheckUpdates} disabled={updatesLoading} title={updatesLoading ? '检查更新中' : '检查更新'} aria-label={updatesLoading ? '检查更新中' : '检查更新'}><RefreshCw size={14} className={updatesLoading ? 'animate-spin' : undefined} /></Button>
        <Button variant="outline" size="sm" onClick={() => setDiscoverOpen(true)}><Plus size={14} />安装 Skill</Button>
      </div>
    </div>
    {displayedSkills.length === 0 ? <div className={styles.skillsEmpty}><span className={styles.emptyIcon}><Sparkles size={20} aria-hidden="true" /></span>{skills.length === 0 ? <><strong>还没有扫描结果</strong><p>先读取本机的 Skill，或者直接从目录安装。</p><div className={styles.emptyActions}><Button size="sm" onClick={() => void onRefresh()}><RefreshCw size={14} />扫描本机</Button><Button size="sm" variant="outline" onClick={() => setDiscoverOpen(true)}><Plus size={14} />安装 Skill</Button></div></> : <><strong>没有匹配的 Skill</strong><p>当前筛选条件下没有结果。</p><div className={styles.emptyActions}><Button size="sm" variant="outline" onClick={() => { setQuery(''); setStatusFilter('all'); onSource('all') }}>清除筛选</Button></div></>}</div> : display === 'list' ? (grouped ? <div className={styles.groupedList}><div className={styles.groupedColumns}><span>名称</span><span>来源</span><span>Agent 可见性</span><span>Tokens</span><span>操作</span></div>{groups.map(([label, items]) => <section key={label} className={styles.groupedSection}>
      <header className={styles.groupedGroupHeader}><SkillIcon skill={items[0]} className="size-5" /><span>{label}</span><small>{items.length} 个 Skill</small>{items.some((skill) => hasUpdate(skill)) && <Button size="xs" variant="outline" className={styles.groupUpdateButton} onClick={() => void requestGroupUpdate(items)}>更新全部</Button>}</header>
      {items.map((skill) => <SkillListRow key={skill.id} skill={skill} roots={roots} links={links} issues={issues} update={updateBySkill.get(skill.id)} selected={selected?.id === skill.id} onSelect={() => onSelect(skill.id)} onUninstall={skill.rootKind === 'global' ? () => void requestUninstallSkill(skill) : undefined} />)}
    </section>)}</div> : <div className={styles.tableWrap}><table>
      <thead><tr><th>名称</th><th>来源</th><th>Agent 可见性</th><th>Tokens</th><th>操作</th></tr></thead>
      <tbody>{displayedSkills.map((skill) => <SkillRow key={skill.id} skill={skill} roots={roots} links={links} issues={issues} update={updateBySkill.get(skill.id)} selected={selected?.id === skill.id} onSelect={() => onSelect(skill.id)} onUninstall={skill.rootKind === 'global' ? () => void requestUninstallSkill(skill) : undefined} />)}</tbody>
    </table></div>) : grouped ? <div className={styles.skillGroups}>{groups.map(([label, items]) => <section key={label} className={styles.skillGroup}><header className={styles.skillGroupHeader}><div className={styles.skillGroupTitle}><SkillIcon skill={items[0]} className="size-5" /><strong>{label}</strong></div><div className={styles.skillGroupMeta}><span>{items.length} 个 Skill</span>{items.some((skill) => hasUpdate(skill)) && <Button size="xs" variant="outline" onClick={() => void requestGroupUpdate(items)}>更新全部</Button>}</div></header><div className={styles.skillGrid}>{items.map((skill) => <SkillCard key={skill.id} skill={skill} roots={roots} links={links} issues={issues} update={updateBySkill.get(skill.id)} selected={selected?.id === skill.id} onSelect={() => onSelect(skill.id)} showIcon={false} showSource={false} onUpdate={() => setUpdateTarget(skill)} onUninstall={skill.rootKind === 'global' ? () => void requestUninstallSkill(skill) : undefined} />)}</div></section>)}</div> : <div className={styles.skillGrid}>{displayedSkills.map((skill) => <SkillCard key={skill.id} skill={skill} roots={roots} links={links} issues={issues} update={updateBySkill.get(skill.id)} selected={selected?.id === skill.id} onSelect={() => onSelect(skill.id)} onUpdate={() => setUpdateTarget(skill)} onUninstall={skill.rootKind === 'global' ? () => void requestUninstallSkill(skill) : undefined} />)}</div>}
    {selected && <Sheet open onOpenChange={(open) => { if (!open) onCloseDetail() }}>
      <SheetContent side="right" className="w-[440px] sm:max-w-[440px]">
        <SheetHeader className="gap-0 border-b px-5 py-4">
          <div className="flex items-start gap-3">
            <SkillIcon skill={selected} className="size-9" />
            <div className="min-w-0 flex-1">
              <div className="flex items-center gap-2"><SheetTitle className="truncate text-[15px]">{selected.name}</SheetTitle><UpdateBadge update={updateBySkill.get(selected.id)} onClick={updateSelected} /><StatusBadge skill={selected} issue={issueReasons(selected, links, issues).length > 0} reasons={issueReasons(selected, links, issues)} /></div>
              <SheetDescription className="mt-1 flex flex-wrap items-center gap-x-1.5 gap-y-0.5 text-[11px]">
                <span>{skillSourceLabel(selected)}</span><span>·</span><span>{selectedVisibleCount}/{roots.length} 个 Agent 可发现</span><span>·</span><span>{selected.descTokens.toLocaleString()} tokens / 轮</span>
              </SheetDescription>
            </div>
          </div>
          {selected.description && <p className={styles.detailHeroDescription}>{selected.description}</p>}
        </SheetHeader>
        <div className="min-h-0 flex-1 overflow-y-auto p-0"><SkillDetail skill={selected} roots={roots} clis={clis} onOpenCLI={onOpenCLI} links={links} issues={issues} update={updateBySkill.get(selected.id)} onUpdate={updateSelected} onCopyPath={copyPath} onReveal={revealPath} onRepairLinks={brokenRoots.length > 0 ? () => void requestLinkPlan(brokenRoots) : undefined} onAddLink={(rootID) => void requestAddLink(rootID)} planning={planning} planError={planError} onSyncPlan={copiedRoots.length > 0 ? () => void requestSyncPlan() : undefined} syncPlanning={syncPlanning} syncError={syncError} onDisable={(rootID) => void requestDisable(rootID)} disabling={planning} onUninstall={selected.rootKind === 'global' ? () => void requestUninstall() : undefined} /></div>
      </SheetContent>
    </Sheet>}
    {discoverOpen && <Suspense fallback={null}><DiscoverSheet open={discoverOpen} roots={roots} installedSkills={allSkills} onOpenInstalled={(id) => { setDiscoverOpen(false); onSelect(id) }} onClose={() => setDiscoverOpen(false)} onInstalled={() => { setDiscoverOpen(false); void onRefresh() }} /></Suspense>}
    {repairOpen && <IssueRepairDialog issues={issueItems} onClose={() => setRepairOpen(false)} onLocate={(id) => { setRepairOpen(false); onSelect(id) }} onApplied={() => { setRepairOpen(false); void onRefresh() }} />}
    {updateTarget && <SkillUpdateDialog skillID={updateTarget.id} skillName={updateTarget.name} source={updateTarget.source} onClose={() => setUpdateTarget(undefined)} onApplied={() => { setUpdateTarget(undefined); onCloseDetail(); void onRefresh() }} />}
    {batchPlans.length > 0 && <BatchPlanDialog plans={batchPlans} onClose={() => setBatchPlans([])} onApplied={() => { setBatchPlans([]); void onRefresh() }} />}
    {plan && <PlanDialog plan={plan} onClose={() => setPlan(undefined)} onApplied={() => { setPlan(undefined); onCloseDetail(); void onRefresh() }} />}
  </section>
}

function Metric({ label, value, detail, tone, onClick }: { label: string; value: string | number; detail: string; tone?: 'warn'; onClick?: () => void }) {
  const content = <><span>{label}</span><strong data-tone={tone}>{value}</strong><small>{detail}</small></>
  if (onClick) return <button type="button" className={`${styles.metric} ${styles.metricClickable}`} onClick={onClick} title="批量处理问题">{content}</button>
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
  return ({ linked: '共享链接', copied: '副本', local: '本地', shared: '共享发现', missing: '未发现', broken: '链接失效', unavailable: '无法读取' } as Record<string, string>)[mode] ?? mode
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
  return <Badge className={styles.originBadge} variant={origin === 'plugin' ? 'info' : 'outline'} title={origin === 'plugin' ? '来自 Agent 插件缓存' : 'Agent 系统内置 Skill'}>{origin === 'plugin' ? '插件' : '系统'}</Badge>
}

function SkillHealthText({ issue, reasons }: { issue: boolean; reasons: string[] }) {
  if (!issue) return null
  const title = reasons.join('、') || '需要处理'
  return <span className={`${styles.statusText} ${styles.statusTextIssue}`} title={title} aria-label={title}>异常</span>
}

function SkillUpdateText({ update, onClick }: { update?: UpdateItem; onClick?: () => void }) {
  const canUpdate = update?.state === 'changed' || update?.state === 'available'
  if (!canUpdate) return null
  const content = <><RefreshCw size={12} aria-hidden="true" />可更新</>
  if (onClick) return <button type="button" className={styles.skillUpdateTextButton} title={update?.message || '可更新'} onClick={(event) => { event.stopPropagation(); onClick() }}>{content}</button>
  return <span className={styles.skillUpdateText} title={update?.message || '可更新'}>{content}</span>
}

function UpdateBadge({ update, onClick }: { update?: UpdateItem; onClick?: () => void }) {
  if (!update || (update.state !== 'changed' && update.state !== 'available')) return null
  const content = <><RefreshCw size={12} aria-hidden="true" />可更新</>
  if (onClick) return <button type="button" className={styles.updateInlineButton} title={update.message || '来源有变化'} onClick={onClick}>{content}</button>
  return <span className={styles.updateInline} title={update.message || '来源有变化'}>{content}</span>
}

function StatusBadge({ issue, reasons = [] }: { skill?: Skill; issue: boolean; reasons?: string[] }) {
  if (issue) {
    const label = reasons.length <= 1 ? (reasons[0] ?? '有问题') : `${reasons.length} 项问题`
    return <Badge variant="warning" title={reasons.join(' · ')}>{label}</Badge>
  }
  return <Badge variant="success" title="没有发现链接失效、内容不一致或缺少 SKILL.md">正常</Badge>
}

function SkillRow({ skill, roots, links, issues, update, selected, onSelect, onUninstall, showIcon = true }: { skill: Skill; roots: Root[]; links: Link[]; issues: Issue[]; update?: UpdateItem; selected: boolean; onSelect: () => void; onUninstall?: () => void; showIcon?: boolean }) {
  const { discoverableRoots, issue, reasons } = summarize(skill, roots, links, issues)
  return <tr data-skill-id={skill.id} className={`${selected ? styles.selectedRow : ''} ${styles.clickableRow}`} onClick={onSelect}>
    <td><div className={styles.skillNameRow}><div className={styles.skillName}>{showIcon && <SkillIcon skill={skill} className="size-7" />}<span>{skill.name}</span>{skill.isProject && <Badge variant="info">项目</Badge>}<OriginBadge origin={skill.origin} /><SkillHealthText issue={issue} reasons={reasons} /></div><SkillUpdateText update={update} /></div><p className={styles.skillDescription}>{skill.description || '未读取到描述。'}</p></td>
    <td><span className={styles.sourceChip}>{skillSourceLabel(skill)}</span></td>
    <td><AgentAvatarGroup roots={discoverableRoots} /></td>
    <td><span className={styles.tokenValue} title={`每轮约 ${skill.descTokens.toLocaleString()} tokens`} aria-label={`每轮约 ${skill.descTokens.toLocaleString()} tokens`}><Coins size={12} aria-hidden="true" />{skill.descTokens.toLocaleString()}</span></td>
    <td><div className={styles.quickActions}>{onUninstall && <Button variant="ghost" size="xs" className="text-destructive" title="卸载 Skill" aria-label="卸载 Skill" onClick={(event) => { event.stopPropagation(); onUninstall() }}><Trash2 size={14} /></Button>}</div></td>
  </tr>
}

function SkillListRow({ skill, roots, links, issues, update, selected, onSelect, onUninstall }: { skill: Skill; roots: Root[]; links: Link[]; issues: Issue[]; update?: UpdateItem; selected: boolean; onSelect: () => void; onUninstall?: () => void }) {
  const { discoverableRoots, issue, reasons } = summarize(skill, roots, links, issues)
  return <div data-skill-id={skill.id} className={`${styles.groupedRow} ${selected ? styles.groupedRowSelected : ''} ${styles.clickableRow}`} onClick={onSelect}>
    <div className={styles.groupedName}><div className={styles.skillNameRow}><div className={styles.skillName}><span>{skill.name}</span>{skill.isProject && <Badge variant="info">项目</Badge>}<OriginBadge origin={skill.origin} /><SkillHealthText issue={issue} reasons={reasons} /></div><SkillUpdateText update={update} /></div><p className={styles.skillDescription}>{skill.description || '未读取到描述。'}</p></div>
    <div><span className={styles.sourceChip}>{skillSourceLabel(skill)}</span></div>
    <div><AgentAvatarGroup roots={discoverableRoots} /></div>
    <div><span className={styles.tokenValue} title={`每轮约 ${skill.descTokens.toLocaleString()} tokens`} aria-label={`每轮约 ${skill.descTokens.toLocaleString()} tokens`}><Coins size={12} aria-hidden="true" />{skill.descTokens.toLocaleString()}</span></div>
    <div><div className={styles.quickActions}>{onUninstall && <Button variant="ghost" size="xs" className="text-destructive" title="卸载 Skill" aria-label="卸载 Skill" onClick={(event) => { event.stopPropagation(); onUninstall() }}><Trash2 size={14} /></Button>}</div></div>
  </div>
}

function SkillCard({ skill, roots, links, issues, update, selected, onSelect, onUpdate, onUninstall, showIcon = true, showSource = true }: { skill: Skill; roots: Root[]; links: Link[]; issues: Issue[]; update?: UpdateItem; selected: boolean; onSelect: () => void; onUpdate?: () => void; onUninstall?: () => void; showIcon?: boolean; showSource?: boolean }) {
  const { discoverableRoots, issue, reasons } = summarize(skill, roots, links, issues)
  return <div role="button" tabIndex={0} data-skill-id={skill.id} onClick={onSelect} onKeyDown={(event) => { if (event.key === 'Enter' || event.key === ' ') { event.preventDefault(); onSelect() } }} className={`${styles.skillCard} ${selected ? styles.skillCardSelected : ''}`}>
    <div className={styles.skillCardHead}><div className={styles.skillCardTitle}>{showIcon && <SkillIcon skill={skill} className="size-6" />}<strong>{skill.name}</strong>{skill.isProject && <Badge variant="info">项目</Badge>}<OriginBadge origin={skill.origin} /><SkillHealthText issue={issue} reasons={reasons} /><SkillUpdateText update={update} onClick={onUpdate} /></div><div className={styles.skillCardHeadActions}>{onUninstall && <Button variant="ghost" size="xs" className="text-destructive" title="卸载 Skill" aria-label="卸载 Skill" onClick={(event) => { event.stopPropagation(); onUninstall() }}><Trash2 size={14} /></Button>}</div></div>
    <p className={styles.skillCardDescription}>{skill.description || '未读取到描述。'}</p>
    {showSource && <div className={styles.skillCardSource}><span className={styles.sourceChip}>{skillSourceLabel(skill)}</span></div>}
    <div className={styles.skillCardFoot}><div className={styles.skillCardAgents}><AgentAvatarGroup roots={discoverableRoots} /></div><span className={styles.tokenMeta}><span className={styles.tokenValue} title={`每轮约 ${skill.descTokens.toLocaleString()} tokens`}><Coins size={12} aria-hidden="true" />{skill.descTokens.toLocaleString()}</span><span>/ 轮</span></span></div>
  </div>
}

function SkillDetail({ skill, roots, links, issues, clis, update, onUpdate, onCopyPath, onReveal, onRepairLinks, onAddLink, planning, planError, onSyncPlan, syncPlanning, syncError, onDisable, disabling, onUninstall, onOpenCLI }: { skill: Skill; roots: Root[]; links: Link[]; issues: Issue[]; clis: CLIInfo[]; update?: UpdateItem; onCopyPath?: () => void; onReveal?: () => void; onUpdate?: () => void; onRepairLinks?: () => void; onAddLink?: (rootID: string) => void; planning?: boolean; planError?: string; onSyncPlan?: () => void; syncPlanning?: boolean; syncError?: string; onDisable?: (rootID: string) => void; disabling?: boolean; onUninstall?: () => void; onOpenCLI?: (id: string) => void }) {
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
  const healthTitle = primary === 'sync' ? '内容不一致' : primary === 'repair' ? '链接失效' : reasons.length > 0 ? '需要手动处理' : '一切正常'
  const healthText = primary === 'sync' ? 'Agent 目录里的副本和共享版本不同。' : primary === 'repair' ? 'Agent 目录里的链接已经打不开。' : reasons.length > 0 ? (reasons.includes('SKILL.md 格式无效') ? 'SKILL.md 的 frontmatter 无法解析，需要修复文件格式或重新安装。' : '目录缺少 SKILL.md，需要从来源重新安装或更新。') : '没有发现内容不一致或链接失效。'
  const healthMeta = primary === 'sync' ? `${driftCount} 个 Agent 副本待同步` : primary === 'repair' ? `${brokenCount} 个 Agent 链接待修复` : undefined
  const HealthIcon = primary ? AlertTriangle : reasons.length > 0 ? Wrench : CheckCircle2
  const hasAdvanced = (onRepairLinks && primary !== 'repair') || (onSyncPlan && primary !== 'sync')
  return <div>
    {reasons.length > 0 && <section className={styles.detailSection}>
      <h3>健康</h3>
      <div className={`${styles.healthCard} ${primary ? styles.healthCardWarn : styles.healthCardManual}`}>
        <span className={styles.healthIcon}><HealthIcon size={16} aria-hidden="true" /></span>
        <div className={styles.healthBody}><strong>{healthTitle}</strong><p>{healthText}</p>{healthMeta && <small>{healthMeta}</small>}</div>
        {primary && <Button size="xs" className={styles.healthAction} onClick={primary === 'sync' ? onSyncPlan : onRepairLinks} disabled={planning || syncPlanning}>{planning || syncPlanning ? '准备中…' : primary === 'sync' ? '同步内容' : '修复链接'}</Button>}
      </div>
      {planError && <p className={styles.planError} role="alert">{planError}</p>}
      {syncError && <p className={styles.planError} role="alert">{syncError}</p>}
    </section>}
    <section className={styles.detailSection}>
      <h3>Agent 可见性 <span className={styles.sectionCount}>{visibleCount}/{roots.length}</span></h3>
      <div className={styles.visibilityList}>{sortedRoots.map((root) => {
        const link = linkForRoot(root.id)
        const mode = link?.mode ?? 'missing'
        const tone: 'error' | 'outline' | 'info' | 'success' = mode === 'broken' ? 'error' : mode === 'missing' || mode === 'unavailable' ? 'outline' : mode === 'copied' ? 'info' : 'success'
        return <div key={root.id} className={styles.visibilityRow}><span className={styles.visibilityAgent}><AgentIcon name={root.logicalAgent || root.label} className="size-4" /><span>{root.label}</span></span><span className={styles.visibilityActions}><Badge variant={tone}>{modeLabel(mode)}</Badge>{onAddLink && mode === 'missing' && <Button variant="outline" size="xs" onClick={() => onAddLink(root.id)} disabled={planning}>添加可见</Button>}{onDisable && mode === 'linked' && <Button variant="ghost" size="xs" onClick={() => onDisable(root.id)} disabled={disabling}>停用</Button>}</span></div>
      })}</div>
    </section>
    {(hasAdvanced || onUninstall) && <section className={styles.detailSection}>
      <h3>操作</h3>
      <div className={styles.actionRow}>
        {onSyncPlan && primary !== 'sync' && <Button variant="outline" size="xs" onClick={onSyncPlan} disabled={syncPlanning}>{syncPlanning ? '准备中…' : '同步内容'}</Button>}
        {onRepairLinks && primary !== 'repair' && <Button variant="outline" size="xs" onClick={onRepairLinks} disabled={planning}>{planning ? '准备中…' : '修复链接'}</Button>}
        {update && (update.state === 'changed' || update.state === 'available') && <Button variant="outline" size="xs" onClick={onUpdate}>生成更新计划</Button>}
        {onUninstall && <Button variant="outline" size="xs" className={`${styles.dangerAction} ml-auto`} onClick={onUninstall}><Trash2 size={12} />卸载 Skill</Button>}
      </div>
    </section>}
    {skill.bins && skill.bins.length > 0 && <section className={styles.detailSection}>
      <h3>依赖 CLI</h3>
      <div className={styles.dependencyList}>{skill.bins.map((name) => {
        const cli = clis.find((item) => item.name === name)
        return <div key={name} className={styles.dependencyRow}><code>{name}</code>{cli ? <><Badge variant="success">{cli.version || '已安装'}</Badge><Button size="xs" variant="ghost" onClick={() => onOpenCLI?.(cli.id)}>查看 CLI</Button></> : <Badge variant="outline">未检测到</Badge>}</div>
      })}</div>
    </section>}

    <section className={styles.detailSection}>
      <h3>来源与基线</h3>
      <dl className={styles.definition}><dt>本地路径</dt><dd><code className={styles.detailPath}>{skill.path}</code></dd><dt>来源</dt><dd>{skillSourceLabel(skill)}</dd><dt>上游路径</dt><dd>{skill.skillPath || '—'}</dd><dt>内容基线</dt><dd><code>{skill.folderHash ? skill.folderHash.slice(0, 16) : '未计算'}</code></dd></dl>
      {(onCopyPath || onReveal) && <div className={styles.detailPathActions}>{onCopyPath && <Button size="xs" variant="outline" onClick={onCopyPath}>复制路径</Button>}{onReveal && <Button size="xs" variant="ghost" onClick={onReveal}>在 Finder 中显示</Button>}</div>}
    </section>
  </div>
}


export { SkillsView }
