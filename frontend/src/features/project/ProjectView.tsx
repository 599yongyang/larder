import { useState } from 'react'
import { ChevronRight, Folder } from 'lucide-react'
import { Badge } from '@/components/ui/badge'
import { Sheet, SheetContent, SheetDescription, SheetHeader, SheetTitle } from '@/components/ui/sheet'
import { AgentIcon } from '../../components/AgentIcon'
import { InstructionDetailSheet } from '../../components/InstructionDetailSheet'
import { InstructionEditorDialog } from '../../components/InstructionEditorDialog'
import { MCPIcon } from '../../components/MCPIcon'
import { SkillIcon } from '../../components/SkillIcon'
import styles from '../../App.module.css'
import projectStyles from './ProjectView.module.css'
import type { InstructionFile, MCPServer, Skill } from '../../types'

function Section({ title, count, children }: { title: string; count: number; children: React.ReactNode }) {
  return <section className={projectStyles.section}><div className={projectStyles.sectionHead}><h3>{title}</h3><span>{count}</span></div>{children}</section>
}

function ProjectView({ path, skills, servers, instructions, projects, onRefresh }: { path: string; skills: Skill[]; servers: MCPServer[]; instructions: InstructionFile[]; projects: string[]; onRefresh: () => Promise<void> }) {
  const projectSkills = skills.filter((skill) => skill.isProject && (skill.path === path || skill.path.startsWith(path + '/')))
  const projectMCPs = servers.filter((server) => server.agents.some((agent) => agent.scope === path))
  const projectInstructions = instructions.filter((file) => file.path.startsWith(path + '/'))
  const [selected, setSelected] = useState<{ kind: 'skill'; item: Skill } | { kind: 'mcp'; item: MCPServer } | { kind: 'instruction'; item: InstructionFile }>()
  const [editingInstruction, setEditingInstruction] = useState<InstructionFile>()
  const hasLeft = projectSkills.length > 0
  const hasRight = projectMCPs.length > 0 || projectInstructions.length > 0
  return <section className={styles.ledgerPanel}>
    <header className={projectStyles.hero}>
      <span className={projectStyles.heroIcon}><Folder size={18} aria-hidden="true" /></span>
      <div className={projectStyles.heroText}><span className={projectStyles.kicker}>项目路径</span><code>{path}</code></div>
      <div className={projectStyles.stats}>
        <div className={projectStyles.stat}><strong>{projectSkills.length}</strong><span>Skills</span></div>
        <div className={projectStyles.stat}><strong>{projectMCPs.length}</strong><span>MCP</span></div>
        <div className={projectStyles.stat}><strong>{projectInstructions.length}</strong><span>指令</span></div>
      </div>
    </header>
    {!hasLeft && !hasRight ? <div className={projectStyles.emptyState}><p>这个项目还没有项目级资源。</p><span>在项目中添加 Skills、MCP 配置或指令文件后，这里会汇总显示。</span></div> : <div className={`${projectStyles.columns} ${(!hasLeft || !hasRight) ? projectStyles.singleColumn : ''}`}>
      {hasLeft && <div className={projectStyles.column}>
        <Section title="Skills" count={projectSkills.length}>{projectSkills.map((skill) => <button key={skill.id} type="button" className={projectStyles.row} onClick={() => setSelected({ kind: 'skill', item: skill })}><SkillIcon skill={skill} className="size-4" /><span className={projectStyles.rowMain}><span className={projectStyles.name}>{skill.name}</span><span className={projectStyles.meta}>{skill.source || '本地'}</span></span><ChevronRight size={12} className={projectStyles.rowEnd} /></button>)}</Section>
      </div>}
      {hasRight && <div className={projectStyles.column}>
        {projectMCPs.length > 0 && <Section title="项目 MCP" count={projectMCPs.length}>{projectMCPs.map((server) => <button key={server.id} type="button" className={projectStyles.row} onClick={() => setSelected({ kind: 'mcp', item: server })}><MCPIcon name={server.name} command={server.command} url={server.url} className="size-4" /><span className={projectStyles.rowMain}><span className={projectStyles.name}>{server.name}</span><span className={projectStyles.meta}>{server.transport} · {server.agents.length} 个 Agent</span></span><ChevronRight size={12} className={projectStyles.rowEnd} /></button>)}</Section>}
        {projectInstructions.length > 0 && <Section title="指令文件" count={projectInstructions.length}>{projectInstructions.map((file) => { const relative = file.path.startsWith(path + '/') ? file.path.slice(path.length + 1) : file.path; const detail = relative === file.name ? `${file.agent} · ${file.size < 1024 ? file.size + ' B' : (file.size / 1024).toFixed(1) + ' KB'}` : `${file.agent} · ${relative}`; return <button key={file.id} type="button" className={projectStyles.row} onClick={() => setSelected({ kind: 'instruction', item: file })}><AgentIcon name={file.agent} className="size-4" /><span className={projectStyles.rowMain}><span className={projectStyles.name}>{file.name}</span><span className={projectStyles.meta}>{detail}</span></span><ChevronRight size={12} className={projectStyles.rowEnd} /></button> })}</Section>}
      </div>}
    </div>}
    {selected?.kind === 'instruction' && <InstructionDetailSheet file={selected.item} onClose={() => setSelected(undefined)} onEdit={() => setEditingInstruction(selected.item)} />}
    {editingInstruction && <InstructionEditorDialog file={editingInstruction} projects={projects} onClose={() => setEditingInstruction(undefined)} onSaved={() => { setSelected(undefined); void onRefresh() }} />}
    {selected && selected.kind !== 'instruction' && <ProjectAssetSheet selected={selected} onClose={() => setSelected(undefined)} />}
  </section>
}

