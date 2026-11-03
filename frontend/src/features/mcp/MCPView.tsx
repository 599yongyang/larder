import { lazy, Suspense, useEffect, useState } from 'react'
import { Plus, Trash2 } from 'lucide-react'
import { AgentIcon } from '../../components/AgentIcon'
import { MCPIcon } from '../../components/MCPIcon'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { MCPRemoveDialog } from '../../components/MCPRemoveDialog'
import { MCPCapabilitySheet } from './MCPCapabilitySheet'
const MCPAddSheet = lazy(() => import('../../components/MCPAddSheet').then((module) => ({ default: module.MCPAddSheet })))
import { AssetSearch } from '../../components/AssetSearch'
import { backend } from '../../backend'
import styles from '../../App.module.css'
import { t } from '../../i18n'
import { shortPath } from '../../lib/format'
import type { CLIInfo, MCPAgent, MCPCapabilitySummary, MCPServer } from '../../types'

type Filter = 'all' | 'issues' | 'enabled' | 'disabled'
const statusLabel = (status: string) => status === 'missing' ? t('mcp.commandMissing') : status === 'disabled' ? t('common.disabled') : status === 'invalid' ? t('mcp.incompleteConfig') : t('common.healthy')
const statusTone = (status: string): 'success' | 'warning' | 'error' | 'outline' => status === 'missing' || status === 'invalid' ? 'error' : status === 'disabled' ? 'outline' : 'success'

function AgentIcons({ agents }: { agents: MCPAgent[] }) {
  const shown = agents.slice(0, 4)
  return <div className={styles.mcpAgentIcons} title={agents.map((agent) => agent.label).join('、')}>{shown.map((agent) => <span key={`${agent.label}:${agent.path}`} className={styles.mcpAgentIcon}><AgentIcon name={agent.label} className="size-3.5" /></span>)}{agents.length > shown.length && <span className={styles.mcpAgentMore}>+{agents.length - shown.length}</span>}</div>
}

