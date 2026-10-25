import { lazy, Suspense, useEffect, useState } from 'react'
import { Copy, Plus, Trash2 } from 'lucide-react'
import { AgentIcon } from '../../components/AgentIcon'
import { MCPIcon } from '../../components/MCPIcon'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { Sheet, SheetContent, SheetDescription, SheetHeader, SheetTitle } from '@/components/ui/sheet'
import { MCPRemoveDialog } from '../../components/MCPRemoveDialog'
const MCPAddSheet = lazy(() => import('../../components/MCPAddSheet').then((module) => ({ default: module.MCPAddSheet })))
import { MCPEditDialog } from '../../components/MCPEditDialog'
import { AssetSearch } from '../../components/AssetSearch'
import { backend } from '../../backend'
import styles from '../../App.module.css'
import { t } from '../../i18n'
import { backendErrorMessage } from '../../lib/backendError'
import { shortPath } from '../../lib/format'
import { toastError, toastSuccess } from '../../lib/toast'
import type { CLIInfo, MCPAgent, MCPServer } from '../../types'
import { commandBasename } from '../../lib/relations'

type Filter = 'all' | 'issues' | 'enabled' | 'disabled'

const statusLabel = (status: string) => status === 'missing' ? t('mcp.commandMissing') : status === 'disabled' ? t('common.disabled') : status === 'invalid' ? t('mcp.incompleteConfig') : t('common.healthy')
const statusTone = (status: string): 'success' | 'warning' | 'error' | 'outline' => status === 'missing' || status === 'invalid' ? 'error' : status === 'disabled' ? 'outline' : 'success'

function AgentIcons({ agents }: { agents: MCPAgent[] }) {
  const shown = agents.slice(0, 4)
  return <div className={styles.mcpAgentIcons} title={agents.map((agent) => agent.label).join('、')} aria-label={agents.map((agent) => agent.label).join('、')}>{shown.map((agent) => <span key={`${agent.label}:${agent.path}`} className={styles.mcpAgentIcon}><AgentIcon name={agent.label} className="size-3.5" /></span>)}{agents.length > shown.length && <span className={styles.mcpAgentMore}>+{agents.length - shown.length}</span>}</div>
}

