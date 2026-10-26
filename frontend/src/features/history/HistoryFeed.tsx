import { Fragment, useMemo, useState } from 'react'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import styles from '../../App.module.css'
import { t } from '../../i18n'
import { backendErrorMessage } from '../../lib/backendError'
import { toastError, toastSuccess } from '../../lib/toast'
import { formatTime, shortPath } from '../../lib/format'
import type { Operation, TimelineEvent, UsageStat } from '../../types'

export const operationStatusKeys: Record<string, string> = { success: 'history.success', failed: 'history.failed', rolled_back: 'history.rolledBack', deleted: 'history.deleted' }
export const operationKeys: Record<string, string> = { link_skill: 'common.repairLinks', replace_copy: 'common.useSharedVersion', sync_copy: 'common.syncContent', disable_link: 'history.disableVisibility', cli_update: 'history.cliUpdate', cli_uninstall: 'history.cliUninstall', skill_update: 'history.skillUpdate', install_skill: 'common.installSkill', uninstall_skill: 'common.uninstallSkill', mcp_remove: 'history.removeMcp', mcp_add: 'common.addMcp', mcp_update: 'history.editMcp', mcp_detach: 'history.disableMcp', mcp_attach: 'history.enableMcp', rollback: 'history.rollBack', command_create: 'history.commandCreate', command_update: 'history.commandUpdate', command_convert: 'history.commandConvert' }
const rollbackable = new Set(['link_skill', 'replace_copy', 'sync_copy', 'disable_link', 'skill_update', 'install_skill', 'uninstall_skill', 'mcp_remove', 'rollback', 'command_create', 'command_update', 'command_convert'])
const prefixes = ['installed:', 'uninstalled:', 'updated:', 'removed:', 'changed:', 'added:', 'command:']
const operationLabel = (value: string) => t(operationKeys[value] ?? value)
const operationSummary = (operation: Operation) => operation.operation.startsWith('command_') ? t('history.commandWritten') : (operation.message ?? '')
const statusLabel = (value: string) => t(operationStatusKeys[value] ?? value)
const eventLabel = (value: string) => t(value === 'added' ? 'history.added' : value === 'removed' ? 'common.removed' : 'history.contentChanged')
const canRollback = (operation?: Operation) => Boolean(operation && operation.status === 'success' && rollbackable.has(operation.operation))

function pathsOf(operation: Operation) {
  return (operation.changes ?? []).map((change) => {
    const prefix = prefixes.find((value) => change.startsWith(value))
    return prefix ? change.slice(prefix.length) : ''
  }).filter(Boolean)
}

function targetName(path: string) {
  return path.split('/').filter(Boolean).pop() || path
}

function OperationTarget({ operation }: { operation: Operation }) {
  const paths = pathsOf(operation)
  if (paths.length === 0) return <span className={styles.operationMuted}>—</span>
  return <span className={styles.operationTarget} title={paths.join('\n')}><strong>{targetName(paths[0])}</strong><small>{paths.length > 1 ? t('history.targetCount', { n: paths.length }) : shortPath(paths[0])}</small></span>
}

