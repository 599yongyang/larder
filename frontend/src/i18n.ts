import i18next from 'i18next'
import en from './locales/en'
import zh from './locales/zh-CN'
import dynamicValues from './locales/dynamic'

export type Language = 'zh' | 'en'
export type LanguageSetting = 'system' | Language

const STORAGE_KEY = 'larder.language'

export const languages = [
  { value: 'system', label: 'app.system' },
  { value: 'zh', label: '中文' },
  { value: 'en', label: 'English' },
] as const satisfies ReadonlyArray<{ value: LanguageSetting; label: string }>

function systemLanguage(): Language { return navigator.language.toLowerCase().startsWith('zh') ? 'zh' : 'en' }
let setting: LanguageSetting = 'system'
try {
  const saved = localStorage.getItem(STORAGE_KEY)
  if (saved === 'en' || saved === 'zh' || saved === 'system') setting = saved
} catch {}
let current: Language = setting === 'system' ? systemLanguage() : setting
try { document.documentElement.lang = current === 'en' ? 'en' : 'zh-CN' } catch {}

const listeners = new Set<() => void>()
export function subscribeLanguage(listener: () => void) {
  listeners.add(listener)
  return () => { listeners.delete(listener) }
}
function notifyLanguage() { for (const listener of listeners) listener() }

export function getLanguage(): Language { return current }
export function getLanguageSetting(): LanguageSetting { return setting }
export function isEnglish(): boolean { return current === 'en' }
void i18next.init({
  lng: current,
  fallbackLng: { zh: ['zh'], default: ['en'] },
  supportedLngs: ['zh', 'en'],
  resources: {
    en: { translation: en },
    zh: { translation: zh },
  },
  initAsync: false,
  keySeparator: false,
  nsSeparator: false,
  interpolation: { prefix: '{', suffix: '}', escapeValue: false },
  returnNull: false,
  returnEmptyString: false,
})

export function setLanguage(next: LanguageSetting) {
  setting = next
  current = next === 'system' ? systemLanguage() : next
  try {
    if (next === 'system') localStorage.removeItem(STORAGE_KEY)
    else localStorage.setItem(STORAGE_KEY, next)
  } catch {}
  try { document.documentElement.lang = current === 'en' ? 'en' : 'zh-CN' } catch {}
  void i18next.changeLanguage(current).then(notifyLanguage)
}

export function t(key: string, vars?: Record<string, string | number>): string {
  const value = i18next.t(key, vars as Record<string, unknown>)
  return typeof value === 'string' ? value : key
}

/** Translate a value that comes from the Go backend (legacy string value). */
export function tValue(value: string, vars?: Record<string, string | number>): string {
  return t(dynamicValues[value] ?? value, vars)
}
