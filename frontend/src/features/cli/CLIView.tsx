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
  const ownerItems = [{ label: '全部来源', value: 'all' }, ...owners.map((item) => ({ label: item, value: item }))]
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

  const copyText = async (value: string, label: string) => { try { await navigator.clipboard.writeText(value); toastSuccess(`已复制${label}`) } catch (error) { toastError(error, `复制${label}失败`) } }
  const revealCLI = async (cli: CLIInfo) => { try { await backend.revealCLI(cli.id) } catch (error) { toastError(error, '打开位置失败') } }
  const canUninstall = (cli: CLIInfo) => ['Homebrew', 'npm', 'winget', 'Bun', 'Cargo', 'Conda', 'pipx', 'uv'].includes(cli.owner) && Boolean(cli.ownerDetail)
  const openUninstall = async (cli: CLIInfo) => {
    setPlanError('')
    try { setPlan(await backend.planCLI(cli.id, true, '')) } catch (cause) { setPlanError(cause instanceof Error ? cause.message : '生成卸载计划失败') }
  }

  const updateForSelected = selected ? updateFor(selected) : undefined
  const commandRow = (cli: CLIInfo, nested = false) => {
    const update = updateFor(cli)
    const canUpdate = update?.state === 'available'
    return <tr key={cli.id} className={`${selected?.id === cli.id ? styles.selectedRow : ''} ${styles.clickableRow}`} onClick={() => setSelectedId(cli.id)}>
      <td className={nested ? styles.cliNestedCell : undefined}><div className={styles.cliName}><CLIIcon owner={cli.owner} className="size-4" /><strong>{cli.name}</strong>{canUpdate && <Button variant="outline" size="xs" className={styles.cliUpdateButton} onClick={(event) => { event.stopPropagation(); setUpdateTarget(cli); setUpdateLabel(cli.name) }}>可更新 {update.latestVersion}</Button>}</div></td>
      <td><code>{cli.version || '—'}</code></td>
      <td><code>{shortPath(cli.path)}</code></td>
      <td><Badge variant={cli.status === 'broken' ? 'error' : cli.duplicateCount > 0 ? 'warning' : 'success'}>{cli.status === 'broken' ? '链接失效' : cli.duplicateCount > 0 ? `重复 ${cli.duplicateCount + 1} 处` : '正常'}</Badge></td>
      <td><div className={styles.quickActions}>{canUninstall(cli) && <Button variant="ghost" size="xs" className="text-destructive" title="卸载 CLI" aria-label="卸载 CLI" onClick={(event) => { event.stopPropagation(); void openUninstall(cli) }}><Trash2 size={14} /></Button>}</div></td>
    </tr>
  }

  return <section className={styles.fullPanel}>
    <div className={styles.metrics}>
      <div className={styles.metric}><span>命令</span><strong>{clis.length}</strong><small>PATH 去重后</small></div>
      <div className={styles.metric}><span>安装包</span><strong>{groups.length}</strong><small>当前筛选后</small></div>
      <div className={styles.metric}><span>重复安装</span><strong>{duplicates}</strong><small>同一命令多个路径</small></div>
      <div className={styles.metric}><span>可更新</span><strong data-tone={availableCount ? 'warn' : undefined}>{availableCount}</strong><small>检查结果</small></div>
    </div>
    <div className={styles.filterBar}>
      <div className={styles.filterLeft}>
        <AssetSearch value={query} onChange={setQuery} placeholder="搜索命令、来源、路径" label="搜索 CLI" />
        <Select value={owner} onValueChange={setOwner}>
          <SelectTrigger size="sm" className="w-[120px]"><SelectValue /></SelectTrigger>
          <SelectContent>{ownerItems.map((item) => <SelectItem key={item.value} value={item.value}><span className={styles.ownerOption}>{item.value !== 'all' && <CLIIcon owner={item.value} className="size-4" />}{item.label}</span></SelectItem>)}</SelectContent>
        </Select>
        <Select value={statusFilter} onValueChange={(value) => setStatusFilter(value as typeof statusFilter)}><SelectTrigger size="sm" className="w-[120px]"><SelectValue /></SelectTrigger><SelectContent><SelectItem value="all">全部状态</SelectItem><SelectItem value="updates">可更新</SelectItem><SelectItem value="duplicates">有重复</SelectItem><SelectItem value="broken">链接失效</SelectItem></SelectContent></Select>
      </div>
      <div className={styles.filterRight}>
        <div className={styles.segmentedControl} role="group" aria-label="CLI 展示方式">
          <Button variant={mode === 'packages' ? 'secondary' : 'ghost'} size="sm" onClick={() => setMode('packages')}>安装包</Button>
          <Button variant={mode === 'commands' ? 'secondary' : 'ghost'} size="sm" onClick={() => setMode('commands')}>命令</Button>
        </div>
        <Select value={sort} onValueChange={(value) => setSort(value as typeof sort)}><SelectTrigger size="sm" className="w-28"><SelectValue /></SelectTrigger><SelectContent><SelectItem value="name">按名称</SelectItem><SelectItem value="duplicates">重复优先</SelectItem><SelectItem value="updates">更新优先</SelectItem></SelectContent></Select>
        <Button variant="outline" size="sm" onClick={onCheckUpdates} disabled={updatesLoading}><RefreshCw size={14} />{updatesLoading ? '检查中…' : '检查更新'}</Button>
      </div>
    </div>
    {mode === 'packages' ? <div className={styles.tableWrap}><table>
      <thead><tr><th>安装包</th><th>命令</th><th>版本</th><th>路径</th><th>状态</th><th>操作</th></tr></thead>
      <tbody>{groups.length === 0 ? <tr><td colSpan={5} className={styles.empty}>{clis.length === 0 ? <div className={styles.emptyAction}><span>还没有扫描到 CLI。</span><Button size="sm" onClick={() => void onRefresh()}><RefreshCw size={14} />重新扫描</Button></div> : '没有匹配当前筛选的 CLI。'}</td></tr> : visibleGroups.map((group) => {
        const summary = groupSummary(group, updateFor)
        return <tr key={group.key} className={`${selectedPackage === group.key ? styles.selectedRow : ''} ${styles.clickableRow}`} onClick={() => setSelectedPackage(group.key)}>
          <td><div className={styles.cliName}><CLIIcon owner={group.owner} className="size-4" /><strong>{group.label}</strong><ChevronRight size={13} className="text-muted-foreground" /></div></td>
          <td>{group.items.length}</td>
          <td><code title={summary.versions.join(', ')}>{summary.versions.length === 0 ? '—' : summary.versions.length === 1 ? summary.versions[0] : `${summary.versions.length} 个版本`}</code></td>
          <td><code>{shortPath(summary.path)}</code></td>
          <td><div className={styles.cliGroupStatus}>{summary.brokenCount > 0 && <Badge variant="error">{summary.brokenCount} 失效</Badge>}{summary.duplicateTotal > 0 && <Badge variant="warning">{summary.duplicateTotal} 处重复</Badge>}{summary.updatable > 0 ? <Badge variant="warning">{summary.updatable} 可更新</Badge> : summary.brokenCount === 0 && summary.duplicateTotal === 0 ? <Badge variant="success">正常</Badge> : null}</div></td>
          <td><div className={styles.quickActions}>{(() => { const target = group.items.find((item) => canUninstall(item)); return target ? <Button variant="ghost" size="xs" className="text-destructive" title="卸载安装包" aria-label="卸载安装包" onClick={(event) => { event.stopPropagation(); void openUninstall(target) }}><Trash2 size={14} /></Button> : null })()}</div></td>
        </tr>
      })}</tbody>
    </table>{groups.length > limit && <div className={styles.loadMore}><Button variant="outline" size="xs" onClick={() => setLimit((value) => value + 120)}>显示更多（剩余 {groups.length - limit} 个安装包）</Button></div>}</div> : <div className={styles.tableWrap}><table>
      <thead><tr><th>命令</th><th>版本</th><th>路径</th><th>状态</th><th>操作</th></tr></thead>
      <tbody>{filtered.length === 0 ? <tr><td colSpan={5} className={styles.empty}>{clis.length === 0 ? <div className={styles.emptyAction}><span>还没有扫描到 CLI。</span><Button size="sm" onClick={() => void onRefresh()}><RefreshCw size={14} />重新扫描</Button></div> : '没有匹配当前筛选的 CLI。'}</td></tr> : visibleCommands.map((cli) => commandRow(cli))}</tbody>
    </table>{filtered.length > limit && <div className={styles.loadMore}><Button variant="outline" size="xs" onClick={() => setLimit((value) => value + 120)}>显示更多（剩余 {filtered.length - limit} 条）</Button></div>}</div>}

    {selectedGroup && <Sheet open onOpenChange={(open) => { if (!open) setSelectedPackage(undefined) }}>
      <SheetContent side="right" className="w-[440px] sm:max-w-[440px]">
        <SheetHeader className="gap-0 border-b px-5 py-4"><div className="flex items-start gap-3"><CLIIcon owner={selectedGroup.owner} className="size-9" /><div className="min-w-0 flex-1"><SheetTitle className="truncate text-[15px]">{selectedGroup.label}</SheetTitle><SheetDescription className="mt-1 text-[11px]">{selectedGroup.owner} · {selectedGroup.items.length} 个命令</SheetDescription></div></div></SheetHeader>
        <div className="min-h-0 flex-1 overflow-y-auto p-0">
          <section className={styles.detailSection}><h3>概况</h3><dl className={styles.definition}><dt>命令数</dt><dd>{selectedGroup.items.length}</dd><dt>版本</dt><dd>{groupSummary(selectedGroup, updateFor).versions.join(', ') || '—'}</dd><dt>公共路径</dt><dd><code>{groupSummary(selectedGroup, updateFor).path || '—'}</code></dd><dt>重复</dt><dd>{groupSummary(selectedGroup, updateFor).duplicateTotal > 0 ? `${groupSummary(selectedGroup, updateFor).duplicateTotal} 处` : '—'}</dd></dl>{selectedGroupUpdate?.update && <div className={styles.planAction}><Button size="xs" onClick={() => { setUpdateTarget(selectedGroupUpdate.cli); setUpdateLabel(selectedGroup.label) }}>更新安装包</Button><small>{selectedGroupUpdate.update.currentVersion || '当前'} → {selectedGroupUpdate.update.latestVersion}</small></div>}</section>
          <section className={styles.detailSection}><h3>命令列表</h3><div className={styles.cliPathList}>{selectedGroup.items.map((cli) => {
            const update = updateFor(cli)
            return <div key={cli.id} className={styles.cliPackageCommand} onClick={() => { setSelectedPackage(undefined); setSelectedId(cli.id) }}>
              <span>{cli.name}</span><code>{cli.version || '—'}</code><Badge variant={cli.status === 'broken' ? 'error' : cli.duplicateCount > 0 ? 'warning' : 'success'}>{cli.status === 'broken' ? '失效' : cli.duplicateCount > 0 ? '重复' : '正常'}</Badge>
              {update?.state === 'available' && <Button variant="outline" size="xs" className={styles.cliUpdateButton} onClick={(event) => { event.stopPropagation(); setUpdateTarget(cli); setUpdateLabel(cli.name) }}>可更新 {update.latestVersion}</Button>}
            </div>
          })}</div></section>
          {(() => { const related = dependents(selectedGroup.items.map((item) => item.name)); return <section className={styles.detailSection}><h3>被依赖</h3><RelationList skills={related.skills} mcps={related.mcps} onSkill={onOpenSkill} onMCP={onOpenMCP} /></section> })()}
        </div>
      </SheetContent>
    </Sheet>}

    {selected && <Sheet open onOpenChange={(open) => { if (!open) setSelectedId(undefined) }}>
      <SheetContent side="right" className="w-[440px] sm:max-w-[440px]">
        <SheetHeader className="gap-0 border-b px-5 py-4">
          <div className="flex items-start gap-3">
            <div className="min-w-0 flex-1"><SheetTitle className="truncate text-[15px]">{selected.name}</SheetTitle><SheetDescription className="mt-1 flex flex-wrap items-center gap-x-1.5 gap-y-0.5 text-[11px]"><span>{selected.owner}</span><span>·</span><span>{selected.ownerDetail || '未知包'}</span><span>·</span><span>{selected.version || '版本未知'}</span></SheetDescription></div>
            <Badge variant={selected.status === 'broken' ? 'error' : selected.duplicateCount > 0 ? 'warning' : 'success'}>{selected.status === 'broken' ? '链接失效' : selected.duplicateCount > 0 ? '重复' : '正常'}</Badge>
          </div>
        </SheetHeader>
        <div className="min-h-0 flex-1 overflow-y-auto p-0">
          <section className={styles.detailSection}><h3>来源</h3><dl className={styles.definition}><dt>路径</dt><dd><code>{selected.path}</code></dd><dt>Owner</dt><dd>{selected.owner}</dd><dt>包</dt><dd>{selected.ownerDetail || '—'}</dd><dt>版本</dt><dd><code>{selected.version || '未解析'}</code></dd><dt>状态</dt><dd><Badge variant={selected.status === 'broken' ? 'error' : selected.duplicateCount > 0 ? 'warning' : 'success'}>{selected.status === 'broken' ? '链接失效' : selected.duplicateCount > 0 ? '重复安装' : '正常'}</Badge></dd></dl>
            <div className={styles.detailPathActions}><Button size="xs" variant="outline" onClick={() => void copyText(selected.name, '命令')}>复制命令</Button><Button size="xs" variant="outline" onClick={() => void copyText(selected.path, '路径')}>复制路径</Button><Button size="xs" variant="ghost" onClick={() => void revealCLI(selected)}>在 Finder 中显示</Button></div>
            {updateForSelected?.state === 'available' && <div className={styles.planAction}><Button size="xs" onClick={() => setUpdateTarget(selected)}>更新 CLI</Button><small>当前 {updateForSelected.currentVersion || selected.version || '—'} → {updateForSelected.latestVersion}</small></div>}
            {planError && <p className={styles.planError} role="alert">{planError}</p>}
          </section>
          {canUninstall(selected) && <section className={styles.detailSection}><h3>管理</h3><div className={styles.destructiveAction}><Button variant="outline" size="xs" onClick={() => void openUninstall(selected)}>卸载 CLI</Button><small>会调用 {selected.owner} 卸载 {selected.ownerDetail || selected.name}，执行前会生成快照。</small></div></section>}
          {(() => { const related = dependents([selected.name]); return <section className={styles.detailSection}><h3>被依赖</h3><RelationList skills={related.skills} mcps={related.mcps} onSkill={onOpenSkill} onMCP={onOpenMCP} /></section> })()}
          <section className={styles.detailSection}><h3>PATH 命中</h3><div className={styles.cliPathList}>{(selected.paths ?? []).map((item) => <div key={item.path} className={styles.cliPathRow}><code>{item.path}</code><span>{item.effective ? <Badge variant="success">生效</Badge> : item.broken ? <Badge variant="error">失效</Badge> : <Badge variant="outline">影子副本</Badge>}</span></div>)}{!selected.paths?.length && <p>没有路径详情。</p>}</div></section>
        </div>
      </SheetContent>
    </Sheet>}

    {updateTarget && updateFor(updateTarget) && <CLIUpdateDialog cli={updateTarget} update={updateFor(updateTarget)!} label={updateLabel} onClose={() => setUpdateTarget(undefined)} onApplied={() => { setUpdateTarget(undefined); void onRefresh() }} />}
    {plan && <PlanDialog plan={plan} onClose={() => setPlan(undefined)} onApplied={() => { setPlan(undefined); setSelectedId(undefined); void onRefresh() }} />}
  </section>
}

function RelationList({ skills, mcps, onSkill, onMCP }: { skills: Skill[]; mcps: MCPServer[]; onSkill?: (id: string) => void; onMCP?: () => void }) {
  if (skills.length === 0 && mcps.length === 0) return <p className={styles.relationEmpty}>没有被 Skill 或 MCP 引用。</p>
  return <div className={styles.relationList}>{skills.map((skill) => <button key={skill.id} type="button" className={styles.relationRow} onClick={() => onSkill?.(skill.id)}><Badge variant="info">Skill</Badge><span>{skill.name}</span></button>)}{mcps.map((mcp) => <button key={mcp.id} type="button" className={styles.relationRow} onClick={() => onMCP?.()}><Badge variant="outline">MCP</Badge><span>{mcp.name}</span></button>)}</div>
}

export { CLIView }
