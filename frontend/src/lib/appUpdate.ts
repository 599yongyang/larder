import { Browser } from '@wailsio/runtime'

export async function openExternal(url: string) {
  try { await Browser.OpenURL(url) } catch { window.open(url, '_blank') }
}
