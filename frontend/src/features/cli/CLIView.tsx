import { useEffect, useState } from 'react'
import { ChevronRight, RefreshCw, Trash2 } from 'lucide-react'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Sheet, SheetContent, SheetDescription, SheetHeader, SheetTitle } from '@/components/ui/sheet'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { CLIIcon } from '../../components/CLIIcon'
import { CLIUpdateDialog } from '../../components/CLIUpdateDialog'
import { PlanDialog } from '../../components/PlanDialog'
import { AssetSearch } from '../../components/AssetSearch'
import { backend } from '../../backend'
import styles from '../../App.module.css'
import { t, tValue } from '../../i18n'
import { backendErrorMessage } from '../../lib/backendError'
import { shortPath } from '../../lib/format'
import { toastError, toastSuccess } from '../../lib/toast'
import type { CLIInfo, MCPServer, Plan, Skill, UpdateItem, UpdateReport } from '../../types'
import { commandBasename } from '../../lib/relations'

function commonDir(paths: string[]) {
  const parts = paths.map((path) => path.split('/').slice(0, -1))
  if (parts.length === 0) return ''
  const common = [...parts[0]]
  for (const part of parts.slice(1)) {
    let index = 0
    while (index < common.length && index < part.length && common[index] === part[index]) index += 1
    common.length = index
  }
  return common.join('/') || '/'
}

type PackageGroup = { key: string; owner: string; label: string; items: CLIInfo[] }

function groupSummary(group: PackageGroup, updateFor: (cli: CLIInfo) => UpdateItem | undefined) {
  const versions = Array.from(new Set(group.items.map((item) => item.version).filter((value): value is string => Boolean(value))))
  const duplicateTotal = group.items.reduce((sum, item) => sum + item.duplicateCount + (item.duplicateCount > 0 ? 1 : 0), 0)
  const brokenCount = group.items.filter((item) => item.status === 'broken').length
  const updatable = group.items.filter((item) => updateFor(item)?.state === 'available').length
  return { versions, duplicateTotal, brokenCount, updatable, path: commonDir(group.items.map((item) => item.path)) }
}

