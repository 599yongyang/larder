import { gooeyToast } from 'goey-toast'

export function toastError(error: unknown, fallback = '操作失败') {
  const message = error instanceof Error ? error.message : fallback
  gooeyToast.error(fallback, { description: message })
}

export function toastSuccess(title: string, description?: string, action?: { label: string; onClick: () => void; successLabel?: string }) {
  gooeyToast.success(title, { ...(description ? { description } : {}), ...(action ? { action } : {}) })
}

export function toastInfo(title: string, description?: string) {
  gooeyToast.info(title, description ? { description } : undefined)
}
