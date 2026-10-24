import { useState } from 'react'
import { Button } from '@/components/ui/button'
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { backend } from '../backend'
import styles from '../App.module.css'
import { t } from '../i18n'
import { backendErrorMessage } from '../lib/backendError'
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
      setError(backendErrorMessage(cause, t('common.updateFailed')))
    } finally {
      setApplying(false)
    }
  }
  return <Dialog open onOpenChange={(open) => { if (!open && !applying) onClose() }}>
    <DialogContent className="max-w-md">
      <DialogHeader><DialogTitle className="text-base">{t('common.update', { name: label || cli.name })}</DialogTitle><DialogDescription className="text-xs leading-5">{t('cli.callsToUpdateTheOperationIsRecordedFirst', { owner: cli.owner, name: cli.ownerDetail || cli.name })}</DialogDescription></DialogHeader>
      <div className="px-6 pb-2"><div className={styles.updateVersionBox}><span>{update.currentVersion || cli.version || t('cli.currentVersion')}</span><strong>→</strong><span>{update.latestVersion}</span></div></div>
      {error && <p className={styles.planError} role="alert">{error}</p>}
      <DialogFooter><Button size="sm" variant="outline" onClick={onClose} disabled={applying}>{t('common.cancel')}</Button><Button size="sm" disabled={applying} onClick={() => void apply()}>{applying ? t('common.updating') : t('common.updateAction')}</Button></DialogFooter>
    </DialogContent>
  </Dialog>
}
