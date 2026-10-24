import { useState } from 'react'
import { Button } from '@/components/ui/button'
import { Checkbox } from '@/components/ui/checkbox'
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { Input } from '@/components/ui/input'
import { AgentIcon } from './AgentIcon'
import { backend } from '../backend'
import styles from '../App.module.css'
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
      toastSuccess('MCP 配置已更新', name.trim())
      onSaved(); onClose()
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : '更新 MCP 失败')
    } finally { setBusy(false) }
  }
  return <Dialog open onOpenChange={(next) => { if (!next && !busy) onClose() }}>
    <DialogContent className="max-w-[540px] gap-0 p-0">
      <DialogHeader className="border-b px-6 py-4 pr-12"><DialogTitle className="text-base">编辑 {server.name}</DialogTitle><DialogDescription className="text-xs">修改命令或 URL；已有环境变量的值会保留。</DialogDescription></DialogHeader>
      <div className={styles.mcpCustomBody}>
        <div className={styles.mcpCustomRow}><span>名称</span><Input value={name} onChange={(event) => setName(event.target.value)} /></div>
        <div className={styles.mcpCustomRow}><span>类型</span><div className={styles.mcpTransport}><button type="button" className={transport === 'stdio' ? styles.mcpTransportActive : styles.mcpTransportButton} onClick={() => setTransport('stdio')}>stdio</button><button type="button" className={transport === 'http' ? styles.mcpTransportActive : styles.mcpTransportButton} onClick={() => setTransport('http')}>http</button></div></div>
        {transport === 'stdio' ? <><div className={styles.mcpCustomRow}><span>命令</span><Input value={command} onChange={(event) => setCommand(event.target.value)} /></div><div className={styles.mcpCustomRow}><span>参数</span><Input value={args} onChange={(event) => setArgs(event.target.value)} /></div></> : <div className={styles.mcpCustomRow}><span>URL</span><Input value={url} onChange={(event) => setURL(event.target.value)} /></div>}
        <div className={styles.mcpCustomColumn}><span>写入 Agent 配置</span><div className={styles.mcpAddTargets}>{server.agents.filter((agent) => agent.enabled).map((agent) => <label key={`${agent.label}:${agent.path}`} className={styles.discoverAgent}><Checkbox checked={paths.includes(agent.path)} onCheckedChange={(checked) => setPaths((current) => checked ? [...new Set([...current, agent.path])] : current.filter((path) => path !== agent.path))} /><AgentIcon name={agent.label} className="size-4" /><span>{agent.label}</span></label>)}</div></div>
        {error && <p className={styles.planError} role="alert">{error}</p>}
      </div>
      <DialogFooter className="border-t px-6 py-2.5"><Button size="sm" variant="outline" disabled={busy} onClick={onClose}>取消</Button><Button size="sm" disabled={busy || !valid} onClick={() => void save()}>{busy ? '保存中…' : '保存'}</Button></DialogFooter>
    </DialogContent>
  </Dialog>
}
