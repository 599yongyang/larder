import { useState } from 'react'
import { ChevronDown, Database, Folder, FolderPlus, History, FileText, LayoutList, Server, Settings as SettingsIcon, Settings2, Terminal } from 'lucide-react'
import { Sidebar, SidebarContent, SidebarFooter, SidebarGroup, SidebarGroupAction, SidebarGroupLabel, SidebarHeader, SidebarMenu, SidebarMenuBadge, SidebarMenuButton, SidebarMenuItem } from '@/components/ui/sidebar'
import { AgentIcon } from './AgentIcon'
import { NavButton } from './NavButton'
import styles from '../App.module.css'
import { t } from '../i18n'
import { isMacPlatform } from '../lib/platform'
import type { View } from '../types'

type Scope = { kind: 'library'; label: string } | { kind: 'agent'; id: string; label: string } | { kind: 'project'; path: string; label: string } | undefined
type Item = { id?: string; label: string; icon?: string; count?: number; path?: string }

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
  const mac = isMacPlatform()
  const [openGroup, setOpenGroup] = useState<'agents' | 'projects' | null>('agents')
  const toggleGroup = (group: 'agents' | 'projects') => setOpenGroup((current) => current === group ? null : group)
  return <Sidebar collapsible="none">
    <SidebarHeader style={mac ? { paddingTop: 8, paddingBottom: 0, paddingLeft: 82 } : { paddingTop: 10, paddingBottom: 0, paddingLeft: 8 }}>
      <div className={`flex items-center px-2 ${mac ? 'h-9' : 'h-10'}`}><span className={styles.brandWordmark}>Larder</span></div>
    </SidebarHeader>
    <SidebarContent className="overflow-hidden" style={{ gap: 4 }}>
      <SidebarGroup className="shrink-0" style={{ paddingTop: 4, paddingBottom: 4 }}>
        <SidebarGroupLabel>{t('nav.resources')}</SidebarGroupLabel>
        <SidebarMenu>
          <NavButton active={view === 'skills' && !scope} onClick={() => onView('skills')} icon={<LayoutList size={15} aria-hidden="true" />} label={t('common.skills')} count={skillsCount} />
          <NavButton active={view === 'mcp'} onClick={() => onView('mcp')} icon={<Server size={15} aria-hidden="true" />} label="MCP" count={mcpCount} />
          <NavButton active={view === 'cli'} onClick={() => onView('cli')} icon={<Terminal size={15} aria-hidden="true" />} label="CLI" count={cliCount} />
          <NavButton active={view === 'instructions'} onClick={() => onView('instructions')} icon={<FileText size={15} aria-hidden="true" />} label={t('common.instructions')} count={instructionCount} />
        </SidebarMenu>
      </SidebarGroup>
      <SidebarGroup className="shrink-0" style={{ paddingTop: 4, paddingBottom: 4 }}>
        <SidebarGroupLabel>{t('nav.shared')}</SidebarGroupLabel>
        <SidebarMenu>
          <NavButton active={scope?.kind === 'library'} onClick={() => onScope({ kind: 'library', label: t('common.sharedLibrary') })} icon={<Database size={15} aria-hidden="true" />} label={t('common.sharedLibrary')} count={centralCount} />
        </SidebarMenu>
      </SidebarGroup>
      <SidebarGroup className="shrink-0" style={{ paddingTop: 4, paddingBottom: 4 }}>
        <SidebarGroupLabel asChild><button type="button" className={styles.sidebarGroupToggle} aria-expanded={openGroup === 'agents'} onClick={() => toggleGroup('agents')}><ChevronDown size={13} className={openGroup === 'agents' ? styles.sidebarChevronOpen : undefined} /><span>{t('nav.agents')}</span></button></SidebarGroupLabel>
        <SidebarGroupAction title={t('common.manageAgents')} aria-label={t('common.manageAgents')} onClick={onManageAgents}><Settings2 size={12} style={{ width: 12, height: 12 }} /></SidebarGroupAction>
        <div className={`${styles.sidebarGroupBody} ${openGroup === 'agents' ? styles.sidebarGroupBodyOpen : ''}`} aria-hidden={openGroup !== 'agents'}>
          <div className={styles.sidebarGroupBodyInner}><div className={styles.agentsScroll}>
          <SidebarMenu>
            {agents.map((agent) => <SidebarMenuItem key={agent.id ?? agent.label}>
              <SidebarMenuButton isActive={scope?.kind === 'agent' && scope.id === agent.id} onClick={() => onScope({ kind: 'agent', id: agent.id ?? '', label: agent.label })} title={`${agent.label}: ${agent.count ?? 0} ${t('nav.discoverableSkills')}`} className="h-[28px] text-[12.5px]">
                <AgentIcon name={agent.icon ?? agent.label} className="size-4" /><span>{agent.label}</span>
              </SidebarMenuButton>
              {agent.count !== undefined && <SidebarMenuBadge title={`${agent.count} ${t('nav.discoverableSkills')}`}>{agent.count}</SidebarMenuBadge>}
            </SidebarMenuItem>)}
          </SidebarMenu>
          </div></div>
        </div>
      </SidebarGroup>
      <SidebarGroup className="shrink-0" style={{ paddingTop: 4, paddingBottom: 4 }}>
        <SidebarGroupLabel asChild><button type="button" className={styles.sidebarGroupToggle} aria-expanded={openGroup === 'projects'} onClick={() => toggleGroup('projects')}><ChevronDown size={13} className={openGroup === 'projects' ? styles.sidebarChevronOpen : undefined} /><span>{t('common.projects')}</span></button></SidebarGroupLabel>
        <SidebarGroupAction title={t('common.manageProjects')} aria-label={t('common.manageProjects')} onClick={onManageProjects}><FolderPlus size={12} style={{ width: 12, height: 12 }} /></SidebarGroupAction>
        <div className={`${styles.sidebarGroupBody} ${openGroup === 'projects' ? styles.sidebarGroupBodyOpen : ''}`} aria-hidden={openGroup !== 'projects'}>
          <div className={styles.sidebarGroupBodyInner}><div className={styles.agentsScroll}><SidebarMenu>
          {projects.map((project) => <SidebarMenuItem key={project.path}>
            <SidebarMenuButton isActive={scope?.kind === 'project' && scope.path === project.path} onClick={() => onScope({ kind: 'project', path: project.path || '', label: project.label })} className="h-[28px] text-[12.5px]" title={`${project.label}: ${project.count ?? 0} ${t('nav.skills')}`}>
              <Folder className="size-4" /><span>{project.label}</span>
            </SidebarMenuButton>
            {project.count !== undefined && <SidebarMenuBadge>{project.count}</SidebarMenuBadge>}
          </SidebarMenuItem>)}
        </SidebarMenu></div></div>
        </div>
      </SidebarGroup>

    </SidebarContent>
    <SidebarFooter>
      <SidebarMenu><NavButton active={view === 'history'} onClick={() => onView('history')} icon={<History size={15} aria-hidden="true" />} label={t('common.history')} /><NavButton active={view === 'settings'} onClick={() => onView('settings')} icon={<SettingsIcon size={15} aria-hidden="true" />} label={t('common.settings')} /></SidebarMenu>
    </SidebarFooter>
  </Sidebar>
}
