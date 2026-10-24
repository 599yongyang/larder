import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeEach, expect, test, vi } from 'vitest'
import { MCPAddSheet } from './MCPAddSheet'

const planMCPAdd = vi.fn()
const applyPlan = vi.fn()
const mcpTargets = vi.fn()
const searchMCPRegistry = vi.fn()

vi.mock('../backend', () => ({
  backend: {
    planMCPAdd: (...args: unknown[]) => planMCPAdd(...args),
    applyPlan: (...args: unknown[]) => applyPlan(...args),
    mcpTargets: () => mcpTargets(),
    searchMCPRegistry: (...args: unknown[]) => searchMCPRegistry(...args),
  },
}))

beforeEach(() => {
  planMCPAdd.mockReset().mockResolvedValue({ id: 'plan:mcp-add' })
  applyPlan.mockReset().mockResolvedValue({})
  mcpTargets.mockReset().mockResolvedValue([{ label: 'Cursor', path: '/tmp/mcp.json', enabled: true }])
  searchMCPRegistry.mockReset().mockResolvedValue([])
})

test('filters featured servers and installs with an agent target', async () => {
  const onAdded = vi.fn()
  render(<MCPAddSheet open existingNames={[]} onClose={() => {}} onAdded={onAdded} />)
  const search = await screen.findByLabelText('搜索 MCP Server')
  await userEvent.type(search, 'figma')
  expect(screen.getAllByText('Figma').length).toBeGreaterThan(0)
  expect(screen.queryByText('Playwright')).toBeNull()
  await userEvent.click(screen.getByRole('button', { name: /添加/ }))
  await userEvent.click(await screen.findByText('Cursor'))
  await userEvent.click(screen.getByRole('button', { name: '添加' }))
  await waitFor(() => expect(planMCPAdd).toHaveBeenCalledTimes(1))
  expect(planMCPAdd.mock.calls[0][0]).toBe('figma')
  expect(onAdded).toHaveBeenCalled()
})

test('shows online registry results and marks unsupported entries', async () => {
  searchMCPRegistry.mockResolvedValue([
    { id: 'io.github.demo/demo', name: 'demo', title: 'Demo Registry', description: 'From registry', transport: 'http', command: '', args: [], url: 'https://example.com/mcp', env: [] },
    { id: 'io.github.auth/auth', name: 'auth', title: 'Needs Auth', description: 'Needs a header', transport: 'http', command: '', args: [], url: '', env: [], unsupported: '需要 HTTP Header' },
  ])
  render(<MCPAddSheet open existingNames={[]} onClose={() => {}} onAdded={() => {}} />)
  const search = await screen.findByLabelText('搜索 MCP Server')
  await userEvent.type(search, 'brave')
  expect(await screen.findByText('Demo Registry')).toBeTruthy()
  expect(screen.getByText('Needs Auth')).toBeTruthy()
  expect(screen.getByText('需要 HTTP Header')).toBeTruthy()
})

test('adds a custom MCP server with compact fields', async () => {
  render(<MCPAddSheet open existingNames={[]} onClose={() => {}} onAdded={() => {}} />)
  await userEvent.click(await screen.findByRole('button', { name: '自定义' }))
  await userEvent.type(screen.getByPlaceholderText('my-server'), 'my-server')
  await userEvent.type(screen.getByPlaceholderText('npx'), 'node')
  await userEvent.click(await screen.findByText('Cursor'))
  await userEvent.click(screen.getByRole('button', { name: '添加' }))
  await waitFor(() => expect(planMCPAdd).toHaveBeenCalledTimes(1))
  expect(planMCPAdd.mock.calls[0][0]).toBe('my-server')
})
