import { useState } from 'react'
import { Button } from '@/components/ui/button'
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { backend } from '../backend'
import styles from '../App.module.css'

export function SkillUpdateDialog({ skillID, skillName, source, onClose, onApplied }: { skillID: string; skillName: string; source?: string; onClose: () => void; onApplied: () => void }) {
  const [applying, setApplying] = useState(false)
  const [error, setError] = useState('')
  const update = async () => {
    setApplying(true)
    setError('')
    try {
      const plan = await backend.planSkillUpdate(skillID)
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
      <DialogHeader><DialogTitle className="text-base">更新 {skillName}？</DialogTitle><DialogDescription className="text-xs leading-5">会用来源仓库的最新内容覆盖本地 Skill 文件夹，执行前会写入快照。</DialogDescription></DialogHeader>
      <div className="px-6 pb-2"><div className={styles.updateConfirmSource}><span>来源</span><code>{source || 'GitHub'}</code></div></div>
      {error && <p className={styles.planError} role="alert">{error}</p>}
      <DialogFooter><Button size="sm" variant="outline" onClick={onClose} disabled={applying}>取消</Button><Button size="sm" disabled={applying} onClick={() => void update()}>{applying ? '更新中…' : '确认更新'}</Button></DialogFooter>
    </DialogContent>
  </Dialog>
}
