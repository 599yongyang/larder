import { CheckCircle2, Download, Loader2, RotateCcw, TriangleAlert } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { t } from '../i18n'
import type { AppUpdateInfo } from '../types'

export type AppUpdateStage = 'idle' | 'checking' | 'available' | 'downloading' | 'ready' | 'uptodate' | 'error'

type Props = {
  stage: AppUpdateStage
  info?: AppUpdateInfo
  error?: string
  progress?: { written: number; total: number }
  currentVersion?: string
  onClose: () => void
  onInstall: () => void
  onRestart: () => void
  onRetry: () => void
}

const formatSize = (value: number) => value >= 1024 * 1024 ? `${(value / 1024 / 1024).toFixed(1)} MB` : value > 0 ? `${Math.ceil(value / 1024)} KB` : ''

export function AppUpdateDialog({ stage, info, error, progress, currentVersion, onClose, onInstall, onRestart, onRetry }: Props) {
  const percent = progress && progress.total > 0 ? Math.min(100, Math.round(progress.written / progress.total * 100)) : 0
  const title = stage === 'checking' ? t('appUpdate.checking')
    : stage === 'uptodate' ? t('appUpdate.upToDate')
      : stage === 'ready' ? t('appUpdate.readyTitle')
        : stage === 'error' ? t('appUpdate.errorTitle')
          : stage === 'downloading' ? t('appUpdate.downloading')
            : t('appUpdate.availableTitle')
  return <Dialog open={stage !== 'idle'} onOpenChange={(open) => { if (!open) onClose() }}>
    <DialogContent className="max-w-md">
      <DialogHeader>
        <DialogTitle className="flex items-center gap-2">
          {stage === 'checking' || stage === 'downloading' ? <Loader2 size={16} className="animate-spin text-muted-foreground" /> : stage === 'available' || stage === 'ready' ? <span className="grid size-7 place-items-center rounded-full bg-[var(--brand-soft)] text-[var(--brand)]"><Download size={15} /></span> : stage === 'uptodate' ? <CheckCircle2 size={16} className="text-[var(--success)]" /> : <TriangleAlert size={16} className="text-[var(--warning)]" />}
          {title}
        </DialogTitle>
        <DialogDescription>{stage === 'available' && info ? `Larder ${info.version}${currentVersion ? ` · ${t('appUpdate.currentVersion', { version: currentVersion })}` : ''}` : stage === 'ready' ? t('appUpdate.readyHint') : stage === 'uptodate' && currentVersion ? t('appUpdate.upToDateHint', { version: currentVersion }) : stage === 'error' ? t('appUpdate.errorHint') : stage === 'downloading' ? t('appUpdate.downloadingHint') : t('appUpdate.checkingHint')}</DialogDescription>
      </DialogHeader>
      {(stage === 'available' || stage === 'downloading') && info && <div className="space-y-3">
        {stage === 'available' && info.notes && <div className="max-h-44 overflow-y-auto whitespace-pre-wrap rounded-lg border bg-muted/30 p-3 text-xs leading-relaxed text-muted-foreground">{info.notes}</div>}
        {stage === 'available' && <div className="flex items-center justify-between text-[11px] text-muted-foreground"><span>{t('appUpdate.size')}</span><span>{formatSize(info.size) || '—'}</span></div>}
        {stage === 'downloading' && <div className="space-y-1.5"><div className="h-1.5 overflow-hidden rounded-full bg-muted"><div className="h-full rounded-full bg-[var(--brand)] transition-[width] duration-150" style={{ width: `${percent}%` }} /></div><div className="flex justify-between text-[10px] tabular-nums text-muted-foreground"><span>{t('appUpdate.downloading')}</span><span>{percent}%</span></div></div>}
      </div>}
      {stage === 'error' && <div className="rounded-lg border border-[color-mix(in_srgb,var(--destructive)_30%,var(--line))] bg-[color-mix(in_srgb,var(--destructive)_5%,var(--paper))] p-3 text-xs leading-relaxed text-destructive">{error || t('appUpdate.errorHint')}</div>}
      <DialogFooter className="gap-2">
        {stage === 'available' && <><Button variant="outline" size="sm" onClick={onClose}>{t('appUpdate.later')}</Button><Button size="sm" onClick={onInstall}><Download size={14} />{t('appUpdate.download')}</Button></>}
        {stage === 'ready' && <><Button variant="outline" size="sm" onClick={onClose}>{t('appUpdate.later')}</Button><Button size="sm" onClick={onRestart}><RotateCcw size={14} />{t('appUpdate.restart')}</Button></>}
        {stage === 'error' && <><Button variant="outline" size="sm" onClick={onClose}>{t('common.close')}</Button><Button size="sm" onClick={onRetry}>{t('common.retry')}</Button></>}
        {stage === 'uptodate' && <Button size="sm" onClick={onClose}>{t('common.close')}</Button>}
      </DialogFooter>
    </DialogContent>
  </Dialog>
}
