import { useEffect, useMemo, useState } from 'react'
import { Loader2, Play, RefreshCw } from 'lucide-react'
import { AgentIcon } from '../../components/AgentIcon'
import { MCPIcon } from '../../components/MCPIcon'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { backend } from '../../backend'
import { t } from '../../i18n'
import { backendErrorMessage } from '../../lib/backendError'
import { copyText } from '../../lib/clipboard'
import { toastSuccess } from '../../lib/toast'
import { shortPath } from '../../lib/format'
import styles from '../../App.module.css'
import type { MCPAgent, MCPAuthStatus, MCPInspectTarget, MCPInspection, MCPLogEntry, MCPPrompt, MCPResource, MCPResourceTemplate, MCPServer, MCPTool, MCPToolCallRecord } from '../../types'

type Tab = 'details' | 'tools' | 'resources' | 'prompts' | 'history' | 'logs'
const statusTone = (status: string): 'success' | 'warning' | 'error' | 'outline' => status === 'missing' || status === 'invalid' ? 'error' : status === 'disabled' ? 'outline' : 'success'
const statusLabel = (status: string) => status === 'missing' ? t('mcp.commandMissing') : status === 'disabled' ? t('common.disabled') : status === 'invalid' ? t('mcp.incompleteConfig') : t('common.healthy')

function formatResult(value?: string) {
  if (!value || value === 'null') return ''
  try { return JSON.stringify(JSON.parse(value), null, 2) } catch { return value }
}

function ToolResult({ value }: { value: string }) {
  if (!value) return null
  try {
    const parsed = JSON.parse(value)
    if (Array.isArray(parsed)) return <div className={styles.mcpResultBlocks}>{parsed.map((item, index) => {
      if (item?.type === 'text' && typeof item.text === 'string') return <pre key={index}>{item.text}</pre>
      if (item?.type === 'image' && typeof item.data === 'string') return <img key={index} className={styles.mcpResultImage} src={`data:${item.mimeType || 'image/png'};base64,${item.data}`} alt="MCP tool result" />
      if (item?.type === 'resource' && item.resource?.text) return <pre key={index}>{item.resource.text}</pre>
      return <pre key={index}>{JSON.stringify(item, null, 2)}</pre>
    })}</div>
    return <pre>{JSON.stringify(parsed, null, 2)}</pre>
  } catch { return <pre>{value}</pre> }
}

