import { useState } from 'react'
import { Button } from '@/components/ui/button'
import { Checkbox } from '@/components/ui/checkbox'
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { Input } from '@/components/ui/input'
import { AgentIcon } from './AgentIcon'
import { backend } from '../backend'
import styles from '../App.module.css'
import { t } from '../i18n'
import { backendErrorMessage } from '../lib/backendError'
import { toastSuccess } from '../lib/toast'
import type { MCPServer } from '../types'

export function MCPEditDialog({ server, onClose, onSaved }: { server: MCPServer; onClose: () => void; onSaved: () => void }) {
  const [name, setName] = useState(server.name)
  const [transport, setTransport] = useState<'stdio' | 'http'>(server.url ? 'http' : 'stdio')
  const [command, setCommand] = useState(server.command ?? '')
  const [args, setArgs] = useState((server.args ?? []).join(' '))
  const [url, setURL] = useState(server.url ?? '')
  const [paths, setPaths] = useState<string[]>(server.agents.map((agent) => agent.path))
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')
  const valid = Boolean(name.trim()) && paths.length > 0 && Boolean(transport === 'stdio' ? command.trim() : url.trim())
  const save = async () => {
    if (!valid) return
    setBusy(true); setError('')
    try {
      const plan = await backend.planMCPUpdate(server.id, name.trim(), transport === 'stdio' ? command.trim() : '', transport === 'stdio' && args.trim() ? args.trim().split(/\s+/) : [], transport === 'http' ? url.trim() : '', paths)
      await backend.applyPlan(plan.id)
      toastSuccess(t('mcp.mcpConfigurationUpdated'), name.trim())
      onSaved(); onClose()
    } catch (cause) {
      setError(backendErrorMessage(cause, t('mcp.failedToUpdateMcp')))
    } finally { setBusy(false) }
  }
  return <Dialog open onOpenChange={(next) => { if (!next && !busy) onClose() }}>
    <DialogContent className="max-w-[540px] gap-0 p-0">
      <DialogHeader className="border-b px-6 py-4 pr-12"><DialogTitle className="text-base">{t('common.edit', { name: server.name })}</DialogTitle><DialogDescription className="text-xs">{t('mcp.editTheCommandOrUrlExistingEnvironmentValuesAreKept')}</DialogDescription></DialogHeader>
      <div className={styles.mcpCustomBody}>
        <div className={styles.mcpCustomRow}><span>{t('common.name')}</span><Input value={name} onChange={(event) => setName(event.target.value)} /></div>
        <div className={styles.mcpCustomRow}><span>{t('common.type')}</span><div className={styles.mcpTransport}><button type="button" className={transport === 'stdio' ? styles.mcpTransportActive : styles.mcpTransportButton} onClick={() => setTransport('stdio')}>stdio</button><button type="button" className={transport === 'http' ? styles.mcpTransportActive : styles.mcpTransportButton} onClick={() => setTransport('http')}>http</button></div></div>
        {transport === 'stdio' ? <><div className={styles.mcpCustomRow}><span>{t('common.commandLabel')}</span><Input value={command} onChange={(event) => setCommand(event.target.value)} /></div><div className={styles.mcpCustomRow}><span>{t('common.arguments')}</span><Input value={args} onChange={(event) => setArgs(event.target.value)} /></div></> : <div className={styles.mcpCustomRow}><span>URL</span><Input value={url} onChange={(event) => setURL(event.target.value)} /></div>}
        <div className={styles.mcpCustomColumn}><span>{t('common.writeToAgentConfig')}</span><div className={styles.mcpAddTargets}>{server.agents.filter((agent) => agent.enabled).map((agent) => <label key={`${agent.label}:${agent.path}`} className={styles.discoverAgent}><Checkbox checked={paths.includes(agent.path)} onCheckedChange={(checked) => setPaths((current) => checked ? [...new Set([...current, agent.path])] : current.filter((path) => path !== agent.path))} /><AgentIcon name={agent.label} className="size-4" /><span>{agent.label}</span></label>)}</div></div>
        {error && <p className={styles.planError} role="alert">{error}</p>}
      </div>
      <DialogFooter className="border-t px-6 py-2.5"><Button size="sm" variant="outline" disabled={busy} onClick={onClose}>{t('common.cancel')}</Button><Button size="sm" disabled={busy || !valid} onClick={() => void save()}>{busy ? t('common.saving') : t('common.save')}</Button></DialogFooter>
    </DialogContent>
  </Dialog>
}
