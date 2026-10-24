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
const skills = [{ id: 's1', name: 'alpha', path: '/tmp/skills/alpha', rootId: 'global', rootKind: 'global', locked: false, hasSkillMd: true, description: 'desc', descTokens: 12, scanState: 'ok', isAgentLocal: false, isProject: false, copyDrift: false }]
const roots = [{ id: 'a1', label: 'Agent One', logicalAgent: 'Agent One', path: '/tmp/a1', kind: 'known_agent', known: true, status: 'available', entryCount: 1 }]
const links = [{ skillId: 's1', rootId: 'a1', mode: 'linked', drift: false, visible: true }]
vi.mock('../backend', () => ({
  backend: {
    state: async () => ({ snapshot: { schemaVersion: '1', status: 'complete', roots, skills, agentLinks: links, clis: [], issues: [], fromCache: false }, cached: false, scanning: false, lastError: '', updatedAt: '' }),
    settings: async () => ({ projectWorkspaces: [], ignoredAgents: [], showAgentBuiltins: false, usageStatsEnabled: false, autoCheckUpdates: false, updateCheckIntervalHours: 24 }),
    operations: async () => [], timeline: async () => [], usageStats: async () => [], mcp: async () => [], instructions: async () => [], appInfo: async () => ({ version: 'dev', schemaVersion: '1', goVersion: 'go', platform: 'darwin/arm64', dataDir: '/tmp', homeDir: '/tmp' }),
    checkUpdates: async () => ({ checkedAt: '', items: [], errors: [], scanned: 0, available: 0, current: 0, skillScanned: 0 }),
    planUninstallSkill: async () => ({ id: 'plan:test', operation: 'uninstall_skill', title: '卸载 Skill', summary: 'test', skillId: 's1', skillName: 'alpha', changes: [], requiresSnapshot: true, executable: true, createdAt: '' }),
  },
}))
import App from '../App'
test('delete asks for plan confirmation', async () => {
  render(<App />)
  await screen.findByText('alpha')
  await userEvent.click(screen.getByRole('button', { name: '卸载 Skill' }))
  expect(await screen.findByRole('dialog')).toBeTruthy()
})
