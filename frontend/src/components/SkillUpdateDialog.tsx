import { useEffect, useState } from 'react'
import { AlertTriangle, Check, FolderLock, GitCompareArrows } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { backend } from '../backend'
import styles from '../App.module.css'
import { t } from '../i18n'
import { backendErrorMessage } from '../lib/backendError'
import { toastError } from '../lib/toast'
import { FileDiff, type DiffLine } from './ai/file-diff'
import type { SkillUpdateDiff, SkillUpdateFile } from '../types'



const diffCache = new Map<string, { at: number; diff: SkillUpdateDiff }>()
const DIFF_CACHE_TTL = 5_000

function defaultsForDiff(diff: SkillUpdateDiff): Record<string, string> {
  return Object.fromEntries(diff.files.filter((file) => file.status === 'conflict' || file.status === 'local_modified' || file.status === 'local' || (!diff.hasBaseline && file.status === 'modified')).map((file) => [file.path, 'local']))
}


function fileStatusLabel(status: SkillUpdateFile['status'], hasBaseline: boolean) {
  switch (status) {
    case 'added': return t('skills.fileAdded')
    case 'modified': return hasBaseline ? t('skills.fileModified') : t('skills.fileDiffers')
    case 'local': return t('skills.fileLocalOnly')
    case 'local_modified': return t('skills.fileLocalModified')
    case 'conflict': return t('skills.fileConflict')
    case 'removed': return t('skills.fileRemoved')
    default: return status
  }
}

