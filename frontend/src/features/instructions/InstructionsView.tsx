import { useMemo, useState } from 'react'
import { FileText, Plus } from 'lucide-react'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { AssetSearch } from '../../components/AssetSearch'
import { InstructionDetailSheet, formatInstructionDate, formatInstructionSize, instructionScopeLabel } from '../../components/InstructionDetailSheet'
import { InstructionEditorDialog } from '../../components/InstructionEditorDialog'
import { AgentIcon } from '../../components/AgentIcon'
import styles from '../../App.module.css'
import type { InstructionFile } from '../../types'
import { t, tValue } from '../../i18n'


function InstructionsView({ files, projects, onRefresh }: { files: InstructionFile[]; projects: string[]; onRefresh: () => Promise<void> }) {
  const [query, setQuery] = useState('')
  const [agent, setAgent] = useState('all')
  const [scope, setScope] = useState('all')
  const [selectedId, setSelectedId] = useState<string>()
  const [editing, setEditing] = useState<{ file?: InstructionFile }>()
  const agents = useMemo(() => Array.from(new Set(files.map((file) => file.agent))).sort(), [files])
  const filtered = files.filter((file) => (agent === 'all' || file.agent === agent) && (scope === 'all' || file.scope === scope) && (!query.trim() || `${file.name} ${file.path} ${file.agent} ${file.project ?? ''}`.toLowerCase().includes(query.trim().toLowerCase())))
  const selected = files.find((file) => file.id === selectedId)
  return <section className={styles.ledgerPanel}>
    <div className={styles.metrics}>
      <div className={styles.metric}><span>{t('common.instructionFiles')}</span><strong>{files.length}</strong><small>{t('instructions.globalAndProjectRules')}</small></div>
      <div className={styles.metric}><span>{t('common.global')}</span><strong>{files.filter((file) => file.scope === 'global').length}</strong><small>{t('instructions.userLevelRules')}</small></div>
      <div className={styles.metric}><span>{t('common.projects')}</span><strong>{files.filter((file) => file.scope === 'project').length}</strong><small>{t('instructions.currentWorkspace')}</small></div>
      <div className={styles.metric}><span>{t('common.duplicate')}</span><strong data-tone={files.some((file) => file.duplicate) ? 'warn' : undefined}>{files.filter((file) => file.duplicate).length}</strong><small>{t('instructions.sameContentHash')}</small></div>
    </div>
    <div className={styles.filterBar}>
      <div className={styles.filterLeft}>
        <AssetSearch value={query} onChange={setQuery} placeholder={t('instructions.searchInstructionFiles')} label={t('instructions.searchInstructionFiles')} />
        <Select value={agent} onValueChange={setAgent}><SelectTrigger size="sm" className="w-[130px]"><SelectValue /></SelectTrigger><SelectContent><SelectItem value="all">{t('common.allAgents')}</SelectItem>{agents.map((item) => <SelectItem key={item} value={item}>{item}</SelectItem>)}</SelectContent></Select>
        <Select value={scope} onValueChange={setScope}><SelectTrigger size="sm" className="w-[110px]"><SelectValue /></SelectTrigger><SelectContent><SelectItem value="all">{t('instructions.allScopes')}</SelectItem><SelectItem value="global">{t('common.global')}</SelectItem><SelectItem value="project">{t('common.projects')}</SelectItem></SelectContent></Select>
      </div>
      <div className={styles.filterRight}><span className={styles.resultCount}>{t('instructions.files', { n: filtered.length })}</span><Button variant="outline" size="sm" disabled={projects.length === 0} onClick={() => setEditing({})}><Plus size={14} />{t('instructions.newInstruction')}</Button></div>
    </div>
    <div className={`${styles.tableWrap} ${styles.instructionTable}`}><table>
      <thead><tr><th>{t('common.file')}</th><th>Agent</th><th>{t('common.scope')}</th><th>{t('common.path')}</th><th>{t('common.size')}</th><th>{t('common.modified')}</th></tr></thead>
      <tbody>{filtered.length === 0 ? <tr><td colSpan={6} className={styles.empty}>{files.length === 0 ? <div className={styles.emptyAction}><span>{t('instructions.noAgentInstructionFilesFoundYet')}</span>{projects.length > 0 && <Button size="sm" onClick={() => setEditing({})}><Plus size={14} />{t('instructions.newInstruction')}</Button>}</div> : t('instructions.noInstructionFileMatchesTheCurrentFilters')}</td></tr> : filtered.map((file) => <tr key={file.id} className={`${selected?.id === file.id ? styles.selectedRow : ''} ${styles.clickableRow}`} onClick={() => setSelectedId(file.id)}>
        <td><div className={styles.cliName}><FileText size={15} aria-hidden="true" /><strong>{file.name}</strong>{file.duplicate && <Badge variant="warning">{t('common.duplicate')}</Badge>}</div></td>
        <td><div className={styles.cliName}><AgentIcon name={file.agent} className="size-4" /><span>{tValue(file.agent)}</span></div></td>
        <td><Badge variant={file.scope === 'global' ? 'outline' : 'info'}>{instructionScopeLabel(file.scope)}</Badge></td>
        <td><code>{file.path}</code></td>
        <td>{formatInstructionSize(file.size)}</td>
        <td>{formatInstructionDate(file.modifiedAt)}</td>
      </tr>)}</tbody>
    </table></div>
    {selected && <InstructionDetailSheet file={selected} onClose={() => setSelectedId(undefined)} onEdit={() => setEditing({ file: selected })} />}
    {editing && <InstructionEditorDialog file={editing.file} projects={projects} project={selected?.project ? projects.find((item) => item.endsWith('/' + selected.project)) : undefined} onClose={() => setEditing(undefined)} onSaved={() => { setSelectedId(undefined); void onRefresh() }} />}
  </section>
}

export { InstructionsView }
