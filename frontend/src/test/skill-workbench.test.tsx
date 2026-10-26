import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { test, expect, vi } from 'vitest'

window.matchMedia = window.matchMedia || ((query: string) => ({ matches: false, media: query, onchange: null, addListener: () => {}, removeListener: () => {}, addEventListener: () => {}, removeEventListener: () => {}, dispatchEvent: () => false }))
class ResizeObserverMock { observe() {} unobserve() {} disconnect() {} }
window.ResizeObserver = window.ResizeObserver || ResizeObserverMock as any
vi.mock('@wailsio/runtime', async (importOriginal) => {
  const actual = await (importOriginal as any)() as typeof import('@wailsio/runtime')
  return { ...actual, Events: { ...actual.Events, On: () => () => {} } }
})
vi.mock('../../bindings/events', () => ({ LarderState: 'larder:state' }))
vi.mock('../lib/deleteAnimation', () => ({ playDeleteAnimation: async () => {}, findSkillElement: () => null }))
const skills = [{ id: 's1', name: 'alpha', path: '/tmp/skills/alpha', rootId: 'global', rootKind: 'global', locked: false, hasSkillMd: true, description: 'desc', descTokens: 12, scanState: 'ok', isAgentLocal: false, isProject: false, copyDrift: false }]
const roots = [{ id: 'a1', label: 'Agent One', logicalAgent: 'Agent One', path: '/tmp/a1', kind: 'known_agent', known: true, status: 'available', entryCount: 1 }]
const links = [{ skillId: 's1', rootId: 'a1', mode: 'linked', drift: false, visible: true }]
vi.mock('../backend', () => ({
  backend: {
    state: async () => ({ snapshot: { schemaVersion: '1', status: 'complete', roots, skills, agentLinks: links, clis: [], issues: [], fromCache: false }, cached: false, scanning: false, lastError: '', updatedAt: '' }),
    settings: async () => ({ projectWorkspaces: [], ignoredAgents: [], showAgentBuiltins: false, usageStatsEnabled: false, autoCheckUpdates: false, updateCheckIntervalHours: 24 }),
    operations: async () => [], timeline: async () => [], usageStats: async () => [], mcp: async () => [], instructions: async () => [], commands: async () => [], appInfo: async () => ({ version: 'dev', schemaVersion: '1', goVersion: 'go', platform: 'darwin/arm64', dataDir: '/tmp', homeDir: '/tmp' }),
    checkUpdates: async () => ({ checkedAt: '', items: [], errors: [], scanned: 0, available: 0, current: 0, skillScanned: 0 }),
    skillAudits: async () => [],
    skillAudit: async () => ({ skillId: 's1', available: false, verdict: 'unreviewed', reason: 'not_audited', entries: [] }),
    skillContent: async () => '---\nname: alpha\ndescription: desc\n---\n\nbody',
    planSkillEdit: async () => ({ id: 'edit:test', skillId: 's1', path: '/tmp/skills/alpha/SKILL.md', content: '---\nname: alpha\ndescription: desc\n---\n\nbody', currentContent: '', exists: true, createdAt: '' }),
  },
}))
import App from '../App'
test('opens the inline Skill editor in the same sheet', async () => {
  render(<App />)
  await screen.findByText('alpha')
  await userEvent.click(screen.getAllByText('alpha')[0])
  const edit = await screen.findByRole('button', { name: '编辑 Skill' })
  await userEvent.click(edit)
  console.log('AFTER CLICK', document.body.textContent?.slice(-3000))
  expect(await screen.findByText('返回详情')).toBeTruthy()
  expect(screen.getByRole('textbox', { name: 'SKILL.md 内容' })).toBeTruthy()
  const sheet = document.querySelector('[data-slot="sheet-content"]') as HTMLElement
  expect(Number.parseInt(sheet.style.width, 10)).toBeGreaterThan(500)
})
