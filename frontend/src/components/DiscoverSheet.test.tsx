import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import { beforeEach, expect, test, vi } from 'vitest'
import { DiscoverSheet } from './DiscoverSheet'
import type { Root, Skill } from '../types'

const catalogSkills = vi.fn()
const searchCatalogSkills = vi.fn()
const catalogInstalls = vi.fn()
const catalogDescription = vi.fn()

vi.mock('../backend', () => ({
  backend: {
    catalogSkills: (...args: unknown[]) => catalogSkills(...args),
    searchCatalogSkills: (...args: unknown[]) => searchCatalogSkills(...args),
    catalogInstalls: () => catalogInstalls(),
    catalogDescription: (...args: unknown[]) => catalogDescription(...args),
  },
}))

class ObserverMock {
  callback: IntersectionObserverCallback
  constructor(callback: IntersectionObserverCallback) { this.callback = callback }
  observe(target: Element) { this.callback([{ isIntersecting: true, target } as IntersectionObserverEntry], this as unknown as IntersectionObserver) }
  unobserve() {}
  disconnect() {}
  takeRecords() { return [] }
  get root() { return null }
  get rootMargin() { return '' }
  get thresholds() { return [] }
}
Object.defineProperty(window, 'IntersectionObserver', { writable: true, value: ObserverMock })

const card = (name: string, source = 'acme/skills') => ({ id: `${source}/${name}`, source, skillId: name, name, installs: 1200 })
const roots: Root[] = [{ id: 'r1', label: 'Claude Code', path: '/tmp/claude', kind: 'known_agent', known: true, status: 'available', entryCount: 0 }]
const installedSkill: Skill = { id: 's1', name: 'pdf', path: '/tmp/skills/pdf', rootId: 'global', rootKind: 'global', locked: false, hasSkillMd: true, descTokens: 1, scanState: 'ok', isAgentLocal: false, isProject: false, copyDrift: false }

beforeEach(() => {
  catalogSkills.mockReset().mockResolvedValue([card('pdf')])
  searchCatalogSkills.mockReset()
  catalogInstalls.mockReset().mockResolvedValue([])
  catalogDescription.mockReset().mockResolvedValue('PDF helper')
})

test('renders popular cards and marks installed skills', async () => {
  catalogInstalls.mockResolvedValue([{ path: '/tmp/skills/pdf', source: 'acme/skills', skillId: 'pdf' }])
  render(<DiscoverSheet open roots={roots} installedSkills={[installedSkill]} onClose={() => {}} onInstalled={() => {}} />)
  expect(await screen.findByText('pdf')).toBeTruthy()
  expect(screen.getByText('已安装')).toBeTruthy()
  expect(screen.queryByRole('button', { name: '安装' })).toBeNull()
})

test('opens the local skill from an installed card', async () => {
  catalogInstalls.mockResolvedValue([{ path: '/tmp/skills/pdf', source: 'acme/skills', skillId: 'pdf' }])
  const onOpenInstalled = vi.fn()
  render(<DiscoverSheet open roots={roots} installedSkills={[installedSkill]} onClose={() => {}} onInstalled={() => {}} onOpenInstalled={onOpenInstalled} />)
  await screen.findByText('pdf')
  fireEvent.click(screen.getByText('pdf').closest('[role="button"]')!)
  expect(onOpenInstalled).toHaveBeenCalledWith('s1')
})

test('only accepts the latest search response', async () => {
  let resolveFirst: (value: unknown) => void = () => {}
  let resolveSecond: (value: unknown) => void = () => {}
  const first = new Promise((resolve) => { resolveFirst = resolve })
  const second = new Promise((resolve) => { resolveSecond = resolve })
  searchCatalogSkills.mockReturnValueOnce(first).mockReturnValueOnce(second)
  render(<DiscoverSheet open roots={roots} installedSkills={[]} onClose={() => {}} onInstalled={() => {}} />)
  const input = await screen.findByLabelText('搜索 Skills')
  fireEvent.change(input, { target: { value: 'aa' } })
  fireEvent.submit(input.closest('form')!)
  fireEvent.change(input, { target: { value: 'bb' } })
  fireEvent.submit(input.closest('form')!)
  resolveSecond([card('beta')])
  expect(await screen.findByText('beta')).toBeTruthy()
  resolveFirst([card('alpha')])
  await waitFor(() => expect(screen.queryByText('alpha')).toBeNull())
})
