import { render, screen } from '@testing-library/react'
import { describe, expect, it, vi } from 'vitest'
import { SkillAuditInline } from './SkillAuditSection'

vi.mock('../backend', () => ({
  backend: {
    skillAudit: async () => ({ skillId: 's1', source: 'owner/repo', skill: 'alpha', available: true, verdict: 'warn', riskLevel: 'MEDIUM', entries: [{ provider: 'Socket', status: 'pass' }, { provider: 'Snyk', status: 'warn' }] }),
  },
}))

describe('SkillAuditInline', () => {
  it('shows the upstream verdict and provider count', async () => {
    render(<SkillAuditInline skillID="s1" />)
    expect(await screen.findByText('安全审计：建议复核')).toBeTruthy()
    expect(screen.getByText('中风险')).toBeTruthy()
  })
})
