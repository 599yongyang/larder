import { useEffect, useMemo, useState } from 'react'
import { Command as CommandIcon, Plus, Terminal } from 'lucide-react'
import { AgentIcon } from '../../components/AgentIcon'
import { AssetSearch } from '../../components/AssetSearch'
import { CommandDetailSheet, commandScopeLabel } from '../../components/CommandDetailSheet'
import { CommandEditorDialog } from '../../components/CommandEditorDialog'
import { backend } from '../../backend'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { t, tValue } from '../../i18n'
import type { Command, CommandTarget } from '../../types'
import styles from '../../App.module.css'

function CommandsView({ commands, onRefresh }: { commands: Command[]; onRefresh: () => Promise<void> }) {
  const [query, setQuery] = useState('')
  const [agent, setAgent] = useState('all')
  const [scope, setScope] = useState('all')
  const [selectedId, setSelectedId] = useState<string>()
  const [targets, setTargets] = useState<CommandTarget[]>([])
  const [targetError, setTargetError] = useState('')
  const [editing, setEditing] = useState<Command | true>()
  const [converting, setConverting] = useState<Command>()
  const agents = useMemo(() => Array.from(new Set(commands.map((command) => command.agent))).sort(), [commands])
  useEffect(() => { let active = true; backend.commandTargets().then((next) => { if (active) setTargets(next) }).catch(() => { if (active) setTargetError(t('commands.targetsUnavailable')) }); return () => { active = false } }, [])
  const queryText = query.trim().toLowerCase()
  const filtered = commands.filter((command) => (agent === 'all' || command.agent === agent) && (scope === 'all' || command.scope === scope) && (!queryText || [command.name, command.description ?? '', command.agent, command.project ?? '', command.path].some((value) => value.toLowerCase().includes(queryText))))
  const selected = commands.find((command) => command.id === selectedId)

  return <section className={styles.ledgerPanel}>
    <div className={styles.metrics}>
      <div className={styles.metric}><span>{t('commands.total')}</span><strong>{commands.length}</strong><small>{t('commands.totalHint')}</small></div>
      <div className={styles.metric}><span>{t('common.global')}</span><strong>{commands.filter((command) => command.scope === 'global').length}</strong><small>{t('commands.globalHint')}</small></div>
      <div className={styles.metric}><span>{t('common.projects')}</span><strong>{commands.filter((command) => command.scope === 'project').length}</strong><small>{t('commands.projectHint')}</small></div>
      <div className={styles.metric}><span>{t('commands.agents')}</span><strong>{agents.length}</strong><small>{t('commands.agentsHint')}</small></div>
    </div>
    <div className={styles.filterBar}>
      <div className={styles.filterLeft}>
        <AssetSearch value={query} onChange={setQuery} placeholder={t('commands.search')} label={t('commands.search')} />
        <Select value={agent} onValueChange={setAgent}><SelectTrigger size="sm" className="w-[140px]"><SelectValue /></SelectTrigger><SelectContent><SelectItem value="all">{t('common.allAgents')}</SelectItem>{agents.map((item) => <SelectItem key={item} value={item}>{item}</SelectItem>)}</SelectContent></Select>
        <Select value={scope} onValueChange={setScope}><SelectTrigger size="sm" className="w-[110px]"><SelectValue /></SelectTrigger><SelectContent><SelectItem value="all">{t('commands.allScopes')}</SelectItem><SelectItem value="global">{t('common.global')}</SelectItem><SelectItem value="project">{t('common.projects')}</SelectItem></SelectContent></Select>
      </div>
      <div className={styles.filterRight}>{targetError && <span className={styles.commandTargetError} role="status">{targetError}</span>}<span className={styles.resultCount}>{t('commands.count', { n: filtered.length })}</span><Button variant="outline" size="sm" disabled={targets.length === 0} title={targetError || undefined} onClick={() => setEditing(true)}><Plus size={14} />{t('commands.newCommand')}</Button></div>
    </div>
    <div className={`${styles.tableWrap} ${styles.commandsTable}`}><table>
      <thead><tr><th>{t('common.command')}</th><th>{t('common.agent')}</th><th>{t('common.scope')}</th><th>{t('commands.description')}</th><th>{t('common.path')}</th></tr></thead>
      <tbody>{filtered.length === 0 ? <tr><td colSpan={5} className={styles.empty}>{commands.length === 0 ? <div className={styles.emptyAction}><Terminal size={18} aria-hidden="true" /><span>{t('commands.noCommands')}</span><small>{t('commands.emptyHint')}</small><Button variant="outline" size="sm" disabled={targets.length === 0} title={targetError || undefined} onClick={() => setEditing(true)}><Plus size={14} />{t('commands.newCommand')}</Button></div> : t('commands.noMatches')}</td></tr> : filtered.map((command) => <tr key={command.id} className={`${selected?.id === command.id ? styles.selectedRow : ''} ${styles.clickableRow}`} tabIndex={0} onKeyDown={(event) => { if (event.key === 'Enter' || event.key === ' ') { event.preventDefault(); setSelectedId(command.id) } }} onClick={() => setSelectedId(command.id)}>
        <td><div className={styles.commandName}><CommandIcon size={15} aria-hidden="true" /><strong>/{command.name}</strong>{command.format === 'toml' && <Badge variant="outline">TOML</Badge>}</div></td>
        <td><div className={styles.cliName}><AgentIcon name={command.agent} className="size-4" /><span>{tValue(command.agent)}</span></div></td>
        <td><Badge variant={command.scope === 'global' ? 'outline' : 'info'}>{commandScopeLabel(command.scope)}</Badge></td>
        <td><span className={styles.commandDescription}>{command.description || t('commands.noDescription')}</span></td>
        <td><code>{command.path}</code></td>
      </tr>)}</tbody>
    </table></div>
    {selected && <CommandDetailSheet command={selected} onClose={() => setSelectedId(undefined)} onEdit={() => setEditing(selected)} onConvert={() => setConverting(selected)} />}
    {editing && <CommandEditorDialog command={editing === true ? undefined : editing} targets={targets} onClose={() => setEditing(undefined)} onSaved={onRefresh} />}
    {converting && <CommandEditorDialog convertFrom={converting} targets={targets} onClose={() => setConverting(undefined)} onSaved={onRefresh} />}
  </section>
}

export { CommandsView }
