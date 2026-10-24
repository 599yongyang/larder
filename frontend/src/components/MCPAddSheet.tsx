import { useEffect, useRef, useState } from 'react'
import { Plus, Search } from 'lucide-react'
import { MCPIcon } from './MCPIcon'
import { AgentIcon } from './AgentIcon'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Checkbox } from '@/components/ui/checkbox'
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { Input } from '@/components/ui/input'
import { Sheet, SheetContent, SheetDescription, SheetHeader, SheetTitle } from '@/components/ui/sheet'
import styles from '../App.module.css'
import { backend } from '../backend'
import { toastSuccess } from '../lib/toast'
import type { MCPAgent } from '../types'

type RegistryEnv = { name: string; description?: string; required?: boolean; secret?: boolean }
type FeaturedMCP = { name: string; title: string; publisher: string; description: string; transport: 'stdio' | 'http'; command?: string; args?: string[]; url?: string; env?: RegistryEnv[]; source?: 'featured' | 'registry'; unsupported?: string }

const FEATURED: FeaturedMCP[] = [
  { name: 'context7', title: 'Context7', publisher: 'Upstash', description: '拉取最新库文档和代码示例。', transport: 'http', url: 'https://mcp.context7.com/mcp' },
  { name: 'playwright', title: 'Playwright', publisher: 'Microsoft', description: '驱动真实浏览器：打开页面、点击、输入和读取页面。', transport: 'stdio', command: 'npx', args: ['-y', '@playwright/mcp@latest'] },
  { name: 'chrome-devtools', title: 'Chrome DevTools', publisher: 'Google', description: '检查实时 Chrome：控制台、网络和性能。', transport: 'stdio', command: 'npx', args: ['-y', 'chrome-devtools-mcp@latest'] },
  { name: 'deepwiki', title: 'DeepWiki', publisher: 'Cognition', description: '提问并阅读任意公开仓库的生成 wiki。', transport: 'http', url: 'https://mcp.deepwiki.com/mcp' },
  { name: 'memory', title: 'Memory', publisher: 'Model Context Protocol', description: '跨对话保存知识的本地知识图谱。', transport: 'stdio', command: 'npx', args: ['-y', '@modelcontextprotocol/server-memory'] },
  { name: 'sequential-thinking', title: 'Sequential Thinking', publisher: 'Model Context Protocol', description: '逐步推理、回退和分支。', transport: 'stdio', command: 'npx', args: ['-y', '@modelcontextprotocol/server-sequential-thinking'] },
  { name: 'fetch', title: 'Fetch', publisher: 'Model Context Protocol', description: '抓取网页并转换成 Markdown。', transport: 'stdio', command: 'uvx', args: ['mcp-server-fetch'] },
  { name: 'git', title: 'Git', publisher: 'Model Context Protocol', description: '读取、搜索和修改 Git 仓库。', transport: 'stdio', command: 'uvx', args: ['mcp-server-git'] },
  { name: 'time', title: 'Time', publisher: 'Model Context Protocol', description: '获取时区时间和时间转换。', transport: 'stdio', command: 'uvx', args: ['mcp-server-time'] },
  { name: 'exa', title: 'Exa', publisher: 'Exa', description: '面向 Agent 的网页和代码搜索。', transport: 'http', url: 'https://mcp.exa.ai/mcp' },
  { name: 'figma', title: 'Figma', publisher: 'Figma', description: '读取设计稿、组件和样式上下文。', transport: 'http', url: 'https://mcp.figma.com/mcp' },
  { name: 'notion', title: 'Notion', publisher: 'Notion', description: '搜索、读取和写入 Notion 页面。', transport: 'http', url: 'https://mcp.notion.com/mcp' },
  { name: 'linear', title: 'Linear', publisher: 'Linear', description: '查找、创建和更新 Linear issue。', transport: 'http', url: 'https://mcp.linear.app/mcp' },
  { name: 'sentry', title: 'Sentry', publisher: 'Sentry', description: '读取 issue、错误和 trace，并做分析。', transport: 'http', url: 'https://mcp.sentry.dev/mcp' },
  { name: 'atlassian', title: 'Atlassian', publisher: 'Atlassian', description: '读取 Jira issue 和 Confluence 页面。', transport: 'http', url: 'https://mcp.atlassian.com/v1/mcp' },
  { name: 'supabase', title: 'Supabase', publisher: 'Supabase', description: '管理 Supabase 项目、SQL、迁移和日志。', transport: 'http', url: 'https://mcp.supabase.com/mcp' },
  { name: 'stripe', title: 'Stripe', publisher: 'Stripe', description: '查询客户、支付、订阅和 Stripe 文档。', transport: 'http', url: 'https://mcp.stripe.com' },
  { name: 'vercel', title: 'Vercel', publisher: 'Vercel', description: '读取 Vercel 项目、部署和日志。', transport: 'http', url: 'https://mcp.vercel.com' },
  { name: 'neon', title: 'Neon', publisher: 'Neon', description: '管理 Neon Postgres 项目、分支和 SQL。', transport: 'http', url: 'https://mcp.neon.tech/mcp' },
  { name: 'cloudflare-docs', title: 'Cloudflare Docs', publisher: 'Cloudflare', description: '搜索 Cloudflare 文档。', transport: 'http', url: 'https://docs.mcp.cloudflare.com/mcp' },
  { name: 'microsoft-learn', title: 'Microsoft Learn', publisher: 'Microsoft', description: '官方 Microsoft 和 Azure 文档与代码示例。', transport: 'http', url: 'https://learn.microsoft.com/api/mcp' },
  { name: 'huggingface', title: 'Hugging Face', publisher: 'Hugging Face', description: '搜索模型、数据集、Spaces 和论文。', transport: 'http', url: 'https://huggingface.co/mcp' },
]

