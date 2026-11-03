import { useEffect, useRef, useState } from 'react'
import { ArrowDownToLine, BadgeCheck, Check, CheckCircle2, Search, Sparkles, TrendingUp, Trophy } from 'lucide-react'
import { AgentIcon } from './AgentIcon'
import { Button } from '@/components/ui/button'
import { Checkbox } from '@/components/ui/checkbox'
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { Input } from '@/components/ui/input'
import { Sheet, SheetContent, SheetHeader, SheetTitle } from '@/components/ui/sheet'
import styles from '../App.module.css'
import { getLanguage, t } from '../i18n'
import { backendErrorMessage } from '../lib/backendError'
import { backend } from '../backend'
import { toastError, toastSuccess } from '../lib/toast'
import { githubAvatarUrls } from '../lib/github'
import { MarkdownPreview, splitFrontmatter } from './MarkdownPreview'
import type { Plan, Root, Skill } from '../types'

type CatalogSkill = { id: string; source: string; skillId: string; name: string; installs: number; installsYesterday?: number; change?: number; isOfficial?: boolean; version?: string; description?: string; homepage?: string; iconUrl?: string; author?: string; provider?: 'skills.sh' | 'skillhub'; providers?: string[] }
type CatalogInstall = { path: string; source: string; skillId: string; provider?: string; version?: string; homepage?: string; installedAt?: string }
type View = 'popular' | 'trending'

function withProvider(skills: CatalogSkill[]): CatalogSkill[] {
  return skills.map((skill) => ({ ...skill, provider: skill.provider ?? (skill.id.startsWith('skillhub:') ? 'skillhub' : 'skills.sh'), providers: skill.providers?.length ? skill.providers : [skill.provider ?? (skill.id.startsWith('skillhub:') ? 'skillhub' : 'skills.sh')] }))
}

function formatInstalls(value: number) {
  if (!value) return '0'
  return new Intl.NumberFormat(getLanguage() === 'en' ? 'en-US' : 'zh-CN', {
    notation: 'compact',
    maximumFractionDigits: 1,
  }).format(value)
}

