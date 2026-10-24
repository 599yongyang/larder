import { useEffect, useRef, useState } from 'react'
import { ArrowDownToLine, BadgeCheck, Check, CheckCircle2, Search, Sparkles, TrendingUp, Trophy } from 'lucide-react'
import { AgentIcon } from './AgentIcon'
import { Button } from '@/components/ui/button'
import { Checkbox } from '@/components/ui/checkbox'
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { Input } from '@/components/ui/input'
import { Sheet, SheetContent, SheetDescription, SheetHeader, SheetTitle } from '@/components/ui/sheet'
import styles from '../App.module.css'
import { backend } from '../backend'
import { toastError, toastSuccess } from '../lib/toast'
import { githubAvatarUrls } from '../lib/github'
import type { Plan, Root, Skill } from '../types'

type CatalogSkill = { id: string; source: string; skillId: string; name: string; installs: number; installsYesterday?: number; change?: number; isOfficial?: boolean }
type CatalogInstall = { path: string; source: string; skillId: string; installedAt?: string }
type View = 'popular' | 'trending'

function formatInstalls(value: number) {
  if (!value) return '0'
  if (value >= 1_000_000) return `${(value / 1_000_000).toFixed(1).replace('.0', '')}M`
  if (value >= 1_000) return `${(value / 1_000).toFixed(1).replace('.0', '')}K`
  return String(value)
}

function CatalogIcon({ skill }: { skill: CatalogSkill }) {
  const owner = skill.source.includes('/') ? skill.source.split('/')[0].trim() : ''
  const urls = owner ? githubAvatarUrls(owner) : []
  const [attempt, setAttempt] = useState(0)
  const [failed, setFailed] = useState(false)
  if (!owner || failed) return <span className={`${styles.discoverIcon} ${styles.discoverIconFallback}`} aria-hidden="true"><Sparkles size={14} /></span>
  return <span className={`${styles.discoverIcon} ${styles.discoverIconLoading}`} aria-hidden="true"><Sparkles size={14} /><img src={urls[attempt]} alt="" loading="eager" decoding="async" referrerPolicy="no-referrer" onError={() => { if (attempt + 1 < urls.length) setAttempt(attempt + 1); else setFailed(true) }} /></span>
}

const descriptionCache = new Map<string, string>()
const descriptionPending = new Map<string, Promise<string>>()
const descriptionQueue: Array<() => void> = []
let descriptionActive = 0

function pumpDescriptions() {
  while (descriptionActive < 6 && descriptionQueue.length > 0) {
    const task = descriptionQueue.shift()
    if (!task) return
    descriptionActive += 1
    task()
  }
}

function loadDescription(source: string, skillID: string): Promise<string> {
  const key = `${source}/${skillID}`
  const cached = descriptionCache.get(key)
  if (cached !== undefined) return Promise.resolve(cached)
  const pending = descriptionPending.get(key)
  if (pending) return pending
  const request = new Promise<string>((resolve) => {
    descriptionQueue.push(() => {
      backend.catalogDescription(source, skillID).then((description) => {
        descriptionCache.set(key, description)
        resolve(description)
      }).catch(() => resolve('')).finally(() => {
        descriptionActive -= 1
        descriptionPending.delete(key)
        pumpDescriptions()
      })
    })
    pumpDescriptions()
  })
  descriptionPending.set(key, request)
  return request
}

