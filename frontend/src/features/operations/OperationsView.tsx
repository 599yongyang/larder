import { useState } from 'react'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import styles from '../../App.module.css'
import { formatTime } from '../../lib/format'
import type { Operation } from '../../types'

export const statusLabel: Record<string, string> = { success: '成功', failed: '失败', rolled_back: '已回滚' }
export const opLabel: Record<string, string> = { link_skill: '修复链接', replace_copy: '改用共享版本', sync_copy: '同步内容', disable_link: '停用可见性', cli_update: 'CLI 更新', cli_uninstall: 'CLI 卸载', skill_update: 'Skill 更新', install_skill: '安装 Skill', uninstall_skill: '卸载 Skill', mcp_remove: '移除 MCP', mcp_add: '添加 MCP', mcp_update: '编辑 MCP', mcp_detach: '停用 MCP', mcp_attach: '启用 MCP', rollback: '回滚' }
const rollbackable = new Set(['link_skill', 'replace_copy', 'sync_copy', 'disable_link', 'skill_update', 'install_skill', 'uninstall_skill', 'mcp_remove'])

function OperationsView({ operations, onRollback, query = '', type = 'all', status = 'all' }: { operations: Operation[]; onRollback: (id: string) => Promise<void>; query?: string; type?: string; status?: string }) {
  const [target, setTarget] = useState<Operation>()
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')
  const needle = query.trim().toLowerCase()
  const filteredOperations = operations.filter((operation) => (type === 'all' || operation.operation === type) && (status === 'all' || operation.status === status) && (!needle || [opLabel[operation.operation] ?? operation.operation, statusLabel[operation.status] ?? operation.status, operation.message ?? '', ...(operation.changes ?? [])].some((value) => value.toLowerCase().includes(needle))))
  const rollback = async () => {
    if (!target || busy) return
    setBusy(true); setError('')
    try { await onRollback(target.id); setTarget(undefined) } catch (cause) { setError(cause instanceof Error ? cause.message : '回滚失败') } finally { setBusy(false) }
  }
  return <section className={styles.historyContent}>
    {error && <div className={`${styles.banner} ${styles.bannerError}`} role="alert">回滚失败：{error}</div>}
    <div className={styles.historyTableWrap}><table className={styles.historyTable}>
      <thead><tr><th>时间</th><th>操作</th><th>状态</th><th>变更</th><th>说明</th><th>回滚</th></tr></thead>
      <tbody>{filteredOperations.length === 0 ? <tr><td colSpan={6} className={styles.historyEmptyRow}>{needle || type !== 'all' || status !== 'all' ? '没有匹配的操作记录。' : '还没有写操作；链接修复、更新或卸载成功后会记录在这里。'}</td></tr> : filteredOperations.map((op) => <tr key={op.id}>
        <td><code>{formatTime(op.createdAt) || '—'}</code></td>
        <td><strong>{opLabel[op.operation] ?? op.operation}</strong></td>
        <td><Badge variant={op.status === 'success' ? 'success' : op.status === 'rolled_back' ? 'outline' : 'error'}>{statusLabel[op.status] ?? op.status}</Badge></td>
        <td>{op.changes?.length ? <details className={styles.opDetails}><summary>{op.changes.length} 项</summary><ul>{op.changes.map((change) => <li key={change}><code>{change}</code></li>)}</ul></details> : '—'}</td>
        <td>{op.message ? <span className={styles.historyMessage} title={op.message}>{op.message}</span> : '—'}</td>
        <td>{rollbackable.has(op.operation) && op.status === 'success' ? <Button variant="outline" size="xs" onClick={() => { setError(''); setTarget(op) }}>回滚</Button> : '—'}</td>
      </tr>)}</tbody>
    </table></div>
    {target && <Dialog open onOpenChange={(open) => { if (!open) setTarget(undefined) }}>
      <DialogContent><DialogHeader><DialogTitle>回滚这次操作？</DialogTitle><DialogDescription>只会恢复该操作记录关联的快照；快照和记录会保留。</DialogDescription></DialogHeader><div className="p-6"><p className={styles.updateMessage}>{target.message}</p>{target.snapshotPath && <code className={styles.opSnapshot}>{target.snapshotPath}</code>}</div><DialogFooter><Button size="sm" variant="outline" onClick={() => setTarget(undefined)}>取消</Button><Button size="sm" variant="destructive" disabled={busy} onClick={() => void rollback()}>{busy ? '回滚中…' : '确认回滚'}</Button></DialogFooter></DialogContent>
    </Dialog>}
  </section>
}

export { OperationsView }
