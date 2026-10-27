import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { expect, test, vi } from 'vitest'
import { ProjectView } from './ProjectView'

vi.mock('../../backend', () => ({ backend: { instructionContent: async () => '# Rules\n\nrules' } }))

test('aggregates project skills, mcp and instructions', async () => {
  const skill = { id: 's1', name: 'alpha', path: '/p/demo/.agents/skills/alpha/SKILL.md', rootId: 'r', rootKind: 'project', locked: false, hasSkillMd: true, descTokens: 1, scanState: 'ok', isAgentLocal: false, isProject: true, copyDrift: false }
  const server = { id: 'm1', name: 'context7', transport: 'http', status: 'ok', agents: [{ label: 'Cursor · demo', path: '/p/demo/.cursor/mcp.json', scope: '/p/demo', enabled: true }] }
  const file = { id: 'i1', name: 'AGENTS.md', path: '/p/demo/AGENTS.md', agent: '通用', scope: 'project', project: 'demo', size: 10, modifiedAt: '2026-01-01T00:00:00Z', hash: 'a', duplicate: false }
  render(<ProjectView path="/p/demo" skills={[skill]} servers={[server]} instructions={[file]} commands={[]} operations={[]} sessions={[]} sessionsLoading={false} projects={[]} onRefresh={async () => {}} onOpenCommands={() => {}} />)
  await userEvent.click(screen.getByRole('tab', { name: /项目资源|Resources/ }))
  expect(screen.getByText('alpha')).toBeTruthy()
  expect(screen.getByText('context7')).toBeTruthy()
  expect(screen.getByText('AGENTS.md')).toBeTruthy()
})

test('opens project asset details inline', async () => {
  const skill = { id: 's1', name: 'alpha', path: '/p/demo/.agents/skills/alpha/SKILL.md', rootId: 'r', rootKind: 'project', locked: false, hasSkillMd: true, descTokens: 10, scanState: 'ok', isAgentLocal: false, isProject: true, copyDrift: false }
  const file = { id: 'i1', name: 'AGENTS.md', path: '/p/demo/AGENTS.md', agent: '通用', scope: 'project', project: 'demo', size: 10, modifiedAt: '2026-01-01T00:00:00Z', hash: 'a', duplicate: false, excerpt: 'rules' }
  render(<ProjectView path="/p/demo" skills={[skill]} servers={[]} instructions={[file]} commands={[]} operations={[]} sessions={[]} sessionsLoading={false} projects={[]} onRefresh={async () => {}} onOpenCommands={() => {}} />)
  await userEvent.click(screen.getByRole('tab', { name: /项目资源|Resources/ }))
  await userEvent.click(screen.getByText('alpha'))
  expect(await screen.findByText('项目 Skill')).toBeTruthy()
  expect(screen.getByText('/p/demo/.agents/skills/alpha/SKILL.md')).toBeTruthy()
  await userEvent.keyboard('{Escape}')
  await userEvent.click(screen.getByText('AGENTS.md'))
  expect(await screen.findByText('内容')).toBeTruthy()
})

test('shows project commands and recent project activity', async () => {
  const command = { id: 'c1', name: 'review', description: 'Review changes', agent: 'Claude Code', rootPath: '/p/demo/.claude/commands', path: '/p/demo/.claude/commands/review.md', format: 'markdown', scope: 'project', project: '/p/demo', size: 10, modifiedAt: '2026-01-01T00:00:00Z', hash: 'h' }
  const operation = { id: 'o1', planId: 'p1', operation: 'command_create', status: 'success', changes: ['command:/p/demo/.claude/commands/review.md'], createdAt: '2026-01-01T00:00:00Z' }
  render(<ProjectView path="/p/demo" skills={[]} servers={[]} instructions={[]} commands={[command]} operations={[operation]} sessions={[]} sessionsLoading={false} projects={[]} onRefresh={async () => {}} onOpenCommands={() => {}} />)
  await userEvent.click(screen.getByRole('tab', { name: /项目资源|Resources/ }))
  expect(screen.getByText('review')).toBeTruthy()
  await userEvent.click(screen.getByRole('tab', { name: /最近变更|Recent activity/ }))
  expect(screen.getByText('新建命令')).toBeTruthy()
})

test('shows recent project sessions', () => {
  const session = { id: 's1', agent: 'Claude Code', cwd: '/p/demo', title: 'Fix the build', model: 'claude-sonnet', path: '/tmp/s.jsonl', startedAt: '2026-01-01T00:00:00Z', lastAt: '2026-01-01T00:05:00Z', messages: 12, tokensIn: 10, tokensOut: 20 }
  render(<ProjectView path="/p/demo" skills={[]} servers={[]} instructions={[]} commands={[]} operations={[]} sessions={[session]} sessionsLoading={false} projects={[]} onRefresh={async () => {}} onOpenCommands={() => {}} />)
  expect(screen.getByText('Fix the build')).toBeTruthy()
  expect(screen.getByText(/12 条消息/)).toBeTruthy()
})
