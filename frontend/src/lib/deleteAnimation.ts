import { toCanvas } from 'html-to-image'
import { getMotionMode, type MotionMode } from './motion'

const easeOut = 'cubic-bezier(.23,1,.32,1)'
const reduceMotion = () => typeof window.matchMedia === 'function' && window.matchMedia('(prefers-reduced-motion: reduce)').matches

function wait(ms: number) {
  return new Promise<void>((resolve) => setTimeout(resolve, ms))
}

async function ashDissolve(element: HTMLElement) {
  const rect = element.getBoundingClientRect()
  if (rect.width === 0 || rect.height === 0) return
  const canvas = document.createElement('canvas')
  const ratio = Math.min(window.devicePixelRatio || 1, 2)
  canvas.width = Math.round(window.innerWidth * ratio)
  canvas.height = Math.round(window.innerHeight * ratio)
  canvas.style.cssText = `position:fixed;inset:0;width:100vw;height:100vh;pointer-events:none;z-index:9999`
  document.body.appendChild(canvas)
  const context = canvas.getContext('2d')
  if (!context) { canvas.remove(); return }
  const ctx = context
  ctx.scale(ratio, ratio)
  const particles = Array.from({ length: 120 }, () => {
    const x = rect.left + Math.random() * rect.width
    const y = rect.top + Math.random() * rect.height
    return {
      x,
      y,
      wind: 95 + Math.random() * 190,
      lift: -12 + Math.random() * 26,
      turbulence: 3 + Math.random() * 10,
      size: 0.5 + Math.random() * 1.8,
      delay: Math.random() * 180,
      alpha: 0.25 + Math.random() * 0.55,
    }
  })
  const started = performance.now()
  const windFade = element.tagName === 'TR'
    ? [{ opacity: 1, filter: 'blur(0px)' }, { opacity: 0.7, filter: 'blur(1px)', offset: 0.4 }, { opacity: 0, filter: 'blur(8px)' }]
    : [
        { opacity: 1, transform: 'translateX(0) scale(1)', filter: 'blur(0px)', clipPath: 'inset(0 0 0 0)' },
        { opacity: 0.68, transform: 'translateX(16px) scale(0.995)', filter: 'blur(1px)', clipPath: 'inset(0 0 0 12%)', offset: 0.42 },
        { opacity: 0, transform: 'translateX(38px) scale(0.98)', filter: 'blur(8px)', clipPath: 'inset(0 0 0 100%)' },
      ]
  const fade = element.animate(windFade, { duration: 820, easing: easeOut, fill: 'forwards' })
  await new Promise<void>((resolve) => {
    function frame(now: number) {
      const elapsed = now - started
      ctx.clearRect(0, 0, window.innerWidth, window.innerHeight)
      let alive = false
      for (const particle of particles) {
        const t = Math.max(0, elapsed - particle.delay) / 1120
        if (t >= 1) continue
        alive = true
        const progress = Math.min(1, t)
        const wind = progress * progress
        const x = particle.x + particle.wind * wind + Math.sin((progress + particle.delay) * 0.018) * particle.turbulence * progress
        const y = particle.y + particle.lift * progress + 30 * progress * progress
        const tail = (1.5 + progress * 5) * (0.6 + particle.size)
        ctx.globalAlpha = particle.alpha * (1 - progress)
        ctx.fillStyle = progress < 0.42 ? 'rgba(166,160,150,.9)' : 'rgba(116,111,104,.65)'
        ctx.beginPath()
        ctx.ellipse(x, y, tail, particle.size * 0.55, 0, 0, Math.PI * 2)
        ctx.fill()
        if (particle.size > 1.4) {
          ctx.globalAlpha *= 0.5
          ctx.fillRect(x - tail * 0.4, y - 0.5, tail * 0.8, 0.6)
        }
      }
      if (alive) requestAnimationFrame(frame)
      else resolve()
    }
    requestAnimationFrame(frame)
  })
  await fade.finished.catch(() => undefined)
  canvas.remove()
  element.style.visibility = 'hidden'
}

async function subtleExit(element: HTMLElement) {
  const animation = element.animate(
    [
      { opacity: 1 },
      { opacity: 0 },
    ],
    { duration: 220, easing: easeOut, fill: 'forwards' },
  )
  await animation.finished.catch(() => undefined)
  element.style.visibility = 'hidden'
}

type Texture = {
  image: HTMLCanvasElement
  rect: DOMRect
}

async function captureTexture(element: HTMLElement): Promise<Texture | null> {
  const rect = element.getBoundingClientRect()
  if (rect.width < 8 || rect.height < 8) return null
  const image = await toCanvas(element, {
    cacheBust: true,
    pixelRatio: Math.min(window.devicePixelRatio || 1, 2),
    skipFonts: false,
  })
  if (!image.width || !image.height) return null
  return { image, rect }
}

