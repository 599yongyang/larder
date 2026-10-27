import { useEffect, useMemo, useState } from 'react'
import { Loader2 } from 'lucide-react'
import { ModelIcon } from '../../components/ModelIcon'
import { AssetSearch } from '../../components/AssetSearch'
import { SessionDetailSheet } from '../../components/SessionDetailSheet'
import { SessionQuickActions } from '../../components/SessionQuickActions'
import { Button } from '@/components/ui/button'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { formatCompactNumber, formatRelativeTime } from '../../lib/format'
import { t } from '../../i18n'
import styles from '../../App.module.css'
import viewStyles from './AgentSessionsView.module.css'
import type { Session } from '../../types'

export function AgentSessionsView({ sessions, loading, error, onRetry, onOpenProject }: { sessions: Session[]; loading: boolean; error?: boolean; onRetry?: () => void; onOpenProject?: (path: string) => void }) {
  const [selected, setSelected] = useState<Session>()
  const [query, setQuery] = useState('')
  const [project, setProject] = useState('all')
  const [range, setRange] = useState<'all' | 'today' | '7d' | '30d'>('all')
  const [visibleCount, setVisibleCount] = useState(50)
  const timeFiltered = useMemo(() => {
    const cutoff = range === 'today' ? startOfDay() : range === '7d' ? Date.now() - 7 * 86400000 : range === '30d' ? Date.now() - 30 * 86400000 : 0
    return cutoff ? sessions.filter((session) => Date.parse(session.lastAt) >= cutoff) : sessions
  }, [sessions, range])
  const projects = useMemo(() => Array.from(new Set(timeFiltered.map((session) => session.cwd).filter(Boolean))).sort(), [timeFiltered])
  const filtered = useMemo(() => {
    const needle = query.trim().toLowerCase()
    return timeFiltered.filter((session) => {
      if (project !== 'all' && session.cwd !== project) return false
      if (!needle) return true
      return [session.title, session.id, session.cwd, session.model].some((value) => (value ?? '').toLowerCase().includes(needle))
    })
  }, [timeFiltered, query, project])
  useEffect(() => { setVisibleCount(50) }, [query, project, range])
  const visible = filtered.slice(0, visibleCount)
  const groups = useMemo(() => groupSessions(visible), [visible])
  const hasMore = filtered.length > visible.length
  const onCloseDetail = () => setSelected(undefined)
  const messageCount = timeFiltered.reduce((sum, session) => sum + session.messages, 0)
  const tokenCount = timeFiltered.reduce((sum, session) => sum + session.tokensIn + session.tokensOut, 0)
  const hasTokenData = timeFiltered.some((session) => session.tokensIn + session.tokensOut > 0)
  const sessionCountLabel = sessions.length >= 500 ? '500+' : formatCompactNumber(timeFiltered.length)
  return <section className={`${styles.ledgerPanel} ${viewStyles.panel}`}>
    <div className={styles.metrics}>
      <SessionMetric label={t('sessions.totalSessions')} value={sessionCountLabel} detail={timeFiltered[0] ? formatRelativeTime(timeFiltered[0].lastAt) : t('sessions.noActivity')} />
      <SessionMetric label={t('sessions.messages')} value={formatCompactNumber(messageCount)} detail={timeFiltered.length ? t('sessions.averageMessages', { n: Math.round(messageCount / timeFiltered.length) }) : t('sessions.noMessages')} />
      <SessionMetric label={t('sessions.projects')} value={projects.length.toLocaleString()} detail={t('sessions.acrossProjects')} />
      <SessionMetric label={t('sessions.tokens')} value={hasTokenData ? formatCompactNumber(tokenCount) : '—'} detail={hasTokenData ? t('sessions.inputAndOutput') : t('sessions.noTokenData')} />
    </div>
    <div className={`${styles.filterBar} ${viewStyles.toolbar}`}>
      <AssetSearch value={query} onChange={setQuery} placeholder={t('sessions.searchPlaceholder')} label={t('sessions.searchPlaceholder')} width={260} />
      <Select value={project} onValueChange={setProject}><SelectTrigger size="sm" className="w-[230px]"><SelectValue /></SelectTrigger><SelectContent className="max-h-[240px]"><SelectItem value="all">{t('sessions.allProjects')}</SelectItem>{projects.map((value) => <SelectItem key={value} value={value}>{projectLabel(value)}</SelectItem>)}</SelectContent></Select>
      <Select value={range} onValueChange={(value) => setRange(value as 'all' | 'today' | '7d' | '30d')}><SelectTrigger size="sm" className="w-[125px]"><SelectValue /></SelectTrigger><SelectContent><SelectItem value="all">{t('sessions.allTime')}</SelectItem><SelectItem value="today">{t('sessions.today')}</SelectItem><SelectItem value="7d">{t('sessions.last7Days')}</SelectItem><SelectItem value="30d">{t('sessions.last30Days')}</SelectItem></SelectContent></Select>
      <span className={styles.resultCount}>{t('sessions.resultCount', { shown: visible.length, total: filtered.length })}</span>
    </div>
    {loading ? <div className={viewStyles.loading} role="status" aria-live="polite"><Loader2 size={18} className="animate-spin" aria-hidden="true" /><strong>{t('project.loadingSessions')}</strong><span>{t('sessions.loadingHint')}</span></div> : error ? <div className={viewStyles.empty} role="alert"><strong>{t('sessions.loadFailed')}</strong><span>{t('sessions.loadFailedHint')}</span>{onRetry && <Button size="sm" variant="outline" onClick={onRetry}>{t('common.retry')}</Button>}</div> : groups.length === 0 ? <div className={viewStyles.empty}><strong>{sessions.length === 0 ? t('agent.noSessionsTitle') : t('sessions.noMatchingSessions')}</strong><span>{sessions.length === 0 ? t('agent.noSessionsHint') : t('sessions.clearFiltersHint')}</span></div> : <div className={viewStyles.list}>
      {groups.map((group) => <section key={group.key} className={viewStyles.group}>
        <h4 className={viewStyles.groupTitle}>{group.label}<span>{group.sessions.length}</span></h4>
        {group.sessions.map((session) => <div key={`${session.agent}:${session.id}`} role="button" tabIndex={0} className={viewStyles.row} title={session.path} onClick={() => setSelected(session)} onKeyDown={(event) => { if (event.key === 'Enter' || event.key === ' ') { event.preventDefault(); setSelected(session) } }}>
          <span className={viewStyles.titleRow}>
            <span className={viewStyles.title}>{session.title || session.id}</span>
            <span className={viewStyles.titleActions}><span className={viewStyles.time}>{formatRelativeTime(session.lastAt)}</span><SessionQuickActions session={session} /></span>
          </span>
          <span className={viewStyles.meta}>
            <span className={viewStyles.project}>{session.cwd ? projectLabel(session.cwd) : t('agent.unknownProject')}</span>
            <span>{t('project.sessionMessages', { n: formatCompactNumber(session.messages) })}</span>
            {session.tokensIn + session.tokensOut > 0 && <span>{formatCompactNumber(session.tokensIn + session.tokensOut)} {t('sessions.tokens')}</span>}
            {session.model && <span className={viewStyles.modelMeta}>
              <ModelIcon model={session.model} className="size-3" />
              <span>{session.model}</span>
            </span>}
          </span>
        </div>)}
      </section>)}
    </div>}
    {hasMore && <div className={viewStyles.loadMore}><Button variant="outline" size="sm" onClick={() => setVisibleCount((value) => value + 50)}>{t('sessions.loadMore', { n: Math.min(50, filtered.length - visible.length) })}</Button></div>}
    {selected && <SessionDetailSheet session={selected} onClose={() => setSelected(undefined)} onOpenProject={selected.cwd && onOpenProject ? () => { onCloseDetail(); onOpenProject(selected.cwd) } : undefined} />}
  </section>
}

