import { useState } from 'react'
import { Folder, FolderOpen, Trash2 } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { backend } from '../backend'
import styles from '../App.module.css'
import { t } from '../i18n'
import { toastError } from '../lib/toast'

type Project = { path: string; label: string; count: number }
export function ProjectManagerDialog({ projects, onClose, onAdd, onRemove }: { projects: Project[]; onClose: () => void; onAdd: (path: string) => Promise<void>; onRemove: (path: string) => Promise<void> }) {
  const [busy, setBusy] = useState('')
  const [error, setError] = useState('')
  const browse = async () => {
    setError('')
    try {
      const path = await backend.pickFolder()
      if (!path) return
      if (projects.some((project) => project.path === path)) { setError(t('common.thisProjectHasAlreadyBeenAdded')); return }
      setBusy('add')
      await onAdd(path)
    } catch (cause) {
      toastError(cause, t('common.failedToAddProject'))
    } finally {
      setBusy('')
    }
  }
  const remove = async (path: string) => {
    setBusy(path)
    setError('')
    try { await onRemove(path) } catch { setError(t('common.failedToRemove')) } finally { setBusy('') }
  }
  return <Dialog open onOpenChange={(open) => { if (!open && !busy) onClose() }}>
    <DialogContent className="max-w-2xl gap-0 p-0">
      <DialogHeader className="border-b px-6 py-4 pr-14"><DialogTitle className="text-base">{t('common.manageProjects')}</DialogTitle><DialogDescription className="max-w-xl text-xs leading-5">{t('common.projectSkillsAreReadOnlyFromWorkspacesYouAddRemovingAProjectNeverDeletesFiles')}</DialogDescription></DialogHeader>
      <div className="max-h-[56vh] overflow-y-auto px-6 py-5">
        <Button variant="outline" size="sm" disabled={busy === 'add'} onClick={() => void browse()}><FolderOpen size={14} />{busy === 'add' ? t('common.adding') : t('common.chooseProjectFolder')}</Button>
        {error && <p className={styles.planError} role="alert">{error}</p>}
        <div className={styles.projectManagerList}>{projects.map((project) => <div key={project.path} className={styles.projectManagerRow}><Folder size={16} aria-hidden="true" /><div><strong>{project.label}</strong><small>{project.path}</small></div><span>{t('common.skillCount', { n: project.count })}</span><Button variant="ghost" size="xs" className="text-destructive" disabled={busy === project.path} onClick={() => void remove(project.path)}><Trash2 size={13} />{t('common.removed')}</Button></div>)}{projects.length === 0 && <p className={styles.footerHint}>{t('common.noProjectWorkspacesYetUseTheButtonAboveToChooseAFolder')}</p>}</div>
      </div>
      <DialogFooter className="border-t px-6 py-3"><Button size="sm" variant="outline" onClick={onClose} disabled={Boolean(busy)}>{t('common.close')}</Button></DialogFooter>
    </DialogContent>
  </Dialog>
}
