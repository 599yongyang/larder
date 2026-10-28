import { useEffect, useState } from 'react'
import { Archive, Download, Upload } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Checkbox } from '@/components/ui/checkbox'
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { t } from '../i18n'
import type { DataPackageInfo, DataPackageSelection } from '../types'

type Mode = 'export' | 'import'

const allCategories: Array<keyof DataPackageSelection> = ['settings', 'appState', 'mcp', 'instructions', 'commands', 'skills']

const emptySelection = (): DataPackageSelection => ({ settings: true, appState: true, mcp: true, instructions: true, commands: true, skills: true })

type Props = {
  mode: Mode
  open: boolean
  info?: DataPackageInfo
  busy?: boolean
  title?: string
  hint?: string
  confirmLabel?: string
  onClose: () => void
  onConfirm: (selection: DataPackageSelection) => void
}

export function DataPackageDialog({ mode, open, info, busy, title, hint, confirmLabel, onClose, onConfirm }: Props) {
  const [selection, setSelection] = useState<DataPackageSelection>(emptySelection)
  useEffect(() => {
    if (!open) return
    setSelection(info ? { ...emptySelection(), ...info.selection } : emptySelection())
  }, [open, info])
  const all = allCategories.every((key) => selection[key])
  const none = allCategories.every((key) => !selection[key])
  return <Dialog open={open} onOpenChange={(next) => { if (!next && !busy) onClose() }}>
    <DialogContent className="max-w-md">
      <DialogHeader>
        <DialogTitle className="flex items-center gap-2">{mode === 'export' ? <Download size={16} /> : <Upload size={16} />}{title || (mode === 'export' ? t('dataPackage.exportTitle') : t('dataPackage.importTitle'))}</DialogTitle>
        <DialogDescription>{hint || (mode === 'export' ? t('dataPackage.exportHint') : t('dataPackage.importHint'))}</DialogDescription>
      </DialogHeader>
      <div className="space-y-1">
        <label className="flex cursor-pointer items-center justify-between gap-3 rounded-md px-1 py-2 text-xs font-medium"><span>{t('dataPackage.all')}</span><Checkbox checked={all} onCheckedChange={(checked) => setSelection(checked ? emptySelection() : { settings: false, appState: false, mcp: false, instructions: false, commands: false, skills: false })} /></label>
        <div className="h-px bg-border" />
        {allCategories.map((key) => <label key={key} className="flex cursor-pointer items-center justify-between gap-3 rounded-md px-1 py-2"><span className="grid gap-0.5"><strong className="text-[11px] font-medium text-foreground">{t(`dataPackage.category.${key}`)}</strong><small className="text-[10px] text-muted-foreground">{t(`dataPackage.category.${key}Hint`)}</small></span><span className="flex items-center gap-2"><span className="text-[10px] tabular-nums text-muted-foreground">{info?.categories?.[key] ?? ''}</span><Checkbox checked={selection[key]} onCheckedChange={(checked) => setSelection((current) => ({ ...current, [key]: Boolean(checked) }))} /></span></label>)}
      </div>
      {selection.mcp && <p className="rounded-md border border-amber-500/25 bg-amber-500/5 p-2 text-[10px] leading-relaxed text-muted-foreground">{t('dataPackage.mcpWarning')}</p>}
      <DialogFooter>
        <Button variant="outline" size="sm" disabled={busy} onClick={onClose}>{t('common.cancel')}</Button>
        <Button size="sm" disabled={busy || none} onClick={() => onConfirm(selection)}>{mode === 'export' ? <Archive size={14} /> : <Upload size={14} />}{busy ? t('dataPackage.working') : confirmLabel || (mode === 'export' ? t('dataPackage.export') : t('dataPackage.import'))}</Button>
      </DialogFooter>
    </DialogContent>
  </Dialog>
}
