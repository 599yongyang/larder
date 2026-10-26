import { Browser } from '@wailsio/runtime'
import { getLanguage } from '../i18n'

export type AppUpdate = {
  current: string
  latest: string
  available: boolean
  url: string
  publishedAt: string
  notes: string
}

const RELEASE_API = 'https://api.github.com/repos/599yongyang/larder/releases/latest'
const VERSION_MANIFEST = 'https://raw.githubusercontent.com/599yongyang/larder/main/version.json'
const FETCH_TIMEOUT = 12_000

type ReleasePayload = { version?: string; tag?: string; tag_name?: string; url?: string; html_url?: string; publishedAt?: string; published_at?: string; notes?: string; body?: string }

function normalize(version: string) {
  return version.replace(/^v/i, '').split('.').map((part) => Number.parseInt(part, 10) || 0)
}

function isNewer(latest: string, current: string) {
  const a = normalize(latest)
  const b = normalize(current)
  for (let index = 0; index < Math.max(a.length, b.length); index++) {
    const left = a[index] ?? 0
    const right = b[index] ?? 0
    if (left !== right) return left > right
  }
  return false
}

async function fetchWithTimeout(url: string, headers: Record<string, string>) {
  const controller = new AbortController()
  const timer = window.setTimeout(() => controller.abort(), FETCH_TIMEOUT)
  try {
    return await fetch(url, { headers, signal: controller.signal })
  } finally {
    window.clearTimeout(timer)
  }
}

function wait(ms: number) {
  return new Promise((resolve) => window.setTimeout(resolve, ms))
}

async function fetchWithRetry(url: string, headers: Record<string, string>, attempts = 2) {
  let lastError: unknown
  for (let attempt = 0; attempt < attempts; attempt++) {
    try {
      const response = await fetchWithTimeout(url, headers)
      if ((response.status === 502 || response.status === 503 || response.status === 504) && attempt + 1 < attempts) {
        lastError = new Error(`GitHub ${response.status}`)
        await wait(400 * (attempt + 1))
        continue
      }
      return response
    } catch (error) {
      lastError = error
      if (attempt + 1 < attempts) await wait(400 * (attempt + 1))
    }
  }
  throw lastError instanceof Error ? lastError : new Error('GitHub request failed')
}

function toUpdate(payload: ReleasePayload, current: string): AppUpdate {
  const latest = (payload.version ?? payload.tag ?? payload.tag_name ?? '').replace(/^v/i, '')
  return {
    current,
    latest,
    available: Boolean(latest) && isNewer(latest, current),
    url: payload.url ?? payload.html_url ?? 'https://github.com/599yongyang/larder/releases',
    publishedAt: payload.publishedAt ?? payload.published_at ?? '',
    notes: payload.notes ?? payload.body ?? '',
  }
}

async function checkManifest(current: string) {
  const response = await fetchWithRetry(VERSION_MANIFEST, { Accept: 'application/json' })
  if (!response.ok) throw new Error(`GitHub ${response.status}`)
  return toUpdate(await response.json() as ReleasePayload, current)
}

async function checkReleaseAPI(current: string) {
  const response = await fetchWithRetry(RELEASE_API, { Accept: 'application/vnd.github+json' })
  if (response.status === 404) throw new Error(getLanguage() === 'en' ? 'No published releases yet.' : '还没有已发布的版本。')
  if (!response.ok) throw new Error(`GitHub ${response.status}`)
  return toUpdate(await response.json() as ReleasePayload, current)
}

export async function checkAppUpdate(current: string): Promise<AppUpdate> {
  try {
    return await checkManifest(current)
  } catch {
    // The anonymous GitHub API is rate-limited and occasionally returns 502/504.
    // Fall back to it only when the lightweight manifest cannot be read.
    try {
      return await checkReleaseAPI(current)
    } catch {
      throw new Error(getLanguage() === 'en' ? 'Could not reach GitHub. Check your network and try again.' : '无法连接 GitHub，请检查网络后重试。')
    }
  }
}

export async function openExternal(url: string) {
  try { await Browser.OpenURL(url) } catch { window.open(url, '_blank') }
}
