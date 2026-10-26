import { describe, expect, it } from 'vitest'
import { lineDiff } from './lineDiff'

describe('lineDiff', () => {
  it('marks changed lines as deletions and additions', () => {
    const lines = lineDiff('one\ntwo\nthree', 'one\nTWO\nthree')
    expect(lines.map((line) => line.type)).toEqual(['context', 'deletion', 'addition', 'context'])
    expect(lines[1].content).toBe('two')
    expect(lines[2].content).toBe('TWO')
  })
})