function ProjectAssetSheet({ selected, onClose }: { selected: { kind: 'skill'; item: Skill } | { kind: 'mcp'; item: MCPServer }; onClose: () => void }) {
  const item = selected.item
  return <Sheet open onOpenChange={(open) => { if (!open) onClose() }}>
    <SheetContent side="right" className="w-[440px] sm:max-w-[440px]">
      <SheetHeader className="gap-0 border-b px-5 py-4"><div className="flex items-start gap-3">{selected.kind === 'skill' ? <SkillIcon skill={item as Skill} className="size-9" /> : <span className={styles.mcpHero}><MCPIcon name={item.name} command={(item as MCPServer).command} url={(item as MCPServer).url} className="size-5" /></span>}<div className="min-w-0 flex-1"><SheetTitle className="truncate text-[15px]">{item.name}</SheetTitle><SheetDescription className="mt-1 flex flex-wrap items-center gap-x-1.5 gap-y-0.5 text-[11px]"><span>{selected.kind === 'skill' ? '项目 Skill' : (item as MCPServer).transport}</span>{selected.kind === 'mcp' && <><span>·</span><span>{(item as MCPServer).agents.length} 个 Agent</span></>}</SheetDescription></div></div></SheetHeader>
      <div className="min-h-0 flex-1 overflow-y-auto p-0">
        {selected.kind === 'skill' ? <SkillSections skill={item as Skill} /> : <MCPSections mcp={item as MCPServer} />}
      </div>
    </SheetContent>
  </Sheet>
}

function SkillSections({ skill }: { skill: Skill }) {
  return <>
    <section className={styles.detailSection}><h3>来源</h3><dl className={styles.definition}><dt>路径</dt><dd><code className={styles.detailPath}>{skill.path}</code></dd><dt>来源</dt><dd>{skill.source || '本地'}</dd><dt>上下文成本</dt><dd>{skill.descTokens.toLocaleString()} tokens</dd><dt>内容哈希</dt><dd><code>{skill.folderHash?.slice(0, 16) || '—'}</code></dd></dl></section>
    {skill.description && <section className={styles.detailSection}><h3>描述</h3><p className={styles.assetDescription}>{skill.description}</p></section>}
    {skill.bins && skill.bins.length > 0 && <section className={styles.detailSection}><h3>依赖 CLI</h3><div className={styles.mcpKeys}>{skill.bins.map((bin) => <Badge key={bin} variant="outline">{bin}</Badge>)}</div></section>}
  </>
}

function MCPSections({ mcp }: { mcp: MCPServer }) {
  return <>
    <section className={styles.detailSection}><h3>启动方式</h3><dl className={styles.definition}>{mcp.command && <><dt>命令</dt><dd><code>{mcp.command}</code></dd></>}{mcp.args && mcp.args.length > 0 && <><dt>参数</dt><dd><code>{mcp.args.join(' ')}</code></dd></>}{mcp.url && <><dt>URL</dt><dd><code>{mcp.url}</code></dd></>}<dt>传输</dt><dd>{mcp.transport}</dd></dl></section>
    <section className={styles.detailSection}><h3>Agent 配置</h3><div className={styles.mcpAgents}>{mcp.agents.map((agent) => <div key={agent.path} className={styles.mcpAgentRow}><AgentIcon name={agent.label} className="size-4" /><div><strong>{agent.label}</strong><small>{agent.path}</small></div><Badge variant={agent.enabled ? 'success' : 'outline'}>{agent.enabled ? '已启用' : '未启用'}</Badge></div>)}</div></section>
    {(mcp.envKeys?.length || mcp.headerKeys?.length) ? <section className={styles.detailSection}><h3>敏感字段</h3><div className={styles.mcpKeys}>{(mcp.envKeys ?? []).map((key) => <Badge key={key} variant="outline">{key}</Badge>)}{(mcp.headerKeys ?? []).map((key) => <Badge key={key} variant="outline">{key}</Badge>)}</div></section> : null}
  </>
}

export { ProjectView }
