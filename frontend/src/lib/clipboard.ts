import { Clipboard } from '@wailsio/runtime'

export async function copyText(value: string) {
  try {
    await Clipboard.SetText(value)
    return
  } catch {
    await navigator.clipboard.writeText(value)
  }
}