function CLIView({ clis, skills, mcps, onRefresh, updates, updatesLoading, onCheckUpdates, focusCLI, onFocusConsumed, onOpenSkill, onOpenMCP }: { clis: CLIInfo[]; skills: Skill[]; mcps: MCPServer[]; onRefresh: () => Promise<void>; updates?: UpdateReport; updatesLoading: boolean; onCheckUpdates: () => void; focusCLI?: string; onFocusConsumed?: () => void; onOpenSkill?: (id: string) => void; onOpenMCP?: () => void }) {
  const [owner, setOwner] = useState('all')
  const [selectedId, setSelectedId] = useState<string>()
  const [selectedPackage, setSelectedPackage] = useState<string>()
  const [plan, setPlan] = useState<Plan>()
  const [planError, setPlanError] = useState('')
  const [query, setQuery] = useState('')
  const [statusFilter, setStatusFilter] = useState<'all' | 'updates' | 'duplicates' | 'broken'>('all')
  const [sort, setSort] = useState<'name' | 'duplicates' | 'updates'>('name')
  const [updateTarget, setUpdateTarget] = useState<CLIInfo>()
  const [updateLabel, setUpdateLabel] = useState('')
  const [mode, setMode] = useState<'packages' | 'commands'>('packages')
  const [limit, setLimit] = useState(120)

  const owners = Array.from(new Set(clis.map((item) => item.owner))).sort()
  const ownerItems = [{ label: t('common.allSources'), value: 'all' }, ...owners.map((item) => ({ label: item, value: item }))]
  const updateItems = (updates?.items ?? []).filter((item) => item.kind === 'cli')
  const updateFor = (cli: CLIInfo): UpdateItem | undefined => updateItems.find((item) => item.cliId === cli.id || item.name === cli.name)
  const availableCount = updateItems.filter((item) => item.state === 'available').length
  const duplicates = clis.filter((item) => item.duplicateCount > 0).length
  const sortCommands = (a: CLIInfo, b: CLIInfo) => sort === 'duplicates' ? b.duplicateCount - a.duplicateCount || a.name.localeCompare(b.name) : sort === 'updates' ? Number(updateFor(b)?.state === 'available') - Number(updateFor(a)?.state === 'available') || a.name.localeCompare(b.name) : a.name.localeCompare(b.name)
  const filtered = clis.filter((item) => (owner === 'all' || item.owner === owner) && (statusFilter === 'all' || (statusFilter === 'updates' ? updateFor(item)?.state === 'available' : statusFilter === 'duplicates' ? item.duplicateCount > 0 : item.status === 'broken')) && (!query.trim() || `${item.name} ${item.owner} ${item.ownerDetail ?? ''} ${item.path}`.toLowerCase().includes(query.trim().toLowerCase()))).sort(sortCommands)
  const groups = Array.from(filtered.reduce((map, item) => {
    const key = `${item.owner}:${item.ownerDetail || item.name}`
    if (!map.has(key)) map.set(key, { key, owner: item.owner, label: item.ownerDetail || item.name, items: [] })
    map.get(key)!.items.push(item)
    return map
  }, new Map<string, PackageGroup>())).map(([, group]) => { group.items.sort(sortCommands); return group }).sort((a, b) => sort === 'duplicates' ? Math.max(...b.items.map((item) => item.duplicateCount)) - Math.max(...a.items.map((item) => item.duplicateCount)) || a.label.localeCompare(b.label) : sort === 'updates' ? b.items.filter((item) => updateFor(item)?.state === 'available').length - a.items.filter((item) => updateFor(item)?.state === 'available').length || a.label.localeCompare(b.label) : a.label.localeCompare(b.label))
  const selectedGroup = groups.find((group) => group.key === selectedPackage)
  const dependents = (names: string[]) => ({ skills: skills.filter((skill) => (skill.bins ?? []).some((bin) => names.includes(bin))), mcps: mcps.filter((mcp) => names.includes(commandBasename(mcp.command))) })
  useEffect(() => { if (!focusCLI) return; setSelectedId(focusCLI); setSelectedPackage(undefined); onFocusConsumed?.() }, [focusCLI, onFocusConsumed])
  useEffect(() => { setLimit(120) }, [query, owner, statusFilter, sort, mode])
  const visibleGroups = groups.slice(0, limit)
  const visibleCommands = filtered.slice(0, limit)
  const selected = clis.find((item) => item.id === selectedId)
  const selectedGroupUpdate = selectedGroup ? selectedGroup.items.map((item) => ({ cli: item, update: updateFor(item) })).find((entry) => entry.update?.state === 'available') : undefined

  const copyText = async (value: string, label: string) => { try { await navigator.clipboard.writeText(value); toastSuccess(t('common.copied', { label })) } catch (error) { toastError(error, t('common.failedToCopy', { label })) } }
  const revealCLI = async (cli: CLIInfo) => { try { await backend.revealCLI(cli.id) } catch (error) { toastError(error, t('common.failedToOpenLocation')) } }
  const canUninstall = (cli: CLIInfo) => ['Homebrew', 'npm', 'winget', 'Bun', 'Cargo', 'Conda', 'pipx', 'uv'].includes(cli.owner) && Boolean(cli.ownerDetail)
  const openUninstall = async (cli: CLIInfo) => {
    setPlanError('')
    try { setPlan(await backend.planCLI(cli.id, true, '')) } catch (cause) { setPlanError(backendErrorMessage(cause, t('cli.failedToCreateUninstallPlan'))) }
  }

  const updateForSelected = selected ? updateFor(selected) : undefined
  const commandRow = (cli: CLIInfo, nested = false) => {
    const update = updateFor(cli)
    const canUpdate = update?.state === 'available'
    return <tr key={cli.id} className={`${selected?.id === cli.id ? styles.selectedRow : ''} ${styles.clickableRow}`} tabIndex={0} onKeyDown={(event) => { if (event.key === 'Enter' || event.key === ' ') { event.preventDefault(); setSelectedId(cli.id) } }} onClick={() => setSelectedId(cli.id)}>
      <td className={nested ? styles.cliNestedCell : undefined}><div className={styles.cliName}><CLIIcon owner={cli.owner} className="size-4" /><strong>{cli.name}</strong>{canUpdate && <Button variant="outline" size="xs" className={styles.cliUpdateButton} onClick={(event) => { event.stopPropagation(); setUpdateTarget(cli); setUpdateLabel(cli.name) }}>{t('common.updateAvailable')} {update.latestVersion}</Button>}</div></td>
      <td><code>{cli.version || '—'}</code></td>
      <td><code>{shortPath(cli.path)}</code></td>
      <td><Badge variant={cli.status === 'broken' ? 'error' : cli.duplicateCount > 0 ? 'warning' : 'success'}>{cli.status === 'broken' ? t('common.brokenLink') : cli.duplicateCount > 0 ? t('cli.duplicateCount', { n: cli.duplicateCount + 1 }) : t('common.healthy')}</Badge></td>
      <td><div className={styles.quickActions}>{canUninstall(cli) && <Button variant="ghost" size="xs" className="text-destructive" title={t('common.uninstallCliAction')} aria-label={t('common.uninstallCliAction')} onClick={(event) => { event.stopPropagation(); void openUninstall(cli) }}><Trash2 size={14} /></Button>}</div></td>
    </tr>
  }

  return <section className={styles.fullPanel}>
    <div className={styles.metrics}>
      <div className={styles.metric}><span>{t('common.commandLabel')}</span><strong>{clis.length}</strong><small>{t('cli.deduplicatedByPath')}</small></div>
      <div className={styles.metric}><span>{t('cli.packages')}</span><strong>{groups.length}</strong><small>{t('cli.afterCurrentFilters')}</small></div>
      <div className={styles.metric}><span>{t('cli.duplicateInstalls')}</span><strong>{duplicates}</strong><small>{t('cli.oneCommandOnMultiplePaths')}</small></div>
      <div className={styles.metric}><span>{t('common.updateAvailable')}</span><strong data-tone={availableCount ? 'warn' : undefined}>{availableCount}</strong><small>{t('cli.checkResults')}</small></div>
    </div>
    <div className={styles.filterBar}>
      <div className={styles.filterLeft}>
        <AssetSearch value={query} onChange={setQuery} placeholder={t('cli.searchCommandsOwnersOrPaths')} label={t('cli.searchClis')} />
        <Select value={owner} onValueChange={setOwner}>
          <SelectTrigger size="sm" className="w-[120px]"><SelectValue /></SelectTrigger>
          <SelectContent>{ownerItems.map((item) => <SelectItem key={item.value} value={item.value}><span className={styles.ownerOption}>{item.value !== 'all' && <CLIIcon owner={item.value} className="size-4" />}{item.label}</span></SelectItem>)}</SelectContent>
        </Select>
        <Select value={statusFilter} onValueChange={(value) => setStatusFilter(value as typeof statusFilter)}><SelectTrigger size="sm" className="w-[120px]"><SelectValue /></SelectTrigger><SelectContent><SelectItem value="all">{t('common.allStates')}</SelectItem><SelectItem value="updates">{t('common.updateAvailable')}</SelectItem><SelectItem value="duplicates">{t('cli.hasDuplicates')}</SelectItem><SelectItem value="broken">{t('common.brokenLink')}</SelectItem></SelectContent></Select>
      </div>
      <div className={styles.filterRight}>
        <div className={styles.segmentedControl} role="group" aria-label={t('cli.cliDisplayMode')}>
          <Button variant={mode === 'packages' ? 'secondary' : 'ghost'} size="sm" onClick={() => setMode('packages')}>{t('cli.packages')}</Button>
          <Button variant={mode === 'commands' ? 'secondary' : 'ghost'} size="sm" onClick={() => setMode('commands')}>{t('common.commandLabel')}</Button>
        </div>
        <Select value={sort} onValueChange={(value) => setSort(value as typeof sort)}><SelectTrigger size="sm" className="w-28"><SelectValue /></SelectTrigger><SelectContent><SelectItem value="name">{t('cli.byName')}</SelectItem><SelectItem value="duplicates">{t('cli.duplicatesFirst')}</SelectItem><SelectItem value="updates">{t('cli.updatesFirst')}</SelectItem></SelectContent></Select>
        <Button variant="outline" size="sm" onClick={onCheckUpdates} disabled={updatesLoading}><RefreshCw size={14} />{updatesLoading ? t('cli.checking') : t('common.checkForUpdates')}</Button>
      </div>
    </div>
    {mode === 'packages' ? <div className={styles.tableWrap}><table>
      <thead><tr><th>{t('cli.packages')}</th><th>{t('common.commandLabel')}</th><th>{t('common.version')}</th><th>{t('common.path')}</th><th>{t('common.status')}</th><th>{t('common.actions')}</th></tr></thead>
      <tbody>{groups.length === 0 ? <tr><td colSpan={5} className={styles.empty}>{clis.length === 0 ? <div className={styles.emptyAction}><span>{t('cli.noClisFoundYet')}</span><Button size="sm" onClick={() => void onRefresh()}><RefreshCw size={14} />{t('common.rescan')}</Button></div> : t('cli.noCliMatchesTheCurrentFilters')}</td></tr> : visibleGroups.map((group) => {
        const summary = groupSummary(group, updateFor)
        return <tr key={group.key} className={`${selectedPackage === group.key ? styles.selectedRow : ''} ${styles.clickableRow}`} tabIndex={0} onKeyDown={(event) => { if (event.key === 'Enter' || event.key === ' ') { event.preventDefault(); setSelectedPackage(group.key) } }} onClick={() => setSelectedPackage(group.key)}>
          <td><div className={styles.cliName}><CLIIcon owner={group.owner} className="size-4" /><strong>{group.label}</strong><ChevronRight size={13} className="text-muted-foreground" /></div></td>
          <td>{group.items.length}</td>
          <td><code title={summary.versions.join(', ')}>{summary.versions.length === 0 ? '—' : summary.versions.length === 1 ? summary.versions[0] : t('cli.versions', { n: summary.versions.length })}</code></td>
          <td><code>{shortPath(summary.path)}</code></td>
          <td><div className={styles.cliGroupStatus}>{summary.brokenCount > 0 && <Badge variant="error">{t('cli.broken', { n: summary.brokenCount })}</Badge>}{summary.duplicateTotal > 0 && <Badge variant="warning">{t('cli.duplicates', { n: summary.duplicateTotal })}</Badge>}{summary.updatable > 0 ? <Badge variant="warning">{t('cli.updatable', { n: summary.updatable })}</Badge> : summary.brokenCount === 0 && summary.duplicateTotal === 0 ? <Badge variant="success">{t('common.healthy')}</Badge> : null}</div></td>
          <td><div className={styles.quickActions}>{(() => { const target = group.items.find((item) => canUninstall(item)); return target ? <Button variant="ghost" size="xs" className="text-destructive" title={t('cli.uninstallPackage')} aria-label={t('cli.uninstallPackage')} onClick={(event) => { event.stopPropagation(); void openUninstall(target) }}><Trash2 size={14} /></Button> : null })()}</div></td>
        </tr>
      })}</tbody>
    </table>{groups.length > limit && <div className={styles.loadMore}><Button variant="outline" size="xs" onClick={() => setLimit((value) => value + 120)}>{t('cli.showMorePackagesRemaining', { n: groups.length - limit })}</Button></div>}</div> : <div className={styles.tableWrap}><table>
      <thead><tr><th>{t('common.commandLabel')}</th><th>{t('common.version')}</th><th>{t('common.path')}</th><th>{t('common.status')}</th><th>{t('common.actions')}</th></tr></thead>
      <tbody>{filtered.length === 0 ? <tr><td colSpan={5} className={styles.empty}>{clis.length === 0 ? <div className={styles.emptyAction}><span>{t('cli.noClisFoundYet')}</span><Button size="sm" onClick={() => void onRefresh()}><RefreshCw size={14} />{t('common.rescan')}</Button></div> : t('cli.noCliMatchesTheCurrentFilters')}</td></tr> : visibleCommands.map((cli) => commandRow(cli))}</tbody>
    </table>{filtered.length > limit && <div className={styles.loadMore}><Button variant="outline" size="xs" onClick={() => setLimit((value) => value + 120)}>{t('cli.showMoreRemaining', { n: filtered.length - limit })}</Button></div>}</div>}

    {selectedGroup && <Sheet open onOpenChange={(open) => { if (!open) setSelectedPackage(undefined) }}>
      <SheetContent side="right" className="w-[440px] sm:max-w-[440px]">
        <SheetHeader className="gap-0 border-b px-5 py-4"><div className="flex items-start gap-3"><CLIIcon owner={selectedGroup.owner} className="size-9" /><div className="min-w-0 flex-1"><SheetTitle className="truncate text-[15px]">{selectedGroup.label}</SheetTitle><SheetDescription className="mt-1 text-[11px]">{tValue(selectedGroup.owner)} · {t('cli.commands', { n: selectedGroup.items.length })}</SheetDescription></div></div></SheetHeader>
        <div className="min-h-0 flex-1 overflow-y-auto p-0">
          <section className={styles.detailSection}><h3>{t('cli.overview')}</h3><dl className={styles.definition}><dt>{t('cli.commandCount')}</dt><dd>{selectedGroup.items.length}</dd><dt>{t('common.version')}</dt><dd>{groupSummary(selectedGroup, updateFor).versions.join(', ') || '—'}</dd><dt>{t('cli.sharedPath')}</dt><dd><code>{groupSummary(selectedGroup, updateFor).path || '—'}</code></dd><dt>{t('common.duplicate')}</dt><dd>{groupSummary(selectedGroup, updateFor).duplicateTotal > 0 ? `${groupSummary(selectedGroup, updateFor).duplicateTotal} 处` : '—'}</dd></dl>{selectedGroupUpdate?.update && <div className={styles.planAction}><Button size="xs" onClick={() => { setUpdateTarget(selectedGroupUpdate.cli); setUpdateLabel(selectedGroup.label) }}>{t('cli.updatePackage')}</Button><small>{selectedGroupUpdate.update.currentVersion || t('cli.current')} → {selectedGroupUpdate.update.latestVersion}</small></div>}</section>
          <section className={styles.detailSection}><h3>{t('cli.commandList')}</h3><div className={styles.cliPathList}>{selectedGroup.items.map((cli) => {
            const update = updateFor(cli)
            return <div key={cli.id} className={styles.cliPackageCommand} onClick={() => { setSelectedPackage(undefined); setSelectedId(cli.id) }}>
              <span>{cli.name}</span><code>{cli.version || '—'}</code><Badge variant={cli.status === 'broken' ? 'error' : cli.duplicateCount > 0 ? 'warning' : 'success'}>{cli.status === 'broken' ? t('cli.brokenState') : cli.duplicateCount > 0 ? t('common.duplicate') : t('common.healthy')}</Badge>
              {update?.state === 'available' && <Button variant="outline" size="xs" className={styles.cliUpdateButton} onClick={(event) => { event.stopPropagation(); setUpdateTarget(cli); setUpdateLabel(cli.name) }}>{t('common.updateAvailable')} {update.latestVersion}</Button>}
            </div>
          })}</div></section>
          {(() => { const related = dependents(selectedGroup.items.map((item) => item.name)); return <section className={styles.detailSection}><h3>{t('cli.dependedOnBy')}</h3><RelationList skills={related.skills} mcps={related.mcps} onSkill={onOpenSkill} onMCP={onOpenMCP} /></section> })()}
        </div>
      </SheetContent>
    </Sheet>}

    {selected && <Sheet open onOpenChange={(open) => { if (!open) setSelectedId(undefined) }}>
      <SheetContent side="right" className="w-[440px] sm:max-w-[440px]">
        <SheetHeader className="gap-0 border-b px-5 py-4">
          <div className="flex items-start gap-3">
            <div className="min-w-0 flex-1"><SheetTitle className="truncate text-[15px]">{selected.name}</SheetTitle><SheetDescription className="mt-1 flex flex-wrap items-center gap-x-1.5 gap-y-0.5 text-[11px]"><span>{tValue(selected.owner)}</span><span>·</span><span>{selected.ownerDetail || t('cli.unknownPackage')}</span><span>·</span><span>{selected.version || t('cli.unknownVersion')}</span></SheetDescription></div>
            <Badge variant={selected.status === 'broken' ? 'error' : selected.duplicateCount > 0 ? 'warning' : 'success'}>{selected.status === 'broken' ? t('common.brokenLink') : selected.duplicateCount > 0 ? t('common.duplicate') : t('common.healthy')}</Badge>
          </div>
        </SheetHeader>
        <div className="min-h-0 flex-1 overflow-y-auto p-0">
          <section className={styles.detailSection}><h3>{t('common.sourceLabel')}</h3><dl className={styles.definition}><dt>{t('common.path')}</dt><dd><code>{selected.path}</code></dd><dt>{t('common.owner')}</dt><dd>{tValue(selected.owner)}</dd><dt>{t('cli.package')}</dt><dd>{selected.ownerDetail || '—'}</dd><dt>{t('common.version')}</dt><dd><code>{selected.version || t('cli.unresolved')}</code></dd><dt>{t('common.status')}</dt><dd><Badge variant={selected.status === 'broken' ? 'error' : selected.duplicateCount > 0 ? 'warning' : 'success'}>{selected.status === 'broken' ? t('common.brokenLink') : selected.duplicateCount > 0 ? t('cli.duplicateInstalls') : t('common.healthy')}</Badge></dd></dl>
            <div className={styles.detailPathActions}><Button size="xs" variant="outline" onClick={() => void copyText(selected.name, t('common.commandLabel'))}>{t('cli.copyCommand')}</Button><Button size="xs" variant="outline" onClick={() => void copyText(selected.path, t('common.path'))}>{t('common.copyPath')}</Button><Button size="xs" variant="ghost" onClick={() => void revealCLI(selected)}>{t('common.revealInFinder')}</Button></div>
            {updateForSelected?.state === 'available' && <div className={styles.planAction}><Button size="xs" onClick={() => setUpdateTarget(selected)}>{t('cli.updateCli')}</Button><small>{t('cli.current')} {updateForSelected.currentVersion || selected.version || '—'} → {updateForSelected.latestVersion}</small></div>}
            {planError && <p className={styles.planError} role="alert">{planError}</p>}
          </section>
          {canUninstall(selected) && <section className={styles.detailSection}><h3>{t('cli.manage')}</h3><div className={styles.destructiveAction}><Button variant="outline" size="xs" onClick={() => void openUninstall(selected)}>{t('common.uninstallCliAction')}</Button><small>{t('cli.callsToUninstallASnapshotIsCreatedFirst', { owner: tValue(selected.owner), name: selected.ownerDetail || selected.name })}</small></div></section>}
          {(() => { const related = dependents([selected.name]); return <section className={styles.detailSection}><h3>{t('cli.dependedOnBy')}</h3><RelationList skills={related.skills} mcps={related.mcps} onSkill={onOpenSkill} onMCP={onOpenMCP} /></section> })()}
          <section className={styles.detailSection}><h3>{t('cli.pathMatches')}</h3><div className={styles.cliPathList}>{(selected.paths ?? []).map((item) => <div key={item.path} className={styles.cliPathRow}><code>{item.path}</code><span>{item.effective ? <Badge variant="success">{t('cli.active')}</Badge> : item.broken ? <Badge variant="error">{t('cli.brokenState')}</Badge> : <Badge variant="outline">{t('cli.shadowCopy')}</Badge>}</span></div>)}{!selected.paths?.length && <p>{t('cli.noPathDetails')}</p>}</div></section>
        </div>
      </SheetContent>
    </Sheet>}

    {updateTarget && updateFor(updateTarget) && <CLIUpdateDialog cli={updateTarget} update={updateFor(updateTarget)!} label={updateLabel} onClose={() => setUpdateTarget(undefined)} onApplied={() => { setUpdateTarget(undefined); void onRefresh() }} />}
    {plan && <PlanDialog plan={plan} onClose={() => setPlan(undefined)} onApplied={() => { setPlan(undefined); setSelectedId(undefined); void onRefresh() }} />}
  </section>
}

function RelationList({ skills, mcps, onSkill, onMCP }: { skills: Skill[]; mcps: MCPServer[]; onSkill?: (id: string) => void; onMCP?: () => void }) {
  if (skills.length === 0 && mcps.length === 0) return <p className={styles.relationEmpty}>{t('cli.notReferencedByASkillOrMcp')}</p>
  return <div className={styles.relationList}>{skills.map((skill) => <button key={skill.id} type="button" className={styles.relationRow} onClick={() => onSkill?.(skill.id)}><Badge variant="info">Skill</Badge><span>{skill.name}</span></button>)}{mcps.map((mcp) => <button key={mcp.id} type="button" className={styles.relationRow} onClick={() => onMCP?.()}><Badge variant="outline">MCP</Badge><span>{mcp.name}</span></button>)}</div>
}

export { CLIView }
