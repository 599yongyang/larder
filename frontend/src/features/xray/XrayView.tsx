import { useEffect, useRef, useState } from 'react'
import { CircleAlert, CircleCheck, CircleHelp, CircleX, Folder, GitCompareArrows, Layers3, Minus, Server, Sparkles } from 'lucide-react'
import { t } from '../../i18n'
import type { ResourceUsage, Root, Skill } from '../../types'
import { XrayStatus, type XrayEntry as XrayEntryModel, type XrayReport } from '../../../bindings/larder/internal/core/model/models'
import styles from '../../App.module.css'
import { backend } from '../../backend'
import { AgentIcon } from '../../components/AgentIcon'
import { formatRelativeTime } from '../../lib/format'
import { SkillGlyph, SkillIcon } from '../../components/SkillIcon'
import { Checkbox } from '@/components/ui/checkbox'
import { Input } from '@/components/ui/input'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { MCPIcon } from '../../components/MCPIcon'

type Props = {
  root: Root
  projects?: string[]
  compareOptions?: { id: string; label: string; logicalAgent?: string }[]
  refreshKey?: string
  onOpenSkill: (id: string, project?: string) => void
  onReveal?: (id: string) => void
}

type Tab = 'overview' | 'compare' | 'budget'
type StatusValue = XrayStatus | 'absent'

const tokens = new Intl.NumberFormat('en', { maximumFractionDigits: 0 })
const formatTokens = (value: number) => value >= 1000 ? `${(value / 1000).toFixed(1)}k` : tokens.format(value)
const skillForIcon = (skill: XrayEntryModel['skill']) => skill as unknown as Skill

function statusLabel(status: StatusValue | string) {
  return t(`xray.status.${status === 'absent' ? 'absent' : status}`)
}

function statusTone(status: StatusValue | string) {
  return status === 'active' ? styles.xrayStatusOk : status === 'candidate' ? styles.xrayStatusWarn : status === 'broken' || status === 'missing' ? styles.xrayStatusDanger : styles.xrayStatusMuted
}

function StatusBadge({ status }: { status: StatusValue }) {
  const Icon = status === 'active' ? CircleCheck : status === 'candidate' ? CircleHelp : status === 'absent' ? Minus : CircleAlert
  return <span className={`${styles.xrayStatus} ${statusTone(status)}`}><Icon size={10} aria-hidden="true" /><span>{statusLabel(status)}</span></span>
}

type Check = { state: 'ok' | 'warn' | 'bad'; label: string }

function checksFor(entry: XrayEntryModel): Check[] {
  const checks: Check[] = []
  if (entry.reason === 'scan_failure') {
    checks.push({ state: 'bad', label: t('xray.check.unreadable') })
    return checks
  }
  checks.push({ state: 'ok', label: t('xray.check.file') })
  checks.push({ state: entry.skill.hasSkillMd ? 'ok' : 'bad', label: t('xray.check.skillMd') })
  checks.push({ state: entry.skill.description?.trim() ? 'ok' : 'bad', label: t('xray.check.description') })
  if (entry.link && (entry.link.mode === 'broken' || entry.link.mode === 'missing' || entry.link.mode === 'unavailable')) checks.push({ state: 'bad', label: t('xray.check.link') })
  if ((entry.candidates?.length ?? 0) > 1) checks.push({ state: 'warn', label: t('xray.check.duplicate', { n: entry.candidates?.length ?? 0 }) })
  return checks
}

function StatusChecks({ entry }: { entry: XrayEntryModel }) {
  return <ul className={styles.xrayChecks}>{checksFor(entry).map((check) => <li key={check.label} className={check.state === 'ok' ? styles.xrayCheckOk : check.state === 'warn' ? styles.xrayCheckWarn : styles.xrayCheckBad}>{check.state === 'ok' ? <CircleCheck size={13} aria-hidden="true" /> : check.state === 'warn' ? <CircleAlert size={13} aria-hidden="true" /> : <CircleX size={13} aria-hidden="true" />}<span>{check.label}</span></li>)}</ul>
}

