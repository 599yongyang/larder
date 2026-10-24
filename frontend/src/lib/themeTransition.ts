import { applyTheme } from './theme'
import type { ResolvedTheme, ThemeMode } from './theme'
import { getMotionMode } from './motion'

type Origin = { x: number; y: number }
type ViewTransition = { ready: Promise<void>; finished: Promise<void> }

export function switchTheme(mode: ThemeMode, origin: Origin | undefined, system: ResolvedTheme, setMode: (mode: ThemeMode) => void) {
  const reduceMotion = typeof window.matchMedia === 'function' && window.matchMedia('(prefers-reduced-motion: reduce)').matches
  const startViewTransition = (document as Document & { startViewTransition?: (callback: () => void) => ViewTransition }).startViewTransition
  const motion = getMotionMode()
  const apply = () => {
    setMode(mode)
    applyTheme(mode, system)
  }
  if (motion === 'off' || reduceMotion || typeof startViewTransition !== 'function') {
    apply()
    return
  }

  const x = origin?.x ?? window.innerWidth / 2
  const y = origin?.y ?? window.innerHeight / 2
  const radius = Math.hypot(Math.max(x, window.innerWidth - x), Math.max(y, window.innerHeight - y))
  const transition = startViewTransition.call(document, apply)
  transition.ready.then(() => {
    document.documentElement.animate(
      { clipPath: [`circle(0px at ${x}px ${y}px)`, `circle(${radius}px at ${x}px ${y}px)`] },
      { duration: motion === 'subtle' ? 220 : 460, easing: 'cubic-bezier(.2,0,0,1)', pseudoElement: '::view-transition-new(root)' }
    )
  }).catch(() => {})
}
