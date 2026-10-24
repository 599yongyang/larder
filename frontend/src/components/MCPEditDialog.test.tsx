import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeEach, expect, test, vi } from 'vitest'
import { MCPEditDialog } from './MCPEditDialog'

const planMCPUpdate = vi.fn()
const applyPlan = vi.fn()
vi.mock('../backend', () => ({ backend: { planMCPUpdate: (...args: unknown[]) => planMCPUpdate(...args), applyPlan: (...args: unknown[]) => applyPlan(...args) } }))

beforeEach(() => {
  planMCPUpdate.mockReset().mockResolvedValue({ id: 'plan:mcp-update' })
  applyPlan.mockReset().mockResolvedValue({})
})

test('edits command and keeps the existing agents', async () => {
  const onSaved = vi.fn()
  const server = { id: 'mcp:demo', name: 'demo', transport: 'stdio', command: 'npx', args: ['old'], status: 'ok', agents: [{ label: 'Cursor', path: '/tmp/mcp.json', enabled: true }] }
  render(<MCPEditDialog server={server} onClose={() => {}} onSaved={onSaved} />)
  const command = screen.getByDisplayValue('npx')
  await userEvent.clear(command)
  await userEvent.type(command, 'node')
  await userEvent.click(screen.getByRole('button', { name: '保存' }))
  await waitFor(() => expect(planMCPUpdate).toHaveBeenCalledTimes(1))
  expect(planMCPUpdate.mock.calls[0][1]).toBe('demo')
  expect(planMCPUpdate.mock.calls[0][5]).toEqual(['/tmp/mcp.json'])
  expect(onSaved).toHaveBeenCalled()
})
