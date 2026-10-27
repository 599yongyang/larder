import type { CLIInfo, Root } from '../types'

const aliases: Record<string, string[]> = {
  'claude-code': ['claude', 'claude-code'],
  codex: ['codex'],
  'gemini-cli': ['gemini', 'gemini-cli'],
  opencode: ['opencode'],
  cursor: ['cursor-agent', 'cursor'],
  'cursor-agent': ['cursor-agent'],
  'cursor-cli': ['cursor-agent'],
  'github-copilot': ['copilot'],
  'qwen-code': ['qwen'],
  'kiro-cli': ['kiro-cli', 'kiro'],
  'amazon-q': ['q'],
  crush: ['crush'],
  droid: ['droid'],
  cline: ['cline'],
  kiro: ['kiro', 'kiro-cli'],
  qwen: ['qwen', 'qwen-code'],
  copilot: ['copilot', 'github-copilot'],
  windsurf: ['windsurf'],
  amp: ['amp'],
  goose: ['goose'],
  aider: ['aider'],
  qoder: ['qoder'],
  openclaw: ['openclaw'],
}

export function agentCLIName(root: Root): string[] {
  const ids = [root.id, root.logicalAgent, root.label].filter((value): value is string => Boolean(value)).map((value) => value.toLowerCase())
  return ids.flatMap((id) => aliases[id] ?? [id])
}

export function findAgentCLI(root: Root, clis: CLIInfo[]): CLIInfo | undefined {
  const names = agentCLIName(root)
  return clis.find((cli) => { const actual = cli.name.toLowerCase(); return names.some((name) => actual === name || actual.endsWith('/' + name)) })
}
