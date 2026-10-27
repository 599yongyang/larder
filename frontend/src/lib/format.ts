import type { LinkMode, Skill } from '../types'
import { getLanguage, t } from '../i18n'

export function shortPath(path: string) {
  const parts = path.split('/')
  return parts.length > 4 ? `…/${parts.slice(-3).join('/')}` : path
}

export function matrixGlyph(mode?: LinkMode) {
  return mode === 'linked' ? '↗' : mode === 'copied' ? '·' : mode === 'broken' ? '×' : mode === 'unavailable' ? '?' : '—'
}

export function formatTime(value: string) {
  const date = new Date(value)
  return Number.isNaN(date.getTime()) ? '' : date.toLocaleTimeString(getLanguage() === 'en' ? 'en-US' : 'zh-CN', { hour: '2-digit', minute: '2-digit' })
}

export function formatRelativeTime(value: string) {
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return ''
  const diff = Date.now() - date.getTime()
  const locale = getLanguage() === 'en' ? 'en-US' : 'zh-CN'
  const formatter = new Intl.RelativeTimeFormat(locale, { numeric: 'auto' })
  const minute = 60_000
  const hour = 60 * minute
  const day = 24 * hour
  if (Math.abs(diff) < minute) return formatter.format(-Math.round(diff / 1000), 'second')
  if (Math.abs(diff) < hour) return formatter.format(-Math.round(diff / minute), 'minute')
  if (Math.abs(diff) < day) return formatter.format(-Math.round(diff / hour), 'hour')
  return formatter.format(-Math.round(diff / day), 'day')
}

export function skillSourceLabel(skill: Pick<Skill, 'source' | 'origin' | 'originDetail'>) {
  if (skill.source) return skill.source
  if (skill.origin === 'plugin') return `${t('common.plugIn')} · ${skill.originDetail || t('common.unknownPlugIn')}`
  if (skill.origin === 'system') return `${t('common.system')} · ${skill.originDetail || t('common.builtIn')}`
  return t('common.noSource')
}

export function formatCompactNumber(value: number, language: 'zh' | 'en' = getLanguage()) {
  if (!Number.isFinite(value)) return '—'
  const sign = value < 0 ? '-' : ''
  const size = Math.abs(value)
  const compact = (divisor: number) => {
    const scaled = size / divisor
    const digits = scaled >= 100 ? 0 : 1
    return `${sign}${scaled.toFixed(digits).replace(/\.0+$/, '')}`
  }
  if (language === 'zh') {
    if (size >= 100_000_000) return `${compact(100_000_000)}亿`
    if (size >= 10_000) return `${compact(10_000)}万`
    return `${sign}${Math.round(size).toLocaleString('zh-CN')}`
  }
  if (size >= 1_000_000_000) return `${compact(1_000_000_000)}B`
  if (size >= 1_000_000) return `${compact(1_000_000)}M`
  if (size >= 1_000) return `${compact(1_000)}k`
  return `${sign}${Math.round(size).toLocaleString('en-US')}`
}
