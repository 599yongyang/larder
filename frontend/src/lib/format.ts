import type { LinkMode, Skill } from '../types'

export function shortPath(path: string) {
  const parts = path.split('/')
  return parts.length > 4 ? `…/${parts.slice(-3).join('/')}` : path
}

export function matrixGlyph(mode?: LinkMode) {
  return mode === 'linked' ? '↗' : mode === 'copied' ? '·' : mode === 'broken' ? '×' : mode === 'unavailable' ? '?' : '—'
}

export function formatTime(value: string) {
  const date = new Date(value)
  return Number.isNaN(date.getTime()) ? '' : date.toLocaleTimeString('zh-CN', { hour: '2-digit', minute: '2-digit' })
}

export function skillSourceLabel(skill: Pick<Skill, 'source' | 'origin' | 'originDetail'>) {
  if (skill.source) return skill.source
  if (skill.origin === 'plugin') return `插件 · ${skill.originDetail || '未知插件'}`
  if (skill.origin === 'system') return `系统 · ${skill.originDetail || '内置'}`
  return '无来源'
}