function OperationDetail({ operation, onClose, onRollback }: { operation: Operation; onClose: () => void; onRollback: (id: string) => Promise<Operation> }) {
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')
  const rollback = async () => {
    if (busy) return
    setBusy(true); setError('')
    try {
      const restored = await onRollback(operation.id)
      onClose()
      toastSuccess(t('history.restored'), operationLabel(operation.operation), {
        label: t('skills.undo'),
        onClick: () => { void onRollback(restored.id).then(() => toastSuccess(t('history.restored'))).catch((cause) => toastError(cause, t('history.rollbackFailed'))) },
      })
    } catch (cause) {
      setError(backendErrorMessage(cause, t('history.rollbackFailed')))
    } finally {
      setBusy(false)
    }
  }
  const paths = pathsOf(operation)
  return <Dialog open onOpenChange={(open) => { if (!open && !busy) onClose() }}>
    <DialogContent className="flex max-h-[82vh] max-w-[680px] flex-col gap-0 overflow-hidden p-0">
      <DialogHeader className={styles.operationDetailHeader}>
        <div className="flex items-center gap-2"><DialogTitle className="text-base">{operationLabel(operation.operation)}</DialogTitle><Badge variant={operation.status === 'success' ? 'success' : operation.status === 'rolled_back' ? 'outline' : 'error'}>{statusLabel(operation.status)}</Badge></div>
        <DialogDescription className={styles.operationDetailMeta}>{formatTime(operation.createdAt) || '—'} · {operation.id}</DialogDescription>
      </DialogHeader>
      <div className="min-h-0 flex-1 overflow-y-auto">
        <div className={styles.operationDetailBody}>
          <dl className={styles.operationDefinition}>
            <dt>{t('history.operationType')}</dt><dd>{operationLabel(operation.operation)}</dd>
            <dt>{t('history.object')}</dt><dd>{paths.length ? <span className={styles.operationDetailPaths}>{paths.map((path) => <code key={path}>{path}</code>)}</span> : '—'}</dd>
            <dt>{t('history.summary')}</dt><dd>{operationSummary(operation) || '—'}</dd>
            <dt>{t('history.snapshot')}</dt><dd>{operation.snapshotPath ? <code>{shortPath(operation.snapshotPath)}</code> : '—'}</dd>
          </dl>
          {(operation.changes ?? []).length > 0 && <section className={styles.operationChanges}><h3>{t('history.changes')} <span>{operation.changes?.length ?? 0}</span></h3><ul>{(operation.changes ?? []).map((change) => <li key={change}><code>{change}</code></li>)}</ul></section>}
          {error && <p className={styles.planError} role="alert">{error}</p>}
          {canRollback(operation) && <p className={styles.operationRollbackHint}>{t('history.rollbackDirectHint')}</p>}
        </div>
      </div>
      <DialogFooter className="border-t px-5 py-3"><Button size="sm" variant="outline" onClick={onClose} disabled={busy}>{t('common.close')}</Button>{canRollback(operation) && <Button size="sm" variant="destructive" disabled={busy} onClick={() => void rollback()}>{busy ? t('history.rollingBack') : t('history.rollBack')}</Button>}</DialogFooter>
    </DialogContent>
  </Dialog>
}