export function MCPAddSheet({ open, existingNames, onClose, onAdded }: { open: boolean; existingNames: string[]; onClose: () => void; onAdded: () => void }) {
  const [query, setQuery] = useState('')
  const [selected, setSelected] = useState<FeaturedMCP>()
  const [envValues, setEnvValues] = useState<Record<string, string>>({})
  const [registry, setRegistry] = useState<FeaturedMCP[]>([])
  const [registryLoading, setRegistryLoading] = useState(false)
  const [registryError, setRegistryError] = useState('')
  const registryRequest = useRef(0)
  const [targets, setTargets] = useState<MCPAgent[]>([])
  const [paths, setPaths] = useState<string[]>([])
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')
  const [customOpen, setCustomOpen] = useState(false)
  const [customName, setCustomName] = useState('')
  const [customTransport, setCustomTransport] = useState<'stdio' | 'http'>('stdio')
  const [customCommand, setCustomCommand] = useState('')
  const [customArgs, setCustomArgs] = useState('')
  const [customURL, setCustomURL] = useState('')
  const [customEnv, setCustomEnv] = useState('')
  const [customPaths, setCustomPaths] = useState<string[]>([])
  const [customBusy, setCustomBusy] = useState(false)
  const [customError, setCustomError] = useState('')
  useEffect(() => { if (!open) return; setQuery(''); setSelected(undefined); setPaths([]); setError(''); setRegistry([]); setRegistryError(''); backend.mcpTargets().then((value) => setTargets((value as MCPAgent[]) ?? [])).catch(() => setTargets([])) }, [open])
  useEffect(() => {
    const term = query.trim()
    if (term.length < 2) { setRegistry([]); setRegistryLoading(false); setRegistryError(''); return }
    const request = ++registryRequest.current
    setRegistryLoading(true); setRegistryError('')
    const timer = window.setTimeout(() => {
      void backend.searchMCPRegistry(term).then((value) => {
        if (request !== registryRequest.current) return
        const mapped = (value as Array<{ id: string; name: string; title: string; description: string; transport: 'stdio' | 'http'; command?: string; args?: string[]; url?: string; env?: RegistryEnv[]; unsupported?: string }>).map((item) => ({ ...item, publisher: 'MCP Registry', source: 'registry' as const }))
        setRegistry(mapped)
      }).catch((cause) => { if (request === registryRequest.current) setRegistryError(cause instanceof Error ? cause.message : 'Registry 搜索失败') }).finally(() => { if (request === registryRequest.current) setRegistryLoading(false) })
    }, 350)
    return () => window.clearTimeout(timer)
  }, [query])
  const requiredEnvMissing = Boolean(selected?.env?.some((env) => env.required && !envValues[env.name]?.trim()))
  const openCustom = () => { setCustomName(''); setCustomTransport('stdio'); setCustomCommand(''); setCustomArgs(''); setCustomURL(''); setCustomEnv(''); setCustomPaths([]); setCustomError(''); setCustomOpen(true) }
  const installCustom = async () => {
    const env: Record<string, string> = {}
    for (const line of customEnv.split('\n')) { const trimmed = line.trim(); if (!trimmed) continue; const [key, ...rest] = trimmed.split('='); env[key.trim()] = rest.join('=').trim() }
    setCustomBusy(true); setCustomError('')
    try {
      const plan = await backend.planMCPAdd(customName.trim(), customTransport, customCommand.trim(), customArgs.trim() ? customArgs.trim().split(/\s+/) : [], customURL.trim(), env, customPaths)
      await backend.applyPlan(plan.id)
      toastSuccess('已添加自定义 MCP', customName.trim())
      setCustomOpen(false); onAdded(); onClose()
    } catch (cause) {
      setCustomError(cause instanceof Error ? cause.message : '添加 MCP 失败')
    } finally { setCustomBusy(false) }
  }
  const filtered = FEATURED.filter((server) => !query.trim() || `${server.title} ${server.name} ${server.publisher} ${server.description}`.toLowerCase().includes(query.trim().toLowerCase()))
  const install = async () => {
    if (!selected || paths.length === 0) return
    setBusy(true); setError('')
    try {
      const plan = await backend.planMCPAdd(selected.name, selected.transport, selected.command ?? '', selected.args ?? [], selected.url ?? '', envValues, paths)
      await backend.applyPlan(plan.id)
      toastSuccess('已添加 MCP Server', selected.title)
      onAdded(); onClose()
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : '添加 MCP 失败')
    } finally { setBusy(false) }
  }
  return <Sheet open={open} onOpenChange={(next) => { if (!next && !busy) onClose() }}>
    <SheetContent side="right" className={`${styles.mcpAddSheet} gap-0 p-0`} showCloseButton>
      <SheetHeader className={styles.discoverHeader}><SheetTitle className={styles.discoverTitle}>添加 MCP Server</SheetTitle><SheetDescription className={styles.discoverDescription}>精选的本地 MCP Server，只写配置文件，不安装命令、不启动服务。</SheetDescription></SheetHeader>
      <div className={styles.discoverToolbar}><label className={styles.discoverSearchField}><Search size={13} aria-hidden="true" /><Input className={styles.discoverSearchInput} value={query} onChange={(event) => setQuery(event.target.value)} placeholder="搜索 MCP Server…" aria-label="搜索 MCP Server" /></label><Button size="xs" variant="outline" onClick={openCustom}><Plus size={12} />自定义</Button></div>
      <div className={styles.mcpAddGrid}>{filtered.map((server) => {
        const exists = existingNames.includes(server.name)
        return <section key={server.name} className={styles.mcpAddCard}><div className={styles.mcpAddCardHead}><span className={styles.mcpAddIcon}><MCPIcon name={server.name} command={server.command} url={server.url} className="size-5" /></span><div><strong>{server.title}</strong><small>{server.publisher}</small></div></div><p title={server.description}>{server.description}</p><div className={styles.mcpAddCardFoot}><Badge variant="outline">{server.transport}</Badge>{exists ? <span className={styles.mcpAddExists}>已存在</span> : <Button size="xs" variant="outline" onClick={() => { setSelected(server); setEnvValues({}); setPaths([]); setError('') }}><Plus size={12} />添加</Button>}</div></section>
      })}
        {query.trim().length >= 2 && <div className={styles.mcpRegistryHeading}><span>官方 Registry</span>{registryLoading && <small>搜索中…</small>}{registryError && <small className={styles.discoverError}>{registryError}</small>}</div>}
        {query.trim().length >= 2 && registryLoading && registry.length === 0 && <div className={styles.mcpRegistryState}>正在查询 registry.modelcontextprotocol.io…</div>}
        {query.trim().length >= 2 && !registryLoading && !registryError && registry.length === 0 && <div className={styles.mcpRegistryState}>Registry 没有可安装的结果。</div>}
        {registry.map((server) => { const exists = existingNames.includes(server.name); return <section key={server.name} className={styles.mcpAddCard}><div className={styles.mcpAddCardHead}><span className={styles.mcpAddIcon}><MCPIcon name={server.name} command={server.command} url={server.url} className="size-5" /></span><div><strong>{server.title}</strong><small>MCP Registry</small></div></div><p title={server.description}>{server.description}</p><div className={styles.mcpAddCardFoot}><Badge variant={server.unsupported ? 'warning' : 'outline'}>{server.transport || 'unknown'}</Badge>{server.unsupported ? <span className={styles.mcpAddUnsupported} title={server.unsupported}>{server.unsupported}</span> : exists ? <span className={styles.mcpAddExists}>已存在</span> : <Button size="xs" variant="outline" onClick={() => { setSelected(server); setEnvValues({}); setPaths([]); setError('') }}><Plus size={12} />添加</Button>}</div></section> })}
      </div>
      {filtered.length === 0 && query.trim().length < 2 && <div className={styles.discoverState}>没有找到精选 MCP Server。</div>}
    </SheetContent>
    {customOpen && <Dialog open onOpenChange={(next) => { if (!next && !customBusy) setCustomOpen(false) }}>
      <DialogContent className="max-w-[560px] gap-0 p-0">
        <DialogHeader className="border-b px-6 py-4 pr-12"><DialogTitle className="text-base">自定义 MCP Server</DialogTitle><DialogDescription className="text-xs">写入 Agent 配置；不会安装命令或启动服务。</DialogDescription></DialogHeader>
        <div className={styles.mcpCustomBody}>
          <div className={styles.mcpCustomRow}><span>名称</span><Input value={customName} onChange={(event) => setCustomName(event.target.value)} placeholder="my-server" /></div>
          <div className={styles.mcpCustomRow}><span>类型</span><div className={styles.mcpTransport}><button type="button" className={customTransport === 'stdio' ? styles.mcpTransportActive : styles.mcpTransportButton} onClick={() => setCustomTransport('stdio')}>stdio</button><button type="button" className={customTransport === 'http' ? styles.mcpTransportActive : styles.mcpTransportButton} onClick={() => setCustomTransport('http')}>http</button></div></div>
          {customTransport === 'stdio' ? <><div className={styles.mcpCustomRow}><span>命令</span><Input value={customCommand} onChange={(event) => setCustomCommand(event.target.value)} placeholder="npx" /></div><div className={styles.mcpCustomRow}><span>参数</span><Input value={customArgs} onChange={(event) => setCustomArgs(event.target.value)} placeholder="-y @scope/server" /></div><div className={styles.mcpCustomColumn}><span>环境变量</span><textarea value={customEnv} onChange={(event) => setCustomEnv(event.target.value)} placeholder={'API_KEY=...\nBASE_URL=...'} aria-label="环境变量" /></div></> : <div className={styles.mcpCustomRow}><span>URL</span><Input value={customURL} onChange={(event) => setCustomURL(event.target.value)} placeholder="https://example.com/mcp" /></div>}
          <div className={styles.mcpCustomColumn}><span>写入 Agent 配置</span><div className={styles.mcpAddTargets}>{targets.map((target) => <label key={target.path} className={styles.discoverAgent}><Checkbox checked={customPaths.includes(target.path)} onCheckedChange={(checked) => setCustomPaths((current) => checked ? [...new Set([...current, target.path])] : current.filter((path) => path !== target.path))} /><AgentIcon name={target.label} className="size-4" /><span>{target.label}</span></label>)}</div></div>
          {customError && <p className={styles.planError} role="alert">{customError}</p>}
        </div>
        <DialogFooter className="border-t px-6 py-2.5"><Button size="sm" variant="outline" disabled={customBusy} onClick={() => setCustomOpen(false)}>取消</Button><Button size="sm" disabled={customBusy || !customName.trim() || customPaths.length === 0 || (customTransport === 'stdio' ? !customCommand.trim() : !customURL.trim())} onClick={() => void installCustom()}>{customBusy ? '添加中…' : '添加'}</Button></DialogFooter>
      </DialogContent>
    </Dialog>}
    {selected && <Dialog open onOpenChange={(next) => { if (!next && !busy) setSelected(undefined) }}>
      <DialogContent className="max-w-[480px] gap-0 p-0">
        <DialogHeader className="border-b px-6 py-4 pr-12"><DialogTitle className="flex items-center gap-2.5 text-base"><MCPIcon name={selected.name} command={selected.command} url={selected.url} className="size-6" />添加 {selected.title}</DialogTitle><DialogDescription className="text-xs">{selected.publisher} · {selected.transport}</DialogDescription></DialogHeader>
        <div className={styles.mcpAddDialogBody}><p className={styles.mcpInstallDescription}>{selected.description}</p><div className={styles.mcpInstallMethod}><div className={styles.discoverInstallLabel}>启动方式</div><code>{selected.url || [selected.command, ...(selected.args ?? [])].filter(Boolean).join(' ') || '—'}</code></div>{selected.env && selected.env.length > 0 && <div className={styles.mcpRegistryEnv}><div className={styles.discoverInstallLabel}>环境变量</div>{selected.env.map((env) => <label key={env.name}><span>{env.name}{env.required ? ' *' : ''}</span><Input type={env.secret ? 'password' : 'text'} value={envValues[env.name] ?? ''} onChange={(event) => setEnvValues((current) => ({ ...current, [env.name]: event.target.value }))} placeholder={env.description || env.name} /></label>)}</div>}<div className={styles.discoverInstallLabel}>写入 Agent 配置</div><div className={styles.mcpAddTargets}>{targets.map((target) => <label key={target.path} className={styles.discoverAgent}><Checkbox checked={paths.includes(target.path)} onCheckedChange={(checked) => setPaths((current) => checked ? [...new Set([...current, target.path])] : current.filter((path) => path !== target.path))} /><AgentIcon name={target.label} className="size-4" /><span>{target.label}</span></label>)}</div><p className={styles.installDialogHint}>会备份配置文件；不会自动安装 npx/uvx 命令，也不会启动 MCP Server。</p>{error && <p className={styles.planError} role="alert">{error}</p>}</div>
        <DialogFooter className="border-t px-6 py-2.5"><Button size="sm" variant="outline" disabled={busy} onClick={() => setSelected(undefined)}>取消</Button><Button size="sm" disabled={busy || paths.length === 0 || requiredEnvMissing} onClick={() => void install()}>{busy ? '添加中…' : '添加'}</Button></DialogFooter>
      </DialogContent>
    </Dialog>}
  </Sheet>
}
