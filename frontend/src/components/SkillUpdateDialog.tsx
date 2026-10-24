import { useState } from 'react'
import { Button } from '@/components/ui/button'
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { backend } from '../backend'
import styles from '../App.module.css'
import { t } from '../i18n'
import { backendErrorMessage } from '../lib/backendError'

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
      setError(backendErrorMessage(cause, t('common.updateFailed')))
    } finally {
      setApplying(false)
    }
  }
  return <Dialog open onOpenChange={(open) => { if (!open && !applying) onClose() }}>
    <DialogContent className="max-w-md">
      <DialogHeader><DialogTitle className="text-base">{t('common.update', { name: skillName })}</DialogTitle><DialogDescription className="text-xs leading-5">{t('common.theLocalSkillFolderIsReplacedWithTheLatestContentFromItsSourceASnapshotIsTakenFirst')}</DialogDescription></DialogHeader>
      <div className="px-6 pb-2"><div className={styles.updateConfirmSource}><span>{t('common.sourceLabel')}</span><code>{source || 'GitHub'}</code></div></div>
      {error && <p className={styles.planError} role="alert">{error}</p>}
      <DialogFooter><Button size="sm" variant="outline" onClick={onClose} disabled={applying}>{t('common.cancel')}</Button><Button size="sm" disabled={applying} onClick={() => void update()}>{applying ? t('common.updating') : t('common.updateAction')}</Button></DialogFooter>
    </DialogContent>
  </Dialog>
}
