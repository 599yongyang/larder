import { Window } from '@wailsio/runtime'

const STORAGE_KEY = 'larder.zoom'
export const zoomLevels = [100, 110, 120] as const
export type ZoomLevel = (typeof zoomLevels)[number]

export function getZoom(): ZoomLevel {
  try {
    const saved = Number(localStorage.getItem(STORAGE_KEY))
    if (zoomLevels.includes(saved as ZoomLevel)) return saved as ZoomLevel
  } catch {}
  return 100
}

export function setZoom(value: ZoomLevel) {
  try { localStorage.setItem(STORAGE_KEY, String(value)) } catch {}
  try { window.dispatchEvent(new CustomEvent('larder:zoom', { detail: value })) } catch {}
}

export function cycleZoom(direction: 1 | -1) {
  const current = getZoom()
  const index = zoomLevels.indexOf(current)
  const next = zoomLevels[Math.max(0, Math.min(zoomLevels.length - 1, index + direction))]
  if (next === current) return
  setZoom(next)
  void applyZoom(next)
}

export function resetZoom() {
  setZoom(100)
  void applyZoom(100)
}

export async function applyZoom(value: ZoomLevel) {
  try {
    await Window.SetZoom(value / 100)
  } catch {
    try { document.documentElement.style.zoom = String(value / 100) } catch {}
  }
}

export async function restoreZoom() {
  await applyZoom(getZoom())
}
