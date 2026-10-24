import { getLanguage, t } from '../i18n'

const MARKER = 'larder-error:'

type Envelope = { code?: string; params?: Record<string, string>; fallback?: string }

function envelopeFrom(error: unknown): Envelope | undefined {
  const raw = error instanceof Error ? error.message : typeof error === 'string' ? error : ''
  if (!raw.startsWith(MARKER)) return undefined
  try { return JSON.parse(raw.slice(MARKER.length)) as Envelope } catch { return undefined }
}

/** Render a Go error as user-facing copy without leaking encoded error data. */
export function backendErrorMessage(error: unknown, fallback: string): string {
  const envelope = envelopeFrom(error)
  if (envelope?.code) {
    const translated = t(envelope.code, envelope.params)
    if (translated !== envelope.code) return translated
  }
  const raw = error instanceof Error ? error.message : ''
  if (getLanguage() === 'zh') return raw && !raw.startsWith(MARKER) ? raw : fallback
  return fallback
}
