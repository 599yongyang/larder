import { isEnglish } from '../i18n'
import type { Root } from '../types'

export function sortAgentRoots(roots: Root[]): Root[] {
  const language = isEnglish() ? 'en' : 'zh-CN'
  return [...roots].sort((a, b) => {
    return (a.logicalAgent || a.label).localeCompare(b.logicalAgent || b.label, language, { sensitivity: 'base' })
  })
}
