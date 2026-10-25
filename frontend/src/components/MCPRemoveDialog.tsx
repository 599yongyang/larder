import { useState } from 'react'
import { Checkbox } from '@/components/ui/checkbox'
import { Button } from '@/components/ui/button'
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { AgentIcon } from './AgentIcon'
import { backend } from '../backend'
import styles from '../App.module.css'
import { t } from '../i18n'
import { toastError, toastSuccess } from '../lib/toast'
import type { MCPServer } from '../types'

export function MCPRemoveDialog({ server, onClose, onApplied }: { server: MCPServer; onClose: () => void; onApplied: () => void }) {
  const [paths, setPaths] = useState<Set<string>>(() => new Set(server.agents.filter((agent) => agent.enabled).map((agent) => agent.path)))
  const [busy, setBusy] = useState(false)
  const toggle = (path: string) => setPaths((current) => { const next = new Set(current); if (next.has(path)) next.delete(path); else next.add(path); return next })
  const remove = async () => {
    setBusy(true)
    try {
      const plan = await backend.planMCPRemove(server.id, [...paths])
      const operation = await backend.applyPlan(plan.id) as { id: string }
      onApplied()
      toastSuccess(t('mcp.removeMcpConfig'), server.name, {
        label: t('skills.undo'),
        onClick: () => {
          void backend.rollbackOperation(operation.id).then(() => { onApplied(); toastSuccess(t('history.restored'), server.name) }).catch((error) => toastError(error, t('skills.undoFailed')))
        },
      })
    } catch (error) {
      toastError(error, t('mcp.failedToCreateMcpRemovalPlan'))
    } finally {
      setBusy(false)
    }
  }
  return <Dialog open onOpenChange={(open) => { if (!open && !busy) onClose() }}>
    <DialogContent className="max-w-lg">
      <DialogHeader><DialogTitle className="text-base">{t('mcp.remove', { name: server.name })}</DialogTitle><DialogDescription className="text-xs leading-5">{t('mcp.removesTheServerOnlyFromSelectedAgentConfigsCommandsAndFilesAreNotDeletedConfigsAreBackedUpFirst')}</DialogDescription></DialogHeader>
      <div className="px-6 py-2"><div className={styles.mcpRemoveList}>{server.agents.filter((agent) => agent.enabled).map((agent) => <label key={`${agent.label}:${agent.path}`} className={styles.mcpRemoveRow}><Checkbox checked={paths.has(agent.path)} onCheckedChange={() => toggle(agent.path)} /><AgentIcon name={agent.label} className="size-4" /><span><strong>{agent.label}</strong><small>{agent.path}</small></span></label>)}</div></div>
      <DialogFooter><Button size="sm" variant="outline" onClick={onClose} disabled={busy}>{t('common.cancel')}</Button><Button size="sm" variant="destructive" disabled={busy || paths.size === 0} onClick={() => void remove()}>{busy ? t('mcp.generating') : t('common.remove')}</Button></DialogFooter>
    </DialogContent>
  </Dialog>
}
