import { render, screen } from '@testing-library/react'
import { expect, test } from 'vitest'
import { AgentSessionsView } from './AgentSessionsView'

test('lists sessions across projects for one agent', () => {
  const session = { id: 's1', agent: 'Claude Code', cwd: '/p/demo', title: 'Fix the build', model: 'claude-sonnet', path: '/tmp/s.jsonl', startedAt: '2026-01-01T00:00:00Z', lastAt: '2026-01-01T00:05:00Z', messages: 12, tokensIn: 0, tokensOut: 0 }
  render(<AgentSessionsView sessions={[session]} loading={false} />)
  expect(screen.getByText('Fix the build')).toBeTruthy()
  expect(screen.getByText(/12 条消息/)).toBeTruthy()
})

test('shows an empty state before any session is found', () => {
  render(<AgentSessionsView sessions={[]} loading={false} />)
  expect(screen.getByText('这个 Agent 还没有会话')).toBeTruthy()
})