function ToolRunner({ server, tool, onCalled }: { server: MCPServer; tool: MCPTool; onCalled?: () => void }) {
  const fields = useMemo(() => {
    const schema = tool.inputSchema as { properties?: Record<string, { type?: string; description?: string; enum?: string[] }>; required?: string[] } | undefined
    if (!schema?.properties) return []
    return Object.entries(schema.properties).flatMap(([name, value]) => {
      if (!value || !['string', 'number', 'integer', 'boolean', 'object', 'array'].includes(value.type ?? '')) return []
      return [{ name, type: value.type ?? 'string', required: (schema.required ?? []).includes(name), description: value.description, options: value.enum }]
    })
  }, [tool])
  const [values, setValues] = useState<Record<string, string | boolean>>({})
  const [jsonMode, setJsonMode] = useState(fields.length === 0)
  const [args, setArgs] = useState('{}')
  const [result, setResult] = useState('')
  const [error, setError] = useState('')
  const [loading, setLoading] = useState(false)
  useEffect(() => { setValues({}); setResult(''); setError(''); setJsonMode(fields.length === 0) }, [tool.name])
  const run = async () => {
    let payload = args
    if (!jsonMode) {
      const object: Record<string, unknown> = {}
      for (const field of fields) {
        const value = values[field.name]
        if (value === undefined || value === '') { if (field.required) { setError(t('mcp.requiredArgument', { name: field.name })); return }; continue }
        if (field.type === 'object' || field.type === 'array') {
          try { object[field.name] = JSON.parse(String(value)) } catch { setError(t('mcp.invalidJsonArgument', { name: field.name })); return }
        } else {
          object[field.name] = field.type === 'boolean' ? value === true : field.type === 'number' || field.type === 'integer' ? Number(value) : value
        }
      }
      payload = JSON.stringify(object)
    }
    setLoading(true); setError('')
    try { const response = await backend.callMCPTool(server.id, tool.name, payload); setResult(formatResult(response.structuredContent && response.structuredContent !== 'null' ? response.structuredContent : response.content)); if (response.isError) setError(t('mcp.toolReturnedError')) }
    catch (cause) { setError(backendErrorMessage(cause, t('mcp.toolCallFailed'))) }
    finally { setLoading(false); onCalled?.() }
  }
  return <div className={styles.mcpRunPanel}>
    <div className="mb-2 flex items-center justify-between"><h4>{t('mcp.arguments')}</h4><Button size="xs" variant="ghost" onClick={() => setJsonMode((value) => !value)}>{jsonMode ? t('mcp.formMode') : t('mcp.jsonMode')}</Button></div>
    {jsonMode ? <textarea className={styles.mcpArgsEditor} value={args} onChange={(event) => setArgs(event.target.value)} spellCheck={false} /> : <div className="space-y-2">{fields.map((field) => <label key={field.name} className="block text-[10px]"><span className="mb-1 block font-medium">{field.name}{field.required && <span className="text-destructive"> *</span>}</span>{field.type === 'boolean' ? <input type="checkbox" checked={values[field.name] === true} onChange={(event) => setValues((current) => ({ ...current, [field.name]: event.target.checked }))} /> : field.options?.length ? <select className={styles.mcpArgInput} value={String(values[field.name] ?? '')} onChange={(event) => setValues((current) => ({ ...current, [field.name]: event.target.value }))}><option value="">—</option>{field.options.map((option) => <option key={option} value={option}>{option}</option>)}</select> : field.type === 'object' || field.type === 'array' ? <textarea className={styles.mcpArgJson} value={String(values[field.name] ?? '')} onChange={(event) => setValues((current) => ({ ...current, [field.name]: event.target.value }))} placeholder={field.type === 'array' ? '[]' : '{}'} spellCheck={false} /> : <input className={styles.mcpArgInput} type={field.type === 'number' || field.type === 'integer' ? 'number' : 'text'} value={String(values[field.name] ?? '')} onChange={(event) => setValues((current) => ({ ...current, [field.name]: event.target.value }))} />}{field.description && <span className="mt-0.5 block text-muted-foreground">{field.description}</span>}</label>)}</div>}
    {error && <p className={styles.planError} role="alert">{error}</p>}
    <div className="mt-2 flex justify-end gap-2">{loading ? <Button size="sm" variant="outline" onClick={() => void backend.cancelMCPTool(server.id)}>{t('common.cancel')}</Button> : <Button size="sm" onClick={() => void run()}>{result ? <Play size={13} /> : <Play size={13} />}{t('mcp.runTool')}</Button>}</div>
    <div className={styles.mcpRunResult}>{result ? <><div className="mb-2 flex justify-end"><Button size="xs" variant="ghost" onClick={() => void copyText(result)}>{t('common.copy')}</Button></div><ToolResult value={result} /></> : <p>{t('mcp.resultPlaceholder')}</p>}</div>
  </div>
}

function ResourceReader({ server, uri }: { server: MCPServer; uri: string }) {
  const [content, setContent] = useState(''); const [error, setError] = useState(''); const [loading, setLoading] = useState(false)
  const load = async () => { setLoading(true); setError(''); try { setContent(formatResult((await backend.readMCPResource(server.id, uri)).content)) } catch (cause) { setError(backendErrorMessage(cause, t('mcp.resourceReadFailed'))) } finally { setLoading(false) } }
  return <div className={styles.mcpInlinePanel}><div className="mb-2 flex items-center justify-between"><code className="text-[10px]">{uri}</code><Button size="xs" disabled={loading} onClick={() => void load()}>{loading ? <Loader2 size={13} className="animate-spin" /> : null}{t('mcp.readResource')}</Button></div>{error && <p className={styles.planError}>{error}</p>}{content && <ResourceContent value={content} />}</div>
}

function PromptContent({ value }: { value: string }) {
  try {
    const parsed = JSON.parse(value) as { messages?: Array<{ role?: string; content?: { type?: string; text?: string; data?: string; mimeType?: string; resource?: { text?: string; uri?: string; mimeType?: string } } }> }
    if (!parsed.messages?.length) return <pre>{value}</pre>
    return <div className={styles.mcpMessages}>{parsed.messages.map((message, index) => {
      const content = message.content
      return <div key={index} className={styles.mcpMessage}><span className={styles.mcpMessageRole}>{message.role || 'message'}</span>{content?.type === 'text' && <p>{content.text}</p>}{content?.type === 'image' && content.data && <img className={styles.mcpResultImage} src={`data:${content.mimeType || 'image/png'};base64,${content.data}`} alt={`Prompt ${message.role || ''}`} />}{content?.type === 'resource' && content.resource?.text && <p>{content.resource.text}</p>}{content?.type === 'resource' && content.resource?.uri && <code>{content.resource.uri}</code>}{!content && <p>{JSON.stringify(message)}</p>}</div>
    })}</div>
  } catch { return <pre>{value}</pre> }
}

