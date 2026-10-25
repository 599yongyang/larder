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
  render(<HistoryView events={events} usage={[]} operations={[]} onRollback={async () => ({ id: 'rollback', planId: '', operation: 'rollback', status: 'success', createdAt: '' })} />)
  expect(screen.getByText('alpha')).toBeTruthy()
  await userEvent.type(screen.getByLabelText('搜索历史'), 'beta')
  expect(screen.queryByText('alpha')).toBeNull()
  expect(screen.getByText('beta')).toBeTruthy()
})

test('shows operations and external changes in one searchable feed', async () => {
  const operations = [
    { id: 'o1', planId: 'p1', operation: 'skill_update', status: 'success', message: 'updated alpha', createdAt: '2026-01-01T00:00:00Z' },
    { id: 'o2', planId: 'p2', operation: 'mcp_remove', status: 'failed', message: 'remove failed', createdAt: '2026-01-02T00:00:00Z' },
  ]
  const external = [{ id: 3, scanId: 3, kind: 'changed', skillName: 'gamma', path: '/g', at: '2026-01-03T00:00:00Z' }]
  render(<HistoryView events={external} usage={[]} operations={operations} onRollback={async () => ({ id: 'rollback', planId: '', operation: 'rollback', status: 'success', createdAt: '' })} />)
  expect(screen.getByText('updated alpha')).toBeTruthy()
  expect(screen.getByText('gamma')).toBeTruthy()
  await userEvent.type(screen.getByLabelText('搜索历史'), 'remove failed')
  expect(screen.queryByText('updated alpha')).toBeNull()
  expect(screen.getByText('remove failed')).toBeTruthy()
})
