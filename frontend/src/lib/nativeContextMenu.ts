import type { CSSProperties } from 'react'

export type NativeContextMenuData = {
  kind: string
  name?: string
  path?: string
  project?: string
  id?: string
}

export function nativeContextMenu(data: NativeContextMenuData): CSSProperties {
  return {
    '--custom-contextmenu': 'larder-resource',
    '--custom-contextmenu-data': JSON.stringify(data),
  } as CSSProperties
}