function MCPView({ servers, clis, onRefresh, focusMCP, onFocusConsumed, onOpenCLI }: { servers: MCPServer[]; clis: CLIInfo[]; onRefresh: () => Promise<void>; focusMCP?: string; onFocusConsumed?: () => void; onOpenCLI?: (id: string) => void }) {
  const [query, setQuery] = useState('')
  const [filter, setFilter] = useState<Filter>('all')
  const [agentFilter, setAgentFilter] = useState('all')
  const [selectedId, setSelectedId] = useState<string>()
  const [removeTarget, setRemoveTarget] = useState<MCPServer>()
  const [addOpen, setAddOpen] = useState(false)
  const [editOpen, setEditOpen] = useState(false)
  const [agentBusy, setAgentBusy] = useState('')
  const [testBusy, setTestBusy] = useState(false)
  const [agentError, setAgentError] = useState('')
  const [testResult, setTestResult] = useState<{ ok: boolean; message: string }>()
  useEffect(() => { setTestResult(undefined); setAgentError('') }, [selectedId])
  useEffect(() => { if (focusMCP) { setSelectedId(focusMCP); onFocusConsumed?.() } }, [focusMCP, onFocusConsumed])
  const agents = Array.from(new Set(servers.flatMap((server) => server.agents.map((agent) => agent.label)))).sort()
  const agentCount = agents.length
  const issues = servers.filter((server) => server.status === 'missing' || server.status === 'invalid').length
  const toggleAgent = async (agent: MCPAgent) => {
    if (!selected || agentBusy) return
    setAgentError('')
    setAgentBusy(agent.path)
    try {
      if (agent.enabled) await backend.disableMCPAgent(selected.id, agent.path)
      else await backend.enableMCPAgent(selected.name, agent.path)
      await onRefresh()
      if (!agent.enabled && selected.status === 'disabled') setSelectedId(undefined)
    } catch (error) {
      setAgentError(backendErrorMessage(error, agent.enabled ? t('mcp.failedToDisableMcpAgent') : t('mcp.failedToEnableMcpAgent')))
    } finally { setAgentBusy('') }
  }
  const testConnection = async () => {
    if (!selected || testBusy) return
    setTestBusy(true)
    try { setTestResult(await backend.testMCP(selected.id)) } catch (error) { toastError(error, t('mcp.connectionTestFailed')) } finally { setTestBusy(false) }
  }
  const filtered = servers.filter((server) => {
    const inFilter = (filter === 'all' || (filter === 'issues' ? server.status === 'missing' || server.status === 'invalid' : filter === 'enabled' ? server.status !== 'disabled' : server.status === 'disabled')) && (agentFilter === 'all' || server.agents.some((agent) => agent.label === agentFilter))
    const text = `${server.name} ${server.command ?? ''} ${(server.args ?? []).join(' ')} ${server.url ?? ''} ${(server.envKeys ?? []).join(' ')} ${server.agents.map((agent) => `${agent.label} ${agent.path}`).join(' ')}`
    return inFilter && (!query.trim() || text.toLowerCase().includes(query.trim().toLowerCase()))
  })
  const copyText = async (value: string, label: string) => { try { await navigator.clipboard.writeText(value); toastSuccess(t('common.copied', { label })) } catch (error) { toastError(error, t('common.failedToCopy', { label })) } }
  const selected = servers.find((server) => server.id === selectedId)
  return <section className={styles.fullPanel}>
    <div className={styles.metrics}>
      <div className={styles.metric}><span>MCP Server</span><strong>{servers.length}</strong><small>{t('mcp.deduplicated')}</small></div>
      <div className={styles.metric}><span>Agent</span><strong>{agentCount}</strong><small>{t('mcp.agentsUsingMcp')}</small></div>
      <div className={styles.metric}><span>{t('common.enabled')}</span><strong>{servers.filter((server) => server.status !== 'disabled').length}</strong><small>{t('mcp.enabledInAtLeastOneAgent')}</small></div>
      <div className={styles.metric}><span>{t('common.needsAttention')}</span><strong data-tone={issues ? 'warn' : undefined}>{issues}</strong><small>{t('mcp.missingCommandOrIncompleteConfig')}</small></div>
    </div>
    <div className={styles.filterBar}>
      <div className={styles.filterLeft}>
        <AssetSearch value={query} onChange={setQuery} placeholder={t('mcp.searchServersCommandsOrAgents')} label={t('common.searchMcpServers')} />
        <Select value={agentFilter} onValueChange={setAgentFilter}>
          <SelectTrigger size="sm" className="w-[120px]"><SelectValue /></SelectTrigger>
          <SelectContent><SelectItem value="all">{t('common.allAgents')}</SelectItem>{agents.map((agent) => <SelectItem key={agent} value={agent}>{agent}</SelectItem>)}</SelectContent>
        </Select>
        <Select value={filter} onValueChange={(value) => setFilter(value as Filter)}>
          <SelectTrigger size="sm" className="w-[110px]"><SelectValue /></SelectTrigger>
          <SelectContent><SelectItem value="all">{t('common.allStates')}</SelectItem><SelectItem value="issues">{t('common.needsAttention')}</SelectItem><SelectItem value="enabled">{t('common.enabled')}</SelectItem><SelectItem value="disabled">{t('common.disabled')}</SelectItem></SelectContent>
        </Select>
      </div>
      <div className={styles.filterRight}>
        <Button size="sm" variant="outline" onClick={() => setAddOpen(true)}><Plus size={14} />{t('common.addMcp')}</Button>
      </div>
    </div>
    <div className={styles.tableWrap}><table>
      <thead><tr><th>Server</th><th>{t('common.transport')}</th><th>{t('common.launch')}</th><th>Agent</th><th>{t('common.status')}</th><th>{t('common.actions')}</th></tr></thead>
      <tbody>{filtered.length === 0 ? <tr><td colSpan={6} className={styles.empty}>{servers.length === 0 ? <div className={styles.emptyAction}><span>{t('mcp.noMcpConfigurationFoundYet')}</span><Button size="sm" onClick={() => setAddOpen(true)}><Plus size={14} />{t('common.addMcp')}</Button></div> : t('mcp.noMcpServerMatchesTheCurrentFilters')}</td></tr> : filtered.map((server) => <tr key={server.id} className={`${selected?.id === server.id ? styles.selectedRow : ''} ${styles.clickableRow}`} onClick={() => setSelectedId(server.id)}>
        <td><div className={styles.cliName}><MCPIcon name={server.name} command={server.command} url={server.url} className="size-4" /><strong>{server.name}</strong></div></td>
        <td><Badge variant="outline">{server.transport}</Badge></td>
        <td><code>{server.command ? `${shortPath(server.command)}${server.args?.length ? ' ' + server.args.slice(0, 2).join(' ') : ''}` : server.url || '—'}</code></td>
        <td><AgentIcons agents={server.agents} /></td>
        <td><Badge variant={statusTone(server.status)}>{statusLabel(server.status)}</Badge></td>
        <td><div className={styles.quickActions}>{server.agents.some((agent) => agent.enabled) && <Button variant="ghost" size="xs" className="text-destructive" title={t('mcp.removeMcpConfig')} aria-label={t('mcp.removeMcpConfig')} onClick={(event) => { event.stopPropagation(); setRemoveTarget(server) }}><Trash2 size={14} /></Button>}</div></td>
      </tr>)}</tbody>
    </table></div>
    {selected && <Sheet open onOpenChange={(open) => { if (!open) setSelectedId(undefined) }}>
      <SheetContent side="right" className="w-[440px] sm:max-w-[440px]">
        <SheetHeader className="gap-0 border-b px-5 py-4"><div className="flex items-start gap-3 pr-9"><span className={styles.mcpHero}><MCPIcon name={selected.name} command={selected.command} url={selected.url} className="size-5" /></span><div className="min-w-0 flex-1"><SheetTitle className="truncate text-[15px]">{selected.name}</SheetTitle><SheetDescription className="mt-1 text-[11px]">{selected.transport} · {t('common.agents', { n: selected.agents.length })}</SheetDescription></div><Badge variant={statusTone(selected.status)}>{statusLabel(selected.status)}</Badge></div></SheetHeader>
        <div className="min-h-0 flex-1 overflow-y-auto p-0">
          <section className={styles.detailSection}><h3>{t('common.launch')}</h3><dl className={styles.definition}>{selected.command && <><dt>{t('common.commandLabel')}</dt><dd><code>{selected.command}</code></dd></>}{selected.args && selected.args.length > 0 && <><dt>{t('common.arguments')}</dt><dd><code className={styles.mcpArgs}>{selected.args.join(' ')}</code></dd></>}{selected.url && <><dt>URL</dt><dd><code className={styles.mcpArgs}>{selected.url}</code></dd></>}<dt>{t('common.transport')}</dt><dd>{selected.transport}</dd></dl><div className={styles.detailPathActions}><Button size="xs" variant="outline" onClick={() => void copyText(selected.command ? [selected.command, ...(selected.args ?? [])].join(' ') : selected.url || '', t('mcp.launchCommand'))} disabled={!selected.command && !selected.url}><Copy size={12} />{t('mcp.copyLaunchCommand')}</Button>{selected.agents.some((agent) => agent.enabled) && <Button size="xs" variant="outline" onClick={() => setEditOpen(true)}>{t('common.editAction')}</Button>}</div>{selected.agents.some((agent) => agent.enabled) && <div className={styles.mcpTestRow}><Button size="xs" variant="outline" disabled={testBusy} onClick={() => void testConnection()}>{testBusy ? t('mcp.testing') : selected.url ? t('mcp.testConnection') : t('mcp.checkCommand')}</Button>{testResult && <Badge variant={testResult.ok ? 'success' : 'warning'}>{testResult.message}</Badge>}</div>}{selected.command && (() => { const name = commandBasename(selected.command); const cli = clis.find((item) => item.name === name); return <div className={styles.mcpDependency}><span>{t('common.cliDependencies')}</span><code>{name}</code>{cli ? <><Badge variant="success">{cli.version || t('common.installed')}</Badge><Button size="xs" variant="ghost" onClick={() => onOpenCLI?.(cli.id)}>{t('common.viewCli')}</Button></> : <Badge variant="outline">{t('common.notDetected')}</Badge>}</div> })()}</section>
          {(selected.envKeys?.length || selected.headerKeys?.length) ? <section className={styles.detailSection}><h3>{t('common.sensitiveFields')}</h3><div className={styles.mcpKeys}>{(selected.envKeys ?? []).map((key) => <Badge key={key} variant="outline">{key}</Badge>)}{(selected.headerKeys ?? []).map((key) => <Badge key={key} variant="outline">{key}</Badge>)}</div><p>{t('mcp.onlyFieldNamesAreShownSecretValuesAreNeverReadOrDisplayed')}</p></section> : null}
          <section className={styles.detailSection}><h3>{t('common.agentConfiguration')}</h3><div className={styles.mcpAgents}>{selected.agents.map((agent: MCPAgent) => <div key={`${agent.label}:${agent.path}`} className={styles.mcpAgentRow}><span className={styles.mcpAgentAvatar}><AgentIcon name={agent.label} className="size-4" /></span><div><strong>{agent.label}</strong><small>{agent.scope || shortPath(agent.path)}</small></div><span className={styles.mcpAgentActions}><Badge variant={agent.enabled ? 'success' : 'outline'}>{agent.enabled ? t('common.enabled') : t('common.disabled')}</Badge><Button size="xs" variant={agent.enabled ? 'ghost' : 'outline'} disabled={Boolean(agentBusy)} onClick={() => void toggleAgent(agent)}>{agentBusy === agent.path ? t('common.working') : agent.enabled ? t('common.disableAction') : t('mcp.enable')}</Button></span></div>)}</div>{agentError && <p className={styles.planError} role="alert">{agentError}</p>}</section>
        </div>
      </SheetContent>
    </Sheet>}
    {selected && editOpen && <MCPEditDialog server={selected} onClose={() => setEditOpen(false)} onSaved={() => void onRefresh()} />}
    {addOpen && <Suspense fallback={null}><MCPAddSheet open={addOpen} existingNames={servers.map((server) => server.name)} onClose={() => setAddOpen(false)} onAdded={() => void onRefresh()} /></Suspense>}
    {removeTarget && <MCPRemoveDialog server={removeTarget} onClose={() => setRemoveTarget(undefined)} onApplied={() => { setRemoveTarget(undefined); setSelectedId(undefined); void onRefresh() }} />}
  </section>
}

export { MCPView }
