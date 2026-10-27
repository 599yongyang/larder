import { useState } from 'react'
import { Copy, Terminal } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { backend } from '../backend'
import { copyText } from '../lib/clipboard'
import { t } from '../i18n'
import { toastError, toastSuccess } from '../lib/toast'
import type { Session } from '../types'

export function canResumeSession(agent: string) {
  return agent === 'Claude Code' || agent === 'Codex'
}

export function SessionQuickActions({ session }: { session: Session }) {
  const [opening, setOpening] = useState(false)
  const openTerminal = async (event: React.MouseEvent) => {
    event.stopPropagation()
    setOpening(true)
    try { await backend.openSessionInTerminal(session) } catch (error) { toastError(error, t('sessions.terminalOpenFailed')) } finally { setOpening(false) }
  }
  const copyPath = async (event: React.MouseEvent) => {
    event.stopPropagation()
    try {
      await copyText(session.path)
      toastSuccess(t('common.copied', { label: t('common.path') }))
    } catch (error) { toastError(error, t('common.failedToCopy', { label: t('common.path') })) }
  }
  return <span className="flex items-center gap-0.5">
    {canResumeSession(session.agent) && <Button type="button" size="xs" variant="ghost" disabled={opening} title={t('sessions.openInTerminal')} aria-label={t('sessions.openInTerminal')} onClick={(event) => void openTerminal(event)}><Terminal size={13} /></Button>}
    <Button type="button" size="xs" variant="ghost" title={t('common.copyPath')} aria-label={t('common.copyPath')} onClick={(event) => void copyPath(event)}><Copy size={13} /></Button>
  </span>
}
