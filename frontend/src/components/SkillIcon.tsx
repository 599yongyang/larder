import { useEffect, useRef, useState } from 'react'
import { BookOpen, Bug, Cpu, FolderGit2, Palette, Puzzle, Search, Sparkles, Terminal, Workflow, Wrench, type LucideIcon } from 'lucide-react'
import { backend } from '../backend'
import type { Skill } from '../types'
import { githubAvatarUrls } from '../lib/github'

const localIconCache = new Map<string, string>()
const localIconPending = new Map<string, Promise<string>>()
const localIconFailed = new Set<string>()

function loadLocalIcon(skillID: string, icon: string): Promise<string> {
  const key = `${skillID}:${icon}`
  if (localIconCache.has(key)) return Promise.resolve(localIconCache.get(key) ?? '')
  if (localIconFailed.has(key)) return Promise.resolve('')
  const pending = localIconPending.get(key)
  if (pending) return pending
  const request = backend.skillIcon(skillID).then((value) => {
    localIconCache.set(key, value)
    return value
  }).catch(() => {
    localIconFailed.add(key)
    return ''
  }).finally(() => { localIconPending.delete(key) })
  localIconPending.set(key, request)
  return request
}

const avatarCache = new Map<string, string>()
const avatarPending = new Map<string, Promise<string>>()

async function fetchAvatar(url: string): Promise<string> {
  const controller = new AbortController()
  const timeout = window.setTimeout(() => controller.abort(), 4000)
  try {
    const response = await fetch(url, { signal: controller.signal })
    if (!response.ok) throw new Error('avatar request failed')
    const blob = await response.blob()
    return await new Promise<string>((resolve, reject) => {
      const reader = new FileReader()
      reader.onload = () => resolve(String(reader.result || ''))
      reader.onerror = () => reject(reader.error)
      reader.readAsDataURL(blob)
    })
  } finally {
    window.clearTimeout(timeout)
  }
}

function firstSuccessful<T>(promises: Promise<T>[]): Promise<T> {
  return new Promise((resolve, reject) => {
    let remaining = promises.length
    let settled = false
    if (remaining === 0) { reject(new Error('no candidates')); return }
    for (const promise of promises) {
      promise.then((value) => { if (!settled) { settled = true; resolve(value) } }).catch(() => { remaining -= 1; if (remaining === 0 && !settled) reject(new Error('all candidates failed')) })
    }
  })
}

function loadOwnerAvatar(owner: string): Promise<string> {
  const cached = avatarCache.get(owner)
  if (cached) return Promise.resolve(cached)
  const pending = avatarPending.get(owner)
  if (pending) return pending
  const request = (async () => {
    try {
      const value = await firstSuccessful(githubAvatarUrls(owner).map(async (url) => {
        const result = await fetchAvatar(url)
        if (!result) throw new Error('empty avatar')
        avatarCache.set(owner, result)
        return result
      }))
      return value
    } catch {
      return ''
    }
  })().finally(() => { avatarPending.delete(owner) })
  avatarPending.set(owner, request)
  return request
}

const palettes = [
  'bg-violet-500/12 text-violet-600 dark:text-violet-300',
  'bg-sky-500/12 text-sky-600 dark:text-sky-300',
  'bg-emerald-500/12 text-emerald-600 dark:text-emerald-300',
  'bg-amber-500/12 text-amber-600 dark:text-amber-300',
  'bg-rose-500/12 text-rose-600 dark:text-rose-300',
  'bg-indigo-500/12 text-indigo-600 dark:text-indigo-300',
]

function glyphFor(name: string, description: string): LucideIcon {
  const text = `${name} ${description}`.toLowerCase()
  if (/terminal|cli|command|shell|script/.test(text)) return Terminal
  if (/design|frontend|ui|css|visual|palette/.test(text)) return Palette
  if (/debug|bug|diagnos|error|test/.test(text)) return Bug
  if (/search|research|find|discover/.test(text)) return Search
  if (/workflow|automation|pipeline|agent/.test(text)) return Workflow
  if (/doc|guide|writing|book|tutorial/.test(text)) return BookOpen
  if (/model|ai|llm|inference/.test(text)) return Cpu
  if (/tool|utility|build|config/.test(text)) return Wrench
  if (/skill|plugin|package/.test(text)) return Puzzle
  return Sparkles
}


