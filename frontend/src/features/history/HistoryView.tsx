import { copyText as writeClipboard } from '../../lib/clipboard'
import { useState } from 'react'
import { Copy } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { AssetSearch } from '../../components/AssetSearch'
import { backend } from '../../backend'
import styles from '../../App.module.css'
import { t } from '../../i18n'
import { toastError, toastSuccess } from '../../lib/toast'
import { HistoryFeed } from './HistoryFeed'
import { TrashView } from './TrashView'
import type { Operation, TimelineEvent, UsageStat } from '../../types'

function HistoryView({ events, usage, operations, onRollback, onRefresh }: { events: TimelineEvent[]; usage: UsageStat[]; operations: Operation[]; onRollback: (id: string) => Promise<Operation>; onRefresh?: () => Promise<void> }) {
  const [tab, setTab] = useState<'history' | 'trash'>('history')
  const [query, setQuery] = useState('')
  const trashCount = operations.filter((operation) => (operation.operation === 'uninstall_skill' || operation.operation === 'mcp_remove') && operation.status === 'success').length
  const copyReport = async () => {
    try { await writeClipboard(await backend.exportReport()); toastSuccess(t('history.reportCopied')) } catch (error) { toastError(error, t('history.failedToCopyReport')) }
  }
  return <div className={styles.historyPage}>
    <div className={styles.historyToolbar}>
      <div className={styles.segmentedControl} role="group" aria-label={t('history.historyView')}>
        <Button variant={tab === 'history' ? 'secondary' : 'ghost'} size="xs" onClick={() => setTab('history')}>{t('history.history')}</Button>
        <Button variant={tab === 'trash' ? 'secondary' : 'ghost'} size="xs" onClick={() => setTab('trash')}>{t('history.trash')}<span className={styles.tabCount}>{trashCount}</span></Button>
      </div>
      <div className={styles.historyToolbarActions}><AssetSearch value={query} onChange={setQuery} placeholder={t('history.searchHistory')} label={t('history.searchHistory')} width={170} />{tab === 'history' && <Button variant="outline" size="xs" onClick={() => void copyReport()}><Copy size={12} aria-hidden="true" />{t('history.copyReport')}</Button>}</div>
    </div>
    <div className={styles.historyBody}>{tab === 'history' ? <HistoryFeed events={events} operations={operations} usage={usage} query={query} onRollback={onRollback} /> : <TrashView operations={operations} onRollback={onRollback} onRefresh={onRefresh} />}</div>
  </div>
}

export { HistoryView }
