import { fireEvent, render, screen } from '@testing-library/react'
import { describe, expect, test, vi } from 'vitest'
import { AppUpdateDialog } from '../components/AppUpdateDialog'
import { t } from '../i18n'

const info = { available: true, version: '0.0.8', name: 'Larder 0.0.8', notes: 'A calmer update.', publishedAt: '', size: 1024 * 1024 }

describe('AppUpdateDialog', () => {
  test('shows a single up-to-date message', () => {
    render(<AppUpdateDialog stage="uptodate" currentVersion="0.0.7" onClose={() => {}} onInstall={() => {}} onRestart={() => {}} onRetry={() => {}} />)
    expect(screen.getAllByText(t('appUpdate.upToDateHint', { version: '0.0.7' }))).toHaveLength(1)
  })

  test('offers download for an available update', () => {
    const onInstall = vi.fn()
    render(<AppUpdateDialog stage="available" info={info} currentVersion="0.0.7" onClose={() => {}} onInstall={onInstall} onRestart={() => {}} onRetry={() => {}} />)
    expect(screen.getByText(/Larder 0\.0\.8/)).toBeTruthy()
    fireEvent.click(screen.getByRole('button', { name: t('appUpdate.download') }))
    expect(onInstall).toHaveBeenCalledOnce()
  })

  test('shows download progress and restart when ready', () => {
    const onRestart = vi.fn()
    render(<AppUpdateDialog stage="downloading" info={info} progress={{ written: 50, total: 100 }} currentVersion="0.0.7" onClose={() => {}} onInstall={() => {}} onRestart={onRestart} onRetry={() => {}} />)
    expect(screen.getByText('50%')).toBeTruthy()
    render(<AppUpdateDialog stage="ready" info={info} currentVersion="0.0.7" onClose={() => {}} onInstall={() => {}} onRestart={onRestart} onRetry={() => {}} />)
    fireEvent.click(screen.getByRole('button', { name: t('appUpdate.restart') }))
    expect(onRestart).toHaveBeenCalledOnce()
  })
})
