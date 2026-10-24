import { useState } from 'react'
import { Bot, RotateCcw, Trash2 } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { AgentIcon } from './AgentIcon'
import styles from '../App.module.css'
import { t } from '../i18n'
import type { IgnoredAgent, Root } from '../types'

export function AgentManagerDialog({ agents, ignored, onClose, onIgnore, onRestore, onCleanup }: { agents: Root[]; ignored: IgnoredAgent[]; onClose: () => void; onIgnore: (root: Root) => Promise<void>; onRestore: (path: string) => Promise<void>; onCleanup: (path: string) => Promise<void> }) {
  const [busy, setBusy] = useState('')
  const [cleanTarget, setCleanTarget] = useState<IgnoredAgent>()
  const run = async (key: string, action: () => Promise<void>) => { setBusy(key); try { await action() } finally { setBusy('') } }
  return <Dialog open onOpenChange={(open) => { if (!open && !busy) onClose() }}>
    <DialogContent className="max-w-2xl gap-0 p-0">
      <DialogHeader className="border-b px-6 py-4 pr-14">
        <DialogTitle className="text-base">{t('common.manageAgents')}</DialogTitle>
        <DialogDescription className="max-w-xl text-xs leading-5">{t('common.ignoringSkipsScanningAndDisplayOnlyFilesAreNotDeletedAndCanBeRestoredAtAnyTime')}</DialogDescription>
      </DialogHeader>
      <div className="max-h-[44vh] overflow-y-auto px-6 py-4">
        <section className={styles.issueSection}>
          <div className={styles.issueSectionHead}><div><h3>{t('common.scanning')}</h3><p>{t('common.theseDirectoriesAppearInTheAgentsListAndSkillVisibility')}</p></div><span className={styles.footerHint}>{t('common.itemUnit', { count: agents.length })}</span></div>
          <div className={styles.agentManagerList}>{agents.map((agent) => <div key={agent.id} className={styles.agentManagerRow}>
            <AgentIcon name={agent.logicalAgent || agent.label} className="size-5" />
            <div className={styles.agentManagerInfo}><strong>{agent.label}</strong><small>{agent.path}</small></div>
            <Badge variant="outline">{t('common.changeCount', { n: agent.entryCount })}</Badge>{agent.sharedSkills && <Badge variant="info">{t('common.sharedDiscovery')}</Badge>}
            <Button variant="ghost" size="xs" disabled={Boolean(busy)} onClick={() => void run(agent.id, () => onIgnore(agent))}>{busy === agent.id ? t('common.working') : t('common.ignore')}</Button>
          </div>)}</div>
        </section>
        {ignored.length > 0 && <section className={styles.issueSection}>
          <div className={styles.issueSectionHead}><div><h3>{t('common.ignored')}</h3><p>{t('common.theseDirectoriesAreNotScannedButTheirFilesStayWhereTheyAre')}</p></div><span className={styles.footerHint}>{t('common.itemUnit', { count: ignored.length })}</span></div>
          <div className={styles.agentManagerList}>{ignored.map((agent) => <div key={agent.path} className={styles.agentManagerRow}>
            <Bot className="size-5 text-muted-foreground" />
            <div className={styles.agentManagerInfo}><strong>{agent.label}</strong><small>{agent.path}</small></div>
            <Button variant="outline" size="xs" disabled={Boolean(busy)} onClick={() => void run(agent.path, () => onRestore(agent.path))}><RotateCcw size={13} />{busy === agent.path ? t('common.restoring') : t('common.restore')}</Button>
            <Button variant="ghost" size="xs" className="text-destructive" disabled={Boolean(busy)} onClick={() => setCleanTarget(agent)}><Trash2 size={13} />{t('common.cleanUp')}</Button>
          </div>)}</div>
        </section>}
        {agents.length === 0 && ignored.length === 0 && <div className={styles.emptyState}><strong>{t('common.noAgentDirectoriesFoundYet')}</strong><p>{t('common.theyWillBeListedHereAfterScanning')}</p></div>}
      </div>
      <DialogFooter className="border-t px-6 py-3"><Button size="sm" variant="outline" onClick={onClose} disabled={Boolean(busy)}>{t('common.close')}</Button></DialogFooter>
    </DialogContent>
    {cleanTarget && <Dialog open onOpenChange={(open) => { if (!open && !busy) setCleanTarget(undefined) }}>
      <DialogContent className="max-w-md">
        <DialogHeader><DialogTitle className="text-base">{t('common.cleanUpAgentLeftovers')}</DialogTitle><DialogDescription className="text-xs leading-5">{t('common.theWholeDirectoryIsMovedToTrashNotPermanentlyDeletedItCanBeRestoredFromFinder')}</DialogDescription></DialogHeader>
        <div className="px-6 py-2"><code className={styles.cleanupPath}>{cleanTarget.path}</code></div>
        <DialogFooter><Button size="sm" variant="outline" onClick={() => setCleanTarget(undefined)} disabled={Boolean(busy)}>{t('common.cancel')}</Button><Button size="sm" variant="destructive" disabled={Boolean(busy)} onClick={() => { const target = cleanTarget; setCleanTarget(undefined); void run(`clean:${target.path}`, () => onCleanup(target.path)) }}>{busy.startsWith('clean:') ? t('common.cleaningUp') : t('common.moveToTrash')}</Button></DialogFooter>
      </DialogContent>
    </Dialog>}
  </Dialog>
}
