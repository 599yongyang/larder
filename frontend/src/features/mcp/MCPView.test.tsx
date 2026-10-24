import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { expect, test, vi } from 'vitest'
import { MCPView } from './MCPView'

vi.mock('../../backend', () => ({
  backend: {
    planMCPRemove: async () => ({ id: 'plan:mcp', operation: 'mcp_remove', title: '移除 MCP Server', summary: 'test', changes: [{ action: 'remove_mcp_entry', rootId: 'Cursor', rootLabel: 'Cursor', target: '/tmp/mcp.json' }], impact: [], requiresSnapshot: true, executable: true, createdAt: '' }),
    applyPlan: async () => ({}),
  },
}))

const server = { id: 'mcp:demo', name: 'demo', transport: 'stdio', command: 'npx', args: ['-y', 'demo'], status: 'ok', agents: [{ label: 'Cursor', path: '/tmp/mcp.json', enabled: true }] }

test('mcp removal asks for agents and a plan confirmation', async () => {
  render(<MCPView servers={[server]} clis={[]} onRefresh={async () => {}} />)
  await userEvent.click(screen.getByRole('button', { name: '移除 MCP 配置' }))
  expect(await screen.findByText('移除 demo')).toBeTruthy()
  await userEvent.click(screen.getByRole('button', { name: '预览移除' }))
  expect(await screen.findByText('移除 MCP Server')).toBeTruthy()
})
