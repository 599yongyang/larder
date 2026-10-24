import { useState } from 'react'
import { Copy } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { AssetSearch } from '@/components/AssetSearch'
import { backend } from '../../backend'
import styles from '../../App.module.css'
import { toastError, toastSuccess } from '../../lib/toast'
import { OperationsView, opLabel, statusLabel } from '../operations/OperationsView'
import { TimelineView } from '../timeline/TimelineView'
import type { Operation, TimelineEvent, UsageStat } from '../../types'

function HistoryView({ events, usage, operations, onRollback }: { events: TimelineEvent[]; usage: UsageStat[]; operations: Operation[]; onRollback: (id: string) => Promise<void> }) {
  const [tab, setTab] = useState<'timeline' | 'operations'>('timeline')
  const [query, setQuery] = useState('')
  const [opType, setOpType] = useState('all')
  const [opStatus, setOpStatus] = useState('all')
  const copyReport = async () => {
    try { await navigator.clipboard.writeText(await backend.exportReport()); toastSuccess('报告已复制') } catch (error) { toastError(error, '复制报告失败') }
  }
  return <div className={styles.historyPage}>
    <div className={styles.historyToolbar}>
      <div className={styles.segmentedControl} role="group" aria-label="历史视图">
        <Button variant={tab === 'timeline' ? 'secondary' : 'ghost'} size="xs" onClick={() => setTab('timeline')}>变化时间线<span className={styles.tabCount}>{events.length}</span></Button>
        <Button variant={tab === 'operations' ? 'secondary' : 'ghost'} size="xs" onClick={() => setTab('operations')}>操作记录<span className={styles.tabCount}>{operations.length}</span></Button>
      </div>
      <div className={styles.historyToolbarActions}><AssetSearch value={query} onChange={setQuery} placeholder="搜索历史" label="搜索历史" width={170} />{tab === 'operations' && <><Select value={opType} onValueChange={setOpType}><SelectTrigger size="sm" className="w-[120px]" aria-label="操作类型"><SelectValue /></SelectTrigger><SelectContent><SelectItem value="all">全部操作</SelectItem>{Array.from(new Set(operations.map((op) => op.operation))).map((item) => <SelectItem key={item} value={item}>{opLabel[item] ?? item}</SelectItem>)}</SelectContent></Select><Select value={opStatus} onValueChange={setOpStatus}><SelectTrigger size="sm" className="w-[110px]" aria-label="操作状态"><SelectValue /></SelectTrigger><SelectContent><SelectItem value="all">全部状态</SelectItem>{Object.entries(statusLabel).map(([value, label]) => <SelectItem key={value} value={value}>{label}</SelectItem>)}</SelectContent></Select></>}{tab === 'timeline' && <Button variant="outline" size="xs" onClick={() => void copyReport()}><Copy size={12} aria-hidden="true" />复制报告</Button>}</div>
    </div>
    <div className={styles.historyBody}>{tab === 'timeline' ? <TimelineView events={events} usage={usage} query={query} /> : <OperationsView operations={operations} onRollback={onRollback} query={query} type={opType} status={opStatus} />}</div>
  </div>
}

export { HistoryView }
