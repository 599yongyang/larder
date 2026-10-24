import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { expect, test, vi } from 'vitest'
import { ProjectView } from './ProjectView'

vi.mock('../../backend', () => ({ backend: { instructionContent: async () => '# Rules\n\nrules' } }))

test('aggregates project skills, mcp and instructions', () => {
  const skill = { id: 's1', name: 'alpha', path: '/p/demo/.agents/skills/alpha/SKILL.md', rootId: 'r', rootKind: 'project', locked: false, hasSkillMd: true, descTokens: 1, scanState: 'ok', isAgentLocal: false, isProject: true, copyDrift: false }
  const server = { id: 'm1', name: 'context7', transport: 'http', status: 'ok', agents: [{ label: 'Cursor · demo', path: '/p/demo/.cursor/mcp.json', scope: '/p/demo', enabled: true }] }
  const file = { id: 'i1', name: 'AGENTS.md', path: '/p/demo/AGENTS.md', agent: '通用', scope: 'project', project: 'demo', size: 10, modifiedAt: '2026-01-01T00:00:00Z', hash: 'a', duplicate: false }
  render(<ProjectView path="/p/demo" skills={[skill]} servers={[server]} instructions={[file]} projects={[]} onRefresh={async () => {}} />)
  expect(screen.getByText('alpha')).toBeTruthy()
  expect(screen.getByText('context7')).toBeTruthy()
  expect(screen.getByText('AGENTS.md')).toBeTruthy()
})

test('opens project asset details inline', async () => {
  const skill = { id: 's1', name: 'alpha', path: '/p/demo/.agents/skills/alpha/SKILL.md', rootId: 'r', rootKind: 'project', locked: false, hasSkillMd: true, descTokens: 10, scanState: 'ok', isAgentLocal: false, isProject: true, copyDrift: false }
  const file = { id: 'i1', name: 'AGENTS.md', path: '/p/demo/AGENTS.md', agent: '通用', scope: 'project', project: 'demo', size: 10, modifiedAt: '2026-01-01T00:00:00Z', hash: 'a', duplicate: false, excerpt: 'rules' }
  render(<ProjectView path="/p/demo" skills={[skill]} servers={[]} instructions={[file]} projects={[]} onRefresh={async () => {}} />)
  await userEvent.click(screen.getByText('alpha'))
  expect(await screen.findByText('项目 Skill')).toBeTruthy()
  expect(screen.getByText('/p/demo/.agents/skills/alpha/SKILL.md')).toBeTruthy()
  await userEvent.keyboard('{Escape}')
  await userEvent.click(screen.getByText('AGENTS.md'))
  expect(await screen.findByText('内容')).toBeTruthy()
})
