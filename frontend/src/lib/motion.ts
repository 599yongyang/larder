export type MotionMode = 'full' | 'subtle' | 'off'

const STORAGE_KEY = 'larder.motion'

export function getMotionMode(): MotionMode {
  try {
    const saved = localStorage.getItem(STORAGE_KEY)
    if (saved === 'full' || saved === 'subtle' || saved === 'off') return saved
  } catch {}
  return 'full'
}

export function setMotionMode(mode: MotionMode) {
  try { localStorage.setItem(STORAGE_KEY, mode) } catch {}
}