function SessionMetric({ label, value, detail }: { label: string; value: string; detail: string }) {
  return <div className={styles.metric}><span>{label}</span><strong>{value}</strong><small>{detail}</small></div>
}

function groupSessions(sessions: Session[]) {
  const today = startOfDay()
  const yesterday = today - 86400000
  const week = today - 7 * 86400000
  const groups = [
    { key: 'today', label: t('sessions.today'), sessions: [] as Session[] },
    { key: 'yesterday', label: t('sessions.yesterday'), sessions: [] as Session[] },
    { key: 'week', label: t('sessions.thisWeek'), sessions: [] as Session[] },
    { key: 'earlier', label: t('sessions.earlier'), sessions: [] as Session[] },
  ]
  for (const session of sessions) {
    const at = Date.parse(session.lastAt)
    const bucket = at >= today ? groups[0] : at >= yesterday ? groups[1] : at >= week ? groups[2] : groups[3]
    bucket.sessions.push(session)
  }
  return groups.filter((group) => group.sessions.length > 0)
}

function startOfDay() {
  const date = new Date()
  date.setHours(0, 0, 0, 0)
  return date.getTime()
}

function projectLabel(path: string) {
  const parts = path.split('/').filter(Boolean)
  return parts.slice(-2).join('/') || path
}
