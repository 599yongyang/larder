import { useState } from 'react'
import { Button } from '@/components/ui/button'
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { backend } from '../backend'
import styles from '../App.module.css'
import type { CLIInfo, UpdateItem } from '../types'

export function CLIUpdateDialog({ cli, update, label, onClose, onApplied }: { cli: CLIInfo; update: UpdateItem; label?: string; onClose: () => void; onApplied: () => void }) {
  const [applying, setApplying] = useState(false)
  const [error, setError] = useState('')
  const apply = async () => {
    setApplying(true)
    setError('')
    try {
      const plan = await backend.planCLI(cli.id, false, update.latestVersion ?? '')
      await backend.applyPlan(plan.id)
      onApplied()
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : '更新失败')
    } finally {
      setApplying(false)
    }
  }
  return <Dialog open onOpenChange={(open) => { if (!open && !applying) onClose() }}>
    <DialogContent className="max-w-md">
      <DialogHeader><DialogTitle className="text-base">更新 {label || cli.name}？</DialogTitle><DialogDescription className="text-xs leading-5">会调用 {cli.owner} 更新 {cli.ownerDetail || cli.name}，执行前会记录操作。</DialogDescription></DialogHeader>
      <div className="px-6 pb-2"><div className={styles.updateVersionBox}><span>{update.currentVersion || cli.version || '当前版本'}</span><strong>→</strong><span>{update.latestVersion}</span></div></div>
      {error && <p className={styles.planError} role="alert">{error}</p>}
      <DialogFooter><Button size="sm" variant="outline" onClick={onClose} disabled={applying}>取消</Button><Button size="sm" disabled={applying} onClick={() => void apply()}>{applying ? '更新中…' : '确认更新'}</Button></DialogFooter>
    </DialogContent>
  </Dialog>
}