function ResourceContent({ value }: { value: string }) {
  try {
    const parsed = JSON.parse(value) as Array<{ uri?: string; mimeType?: string; text?: string; blob?: string }>
    if (!Array.isArray(parsed)) return <pre>{value}</pre>
    return <div className={styles.mcpResultBlocks}>{parsed.map((item, index) => {
      if (item.text) return <pre key={index}>{item.text}</pre>
      if (item.blob && item.mimeType?.startsWith('image/')) return <img key={index} className={styles.mcpResultImage} src={`data:${item.mimeType};base64,${item.blob}`} alt={item.uri || 'MCP resource'} />
      if (item.blob) return <a key={index} className={styles.mcpResourceDownload} href={`data:${item.mimeType || 'application/octet-stream'};base64,${item.blob}`} download={item.uri?.split('/').pop() || 'resource'}>Download {item.mimeType || 'binary'}</a>
      if (item.uri) return <a key={index} className={styles.mcpResourceDownload} href={item.uri} target="_blank" rel="noreferrer">{item.uri}</a>
      return <pre key={index}>{JSON.stringify(item, null, 2)}</pre>
    })}</div>
  } catch { return <pre>{value}</pre> }
}

function PromptRunner({ server, prompt }: { server: MCPServer; prompt: MCPPrompt }) {
  const [values, setValues] = useState<Record<string, string>>({}); const [content, setContent] = useState(''); const [error, setError] = useState(''); const [loading, setLoading] = useState(false)
  const run = async () => {
    for (const field of prompt.arguments ?? []) if (field.required && !values[field.name]) { setError(t('mcp.requiredArgument', { name: field.name })); return }
    setLoading(true); setError('')
    try { setContent(formatResult((await backend.getMCPPrompt(server.id, prompt.name, JSON.stringify(values))).content)) }
    catch (cause) { setError(backendErrorMessage(cause, t('mcp.promptReadFailed'))) }
    finally { setLoading(false) }
  }
  return <div className={styles.mcpInlinePanel}><div className="space-y-2">{(prompt.arguments ?? []).map((field) => <label key={field.name} className="block text-[10px]"><span className="mb-1 block font-medium">{field.name}{field.required && <span className="text-destructive"> *</span>}</span><input className={styles.mcpArgInput} value={values[field.name] ?? ''} onChange={(event) => setValues((current) => ({ ...current, [field.name]: event.target.value }))} /></label>)}</div>{error && <p className={styles.planError}>{error}</p>}<div className="mt-2 flex justify-end"><Button size="sm" onClick={() => void run()} disabled={loading}>{loading ? <Loader2 size={13} className="animate-spin" /> : <Play size={13} />}{t('mcp.getPrompt')}</Button></div>{content && <PromptContent value={content} />}</div>
}

function PendingConnection({ loading, error, onRetry }: { loading: boolean; error: string; onRetry: () => void }) {
  if (loading) return <div className={styles.mcpInlineEmpty}><Loader2 size={16} className="animate-spin" /><p>{t('mcp.connecting')}</p><p className={styles.mcpMuted}>{t('mcp.oauthHint')}</p></div>
  return <div className={styles.mcpInlineEmpty}>{error && <p className={styles.planError}>{error}</p>}<p>{t('mcp.connectFirst')}</p><Button size="sm" onClick={onRetry}>{t('mcp.connect')}</Button></div>
}

