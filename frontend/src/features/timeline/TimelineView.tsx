import { Badge } from '@/components/ui/badge'
import styles from '../../App.module.css'
import { formatTime } from '../../lib/format'
import type { TimelineEvent, UsageStat } from '../../types'

const labels: Record<string, string> = { added: '新增', changed: '内容变化', removed: '移除' }

function TimelineView({ events, usage, query = '' }: { events: TimelineEvent[]; usage: UsageStat[]; query?: string }) {
  const needle = query.trim().toLowerCase()
  const filteredEvents = !needle ? events : events.filter((event) => [event.skillName, event.path ?? '', event.kind].some((value) => value.toLowerCase().includes(needle)))
  return <section className={styles.historyContent}>
    <div className={styles.historyTableWrap}><table className={styles.historyTable}>
      <thead><tr><th>时间</th><th>变化</th><th>Skill</th><th>路径</th><th>旧 hash</th><th>新 hash</th></tr></thead>
      <tbody>{filteredEvents.length === 0 ? <tr><td colSpan={6} className={styles.historyEmptyRow}>{needle ? '没有匹配的历史记录。' : '下一次扫描发现新增、内容变化或移除时，会记录在这里。'}</td></tr> : filteredEvents.map((event) => <tr key={event.id}>
        <td><code>{formatTime(event.at) || event.at}</code></td>
        <td><Badge variant={event.kind === 'added' ? 'success' : event.kind === 'removed' ? 'error' : 'warning'}>{labels[event.kind] ?? event.kind}</Badge></td>
        <td><strong>{event.skillName}</strong></td>
        <td><code className={styles.historyPath} title={event.path}>{event.path || '—'}</code></td>
        <td><code>{event.oldHash ? event.oldHash.slice(0, 10) : '—'}</code></td>
        <td><code>{event.newHash ? event.newHash.slice(0, 10) : '—'}</code></td>
      </tr>)}</tbody>
    </table></div>
    {usage.length > 0 && <section className={styles.historySection}><div className={styles.historySectionHead}><h3>CLI 使用频率</h3><small>来自本机 zsh history</small></div><div className={styles.historyTableWrap}><table className={styles.historyTable}><thead><tr><th>命令</th><th className={styles.numeric}>次数</th></tr></thead><tbody>{usage.map((item) => <tr key={item.name}><td><strong>{item.name}</strong></td><td className={styles.numeric}>{item.count}</td></tr>)}</tbody></table></div></section>}
  </section>
}

export { TimelineView }
