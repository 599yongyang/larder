import { useEffect } from 'react'
import { FileText, History, LayoutList, RefreshCw, Search, Server, Settings as SettingsIcon, Sun, Terminal } from 'lucide-react'
import { CommandDialog, CommandEmpty, CommandGroup, CommandInput, CommandItem, CommandList, CommandShortcut } from '@/components/ui/command'
import { SkillIcon } from './SkillIcon'
import type { CLIInfo, InstructionFile, MCPServer, Skill, View } from '../types'
import { t } from '../i18n'
import { shortcutLabel } from '../lib/platform'

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
  onScan: () => void
  onCheckUpdates: () => void
  onToggleAppearance: () => void
}

export function CommandPalette({ open, onOpenChange, skills, clis, mcps, instructions, onView, onSkill, onCLI, onMCP, onInstruction, onScan, onCheckUpdates, onToggleAppearance }: Props) {
  const views = [
    { view: 'skills' as View, label: t('common.skills'), icon: LayoutList },
    { view: 'cli' as View, label: 'CLI', icon: Terminal },
    { view: 'mcp' as View, label: 'MCP', icon: Server },
    { view: 'instructions' as View, label: t('common.instructions'), icon: FileText },
    { view: 'history' as View, label: t('common.history'), icon: History },
    { view: 'settings' as View, label: t('common.settings'), icon: SettingsIcon },
  ]
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

  return <CommandDialog open={open} onOpenChange={onOpenChange} title={t("command.searchAndJump")} description={t("command.searchSkillsClisMcpsInstructionsOrPages")}>
    <CommandInput placeholder={t("command.searchSkillsClisMcpsInstructions")} aria-label={t("command.searchAndJump")} />
    <CommandList>
      <CommandEmpty>{t('command.noMatchingResults')}</CommandEmpty>
      <CommandGroup heading={t('command.actions')}>
        <CommandItem value={t('command.scanMachine')} onSelect={() => { onScan(); onOpenChange(false) }}><RefreshCw /><span>{t('command.scanMachine')}</span><CommandShortcut>{shortcutLabel('R')}</CommandShortcut></CommandItem>
        <CommandItem value={t('command.checkUpdates')} onSelect={() => { onCheckUpdates(); onOpenChange(false) }}><RefreshCw /><span>{t('command.checkUpdates')}</span></CommandItem>
        <CommandItem value={t('command.toggleAppearance')} onSelect={() => { onToggleAppearance(); onOpenChange(false) }}><Sun /><span>{t('command.toggleAppearance')}</span></CommandItem>
      </CommandGroup>
      <CommandGroup heading={t("command.pages")}>
        {views.map((item) => <CommandItem key={item.view} value={`${t("command.pages")} ${item.label}`} onSelect={() => { onView(item.view); onOpenChange(false) }}>
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
      {instructions.length > 0 && <CommandGroup heading={t("common.instructions")}>
        {instructions.map((file) => <CommandItem key={file.id} value={`${file.name} ${file.agent} ${file.path}`} onSelect={() => { onInstruction(); onOpenChange(false) }}>
          <FileText className="text-muted-foreground" /><span>{file.name}</span><CommandShortcut>{file.agent}</CommandShortcut>
        </CommandItem>)}
      </CommandGroup>}
      {skills.length > 0 && <CommandGroup heading="Skills">
        {skills.map((skill) => <CommandItem key={skill.id} value={`${skill.name} ${skill.source ?? ''} ${skill.description ?? ''} ${skill.path}`} onSelect={() => { onSkill(skill); onOpenChange(false) }}>
          <SkillIcon skill={skill} className="size-5" /><span>{skill.name}</span>{skill.source && <CommandShortcut>{skill.source}</CommandShortcut>}
        </CommandItem>)}
      </CommandGroup>}
      <CommandGroup heading={t("command.tips")}>
        <CommandItem disabled><Search /><span>{t('command.typeANameSourcePathOrPageName')}</span><CommandShortcut>{shortcutLabel('K')}</CommandShortcut></CommandItem>
      </CommandGroup>
    </CommandList>
  </CommandDialog>
}