export function HistoryFeed({ events, operations, usage, query = '', onRollback }: { events: TimelineEvent[]; operations: Operation[]; usage: UsageStat[]; query?: string; onRollback: (id: string) => Promise<Operation> }) {
  const [detail, setDetail] = useState<Operation>()
  const [eventDetail, setEventDetail] = useState<number>()
  const needle = query.trim().toLowerCase()

  const operationPaths = useMemo(() => operations.filter((operation) => operation.status === 'success').flatMap((operation) => {
    const at = Date.parse(operation.createdAt)
    return pathsOf(operation).map((path) => ({ path, at }))
  }), [operations])
  const externalEvents = useMemo(() => events.filter((event) => {
    const at = Date.parse(event.at)
    return !operationPaths.some((operation) => operation.path === event.path && Number.isFinite(operation.at) && Number.isFinite(at) && Math.abs(operation.at - at) < 5 * 60 * 1000)
  }), [events, operationPaths])
  const feed = useMemo(() => {
    const rows: Array<{ key: string; at: number; kind: 'operation'; operation: Operation } | { key: string; at: number; kind: 'event'; event: TimelineEvent }> = [
      ...operations.map((operation) => ({ key: `op:${operation.id}`, at: Date.parse(operation.createdAt), kind: 'operation' as const, operation })),
      ...externalEvents.map((event) => ({ key: `ev:${event.id}`, at: Date.parse(event.at), kind: 'event' as const, event })),
    ]
    return rows.filter((row) => {
      if (!needle) return true
      const haystack = row.kind === 'operation' ? [operationLabel(row.operation.operation), statusLabel(row.operation.status), operationSummary(row.operation), ...pathsOf(row.operation)] : [row.event.skillName, row.event.path ?? '', eventLabel(row.event.kind)]
      return haystack.some((value) => value.toLowerCase().includes(needle))
    }).sort((a, b) => b.at - a.at)
  }, [operations, externalEvents, needle])

  return <section className={styles.historyContent}>
    <div className={styles.historyTableWrap}><table className={styles.historyTable}>
      <thead><tr><th>{t('common.time')}</th><th>{t('history.recordType')}</th><th>{t('history.object')}</th><th>{t('history.summary')}</th><th>{t('common.status')}</th><th>{t('history.details')}</th></tr></thead>
      <tbody>{feed.length === 0 ? <tr><td colSpan={6} className={styles.historyEmptyRow}>{needle ? t('history.noMatchingHistory') : t('history.noHistoryYet')}</td></tr> : feed.map((row) => row.kind === 'operation' ? <tr key={row.key} className={styles.clickableRow} tabIndex={0} onClick={() => setDetail(row.operation)} onKeyDown={(event) => { if (event.key === 'Enter' || event.key === ' ') { event.preventDefault(); setDetail(row.operation) } }}>
        <td><code>{formatTime(row.operation.createdAt) || '—'}</code></td>
        <td><Badge variant="outline">{operationLabel(row.operation.operation)}</Badge></td>
        <td><OperationTarget operation={row.operation} /></td>
        <td><span className={styles.historyMessage} title={row.operation.message}>{row.operation.message || '—'}</span></td>
        <td><Badge variant={row.operation.status === 'success' ? 'success' : row.operation.status === 'rolled_back' ? 'outline' : 'error'}>{statusLabel(row.operation.status)}</Badge></td>
        <td><Button variant="ghost" size="xs" onClick={(event) => { event.stopPropagation(); setDetail(row.operation) }}>{t('history.viewDetails')}</Button></td>
      </tr> : <Fragment key={row.key}>
        <tr className={styles.clickableRow} onClick={() => setEventDetail(row.event.id === eventDetail ? undefined : row.event.id)}>
          <td><code>{formatTime(row.event.at) || row.event.at}</code></td>
          <td><Badge variant={row.event.kind === 'added' ? 'success' : row.event.kind === 'removed' ? 'error' : 'warning'}>{eventLabel(row.event.kind)}</Badge></td>
          <td><strong>{row.event.skillName}</strong></td>
          <td><span className={styles.historyPath} title={row.event.path}>{row.event.path ? shortPath(row.event.path) : '—'}</span></td>
          <td>—</td>
          <td><Button variant="ghost" size="xs" onClick={() => setEventDetail(row.event.id === eventDetail ? undefined : row.event.id)}>{t('history.viewDetails')}</Button></td>
        </tr>
        {eventDetail === row.event.id && <tr><td colSpan={6} className={styles.historyEventDetails}><dl><dt>{t('common.path')}</dt><dd><code>{row.event.path || '—'}</code></dd><dt>{t('history.oldHash')}</dt><dd><code>{row.event.oldHash || '—'}</code></dd><dt>{t('history.newHash')}</dt><dd><code>{row.event.newHash || '—'}</code></dd></dl></td></tr>}
      </Fragment>)}</tbody>
    </table></div>
    {usage.length > 0 && <section className={styles.historySection}><div className={styles.historySectionHead}><h3>{t('history.cliUsage')}</h3><small>{t('history.fromLocalZshHistory')}</small></div><div className={styles.historyTableWrap}><table className={styles.historyTable}><thead><tr><th>{t('common.commandLabel')}</th><th className={styles.numeric}>{t('history.uses')}</th></tr></thead><tbody>{usage.map((item) => <tr key={item.name}><td><strong>{item.name}</strong></td><td className={styles.numeric}>{item.count}</td></tr>)}</tbody></table></div></section>}
    {detail && <OperationDetail operation={detail} onClose={() => setDetail(undefined)} onRollback={onRollback} />}
  </section>
}
