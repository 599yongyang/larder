import { render, screen } from '@testing-library/react'
import { describe, expect, it, vi } from 'vitest'
import type { Root, Skill } from '../../types'
import { XrayView } from './XrayView'

const root: Root = { id: 'claude', label: 'Claude Code', logicalAgent: 'Claude Code', path: '/claude', kind: 'known_agent', known: true, status: 'available', entryCount: 1 }
const skill: Skill = { id: 's1', name: 'frontend-design', path: '/claude/frontend-design', rootId: 'claude', rootKind: 'known_agent', source: 'anthropics/skills', locked: true, hasSkillMd: true, description: 'Design guidance', descTokens: 58, scanState: 'ok', isAgentLocal: true, isProject: false, copyDrift: false }

vi.mock('../../backend', () => ({
  backend: {
    agentUsage: vi.fn(async () => []),
    xray: vi.fn(async () => ({
      schemaVersion: '1', agentId: 'claude', agentLabel: 'Claude Code', root,
      entries: [{ skill, status: 'active', reason: 'visible', candidates: [skill] }],
      activeCount: 1, missingCount: 0, candidateCount: 0, brokenCount: 0,
      budget: { skillTokens: 58, instructionTokens: 0, knownTokens: 58, unmodeled: true },
      instructions: [], mcps: [], contributors: [{ skill, status: 'active', reason: 'visible', candidates: [skill] }],
      compare: { columns: [{ root, skillCount: 1 }], rows: [] }, generatedAt: '',
    })),
  },
}))

describe('XrayView', () => {
  it('renders the backend report and exposes the three X-ray tabs', async () => {
    render(<XrayView root={root} onOpenSkill={() => undefined} />)
    expect((await screen.findAllByText('frontend-design')).length).toBeGreaterThan(0)
    expect(screen.getByRole('tab', { name: /上下文预算|Context budget/ })).toBeTruthy()
  })
})
