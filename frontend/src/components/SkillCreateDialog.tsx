import { useState } from 'react'
import { FilePlus2, Loader2, AlertTriangle } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Textarea } from '@/components/ui/textarea'
import { Input } from '@/components/ui/input'
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { backend } from '../backend'
import { backendErrorMessage } from '../lib/backendError'
import { toastSuccess } from '../lib/toast'
import { t } from '../i18n'
import styles from '../App.module.css'
import type { SkillCreatePlan } from '../types'

const warningText = (warning: string) => {
  if (warning === 'SKILL.md should start with YAML frontmatter') return t('skills.frontmatterMissing')
  if (warning === 'frontmatter is not closed with ---') return t('skills.frontmatterUnclosed')
  if (warning === 'frontmatter is missing name') return t('skills.frontmatterNameMissing')
  if (warning === 'frontmatter is missing description') return t('skills.frontmatterDescriptionMissing')
  return warning
}

export function SkillCreateDialog({ onClose, onSaved }: { onClose: () => void; onSaved: () => void }) {
  const [name, setName] = useState('')
  const [description, setDescription] = useState('')
  const [content, setContent] = useState('')
  const [plan, setPlan] = useState<SkillCreatePlan>()
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')

  const yamlValue = (value: string) => JSON.stringify(value)
  const generated = content || `---\nname: ${yamlValue(name)}\ndescription: ${yamlValue(description)}\n---\n\n# ${name || 'New Skill'}\n`
  const planCreate = async () => {
    if (!name.trim()) { setError(t('skills.skillNameRequired')); return }
    setBusy(true); setError('')
    try { setPlan(await backend.planSkillCreate(name.trim(), generated) as SkillCreatePlan) } catch (cause) { setError(backendErrorMessage(cause, t('skills.skillCreateFailed'))) } finally { setBusy(false) }
  }
  const apply = async () => {
    if (!plan) return
    setBusy(true); setError('')
    try { await backend.applySkillCreate(plan.id); toastSuccess(t('skills.skillCreated')); onSaved(); onClose() } catch (cause) { setError(backendErrorMessage(cause, t('skills.skillCreateFailed'))); setBusy(false) }
  }
  const close = () => { if (plan && !busy) void backend.cancelSkillCreate(plan.id); onClose() }
  return <Dialog open onOpenChange={(open) => { if (!open && !busy) close() }}>
    <DialogContent className="max-w-2xl gap-0 p-0">
      <DialogHeader className="border-b px-6 py-4 pr-14"><DialogTitle className="flex items-center gap-2 text-base"><FilePlus2 size={17} />{t('skills.newSkill')}</DialogTitle><DialogDescription className="text-xs">{t('skills.newSkillHint')}</DialogDescription></DialogHeader>
      <div className="max-h-[68vh] overflow-y-auto px-6 py-5">
        {!plan ? <>
          <div className={styles.commandFieldRow}>
            <label className={styles.commandField}><span>{t('skills.nameLabel')}</span><Input className="!h-8" value={name} onChange={(event) => setName(event.target.value)} placeholder="my-skill" disabled={busy} /></label>
            <label className={styles.commandField}><span>{t('skills.descriptionLabel')}</span><Input className="!h-8" value={description} onChange={(event) => setDescription(event.target.value)} placeholder={t('skills.shortDescription')} disabled={busy} /></label>
          </div>
          <label className={styles.commandField}><span>SKILL.md</span><Textarea className={styles.commandTextarea} value={generated} onChange={(event) => setContent(event.target.value)} spellCheck={false} aria-label="SKILL.md" /></label>
          {error && <p className={styles.planError} role="alert">{error}</p>}
        </> : <div className={styles.commandPlan}>
          <div className={styles.commandPlanHead}><div><strong>{t('skills.createSkillPlan')}</strong><small>{plan.path}</small></div></div>
          {plan.warnings && plan.warnings.length > 0 && <div className={styles.commandWarnings}>{plan.warnings.map((warning) => <p key={warning}><AlertTriangle size={12} aria-hidden="true" />{warningText(warning)}</p>)}</div>}
          <pre className={styles.commandPlanPreview}>{plan.content}</pre>
          {error && <p className={styles.planError} role="alert">{error}</p>}
        </div>}
      </div>
      <DialogFooter className="border-t px-6 py-3"><Button size="sm" variant="outline" onClick={close} disabled={busy}>{t('common.cancel')}</Button>{!plan ? <Button size="sm" disabled={busy} onClick={() => void planCreate()}>{busy && <Loader2 size={13} className="animate-spin" />}{t('skills.previewChanges')}</Button> : <><Button size="sm" variant="outline" onClick={() => setPlan(undefined)} disabled={busy}>{t('skills.backToEdit')}</Button><Button size="sm" onClick={() => void apply()} disabled={busy}>{busy && <Loader2 size={13} className="animate-spin" />}{t('skills.applyChanges')}</Button></>}</DialogFooter>
    </DialogContent>
  </Dialog>
}
