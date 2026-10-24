import { useState } from 'react'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import styles from '../../App.module.css'
import { t } from '../../i18n'
import { backendErrorMessage } from '../../lib/backendError'
import { formatTime } from '../../lib/format'
import type { Operation } from '../../types'

export const statusKeys: Record<string, string> = { success: 'history.success', failed: 'history.failed', rolled_back: 'history.rolledBack' }
export const opKeys: Record<string, string> = { link_skill: 'common.repairLinks', replace_copy: 'common.useSharedVersion', sync_copy: 'common.syncContent', disable_link: 'history.disableVisibility', cli_update: 'history.cliUpdate', cli_uninstall: 'history.cliUninstall', skill_update: 'history.skillUpdate', install_skill: 'common.installSkill', uninstall_skill: 'common.uninstallSkill', mcp_remove: 'history.removeMcp', mcp_add: 'common.addMcp', mcp_update: 'history.editMcp', mcp_detach: 'history.disableMcp', mcp_attach: 'history.enableMcp', rollback: 'history.rollBack' }
export const statusLabel = (value: string) => t(statusKeys[value] ?? value)
export const opLabel = (value: string) => t(opKeys[value] ?? value)
const rollbackable = new Set(['link_skill', 'replace_copy', 'sync_copy', 'disable_link', 'skill_update', 'install_skill', 'uninstall_skill', 'mcp_remove'])

function OperationsView({ operations, onRollback, query = '', type = 'all', status = 'all' }: { operations: Operation[]; onRollback: (id: string) => Promise<void>; query?: string; type?: string; status?: string }) {
  const [target, setTarget] = useState<Operation>()
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')
  const needle = query.trim().toLowerCase()
  const filteredOperations = operations.filter((operation) => (type === 'all' || operation.operation === type) && (status === 'all' || operation.status === status) && (!needle || [opLabel(operation.operation), statusLabel(operation.status), operation.message ?? '', ...(operation.changes ?? [])].some((value) => value.toLowerCase().includes(needle))))
  const rollback = async () => {
    if (!target || busy) return
    setBusy(true); setError('')
    try { await onRollback(target.id); setTarget(undefined) } catch (cause) { setError(backendErrorMessage(cause, t('history.rollbackFailed'))) } finally { setBusy(false) }
  }
  return <section className={styles.historyContent}>
    {error && <div className={`${styles.banner} ${styles.bannerError}`} role="alert">{t('history.rollbackFailedWithMessage', { message: error })}</div>}
    <div className={styles.historyTableWrap}><table className={styles.historyTable}>
      <thead><tr><th>{t('common.time')}</th><th>{t('common.actions')}</th><th>{t('common.status')}</th><th>{t('history.changes')}</th><th>{t('history.details')}</th><th>{t('history.rollBack')}</th></tr></thead>
      <tbody>{filteredOperations.length === 0 ? <tr><td colSpan={6} className={styles.historyEmptyRow}>{needle || type !== 'all' || status !== 'all' ? t('history.noMatchingOperations') : t('history.noWriteOperationsYetSuccessfulRepairsUpdatesAndUninstallsAreRecordedHere')}</td></tr> : filteredOperations.map((op) => <tr key={op.id}>
        <td><code>{formatTime(op.createdAt) || '—'}</code></td>
        <td><strong>{opLabel(op.operation)}</strong></td>
        <td><Badge variant={op.status === 'success' ? 'success' : op.status === 'rolled_back' ? 'outline' : 'error'}>{statusLabel(op.status)}</Badge></td>
        <td>{op.changes?.length ? <details className={styles.opDetails}><summary>{t('common.changeCount', { n: op.changes.length })}</summary><ul>{op.changes.map((change) => <li key={change}><code>{change}</code></li>)}</ul></details> : '—'}</td>
        <td>{op.message ? <span className={styles.historyMessage} title={op.message}>{op.message}</span> : '—'}</td>
        <td>{rollbackable.has(op.operation) && op.status === 'success' ? <Button variant="outline" size="xs" onClick={() => { setError(''); setTarget(op) }}>{t('history.rollBack')}</Button> : '—'}</td>
      </tr>)}</tbody>
    </table></div>
    {target && <Dialog open onOpenChange={(open) => { if (!open) setTarget(undefined) }}>
      <DialogContent><DialogHeader><DialogTitle>{t('history.rollBackThisOperation')}</DialogTitle><DialogDescription>{t('history.onlyTheSnapshotLinkedToThisOperationIsRestoredSnapshotsAndRecordsAreKept')}</DialogDescription></DialogHeader><div className="p-6"><p className={styles.updateMessage}>{target.message}</p>{target.snapshotPath && <code className={styles.opSnapshot}>{target.snapshotPath}</code>}</div><DialogFooter><Button size="sm" variant="outline" onClick={() => setTarget(undefined)}>{t('common.cancel')}</Button><Button size="sm" variant="destructive" disabled={busy} onClick={() => void rollback()}>{busy ? t('history.rollingBack') : t('history.confirmRollback')}</Button></DialogFooter></DialogContent>
    </Dialog>}
  </section>
}

export { OperationsView }
