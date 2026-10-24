import { Badge } from '@/components/ui/badge'
import styles from '../../App.module.css'
import { t } from '../../i18n'
import { formatTime } from '../../lib/format'
import type { TimelineEvent, UsageStat } from '../../types'

function TimelineView({ events, usage, query = '' }: { events: TimelineEvent[]; usage: UsageStat[]; query?: string }) {
  const labels: Record<string, string> = { added: t('history.added'), changed: t('history.contentChanged'), removed: t('common.removed') }
  const needle = query.trim().toLowerCase()
  const filteredEvents = !needle ? events : events.filter((event) => [event.skillName, event.path ?? '', event.kind].some((value) => value.toLowerCase().includes(needle)))
  return <section className={styles.historyContent}>
    <div className={styles.historyTableWrap}><table className={styles.historyTable}>
      <thead><tr><th>{t('common.time')}</th><th>{t('history.change')}</th><th>Skill</th><th>{t('common.path')}</th><th>{t('history.oldHash')}</th><th>{t('history.newHash')}</th></tr></thead>
      <tbody>{filteredEvents.length === 0 ? <tr><td colSpan={6} className={styles.historyEmptyRow}>{needle ? t('history.noMatchingHistory') : t('history.additionsContentChangesAndRemovalsFoundByTheNextScanAreRecordedHere')}</td></tr> : filteredEvents.map((event) => <tr key={event.id}>
        <td><code>{formatTime(event.at) || event.at}</code></td>
        <td><Badge variant={event.kind === 'added' ? 'success' : event.kind === 'removed' ? 'error' : 'warning'}>{labels[event.kind] ?? event.kind}</Badge></td>
        <td><strong>{event.skillName}</strong></td>
        <td><code className={styles.historyPath} title={event.path}>{event.path || '—'}</code></td>
        <td><code>{event.oldHash ? event.oldHash.slice(0, 10) : '—'}</code></td>
        <td><code>{event.newHash ? event.newHash.slice(0, 10) : '—'}</code></td>
      </tr>)}</tbody>
    </table></div>
    {usage.length > 0 && <section className={styles.historySection}><div className={styles.historySectionHead}><h3>{t('history.cliUsage')}</h3><small>{t('history.fromLocalZshHistory')}</small></div><div className={styles.historyTableWrap}><table className={styles.historyTable}><thead><tr><th>{t('common.commandLabel')}</th><th className={styles.numeric}>{t('history.uses')}</th></tr></thead><tbody>{usage.map((item) => <tr key={item.name}><td><strong>{item.name}</strong></td><td className={styles.numeric}>{item.count}</td></tr>)}</tbody></table></div></section>}
  </section>
}

export { TimelineView }
