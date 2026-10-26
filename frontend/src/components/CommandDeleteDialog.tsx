import { useState } from 'react'
import { Loader2, Trash2 } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { backend } from '../backend'
import { backendErrorMessage } from '../lib/backendError'
import { toastSuccess } from '../lib/toast'
import { t } from '../i18n'
import styles from '../App.module.css'
import type { Command } from '../types'

export function CommandDeleteDialog({ command, onClose, onDeleted }: { command: Command; onClose: () => void; onDeleted: () => void }) {
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')
  const remove = async () => {
    setBusy(true); setError('')
    try {
      const plan = await backend.planCommandDelete(command.path)
      await backend.applyCommandDelete(plan.id)
      toastSuccess(t('commands.deleted'))
      onDeleted(); onClose()
    } catch (cause) {
      setError(backendErrorMessage(cause, t('commands.deleteFailed')))
      setBusy(false)
    }
  }
  return <Dialog open onOpenChange={(open) => { if (!open && !busy) onClose() }}>
    <DialogContent className="max-w-lg gap-0 p-0">
      <DialogHeader className="border-b px-6 py-4 pr-14"><DialogTitle className="flex items-center gap-2 text-base"><Trash2 size={17} />{t('commands.deleteCommand')}</DialogTitle><DialogDescription className="text-xs">{command.path}</DialogDescription></DialogHeader>
      <div className="px-6 py-5"><p className={styles.commandDeleteWarning}>{t('commands.deleteHint')}</p>{error && <p className={styles.planError} role="alert">{error}</p>}</div>
      <DialogFooter className="border-t px-6 py-3"><Button size="sm" variant="outline" onClick={onClose} disabled={busy}>{t('common.cancel')}</Button><Button size="sm" variant="destructive" onClick={() => void remove()} disabled={busy}>{busy && <Loader2 size={13} className="animate-spin" />}{t('commands.deleteCommand')}</Button></DialogFooter>
    </DialogContent>
  </Dialog>
}
