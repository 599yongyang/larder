export function isMacPlatform() {
  if (typeof navigator === 'undefined') return false
  return /Mac|iPhone|iPad/.test(navigator.platform || navigator.userAgent)
}

export function isWindowsPlatform() {
  if (typeof navigator === 'undefined') return false
  return /Win/i.test(navigator.platform || navigator.userAgent)
}

export function modifierLabel() {
  return isMacPlatform() ? '⌘' : 'Ctrl'
}

export function shortcutLabel(key: string) {
  return isMacPlatform() ? `⌘${key}` : `Ctrl+${key}`
}

export function ariaShortcut(key: string) {
  return isMacPlatform() ? `Meta+${key}` : `Control+${key}`
}
