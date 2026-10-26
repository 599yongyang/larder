import { expect, test } from 'vitest'
import { sortAgentRoots } from '../lib/agentOrder'
import type { Root } from '../types'

const root = (label: string, kind: Root['kind'], status: Root['status']): Root => ({ id: label, label, kind, status, path: `/tmp/${label}`, known: kind === 'known_agent', entryCount: 1 })

test('sorts Agents alphabetically by logical name', () => {
  const sorted = sortAgentRoots([root('Zed', 'unknown_agent', 'available'), root('Codex', 'known_agent', 'available'), root('Cursor', 'known_agent', 'unavailable'), root('Claude Code', 'known_agent', 'available')])
  expect(sorted.map((item) => item.label)).toEqual(['Claude Code', 'Codex', 'Cursor', 'Zed'])
})
