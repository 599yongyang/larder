import { flushSync } from 'react-dom'

type ViewTransition = { ready: Promise<void>; finished: Promise<void> }
type TransitionDocument = Document & { startViewTransition?: (callback: () => void) => ViewTransition }

export function pageTransition(update: () => void) {
  const reduceMotion = typeof window.matchMedia === 'function' && window.matchMedia('(prefers-reduced-motion: reduce)').matches
  const startViewTransition = (document as TransitionDocument).startViewTransition
  if (reduceMotion || typeof startViewTransition !== 'function') {
    update()
    return
  }
  const root = document.documentElement
  root.classList.add('page-transitioning')
  try {
    const transition = startViewTransition.call(document, () => flushSync(update))
    transition.ready.catch(() => {})
    transition.finished.catch(() => {}).finally(() => root.classList.remove('page-transitioning'))
  } catch {
    root.classList.remove('page-transitioning')
    update()
  }
}