function CatalogIcon({ skill }: { skill: CatalogSkill }) {
  const owner = skill.provider === 'skillhub' ? '' : skill.source.includes('/') ? skill.source.split('/')[0].trim() : ''
  const urls = owner ? githubAvatarUrls(owner) : []
  const [attempt, setAttempt] = useState(0)
  const [failed, setFailed] = useState(false)
  const [loaded, setLoaded] = useState(false)
  useEffect(() => { setAttempt(0); setFailed(false); setLoaded(false) }, [skill.iconUrl, skill.source])
  useEffect(() => {
    if (skill.iconUrl || !owner || failed || loaded) return
    const timer = window.setTimeout(() => {
      if (attempt + 1 < urls.length) setAttempt((value) => value + 1)
      else setFailed(true)
    }, 1800)
    return () => window.clearTimeout(timer)
  }, [attempt, failed, loaded, owner, skill.iconUrl, urls.length])
  if (skill.iconUrl && !failed) return <span className={`${styles.discoverIcon} ${styles.discoverIconLoading} ${styles.discoverIconContain}`} aria-hidden="true"><img src={skill.iconUrl} alt="" loading="lazy" decoding="async" referrerPolicy="no-referrer" onLoad={() => setLoaded(true)} onError={() => setFailed(true)} /></span>
  if (!owner || failed) return <span className={`${styles.discoverIcon} ${styles.discoverIconFallback}`} aria-hidden="true"><Sparkles size={14} /></span>
  return <span className={`${styles.discoverIcon} ${styles.discoverIconLoading}`} aria-hidden="true">{!loaded && <Sparkles size={14} />}<img src={urls[attempt]} alt="" loading="lazy" decoding="async" referrerPolicy="no-referrer" onLoad={() => setLoaded(true)} onError={() => { if (attempt + 1 < urls.length) setAttempt(attempt + 1); else setFailed(true) }} /></span>
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

function DiscoverCard({ skill, installed, updatable, selected, window, onSelect, onInstall }: { skill: CatalogSkill; installed: boolean; updatable?: boolean; selected?: boolean; window: 'all' | '24h'; onSelect: () => void; onInstall: () => void }) {
  const ref = useRef<HTMLDivElement>(null)
  const [description, setDescription] = useState(() => skill.description || descriptionCache.get(`${skill.source}/${skill.skillId}`) || '')
  useEffect(() => {
    if (skill.provider === 'skillhub') return
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
  }, [skill.source, skill.skillId, skill.provider])
  return <div ref={ref} className={`${styles.discoverCard} ${installed ? styles.discoverCardInstalledCard : ''} ${selected ? styles.discoverCardSelected : ''}`} role="button" tabIndex={0} onClick={() => onSelect()} onKeyDown={(event) => { if (event.key === 'Enter' || event.key === ' ') { event.preventDefault(); onSelect() } }}>
    <div className={styles.discoverCardHead}><CatalogIcon skill={skill} /><div className={styles.discoverCardMain}><span className={styles.discoverCardName}>{skill.name}</span><span className={styles.discoverCardSource}>{(skill.provider === 'skillhub' ? (skill.author || 'SkillHub') : skill.source)}{skill.isOfficial && <BadgeCheck className={styles.discoverOfficial} size={11} aria-label={t('skills.officialSource')} />}</span></div></div>
    <p className={styles.discoverCardDescription}>{description || '\u00a0'}</p>
    <div className={styles.discoverCardMeta}><span className={styles.discoverCardStats} title={window === '24h' ? t('skills.installsInTheLast24h', { n: skill.installs.toLocaleString(getLanguage() === 'en' ? 'en-US' : 'zh-CN') }) : t('skills.installs', { n: skill.installs.toLocaleString(getLanguage() === 'en' ? 'en-US' : 'zh-CN') })}><ArrowDownToLine size={10} aria-hidden="true" /><strong>{formatInstalls(skill.installs)}</strong>{window === '24h' && <small className={styles.discoverCardWindow}>24h</small>}{typeof skill.change === 'number' && skill.change > 0 && <em>+{skill.change}/h</em>}</span>{installed ? <span className={styles.discoverCardInstalled}><Check size={10} aria-hidden="true" />{updatable ? t('skills.updateAvailable') : t('common.installed')}</span> : <Button size="xs" className={styles.discoverInstallButton} onClick={(event) => { event.stopPropagation(); onInstall() }}>{t('skills.installAction')}</Button>}</div>
  </div>
}

function versionParts(value: string): number[] { return value.replace(/^v/i, '').split(/[.+-]/).map((part) => Number.parseInt(part, 10) || 0) }
function isNewerVersion(latest: string, current: string): boolean {
  const a = versionParts(latest); const b = versionParts(current)
  for (let i = 0; i < Math.max(a.length, b.length); i++) { if ((a[i] ?? 0) !== (b[i] ?? 0)) return (a[i] ?? 0) > (b[i] ?? 0) }
  return false
}

export function DiscoverSheet({ open, roots, installedSkills, onClose, onInstalled, onOpenInstalled }: { open: boolean; roots: Root[]; installedSkills: Skill[]; onClose: () => void; onInstalled: () => void; onOpenInstalled?: (skillID: string) => void }) {
  const [view, setView] = useState<View>('popular')
  const [provider, setProvider] = useState<'skills.sh' | 'skillhub' | 'all'>('all')
  const [query, setQuery] = useState('')
  const [searchTerm, setSearchTerm] = useState('')
  const [skills, setSkills] = useState<CatalogSkill[]>([])
  const [loading, setLoading] = useState(false)
  const [loadingMore, setLoadingMore] = useState(false)
  const [hasMore, setHasMore] = useState(false)
  const [page, setPage] = useState(1)
  const [category, setCategory] = useState('')
  const [categories, setCategories] = useState<Array<{ key: string; name: string; nameEn: string }>>([])
  const [error, setError] = useState('')
  const [selected, setSelected] = useState<CatalogSkill>()
  const [detail, setDetail] = useState<CatalogSkill>()
  const [detailContent, setDetailContent] = useState('')
  const [detailLoading, setDetailLoading] = useState(false)
  const [detailError, setDetailError] = useState('')
  const [detailClosing, setDetailClosing] = useState(false)
  const detailCloseTimer = useRef<number>()
  const detailRequest = useRef(0)
  const fileRequest = useRef(0)
  const [detailFiles, setDetailFiles] = useState<string[]>([])
  const [detailRootContent, setDetailRootContent] = useState('')
  const [activeFile, setActiveFile] = useState('')
  const [installPreview, setInstallPreview] = useState('')
  const [installPreviewLoading, setInstallPreviewLoading] = useState(false)
  const [detailWidth, setDetailWidth] = useState(() => { const saved = Number(window.localStorage.getItem('larder.skill-detail-width')); return Number.isFinite(saved) && saved >= 320 ? Math.min(620, saved) : 440 })
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
  const localInstall = (skill: CatalogSkill) => installs.find((install) => (install.source === skill.source && install.skillId === skill.skillId) || install.skillId === skill.skillId)
  const installed = (skill: CatalogSkill) => Boolean(localSkill(skill))
  const otherSourceInstall = (skill: CatalogSkill) => { const record = localInstall(skill); return record && record.source !== skill.source ? record : undefined }
  const hasUpdate = (skill: CatalogSkill) => {
    const record = localInstall(skill)
    if (!record?.version || !skill.version) return false
    return isNewerVersion(skill.version, record.version)
  }
  const updateSelected = selected ? hasUpdate(selected) : false
  const targets = roots.filter((root) => (root.kind === 'known_agent' || root.kind === 'unknown_agent') && root.status === 'available')

  const loadView = async (next: View) => {
    const request = ++requestRef.current
    setLoading(true); setError(''); setSearchTerm(''); setSelected(undefined); closeDetail()
    try { const result = provider === 'skillhub' ? await backend.skillHubSkillsPage(next, 1, category) : null; const raw = provider === 'all' ? await backend.catalogAllSkills(next) : result ? result.skills : await backend.catalogSkills(next); const mapped = withProvider(raw as CatalogSkill[]); if (request === requestRef.current) { setSkills(mapped); setPage(1); setHasMore(Boolean(result?.hasMore)) } } catch (cause) { if (request === requestRef.current) setError(backendErrorMessage(cause, provider === 'all' ? t('skills.failedToLoadAllSources') : provider === 'skillhub' ? t('skills.failedToLoadSkillHub') : t('skills.failedToLoadSkillsSh'))) } finally { if (request === requestRef.current) setLoading(false) }
  }
  useEffect(() => { if (open) void loadView(view) }, [open, view, provider, category])
  useEffect(() => () => window.clearTimeout(detailCloseTimer.current), [])
  useEffect(() => { if (!selected) { setInstallPreview(''); return }; let live = true; setInstallPreviewLoading(true); setInstallPreview(''); (selected.provider === 'skillhub' ? backend.skillHubDetail(selected.skillId, selected.version ?? '').then((value) => value.content) : backend.catalogSkillContent(selected.source, selected.skillId)).then((content) => { if (live) setInstallPreview(content ? splitFrontmatter(content).body : '') }).catch(() => {}).finally(() => { if (live) setInstallPreviewLoading(false) }); return () => { live = false } }, [selected])
  useEffect(() => { if (!open) return; if (provider !== 'skillhub') { setCategory(''); return }; backend.skillHubCategories().then((items) => setCategories((items as Array<{ key: string; name: string; nameEn: string }>) ?? [])).catch(() => setCategories([])) }, [open, provider])
  useEffect(() => { if (!open) return; backend.catalogInstalls().then((value) => setInstalls((value as CatalogInstall[]) ?? [])).catch(() => setInstalls([])) }, [open])

  const search = async () => {
    const term = query.trim()
    if (term.length < 2) { setError(t('skills.enterAtLeast2Characters')); return }
    const request = ++requestRef.current
    setLoading(true); setError(''); setSelected(undefined); closeDetail()
    try { const result = provider === 'skillhub' ? await backend.searchSkillHubPage(term, 1, category) : null; const raw = provider === 'all' ? await backend.searchCatalogAll(term) : result ? result.skills : await backend.searchCatalogSkills(term); const mapped = withProvider(raw as CatalogSkill[]); if (request === requestRef.current) { setSkills(mapped); setSearchTerm(term); setPage(1); setHasMore(Boolean(result?.hasMore)) } } catch (cause) { if (request === requestRef.current) setError(backendErrorMessage(cause, t('skills.searchFailed'))) } finally { if (request === requestRef.current) setLoading(false) }
  }
  const clearSearch = () => { setQuery(''); void loadView(view) }
  const selectView = (next: View) => { setQuery(''); if (next === view) void loadView(next); else setView(next) }
  const closeDetail = () => {
    detailRequest.current += 1
    setDetailClosing(true)
    window.clearTimeout(detailCloseTimer.current)
    detailCloseTimer.current = window.setTimeout(() => { setDetail(undefined); setDetailClosing(false) }, 180)
  }

  const loadMore = async () => {
    if (loadingMore || !hasMore || provider !== 'skillhub') return
    setLoadingMore(true)
    try {
      const nextPage = page + 1
      const result = searchTerm ? await backend.searchSkillHubPage(searchTerm, nextPage, category) : await backend.skillHubSkillsPage(view, nextPage, category)
      setSkills((current) => [...current, ...withProvider(result.skills as CatalogSkill[])])
      setPage(nextPage); setHasMore(Boolean(result.hasMore))
    } catch (cause) { setError(backendErrorMessage(cause, t('skills.searchFailed'))) }
    finally { setLoadingMore(false) }
  }

  const openDetail = async (skill: CatalogSkill) => {
    window.clearTimeout(detailCloseTimer.current); setDetailClosing(false)
    const request = ++detailRequest.current
    setDetail(skill); setDetailFiles(skill.provider === 'skillhub' ? [] : ['SKILL.md']); setActiveFile('SKILL.md'); setDetailRootContent(''); setDetailContent(skill.description || ''); setDetailError(''); setDetailLoading(true); fileRequest.current += 1
    try {
      let content = ''
      if (skill.provider === 'skillhub') { const result = await backend.skillHubDetail(skill.skillId, skill.version ?? ''); content = result.content; if (request === detailRequest.current) setDetailFiles((result.files as string[]) ?? []) }
      else content = await backend.catalogSkillContent(skill.source, skill.skillId)
      if (request !== detailRequest.current) return
      const body = content ? splitFrontmatter(content).body : ''
      setDetailRootContent(body || skill.description || '')
      setDetailContent(body || skill.description || '')
    } catch (cause) { if (request === detailRequest.current) setDetailError(backendErrorMessage(cause, t('skills.failedToLoadDetails'))) }
    finally { if (request === detailRequest.current) setDetailLoading(false) }
  }



  const startInstall = async () => {
    const existing = selected ? localInstall(selected) : undefined
    const updating = Boolean(existing?.version && selected?.version && existing.version !== selected.version)
    if (!selected || (existing && !updating)) return
    setResolving(true); setPlanError('')
    try {
      const plan = updating && existing
        ? await backend.planUpdateSkillHub(existing.path, selected.skillId, selected.version ?? '')
        : selected.provider === 'skillhub' ? await backend.planInstallSkillHub(selected.skillId, selected.version ?? '', selected.name || selected.skillId) : await backend.planInstallCatalogSkill(selected.source, selected.skillId, selected.name || selected.skillId)
      await backend.applyPlan(plan.id)
      await finishInstall(plan)
    } catch (cause) {
      setPlanError(backendErrorMessage(cause, t('skills.installFailed')))
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
        await backend.recordCatalogInstall(targetPath, catalogSkill.source, catalogSkill.skillId, catalogSkill.provider ?? '', catalogSkill.version ?? '', catalogSkill.homepage ?? '')
        setInstalls((current) => [...current.filter((item) => item.path !== targetPath), { path: targetPath, source: catalogSkill.source, skillId: catalogSkill.skillId, provider: catalogSkill.provider, version: catalogSkill.version, homepage: catalogSkill.homepage }])
      }
      if (rootIDs.length > 0 && targetPath) {
        const state = await backend.state()
        const installed = state.snapshot.skills.find((skill: Skill) => skill.path === targetPath)
        if (installed) {
          const linkPlan = await backend.planLink(installed.id, rootIDs)
          await backend.applyPlan(linkPlan.id)
        }
      }
      toastSuccess(t('skills.installComplete'), catalogSkill ? `${catalogSkill.name}${rootIDs.length ? ` · ${t('common.agents', { n: rootIDs.length })}` : ''}` : undefined)
    } catch (cause) {
      toastError(cause, t('skills.installedButVisibilityCouldNotBeAdded'))
    } finally {
      setSelected(undefined); setSelectedRoots([]); onInstalled(); onClose()
    }
  }

  const startResize = (event: React.PointerEvent<HTMLDivElement>) => {
    event.preventDefault()
    const startX = event.clientX
    const startWidth = detailWidth
    const target = event.currentTarget
    target.setPointerCapture(event.pointerId)
    const move = (moveEvent: PointerEvent) => {
      const max = Math.min(620, Math.max(320, window.innerWidth - 780))
      setDetailWidth(Math.min(max, Math.max(320, startWidth + startX - moveEvent.clientX)))
    }
    const stop = () => {
      target.removeEventListener('pointermove', move)
      target.removeEventListener('pointerup', stop)
      target.removeEventListener('pointercancel', stop)
      setDetailWidth((width) => { window.localStorage.setItem('larder.skill-detail-width', String(width)); return width })
    }
    target.addEventListener('pointermove', move)
    target.addEventListener('pointerup', stop)
    target.addEventListener('pointercancel', stop)
  }

  const openFile = async (filePath: string) => {
    if (!detail) return
    const request = ++fileRequest.current
    setActiveFile(filePath)
    if (filePath === 'SKILL.md') { setDetailContent(detailRootContent); return }
    if (detail.provider !== 'skillhub') return
    setDetailLoading(true); setDetailError('')
    try { const content = await backend.skillHubFileContent(detail.skillId, detail.version ?? '', filePath); if (request === fileRequest.current) setDetailContent(content) }
    catch (cause) { if (request === fileRequest.current) setDetailError(backendErrorMessage(cause, t('skills.failedToLoadDetails'))) }
    finally { if (request === fileRequest.current) setDetailLoading(false) }
  }

  return <Sheet open={open} onOpenChange={(next) => { if (!next) onClose() }}>
    <SheetContent side="right" className={`${styles.discoverSheet} gap-0 p-0`} showCloseButton>
      <SheetHeader className={styles.discoverHeader}><SheetTitle className={styles.discoverTitle}>{t('skills.discoverSkills')}</SheetTitle></SheetHeader>
      <div className={styles.discoverToolbar}>
        <div className={styles.discoverSourceTabs} role="group" aria-label={t('skills.catalogSource')}>{([['all', t('skills.allSources')], ['skills.sh', 'skills.sh'], ['skillhub', 'SkillHub']] as const).map(([value, label]) => <button key={value} type="button" className={provider === value ? styles.discoverSourceTabActive : styles.discoverSourceTab} onClick={() => { if (provider !== value) { setProvider(value); setCategory(''); setQuery(''); setSearchTerm('') } }}>{label}</button>)}</div>
        {provider === 'skillhub' && <select className={styles.discoverCategory} value={category} onChange={(event) => { setCategory(event.target.value); setQuery(''); setSearchTerm('') }} aria-label={t('skills.category')}><option value="">{t('skills.allCategories')}</option>{categories.map((item) => <option key={item.key} value={item.key}>{getLanguage() === 'en' ? item.nameEn : item.name}</option>)}</select>}
        <div className={styles.discoverTabs} role="group" aria-label={t('skills.catalogView')}><button type="button" className={view === 'popular' && !searchTerm ? styles.discoverTabActive : styles.discoverTab} aria-pressed={view === 'popular' && !searchTerm} title={t('skills.totalInstalls')} onClick={() => selectView('popular')}><Trophy size={12} />{t('skills.popular')}</button><button type="button" className={view === 'trending' && !searchTerm ? styles.discoverTabActive : styles.discoverTab} aria-pressed={view === 'trending' && !searchTerm} title={t('skills.newInstallsInTheLast24h')} onClick={() => selectView('trending')}><TrendingUp size={12} />{t('skills.trending')}</button></div>
        <form className={styles.discoverSearch} onSubmit={(event) => { event.preventDefault(); void search() }}><label className={styles.discoverSearchField}><Search size={13} aria-hidden="true" /><Input className={styles.discoverSearchInput} value={query} onChange={(event) => setQuery(event.target.value)} placeholder={t('skills.searchPlaceholder')} aria-label={t('skills.searchSkills')} /></label><Button type="submit" size="xs" variant="outline" disabled={loading}>{t('skills.search')}</Button></form>
        {searchTerm && <button type="button" className={styles.discoverClear} onClick={clearSearch}>{t('skills.clear')}</button>}
      </div>

      <div className={styles.discoverResults}>
        {loading && <div className={styles.discoverState}>{provider === 'all' ? t('skills.loadingAllSources') : provider === 'skillhub' ? t('skills.loadingSkillHub') : t('skills.loadingSkillsSh')}</div>}
        {!loading && error && <div className={`${styles.discoverState} ${styles.discoverError}`}>{error}<Button variant="outline" size="xs" onClick={() => void (searchTerm ? search() : loadView(view))}>{t('skills.retry')}</Button></div>}
        {!loading && !error && skills.length === 0 && <div className={styles.discoverState}>{t('skills.noResults')}</div>}
        {!loading && !error && skills.length > 0 && <div className={styles.discoverGrid}>{skills.slice(0, 60).map((skill) => <DiscoverCard key={skill.id} skill={skill} installed={installed(skill)} updatable={hasUpdate(skill)} selected={detail?.id === skill.id} window={searchTerm ? 'all' : view === 'trending' ? '24h' : 'all'} onSelect={() => void openDetail(skill)} onInstall={() => { setSelected(skill); setPlanError('') }} />)}        </div>}
        {hasMore && <div className={styles.discoverLoadMore}><Button size="xs" variant="outline" disabled={loadingMore} onClick={() => void loadMore()}>{loadingMore ? t('common.working') : t('skills.loadMore')}</Button></div>}
      </div>
      {detail && <aside className={`${styles.discoverDetail} ${detailClosing ? styles.discoverDetailClosing : ''}`} style={{ width: detailWidth }}>
        <div className={styles.discoverDetailHead}><CatalogIcon skill={detail} /><div className={styles.discoverDetailTitle}><strong>{detail.name}</strong><div className={styles.discoverDetailSubline}><span className={styles.discoverDetailAuthor}>{detail.author || (detail.provider === 'skillhub' ? 'SkillHub' : detail.source)}</span><span className={styles.discoverDetailMetaInline}><span>{(detail.providers?.length ?? 0) > 1 ? (detail.providers ?? []).join(' + ') : detail.provider === 'skillhub' ? 'SkillHub' : 'skills.sh'}</span>{detail.version && <span>v{detail.version}</span>}<span>{formatInstalls(detail.installs)} {t('skills.installsShort')}</span>{hasUpdate(detail) && <span className={styles.discoverUpdateBadge}>{t('skills.updateAvailable')}</span>}{otherSourceInstall(detail) && <span className={styles.discoverOtherSource} title={otherSourceInstall(detail)?.source}>{t('skills.installedFromOtherSource')}</span>}</span></div></div><button type="button" className={styles.discoverDetailClose} onClick={closeDetail} aria-label={t('common.close')}>×</button></div>
        {detailFiles.length > 1 && <details className={styles.discoverFiles}><summary>{t('skills.filesCount', { n: detailFiles.length })}</summary><div>{detailFiles.map((file) => <button key={file} type="button" className={activeFile === file ? styles.discoverFileActive : styles.discoverFile} onClick={() => void openFile(file)}>{file}</button>)}</div></details>}
        <div className={styles.discoverDetailScroll}>{detailLoading ? <div className={styles.discoverState}>{t('skills.loadingDetails')}</div> : detailError ? <p className={styles.planError}>{detailError}</p> : detailContent ? <MarkdownPreview source={detailContent} /> : detail.description ? <p className={styles.discoverDetailDescription}>{detail.description}</p> : <div className={styles.discoverState}>{t('skills.noDetails')}</div>}</div>
        <div className={styles.discoverDetailFooter}><Button size="sm" className={styles.discoverInstallButton} onClick={() => { setSelected(detail); setPlanError('') }} disabled={installed(detail) && !hasUpdate(detail)}>{hasUpdate(detail) ? t('skills.updateAction') : installed(detail) ? t('common.installed') : t('skills.installAction')}</Button></div>
        <div className={styles.discoverDetailResize} onPointerDown={startResize} role="separator" aria-orientation="vertical" aria-label={t('skills.resizeDetails')} />
      </aside>}
    </SheetContent>
    {selected && <Dialog open onOpenChange={(open) => { if (!open && !resolving) { setSelected(undefined); setPlanError('') } }}>
      <DialogContent className="max-w-[520px] gap-0 p-0">
        <DialogHeader className="border-b px-6 py-4 pr-12"><DialogTitle className="flex items-center gap-2.5 text-base"><CatalogIcon skill={selected} />{t('skills.install', { name: selected.name })}</DialogTitle><DialogDescription className="text-xs">{(selected.providers?.length ?? 0) > 1 ? (selected.providers ?? []).join(' + ') : selected.provider === 'skillhub' ? 'SkillHub' : selected.source}{selected.author ? ` · ${selected.author}` : ''}{selected.version ? ` · v${selected.version}` : ''} · {formatInstalls(selected.installs)} {t('skills.installsShort')}</DialogDescription></DialogHeader>
        <div className={styles.installDialogBody}>
          <div className={styles.installDialogSection}><div className={styles.discoverInstallLabel}>{t('skills.installLocation')}</div><div className={styles.discoverSharedTarget}><CheckCircle2 size={14} aria-hidden="true" /><span><strong>{t('common.sharedLibrary')}</strong><small>{t('skills.alwaysInstalledAnyAgentCanUseItAfterward')}</small></span></div></div>
          <div className={styles.installDialogSection}><div className={styles.discoverInstallLabelRow}><div className={styles.discoverInstallLabel}>{t('skills.alsoMakeVisibleToAgents')}{selectedRoots.length > 0 && <span>{t('skills.selected', { n: selectedRoots.length })}</span>}</div>{targets.length > 0 && <button type="button" className={styles.discoverSelectAll} onClick={() => setSelectedRoots(selectedRoots.length === targets.length ? [] : targets.map((root) => root.id))}>{selectedRoots.length === targets.length ? t('skills.clearAll') : t('skills.selectAll')}</button>}</div><div className={styles.installDialogAgents}>{targets.map((root) => <label key={root.id} className={styles.discoverAgent}><Checkbox checked={selectedRoots.includes(root.id)} onCheckedChange={(checked) => setSelectedRoots((current) => checked ? [...new Set([...current, root.id])] : current.filter((id) => id !== root.id))} /><AgentIcon name={root.logicalAgent || root.label} className="size-4" /><span>{root.label}</span></label>)}{targets.length === 0 && <p className={styles.discoverInstallHint}>{t('skills.noAgentsAvailableToLink')}</p>}</div></div>
          <details className={styles.installPreview}><summary>{installPreviewLoading ? t('skills.loadingDetails') : t('skills.previewSkillMd')}</summary>{installPreview && <MarkdownPreview source={installPreview} />}</details>
          {planError && <p className={styles.planError} role="alert">{planError}</p>}
          {updateSelected && <p className={styles.installDialogWarning}>{t('skills.updateReplacesFolderWarning')}</p>}
          <p className={styles.installDialogHint}>{t('skills.aSnapshotIsWrittenBeforeInstallingFailedOrMistakenInstallsCanBeRolledBackFromHistory')}</p>
        </div>
        <DialogFooter className="border-t px-6 py-2.5"><Button size="sm" variant="outline" disabled={resolving} onClick={() => { setSelected(undefined); setPlanError('') }}>{t('common.cancel')}</Button><Button size="sm" className={styles.discoverInstallButton} disabled={resolving} aria-busy={resolving} onClick={() => void startInstall()}>{resolving ? t('common.working') : planError ? t('skills.retryInstall') : updateSelected ? t('skills.updateAction') : t('skills.installAction')}</Button></DialogFooter>
      </DialogContent>
    </Dialog>}
  </Sheet>
}
