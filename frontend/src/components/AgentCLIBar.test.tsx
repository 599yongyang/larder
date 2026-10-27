import { describe, expect, it, vi } from 'vitest'
import { render, screen } from '@testing-library/react'
import { AgentCLIBar } from './AgentCLIBar'
import type { CLIInfo, Root } from '../types'

vi.mock('../backend', () => ({
  backend: {
    cliVersion: vi.fn().mockResolvedValue('0.164.0'),
    nativeCLIUpdateCheck: vi.fn().mockRejectedValue(new Error('unavailable')),
  },
}))

const root: Root = { id: 'codex', label: 'Codex', logicalAgent: 'Codex', path: '/tmp/codex', kind: 'known_agent', known: true, status: 'available', entryCount: 1 }
const cli: CLIInfo = { id: 'cli:codex', name: 'codex', path: '/tmp/codex', owner: 'standalone', version: '', status: 'ok', duplicateCount: 0, scannedAt: '' }

describe('AgentCLIBar', () => {
  it('keeps hook order when a CLI appears after an empty render', () => {
    const props = { root, updates: undefined, loading: false, onCheck: vi.fn(), onOpenCLI: vi.fn(), onUpdated: vi.fn() }
    const { container, rerender } = render(<AgentCLIBar {...props} clis={[]} />)
    expect(container.textContent).not.toBe('')
    expect(() => rerender(<AgentCLIBar {...props} clis={[cli]} />)).not.toThrow()
  })
})

describe('AgentCLIBar update state', () => {
  it('does not call an update available when latest equals the installed version', () => {
    const upToDate: CLIInfo = { ...cli, version: '2.1.311', owner: 'npm' }
    const updates = {
      checkedAt: '2026-10-27T00:00:00Z', scanned: 1, available: 1, current: 0, skillScanned: 0, skillChanged: 0, skillCurrent: 0, skillErrors: 0,
      items: [{ kind: 'cli', cliId: 'cli:codex', owner: 'npm', name: 'codex', currentVersion: '2.1.304', latestVersion: '2.1.311', state: 'available' }],
    }
    const props = { root, updates: updates as never, loading: false, onCheck: vi.fn(), onOpenCLI: vi.fn(), onUpdated: vi.fn() }
    render(<AgentCLIBar {...props} clis={[upToDate]} />)
    expect(screen.queryByRole('button', { name: /升级/ })).toBeNull()
    expect(screen.getByText('已是最新')).toBeTruthy()
  })
})
