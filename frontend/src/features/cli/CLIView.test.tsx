import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, it } from 'vitest'
import { CLIView } from './CLIView'
import type { CLIInfo } from '../../types'

const clis: CLIInfo[] = [
  { id: 'cli:jq', name: 'jq', path: '/opt/homebrew/bin/jq', owner: 'Homebrew', status: 'duplicate', duplicateCount: 1, scannedAt: '' },
  { id: 'cli:rg', name: 'rg', path: '/opt/homebrew/bin/rg', owner: 'Homebrew', status: 'ok', duplicateCount: 0, scannedAt: '' },
]

describe('CLIView', () => {
  it('shows commands and duplicate state', async () => {
    render(<CLIView clis={clis} skills={[]} mcps={[]} onRefresh={async () => {}} updatesLoading={false} onCheckUpdates={() => {}} />)
    expect(screen.getAllByText('jq').length).toBeGreaterThan(0)
    expect(screen.getAllByText('rg').length).toBeGreaterThan(0)
    await userEvent.click(screen.getAllByText('jq')[0])
    expect(screen.getAllByText('重复').length).toBeGreaterThan(0)
    expect(screen.getByText('2 处')).toBeInTheDocument()
  })
})
