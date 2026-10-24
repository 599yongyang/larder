import { getMotionMode } from './motion'

const GLYPHS = 'ABCDEFGHJKLMNPQRSTUVWXYZ'
const MAX_NODES = 80
const FRAME_MS = 36
const DURATION_MS = 380

type Run = { text: string; node: Text; started: number; delay: number; chars: string[] }

let generation = 0
const activeRuns = new Set<Run>()

function visible(node: Text) {
  const parent = node.parentElement
  if (!parent) return false
  if (['SCRIPT', 'STYLE', 'TEXTAREA', 'INPUT', 'OPTION', 'CODE', 'PRE'].includes(parent.tagName)) return false
  if (parent.closest('input, textarea, select, code, pre, [contenteditable="true"], [data-no-language-motion]')) return false
  const value = node.nodeValue?.trim()
  if (!value || value.length < 2 || value.length > 80) return false
  const rect = parent.getBoundingClientRect()
  return rect.width > 0 && rect.height > 0 && rect.bottom > 0 && rect.top < window.innerHeight && rect.right > 0 && rect.left < window.innerWidth
}

function glyph(index: number) {
  return GLYPHS[(index * 17 + Math.floor(Math.random() * GLYPHS.length)) % GLYPHS.length]
}

function restoreAll() {
  for (const run of activeRuns) run.node.nodeValue = run.text
  activeRuns.clear()
  document.body.removeAttribute('aria-busy')
}

export function animateLanguageText() {
  const mode = getMotionMode()
  if (mode === 'off') return
  if (mode === 'subtle') { animateLanguageSubtle(); return }
  animateLanguageFull()
}

function animateLanguageFull() {
  if (typeof window.matchMedia === 'function' && window.matchMedia('(prefers-reduced-motion: reduce)').matches) return
  restoreAll()
  const current = ++generation
  const walker = document.createTreeWalker(document.body, NodeFilter.SHOW_TEXT)
  const runs: Run[] = []
  let node: Node | null
  while ((node = walker.nextNode()) && runs.length < MAX_NODES) {
    if (!(node instanceof Text) || !visible(node)) continue
    const text = node.nodeValue ?? ''
    runs.push({ text, node, started: 0, delay: Math.min(runs.length * 5, 60), chars: Array.from(text) })
  }
  if (runs.length === 0) return

  const started = performance.now()
  for (const run of runs) {
    activeRuns.add(run)
    // Start every block fully scrambled, but reveal with an ease-out curve so
    // the text becomes readable early and the tail settles gently.
    run.node.nodeValue = run.chars.map((char, index) => /\s/.test(char) ? char : glyph(index)).join('')
  }
  document.body.setAttribute('aria-busy', 'true')

  const tick = (now: number) => {
    if (current !== generation) return
    const next: Run[] = []
    for (const run of runs) {
      const progress = (now - started - run.delay) / DURATION_MS
      if (progress >= 1) {
        run.node.nodeValue = run.text
        activeRuns.delete(run)
        continue
      }
      if (progress <= 0) {
        next.push(run)
        continue
      }
      const eased = 1 - Math.pow(1 - progress, 2.2)
      const revealed = Math.floor(eased * run.chars.length)
      let out = ''
      for (let index = 0; index < run.chars.length; index += 1) {
        const char = run.chars[index]
        out += index < revealed || /\s/.test(char) ? char : glyph(index)
      }
      run.node.nodeValue = out
      next.push(run)
    }
    if (next.length === 0) {
      document.body.removeAttribute('aria-busy')
      return
    }
    window.setTimeout(() => tick(performance.now()), FRAME_MS)
  }
  tick(performance.now())
}


function animateLanguageSubtle() {
  if (typeof window.matchMedia === 'function' && window.matchMedia('(prefers-reduced-motion: reduce)').matches) return
  if (typeof Element === 'undefined' || typeof Element.prototype.animate !== 'function') return
  const parents = new Set<HTMLElement>()
  const walker = document.createTreeWalker(document.body, NodeFilter.SHOW_TEXT)
  let node: Node | null
  while ((node = walker.nextNode()) && parents.size < 40) {
    if (!(node instanceof Text) || !visible(node)) continue
    const parent = node.parentElement
    if (parent) parents.add(parent)
  }
  Array.from(parents).forEach((element, index) => {
    const animation = element.animate(
      [
        { opacity: .55, transform: 'translateY(5px)' },
        { opacity: 1, transform: 'translateY(0)' },
      ],
      { duration: 220, delay: Math.min(index * 10, 100), easing: 'cubic-bezier(.2,0,0,1)', fill: 'both' },
    )
    void animation.finished.then(() => animation.cancel()).catch(() => {})
  })
}