export function MCPCapabilityWorkspace({ server, onRefresh, onRemove, autoConnect = false }: { server: MCPServer; onRefresh: () => Promise<void>; onRemove: () => void; autoConnect?: boolean }) {
  const [tab, setTab] = useState<Tab>('details')
  const [data, setData] = useState<MCPInspection>(); const [loading, setLoading] = useState(false); const [error, setError] = useState('')
  const [selectedTool, setSelectedTool] = useState<string>(); const [selectedResource, setSelectedResource] = useState<MCPResource>(); const [selectedTemplate, setSelectedTemplate] = useState<MCPResourceTemplate>(); const [templateValues, setTemplateValues] = useState<Record<string, string>>({}); const [selectedPrompt, setSelectedPrompt] = useState<MCPPrompt>()
  const [history, setHistory] = useState<MCPToolCallRecord[]>([]); const [query, setQuery] = useState('')
  const [runtimeLogs, setRuntimeLogs] = useState<MCPLogEntry[]>([])
  const [authStatus, setAuthStatus] = useState<MCPAuthStatus>()
  const [revealedFields, setRevealedFields] = useState<Record<string, boolean>>({})
  const [editing, setEditing] = useState(false)
  const [saving, setSaving] = useState(false)
  const [saveError, setSaveError] = useState('')
  const [draftName, setDraftName] = useState(server.name)
  const [draftTransport, setDraftTransport] = useState<'stdio' | 'http'>(server.url ? 'http' : 'stdio')
  const [draftCommand, setDraftCommand] = useState(server.command ?? '')
  const [draftArgs, setDraftArgs] = useState((server.args ?? []).join(' '))
  const [draftURL, setDraftURL] = useState(server.url ?? '')
  const [draftPaths, setDraftPaths] = useState<string[]>(server.agents.filter((agent) => agent.enabled).map((agent) => agent.path))
  const [draftEnv, setDraftEnv] = useState<Record<string, string>>(server.envValues ?? {})
  const [draftHeaders, setDraftHeaders] = useState<Record<string, string>>(server.headerValues ?? {})
  const [agentTargets, setAgentTargets] = useState<MCPAgent[]>([])
  const [newEnvKey, setNewEnvKey] = useState('')
  const [newEnvValue, setNewEnvValue] = useState('')
  const [newHeaderKey, setNewHeaderKey] = useState('')
  const [newHeaderValue, setNewHeaderValue] = useState('')
  const refreshHistory = () => { backend.mcpToolHistory(server.id).then(setHistory).catch(() => {}) }
  const refreshLogs = () => { backend.mcpRuntimeLogs(server.id).then((items) => setRuntimeLogs((items as MCPLogEntry[]) ?? [])).catch(() => {}) }
  const logEntries = runtimeLogs.length > 0 ? runtimeLogs : (data?.logs ?? [])
  const [target, setTarget] = useState<MCPInspectTarget>(); const [agentBusy, setAgentBusy] = useState(''); const [agentError, setAgentError] = useState('')
  useEffect(() => () => { void backend.cancelInspectMCP(server.id).catch(() => {}); void backend.closeMCPSession(server.id).catch(() => {}) }, [server.id])
  useEffect(() => { setTab('details'); setEditing(false); setDraftName(server.name); setDraftTransport(server.url ? 'http' : 'stdio'); setDraftCommand(server.command ?? ''); setDraftArgs((server.args ?? []).join(' ')); setDraftURL(server.url ?? ''); setDraftPaths(server.agents.filter((agent) => agent.enabled).map((agent) => agent.path)); setDraftEnv(server.envValues ?? {}); setDraftHeaders(server.headerValues ?? {}); setData(undefined); setSelectedTool(undefined); setSelectedResource(undefined); setSelectedTemplate(undefined); setSelectedPrompt(undefined); setError('') }, [server.id])
  useEffect(() => { if (tab !== 'history') return; backend.mcpToolHistory(server.id).then(setHistory).catch(() => {}) }, [tab, server.id])
  useEffect(() => { if (tab !== 'logs') return; refreshLogs() }, [tab, server.id])
  const saveInline = async () => {
    setSaving(true); setSaveError('')
    try {
      const plan = await backend.planMCPUpdate(server.id, draftName.trim(), draftTransport === 'stdio' ? draftCommand.trim() : '', draftTransport === 'stdio' && draftArgs.trim() ? draftArgs.trim().split(/\s+/) : [], draftTransport === 'http' ? draftURL.trim() : '', draftEnv, draftHeaders, draftPaths)
      await backend.applyPlan(plan.id)
      setEditing(false); setData(undefined)
      const savedAgents = editableAgents.filter((agent) => draftPaths.includes(agent.path)).map((agent) => agent.label)
      toastSuccess(t('mcp.configurationSaved'), savedAgents.length > 0 ? savedAgents.join(' · ') : t('common.agents', { n: draftPaths.length }))
      await backend.invalidateMCPCapability(server.id)
      await backend.closeMCPSession(server.id)
      await onRefresh()
    } catch (cause) { setSaveError(backendErrorMessage(cause, t('mcp.failedToUpdateMcp'))) } finally { setSaving(false) }
  }
  const connect = async () => { setLoading(true); setError(''); try { const result = await backend.inspectMCP(server.id); setData(result); backend.mcpAuthStatus(server.id).then(setAuthStatus).catch(() => {}) } catch (cause) { const message = backendErrorMessage(cause, t('mcp.inspectFailed')); setError(message.includes('deadline') || message.includes('context') ? t('mcp.inspectTimeout') : message) } finally { setLoading(false) } }
  useEffect(() => { if (autoConnect) void connect() }, [server.id, autoConnect])
  useEffect(() => { backend.mcpInspectTarget(server.id).then(setTarget).catch(() => setTarget(undefined)); backend.mcpAuthStatus(server.id).then(setAuthStatus).catch(() => setAuthStatus(undefined)); backend.mcpTargets().then((items) => setAgentTargets((items as MCPAgent[]) ?? [])).catch(() => setAgentTargets([])) }, [server.id])
  const toggleAgent = async (agent: MCPAgent) => { setAgentBusy(agent.path); setAgentError(''); try { if (agent.enabled) await backend.disableMCPAgent(server.id, agent.path); else await backend.enableMCPAgent(server.name, agent.path); await onRefresh() } catch (cause) { setAgentError(backendErrorMessage(cause, t('mcp.failedToDisableMcpAgent'))) } finally { setAgentBusy('') } }
  const tabs: Array<{ id: Tab; label: string; count?: number }> = [{ id: 'details', label: t('mcp.details') },{ id: 'tools', label: t('mcp.tools'), count: data?.tools?.length }, { id: 'resources', label: t('mcp.resources'), count: data?.resources?.length }, { id: 'prompts', label: t('mcp.prompts'), count: data?.prompts?.length }, { id: 'history', label: t('mcp.callHistory'), count: history.length }, { id: 'logs', label: t('mcp.logs'), count: data?.logs?.length ?? 0 }]
  const tools = (data?.tools ?? []).filter((tool) => !query.trim() || `${tool.name} ${tool.description ?? ''}`.toLowerCase().includes(query.toLowerCase()))
  const activeTool = data?.tools?.find((tool) => tool.name === selectedTool)
  const editableAgents = [...server.agents, ...agentTargets.filter((target) => !server.agents.some((agent) => agent.path === target.path))]
  const displayedAgents = editing ? editableAgents : server.agents
  return <div className={styles.mcpWorkspace2}>
    <div className={styles.mcpWorkspaceHeader}>
      <div className="flex min-w-0 items-start gap-3"><span className={styles.mcpHero}><MCPIcon name={server.name} command={server.command} url={server.url} className="size-5" /></span><div className="min-w-0 flex-1"><div className="flex items-center gap-2"><h2 className="truncate text-[15px] font-semibold">{server.name}</h2><Badge variant={statusTone(server.status)}>{statusLabel(server.status)}</Badge><Badge variant={data ? 'success' : loading ? 'outline' : 'warning'}>{data ? t('mcp.connectedShort') : loading ? t('mcp.connecting') : t('mcp.notConnected')}</Badge></div><p className="mt-1 text-[11px] text-muted-foreground">{server.transport} · {t('common.agents', { n: server.agents.length })}{data?.serverName ? ` · ${data.serverName} ${data.serverVersion}` : ''}</p></div></div>
      <div className="flex items-center gap-1 pr-9"><Button size="xs" variant={data ? "outline" : "default"} onClick={() => void connect()} disabled={loading}>{loading ? <Loader2 size={12} className="animate-spin" /> : data ? <RefreshCw size={12} /> : <Play size={12} />}{data ? t('mcp.reconnect') : loading ? t('mcp.connecting') : t('mcp.connect')}</Button>{editing ? <><Button size="xs" variant="ghost" onClick={() => setEditing(false)} disabled={saving}>{t('common.cancel')}</Button><Button size="xs" onClick={() => void saveInline()} disabled={saving}>{saving ? t('common.working') : t('common.save')}</Button></> : <><Button size="xs" variant="outline" onClick={() => setEditing(true)}>{t('common.editAction')}</Button><Button size="xs" variant="ghost" className="text-destructive" onClick={onRemove}>{t('common.removeAction')}</Button></>}</div>
    </div>
    <div className={styles.mcpWorkspaceTabs}>{tabs.map((item) => <button key={item.id} type="button" className={item.id === tab ? styles.mcpWorkspaceTabActive : styles.mcpWorkspaceTab} onClick={() => setTab(item.id)}>{item.label}{item.count !== undefined && <span>{item.count}</span>}</button>)}</div>
    <div className={styles.mcpWorkspaceBody}>
      {tab === 'details' && <div className={styles.mcpDetailsTab}>
        {!editing && error && <p className={styles.planError} role="alert">{error}</p>}
        {editing && saveError && <p className={styles.planError} role="alert">{saveError}</p>}
        <section className={styles.mcpDetailBlock}><h3>{t('mcp.launch')}</h3>{editing ? <div className={styles.mcpInlineLaunch}><div className={styles.mcpInlineField}><span>{t('common.name')}</span><Input value={draftName} onChange={(event) => setDraftName(event.target.value)} /></div><div className={styles.mcpInlineField}><span>{t('common.type')}</span><div className={styles.mcpInlineTransport}><button type="button" className={draftTransport === 'stdio' ? styles.mcpInlineTransportActive : styles.mcpInlineTransportButton} onClick={() => { setDraftTransport('stdio'); setDraftURL('') }}>stdio</button><button type="button" className={draftTransport === 'http' ? styles.mcpInlineTransportActive : styles.mcpInlineTransportButton} onClick={() => { setDraftTransport('http'); setDraftCommand('') }}>http</button></div></div>{draftTransport === 'stdio' ? <><div className={styles.mcpInlineField}><span>{t('common.commandLabel')}</span><Input value={draftCommand} onChange={(event) => setDraftCommand(event.target.value)} placeholder="npx" /></div><div className={styles.mcpInlineField}><span>{t('common.arguments')}</span><Input value={draftArgs} onChange={(event) => setDraftArgs(event.target.value)} placeholder="-y @scope/server" /></div></> : <div className={styles.mcpInlineField}><span>URL</span><Input value={draftURL} onChange={(event) => setDraftURL(event.target.value)} placeholder="https://example.com/mcp" /></div>}</div> : <><code className={styles.mcpDetailCode}>{server.command ? [server.command, ...(server.args ?? [])].join(' ') : server.url || '—'}</code>{target?.agentLabel && <p className={styles.mcpMuted}>{t('mcp.configSource')}: {target.agentLabel}{target.scope ? ` · ${shortPath(target.scope)}` : ''}</p>}</>}</section>
        {authStatus?.configured && <section className={styles.mcpDetailBlock}><div className="flex items-center justify-between"><div><h3>{t('mcp.authentication')}</h3><p>{t('mcp.authorized')}{authStatus.expiresAt ? ` · ${t('mcp.expiresAt', { date: new Date(authStatus.expiresAt).toLocaleString() })}` : ''}</p></div><Button size="xs" variant="outline" onClick={() => { void backend.clearMCPAuth(server.id).then(() => { setAuthStatus(undefined); setData(undefined) }).catch(() => {}) }}>{t('mcp.clearAuthorization')}</Button></div></section>}
        <section className={styles.mcpDetailBlock}><div className="flex items-baseline justify-between"><h3>{t('common.agentConfiguration')}</h3><span className={styles.mcpMuted}>{server.agents.length}</span></div>{displayedAgents.map((agent) => <div key={`${agent.label}:${agent.path}`} className={styles.mcpAgentRow}><AgentIcon name={agent.label} className="size-4" /><div className="min-w-0 flex-1"><strong>{agent.label}</strong><small>{agent.scope || shortPath(agent.path)}</small></div>{editing ? <input type="checkbox" checked={draftPaths.includes(agent.path)} onChange={(event) => setDraftPaths((current) => event.target.checked ? [...new Set([...current, agent.path])] : current.filter((path) => path !== agent.path))} /> : <><Badge variant={agent.enabled ? 'success' : 'outline'}>{agent.enabled ? t('common.enabled') : t('common.disabled')}</Badge><Button size="xs" variant={agent.enabled ? 'ghost' : 'outline'} disabled={Boolean(agentBusy)} onClick={() => void toggleAgent(agent)}>{agentBusy === agent.path ? t('common.working') : agent.enabled ? t('common.disableAction') : t('mcp.enable')}</Button></>}</div>)}{agentError && <p className={styles.planError}>{agentError}</p>}</section>
        {(Object.keys(editing ? draftEnv : (server.envValues ?? {})).length > 0 || Object.keys(editing ? draftHeaders : (server.headerValues ?? {})).length > 0 || editing) && <section className={styles.mcpDetailBlock}><h3>{t('mcp.configurationFields')}</h3><p className={styles.mcpMuted}>{t('mcp.configurationFieldsHint')}</p><div className={styles.mcpConfigFields}>{Object.entries(editing ? draftEnv : (server.envValues ?? {})).map(([key, value]) => <div key={`env:${key}`} className={styles.mcpConfigField}><span>{key}</span>{editing ? <Input type={/(key|token|secret|password|authorization|auth)/i.test(key) ? 'password' : 'text'} value={draftEnv[key] ?? ''} onChange={(event) => setDraftEnv((current) => ({ ...current, [key]: event.target.value }))} /> : <code>{/(key|token|secret|password|authorization|auth)/i.test(key) && !revealedFields[`env:${key}`] ? '••••••••' : value || '—'}</code>}{editing ? <Button size="xs" variant="ghost" onClick={() => setDraftEnv((current) => { const next = { ...current }; delete next[key]; return next })}>×</Button> : /(key|token|secret|password|authorization|auth)/i.test(key) && value && <Button size="xs" variant="ghost" onClick={() => setRevealedFields((current) => ({ ...current, [`env:${key}`]: !current[`env:${key}`] }))}>{revealedFields[`env:${key}`] ? t('mcp.hide') : t('mcp.reveal')}</Button>}</div>)}{Object.entries(editing ? draftHeaders : (server.headerValues ?? {})).map(([key, value]) => <div key={`header:${key}`} className={styles.mcpConfigField}><span>{key}</span>{editing ? <Input type={/(authorization|key|token|secret|password)/i.test(key) ? 'password' : 'text'} value={draftHeaders[key] ?? ''} onChange={(event) => setDraftHeaders((current) => ({ ...current, [key]: event.target.value }))} /> : <code>{/(key|token|secret|password|authorization|auth)/i.test(key) && !revealedFields[`header:${key}`] ? '••••••••' : value || '—'}</code>}{editing ? <Button size="xs" variant="ghost" onClick={() => setDraftHeaders((current) => { const next = { ...current }; delete next[key]; return next })}>×</Button> : /(key|token|secret|password|authorization|auth)/i.test(key) && value && <Button size="xs" variant="ghost" onClick={() => setRevealedFields((current) => ({ ...current, [`header:${key}`]: !current[`header:${key}`] }))}>{revealedFields[`header:${key}`] ? t('mcp.hide') : t('mcp.reveal')}</Button>}</div>)}{editing && <><div className={styles.mcpConfigAddRow}><Input placeholder="ENV_KEY" value={newEnvKey} onChange={(event) => setNewEnvKey(event.target.value)} /><Input placeholder={t('common.value')} value={newEnvValue} onChange={(event) => setNewEnvValue(event.target.value)} /><Button size="xs" variant="outline" onClick={() => { const key = newEnvKey.trim(); if (!key || key in draftEnv) return; setDraftEnv((current) => ({ ...current, [key]: newEnvValue })); setNewEnvKey(''); setNewEnvValue('') }}>＋</Button></div><div className={styles.mcpConfigAddRow}><Input placeholder="Header-Name" value={newHeaderKey} onChange={(event) => setNewHeaderKey(event.target.value)} /><Input placeholder={t('common.value')} value={newHeaderValue} onChange={(event) => setNewHeaderValue(event.target.value)} /><Button size="xs" variant="outline" onClick={() => { const key = newHeaderKey.trim(); if (!key || key in draftHeaders) return; setDraftHeaders((current) => ({ ...current, [key]: newHeaderValue })); setNewHeaderKey(''); setNewHeaderValue('') }}>＋</Button></div></>}</div></section>}
      </div>}
      {tab === 'tools' && <div className={styles.mcpSplitTab}>
        {!data ? <PendingConnection loading={loading} error={error} onRetry={() => void connect()} /> : data.toolError ? <p className={styles.planError}>{data.toolError}</p> : <>
          <div className={styles.mcpSplitList}>
            <input className={styles.mcpArgInput} placeholder={t('mcp.searchTools')} value={query} onChange={(event) => setQuery(event.target.value)} />
            {tools.map((tool) => <button key={tool.name} type="button" className={`${styles.mcpToolRow} ${selectedTool === tool.name ? styles.mcpToolRowActive : ''}`} onClick={() => setSelectedTool(tool.name)}><div><strong>{tool.title || tool.name}</strong>{tool.description && <p>{tool.description}</p>}</div></button>)}
          </div>
          <div className={styles.mcpSplitDetail}>{activeTool ? <><div className={styles.mcpToolDetailHead}><div className="flex items-center gap-2"><h3>{activeTool.title || activeTool.name}</h3>{((activeTool.annotations as { destructiveHint?: boolean } | undefined)?.destructiveHint) && <Badge variant="warning" title={t('mcp.mayHaveSideEffectsHint')}>{t('mcp.mayHaveSideEffects')}</Badge>}</div>{activeTool.description && <p>{activeTool.description}</p>}</div><ToolRunner server={server} tool={activeTool} onCalled={() => { refreshHistory(); refreshLogs() }} /></> : <p className={styles.mcpDetailEmpty}>{t('mcp.selectToolHint')}</p>}</div>
        </>}
      </div>}
      {tab === 'resources' && <div>{!data ? <PendingConnection loading={loading} error={error} onRetry={() => void connect()} /> : ((data.resources?.length ?? 0) + (data.resourceTemplates?.length ?? 0) === 0) ? <div className={styles.mcpInlineEmpty}><p>{t('mcp.noResources')}</p></div> : <div className="space-y-2">{(data.resources ?? []).map((resource) => <button key={resource.uri} type="button" className={`${styles.mcpResourceRow} ${selectedResource?.uri === resource.uri ? styles.mcpToolRowActive : ''}`} onClick={() => setSelectedResource(resource)}><code>{resource.uri}</code>{resource.name && <span>{resource.name}</span>}</button>)}{selectedResource && <ResourceReader server={server} uri={selectedResource.uri} />}{(data.resourceTemplates ?? []).map((template) => <div key={template.uriTemplate} className={styles.mcpResourceRow}><div><code>{template.uriTemplate}</code><small>{template.name}</small></div><Button size="xs" variant="outline" onClick={() => setSelectedTemplate(template)}>{t('mcp.resourceTemplate')}</Button></div>)}{selectedTemplate && <div className={styles.mcpInlinePanel}><h4>{selectedTemplate.uriTemplate}</h4>{Array.from(new Set(Array.from(selectedTemplate.uriTemplate.matchAll(/\{([^}]+)\}/g)).map((match) => match[1]))).map((name) => <input key={name} className={styles.mcpArgInput} placeholder={name} value={templateValues[name] ?? ''} onChange={(event) => setTemplateValues((current) => ({ ...current, [name]: event.target.value }))} />)}<Button size="xs" onClick={() => setSelectedResource({ uri: Object.entries(templateValues).reduce((uri, [key, value]) => uri.split(`{${key}}`).join(value), selectedTemplate.uriTemplate) })}>{t('mcp.readResource')}</Button></div>}</div>}</div>}
      {tab === 'prompts' && <div>{!data ? <PendingConnection loading={loading} error={error} onRetry={() => void connect()} /> : (data.prompts?.length ?? 0) === 0 ? <div className={styles.mcpInlineEmpty}><p>{t('mcp.noPrompts')}</p></div> : <div className="space-y-2">{(data.prompts ?? []).map((prompt) => <button key={prompt.name} type="button" className={`${styles.mcpToolRow} ${selectedPrompt?.name === prompt.name ? styles.mcpToolRowActive : ''}`} onClick={() => setSelectedPrompt(prompt)}><div><strong>{prompt.title || prompt.name}</strong>{prompt.description && <p>{prompt.description}</p>}</div></button>)}{selectedPrompt && <PromptRunner server={server} prompt={selectedPrompt} />}</div>}</div>}
      {tab === 'history' && <div>{history.length === 0 ? <div className={styles.mcpInlineEmpty}><p>{t('mcp.noCallHistory')}</p></div> : <div className="space-y-2">{history.map((record) => <details key={record.id} className={styles.mcpHistoryRow}><summary><strong>{record.tool}</strong><span>{new Date(record.createdAt).toLocaleString()}</span>{record.isError && <Badge variant="warning">error</Badge>}</summary><pre>{record.arguments}</pre>{record.content && <pre>{formatResult(record.content)}</pre>}</details>)}</div>}</div>}
      {tab === 'logs' && <div className={styles.mcpLogList}>{([...(data?.logs ?? []), ...runtimeLogs].length) === 0 ? <div className={styles.mcpInlineEmpty}><p>{t('mcp.noLogs')}</p></div> : logEntries.map((entry, index) => <details key={`${entry.at}:${index}`} className={styles.mcpLogRow}><summary><strong>{entry.method}</strong><span>{entry.durationMs} ms</span>{entry.error && <Badge variant="warning">{t('common.error')}</Badge>}</summary>{entry.error && <p className={styles.planError}>{entry.error}</p>}{entry.request && <><span className={styles.mcpLogLabel}>{t('mcp.logRequest')}</span><pre>{entry.request}</pre></>}{entry.response && <><span className={styles.mcpLogLabel}>{t('mcp.logResponse')}</span><pre>{entry.response}</pre></>}</details>)}</div>}
    </div>
  </div>
}
