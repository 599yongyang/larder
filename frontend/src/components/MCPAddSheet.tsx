import { useEffect, useRef, useState } from 'react'
import { Plus, Search } from 'lucide-react'
import { MCPIcon } from './MCPIcon'
import { AgentIcon } from './AgentIcon'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Checkbox } from '@/components/ui/checkbox'
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { Input } from '@/components/ui/input'
import { Textarea } from '@/components/ui/textarea'
import { Sheet, SheetContent, SheetDescription, SheetHeader, SheetTitle } from '@/components/ui/sheet'
import styles from '../App.module.css'
import { t } from '../i18n'
import { backendErrorMessage } from '../lib/backendError'
import { backend } from '../backend'
import { toastSuccess } from '../lib/toast'
import type { MCPAgent } from '../types'

type RegistryEnv = { name: string; description?: string; required?: boolean; secret?: boolean }
type RegistryHeader = { name: string; description?: string; required?: boolean; secret?: boolean }
function mapRegistryServers(value: Array<{ id: string; name: string; title: string; description: string; transport: 'stdio' | 'http'; command?: string; args?: string[]; url?: string; runtime?: string; requiresAuth?: boolean; publisher?: string; env?: RegistryEnv[]; headers?: RegistryHeader[]; unsupported?: string }>): FeaturedMCP[] {
  return value.map((item) => ({ ...item, publisher: item.publisher || '', source: 'registry' as const }))
}

type FeaturedMCP = { name: string; title: string; publisher?: string; description: string; transport: 'stdio' | 'http'; command?: string; args?: string[]; url?: string; runtime?: string; requiresAuth?: boolean; env?: RegistryEnv[]; headers?: RegistryHeader[]; source?: 'featured' | 'registry'; unsupported?: string }



