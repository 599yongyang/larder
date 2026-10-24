import { gooeyToast } from 'goey-toast'
import { t } from '../i18n'
import { backendErrorMessage } from './backendError'

export function toastError(error: unknown, fallback = t('common.operationFailed')) {
  const message = backendErrorMessage(error, fallback)
  gooeyToast.error(fallback, { description: message })
}

export function toastSuccess(title: string, description?: string, action?: { label: string; onClick: () => void; successLabel?: string }) {
  gooeyToast.success(title, { ...(description ? { description } : {}), ...(action ? { action } : {}) })
}

export function toastInfo(title: string, description?: string) {
  gooeyToast.info(title, description ? { description } : undefined)
}