function sampleTexture(texture: Texture) {
  const { image, rect } = texture
  const maxWidth = 190
  const scale = Math.min(1, maxWidth / image.width)
  const width = Math.max(1, Math.round(image.width * scale))
  const height = Math.max(1, Math.round(image.height * scale))
  const sample = document.createElement('canvas')
  sample.width = width
  sample.height = height
  const context = sample.getContext('2d', { willReadFrequently: true })
  if (!context) return []
  context.drawImage(image, 0, 0, width, height)
  const pixels = context.getImageData(0, 0, width, height).data
  const corner = (x: number, y: number) => {
    const index = (y * width + x) * 4
    return { r: pixels[index], g: pixels[index + 1], b: pixels[index + 2], a: pixels[index + 3] }
  }
  const corners = [corner(1, 1), corner(width - 2, 1), corner(1, height - 2), corner(width - 2, height - 2)]
  const opaque = corners.filter((color) => color.a > 180)
  const background = opaque.length ? opaque[Math.floor(opaque.length / 2)] : { r: 255, g: 255, b: 255 }
  const points: Array<{ x: number; y: number; color: string; size: number; delay: number }> = []
  for (let y = 2; y < height - 1; y += 2) {
    for (let x = 1; x < width - 1; x += 2) {
      const index = (y * width + x) * 4
      const alpha = pixels[index + 3]
      if (alpha < 80) continue
      const r = pixels[index], g = pixels[index + 1], b = pixels[index + 2]
      const distance = Math.abs(r - background.r) + Math.abs(g - background.g) + Math.abs(b - background.b)
      if (distance < 28) continue
      points.push({
        x: rect.left + (x / width) * rect.width,
        y: rect.top + (y / height) * rect.height,
        color: `rgba(${r},${g},${b},${Math.min(0.85, alpha / 255)})`,
        size: 0.45 + (distance / 360) * 1.3,
        delay: Math.random() * 180,
      })
    }
  }
  if (points.length > 760) {
    for (let index = points.length - 1; index > 0; index -= 1) {
      const swap = Math.floor(Math.random() * (index + 1))
      ;[points[index], points[swap]] = [points[swap], points[index]]
    }
    points.length = 760
  }
  return points
}

async function textureDissolve(element: HTMLElement, texture: Texture) {
  const { image, rect } = texture
  const particles = sampleTexture(texture)
  const canvas = document.createElement('canvas')
  const ratio = Math.min(window.devicePixelRatio || 1, 2)
  canvas.width = Math.round(window.innerWidth * ratio)
  canvas.height = Math.round(window.innerHeight * ratio)
  canvas.style.cssText = 'position:fixed;inset:0;width:100vw;height:100vh;pointer-events:none;z-index:9999'
  document.body.appendChild(canvas)
  const context = canvas.getContext('2d')
  if (!context) { canvas.remove(); return }
  const ctx = context
  ctx.scale(ratio, ratio)
  element.style.visibility = 'hidden'
  const started = performance.now()
  await new Promise<void>((resolve) => {
    function frame(now: number) {
      const progress = Math.min(1, (now - started) / 940)
      ctx.clearRect(0, 0, window.innerWidth, window.innerHeight)
      const ease = 1 - (1 - progress) * (1 - progress)
      const edge = rect.left + rect.width * Math.min(1, ease * 1.08)
      ctx.save()
      ctx.beginPath()
      ctx.rect(edge, rect.top, Math.max(0, rect.right - edge), rect.height)
      ctx.clip()
      ctx.globalAlpha = Math.max(0, 1 - progress * 0.48)
      ctx.drawImage(image, 0, 0, image.width, image.height, rect.left, rect.top, rect.width, rect.height)
      ctx.restore()
      for (const particle of particles) {
        const t = Math.max(0, now - started - particle.delay) / 940
        if (t >= 1) continue
        const wind = t * t
        const x = particle.x + (120 + particle.size * 80) * wind + Math.sin((t + particle.delay) * 0.02) * 8 * t
        const y = particle.y - 20 * t + 26 * t * t
        ctx.globalAlpha = (1 - t) * 0.9
        ctx.fillStyle = particle.color
        ctx.beginPath()
        ctx.ellipse(x, y, particle.size * (1.2 + t * 3), particle.size * 0.7, 0, 0, Math.PI * 2)
        ctx.fill()
      }
      if (progress < 1) requestAnimationFrame(frame)
      else resolve()
    }
    requestAnimationFrame(frame)
  })
  canvas.remove()
}

export async function playDeleteAnimation(element: HTMLElement | null, mode: MotionMode = getMotionMode()): Promise<void> {
  if (!element || mode === 'off') return
  if (reduceMotion() || mode === 'subtle') {
    await subtleExit(element)
    return
  }
  try {
    const texture = await captureTexture(element)
    if (texture) {
      await textureDissolve(element, texture)
      return
    }
  } catch {
    // Screenshot capture can fail on cross-origin icons or restricted
    // rendering. The generic ash effect remains the fallback.
  }
  try {
    await ashDissolve(element)
  } catch {
    element.style.visibility = 'hidden'
  }
}

export function findSkillElement(skillID: string): HTMLElement | null {
  return Array.from(document.querySelectorAll<HTMLElement>('[data-skill-id]')).find((element) => element.dataset.skillId === skillID) ?? null
}

export { wait }
