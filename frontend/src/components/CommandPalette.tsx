import { useEffect } from 'react'
import { FileText, History, LayoutList, Search, Server, Settings as SettingsIcon, Terminal } from 'lucide-react'
import { CommandDialog, CommandEmpty, CommandGroup, CommandInput, CommandItem, CommandList, CommandShortcut } from '@/components/ui/command'
import { SkillIcon } from './SkillIcon'
import type { CLIInfo, InstructionFile, MCPServer, Skill, View } from '../types'

type Props = {
  open: boolean
  onOpenChange: (open: boolean) => void
  skills: Skill[]
  clis: CLIInfo[]
  mcps: MCPServer[]
  instructions: InstructionFile[]
  onView: (view: View) => void
  onSkill: (skill: Skill) => void
  onCLI: (cli: CLIInfo) => void
  onMCP: (mcp: MCPServer) => void
  onInstruction: () => void
}

const views = [
  { view: 'skills' as View, label: 'Skills', icon: LayoutList },
  { view: 'cli' as View, label: 'CLI', icon: Terminal },
  { view: 'mcp' as View, label: 'MCP', icon: Server },
  { view: 'instructions' as View, label: '指令', icon: FileText },
  { view: 'history' as View, label: '历史', icon: History },
  { view: 'settings' as View, label: '设置', icon: SettingsIcon },
]

export function CommandPalette({ open, onOpenChange, skills, clis, mcps, instructions, onView, onSkill, onCLI, onMCP, onInstruction }: Props) {
  useEffect(() => {
    const onKey = (event: KeyboardEvent) => {
      if (event.key.toLowerCase() === 'k' && (event.metaKey || event.ctrlKey)) {
        event.preventDefault()
        onOpenChange(true)
      }
    }
    window.addEventListener('keydown', onKey)
    return () => window.removeEventListener('keydown', onKey)
  }, [onOpenChange])

  return <CommandDialog open={open} onOpenChange={onOpenChange} title="搜索与跳转" description="搜索 Skill、CLI、MCP、指令或页面">
    <CommandInput placeholder="搜索 Skill、CLI、MCP、指令…" />
    <CommandList>
      <CommandEmpty>没有匹配结果。</CommandEmpty>
      <CommandGroup heading="页面">
        {views.map((item) => <CommandItem key={item.view} value={`页面 ${item.label}`} onSelect={() => { onView(item.view); onOpenChange(false) }}>
          <item.icon /><span>{item.label}</span>
        </CommandItem>)}
      </CommandGroup>
      {clis.length > 0 && <CommandGroup heading="CLI">
        {clis.map((cli) => <CommandItem key={cli.id} value={`${cli.name} ${cli.owner} ${cli.path}`} onSelect={() => { onCLI(cli); onOpenChange(false) }}>
          <Terminal className="text-muted-foreground" /><span>{cli.name}</span><CommandShortcut>{cli.owner}</CommandShortcut>
        </CommandItem>)}
      </CommandGroup>}
      {mcps.length > 0 && <CommandGroup heading="MCP">
        {mcps.map((mcp) => <CommandItem key={mcp.id} value={`${mcp.name} ${mcp.command ?? ''} ${mcp.url ?? ''}`} onSelect={() => { onMCP(mcp); onOpenChange(false) }}>
          <Server className="text-muted-foreground" /><span>{mcp.name}</span><CommandShortcut>{mcp.transport}</CommandShortcut>
        </CommandItem>)}
      </CommandGroup>}
      {instructions.length > 0 && <CommandGroup heading="指令">
        {instructions.map((file) => <CommandItem key={file.id} value={`${file.name} ${file.agent} ${file.path}`} onSelect={() => { onInstruction(); onOpenChange(false) }}>
          <FileText className="text-muted-foreground" /><span>{file.name}</span><CommandShortcut>{file.agent}</CommandShortcut>
        </CommandItem>)}
      </CommandGroup>}
      {skills.length > 0 && <CommandGroup heading="Skills">
        {skills.map((skill) => <CommandItem key={skill.id} value={`${skill.name} ${skill.source ?? ''} ${skill.description ?? ''} ${skill.path}`} onSelect={() => { onSkill(skill); onOpenChange(false) }}>
          <SkillIcon skill={skill} className="size-5" /><span>{skill.name}</span>{skill.source && <CommandShortcut>{skill.source}</CommandShortcut>}
        </CommandItem>)}
      </CommandGroup>}
      <CommandGroup heading="提示">
        <CommandItem disabled><Search /><span>输入名称、来源、路径或页面名</span><CommandShortcut>⌘K</CommandShortcut></CommandItem>
      </CommandGroup>
    </CommandList>
  </CommandDialog>
}
