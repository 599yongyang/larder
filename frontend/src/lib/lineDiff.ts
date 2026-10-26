export type SimpleDiffLine = { content: string; newNumber?: number; oldNumber?: number; type: 'addition' | 'context' | 'deletion' }

// lineDiff is intentionally small: it keeps the common prefix and suffix and
// marks the changed middle. That is enough for a SKILL.md edit preview and
// avoids sending large files through a full LCS.
export function lineDiff(before: string, after: string): SimpleDiffLine[] {
  if (before === after) return before.split('\n').map((content, index) => ({ content, oldNumber: index + 1, newNumber: index + 1, type: 'context' as const }))
  const a = before.split('\n')
  const b = after.split('\n')
  let prefix = 0
  while (prefix < a.length && prefix < b.length && a[prefix] === b[prefix]) prefix++
  let suffix = 0
  while (suffix < a.length - prefix && suffix < b.length - prefix && a[a.length - 1 - suffix] === b[b.length - 1 - suffix]) suffix++
  const lines: SimpleDiffLine[] = []
  for (let index = 0; index < prefix; index++) lines.push({ content: a[index], oldNumber: index + 1, newNumber: index + 1, type: 'context' })
  for (let index = prefix; index < a.length - suffix; index++) lines.push({ content: a[index], oldNumber: index + 1, type: 'deletion' })
  for (let index = prefix; index < b.length - suffix; index++) lines.push({ content: b[index], newNumber: index + 1, type: 'addition' })
  for (let index = a.length - suffix; index < a.length; index++) lines.push({ content: a[index], oldNumber: index + 1, newNumber: b.length - (a.length - index), type: 'context' })
  return lines.slice(0, 1200)
}