function DiscoverCard({ skill, installed, installedSkill, window, onSelect, onOpenInstalled }: { skill: CatalogSkill; installed: boolean; installedSkill?: Skill; window: 'all' | '24h'; onSelect: () => void; onOpenInstalled?: (id: string) => void }) {
  const ref = useRef<HTMLDivElement>(null)
  const [description, setDescription] = useState(() => descriptionCache.get(`${skill.source}/${skill.skillId}`) ?? '')
  useEffect(() => {
    const node = ref.current
    if (!node) return
    let live = true
    const observer = new IntersectionObserver((entries) => {
      if (!entries.some((entry) => entry.isIntersecting)) return
      observer.disconnect()
      void loadDescription(skill.source, skill.skillId).then((value) => { if (live) setDescription(value) })
    }, { rootMargin: '120px' })
    observer.observe(node)
    return () => { live = false; observer.disconnect() }
  }, [skill.source, skill.skillId])
  return <div ref={ref} className={`${styles.discoverCard} ${installed ? styles.discoverCardInstalledCard : ''}`} role={installed ? 'button' : undefined} tabIndex={installed ? 0 : undefined} title={installed ? '打开本地详情' : undefined} onClick={installed ? () => onOpenInstalled?.(installedSkill?.id ?? '') : undefined} onKeyDown={(event) => { if (installed && (event.key === 'Enter' || event.key === ' ')) { event.preventDefault(); onOpenInstalled?.(installedSkill?.id ?? '') } }}>
    <div className={styles.discoverCardHead}><CatalogIcon skill={skill} /><div className={styles.discoverCardMain}><span className={styles.discoverCardName}>{skill.name}</span><span className={styles.discoverCardSource}>{skill.source}{skill.isOfficial && <BadgeCheck className={styles.discoverOfficial} size={11} aria-label="官方来源" />}</span></div></div>
    <p className={styles.discoverCardDescription}>{description || '\u00a0'}</p>
    <div className={styles.discoverCardMeta}><span className={styles.discoverCardStats} title={window === '24h' ? `${skill.installs.toLocaleString()} 次过去 24 小时安装` : `${skill.installs.toLocaleString()} 次安装`}><ArrowDownToLine size={10} aria-hidden="true" /><strong>{formatInstalls(skill.installs)}</strong>{window === '24h' && <small className={styles.discoverCardWindow}>24h</small>}{typeof skill.change === 'number' && skill.change > 0 && <em>+{skill.change}/h</em>}</span>{installed ? <span className={styles.discoverCardInstalled}><Check size={10} aria-hidden="true" />已安装</span> : <Button size="xs" className={styles.discoverInstallButton} onClick={(event) => { event.stopPropagation(); onSelect() }}>安装</Button>}</div>
  </div>
}

