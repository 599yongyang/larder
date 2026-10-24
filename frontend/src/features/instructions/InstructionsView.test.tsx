import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { expect, test, vi } from 'vitest'
import { InstructionsView } from './InstructionsView'

vi.mock('../../backend', () => ({ backend: { instructionContent: async () => '# Rules\n\nglobal rules' } }))

const files = [
  { id: 'i1', name: 'AGENTS.md', path: '/p/AGENTS.md', agent: '通用', scope: 'project', project: 'demo', size: 20, modifiedAt: '2026-01-01T00:00:00Z', hash: 'a'.repeat(64), excerpt: 'project rules', duplicate: false },
  { id: 'i2', name: 'CLAUDE.md', path: '/h/.claude/CLAUDE.md', agent: 'Claude Code', scope: 'global', size: 12, modifiedAt: '2026-01-02T00:00:00Z', hash: 'b'.repeat(64), excerpt: 'global rules', duplicate: false },
]

test('filters instruction files by search and opens preview', async () => {
  render(<InstructionsView files={files} projects={[]} onRefresh={async () => {}} />)
  expect(screen.getByText('AGENTS.md')).toBeTruthy()
  await userEvent.type(screen.getByLabelText('搜索指令文件'), 'claude')
  expect(screen.queryByText('AGENTS.md')).toBeNull()
  await userEvent.click(screen.getByText('CLAUDE.md'))
  expect(await screen.findByText('内容')).toBeTruthy()
  expect(screen.getByText('global rules')).toBeTruthy()
  await userEvent.click(screen.getByRole('button', { name: '原文' }))
  expect(screen.getByText(/# Rules/)).toBeTruthy()
})
