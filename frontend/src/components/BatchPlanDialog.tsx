import { useState } from 'react'
import { Button } from '@/components/ui/button'
import { Checkbox } from '@/components/ui/checkbox'
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { backend } from '../backend'
import styles from '../App.module.css'
import { t } from '../i18n'
import { backendErrorMessage } from '../lib/backendError'
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
      setError(backendErrorMessage(cause, t('common.batchOperationFailed')))
    } finally {
      setApplying(false)
    }
  }

  return <Dialog open onOpenChange={(open) => { if (!open && !applying) onClose() }}>
    <DialogContent className="max-w-2xl gap-0 p-0">
      <DialogHeader className="border-b px-5 py-4 pr-12">
        <DialogTitle className="text-base">{t('common.confirmChanges')}</DialogTitle>
        <DialogDescription className="max-w-lg text-xs leading-5">{t('common.changesSkillsWithChangesASnapshotIsTakenBeforeEachSkill', { plans: plans.length, changes: totalChanges })}</DialogDescription>
      </DialogHeader>
      <div className="max-h-[50vh] overflow-y-auto px-5 py-4">
        <section className={styles.issueSection}>
          <div className={styles.issueSectionHead}><div><h3>{t('common.willChange')}</h3><p>{plans.map((plan) => plan.skillName).filter(Boolean).slice(0, 6).join('、')}{plans.length > 6 ? ` ${t('common.andMore', { n: plans.length })}` : ''}</p></div></div>
          <div className={styles.batchList}>{plans.map((plan) => <div key={plan.id} className={styles.batchRow}><strong>{plan.skillName || plan.title}</strong><span>{t('common.changeCount', { n: plan.changes.length })}</span></div>)}</div>
        </section>
        {applying && <p className={styles.resultCount}>{t('common.processing', { progress, total: plans.length })}</p>}
        <label className={styles.planConfirm}><Checkbox checked={confirmed} onCheckedChange={(checked) => setConfirmed(Boolean(checked))} /><span>{t('common.iConfirmTheseChangesASnapshotOrOperationRecordIsWrittenForEachOne')}</span></label>
        {error && <p className={styles.planError} role="alert">{error}</p>}
      </div>
      <DialogFooter className="border-t px-5 py-3"><Button size="sm" variant="outline" onClick={onClose} disabled={applying}>{t('common.close')}</Button><Button size="sm" disabled={!confirmed || applying} onClick={() => void apply()}>{applying ? t('common.processingProgress', { progress, total: plans.length }) : t('common.confirmAndChange')}</Button></DialogFooter>
    </DialogContent>
  </Dialog>
}
