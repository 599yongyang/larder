import { describe, expect, it } from 'vitest'
import { formatTime, matrixGlyph, shortPath, skillSourceLabel } from './format'

describe('format helpers', () => {
  it('shortens deep paths', () => {
    expect(shortPath('/a/b/c/d/e/f')).toBe('…/d/e/f')
    expect(shortPath('/a/b')).toBe('/a/b')
  })

  it('maps link modes to glyphs', () => {
    expect(matrixGlyph('linked')).toBe('↗')
    expect(matrixGlyph('broken')).toBe('×')
    expect(matrixGlyph(undefined)).toBe('—')
  })

  it('labels plugin and system origins as sources', () => {
    expect(skillSourceLabel({ source: 'owner/repo', origin: 'plugin', originDetail: 'antislop' })).toBe('owner/repo')
    expect(skillSourceLabel({ source: '', origin: 'plugin', originDetail: 'antislop' })).toBe('插件 · antislop')
    expect(skillSourceLabel({ source: '', origin: 'system' })).toBe('系统 · 内置')
  })

  it('handles invalid times', () => {
    expect(formatTime('not-a-date')).toBe('')
  })
})
