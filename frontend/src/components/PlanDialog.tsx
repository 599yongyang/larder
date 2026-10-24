import { useState } from 'react'
import { AlertTriangle, Copy, Download, Link2, RefreshCw, Server, Trash2, Wrench } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Checkbox } from '@/components/ui/checkbox'
import { Dialog, DialogContent, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { backend } from '../backend'
import styles from '../App.module.css'
import type { Plan } from '../types'

const actionLabels: Record<string, string> = { create_symlink: '新建链接', already_linked: '已经连接', replace_link: '修复失效链接', replace_copy: '改用共享版本', sync_copy: '同步内容', remove_link: '移除链接', conflict: '目标冲突', error: '检查失败', install_skill: '从来源安装', uninstall_skill: '从中央库移除', remove_mcp_entry: '移除配置', cli_uninstall: '卸载 CLI' }

const operationMeta = (operation: string) => {
  if (operation === 'uninstall_skill' || operation === 'cli_uninstall') return { icon: Trash2, tone: 'danger', confirm: operation === 'uninstall_skill' ? '确认卸载' : '确认卸载 CLI' }
  if (operation === 'mcp_remove') return { icon: Server, tone: 'danger', confirm: '确认移除' }
  if (operation === 'install_skill') return { icon: Download, tone: 'brand', confirm: '确认安装' }
  if (operation.includes('update')) return { icon: RefreshCw, tone: 'brand', confirm: '确认更新' }
  if (operation === 'disable_link') return { icon: Link2, tone: 'neutral', confirm: '确认停用' }
  if (operation === 'link_skill' || operation === 'replace_copy') return { icon: Link2, tone: 'brand', confirm: '确认变更' }
  if (operation === 'sync_copy') return { icon: Copy, tone: 'brand', confirm: '确认同步' }
  return { icon: Wrench, tone: 'neutral', confirm: '确认执行' }
}

export function PlanDialog({ plan, onClose, onApplied }: { plan: Plan; onClose: () => void; onApplied: () => void }) {
  const [confirmed, setConfirmed] = useState(false)
  const [applying, setApplying] = useState(false)
  const [error, setError] = useState('')
  const meta = operationMeta(plan.operation)
  const OperationIcon = meta.icon
  const warnings = (plan.warnings ?? []).filter((item) => !item.includes('快照') && !item.includes('snapshot'))
  const coreChanges = (plan.changes ?? []).filter((change) => change.action !== 'already_linked')
  const apply = async () => {
    if (!confirmed || applying) return
    setApplying(true)
    setError('')
    try {
      await backend.applyPlan(plan.id)
      onApplied()
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : '执行失败')
    } finally {
      setApplying(false)
    }
  }
  return <Dialog open onOpenChange={(open) => { if (!open && !applying) onClose() }}>
    <DialogContent className={`${styles.planDialog} max-w-[560px] gap-0 p-0`} showCloseButton onOpenAutoFocus={(event) => { event.preventDefault(); (event.target as HTMLElement).focus() }}>
      <DialogHeader className={styles.planHeader}>
        <span className={`${styles.planIcon} ${styles[`planIcon_${meta.tone}`]}`}><OperationIcon size={18} strokeWidth={1.8} aria-hidden="true" /></span>
        <div className={styles.planHeading}>
          <DialogTitle className={styles.planTitle}>{plan.title}</DialogTitle>
        </div>
      </DialogHeader>
      <div className={styles.planBody}>
        {(coreChanges.length > 0 || plan.command) && <div className={styles.planFlat}>
          {coreChanges.length > 0 && <ul className={styles.planFlatList}>{coreChanges.map((change) => {
            const target = change.target?.split('/').filter(Boolean).pop() || ''
            const label = change.rootLabel || plan.skillName || plan.mcpName || target || '目标'
            return <li key={`${change.rootId}:${change.target}`}><span>{label}</span><em>{actionLabels[change.action] ?? change.action}</em></li>
          })}</ul>}
          {coreChanges.some((change) => change.action === 'install_skill' && change.source) && <p>来源：{coreChanges.find((change) => change.action === 'install_skill' && change.source)?.source}</p>}
          {plan.command && <p className={styles.planFlatCommand}>命令：<code>{[plan.command, ...(plan.args ?? [])].join(' ')}</code></p>}
        </div>}
        {warnings.length > 0 && <div className={styles.planWarning}><AlertTriangle size={15} aria-hidden="true" /><div>{warnings.map((item) => <p key={item}>{item}</p>)}</div></div>}
        <label className={styles.planConfirm}><Checkbox checked={confirmed} onCheckedChange={(checked) => setConfirmed(Boolean(checked))} /><span>我已确认执行此操作。<small>执行前会生成快照，失败时按该操作的策略处理。</small></span></label>
        {error && <p className={styles.planError} role="alert">{error}</p>}
      </div>
      <DialogFooter className={styles.planFooter}><Button size="sm" variant="outline" onClick={onClose} disabled={applying}>取消</Button><Button size="sm" variant={meta.tone === 'danger' ? 'destructive' : 'default'} disabled={!confirmed || applying} onClick={() => void apply()}>{applying ? '执行中…' : meta.confirm}</Button></DialogFooter>
    </DialogContent>
  </Dialog>
}
