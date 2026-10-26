import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { expect, test, vi } from 'vitest'

const installUpstreamSkills = vi.hoisted(() => vi.fn(async () => []))
vi.mock('../backend', () => ({ backend: { installUpstreamSkills, planInstallSkill: vi.fn() } }))

import { NewUpstreamSkillsDialog } from './NewUpstreamSkillsDialog'

const items = [
  { kind: 'skill' as const, owner: 'github', package: 'owner/repo', name: 'alpha', description: 'Alpha skill', skillPath: 'skills/alpha', state: 'new' as const },
  { kind: 'skill' as const, owner: 'github', package: 'owner/repo', name: 'beta', description: 'Beta skill', skillPath: 'skills/beta', state: 'new' as const },
]

test('installs selected upstream skills in one batch', async () => {
  const onClose = vi.fn()
  const onInstalled = vi.fn()
  render(<NewUpstreamSkillsDialog items={items} onClose={onClose} onPlan={() => {}} onInstalled={onInstalled} />)
  const checkboxes = screen.getAllByRole('checkbox')
  await userEvent.click(checkboxes[1])
  await userEvent.click(screen.getByRole('button', { name: /安装选中/ }))
  await waitFor(() => expect(installUpstreamSkills).toHaveBeenCalledWith('owner/repo', [{ name: 'alpha', skillPath: 'skills/alpha/SKILL.md' }]))
  expect(onInstalled).toHaveBeenCalled()
  expect(onClose).toHaveBeenCalled()
})
