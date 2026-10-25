import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { expect, test, vi } from 'vitest'
import { HistoryView } from './HistoryView'

vi.mock('../../backend', () => ({ backend: { exportReport: async () => 'report' } }))

const events = [
  { id: 1, scanId: 1, kind: 'added', skillName: 'alpha', path: '/a', at: '2026-01-01T00:00:00Z' },
  { id: 2, scanId: 2, kind: 'removed', skillName: 'beta', path: '/b', at: '2026-01-02T00:00:00Z' },
]

test('filters timeline events by search', async () => {
  render(<HistoryView events={events} usage={[]} operations={[]} onRollback={async () => {}} />)
  expect(screen.getByText('alpha')).toBeTruthy()
  await userEvent.type(screen.getByLabelText('搜索历史'), 'beta')
  expect(screen.queryByText('alpha')).toBeNull()
  expect(screen.getByText('beta')).toBeTruthy()
})

test('filters the unified feed by record type and search', async () => {
  const operations = [
    { id: 'o1', planId: 'p1', operation: 'skill_update', status: 'success', message: 'updated alpha', createdAt: '2026-01-01T00:00:00Z' },
    { id: 'o2', planId: 'p2', operation: 'mcp_remove', status: 'failed', message: 'remove failed', createdAt: '2026-01-02T00:00:00Z' },
  ]
  render(<HistoryView events={[]} usage={[]} operations={operations} onRollback={async () => {}} />)
  await userEvent.click(screen.getByRole('button', { name: /操作记录/ }))
  expect(screen.getByText('updated alpha')).toBeTruthy()
  await userEvent.type(screen.getByLabelText('搜索历史'), 'remove failed')
  expect(screen.queryByText('updated alpha')).toBeNull()
  expect(screen.getByText('remove failed')).toBeTruthy()
  await userEvent.clear(screen.getByLabelText('搜索历史'))
  await userEvent.click(screen.getByRole('button', { name: /外部变化/ }))
  expect(screen.getByText('还没有历史记录。Larder 操作和外部 Skill 文件变化会显示在这里。')).toBeTruthy()
})
