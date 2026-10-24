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

export async function checkAppUpdate(current: string): Promise<AppUpdate> {
  const response = await fetch(RELEASE_API, { headers: { Accept: 'application/vnd.github+json' } })
  if (response.status === 404) throw new Error(getLanguage() === 'en' ? 'No published releases yet.' : '还没有已发布的版本。')
  if (!response.ok) throw new Error(`GitHub ${response.status}`)
  const release = await response.json() as { tag_name?: string; html_url?: string; published_at?: string; body?: string }
  const latest = (release.tag_name ?? '').replace(/^v/i, '')
  return {
    current,
    latest,
    available: isNewer(latest, current),
    url: release.html_url ?? 'https://github.com/599yongyang/larder/releases',
    publishedAt: release.published_at ?? '',
    notes: release.body ?? '',
  }
}

export async function openExternal(url: string) {
  try { await Browser.OpenURL(url) } catch { window.open(url, '_blank') }
}
