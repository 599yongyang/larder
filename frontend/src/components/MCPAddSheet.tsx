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
type FeaturedMCP = { name: string; title: string; publisher: string; description: string; transport: 'stdio' | 'http'; command?: string; args?: string[]; url?: string; env?: RegistryEnv[]; source?: 'featured' | 'registry'; unsupported?: string }

const FEATURED: FeaturedMCP[] = [
  { name: 'context7', title: 'Context7', publisher: 'Upstash', description: t('mcp.fetchCurrentLibraryDocsAndCodeExamples'), transport: 'http', url: 'https://mcp.context7.com/mcp' },
  { name: 'playwright', title: 'Playwright', publisher: 'Microsoft', description: t('mcp.driveARealBrowserOpenPagesClickTypeAndRead'), transport: 'stdio', command: 'npx', args: ['-y', '@playwright/mcp@latest'] },
  { name: 'chrome-devtools', title: 'Chrome DevTools', publisher: 'Google', description: t('mcp.inspectLiveChromeConsoleNetworkAndPerformance'), transport: 'stdio', command: 'npx', args: ['-y', 'chrome-devtools-mcp@latest'] },
  { name: 'deepwiki', title: 'DeepWiki', publisher: 'Cognition', description: t('mcp.askQuestionsAndReadGeneratedWikisForPublicRepositories'), transport: 'http', url: 'https://mcp.deepwiki.com/mcp' },
  { name: 'memory', title: 'Memory', publisher: 'Model Context Protocol', description: t('mcp.localKnowledgeGraphPersistedAcrossConversations'), transport: 'stdio', command: 'npx', args: ['-y', '@modelcontextprotocol/server-memory'] },
  { name: 'sequential-thinking', title: 'Sequential Thinking', publisher: 'Model Context Protocol', description: t('mcp.stepByStepReasoningWithBacktrackingAndBranching'), transport: 'stdio', command: 'npx', args: ['-y', '@modelcontextprotocol/server-sequential-thinking'] },
  { name: 'fetch', title: 'Fetch', publisher: 'Model Context Protocol', description: t('mcp.fetchPagesAndConvertThemToMarkdown'), transport: 'stdio', command: 'uvx', args: ['mcp-server-fetch'] },
  { name: 'git', title: 'Git', publisher: 'Model Context Protocol', description: t('mcp.readSearchAndModifyGitRepositories'), transport: 'stdio', command: 'uvx', args: ['mcp-server-git'] },
  { name: 'time', title: 'Time', publisher: 'Model Context Protocol', description: t('mcp.getTimezoneAwareTimeAndConvertTimestamps'), transport: 'stdio', command: 'uvx', args: ['mcp-server-time'] },
  { name: 'exa', title: 'Exa', publisher: 'Exa', description: t('mcp.webAndCodeSearchForAgents'), transport: 'http', url: 'https://mcp.exa.ai/mcp' },
  { name: 'figma', title: 'Figma', publisher: 'Figma', description: t('mcp.readDesignFilesComponentsAndStyleContext'), transport: 'http', url: 'https://mcp.figma.com/mcp' },
  { name: 'notion', title: 'Notion', publisher: 'Notion', description: t('mcp.searchReadAndWriteNotionPages'), transport: 'http', url: 'https://mcp.notion.com/mcp' },
  { name: 'linear', title: 'Linear', publisher: 'Linear', description: t('mcp.findCreateAndUpdateLinearIssues'), transport: 'http', url: 'https://mcp.linear.app/mcp' },
  { name: 'sentry', title: 'Sentry', publisher: 'Sentry', description: t('mcp.readIssuesErrorsAndTracesThenAnalyzeThem'), transport: 'http', url: 'https://mcp.sentry.dev/mcp' },
  { name: 'atlassian', title: 'Atlassian', publisher: 'Atlassian', description: t('mcp.readJiraIssuesAndConfluencePages'), transport: 'http', url: 'https://mcp.atlassian.com/v1/mcp' },
  { name: 'supabase', title: 'Supabase', publisher: 'Supabase', description: t('mcp.manageSupabaseProjectsSqlMigrationsAndLogs'), transport: 'http', url: 'https://mcp.supabase.com/mcp' },
  { name: 'stripe', title: 'Stripe', publisher: 'Stripe', description: t('mcp.queryCustomersPaymentsSubscriptionsAndStripeDocs'), transport: 'http', url: 'https://mcp.stripe.com' },
  { name: 'vercel', title: 'Vercel', publisher: 'Vercel', description: t('mcp.readVercelProjectsDeploymentsAndLogs'), transport: 'http', url: 'https://mcp.vercel.com' },
  { name: 'neon', title: 'Neon', publisher: 'Neon', description: t('mcp.manageNeonPostgresProjectsBranchesAndSql'), transport: 'http', url: 'https://mcp.neon.tech/mcp' },
  { name: 'cloudflare-docs', title: 'Cloudflare Docs', publisher: 'Cloudflare', description: t('mcp.searchCloudflareDocumentation'), transport: 'http', url: 'https://docs.mcp.cloudflare.com/mcp' },
  { name: 'microsoft-learn', title: 'Microsoft Learn', publisher: 'Microsoft', description: t('mcp.officialMicrosoftAndAzureDocsPlusCodeSamples'), transport: 'http', url: 'https://learn.microsoft.com/api/mcp' },
  { name: 'huggingface', title: 'Hugging Face', publisher: 'Hugging Face', description: t('mcp.searchModelsDatasetsSpacesAndPapers'), transport: 'http', url: 'https://huggingface.co/mcp' },
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
      }).catch((cause) => { if (request === registryRequest.current) setRegistryError(backendErrorMessage(cause, t('mcp.registrySearchFailed'))) }).finally(() => { if (request === registryRequest.current) setRegistryLoading(false) })
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
      toastSuccess(t('mcp.customMcpAdded'), customName.trim())
      setCustomOpen(false); onAdded(); onClose()
    } catch (cause) {
      setCustomError(backendErrorMessage(cause, t('mcp.failedToAddMcp')))
    } finally { setCustomBusy(false) }
  }
  const filtered = FEATURED.filter((server) => !query.trim() || `${server.title} ${server.name} ${server.publisher} ${server.description}`.toLowerCase().includes(query.trim().toLowerCase()))
  const install = async () => {
    if (!selected || paths.length === 0) return
    setBusy(true); setError('')
    try {
      const plan = await backend.planMCPAdd(selected.name, selected.transport, selected.command ?? '', selected.args ?? [], selected.url ?? '', envValues, paths)
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
      <div className={styles.mcpAddGrid}>{filtered.map((server) => {
        const exists = existingNames.includes(server.name)
        return <section key={server.name} className={styles.mcpAddCard}><div className={styles.mcpAddCardHead}><span className={styles.mcpAddIcon}><MCPIcon name={server.name} command={server.command} url={server.url} className="size-5" /></span><div><strong>{server.title}</strong><small>{server.publisher}</small></div></div><p title={server.description}>{server.description}</p><div className={styles.mcpAddCardFoot}><Badge variant="outline">{server.transport}</Badge>{exists ? <span className={styles.mcpAddExists}>{t('mcp.alreadyAdded')}</span> : <Button size="xs" variant="outline" onClick={() => { setSelected(server); setEnvValues({}); setPaths([]); setError('') }}><Plus size={12} />{t('mcp.add')}</Button>}</div></section>
      })}
        {query.trim().length >= 2 && <div className={styles.mcpRegistryHeading}><span>{t('mcp.officialRegistry')}</span>{registryLoading && <small>{t('mcp.searching')}</small>}{registryError && <small className={styles.discoverError}>{registryError}</small>}</div>}
        {query.trim().length >= 2 && registryLoading && registry.length === 0 && <div className={styles.mcpRegistryState}>{t('mcp.queryingRegistryModelcontextprotocolIo')}</div>}
        {query.trim().length >= 2 && !registryLoading && !registryError && registry.length === 0 && <div className={styles.mcpRegistryState}>{t('mcp.noInstallableResultsInTheRegistry')}</div>}
        {registry.map((server) => { const exists = existingNames.includes(server.name); return <section key={server.name} className={styles.mcpAddCard}><div className={styles.mcpAddCardHead}><span className={styles.mcpAddIcon}><MCPIcon name={server.name} command={server.command} url={server.url} className="size-5" /></span><div><strong>{server.title}</strong><small>MCP Registry</small></div></div><p title={server.description}>{server.description}</p><div className={styles.mcpAddCardFoot}><Badge variant={server.unsupported ? 'warning' : 'outline'}>{server.transport || 'unknown'}</Badge>{server.unsupported ? <span className={styles.mcpAddUnsupported} title={server.unsupported}>{server.unsupported}</span> : exists ? <span className={styles.mcpAddExists}>{t('mcp.alreadyAdded')}</span> : <Button size="xs" variant="outline" onClick={() => { setSelected(server); setEnvValues({}); setPaths([]); setError('') }}><Plus size={12} />{t('mcp.add')}</Button>}</div></section> })}
      </div>
      {filtered.length === 0 && query.trim().length < 2 && <div className={styles.discoverState}>{t('mcp.noCuratedMcpServersFound')}</div>}
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
        <DialogHeader className="border-b px-6 py-4 pr-12"><DialogTitle className="flex items-center gap-2.5 text-base"><MCPIcon name={selected.name} command={selected.command} url={selected.url} className="size-6" />{t('mcp.addServerTitle', { name: selected.title })}</DialogTitle><DialogDescription className="text-xs">{selected.publisher} · {selected.transport}</DialogDescription></DialogHeader>
        <div className={styles.mcpAddDialogBody}><p className={styles.mcpInstallDescription}>{selected.description}</p><div className={styles.mcpInstallMethod}><div className={styles.discoverInstallLabel}>{t('common.launch')}</div><code>{selected.url || [selected.command, ...(selected.args ?? [])].filter(Boolean).join(' ') || '—'}</code></div>{selected.env && selected.env.length > 0 && <div className={styles.mcpRegistryEnv}><div className={styles.discoverInstallLabel}>{t('mcp.environmentVariables')}</div>{selected.env.map((env) => <label key={env.name}><span>{env.name}{env.required ? ' *' : ''}</span><Input type={env.secret ? 'password' : 'text'} value={envValues[env.name] ?? ''} onChange={(event) => setEnvValues((current) => ({ ...current, [env.name]: event.target.value }))} placeholder={env.description || env.name} /></label>)}</div>}<div className={styles.discoverInstallLabel}>{t('common.writeToAgentConfig')}</div><div className={styles.mcpAddTargets}>{targets.map((target) => <label key={target.path} className={styles.discoverAgent}><Checkbox checked={paths.includes(target.path)} onCheckedChange={(checked) => setPaths((current) => checked ? [...new Set([...current, target.path])] : current.filter((path) => path !== target.path))} /><AgentIcon name={target.label} className="size-4" /><span>{target.label}</span></label>)}</div><p className={styles.installDialogHint}>{t('mcp.configFilesAreBackedUpNpxUvxAreNotInstalledAndTheMcpServerIsNotStarted')}</p>{error && <p className={styles.planError} role="alert">{error}</p>}</div>
        <DialogFooter className="border-t px-6 py-2.5"><Button size="sm" variant="outline" disabled={busy} onClick={() => setSelected(undefined)}>{t('common.cancel')}</Button><Button size="sm" disabled={busy || paths.length === 0 || requiredEnvMissing} onClick={() => void install()}>{busy ? t('common.adding') : t('mcp.add')}</Button></DialogFooter>
      </DialogContent>
    </Dialog>}
  </Sheet>
}
