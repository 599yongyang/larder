export type ThemeMode = 'system' | 'light' | 'dark'
export type ResolvedTheme = 'light' | 'dark'

const STORAGE_KEY = 'larder-theme-mode'
const LEGACY_KEY = 'aam-theme'

export function systemTheme(): ResolvedTheme {
  if (typeof window === 'undefined' || typeof window.matchMedia !== 'function') return 'light'
  return window.matchMedia('(prefers-color-scheme: dark)').matches ? 'dark' : 'light'
}

export function getThemeMode(): ThemeMode {
  const stored = typeof localStorage === 'undefined' ? null : localStorage.getItem(STORAGE_KEY)
  if (stored === 'system' || stored === 'light' || stored === 'dark') return stored
  const legacy = typeof localStorage === 'undefined' ? null : localStorage.getItem(LEGACY_KEY)
  if (legacy === 'light' || legacy === 'dark') return legacy
  return 'system'
}

export function resolveTheme(mode: ThemeMode, currentSystem = systemTheme()): ResolvedTheme {
  return mode === 'system' ? currentSystem : mode
}

export function applyTheme(mode: ThemeMode, currentSystem = systemTheme()): ResolvedTheme {
  const resolved = resolveTheme(mode, currentSystem)
  if (typeof document !== 'undefined') {
    document.documentElement.dataset.theme = resolved
    document.documentElement.classList.toggle('dark', resolved === 'dark')
  }
  if (typeof localStorage !== 'undefined') localStorage.setItem(STORAGE_KEY, mode)
  return resolved
}

export function nextThemeMode(mode: ThemeMode): ThemeMode {
  return mode === 'system' ? 'light' : mode === 'light' ? 'dark' : 'system'
}
