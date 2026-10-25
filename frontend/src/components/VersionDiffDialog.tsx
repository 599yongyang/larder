import { useEffect, useState } from 'react'
import { AlertTriangle } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { backend } from '../backend'
import styles from '../App.module.css'
import { t } from '../i18n'
import { backendErrorMessage } from '../lib/backendError'
import { formatTime } from '../lib/format'
import type { Operation, SkillUpdateDiff, SkillUpdateFile } from '../types'
import { FileDiff, type DiffLine } from './ai/file-diff'

function label(status: SkillUpdateFile['status']) {
  if (status === 'added') return t('skills.fileAdded')
  if (status === 'removed') return t('skills.versionFileRemoved')
  return t('skills.fileModified')
}

export function VersionDiffDialog({ operation, onClose }: { operation: Operation; onClose: () => void }) {
  const [diff, setDiff] = useState<SkillUpdateDiff>()
  const [error, setError] = useState('')
  useEffect(() => {
    let active = true
    backend.previewVersionDiff(operation.id)
      .then((next) => { if (active) setDiff(next as SkillUpdateDiff) })
      .catch((cause) => { if (active) setError(backendErrorMessage(cause, t('skills.versionDiffFailed'))) })
    return () => { active = false }
  }, [operation.id])
  const files = diff?.files ?? []
  const removed = files.filter((file) => file.status === 'removed').length
  return <Dialog open onOpenChange={(open) => { if (!open) onClose() }}>
    <DialogContent className="flex max-h-[82vh] max-w-[680px] flex-col gap-0 overflow-hidden">
      <DialogHeader><DialogTitle className="text-base">{t('skills.versionDiffTitle')}</DialogTitle><DialogDescription className="text-xs leading-5">{t('skills.versionDiffDescription')} · {formatTime(operation.createdAt) || '—'}</DialogDescription></DialogHeader>
      <div className={styles.versionDiffBody}>
        {!diff && !error && <p className={styles.updatePreviewState}>{t('skills.versionDiffLoading')}</p>}
        {error && <div className={styles.versionDiffError}><AlertTriangle size={13} aria-hidden="true" />{error}</div>}
        {diff && <>
          <div className={styles.versionDiffSummary}><span><strong>{diff.added}</strong>{t('skills.filesAdded')}</span><span><strong>{diff.modified}</strong>{t('skills.filesModified')}</span><span><strong>{removed}</strong>{t('skills.filesRemoved')}</span></div>
          <div className={styles.versionDiffList}>{files.length === 0 ? <p className={styles.versionDiffEmpty}>{t('skills.versionDiffEmpty')}</p> : files.map((file) => {
            const lines: DiffLine[] = (file.lines ?? []).map((line) => ({ content: line.text, oldNumber: line.oldLine, newNumber: line.newLine, type: line.kind === 'add' ? 'addition' : line.kind === 'del' ? 'deletion' : 'context' }))
            return <FileDiff key={`${file.status}:${file.path}`} filename={`${file.path}  ·  ${label(file.status)}`} additions={file.additions} deletions={file.deletions} lines={lines} defaultOpen={false} />
          })}</div>
        </>}
      </div>
      <DialogFooter><Button size="sm" variant="outline" onClick={onClose}>{t('common.close')}</Button></DialogFooter>
    </DialogContent>
  </Dialog>
}
