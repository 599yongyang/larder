import { useState } from 'react'
import { RotateCcw, Trash2 } from 'lucide-react'
import { backend } from '../../backend'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import styles from '../../App.module.css'
import { t } from '../../i18n'
import { formatTime } from '../../lib/format'
import { backendErrorMessage } from '../../lib/backendError'
import { toastError, toastSuccess } from '../../lib/toast'
import type { Operation } from '../../types'

function deletedPath(operation: Operation) {
  const prefix = operation.operation === 'uninstall_skill' ? 'uninstalled:' : 'removed:'
  const change = operation.changes?.find((value) => value.startsWith(prefix))
  return change?.slice(prefix.length) || operation.message || '—'
}

function labelForPath(path: string) {
  return path.split('/').filter(Boolean).pop() || path
}

export function TrashView({ operations, onRollback, onRefresh }: { operations: Operation[]; onRollback: (id: string) => Promise<void>; onRefresh?: () => Promise<void> }) {
  const [target, setTarget] = useState<Operation>()
  const [busyID, setBusyID] = useState<string>()
  const [error, setError] = useState('')
  const items = operations.filter((operation) => (operation.operation === 'uninstall_skill' || operation.operation === 'mcp_remove') && operation.status === 'success')
  const restore = async (operation: Operation) => {
    if (busyID) return
    setBusyID(operation.id); setError('')
    try {
      await onRollback(operation.id)
      toastSuccess(t('history.restored'), labelForPath(deletedPath(operation)))
    } catch (cause) {
      toastError(cause, t('history.restoreFailed'))
    } finally {
      setBusyID(undefined)
    }
  }
  const runPurge = async () => {
    if (!target || busyID) return
    setBusyID(target.id); setError('')
    try {
      await backend.purgeTrash(target.id)
      await onRefresh?.()
      setTarget(undefined)
    } catch (cause) {
      setError(backendErrorMessage(cause, t('history.purgeFailed')))
    } finally {
      setBusyID(undefined)
    }
  }
  return <section className={styles.historyContent}>
    <p className={styles.trashHint}>{t('history.trashHint')}</p>
    {error && <div className={`${styles.banner} ${styles.bannerError}`} role="alert">{error}</div>}
    <div className={styles.historyTableWrap}><table className={styles.historyTable}>
      <thead><tr><th>{t('common.name')}</th><th>{t('common.path')}</th><th>{t('history.deletedAt')}</th><th>{t('history.details')}</th><th>{t('history.restore')}</th></tr></thead>
      <tbody>{items.length === 0 ? <tr><td colSpan={5} className={styles.historyEmptyRow}>{t('history.trashEmpty')}</td></tr> : items.map((operation) => {
        const path = deletedPath(operation)
        return <tr key={operation.id}>
          <td><strong>{labelForPath(path)}</strong><Badge variant="outline">{operation.operation === 'mcp_remove' ? 'MCP' : 'Skill'}</Badge></td>
          <td><code className={styles.trashPath}>{path}</code></td>
          <td><code>{formatTime(operation.createdAt) || '—'}</code></td>
          <td><span className={styles.historyMessage}>{operation.message || '—'}</span></td>
          <td><span className={styles.trashActions}><Button variant="outline" size="xs" disabled={Boolean(busyID)} onClick={() => void restore(operation)}><RotateCcw size={12} />{t('history.restore')}</Button><Button variant="ghost" size="xs" className={styles.dangerAction} onClick={() => { setError(''); setTarget(operation) }}><Trash2 size={12} />{t('history.purge')}</Button></span></td>
        </tr>
      })}</tbody>
    </table></div>
    {target && <Dialog open onOpenChange={(open) => { if (!open) setTarget(undefined) }}>
      <DialogContent><DialogHeader><DialogTitle>{t('history.purgeTrash')}</DialogTitle><DialogDescription>{t('history.purgeTrashDescription')}</DialogDescription></DialogHeader><div className="p-6"><p className={styles.updateMessage}>{deletedPath(target)}</p><code className={styles.opSnapshot}>{target.snapshotPath || '—'}</code>{error && <p className={styles.planError} role="alert">{error}</p>}</div><DialogFooter><Button size="sm" variant="outline" onClick={() => setTarget(undefined)} disabled={Boolean(busyID)}>{t('common.cancel')}</Button><Button size="sm" variant="destructive" disabled={Boolean(busyID)} onClick={() => void runPurge()}>{busyID ? t('history.working') : t('history.purge')}</Button></DialogFooter></DialogContent>
    </Dialog>}
  </section>
}

export { labelForPath }
