import { useMemo, useState } from 'react'
import { AlertTriangle, GitCompareArrows } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { backend } from '../backend'
import styles from '../App.module.css'
import { t } from '../i18n'
import { backendErrorMessage } from '../lib/backendError'
import { toastError } from '../lib/toast'
import type { Plan, SkillUpdateDiff } from '../types'

export type BatchUpdateItem = {
  plan: Plan
  diff?: SkillUpdateDiff
  error?: string
}

function needsResolution(diff: SkillUpdateDiff) {
  return diff.files.filter((file) => file.status === 'conflict' || file.status === 'local_modified' || file.status === 'local' || (!diff.hasBaseline && file.status === 'modified'))
}

export function BatchUpdateDialog({ items, onClose, onApplied }: { items: BatchUpdateItem[]; onClose: () => void; onApplied: () => void }) {
  const [applying, setApplying] = useState(false)
  const [progress, setProgress] = useState(0)
  const [strategy, setStrategy] = useState<'local' | 'official'>('local')
  const ready = items.length > 0 && items.every((item) => item.diff)
  const totalChanges = items.reduce((sum, item) => sum + (item.plan.changes?.length ?? 0), 0)
  const resolvable = useMemo(() => items.reduce((sum, item) => sum + (item.diff ? needsResolution(item.diff).length : 0), 0), [items])
  const conflicts = useMemo(() => items.reduce((sum, item) => sum + (item.diff?.conflicted ?? 0), 0), [items])
  const noBaseline = useMemo(() => items.filter((item) => item.diff && !item.diff.hasBaseline).length, [items])

  const resolutionsFor = (diff: SkillUpdateDiff) => Object.fromEntries(needsResolution(diff).map((file) => [file.path, strategy === 'official' ? 'upstream' : 'local']))

  const apply = async () => {
    if (applying || !ready) return
    setApplying(true)
    const failures: string[] = []
    for (let index = 0; index < items.length; index += 1) {
      const item = items[index]
      try {
        await backend.applyPlanWithResolutions(item.plan.id, item.diff ? resolutionsFor(item.diff) : {})
        setProgress(index + 1)
      } catch (cause) {
        failures.push(`${item.plan.skillName || item.plan.title}: ${backendErrorMessage(cause, t('common.batchOperationFailed'))}`)
      }
    }
    setApplying(false)
    if (failures.length > 0) {
      toastError(failures[0], t('skills.batchUpdatePartialFailure', { n: failures.length }))
    }
    onApplied()
  }

  return <Dialog open onOpenChange={(open) => { if (!open && !applying) onClose() }}>
    <DialogContent className="flex max-h-[84vh] max-w-2xl flex-col gap-0 overflow-hidden p-0">
      <DialogHeader className="border-b px-5 py-4 pr-12">
        <DialogTitle className="text-base">{t('skills.batchUpdateTitle', { n: items.length })}</DialogTitle>
        <DialogDescription className="max-w-lg text-xs leading-5">{t('skills.batchUpdateDescription')}</DialogDescription>
      </DialogHeader>
      <div className="min-h-0 flex-1 space-y-4 overflow-y-auto px-5 py-4">
        <div className={styles.batchUpdateSummary}>
          <span>{t('common.changeCount', { n: totalChanges })}</span>
          <span>{t('skills.batchResolvableChanges', { n: resolvable })}</span>
          <span className={conflicts > 0 ? styles.batchUpdateWarn : ''}><AlertTriangle size={12} aria-hidden="true" />{t('skills.batchConflicts', { n: conflicts })}</span>
          {noBaseline > 0 && <span>{t('skills.batchNoBaseline', { n: noBaseline })}</span>}
        </div>
        <div className={styles.batchUpdateStrategy}>
          <div><strong>{t('skills.batchDefaultResolution')}</strong><span>{t('skills.batchDefaultResolutionHint')}</span></div>
          <div role="group" aria-label={t('skills.batchDefaultResolution')}>
            <Button type="button" size="xs" variant={strategy === 'local' ? 'secondary' : 'ghost'} onClick={() => setStrategy('local')}>{t('skills.resolveAllLocal')}</Button>
            <Button type="button" size="xs" variant={strategy === 'official' ? 'secondary' : 'ghost'} onClick={() => setStrategy('official')}>{t('skills.resolveAllOfficial')}</Button>
          </div>
        </div>
        <div className={styles.batchList}>{items.map((item) => {
          const diff = item.diff
          const itemConflicts = diff?.conflicted ?? 0
          const itemLocal = diff ? needsResolution(diff).length : 0
          return <div key={item.plan.id} className={styles.batchUpdateRow}>
            <div className={styles.batchUpdateName}><strong>{item.plan.skillName || item.plan.title}</strong>{item.error ? <span className={styles.batchUpdateError}>{item.error}</span> : <span>{diff ? t('skills.batchFileCount', { n: diff.files.length }) : t('skills.batchPreviewFailed')}</span>}</div>
            {diff ? <div className={styles.batchUpdateMeta}><span>{t('skills.filesAddedShort')} {diff.added}</span><span>{t('skills.filesModifiedShort')} {diff.modified}</span><span className={itemConflicts > 0 ? styles.batchUpdateWarn : ''}>{t('skills.filesConflictedShort')} {itemConflicts}</span><span>{t('skills.filesLocalShort')} {itemLocal}</span></div> : <GitCompareArrows size={14} aria-hidden="true" />}
          </div>
        })}</div>
        {!ready && <p className={styles.planError} role="alert">{t('skills.batchPreviewIncomplete')}</p>}
        {applying && <p className={styles.resultCount}>{t('common.processing', { progress, total: items.length })}</p>}
      </div>
      <DialogFooter className="border-t px-5 py-3"><Button size="sm" variant="outline" onClick={onClose} disabled={applying}>{t('common.close')}</Button><Button size="sm" disabled={applying || !ready} onClick={() => void apply()}>{applying ? t('common.processingProgress', { progress, total: items.length }) : t('common.confirmAndChange')}</Button></DialogFooter>
    </DialogContent>
  </Dialog>
}
