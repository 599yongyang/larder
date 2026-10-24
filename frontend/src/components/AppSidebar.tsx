import { useState } from 'react'
import { ChevronDown, Database, Folder, FolderPlus, History, FileText, LayoutList, Server, Settings as SettingsIcon, Settings2, Terminal } from 'lucide-react'
import { Sidebar, SidebarContent, SidebarFooter, SidebarGroup, SidebarGroupAction, SidebarGroupLabel, SidebarHeader, SidebarMenu, SidebarMenuBadge, SidebarMenuButton, SidebarMenuItem } from '@/components/ui/sidebar'
import { AgentIcon } from './AgentIcon'
import { NavButton } from './NavButton'
import styles from '../App.module.css'
import type { View } from '../types'

type Scope = { kind: 'library'; label: string } | { kind: 'agent'; id: string; label: string } | { kind: 'project'; path: string; label: string } | undefined
type Item = { id?: string; label: string; count?: number; path?: string }

type Props = {
  view: View
  scope?: Scope
  onView: (view: View) => void
  onScope: (scope: Scope) => void
  onManageAgents: () => void
  onManageProjects: () => void
  skillsCount: number
  cliCount: number
  mcpCount: number
  instructionCount: number
  agents: Item[]
  projects: Item[]
  centralCount: number
}

export function AppSidebar({ view, scope, onView, onScope, onManageAgents, onManageProjects, skillsCount, cliCount, mcpCount, instructionCount, agents, projects, centralCount }: Props) {
  const [openGroup, setOpenGroup] = useState<'agents' | 'projects' | null>('agents')
  const toggleGroup = (group: 'agents' | 'projects') => setOpenGroup((current) => current === group ? null : group)
  return <Sidebar collapsible="none">
    <SidebarHeader style={{ paddingTop: 36, paddingBottom: 0 }}>
      <div className="flex h-10 items-center px-2"><span className={styles.brandWordmark}>Larder</span></div>
    </SidebarHeader>
    <SidebarContent className="overflow-hidden" style={{ gap: 4 }}>
      <SidebarGroup className="shrink-0" style={{ paddingTop: 4, paddingBottom: 4 }}>
        <SidebarGroupLabel>资源</SidebarGroupLabel>
        <SidebarMenu>
          <NavButton active={view === 'skills' && !scope} onClick={() => onView('skills')} icon={<LayoutList size={15} aria-hidden="true" />} label="Skills" count={skillsCount} />
          <NavButton active={view === 'mcp'} onClick={() => onView('mcp')} icon={<Server size={15} aria-hidden="true" />} label="MCP" count={mcpCount} />
          <NavButton active={view === 'cli'} onClick={() => onView('cli')} icon={<Terminal size={15} aria-hidden="true" />} label="CLI" count={cliCount} />
          <NavButton active={view === 'instructions'} onClick={() => onView('instructions')} icon={<FileText size={15} aria-hidden="true" />} label="指令" count={instructionCount} />
        </SidebarMenu>
      </SidebarGroup>
      <SidebarGroup className="shrink-0" style={{ paddingTop: 4, paddingBottom: 4 }}>
        <SidebarGroupLabel>共享</SidebarGroupLabel>
        <SidebarMenu>
          <NavButton active={scope?.kind === 'library'} onClick={() => onScope({ kind: 'library', label: '共享 Skills' })} icon={<Database size={15} aria-hidden="true" />} label="共享 Skills" count={centralCount} />
        </SidebarMenu>
      </SidebarGroup>
      <SidebarGroup className="shrink-0" style={{ paddingTop: 4, paddingBottom: 4 }}>
        <SidebarGroupLabel asChild><button type="button" className={styles.sidebarGroupToggle} aria-expanded={openGroup === 'agents'} onClick={() => toggleGroup('agents')}><ChevronDown size={13} className={openGroup === 'agents' ? styles.sidebarChevronOpen : undefined} /><span>Agents</span></button></SidebarGroupLabel>
        <SidebarGroupAction title="管理 Agents" onClick={onManageAgents}><Settings2 size={12} style={{ width: 12, height: 12 }} /></SidebarGroupAction>
        <div className={`${styles.sidebarGroupBody} ${openGroup === 'agents' ? styles.sidebarGroupBodyOpen : ''}`} aria-hidden={openGroup !== 'agents'}>
          <div className={styles.sidebarGroupBodyInner}><div className={styles.agentsScroll}>
          <SidebarMenu>
            {agents.map((agent) => <SidebarMenuItem key={agent.id ?? agent.label}>
              <SidebarMenuButton isActive={scope?.kind === 'agent' && scope.id === agent.id} onClick={() => onScope({ kind: 'agent', id: agent.id ?? '', label: agent.label })} title={`${agent.label}：${agent.count ?? 0} 个可发现 Skill`} className="h-[28px] text-[12.5px]">
                <AgentIcon name={agent.label} className="size-4" /><span>{agent.label}</span>
              </SidebarMenuButton>
              {agent.count !== undefined && <SidebarMenuBadge title={`${agent.count} 个可发现 Skill`}>{agent.count}</SidebarMenuBadge>}
            </SidebarMenuItem>)}
          </SidebarMenu>
          </div></div>
        </div>
      </SidebarGroup>
      <SidebarGroup className="shrink-0" style={{ paddingTop: 4, paddingBottom: 4 }}>
        <SidebarGroupLabel asChild><button type="button" className={styles.sidebarGroupToggle} aria-expanded={openGroup === 'projects'} onClick={() => toggleGroup('projects')}><ChevronDown size={13} className={openGroup === 'projects' ? styles.sidebarChevronOpen : undefined} /><span>项目</span></button></SidebarGroupLabel>
        <SidebarGroupAction title="管理项目" onClick={onManageProjects}><FolderPlus size={12} style={{ width: 12, height: 12 }} /></SidebarGroupAction>
        <div className={`${styles.sidebarGroupBody} ${openGroup === 'projects' ? styles.sidebarGroupBodyOpen : ''}`} aria-hidden={openGroup !== 'projects'}>
          <div className={styles.sidebarGroupBodyInner}><div className={styles.agentsScroll}><SidebarMenu>
          {projects.map((project) => <SidebarMenuItem key={project.path}>
            <SidebarMenuButton isActive={scope?.kind === 'project' && scope.path === project.path} onClick={() => onScope({ kind: 'project', path: project.path || '', label: project.label })} className="h-[28px] text-[12.5px]" title={`${project.label}：${project.count ?? 0} 个 Skill`}>
              <Folder className="size-4" /><span>{project.label}</span>
            </SidebarMenuButton>
            {project.count !== undefined && <SidebarMenuBadge>{project.count}</SidebarMenuBadge>}
          </SidebarMenuItem>)}
        </SidebarMenu></div></div>
        </div>
      </SidebarGroup>

    </SidebarContent>
    <SidebarFooter>
      <SidebarMenu><NavButton active={view === 'history'} onClick={() => onView('history')} icon={<History size={15} aria-hidden="true" />} label="历史" /><NavButton active={view === 'settings'} onClick={() => onView('settings')} icon={<SettingsIcon size={15} aria-hidden="true" />} label="设置" /></SidebarMenu>
    </SidebarFooter>
  </Sidebar>
}
