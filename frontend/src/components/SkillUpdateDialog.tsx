import { useEffect, useState } from 'react'
import { Check, GitCompareArrows, ShieldAlert } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { backend } from '../backend'
import styles from '../App.module.css'
import { t } from '../i18n'
import { backendErrorMessage } from '../lib/backendError'
import { toastError } from '../lib/toast'
import { FileDiff, type DiffLine } from './ai/file-diff'
import { SkillAuditInline } from './SkillAuditSection'
import { Checkbox } from '@/components/ui/checkbox'
import type { SkillAudit } from '../types'
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
  const [showAutomatic, setShowAutomatic] = useState(false)
  const [audit, setAudit] = useState<SkillAudit>()
  const [highRiskConfirmed, setHighRiskConfirmed] = useState(false)
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
  const automaticFiles = files.filter((file) => !needsResolution(file))
  const summary = diff ? [diff.added > 0 && `${diff.added} ${t('skills.filesAdded')}`, diff.modified > 0 && `${diff.modified} ${t('skills.filesModified')}`, diff.conflicted > 0 && `${diff.conflicted} ${t('skills.filesConflicted')}`, diff.localOnly > 0 && `${diff.localOnly} ${t('skills.filesLocalOnly')}`].filter(Boolean).join(' · ') : ''
  const choose = (path: string, value: string) => { setResolutions((current) => ({ ...current, [path]: value })); setResolved((current) => new Set(current).add(path)) }
  const setAll = (value: 'local' | 'upstream' | 'both') => { setResolutions(Object.fromEntries(resolvable.map((file) => [file.path, value === 'both' && file.status === 'local' ? 'local' : value]))); setResolved(new Set(resolvable.map((file) => file.path))) }
  const initialFiles = resolvable.length > 0 ? resolvable : automaticFiles
  const renderFile = (file: SkillUpdateFile) => {
    const label = file.binary ? `${fileStatusLabel(file.status, Boolean(diff?.hasBaseline))} · ${t('skills.binaryFile')}` : file.truncated ? `${fileStatusLabel(file.status, Boolean(diff?.hasBaseline))} · ${t('skills.diffTruncated')}` : fileStatusLabel(file.status, Boolean(diff?.hasBaseline))
    const loaded = fileDiffs[file.path]
    const lines: DiffLine[] = (loaded?.lines ?? []).map((line) => ({ content: line.text, oldNumber: line.oldLine, newNumber: line.newLine, type: line.kind === 'add' ? 'addition' : line.kind === 'del' ? 'deletion' : 'context' }))
    return <div key={`${file.status}:${file.path}`} className={styles.updatePreviewFileBlock} onPointerEnter={() => void loadFileDiff(file)}>
      <FileDiff filename={`${file.path}  ·  ${label}`} additions={loaded?.additions} deletions={loaded?.deletions} lines={lines} defaultOpen={false} loading={loadingFiles.has(file.path)} loadingLabel={t('skills.diffLoading')} onOpenChange={(open) => { if (open) void loadFileDiff(file) }} />
      {needsResolution(file) && <div className={styles.updateResolutionRow}><span>{t('skills.resolutionMode')}</span><div role="group" aria-label={t('skills.resolutionMode')}>{[['local', 'skills.resolveLocal'], ['upstream', file.status === 'local' ? 'skills.resolveDeleteLocal' : 'skills.resolveUpstream'], ...(file.status !== 'local' ? [['both', 'skills.resolveBoth']] : [])].map(([value, key]) => <Button key={value} type="button" size="xs" variant={(resolutions[file.path] ?? 'local') === value ? 'secondary' : 'ghost'} onClick={() => choose(file.path, value)}>{t(key)}</Button>)}{resolved.has(file.path) && <span className={styles.resolvedMark}><Check size={11} aria-hidden="true" />{t('skills.resolved')}</span>}</div></div>}
    </div>
  }
  return <Dialog open onOpenChange={(open) => { if (!open && !applying) onClose() }}>
    <DialogContent className="flex max-h-[84vh] max-w-[680px] flex-col gap-0 overflow-hidden">
      <DialogHeader><DialogTitle className="text-base">{t('common.update', { name: skillName })}</DialogTitle><DialogDescription className="flex items-center gap-1.5 text-xs leading-5"><code className="truncate rounded bg-[var(--panel)] px-1.5 py-0.5 text-[10px] text-[var(--ink-soft)]">{source || 'GitHub'}</code><span className="text-[var(--text-muted)]">·</span>{t('skills.updatePreviewIntro')}</DialogDescription></DialogHeader>
      <SkillAuditInline skillID={skillID} onAudit={setAudit} />
      <div className={styles.updatePreview}>
        {!diff && !diffError && <p className={styles.updatePreviewState}>{t('skills.updatePreviewLoading')}</p>}
        {diffError && <p className={styles.updatePreviewState}>{diffError}</p>}
        {diff && <>
          <div className={styles.updateSummaryLine}><span>{summary || t('skills.noFileChanges')}</span>{diff.files.length > 0 && <span className={styles.updateSummaryCount}>{diff.files.length} {t('skills.filesChanged')}</span>}</div>
          {!diff.hasBaseline && <div className={`${styles.updatePreviewNotice} ${styles.updateBaselineNotice}`}><GitCompareArrows size={13} aria-hidden="true" /><span>{t('skills.noBaselineDescription')}</span></div>}
          {resolvable.length > 1 && <div className={styles.updateDecisionBar}>
            <div className={styles.updateDecisionMeta}><strong>{t('skills.decisionCount', { n: resolvable.length })}</strong><span>{resolved.size === resolvable.length ? <><Check size={11} aria-hidden="true" />{t('skills.allResolved')}</> : t('skills.resolvedProgress', { done: resolved.size, total: resolvable.length })}</span></div>
            <Select key={`bulk-${resolved.size}`} onValueChange={(value) => setAll(value as 'local' | 'upstream' | 'both')}>
              <SelectTrigger size="sm" className={styles.updateBatchSelect} aria-label={t('skills.batchApply')}><SelectValue placeholder={t('skills.batchApply')} /></SelectTrigger>
              <SelectContent position="popper" align="end" className="min-w-[132px]">
                <SelectItem value="local">{t('skills.resolveAllLocal')}</SelectItem>
                <SelectItem value="upstream">{t('skills.resolveAllOfficial')}</SelectItem>
                {resolvable.some((file) => file.status !== 'local') && <SelectItem value="both">{t('skills.resolveAllBoth')}</SelectItem>}
              </SelectContent>
            </Select>
          </div>}
          {files.length > 0 && <>
            <div className={styles.updatePreviewDiffList}>{initialFiles.map(renderFile)}</div>
            {resolvable.length > 0 && automaticFiles.length > 0 && <Button type="button" size="xs" variant="ghost" className={styles.automaticFilesToggle} onClick={() => setShowAutomatic((value) => !value)}>{showAutomatic ? t('skills.hideAutomaticFiles', { n: automaticFiles.length }) : t('skills.showAutomaticFiles', { n: automaticFiles.length })}</Button>}
            {showAutomatic && automaticFiles.length > 0 && <div className={styles.updatePreviewDiffList}>{automaticFiles.map(renderFile)}</div>}
          </>}
        </>}
      </div>
      {audit?.verdict === 'fail' && <label className={styles.auditGate}><Checkbox checked={highRiskConfirmed} onCheckedChange={(checked) => setHighRiskConfirmed(Boolean(checked))} /><ShieldAlert size={14} aria-hidden="true" /><span>{t('skills.auditHighRiskConfirm')}</span></label>}
      {error && <p className={`${styles.planError} px-6`} role="alert">{error}</p>}
      <DialogFooter><Button size="sm" variant="outline" onClick={onClose} disabled={applying}>{t('common.cancel')}</Button><Button size="sm" disabled={applying || (!diff && !diffError) || (audit?.verdict === 'fail' && !highRiskConfirmed)} onClick={() => void update()}>{applying ? t('common.updating') : t('common.updateAction')}</Button></DialogFooter>
    </DialogContent>
  </Dialog>
}
