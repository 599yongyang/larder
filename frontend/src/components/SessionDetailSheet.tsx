import { useState } from 'react'
import { Copy, FolderOpen, FolderSearch, Loader2, MessageSquare } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Sheet, SheetContent, SheetDescription, SheetHeader, SheetTitle } from '@/components/ui/sheet'
import { AgentIcon } from './AgentIcon'
import { formatCompactNumber, formatRelativeTime } from '../lib/format'
import { toastError, toastSuccess } from '../lib/toast'
import { backend } from '../backend'
import { t } from '../i18n'
import styles from '../App.module.css'
import type { Session, SessionMessage } from '../types'

export function SessionDetailSheet({ session, onClose, onOpenProject }: { session: Session; onClose: () => void; onOpenProject?: () => void }) {
  const [copied, setCopied] = useState(false)
  const [revealing, setRevealing] = useState(false)
  const [messages, setMessages] = useState<SessionMessage[]>()
  const [messagesLoading, setMessagesLoading] = useState(false)
  const [messagesUnavailable, setMessagesUnavailable] = useState(false)
  const copyPath = async () => {
    try {
      await navigator.clipboard.writeText(session.path)
      setCopied(true)
      toastSuccess(t('common.copied', { label: t('common.path') }))
    } catch (error) {
      toastError(error, t('common.failedToCopy', { label: t('common.path') }))
    }
  }
  const reveal = async () => {
    setRevealing(true)
    try { await backend.revealSession(session.path); } catch (error) { toastError(error, t('sessions.failedToReveal')) } finally { setRevealing(false) }
  }
  const loadMessages = async () => {
    setMessagesLoading(true)
    setMessagesUnavailable(false)
    try { setMessages(await backend.sessionMessages(session.path, 20)) } catch { setMessagesUnavailable(true) } finally { setMessagesLoading(false) }
  }
  const duration = (() => {
    const start = Date.parse(session.startedAt)
    const end = Date.parse(session.lastAt)
    if (!Number.isFinite(start) || !Number.isFinite(end) || end <= start) return ''
    const minutes = Math.round((end - start) / 60000)
    if (minutes < 60) return `${Math.max(1, minutes)} min`
    const hours = Math.floor(minutes / 60)
    return `${hours}h ${minutes % 60}m`
  })()
  return <Sheet open onOpenChange={(open) => { if (!open) onClose() }}>
    <SheetContent side="right" className="w-[460px] sm:max-w-[460px]">
      <SheetHeader className="gap-0 border-b px-5 py-4">
        <div className="flex items-start gap-3">
          <span className={styles.mcpHero}><MessageSquare size={18} /></span>
          <div className="min-w-0 flex-1">
            <SheetTitle className="truncate text-[15px]">{session.title || session.id}</SheetTitle>
            <SheetDescription className="mt-1 flex items-center gap-1.5 text-[11px]"><AgentIcon name={session.agent} className="size-3.5" />{session.agent}</SheetDescription>
          </div>
        </div>
      </SheetHeader>
      <div className="min-h-0 flex-1 overflow-y-auto p-0">
        <section className={styles.detailSection}>
          <h3>{t('sessions.metadata')}</h3>
          <dl className={styles.definition}>
            <dt>{t('sessions.project')}</dt><dd>{session.cwd || '—'}</dd>
            <dt>{t('sessions.messages')}</dt><dd>{formatCompactNumber(session.messages)}</dd>
            <dt>{t('sessions.model')}</dt><dd>{session.model || '—'}</dd>
            {(session.tokensIn > 0 || session.tokensOut > 0) && <><dt>{t('sessions.tokens')}</dt><dd>{formatCompactNumber(session.tokensIn)} {t('sessions.tokenInput')} · {formatCompactNumber(session.tokensOut)} {t('sessions.tokenOutput')}</dd></>}
            <dt>{t('sessions.started')}</dt><dd>{session.startedAt ? formatRelativeTime(session.startedAt) : '—'}</dd>
            <dt>{t('sessions.lastActive')}</dt><dd>{session.lastAt ? formatRelativeTime(session.lastAt) : '—'}</dd>
            {duration && <><dt>{t('sessions.duration')}</dt><dd>{duration}</dd></>}
            <dt>{t('sessions.path')}</dt><dd><code>{session.path}</code></dd>
          </dl>
          <div className="mt-3 flex gap-2">
            <Button size="xs" variant="outline" onClick={() => void copyPath()}><Copy size={12} />{copied ? t('sessions.pathCopied') : t('common.copyPath')}</Button><Button size="xs" variant="ghost" onClick={() => void reveal()} disabled={revealing}><FolderSearch size={12} />{t('common.revealInFinder')}</Button>{onOpenProject && session.cwd && <Button size="xs" variant="ghost" onClick={onOpenProject}><FolderOpen size={12} />{t('sessions.openProject')}</Button>}
          </div>
        </section>
        <section className={styles.detailSection}>
          <div className="flex items-center justify-between gap-3">
            <h3>{t('sessions.previewTitle')}</h3>
            {!messages && !messagesLoading && <Button size="xs" variant="outline" onClick={() => void loadMessages()}>{messagesUnavailable ? t('common.retry') : t('sessions.previewMessages')}</Button>}
          </div>
          {messagesLoading ? <div className={styles.sessionMessagesLoading}><Loader2 size={15} className="animate-spin" />{t('sessions.messagesLoading')}</div> : messagesUnavailable ? <p className={styles.assetDescription}>{t('sessions.messagesUnavailable')}</p> : messages ? <div className={styles.sessionMessages}>{messages.map((message, index) => <div key={index} className={styles.sessionMessage}><div><strong>{message.role === 'user' ? t('sessions.user') : t('sessions.assistant')}</strong><time>{message.at ? formatRelativeTime(message.at) : ''}</time></div><p>{message.text}</p></div>)}</div> : <p className={styles.assetDescription}>{t('sessions.previewHint')}</p>}
        </section>
        <section className={styles.detailSection}>
          <p className={styles.assetDescription}>{t('sessions.metadataOnlyHint')}</p>
        </section>
      </div>
    </SheetContent>
  </Sheet>
}
