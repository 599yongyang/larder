import { isEnglish } from '../i18n'
import type { Root } from '../types'

export function sortAgentRoots(roots: Root[]): Root[] {
  const language = isEnglish() ? 'en' : 'zh-CN'
  return [...roots].sort((a, b) => {
    const known = Number(b.kind === 'known_agent') - Number(a.kind === 'known_agent')
    if (known !== 0) return known
    const available = Number(b.status === 'available') - Number(a.status === 'available')
    if (available !== 0) return available
    return (a.logicalAgent || a.label).localeCompare(b.logicalAgent || b.label, language)
  })
}
