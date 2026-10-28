import { useEffect, useState } from 'react'
import { AlertTriangle, ArrowLeft, FilePenLine, Loader2 } from 'lucide-react'
import { FileDiff } from './ai/file-diff'
import { lineDiff } from '../lib/lineDiff'
import { Button } from '@/components/ui/button'
import { Textarea } from '@/components/ui/textarea'
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { MarkdownPreview, splitFrontmatter } from './MarkdownPreview'
import { backend } from '../backend'
import { backendErrorMessage } from '../lib/backendError'
import { toastSuccess } from '../lib/toast'
import { t } from '../i18n'
import styles from '../App.module.css'
import type { Skill, SkillEditPlan } from '../types'

const warningText = (warning: string) => {
  if (warning === 'SKILL.md should start with YAML frontmatter') return t('skills.frontmatterMissing')
  if (warning === 'frontmatter is not closed with ---') return t('skills.frontmatterUnclosed')
  if (warning === 'frontmatter is missing name') return t('skills.frontmatterNameMissing')
  if (warning === 'frontmatter is missing description') return t('skills.frontmatterDescriptionMissing')
  return warning
}

export function SkillEditorPanel({ skill, onBack, onSaved }: { skill: Skill; onBack: () => void; onSaved: () => void }) {
  const [content, setContent] = useState('')
  const [loading, setLoading] = useState(true)
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')
  const [plan, setPlan] = useState<SkillEditPlan>()
  const [tab, setTab] = useState<'edit' | 'preview'>('edit')
  const [initialContent, setInitialContent] = useState('')

  useEffect(() => {
    let active = true
    setLoading(true); setError('')
    backend.skillContent(skill.id).then((next) => { if (active) { setContent(next); setInitialContent(next) } }).catch((cause) => { if (active) setError(backendErrorMessage(cause, t('skills.skillLoadFailed'))) }).finally(() => { if (active) setLoading(false) })
    return () => { active = false }
  }, [skill.id])

  const planEdit = async () => {
    setBusy(true); setError('')
    try {
      setPlan(await backend.planSkillEdit(skill.id, content) as SkillEditPlan)
    } catch (cause) {
      setError(backendErrorMessage(cause, t('skills.skillPlanFailed')))
    } finally { setBusy(false) }
  }

  const apply = async () => {
    if (!plan) return
    setBusy(true); setError('')
    try {
      await backend.applySkillEdit(plan.id)
      toastSuccess(t('skills.skillSaved'))
      onSaved(); onBack()
    } catch (cause) {
      setError(backendErrorMessage(cause, t('skills.skillSaveFailed')))
      setBusy(false)
    }
  }

  const [confirmDiscard, setConfirmDiscard] = useState(false)
  const back = () => {
    if (busy) return
    if (!plan && content !== initialContent) { setConfirmDiscard(true); return }
    if (plan) void backend.cancelSkillEdit(plan.id)
    onBack()
  }
  const { meta, body } = splitFrontmatter(content)
  const changed = !loading && content !== initialContent
  const diff = plan && plan.currentContent && plan.exists ? lineDiff(plan.currentContent, plan.content) : []
  return <div className={styles.skillEditorPanel}>
    <header className={styles.skillEditorTopbar}>
      <Button variant="ghost" size="xs" onClick={back} disabled={busy}><ArrowLeft size={13} />{t('skills.backToDetail')}</Button>
      <span className={styles.skillEditorTitle}><FilePenLine size={14} />{t('skills.editSkill')}</span>
      <small>{skill.name}</small>
    </header>
    <div className={styles.skillEditorScroll}>
      {!plan ? <>
        <div className={styles.skillEditorTabs}>
          <div className={styles.segmentedControl} role="group" aria-label={t('skills.skillEditorMode')}>
            <Button size="xs" variant={tab === 'edit' ? 'secondary' : 'ghost'} onClick={() => setTab('edit')}>{t('skills.edit')}</Button>
            <Button size="xs" variant={tab === 'preview' ? 'secondary' : 'ghost'} onClick={() => setTab('preview')}>{t('skills.preview')}</Button>
          </div>
          <span className={styles.skillEditorPath}>{skill.path}</span>
        </div>
        {loading ? <div className={styles.commandLoading}><Loader2 size={14} className="animate-spin" />{t('common.loading')}</div> : tab === 'edit' ? <Textarea className={`${styles.commandTextarea} ${styles.skillEditorTextarea}`} value={content} onChange={(event) => setContent(event.target.value)} spellCheck={false} aria-label={t('skills.skillContent')} /> : <div className={styles.skillEditorPreview}>{meta.length > 0 && <dl className={styles.definition}>{meta.map(([key, value]) => <div key={key}><dt>{key}</dt><dd>{value}</dd></div>)}</dl>}<MarkdownPreview source={body} /></div>}
        {error && <p className={styles.planError} role="alert">{error}</p>}
      </> : <div className={styles.commandPlan}>
        <div className={styles.commandPlanHead}><div><strong>{t('skills.skillPlan')}</strong><small>{plan.path}</small></div></div>
        {plan.warnings && plan.warnings.length > 0 && <div className={styles.commandWarnings}>{plan.warnings.map((warning) => <p key={warning}><AlertTriangle size={12} aria-hidden="true" />{warningText(warning)}</p>)}</div>}
        {diff.length > 0 ? <FileDiff className={styles.skillEditorDiff} filename="SKILL.md" lines={diff} additions={diff.filter((line) => line.type === 'addition').length} deletions={diff.filter((line) => line.type === 'deletion').length} defaultOpen /> : <pre className={styles.commandPlanPreview}>{plan.content || t('skills.emptySkillFile')}</pre>}
        {error && <p className={styles.planError} role="alert">{error}</p>}
      </div>}
    </div>
    <footer className={styles.skillEditorFooter}>
      <Button size="sm" variant="outline" onClick={back} disabled={busy}>{t('common.cancel')}</Button>
      <div className={styles.skillEditorFooterRight}>
        {!plan ? <Button size="sm" disabled={busy || loading || !changed} title={changed ? t('skills.savePlanHint') : t('skills.noChangesToSave')} onClick={() => void planEdit()}>{busy && <Loader2 size={13} className="animate-spin" />}{t('skills.saveSkill')}</Button> : <><Button size="sm" variant="outline" onClick={() => setPlan(undefined)} disabled={busy}>{t('skills.backToEdit')}</Button><Button size="sm" onClick={() => void apply()} disabled={busy}>{busy && <Loader2 size={13} className="animate-spin" />}{t('skills.applyChanges')}</Button></>}
      </div>
    </footer>
    <Dialog open={confirmDiscard} onOpenChange={setConfirmDiscard}>
      <DialogContent className="max-w-sm">
        <DialogHeader><DialogTitle>{t('skills.discardChangesTitle')}</DialogTitle><DialogDescription>{t('skills.discardChanges')}</DialogDescription></DialogHeader>
        <DialogFooter><Button size="sm" variant="outline" onClick={() => setConfirmDiscard(false)}>{t('common.cancel')}</Button><Button size="sm" variant="destructive" onClick={onBack}>{t('skills.discardAndBack')}</Button></DialogFooter>
      </DialogContent>
    </Dialog>
  </div>
}