export function MCPView({ servers, clis, onRefresh, focusMCP, onFocusConsumed, onOpenCLI }: { servers: MCPServer[]; clis: CLIInfo[]; onRefresh: () => Promise<void>; focusMCP?: string; onFocusConsumed?: () => void; onOpenCLI?: (id: string) => void }) {
  const [query, setQuery] = useState('')
  const [filter, setFilter] = useState<Filter>('all')
  const [agentFilter, setAgentFilter] = useState('all')
  const [inspectServer, setInspectServer] = useState<MCPServer>()
  const [autoConnect, setAutoConnect] = useState(false)
  const [removeTarget, setRemoveTarget] = useState<MCPServer>()
  const [addOpen, setAddOpen] = useState(false)
  const [capabilities, setCapabilities] = useState<Record<string, MCPCapabilitySummary>>({})
  const loadCapabilities = () => { if (typeof backend.mcpCapabilityCache !== 'function') return; backend.mcpCapabilityCache().then((items) => setCapabilities(Object.fromEntries(items.map((item) => [item.serverId, item])))).catch(() => {}) }
  useEffect(loadCapabilities, [])
  useEffect(() => { if (focusMCP) { const target = servers.find((server) => server.id === focusMCP); if (target) setInspectServer(target); onFocusConsumed?.() } }, [focusMCP, servers, onFocusConsumed])
  useEffect(() => {
    if (!inspectServer) return
    const next = servers.find((server) => server.id === inspectServer.id)
    if (!next) setInspectServer(undefined)
    else if (next !== inspectServer) setInspectServer(next)
  }, [servers, inspectServer])
  const agents = Array.from(new Set(servers.flatMap((server) => server.agents.map((agent) => agent.label)))).sort()
  const issues = servers.filter((server) => server.status === 'missing' || server.status === 'invalid').length
  const filtered = servers.filter((server) => {
    const inFilter = (filter === 'all' || (filter === 'issues' ? server.status === 'missing' || server.status === 'invalid' : filter === 'enabled' ? server.status !== 'disabled' : server.status === 'disabled')) && (agentFilter === 'all' || server.agents.some((agent) => agent.label === agentFilter))
    const text = `${server.name} ${server.command ?? ''} ${(server.args ?? []).join(' ')} ${server.url ?? ''} ${(server.envKeys ?? []).join(' ')} ${server.agents.map((agent) => `${agent.label} ${agent.path}`).join(' ')}`
    return inFilter && (!query.trim() || text.toLowerCase().includes(query.trim().toLowerCase()))
  })
  const open = (server: MCPServer, connect: boolean) => { setAutoConnect(connect); setInspectServer(server) }
  return <section className={styles.fullPanel}>
    <div className={styles.metrics}>
      <div className={styles.metric}><span>{t('mcp.serversLabel')}</span><strong>{servers.length}</strong><small>{t('mcp.deduplicated')}</small></div>
      <div className={styles.metric}><span>{t('common.agent')}</span><strong>{agents.length}</strong><small>{t('mcp.agentsUsingMcp')}</small></div>
      <div className={styles.metric}><span>{t('common.enabled')}</span><strong>{servers.filter((server) => server.status !== 'disabled').length}</strong><small>{t('mcp.enabledInAtLeastOneAgent')}</small></div>
      <div className={styles.metric}><span>{t('common.needsAttention')}</span><strong data-tone={issues ? 'warn' : undefined}>{issues}</strong><small>{t('mcp.missingCommandOrIncompleteConfig')}</small></div>
    </div>
    <div className={styles.filterBar}>
      <div className={styles.filterLeft}>
        <AssetSearch value={query} onChange={setQuery} placeholder={t('mcp.searchServersCommandsOrAgents')} label={t('common.searchMcpServers')} />
        <Select value={agentFilter} onValueChange={setAgentFilter}><SelectTrigger size="sm" className="w-[120px]"><SelectValue /></SelectTrigger><SelectContent><SelectItem value="all">{t('common.allAgents')}</SelectItem>{agents.map((agent) => <SelectItem key={agent} value={agent}>{agent}</SelectItem>)}</SelectContent></Select>
        <Select value={filter} onValueChange={(value) => setFilter(value as Filter)}><SelectTrigger size="sm" className="w-[110px]"><SelectValue /></SelectTrigger><SelectContent><SelectItem value="all">{t('common.allStates')}</SelectItem><SelectItem value="issues">{t('common.needsAttention')}</SelectItem><SelectItem value="enabled">{t('common.enabled')}</SelectItem><SelectItem value="disabled">{t('common.disabled')}</SelectItem></SelectContent></Select>
      </div>
      <div className={styles.filterRight}><Button size="sm" variant="outline" onClick={() => setAddOpen(true)}><Plus size={14} />{t('common.addMcp')}</Button></div>
    </div>
    <div className={styles.tableWrap}><table>
      <thead><tr><th>{t('common.server')}</th><th>{t('common.transport')}</th><th>{t('common.launch')}</th><th>{t('common.agent')}</th><th>{t('mcp.capabilities')}</th><th>{t('common.status')}</th><th>{t('common.actions')}</th></tr></thead>
      <tbody>{filtered.length === 0 ? <tr><td colSpan={7} className={styles.empty}>{servers.length === 0 ? t('mcp.noMcpConfigurationFoundYet') : t('mcp.noMcpServerMatchesTheCurrentFilters')}</td></tr> : filtered.map((server) => <tr key={server.id} className={styles.clickableRow} tabIndex={0} onKeyDown={(event) => { if (event.key === 'Enter' || event.key === ' ') { event.preventDefault(); open(server, true) } }} onClick={() => open(server, true)}>
        <td><div className={styles.cliName}><MCPIcon name={server.name} command={server.command} url={server.url} className="size-4" /><strong>{server.name}</strong></div></td>
        <td><Badge variant="outline">{server.transport}</Badge></td>
        <td><code>{server.command ? `${shortPath(server.command)}${server.args?.length ? ' ' + server.args.slice(0, 2).join(' ') : ''}` : server.url || '—'}</code></td>
        <td><AgentIcons agents={server.agents} /></td>
        <td>{capabilities[server.id] ? (() => { const cap = capabilities[server.id]; const checked = new Date(cap.checkedAt); const stale = Date.now() - checked.getTime() > 24 * 60 * 60 * 1000; return <div className={`${styles.mcpCapabilityCell} ${stale ? styles.mcpCapabilityStale : ''}`} title={`${t('mcp.lastChecked')}: ${checked.toLocaleString()}`}><span>{t('mcp.tools')} <b>{cap.tools}</b></span><span>{t('mcp.resources')} <b>{cap.resources}</b></span><span>{t('mcp.prompts')} <b>{cap.prompts}</b></span>{stale && <em>{t('mcp.stale')}</em>}</div> })() : <span className={styles.mcpCapabilityUnknown}>{t('mcp.notInspected')}</span>}</td>
        <td><Badge variant={statusTone(server.status)}>{statusLabel(server.status)}</Badge></td>
        <td><div className={styles.quickActions}>{server.agents.some((agent) => agent.enabled) && <Button variant="ghost" size="xs" className="text-destructive" title={t('mcp.removeMcpConfig')} aria-label={t('mcp.removeMcpConfig')} onClick={(event) => { event.stopPropagation(); setRemoveTarget(server) }}><Trash2 size={14} /></Button>}</div></td>
      </tr>)}</tbody>
    </table></div>
    {inspectServer && <MCPCapabilitySheet server={inspectServer} autoConnect={autoConnect} onRefresh={onRefresh} onClose={() => { setInspectServer(undefined); loadCapabilities() }} onRemove={() => { setRemoveTarget(inspectServer); setInspectServer(undefined) }} />}
    {addOpen && <Suspense fallback={null}><MCPAddSheet open={addOpen} existingNames={servers.map((server) => server.name)} onClose={() => setAddOpen(false)} onAdded={() => void onRefresh()} /></Suspense>}
    {removeTarget && <MCPRemoveDialog server={removeTarget} onClose={() => setRemoveTarget(undefined)} onApplied={() => { setRemoveTarget(undefined); void onRefresh() }} />}
  </section>
}