export function MCPAddSheet({ open, existingNames, onClose, onAdded }: { open: boolean; existingNames: string[]; onClose: () => void; onAdded: () => void }) {
  const [query, setQuery] = useState('')
  const [selected, setSelected] = useState<FeaturedMCP>()
  const [envValues, setEnvValues] = useState<Record<string, string>>({})
  const [headerValues, setHeaderValues] = useState<Record<string, string>>({})
  const [recommended, setRecommended] = useState<FeaturedMCP[]>([])
  const [recommendedLoading, setRecommendedLoading] = useState(false)
  const [registry, setRegistry] = useState<FeaturedMCP[]>([])
  const [nextCursor, setNextCursor] = useState('')
  const [loadingMore, setLoadingMore] = useState(false)
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
  useEffect(() => { if (!open) return; setQuery(''); setSelected(undefined); setPaths([]); setError(''); setRegistry([]); setNextCursor(''); setRegistryError(''); backend.mcpTargets().then((value) => setTargets((value as MCPAgent[]) ?? [])).catch(() => setTargets([])); setRecommendedLoading(true); backend.discoverMCPRegistry().then((value) => setRecommended(value as FeaturedMCP[])).catch(() => setRecommended([])).finally(() => setRecommendedLoading(false)) }, [open])
  useEffect(() => {
    const term = query.trim()
    if (term.length < 2) { setRegistry([]); setRegistryLoading(false); setRegistryError(''); return }
    const request = ++registryRequest.current
    setRegistryLoading(true); setRegistryError('')
    const timer = window.setTimeout(() => {
      void backend.searchMCPRegistryPage(term, '').then((value) => {
        if (request !== registryRequest.current) return
        setRegistry(mapRegistryServers(value.servers))
        setNextCursor(value.nextCursor ?? '')
      }).catch((cause) => { if (request === registryRequest.current) setRegistryError(backendErrorMessage(cause, t('mcp.registrySearchFailed'))) }).finally(() => { if (request === registryRequest.current) setRegistryLoading(false) })
    }, 350)
    return () => window.clearTimeout(timer)
  }, [query])
  const requiredEnvMissing = Boolean(selected?.env?.some((env) => env.required && !envValues[env.name]?.trim()))
  const requiredHeadersMissing = Boolean(selected?.headers?.some((header) => header.required && !headerValues[header.name]?.trim()))
  const loadMore = async () => {
    const term = query.trim(); if (!term || !nextCursor || loadingMore) return
    setLoadingMore(true)
    try { const page = await backend.searchMCPRegistryPage(term, nextCursor); setRegistry((current) => [...current, ...mapRegistryServers(page.servers)]); setNextCursor(page.nextCursor ?? '') } catch (cause) { setRegistryError(backendErrorMessage(cause, t('mcp.registrySearchFailed'))) } finally { setLoadingMore(false) }
  }
  const openCustom = () => { setCustomName(''); setCustomTransport('stdio'); setCustomCommand(''); setCustomArgs(''); setCustomURL(''); setCustomEnv(''); setCustomPaths([]); setCustomError(''); setCustomOpen(true) }
  const installCustom = async () => {
    const env: Record<string, string> = {}
    for (const line of customEnv.split('\n')) { const trimmed = line.trim(); if (!trimmed) continue; const [key, ...rest] = trimmed.split('='); env[key.trim()] = rest.join('=').trim() }
    setCustomBusy(true); setCustomError('')
    try {
      const plan = await backend.planMCPAdd(customName.trim(), customTransport, customCommand.trim(), customArgs.trim() ? customArgs.trim().split(/\s+/) : [], customURL.trim(), env, {}, customPaths)
      await backend.applyPlan(plan.id)
      toastSuccess(t('mcp.customMcpAdded'), customName.trim())
      setCustomOpen(false); onAdded(); onClose()
    } catch (cause) {
      setCustomError(backendErrorMessage(cause, t('mcp.failedToAddMcp')))
    } finally { setCustomBusy(false) }
  }
  const install = async () => {
    if (!selected || paths.length === 0) return
    setBusy(true); setError('')
    try {
      const plan = await backend.planMCPAdd(selected.name, selected.transport, selected.command ?? '', selected.args ?? [], selected.url ?? '', envValues, headerValues, paths)
      await backend.applyPlan(plan.id)
      toastSuccess(t('mcp.mcpServerAdded'), selected.title)
      onAdded(); onClose()
    } catch (cause) {
      setError(backendErrorMessage(cause, t('mcp.failedToAddMcp')))
    } finally { setBusy(false) }
  }
  return <Sheet open={open} onOpenChange={(next) => { if (!next && !busy) onClose() }}>
    <SheetContent side="right" className={`${styles.mcpAddSheet} gap-0 p-0`} showCloseButton>
      <SheetHeader className={styles.discoverHeader}><SheetTitle className={styles.discoverTitle}>{t('mcp.addMcpServer')}</SheetTitle><SheetDescription className={styles.discoverDescription}>{t('mcp.curatedLocalMcpServersOnlyConfigFilesAreWrittenNoCommandsAreInstalledAndNoServerIsStarted')}</SheetDescription></SheetHeader>
      <div className={styles.discoverToolbar}><label className={styles.discoverSearchField}><Search size={13} aria-hidden="true" /><Input className={styles.discoverSearchInput} value={query} onChange={(event) => setQuery(event.target.value)} placeholder={t('mcp.searchMcpServers')} aria-label={t('common.searchMcpServers')} /></label><Button size="xs" variant="outline" onClick={openCustom}><Plus size={12} />{t('mcp.custom')}</Button></div>
      <div className={styles.mcpAddGrid}>
        {!query.trim() && recommended.map((server) => { const exists = existingNames.includes(server.name); return <section key={`recommended:${server.name}`} className={styles.discoverCard}><div className={styles.discoverCardHead}><span className={styles.discoverIcon}><MCPIcon name={server.name} command={server.command} url={server.url} className="size-4" /></span><div className={styles.discoverCardMain}><span className={styles.discoverCardName}>{server.title}</span>{server.publisher && <span className={styles.discoverCardSource}>{server.publisher}</span>}</div></div><p className={styles.discoverCardDescription} title={server.description}>{server.description}</p><div className={styles.discoverCardMeta}><div className={styles.mcpBadges}><Badge variant="outline">{server.transport === 'http' ? t('mcp.remote') : server.runtime || server.transport}</Badge>{server.requiresAuth && <Badge variant="outline">{t('mcp.loginRequired')}</Badge>}</div>{exists ? <span className={styles.discoverCardInstalled}>{t('mcp.alreadyAdded')}</span> : <Button size="xs" className={styles.discoverInstallButton} onClick={() => { setSelected(server); setEnvValues({}); setHeaderValues({}); setPaths([]); setError('') }}>{t('mcp.add')}</Button>}</div></section> })}
        {!query.trim() && recommendedLoading && recommended.length === 0 && <div className={styles.mcpRegistryState}>{t('mcp.queryingRegistryModelcontextprotocolIo')}</div>}
        {query.trim().length >= 2 && registryError && <div className={styles.mcpRegistryHeading}><small className={styles.discoverError}>{registryError}</small></div>}
        {query.trim().length >= 2 && registryLoading && registry.length === 0 && <div className={styles.mcpRegistryState}>{t('mcp.queryingRegistryModelcontextprotocolIo')}</div>}
        {query.trim().length >= 2 && !registryLoading && !registryError && registry.length === 0 && <div className={styles.mcpRegistryState}>{t('mcp.noInstallableResultsInTheRegistry')}</div>}
        {nextCursor && <div className={styles.mcpLoadMore}><Button size="xs" variant="outline" disabled={loadingMore} onClick={() => void loadMore()}>{loadingMore ? t('common.working') : t('mcp.loadMore')}</Button></div>}
        {registry.map((server) => { const exists = existingNames.includes(server.name); return <section key={server.name} className={styles.discoverCard}><div className={styles.discoverCardHead}><span className={styles.discoverIcon}><MCPIcon name={server.name} command={server.command} url={server.url} className="size-4" /></span><div className={styles.discoverCardMain}><span className={styles.discoverCardName}>{server.title}</span>{server.publisher && <span className={styles.discoverCardSource}>{server.publisher}</span>}</div></div><p className={styles.discoverCardDescription} title={server.description}>{server.description}</p><div className={styles.discoverCardMeta}><div className={styles.mcpBadges}><Badge variant={server.unsupported ? 'warning' : 'outline'}>{server.transport === 'http' ? t('mcp.remote') : server.runtime || server.transport || 'unknown'}</Badge>{server.requiresAuth && <Badge variant="outline">{t('mcp.loginRequired')}</Badge>}</div>{server.unsupported ? <span className={styles.mcpAddUnsupported} title={server.unsupported}>{server.unsupported}</span> : exists ? <span className={styles.discoverCardInstalled}>{t('mcp.alreadyAdded')}</span> : <Button size="xs" className={styles.discoverInstallButton} onClick={() => { setSelected(server); setEnvValues({}); setHeaderValues({}); setPaths([]); setError('') }}>{t('mcp.add')}</Button>}</div></section> })}
      </div>
    </SheetContent>
    {customOpen && <Dialog open onOpenChange={(next) => { if (!next && !customBusy) setCustomOpen(false) }}>
      <DialogContent className="max-w-[560px] gap-0 p-0">
        <DialogHeader className="border-b px-6 py-4 pr-12"><DialogTitle className="text-base">{t('mcp.customMcpServer')}</DialogTitle><DialogDescription className="text-xs">{t('mcp.writesAgentConfigOnlyDoesNotInstallCommandsOrStartAServer')}</DialogDescription></DialogHeader>
        <div className={styles.mcpCustomBody}>
          <div className={styles.mcpCustomRow}><span>{t('common.name')}</span><Input value={customName} onChange={(event) => setCustomName(event.target.value)} placeholder="my-server" /></div>
          <div className={styles.mcpCustomRow}><span>{t('common.type')}</span><div className={styles.mcpTransport}><button type="button" className={customTransport === 'stdio' ? styles.mcpTransportActive : styles.mcpTransportButton} onClick={() => setCustomTransport('stdio')}>stdio</button><button type="button" className={customTransport === 'http' ? styles.mcpTransportActive : styles.mcpTransportButton} onClick={() => setCustomTransport('http')}>http</button></div></div>
          {customTransport === 'stdio' ? <><div className={styles.mcpCustomRow}><span>{t('common.commandLabel')}</span><Input value={customCommand} onChange={(event) => setCustomCommand(event.target.value)} placeholder="npx" /></div><div className={styles.mcpCustomRow}><span>{t('common.arguments')}</span><Input value={customArgs} onChange={(event) => setCustomArgs(event.target.value)} placeholder="-y @scope/server" /></div><div className={styles.mcpCustomColumn}><span>{t('mcp.environmentVariables')}</span><Textarea value={customEnv} onChange={(event) => setCustomEnv(event.target.value)} placeholder={'API_KEY=...\nBASE_URL=...'} aria-label={t('mcp.environmentVariables')} /></div></> : <div className={styles.mcpCustomRow}><span>URL</span><Input value={customURL} onChange={(event) => setCustomURL(event.target.value)} placeholder="https://example.com/mcp" /></div>}
          <div className={styles.mcpCustomColumn}><span>{t('common.writeToAgentConfig')}</span><div className={styles.mcpAddTargets}>{targets.map((target) => <label key={target.path} className={styles.discoverAgent}><Checkbox checked={customPaths.includes(target.path)} onCheckedChange={(checked) => setCustomPaths((current) => checked ? [...new Set([...current, target.path])] : current.filter((path) => path !== target.path))} /><AgentIcon name={target.label} className="size-4" /><span>{target.label}</span></label>)}</div></div>
          {customError && <p className={styles.planError} role="alert">{customError}</p>}
        </div>
        <DialogFooter className="border-t px-6 py-2.5"><Button size="sm" variant="outline" disabled={customBusy} onClick={() => setCustomOpen(false)}>{t('common.cancel')}</Button><Button size="sm" disabled={customBusy || !customName.trim() || customPaths.length === 0 || (customTransport === 'stdio' ? !customCommand.trim() : !customURL.trim())} onClick={() => void installCustom()}>{customBusy ? t('common.adding') : t('mcp.add')}</Button></DialogFooter>
      </DialogContent>
    </Dialog>}
    {selected && <Dialog open onOpenChange={(next) => { if (!next && !busy) setSelected(undefined) }}>
      <DialogContent className="max-w-[480px] gap-0 p-0">
        <DialogHeader className="border-b px-6 py-4 pr-12"><DialogTitle className="flex items-center gap-2.5 text-base"><MCPIcon name={selected.name} command={selected.command} url={selected.url} className="size-6" />{t('mcp.addServerTitle', { name: selected.title })}</DialogTitle><DialogDescription className="text-xs">{[selected.publisher, selected.transport].filter(Boolean).join(' · ')}</DialogDescription></DialogHeader>
        <div className={styles.mcpAddDialogBody}><p className={styles.mcpInstallDescription}>{selected.description}</p><div className={styles.mcpInstallMethod}><div className={styles.discoverInstallLabel}>{t('common.launch')}</div><code>{selected.url || [selected.command, ...(selected.args ?? [])].filter(Boolean).join(' ') || '—'}</code></div>{selected.env && selected.env.length > 0 && <div className={styles.mcpRegistryEnv}><div className={styles.discoverInstallLabel}>{t('mcp.environmentVariables')}</div>{selected.env.map((env) => <label key={env.name}><span>{env.name}{env.required ? ' *' : ''}</span><Input type={env.secret ? 'password' : 'text'} value={envValues[env.name] ?? ''} onChange={(event) => setEnvValues((current) => ({ ...current, [env.name]: event.target.value }))} placeholder={env.description || env.name} /></label>)}</div>}{selected.headers && selected.headers.length > 0 && <div className={styles.mcpRegistryEnv}><div className={styles.discoverInstallLabel}>{t('mcp.requestHeaders')}</div>{selected.headers.map((header) => <label key={header.name}><span>{header.name}{header.required ? ' *' : ''}</span><Input type={header.secret ? 'password' : 'text'} value={headerValues[header.name] ?? ''} onChange={(event) => setHeaderValues((current) => ({ ...current, [header.name]: event.target.value }))} placeholder={header.description || header.name} /></label>)}</div>}
<div className={styles.discoverInstallLabel}>{t('common.writeToAgentConfig')}</div><div className={styles.mcpAddTargets}>{targets.map((target) => <label key={target.path} className={styles.discoverAgent}><Checkbox checked={paths.includes(target.path)} onCheckedChange={(checked) => setPaths((current) => checked ? [...new Set([...current, target.path])] : current.filter((path) => path !== target.path))} /><AgentIcon name={target.label} className="size-4" /><span>{target.label}</span></label>)}</div><p className={styles.installDialogHint}>{t('mcp.configFilesAreBackedUpNpxUvxAreNotInstalledAndTheMcpServerIsNotStarted')}</p>{error && <p className={styles.planError} role="alert">{error}</p>}</div>
        <DialogFooter className="border-t px-6 py-2.5"><Button size="sm" variant="outline" disabled={busy} onClick={() => setSelected(undefined)}>{t('common.cancel')}</Button><Button size="sm" disabled={busy || paths.length === 0 || requiredEnvMissing || requiredHeadersMissing} onClick={() => void install()}>{busy ? t('common.adding') : t('mcp.add')}</Button></DialogFooter>
      </DialogContent>
    </Dialog>}
  </Sheet>
}