export function DiscoverSheet({ open, roots, installedSkills, onClose, onInstalled, onOpenInstalled }: { open: boolean; roots: Root[]; installedSkills: Skill[]; onClose: () => void; onInstalled: () => void; onOpenInstalled?: (skillID: string) => void }) {
  const [view, setView] = useState<View>('popular')
  const [query, setQuery] = useState('')
  const [searchTerm, setSearchTerm] = useState('')
  const [skills, setSkills] = useState<CatalogSkill[]>([])
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState('')
  const [selected, setSelected] = useState<CatalogSkill>()
  const [selectedRoots, setSelectedRoots] = useState<string[]>([])
  const [resolving, setResolving] = useState(false)
  const [planError, setPlanError] = useState('')
  const [installs, setInstalls] = useState<CatalogInstall[]>([])
  const requestRef = useRef(0)
  const localSkill = (skill: CatalogSkill) => {
    const record = installs.find((install) => install.source === skill.source && install.skillId === skill.skillId)
    if (record) { const byPath = installedSkills.find((local) => local.path === record.path); if (byPath) return byPath }
    return installedSkills.find((local) => local.name === skill.name || local.name === skill.skillId)
  }
  const installed = (skill: CatalogSkill) => Boolean(localSkill(skill))
  const targets = roots.filter((root) => (root.kind === 'known_agent' || root.kind === 'unknown_agent') && root.status === 'available')

  const loadView = async (next: View) => {
    const request = ++requestRef.current
    setLoading(true); setError(''); setSearchTerm(''); setSelected(undefined)
    try { const result = (await backend.catalogSkills(next)) as CatalogSkill[]; if (request === requestRef.current) setSkills(result) } catch (cause) { if (request === requestRef.current) setError(cause instanceof Error ? cause.message : '加载 skills.sh 失败') } finally { if (request === requestRef.current) setLoading(false) }
  }
  useEffect(() => { if (open) void loadView(view) }, [open, view])
  useEffect(() => { if (!open) return; backend.catalogInstalls().then((value) => setInstalls((value as CatalogInstall[]) ?? [])).catch(() => setInstalls([])) }, [open])

  const search = async () => {
    const term = query.trim()
    if (term.length < 2) { setError('至少输入 2 个字符'); return }
    const request = ++requestRef.current
    setLoading(true); setError(''); setSelected(undefined)
    try { const result = (await backend.searchCatalogSkills(term)) as CatalogSkill[]; if (request === requestRef.current) { setSkills(result); setSearchTerm(term) } } catch (cause) { if (request === requestRef.current) setError(cause instanceof Error ? cause.message : '搜索失败') } finally { if (request === requestRef.current) setLoading(false) }
  }
  const clearSearch = () => { setQuery(''); void loadView(view) }
  const selectView = (next: View) => { setQuery(''); if (next === view) void loadView(next); else setView(next) }

  const startInstall = async () => {
    if (!selected || installed(selected)) return
    setResolving(true); setPlanError('')
    try {
      const plan = await backend.planInstallCatalogSkill(selected.source, selected.skillId, selected.name || selected.skillId)
      await backend.applyPlan(plan.id)
      await finishInstall(plan)
    } catch (cause) {
      setPlanError(cause instanceof Error ? cause.message : '安装失败')
    } finally {
      setResolving(false)
    }
  }
  const finishInstall = async (installedPlan: Plan) => {
    const rootIDs = selectedRoots
    const catalogSkill = selected
    const targetPath = installedPlan.targetPath ?? installedPlan.changes[0]?.target
    try {
      if (catalogSkill && targetPath) {
        await backend.recordCatalogInstall(targetPath, catalogSkill.source, catalogSkill.skillId)
        setInstalls((current) => [...current, { path: targetPath, source: catalogSkill.source, skillId: catalogSkill.skillId }])
      }
      if (rootIDs.length > 0 && targetPath) {
        const state = await backend.state()
        const installed = state.snapshot.skills.find((skill: Skill) => skill.path === targetPath)
        if (installed) {
          const linkPlan = await backend.planLink(installed.id, rootIDs)
          await backend.applyPlan(linkPlan.id)
        }
      }
      toastSuccess('安装完成', catalogSkill ? `${catalogSkill.name}${rootIDs.length ? ` · ${rootIDs.length} 个 Agent` : ''}` : undefined)
    } catch (cause) {
      toastError(cause, '安装完成，但添加可见性失败')
    } finally {
      setSelected(undefined); setSelectedRoots([]); onInstalled(); onClose()
    }
  }

  return <Sheet open={open} onOpenChange={(next) => { if (!next) onClose() }}>
    <SheetContent side="right" className={`${styles.discoverSheet} gap-0 p-0`} showCloseButton>
      <SheetHeader className={styles.discoverHeader}><SheetTitle className={styles.discoverTitle}>发现 Skills</SheetTitle><SheetDescription className={styles.discoverDescription}>来自 skills.sh 的公开目录。安装仍然走本地计划、冲突检查和快照。</SheetDescription></SheetHeader>
      <div className={styles.discoverToolbar}>
        <div className={styles.discoverTabs} role="group" aria-label="目录视图"><button type="button" className={view === 'popular' && !searchTerm ? styles.discoverTabActive : styles.discoverTab} aria-pressed={view === 'popular' && !searchTerm} title="总安装量" onClick={() => selectView('popular')}><Trophy size={12} />流行</button><button type="button" className={view === 'trending' && !searchTerm ? styles.discoverTabActive : styles.discoverTab} aria-pressed={view === 'trending' && !searchTerm} title="过去 24 小时新增安装" onClick={() => selectView('trending')}><TrendingUp size={12} />趋势</button></div>
        <form className={styles.discoverSearch} onSubmit={(event) => { event.preventDefault(); void search() }}><label className={styles.discoverSearchField}><Search size={13} aria-hidden="true" /><Input className={styles.discoverSearchInput} value={query} onChange={(event) => setQuery(event.target.value)} placeholder="搜索 Skills…" aria-label="搜索 Skills" /></label><Button type="submit" size="xs" variant="outline" disabled={loading}>搜索</Button></form>
        {searchTerm && <button type="button" className={styles.discoverClear} onClick={clearSearch}>清除</button>}
      </div>
      <div className={styles.discoverResults}>
        {loading && <div className={styles.discoverState}>正在读取 skills.sh…</div>}
        {!loading && error && <div className={`${styles.discoverState} ${styles.discoverError}`}>{error}<Button variant="outline" size="xs" onClick={() => void (searchTerm ? search() : loadView(view))}>重试</Button></div>}
        {!loading && !error && skills.length === 0 && <div className={styles.discoverState}>没有找到结果。</div>}
        {!loading && !error && skills.length > 0 && <div className={styles.discoverGrid}>{skills.slice(0, 60).map((skill) => <DiscoverCard key={skill.id} skill={skill} installed={installed(skill)} installedSkill={localSkill(skill)} window={searchTerm ? 'all' : view === 'trending' ? '24h' : 'all'} onSelect={() => { setSelected(skill); setPlanError('') }} onOpenInstalled={onOpenInstalled} />)}        </div>}
      </div>
    </SheetContent>
    {selected && <Dialog open onOpenChange={(open) => { if (!open && !resolving) { setSelected(undefined); setPlanError('') } }}>
      <DialogContent className="max-w-[520px] gap-0 p-0">
        <DialogHeader className="border-b px-6 py-4 pr-12"><DialogTitle className="flex items-center gap-2.5 text-base"><CatalogIcon skill={selected} />安装 {selected.name}</DialogTitle><DialogDescription className="text-xs">{selected.source} · {formatInstalls(selected.installs)} {searchTerm ? 'installs' : view === 'trending' ? '24h installs' : 'installs'}</DialogDescription></DialogHeader>
        <div className={styles.installDialogBody}>
          <div className={styles.installDialogSection}><div className={styles.discoverInstallLabel}>安装位置</div><div className={styles.discoverSharedTarget}><CheckCircle2 size={14} aria-hidden="true" /><span><strong>共享 Skills</strong><small>始终安装，之后可让任意 Agent 使用</small></span></div></div>
          <div className={styles.installDialogSection}><div className={styles.discoverInstallLabelRow}><div className={styles.discoverInstallLabel}>同时让 Agent 可见{selectedRoots.length > 0 && <span>已选 {selectedRoots.length}</span>}</div>{targets.length > 0 && <button type="button" className={styles.discoverSelectAll} onClick={() => setSelectedRoots(selectedRoots.length === targets.length ? [] : targets.map((root) => root.id))}>{selectedRoots.length === targets.length ? '清空' : '全选'}</button>}</div><div className={styles.installDialogAgents}>{targets.map((root) => <label key={root.id} className={styles.discoverAgent}><Checkbox checked={selectedRoots.includes(root.id)} onCheckedChange={(checked) => setSelectedRoots((current) => checked ? [...new Set([...current, root.id])] : current.filter((id) => id !== root.id))} /><AgentIcon name={root.logicalAgent || root.label} className="size-4" /><span>{root.label}</span></label>)}{targets.length === 0 && <p className={styles.discoverInstallHint}>暂无可链接的 Agent。</p>}</div></div>
          {planError && <p className={styles.planError} role="alert">{planError}</p>}
          <p className={styles.installDialogHint}>会先写入快照再安装；失败或误装可在历史中回滚。</p>
        </div>
        <DialogFooter className="border-t px-6 py-2.5"><Button size="sm" variant="outline" disabled={resolving} onClick={() => { setSelected(undefined); setPlanError('') }}>取消</Button><Button size="sm" disabled={resolving} aria-busy={resolving} onClick={() => void startInstall()}>{resolving ? '安装中…' : planError ? '重试安装' : '安装'}</Button></DialogFooter>
      </DialogContent>
    </Dialog>}
  </Sheet>
}
