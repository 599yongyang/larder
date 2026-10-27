import { describe, expect, it } from 'vitest'
import { findAgentCLI } from './agentCLI'
import type { CLIInfo, Root } from '../types'

const cli = (name: string, version: string): CLIInfo => ({ id: name, name, path: `/usr/local/bin/${name}`, owner: 'npm', version, status: 'ok', duplicateCount: 0, scannedAt: '' })
const root = (id: string, logicalAgent: string): Root => ({ id, label: logicalAgent, logicalAgent, path: `/tmp/${id}`, kind: 'known_agent', known: true, status: 'available', entryCount: 1 })

describe('findAgentCLI', () => {
  it('does not match a short Agent name inside an unrelated CLI', () => {
    const omp = root('omp', 'OMP')
    expect(findAgentCLI(omp, [cli('docker-compose', '2.0.0'), cli('omp', '18.13.1')])?.name).toBe('omp')
  })

  it('maps known Agent roots to their CLI executable', () => {
    expect(findAgentCLI(root('claude-code', 'Claude Code'), [cli('claude', '2.0.0')])?.version).toBe('2.0.0')
    expect(findAgentCLI(root('codex', 'Codex'), [cli('codex', '1.1.0')])?.name).toBe('codex')
    expect(findAgentCLI(root('gemini-cli', 'Gemini CLI'), [cli('gemini', '1.2.0')])?.name).toBe('gemini')
  })
})
