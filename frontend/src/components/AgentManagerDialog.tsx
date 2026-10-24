import { useState } from 'react'
import { Bot, RotateCcw, Trash2 } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { AgentIcon } from './AgentIcon'
import styles from '../App.module.css'
import type { IgnoredAgent, Root } from '../types'

export function AgentManagerDialog({ agents, ignored, onClose, onIgnore, onRestore, onCleanup }: { agents: Root[]; ignored: IgnoredAgent[]; onClose: () => void; onIgnore: (root: Root) => Promise<void>; onRestore: (path: string) => Promise<void>; onCleanup: (path: string) => Promise<void> }) {
  const [busy, setBusy] = useState('')
  const [cleanTarget, setCleanTarget] = useState<IgnoredAgent>()
  const run = async (key: string, action: () => Promise<void>) => { setBusy(key); try { await action() } finally { setBusy('') } }
  return <Dialog open onOpenChange={(open) => { if (!open && !busy) onClose() }}>
    <DialogContent className="max-w-2xl gap-0 p-0">
      <DialogHeader className="border-b px-6 py-4 pr-14">
        <DialogTitle className="text-base">管理 Agents</DialogTitle>
        <DialogDescription className="max-w-xl text-xs leading-5">忽略只会跳过扫描和展示，不删除文件，可随时恢复。</DialogDescription>
      </DialogHeader>
      <div className="max-h-[44vh] overflow-y-auto px-6 py-4">
        <section className={styles.issueSection}>
          <div className={styles.issueSectionHead}><div><h3>正在扫描</h3><p>这些目录会出现在左侧 Agents 列表和 Skills 可见性中。</p></div><span className={styles.footerHint}>{agents.length} 个</span></div>
          <div className={styles.agentManagerList}>{agents.map((agent) => <div key={agent.id} className={styles.agentManagerRow}>
            <AgentIcon name={agent.logicalAgent || agent.label} className="size-5" />
            <div className={styles.agentManagerInfo}><strong>{agent.label}</strong><small>{agent.path}</small></div>
            <Badge variant="outline">{agent.entryCount} 项</Badge>{agent.sharedSkills && <Badge variant="info">共享发现</Badge>}
            <Button variant="ghost" size="xs" disabled={Boolean(busy)} onClick={() => void run(agent.id, () => onIgnore(agent))}>{busy === agent.id ? '处理中…' : '忽略'}</Button>
          </div>)}</div>
        </section>
        {ignored.length > 0 && <section className={styles.issueSection}>
          <div className={styles.issueSectionHead}><div><h3>已忽略</h3><p>这些目录不会被扫描，但文件仍然保留在原位置。</p></div><span className={styles.footerHint}>{ignored.length} 个</span></div>
          <div className={styles.agentManagerList}>{ignored.map((agent) => <div key={agent.path} className={styles.agentManagerRow}>
            <Bot className="size-5 text-muted-foreground" />
            <div className={styles.agentManagerInfo}><strong>{agent.label}</strong><small>{agent.path}</small></div>
            <Button variant="outline" size="xs" disabled={Boolean(busy)} onClick={() => void run(agent.path, () => onRestore(agent.path))}><RotateCcw size={13} />{busy === agent.path ? '恢复中…' : '恢复'}</Button>
            <Button variant="ghost" size="xs" className="text-destructive" disabled={Boolean(busy)} onClick={() => setCleanTarget(agent)}><Trash2 size={13} />清理</Button>
          </div>)}</div>
        </section>}
        {agents.length === 0 && ignored.length === 0 && <div className={styles.emptyState}><strong>还没有发现 Agent 目录</strong><p>扫描后会在这里列出。</p></div>}
      </div>
      <DialogFooter className="border-t px-6 py-3"><Button size="sm" variant="outline" onClick={onClose} disabled={Boolean(busy)}>关闭</Button></DialogFooter>
    </DialogContent>
    {cleanTarget && <Dialog open onOpenChange={(open) => { if (!open && !busy) setCleanTarget(undefined) }}>
      <DialogContent className="max-w-md">
        <DialogHeader><DialogTitle className="text-base">清理 Agent 残留？</DialogTitle><DialogDescription className="text-xs leading-5">会把整个目录移到废纸篓，不会永久删除，之后可以从 Finder 恢复。</DialogDescription></DialogHeader>
        <div className="px-6 py-2"><code className={styles.cleanupPath}>{cleanTarget.path}</code></div>
        <DialogFooter><Button size="sm" variant="outline" onClick={() => setCleanTarget(undefined)} disabled={Boolean(busy)}>取消</Button><Button size="sm" variant="destructive" disabled={Boolean(busy)} onClick={() => { const target = cleanTarget; setCleanTarget(undefined); void run(`clean:${target.path}`, () => onCleanup(target.path)) }}>{busy.startsWith('clean:') ? '清理中…' : '移到废纸篓'}</Button></DialogFooter>
      </DialogContent>
    </Dialog>}
  </Dialog>
}
