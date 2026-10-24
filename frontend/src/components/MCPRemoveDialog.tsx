import { useState } from 'react'
import { Checkbox } from '@/components/ui/checkbox'
import { Button } from '@/components/ui/button'
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { AgentIcon } from './AgentIcon'
import { backend } from '../backend'
import styles from '../App.module.css'
import { toastError } from '../lib/toast'
import type { MCPServer, Plan } from '../types'

export function MCPRemoveDialog({ server, onClose, onPlan }: { server: MCPServer; onClose: () => void; onPlan: (plan: Plan) => void }) {
  const [paths, setPaths] = useState<Set<string>>(() => new Set(server.agents.filter((agent) => agent.enabled).map((agent) => agent.path)))
  const [busy, setBusy] = useState(false)
  const toggle = (path: string) => setPaths((current) => { const next = new Set(current); if (next.has(path)) next.delete(path); else next.add(path); return next })
  const createPlan = async () => {
    setBusy(true)
    try {
      const plan = await backend.planMCPRemove(server.id, [...paths])
      onPlan(plan)
    } catch (error) {
      toastError(error, '生成 MCP 移除计划失败')
    } finally {
      setBusy(false)
    }
  }
  return <Dialog open onOpenChange={(open) => { if (!open && !busy) onClose() }}>
    <DialogContent className="max-w-lg">
      <DialogHeader><DialogTitle className="text-base">移除 {server.name}</DialogTitle><DialogDescription className="text-xs leading-5">只从选中的 Agent 配置中移除，不会删除命令或文件。执行前会备份配置。</DialogDescription></DialogHeader>
      <div className="px-6 py-2"><div className={styles.mcpRemoveList}>{server.agents.filter((agent) => agent.enabled).map((agent) => <label key={`${agent.label}:${agent.path}`} className={styles.mcpRemoveRow}><Checkbox checked={paths.has(agent.path)} onCheckedChange={() => toggle(agent.path)} /><AgentIcon name={agent.label} className="size-4" /><span><strong>{agent.label}</strong><small>{agent.path}</small></span></label>)}</div></div>
      <DialogFooter><Button size="sm" variant="outline" onClick={onClose} disabled={busy}>取消</Button><Button size="sm" variant="destructive" disabled={busy || paths.size === 0} onClick={() => void createPlan()}>{busy ? '生成中…' : '预览移除'}</Button></DialogFooter>
    </DialogContent>
  </Dialog>
}
