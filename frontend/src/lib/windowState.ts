import { Screens, Window } from '@wailsio/runtime'

const STORAGE_KEY = 'larder.window'
type Bounds = { x: number; y: number; width: number; height: number }
type ScreenBounds = { X: number; Y: number; Size: { Width: number; Height: number } }

function readBounds(): Bounds | undefined {
  try {
    const value = JSON.parse(localStorage.getItem(STORAGE_KEY) ?? 'null') as Partial<Bounds> | null
    if (!value || !Number.isFinite(value.width) || !Number.isFinite(value.height)) return undefined
    if ((value.width ?? 0) < 1080 || (value.height ?? 0) < 640) return undefined
    return { x: Number(value.x) || 0, y: Number(value.y) || 0, width: Number(value.width), height: Number(value.height) }
  } catch { return undefined }
}

function visibleArea(bounds: Bounds, screen: ScreenBounds) {
  const width = Math.max(0, Math.min(bounds.x + bounds.width, screen.X + screen.Size.Width) - Math.max(bounds.x, screen.X))
  const height = Math.max(0, Math.min(bounds.y + bounds.height, screen.Y + screen.Size.Height) - Math.max(bounds.y, screen.Y))
  return width * height
}

function centeredOn(bounds: Bounds, screen: ScreenBounds): Bounds {
  const width = Math.max(1080, Math.min(bounds.width, screen.Size.Width - 80))
  const height = Math.max(640, Math.min(bounds.height, screen.Size.Height - 80))
  return {
    width,
    height,
    x: screen.X + Math.round((screen.Size.Width - width) / 2),
    y: screen.Y + Math.round((screen.Size.Height - height) / 2),
  }
}

function browserScreen(): ScreenBounds {
  const screen = window.screen
  return {
    X: Number((screen as Screen & { availLeft?: number }).availLeft) || 0,
    Y: Number((screen as Screen & { availTop?: number }).availTop) || 0,
    Size: { Width: screen.availWidth || screen.width || 1440, Height: screen.availHeight || screen.height || 900 },
  }
}

async function normalizeBounds(bounds: Bounds): Promise<Bounds> {
  let screens: ScreenBounds[] = []
  try { screens = await Screens.GetAll() } catch {}
  if (screens.length === 0) screens = [browserScreen()]
  const visible = screens.some((screen) => visibleArea(bounds, screen) >= 160 * 80)
  if (!visible) {
    let primary: ScreenBounds = screens[0]
    try { primary = (await Screens.GetPrimary()) ?? primary } catch {}
    return centeredOn(bounds, primary)
  }
  // Keep the saved position, but do not let a window larger than the current
  // display grow beyond its bounds after a resolution change.
  const target = screens.reduce((best, screen) => visibleArea(bounds, screen) > visibleArea(bounds, best) ? screen : best, screens[0])
  const width = Math.max(1080, Math.min(bounds.width, target.Size.Width - 80))
  const height = Math.max(640, Math.min(bounds.height, target.Size.Height - 80))
  if (width === bounds.width && height === bounds.height) return bounds
  return { ...bounds, width, height }
}

export async function restoreWindowState() {
  const saved = readBounds()
  if (!saved) return
  try {
    const bounds = await normalizeBounds(saved)
    await Window.SetSize(bounds.width, bounds.height)
    await Window.SetPosition(bounds.x, bounds.y)
  } catch {}
}

export function watchWindowState() {
  let timer: number | undefined
  const save = () => {
    window.clearTimeout(timer)
    timer = window.setTimeout(async () => {
      try {
        const [size, position] = await Promise.all([Window.Size(), Window.Position()])
        localStorage.setItem(STORAGE_KEY, JSON.stringify({ ...position, ...size }))
      } catch {}
    }, 200)
  }
  window.addEventListener('resize', save)
  window.addEventListener('focus', save)
  return () => {
    window.clearTimeout(timer)
    window.removeEventListener('resize', save)
    window.removeEventListener('focus', save)
  }
}
