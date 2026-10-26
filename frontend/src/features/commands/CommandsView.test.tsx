import { render, screen } from '@testing-library/react'
import { describe, expect, it, vi } from 'vitest'
import { CommandsView } from './CommandsView'
import type { Command } from '../../types'

vi.mock('../../backend', () => ({ backend: { commandTargets: async () => [], commandContent: async () => '' } }))

const command: Command = { id: 'c1', name: 'engineering/review', description: 'Review a change', agent: 'Claude Code', rootPath: '/claude/commands', path: '/claude/commands/engineering/review.md', scope: 'global', size: 120, modifiedAt: '', hash: 'abc' }

describe('CommandsView', () => {
  it('renders custom commands with their Agent', () => {
    render(<CommandsView commands={[command]} onRefresh={async () => undefined} />)
    expect(screen.getByText('/engineering/review')).toBeTruthy()
    expect(screen.getByText('Review a change')).toBeTruthy()
    expect(screen.getAllByText('Claude Code').length).toBeGreaterThan(0)
  })
})
