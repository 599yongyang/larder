import { useState } from 'react'
import { Button } from '@/components/ui/button'
import { Checkbox } from '@/components/ui/checkbox'
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { backend } from '../backend'
import styles from '../App.module.css'
import type { Plan } from '../types'

export function BatchPlanDialog({ plans, onClose, onApplied }: { plans: Plan[]; onClose: () => void; onApplied: () => void }) {
  const [confirmed, setConfirmed] = useState(false)
  const [applying, setApplying] = useState(false)
  const [progress, setProgress] = useState(0)
  const [error, setError] = useState('')
  const totalChanges = plans.reduce((sum, plan) => sum + plan.changes.length, 0)

  const apply = async () => {
    if (!confirmed || applying) return
    setApplying(true)
    setError('')
    try {
      for (let index = 0; index < plans.length; index += 1) {
        await backend.applyPlan(plans[index].id)
        setProgress(index + 1)
      }
      onApplied()
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : '批量执行失败')
    } finally {
      setApplying(false)
    }
  }

  return <Dialog open onOpenChange={(open) => { if (!open && !applying) onClose() }}>
    <DialogContent className="max-w-2xl gap-0 p-0">
      <DialogHeader className="border-b px-5 py-4 pr-12">
        <DialogTitle className="text-base">确认修改</DialogTitle>
        <DialogDescription className="max-w-lg text-xs leading-5">将修改 {plans.length} 个 Skill，共 {totalChanges} 项变更。每个 Skill 执行前都会写快照。</DialogDescription>
      </DialogHeader>
      <div className="max-h-[50vh] overflow-y-auto px-5 py-4">
        <section className={styles.issueSection}>
          <div className={styles.issueSectionHead}><div><h3>将要修改</h3><p>{plans.map((plan) => plan.skillName).filter(Boolean).slice(0, 6).join('、')}{plans.length > 6 ? ` 等 ${plans.length} 个` : ''}</p></div></div>
          <div className={styles.batchList}>{plans.map((plan) => <div key={plan.id} className={styles.batchRow}><strong>{plan.skillName || plan.title}</strong><span>{plan.changes.length} 项</span></div>)}</div>
        </section>
        {applying && <p className={styles.resultCount}>正在处理 {progress}/{plans.length}…</p>}
        <label className={styles.planConfirm}><Checkbox checked={confirmed} onCheckedChange={(checked) => setConfirmed(Boolean(checked))} /><span>我已确认以上修改；执行前会逐个写入快照或操作记录。</span></label>
        {error && <p className={styles.planError} role="alert">{error}</p>}
      </div>
      <DialogFooter className="border-t px-5 py-3"><Button size="sm" variant="outline" onClick={onClose} disabled={applying}>关闭</Button><Button size="sm" disabled={!confirmed || applying} onClick={() => void apply()}>{applying ? `处理中 ${progress}/${plans.length}` : '确认并修改'}</Button></DialogFooter>
    </DialogContent>
  </Dialog>
}
