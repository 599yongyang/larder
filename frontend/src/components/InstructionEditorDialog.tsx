import { useEffect, useState } from 'react'
import { Button } from '@/components/ui/button'
import { Textarea } from '@/components/ui/textarea'
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { backend } from '../backend'
import styles from '../App.module.css'
import { t } from '../i18n'
import { backendErrorMessage } from '../lib/backendError'
import type { InstructionFile } from '../types'

const instructionNames = ['AGENTS.md', 'CLAUDE.md', 'GEMINI.md', '.cursorrules', '.claude/CLAUDE.md', '.cursor/rules/project.mdc', '.github/copilot-instructions.md']

export function InstructionEditorDialog({ file, projects, project, onClose, onSaved }: { file?: InstructionFile; projects: string[]; project?: string; onClose: () => void; onSaved: () => void }) {
  const [content, setContent] = useState('')
  const [loading, setLoading] = useState(Boolean(file))
  const [saving, setSaving] = useState(false)
  const [error, setError] = useState('')
  const [selectedProject, setSelectedProject] = useState(project || projects[0] || '')
  const [name, setName] = useState('AGENTS.md')
  useEffect(() => {
    if (!file) return
    let active = true
    backend.instructionContent(file.path).then((next) => { if (active) setContent(next) }).catch((cause) => { if (active) setError(backendErrorMessage(cause, t('instructions.failedToReadFile'))) }).finally(() => { if (active) setLoading(false) })
    return () => { active = false }
  }, [file])
  const save = async () => {
    if (saving) return
    setSaving(true); setError('')
    try {
      if (file) await backend.saveInstruction(file.path, content)
      else await backend.createInstruction(selectedProject, name, content)
      onSaved(); onClose()
    } catch (cause) { setError(backendErrorMessage(cause, t('common.failedToSave'))) } finally { setSaving(false) }
  }
  const valid = Boolean(file || (selectedProject && name)) && !loading
  return <Dialog open onOpenChange={(open) => { if (!open && !saving) onClose() }}>
    <DialogContent className="max-w-2xl gap-0 p-0">
      <DialogHeader className="border-b px-6 py-4 pr-12"><DialogTitle className="text-base">{file ? t('common.edit', { name: file.name }) : t('instructions.newInstructionFile')}</DialogTitle><DialogDescription className="text-xs">{file ? file.path : t('instructions.chooseAProjectAndFileNameExistingContentIsBackedUpBeforeSaving')}</DialogDescription></DialogHeader>
      <div className={styles.instructionEditorBody}>
        {!file && <div className={styles.instructionEditorRow}><Select value={selectedProject} onValueChange={setSelectedProject}><SelectTrigger size="sm"><SelectValue placeholder={t("instructions.chooseProject")} /></SelectTrigger><SelectContent>{projects.map((item) => <SelectItem key={item} value={item}>{item.split('/').filter(Boolean).pop()}</SelectItem>)}</SelectContent></Select><Select value={name} onValueChange={setName}><SelectTrigger size="sm"><SelectValue /></SelectTrigger><SelectContent>{instructionNames.map((item) => <SelectItem key={item} value={item}>{item}</SelectItem>)}</SelectContent></Select></div>}
        {loading ? <p className={styles.instructionEditorHint}>{t('common.loading')}</p> : <Textarea className={styles.instructionEditor} value={content} onChange={(event) => setContent(event.target.value)} spellCheck={false} aria-label={t("instructions.instructionFileContent")} />}
        {error && <p className={styles.planError} role="alert">{error}</p>}
        <div className={styles.instructionEditorMeta}><span>{t('instructions.characters', { n: content.length.toLocaleString() })}</span><span>{t('instructions.aBackupIsKeptOnSave')}</span></div>
      </div>
      <DialogFooter className="border-t px-6 py-2.5"><Button size="sm" variant="outline" disabled={saving} onClick={onClose}>{t('common.cancel')}</Button><Button size="sm" disabled={saving || !valid} onClick={() => void save()}>{saving ? t('common.saving') : t('common.save')}</Button></DialogFooter>
    </DialogContent>
  </Dialog>
}