export function SkillIcon({ skill, className = 'size-7', loading = 'lazy' }: { skill: Skill; className?: string; loading?: 'lazy' | 'eager' }) {
  const owner = skill.source?.includes('/') ? skill.source.split('/')[0].trim() : ''
  const iconKey = `${skill.id}:${skill.icon?.trim() ?? ''}`
  const hostRef = useRef<HTMLElement | null>(null)
  const [inView, setInView] = useState(loading === 'eager')
  useEffect(() => {
    if (loading === 'eager' || inView) return
    const node = hostRef.current
    if (!node || typeof IntersectionObserver === 'undefined') { setInView(true); return }
    const observer = new IntersectionObserver(([entry]) => {
      if (entry.isIntersecting) { setInView(true); observer.disconnect() }
    }, { rootMargin: '240px' })
    observer.observe(node)
    return () => observer.disconnect()
  }, [inView, loading])
  const [custom, setCustom] = useState<string | undefined>(() => localIconCache.get(iconKey) ?? (skill.icon && !/^(https?:|data:)/.test(skill.icon) && !skill.icon.includes('/') && !skill.icon.includes('.') ? skill.icon : undefined))
  const [failed, setFailed] = useState(false)
  const [ownerFailed, setOwnerFailed] = useState(false)
  const [avatar, setAvatar] = useState<string | undefined>(() => owner ? avatarCache.get(owner) : undefined)
  const [avatarAttempt, setAvatarAttempt] = useState(0)
  useEffect(() => {
    if (!inView) return
    let live = true
    setFailed(false)
    setOwnerFailed(false)
    setAvatarAttempt(0)
    const icon = skill.icon?.trim()
    if (!icon) { setCustom(undefined); return }
    if (/^(https?:|data:)/.test(icon)) {
      setCustom(icon)
      return
    }
    // An emoji / short text icon from frontmatter.
    if (!icon.includes('/') && !icon.includes('.')) {
      setCustom(icon)
      return
    }
    const cached = localIconCache.get(`${skill.id}:${icon}`)
    if (cached) { setCustom(cached); return }
    setCustom(undefined)
    void loadLocalIcon(skill.id, icon).then((value) => { if (live && value) setCustom(value) })
    return () => { live = false }
  }, [inView, skill.id, skill.icon])
  useEffect(() => {
    if (!inView) return
    setAvatarAttempt(0)
    if (!owner) { setAvatar(undefined); return }
    const cached = avatarCache.get(owner)
    if (cached) { setAvatar(cached); return }
    let live = true
    void loadOwnerAvatar(owner).then((value) => { if (live && value) setAvatar(value) })
    return () => { live = false }
  }, [inView, owner])

  if (custom && !failed) {
    if (/^(https?:|data:)/.test(custom)) {
      return <img ref={(node) => { hostRef.current = node }} src={custom} alt="" aria-hidden="true" loading={loading} decoding="async" referrerPolicy="no-referrer" onError={() => setFailed(true)} className={`${className} shrink-0 rounded-md border border-line object-cover`} />
    }
    return <span ref={(node) => { hostRef.current = node }} className={`${className} inline-flex shrink-0 items-center justify-center rounded-md bg-muted text-[58%] leading-none`} aria-hidden="true">{custom}</span>
  }

  if (inView && owner && !ownerFailed) {
    const urls = githubAvatarUrls(owner)
    const src = avatar || urls[avatarAttempt]
    return <img ref={(node) => { hostRef.current = node }} src={src} alt="" aria-hidden="true" loading={loading} decoding="async" referrerPolicy="no-referrer" onError={() => { if (avatarAttempt + 1 < urls.length) setAvatarAttempt(avatarAttempt + 1); else setOwnerFailed(true) }} className={`${className} shrink-0 rounded-md border border-line object-cover`} />
  }

  return <SkillGlyph name={skill.name} description={skill.description} className={className} />
}

export function SkillGlyph({ name, description = '', className = 'size-7' }: { name: string; description?: string; className?: string }) {
  const Icon = glyphFor(name, description)
  const hash = [...name].reduce((sum, char) => sum + char.codePointAt(0)!, 0)
  return <span className={`${className} inline-flex shrink-0 items-center justify-center rounded-md ${palettes[hash % palettes.length]}`} aria-hidden="true"><Icon className="size-[58%]" strokeWidth={1.8} /></span>
}

export function RepoIcon({ repo, className = 'size-4' }: { repo: string; className?: string }) {
  const owner = repo.replace(/^https?:\/\/github\.com\//, '').replace(/\.git$/, '').split('/')[0].trim()
  const [avatar, setAvatar] = useState<string | undefined>(() => owner ? avatarCache.get(owner) : undefined)
  const [failed, setFailed] = useState(false)
  useEffect(() => {
    if (!owner || avatar) return
    let live = true
    void loadOwnerAvatar(owner).then((value) => { if (live && value) setAvatar(value) })
    return () => { live = false }
  }, [owner, avatar])
  if (avatar && !failed) return <img src={avatar} alt="" aria-hidden="true" loading="lazy" decoding="async" referrerPolicy="no-referrer" onError={() => setFailed(true)} className={`${className} shrink-0 rounded-md border border-line object-cover`} />
  return <span className={`${className} inline-flex shrink-0 items-center justify-center rounded-md bg-muted text-muted-foreground`} aria-hidden="true"><FolderGit2 className="size-[62%]" strokeWidth={1.8} /></span>
}
