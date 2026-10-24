import { flushSync } from 'react-dom'
import { applyTheme } from './theme'
import type { ResolvedTheme, ThemeMode } from './theme'

type Origin = { x: number; y: number }
type ViewTransition = { ready: Promise<void>; finished: Promise<void> }

export function switchTheme(mode: ThemeMode, origin: Origin | undefined, system: ResolvedTheme, setMode: (mode: ThemeMode) => void) {
  const reduceMotion = typeof window.matchMedia === 'function' && window.matchMedia('(prefers-reduced-motion: reduce)').matches
  const startViewTransition = (document as Document & { startViewTransition?: (callback: () => void) => ViewTransition }).startViewTransition
  if (reduceMotion || typeof startViewTransition !== 'function') {
    setMode(mode)
    applyTheme(mode, system)
    return
  }
  const x = origin?.x ?? window.innerWidth / 2
  const y = origin?.y ?? window.innerHeight / 2
  const radius = Math.hypot(Math.max(x, window.innerWidth - x), Math.max(y, window.innerHeight - y))
  const transition = startViewTransition.call(document, () => {
    flushSync(() => {
      setMode(mode)
      applyTheme(mode, system)
    })
  })
  transition.ready.then(() => {
    document.documentElement.animate(
      { clipPath: [`circle(0px at ${x}px ${y}px)`, `circle(${radius}px at ${x}px ${y}px)`] },
      { duration: 460, easing: 'cubic-bezier(.2,0,0,1)', pseudoElement: '::view-transition-new(root)' }
    )
  }).catch(() => {})
}
