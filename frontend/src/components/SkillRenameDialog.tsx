import { useState } from 'react'
import { FilePenLine, Loader2 } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { backend } from '../backend'
import { backendErrorMessage } from '../lib/backendError'
import { toastSuccess } from '../lib/toast'
import { t } from '../i18n'
import styles from '../App.module.css'
import type { Skill, SkillRenamePlan } from '../types'

export function SkillRenameDialog({ skill, onClose, onSaved }: { skill: Skill; onClose: () => void; onSaved: () => void }) {
  const [name, setName] = useState(skill.name)
  const [plan, setPlan] = useState<SkillRenamePlan>()
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')
  const planRename = async () => {
    if (!name.trim()) { setError(t('skills.skillNameRequired')); return }
    setBusy(true); setError('')
    try { setPlan(await backend.planSkillRename(skill.id, name.trim()) as SkillRenamePlan) } catch (cause) { setError(backendErrorMessage(cause, t('skills.skillRenameFailed'))) } finally { setBusy(false) }
  }
  const apply = async () => {
    if (!plan) return
    setBusy(true); setError('')
    try { await backend.applySkillRename(plan.id); toastSuccess(t('skills.skillRenamed')); onSaved(); onClose() } catch (cause) { setError(backendErrorMessage(cause, t('skills.skillRenameFailed'))); setBusy(false) }
  }
  const close = () => { if (plan && !busy) void backend.cancelSkillRename(plan.id); onClose() }
  return <Dialog open onOpenChange={(open) => { if (!open && !busy) close() }}>
    <DialogContent className="max-w-xl gap-0 p-0">
      <DialogHeader className="border-b px-6 py-4 pr-14"><DialogTitle className="flex items-center gap-2 text-base"><FilePenLine size={17} />{t('skills.renameSkill')}</DialogTitle><DialogDescription className="text-xs">{t('skills.renameSkillHint')}</DialogDescription></DialogHeader>
      <div className="px-6 py-5">
        {!plan ? <label className={styles.commandField}><span>{t('skills.nameLabel')}</span><input value={name} onChange={(event) => setName(event.target.value)} disabled={busy} /></label> : <div className={styles.commandPlan}><div className={styles.commandPlanHead}><div><strong>{t('skills.renameSkillPlan')}</strong><small>{plan.from} → {plan.to}</small></div></div></div>}
        {error && <p className={styles.planError} role="alert">{error}</p>}
      </div>
      <DialogFooter className="border-t px-6 py-3"><Button size="sm" variant="outline" onClick={close} disabled={busy}>{t('common.cancel')}</Button>{!plan ? <Button size="sm" disabled={busy} onClick={() => void planRename()}>{busy && <Loader2 size={13} className="animate-spin" />}{t('skills.previewChanges')}</Button> : <><Button size="sm" variant="outline" onClick={() => setPlan(undefined)} disabled={busy}>{t('skills.backToEdit')}</Button><Button size="sm" onClick={() => void apply()} disabled={busy}>{busy && <Loader2 size={13} className="animate-spin" />}{t('skills.applyChanges')}</Button></>}</DialogFooter>
    </DialogContent>
  </Dialog>
}