function WhyBody({ entry, mcps }: { entry: XrayEntryModel; mcps: { id: string; name: string }[] }) {
  const active = entry.status === 'active'
  const caveat = active ? t('xray.why.staticNote') : entry.reason === 'duplicate_candidates' ? t('xray.why.candidateNote') : t('xray.why.problemNote')
  return <>
    <StatusChecks entry={entry} />
    <p className={styles.xrayWhyNote}>{caveat}</p>
    <dl className={styles.xrayDefinition}><dt>{t('xray.why.path')}</dt><dd title={entry.skill.path}>{entry.skill.path}</dd><dt>{t('xray.why.source')}</dt><dd>{entry.skill.source || '—'}</dd><dt>{t('xray.why.mcp')}</dt><dd>{mcps.length ? <span className={styles.xrayMcpList}>{mcps.map((server) => <span key={server.id}><Server size={10} aria-hidden="true" />{server.name}</span>)}</span> : '—'}</dd></dl>
  </>
}

export function XrayView({ root, projects = [], compareOptions = [], refreshKey, onOpenSkill, onReveal }: Props) {
  const [project, setProject] = useState('')
  const [report, setReport] = useState<XrayReport>()
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')
  const [reload, setReload] = useState(0)
  const [tab, setTab] = useState<Tab>('overview')
  const [selectedId, setSelectedId] = useState<string>()
  const [query, setQuery] = useState('')
  const [status, setStatus] = useState<'all' | XrayStatus>('all')
  const [compareIDs, setCompareIDs] = useState<string[]>([])
  const [compareOpen, setCompareOpen] = useState(false)
  const [usage, setUsage] = useState<ResourceUsage[]>([])
  const [usageLoading, setUsageLoading] = useState(true)
  const comparePickerRef = useRef<HTMLDivElement>(null)

  useEffect(() => {
    let cancelled = false
    setLoading(true)
    setError('')
    backend.xray(root.id, project, compareIDs).then((next) => {
      if (cancelled) return
      setReport(next as unknown as XrayReport)
      setLoading(false)
    }).catch((reason) => {
      if (cancelled) return
      setError(String(reason?.message ?? reason))
      setLoading(false)
    })
    return () => { cancelled = true }
  }, [root.id, project, compareIDs, refreshKey, reload])

  useEffect(() => { setCompareIDs([]); setCompareOpen(false) }, [root.id])

  useEffect(() => {
    if (!compareOpen) return
    const close = (event: MouseEvent) => { if (!comparePickerRef.current?.contains(event.target as Node)) setCompareOpen(false) }
    const escape = (event: KeyboardEvent) => { if (event.key === 'Escape') setCompareOpen(false) }
    document.addEventListener('mousedown', close)
    document.addEventListener('keydown', escape)
    return () => { document.removeEventListener('mousedown', close); document.removeEventListener('keydown', escape) }
  }, [compareOpen])

  const entries = report?.entries ?? []
  const columns = report?.compare?.columns ?? []
  const compareRows = report?.compare?.rows ?? []
  const mcps = report?.mcps ?? []
  const contributors = report?.contributors ?? []
  const queryText = query.trim().toLowerCase()
  const filtered = entries.filter((entry) => (status === 'all' || entry.status === status) && (!queryText || [entry.skill.name, entry.skill.path, entry.skill.source ?? ''].some((value) => value.toLowerCase().includes(queryText))))
  const selected = entries.find((entry) => entry.skill.id === selectedId) ?? entries[0]
  const displayedCompareIDs = compareIDs.length ? compareIDs : columns.map((column) => column.root.id)
  const canAddCompare = displayedCompareIDs.length < 4
  const toggleCompare = (id: string) => {
    if (id === root.id) return
    const base = displayedCompareIDs
    const next = base.includes(id) ? base.filter((value) => value !== id) : (canAddCompare ? [...base, id] : base)
    setCompareIDs(next.includes(root.id) ? next : [root.id, ...next])
  }
  const uniqueCompareOptions = compareOptions.filter((option, index, all) => all.findIndex((candidate) => (candidate.logicalAgent || candidate.label) === (option.logicalAgent || option.label)) === index)
  const usageAgent = root.logicalAgent || root.label
  useEffect(() => {
    let active = true
    setUsageLoading(true)
    backend.agentUsage(usageAgent).then((items) => { if (active) setUsage(items) }).catch(() => { if (active) setUsage([]) }).finally(() => { if (active) setUsageLoading(false) })
    return () => { active = false }
  }, [usageAgent, refreshKey])
  const totalKnown = report?.budget.knownTokens ?? 0
  const skillTokens = report?.budget.skillTokens ?? 0
  const instructionTokens = report?.budget.instructionTokens ?? 0


  return <div className={styles.xrayPage}>
    {root.status !== 'available' && <div className={styles.xrayRootAlert} role="status"><CircleAlert size={14} aria-hidden="true" /><span>{t('xray.rootUnavailable', { status: root.status })}{root.error ? ` · ${root.error}` : ''}</span></div>}
    <div className={styles.xrayHeader}>
      <div className={styles.xrayIdentity} title={t('xray.subtitle')}><span className={styles.xrayAgentAvatar}><AgentIcon name={root.logicalAgent || root.label} className="size-5" /></span><code className={styles.xrayRootPath} title={root.path}>{root.path}</code></div>
      <div className={styles.xrayHeaderMeta}>
        {projects.length > 0 && <Select value={project || '__all__'} onValueChange={(value) => setProject(value === '__all__' ? '' : value)}><SelectTrigger size="sm" className="w-[190px]" aria-label={t('xray.project')}><Folder size={13} aria-hidden="true" /><SelectValue /></SelectTrigger><SelectContent><SelectItem value="__all__">{t('xray.allProjects')}</SelectItem>{projects.map((path) => <SelectItem key={path} value={path}>{path.split('/').filter(Boolean).pop() || path}</SelectItem>)}</SelectContent></Select>}
        <span className={styles.xrayHeaderPill}>{t('xray.staticOnly')}</span>
      </div>
    </div>

    <div className={styles.xrayTabs} role="tablist" aria-label={t('xray.title')}>
      <button type="button" role="tab" aria-selected={tab === 'overview'} className={tab === 'overview' ? styles.xrayTabActive : styles.xrayTab} onClick={() => setTab('overview')}><Layers3 size={14} aria-hidden="true" />{t('xray.tab.overview')}</button>
      <button type="button" role="tab" aria-selected={tab === 'compare'} className={tab === 'compare' ? styles.xrayTabActive : styles.xrayTab} onClick={() => setTab('compare')}><GitCompareArrows size={14} aria-hidden="true" />{t('xray.tab.compare')}</button>
      <button type="button" role="tab" aria-selected={tab === 'budget'} className={tab === 'budget' ? styles.xrayTabActive : styles.xrayTab} onClick={() => setTab('budget')}><CircleHelp size={14} aria-hidden="true" />{t('xray.tab.budget')}</button>
    </div>

    {loading && !report ? <div className={styles.xrayState}><span className={styles.xrayStateIcon}><Layers3 size={18} aria-hidden="true" /></span><strong>{t('xray.loading')}</strong><p>{t('xray.loadingHint')}</p></div> : error && !report ? <div className={styles.xrayState}><span className={styles.xrayStateIcon}><CircleAlert size={18} aria-hidden="true" /></span><strong>{t('xray.loadFailed')}</strong><p>{error}</p><button type="button" className={styles.xrayLinkButton} onClick={() => setReload((value) => value + 1)}>{t('skills.retry')} →</button></div> : tab === 'overview' && <>
      <div className={styles.xrayMetrics}>
        <div className={styles.metric}><span>{t('xray.metric.visibleSkills')}</span><strong>{report?.activeCount ?? 0}</strong><small>{t('xray.metric.candidates', { n: report?.candidateCount ?? 0 })}</small></div>
        <div className={styles.metric}><span>{t('xray.metric.needsAttention')}</span><strong data-tone={(report?.missingCount ?? 0) + (report?.brokenCount ?? 0) > 0 ? 'warn' : undefined}>{(report?.missingCount ?? 0) + (report?.brokenCount ?? 0)}</strong><small>{t('xray.metric.notLoaded', { n: (report?.missingCount ?? 0) + (report?.brokenCount ?? 0) })}</small></div>
        <div className={styles.metric}><span>{t('xray.metric.knownContext')}</span><strong>{formatTokens(totalKnown)}</strong><small>{t('xray.metric.knownContextHint')}</small></div>
        <div className={styles.metric}><span>{t('xray.metric.mcp')}</span><strong>{mcps.length}</strong><small>{t('xray.metric.mcpHint')}</small></div>
      </div>

      <div className={styles.xrayGrid}>
        <section className={styles.xrayPanel}>
          <div className={styles.xrayPanelHead}><div><h2>{t('xray.effective.title')}</h2><p>{t('xray.effective.subtitle')}</p></div><span className={styles.xrayCount}>{t('xray.effective.count', { n: entries.length })}</span></div>
          <div className={styles.xrayFilters}><Input className="!h-8" value={query} onChange={(event) => setQuery(event.target.value)} placeholder={t('xray.filter.search')} aria-label={t('xray.filter.search')} /><div className={styles.xrayFilterChips}>{([['all', entries.length], [XrayStatus.XrayStatusActive, report?.activeCount ?? 0], [XrayStatus.XrayStatusCandidate, report?.candidateCount ?? 0], [XrayStatus.XrayStatusMissing, report?.missingCount ?? 0], [XrayStatus.XrayStatusBroken, report?.brokenCount ?? 0]] as const).map(([value, count]) => <button type="button" key={value} className={`${styles.xrayFilterChip} ${status === value ? styles.xrayFilterChipActive : ''}`} onClick={() => setStatus(value)}>{value === 'all' ? t('xray.filter.all') : statusLabel(value)}<b>{count}</b></button>)}</div></div>
          {entries.length === 0 ? <div className={styles.xrayState}><span className={styles.xrayStateIcon}><Sparkles size={17} aria-hidden="true" /></span><strong>{t('xray.empty')}</strong></div> : filtered.length === 0 ? <div className={styles.xrayState}><span className={styles.xrayStateIcon}><Sparkles size={17} aria-hidden="true" /></span><strong>{t('xray.filter.empty')}</strong></div> : <div className={styles.xrayList}>
            {filtered.map((entry) => <button type="button" key={entry.skill.id} className={`${styles.xrayRow} ${selected?.skill.id === entry.skill.id ? styles.xrayRowSelected : ''}`} onClick={() => setSelectedId(entry.skill.id)}>
              <SkillIcon skill={skillForIcon(entry.skill)} className="size-6" />
              <span className={styles.xrayRowMain}><span className={styles.xrayRowName}>{entry.skill.name}<StatusBadge status={entry.status} /></span><span className={styles.xrayRowPath}>{entry.skill.source || entry.skill.rootKind} · {entry.skill.path}</span></span>
              <span className={styles.xrayRowToken}>{entry.status === 'active' || entry.status === 'candidate' ? formatTokens(entry.skill.descTokens) : '—'}</span>
            </button>)}
          </div>}
        </section>

        <aside className={styles.xrayAside}>
          <section className={styles.xrayPanel}>
            <div className={styles.xrayPanelHead}><div><h2>{t('xray.budget.title')}</h2><p>{t('xray.budget.knownHint')}</p></div></div>
            <div className={styles.xrayBudget}>
              <div className={styles.xrayBudgetTotal}><strong>{formatTokens(totalKnown)}</strong><span>{t('xray.budget.known')}</span></div>
              <div className={styles.xrayStackBar} aria-hidden="true"><i className={styles.xrayStackSkills} style={{ width: totalKnown ? `${Math.max(4, skillTokens / totalKnown * 100)}%` : '0%' }} /><i className={styles.xrayStackInstructions} style={{ width: totalKnown ? `${Math.max(4, instructionTokens / totalKnown * 100)}%` : '0%' }} /></div>
              <div className={styles.xrayLegend}><span><i className={styles.xrayDotSkills} />{t('xray.budget.skills')}<b>{formatTokens(skillTokens)}</b></span><span><i className={styles.xrayDotInstructions} />{t('xray.budget.instructions')}<b>{formatTokens(instructionTokens)}</b></span></div>
              <p className={styles.xrayBudgetNote}>{t('xray.budget.unmodeled')}</p>
            </div>
          </section>

          <section className={styles.xrayPanel}>
            <div className={styles.xrayPanelHead}><div><h2>{t('xray.usage.title')}</h2><p>{t('xray.usage.subtitle')}</p></div></div>
            <div className={styles.xrayUsage}>
              {usageLoading ? <span className={styles.xrayUsageEmpty}>{t('xray.usage.loading')}</span> : usage.length === 0 ? <span className={styles.xrayUsageEmpty}>{t('xray.usage.empty')}</span> : usage.slice(0, 6).map((item) => {
                const skill = item.kind === 'skill' ? report?.entries?.find((entry) => entry.skill.name === item.name)?.skill : undefined
                const server = item.kind === 'mcp' ? mcps.find((entry) => entry.name === item.name) : undefined
                return <div key={`${item.kind}:${item.name}`} className={styles.xrayUsageRow}>
                  {skill ? <SkillIcon skill={skillForIcon(skill)} className="size-4" /> : server ? <MCPIcon name={server.name} command={server.command} url={server.url} className="size-4" /> : item.kind === 'skill' ? <Sparkles size={12} /> : <Server size={12} />}
                  <span>{item.name}</span><b>{item.count}×</b><time>{item.lastAt ? formatRelativeTime(item.lastAt) : ''}</time>
                </div>
              })}
            </div>
          </section>

          {selected && <section className={styles.xrayPanel}>
            <div className={styles.xrayPanelHead}><div className={styles.xrayWhyTitle}><SkillIcon skill={skillForIcon(selected.skill)} className="size-7" /><div><h2>{selected.status === 'active' ? t('xray.why.statusTitle') : t('xray.why.problemTitle')}</h2><p className={styles.xrayWhyNameLine}>{selected.skill.name}<StatusBadge status={selected.status} /></p></div></div></div>
            <div className={styles.xrayWhy}>
              <WhyBody entry={selected} mcps={mcps} />
              <div className={styles.xrayWhyActions}><button type="button" className={styles.xrayLinkButton} onClick={() => onOpenSkill(selected.skill.id, project || undefined)}>{t('xray.why.openSkill')} →</button>{onReveal && <button type="button" className={styles.xrayLinkButton} onClick={() => onReveal(selected.skill.id)}>{t('xray.why.reveal')} →</button>}</div>
            </div>
          </section>}
        </aside>
      </div>
    </>}

    {tab === 'compare' && <section className={styles.xrayPanel}>
      <div className={styles.xrayPanelHead}><div><h2>{t('xray.compare.title')}</h2><p>{t('xray.compare.subtitle')}</p></div><div className={styles.xrayCompareActions} ref={comparePickerRef}><span className={styles.xrayCount}>{t('xray.compare.count', { n: compareRows.length })}</span><button type="button" className={styles.xrayCompareButton} onClick={() => setCompareOpen((value) => !value)} aria-expanded={compareOpen} aria-haspopup="dialog">{t('xray.compare.choose')} <b>{displayedCompareIDs.length}/4</b></button>{compareOpen && <div className={styles.xrayCompareMenu} role="dialog" aria-label={t('xray.compare.choose')}><div className={styles.xrayCompareMenuHead}>{t('xray.compare.choose')}<small>{t('xray.compare.max', { n: 4 })}</small></div>{uniqueCompareOptions.map((option) => { const checked = displayedCompareIDs.includes(option.id); const current = option.id === root.id; return <label key={option.id} className={`${styles.xrayCompareOption} ${current ? styles.xrayCompareOptionCurrent : ''}`}><Checkbox checked={checked} disabled={current || (!checked && !canAddCompare)} onCheckedChange={() => toggleCompare(option.id)} /><AgentIcon name={option.logicalAgent || option.label} className="size-4" /><span>{option.label}</span>{current && <em>{t('xray.compare.current')}</em>}</label> })}</div>}</div></div>
      {compareRows.length === 0 ? <div className={styles.xrayState}><span className={styles.xrayStateIcon}><GitCompareArrows size={17} aria-hidden="true" /></span><strong>{t('xray.compare.empty')}</strong></div> : <div className={styles.xrayTableWrap}><table className={styles.xrayTable}><thead><tr><th>{t('xray.compare.resource')}</th>{columns.map(({ root: column }) => <th key={column.id}><span className={styles.xrayCompareAgent}><AgentIcon name={column.logicalAgent || column.label} className="size-4" /><span>{column.label}</span></span>{column.logicalAgent && column.logicalAgent !== column.label && <small>{column.logicalAgent}</small>}</th>)}</tr></thead><tbody>
        {compareRows.map((row) => <tr key={row.name}><td><span className={styles.xrayCompareResource}>{row.skill ? <SkillIcon skill={skillForIcon(row.skill)} className="size-5" /> : <SkillGlyph name={row.name} className="size-5" />}<span><strong>{row.name}</strong><small>{row.source || '—'}</small></span></span></td>{row.cells?.map((cell) => <td key={cell.root.id}><StatusBadge status={cell.status} /></td>)}</tr>)}
      </tbody></table></div>}
      <p className={styles.xrayTableNote}>{t('xray.compare.note')}</p>
    </section>}

    {tab === 'budget' && <>
      <div className={styles.xrayMetrics}>
        <div className={styles.metric}><span>{t('xray.budget.skills')}</span><strong>{formatTokens(skillTokens)}</strong><small>{t('xray.budget.skillsHint')}</small></div>
        <div className={styles.metric}><span>{t('xray.budget.instructions')}</span><strong>{formatTokens(instructionTokens)}</strong><small>{t('xray.budget.instructionsHint')}</small></div>
        <div className={styles.metric}><span>{t('xray.budget.mcp')}</span><strong>{mcps.length}</strong><small>{t('xray.budget.mcpHint')}</small></div>
        <div className={styles.metric}><span>{t('xray.budget.unknown')}</span><strong>—</strong><small>{t('xray.budget.unknownHint')}</small></div>
      </div>
      <div className={styles.xrayGrid}>
        <section className={styles.xrayPanel}>
          <div className={styles.xrayPanelHead}><div><h2>{t('xray.budget.composition')}</h2><p>{t('xray.budget.compositionHint')}</p></div></div>
          <div className={styles.xrayBudgetLarge}><div className={styles.xrayStackBarLarge} aria-hidden="true"><i className={styles.xrayStackSkills} style={{ width: totalKnown ? `${Math.max(4, skillTokens / totalKnown * 100)}%` : '0%' }} /><i className={styles.xrayStackInstructions} style={{ width: totalKnown ? `${Math.max(4, instructionTokens / totalKnown * 100)}%` : '0%' }} /></div><div className={styles.xrayLegendLarge}><span><i className={styles.xrayDotSkills} />{t('xray.budget.skills')} <b>{formatTokens(skillTokens)}</b></span><span><i className={styles.xrayDotInstructions} />{t('xray.budget.instructions')} <b>{formatTokens(instructionTokens)}</b></span><span className={styles.xrayUnknownLegend}><CircleHelp size={11} aria-hidden="true" />{t('xray.budget.unknown')}</span></div><p className={styles.xrayBudgetNote}>{t('xray.budget.unmodeled')}</p></div>
        </section>
        <aside className={styles.xrayAside}><section className={styles.xrayPanel}><div className={styles.xrayPanelHead}><div><h2>{t('xray.budget.contributors')}</h2><p>{t('xray.budget.top')}</p></div></div><div className={styles.xrayContributors}>{contributors.map((entry) => <div key={entry.skill.id} className={styles.xrayContributor}><span className={styles.xrayContributorName}><SkillIcon skill={skillForIcon(entry.skill)} className="size-5" /><span>{entry.skill.name}</span></span><div><i style={{ width: `${contributors[0]?.skill.descTokens ? Math.max(3, entry.skill.descTokens / contributors[0].skill.descTokens * 100) : 0}%` }} /></div><b>{formatTokens(entry.skill.descTokens)}</b></div>)}</div></section></aside>
      </div>
    </>}
  </div>
}
