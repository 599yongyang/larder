import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { expect, test, vi } from 'vitest'
import { MCPView } from './MCPView'

const applyPlan = vi.hoisted(() => vi.fn(async () => ({ id: 'op:mcp', operation: 'mcp_remove', status: 'success' })))
vi.mock('../../backend', () => ({
  backend: {
    planMCPRemove: async () => ({ id: 'plan:mcp', operation: 'mcp_remove', title: '移除 MCP Server', summary: 'test', changes: [{ action: 'remove_mcp_entry', rootId: 'Cursor', rootLabel: 'Cursor', target: '/tmp/mcp.json' }], impact: [], requiresSnapshot: true, executable: true, createdAt: '' }),
    applyPlan,
    rollbackOperation: async () => ({}),
  },
}))

const server = { id: 'mcp:demo', name: 'demo', transport: 'stdio', command: 'npx', args: ['-y', 'demo'], status: 'ok', agents: [{ label: 'Cursor', path: '/tmp/mcp.json', enabled: true }] }

test('mcp removal selects agents and applies directly', async () => {
  render(<MCPView servers={[server]} clis={[]} onRefresh={async () => {}} />)
  await userEvent.click(screen.getByRole('button', { name: '移除 MCP 配置' }))
  expect(await screen.findByText('移除 demo')).toBeTruthy()
  await userEvent.click(screen.getByRole('button', { name: '确认移除' }))
  await waitFor(() => expect(applyPlan).toHaveBeenCalledWith('plan:mcp'))
  expect(screen.queryByText('移除 MCP Server')).toBeNull()
})