export function SkillUpdateDialog({ skillID, skillName, source, onClose, onApplied }: { skillID: string; skillName: string; source?: string; onClose: () => void; onApplied: () => void }) {
  const [applying, setApplying] = useState(false)
  const [error, setError] = useState('')
  const [diff, setDiff] = useState<SkillUpdateDiff>()
  const [diffError, setDiffError] = useState('')
  const [resolutions, setResolutions] = useState<Record<string, string>>({})
  const [resolved, setResolved] = useState<Set<string>>(() => new Set())
  const [fileDiffs, setFileDiffs] = useState<Record<string, SkillUpdateFile>>({})
  const [loadingFiles, setLoadingFiles] = useState<Set<string>>(() => new Set())
  const loadFileDiff = async (file: SkillUpdateFile) => { if (fileDiffs[file.path] || loadingFiles.has(file.path)) return; setLoadingFiles((current) => new Set(current).add(file.path)); try { const next = await backend.previewSkillFileDiff(skillID, file.path) as SkillUpdateFile; setFileDiffs((current) => ({ ...current, [file.path]: next })) } catch (cause) { toastError(cause, t('skills.versionDiffFailed')) } finally { setLoadingFiles((current) => { const next = new Set(current); next.delete(file.path); return next }) } }
  useEffect(() => {
    let active = true
    const cached = diffCache.get(skillID)
    if (cached && Date.now()-cached.at < DIFF_CACHE_TTL) {
      setDiff(cached.diff)
      setResolutions(defaultsForDiff(cached.diff))
      setResolved(new Set())
      return () => { active = false }
    }
    setDiff(undefined)
    setDiffError('')
    backend.previewSkillUpdate(skillID).then((next) => { if (!active) return; const value = next as SkillUpdateDiff; diffCache.set(skillID, { at: Date.now(), diff: value }); setResolved(new Set()); setDiff(value); setResolutions(defaultsForDiff(value)) }).catch((cause) => { if (active) setDiffError(backendErrorMessage(cause, t('skills.updatePreviewFailed'))) })
    return () => { active = false }
  }, [skillID])
  const update = async () => {
    setApplying(true)
    setError('')
    try {
      const plan = await backend.planSkillUpdate(skillID)
      await backend.applyPlanWithResolutions(plan.id, resolutions)
      diffCache.delete(skillID)
      onApplied()
    } catch (cause) {
      setError(backendErrorMessage(cause, t('common.updateFailed')))
    } finally {
      setApplying(false)
    }
  }
  const files = diff?.files ?? []
  const needsResolution = (file: SkillUpdateFile) => file.status === 'conflict' || file.status === 'local_modified' || file.status === 'local' || (!diff?.hasBaseline && file.status === 'modified')
  const resolvable = files.filter(needsResolution)
  const choose = (path: string, value: string) => { setResolutions((current) => ({ ...current, [path]: value })); setResolved((current) => new Set(current).add(path)) }
  const setAll = (value: 'local' | 'upstream' | 'both') => { setResolutions(Object.fromEntries(resolvable.map((file) => [file.path, value === 'both' && file.status === 'local' ? 'local' : value]))); setResolved(new Set(resolvable.map((file) => file.path))) }
  return <Dialog open onOpenChange={(open) => { if (!open && !applying) onClose() }}>
    <DialogContent className="flex max-h-[84vh] max-w-[680px] flex-col gap-0 overflow-hidden">
      <DialogHeader><DialogTitle className="text-base">{t('common.update', { name: skillName })}</DialogTitle><DialogDescription className="text-xs leading-5">{t('common.theLocalSkillFolderIsReplacedWithTheLatestContentFromItsSourceASnapshotIsTakenFirst')}</DialogDescription></DialogHeader>
      <div className={styles.updatePreview}>
        <div className={styles.updateConfirmSource}><span>{t('common.sourceLabel')}</span><code>{source || 'GitHub'}</code></div>
        {!diff && !diffError && <p className={styles.updatePreviewState}>{t('skills.updatePreviewLoading')}</p>}
        {diffError && <p className={styles.updatePreviewState}>{diffError}</p>}
        {diff && <>
          <div className={styles.updatePreviewSummary}>
            <span><strong>{diff.added}</strong>{t('skills.filesAdded')}</span>
            <span><strong>{diff.modified}</strong>{t('skills.filesModified')}</span>
            <span><strong>{diff.conflicted}</strong>{t('skills.filesConflicted')}</span>
            <span><strong>{diff.localOnly}</strong>{t('skills.filesLocalOnly')}</span>
          </div>
          {resolvable.length > 0 && <div className={styles.updateResolveAll}><span>{t('skills.resolveAll')} · {t('skills.resolvedProgress', { done: resolved.size, total: resolvable.length })}</span><div><Button type="button" size="xs" variant="ghost" onClick={() => setAll('local')}>{t('skills.resolveAllLocal')}</Button><Button type="button" size="xs" variant="ghost" onClick={() => setAll('upstream')}>{t('skills.resolveAllOfficial')}</Button>{resolvable.some((file) => file.status !== 'local') && <Button type="button" size="xs" variant="ghost" onClick={() => setAll('both')}>{t('skills.resolveAllBoth')}</Button>}</div></div>}
          {!diff.hasBaseline && <div className={styles.updatePreviewNotice}><GitCompareArrows size={13} aria-hidden="true" /><span>{t('skills.noBaselineDescription')}</span></div>}
          {diff.localOnly > 0 && <div className={styles.updatePreviewNotice}><FolderLock size={13} aria-hidden="true" /><span>{t('skills.updateLocalFilesKept', { n: diff.localOnly })}</span></div>}
          {diff.conflicted > 0 && <div className={`${styles.updatePreviewNotice} ${styles.updatePreviewWarn}`}><AlertTriangle size={13} aria-hidden="true" /><span>{t('skills.updateConflictedFilesKept', { n: diff.conflicted })}</span></div>}
          {files.length > 0 && <div className={styles.updatePreviewDiffList}>{files.slice(0, 20).map((file) => {
            const label = file.binary ? `${fileStatusLabel(file.status, Boolean(diff?.hasBaseline))} · ${t('skills.binaryFile')}` : file.truncated ? `${fileStatusLabel(file.status, Boolean(diff?.hasBaseline))} · ${t('skills.diffTruncated')}` : fileStatusLabel(file.status, Boolean(diff?.hasBaseline))
            const loaded = fileDiffs[file.path]
            const lines: DiffLine[] = (loaded?.lines ?? []).map((line) => ({ content: line.text, oldNumber: line.oldLine, newNumber: line.newLine, type: line.kind === 'add' ? 'addition' : line.kind === 'del' ? 'deletion' : 'context' }))
            return <div key={`${file.status}:${file.path}`} className={styles.updatePreviewFileBlock} onPointerEnter={() => void loadFileDiff(file)}>
              <FileDiff filename={`${file.path}  ·  ${label}`} additions={loaded?.additions} deletions={loaded?.deletions} lines={lines} defaultOpen={false} loading={loadingFiles.has(file.path)} loadingLabel={t('skills.diffLoading')} onOpenChange={(open) => { if (open) void loadFileDiff(file) }} />
              {needsResolution(file) && <div className={styles.updateResolutionRow}><span>{t('skills.resolveChange')}</span><div role="group" aria-label={t('skills.resolveChange')}>{[['local', 'skills.resolveLocal'], ['upstream', file.status === 'local' ? 'skills.resolveDeleteLocal' : 'skills.resolveUpstream'], ...(file.status !== 'local' ? [['both', 'skills.resolveBoth']] : [])].map(([value, key]) => <Button key={value} type="button" size="xs" variant={(resolutions[file.path] ?? 'local') === value ? 'secondary' : 'ghost'} onClick={() => choose(file.path, value)}>{t(key)}</Button>)}{resolved.has(file.path) && <span className={styles.resolvedMark}><Check size={11} aria-hidden="true" />{t('skills.resolved')}</span>}</div></div>}
            </div>
          })}{files.length > 20 && <div className={styles.updatePreviewMore}>{t('skills.moreFiles', { n: files.length - 20 })}</div>}</div>}
        </>}
      </div>
      {error && <p className={`${styles.planError} px-6`} role="alert">{error}</p>}
      <DialogFooter><Button size="sm" variant="outline" onClick={onClose} disabled={applying}>{t('common.cancel')}</Button><Button size="sm" disabled={applying || (!diff && !diffError)} onClick={() => void update()}>{applying ? t('common.updating') : t('common.updateAction')}</Button></DialogFooter>
    </DialogContent>

  </Dialog>
}
