import { useEffect, useState } from 'react'
import { formatRelativeTime } from '../lib/format'
import { CloudDownload, CloudUpload, PlugZap } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Checkbox } from '@/components/ui/checkbox'
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { SegmentedControl } from '@/components/ui/segmented-control'
import { DataPackageDialog } from './DataPackageDialog'
import { backend } from '../backend'
import { t } from '../i18n'
import { toastError, toastSuccess } from '../lib/toast'
import styles from '../App.module.css'
import type { DataPackageInfo, DataPackageSelection, SyncConfig } from '../types'

const defaults: SyncConfig = { provider: 'webdav', url: '', username: '', endpoint: '', region: 'us-east-1', bucket: '', prefix: '', accessKey: '', filename: 'larder-data.zip', passwordSet: false, lastSyncAt: '', lastSyncKind: '', encryptionEnabled: false, encryptionPassphraseSet: false }

export function SyncSettings({ onRestored }: { onRestored: () => void }) {
  const [config, setConfig] = useState<SyncConfig>(defaults)
  const [password, setPassword] = useState('')
  const [encryptionPassword, setEncryptionPassword] = useState('')
  const [busy, setBusy] = useState(false)
  const [status, setStatus] = useState('')
  const [allowPlaintext, setAllowPlaintext] = useState(false)
  const [conflict, setConflict] = useState(false)
  const [pendingSelection, setPendingSelection] = useState<DataPackageSelection>()
  const [dialog, setDialog] = useState<{ mode: 'export' | 'import'; path?: string; info?: DataPackageInfo }>()
  useEffect(() => { backend.syncConfig().then(setConfig).catch(() => {}) }, [])
  const save = async () => {
    await backend.saveSyncConfig(config, password, encryptionPassword)
    setPassword('')
    setEncryptionPassword('')
    setConfig(await backend.syncConfig())
  }
  const run = async (action: () => Promise<void>) => {
    setBusy(true); setStatus('')
    try { await save(); await action(); setStatus(t('sync.done')) } catch (error) { toastError(error, t('sync.failed')) } finally { setBusy(false) }
  }
  const saveConfig = async () => { setBusy(true); try { await save(); toastSuccess(t('sync.saved')); setStatus(t('sync.saved')) } catch (error) { toastError(error, t('sync.failed')) } finally { setBusy(false) } }
  const test = () => run(async () => { await backend.testSync(); toastSuccess(t('sync.testOk')); setStatus(t('sync.testOk')) })
  const upload = () => run(async () => setDialog({ mode: 'export' }))
  const download = () => run(async () => {
    const path = await backend.downloadSyncPackage()
    setConfig(await backend.syncConfig())
    const info = await backend.inspectDataPackage(path)
    setDialog({ mode: 'import', path, info })
  })
  const confirmDialog = async (selection: DataPackageSelection) => {
    if (!dialog) return
    setBusy(true)
    try {
      if (dialog.mode === 'export') {
        try {
          await backend.uploadSyncPackage(selection, allowPlaintext, false)
        } catch (error) {
          if (String(error).includes('remote package changed')) { setPendingSelection(selection); setConflict(true); return }
          throw error
        }
        setConfig(await backend.syncConfig())
        toastSuccess(t('sync.uploaded'))
      } else if (dialog.path) {
        const result = await backend.importDataPackage(dialog.path, selection)
        toastSuccess(t('dataPackage.imported', { n: result.restored }), result.snapshotDir)
        onRestored()
      }
      setDialog(undefined)
    } catch (error) { toastError(error, t('sync.failed')) } finally { setBusy(false) }
  }
  return <section className={styles.settingsGroup}>
    <h2>{t('sync.provider')}</h2>
    <SegmentedControl ariaLabel={t('sync.provider')} value={config.provider} onChange={(value) => setConfig({ ...config, provider: value, passwordSet: false })} options={[{ value: 'webdav', label: 'WebDAV' }, { value: 's3', label: 'S3' }]} />
    {config.provider === 'webdav' && <><div className={styles.settingsRow}><span><strong>{t('sync.url')}</strong></span><Input className="!h-8 w-[240px]" value={config.url} placeholder="https://dav.example.com/larder" onChange={(event) => setConfig({ ...config, url: event.target.value })} aria-label={t('sync.url')} /></div>
    <div className={styles.settingsRow}><span><strong>{t('sync.username')}</strong></span><Input className="!h-8 w-[180px]" value={config.username} onChange={(event) => setConfig({ ...config, username: event.target.value })} aria-label={t('sync.username')} /></div></>}
    {config.provider === 's3' && <><div className={styles.settingsRow}><span><strong>{t('sync.endpoint')}</strong></span><Input className="!h-8 w-[240px]" value={config.endpoint} placeholder="https://s3.example.com" onChange={(event) => setConfig({ ...config, endpoint: event.target.value })} aria-label={t('sync.endpoint')} /></div>
    <div className={styles.settingsRow}><span><strong>{t('sync.region')}</strong></span><Input className="!h-8 w-[180px]" value={config.region} onChange={(event) => setConfig({ ...config, region: event.target.value })} aria-label={t('sync.region')} /></div>
    <div className={styles.settingsRow}><span><strong>{t('sync.bucket')}</strong></span><Input className="!h-8 w-[180px]" value={config.bucket} onChange={(event) => setConfig({ ...config, bucket: event.target.value })} aria-label={t('sync.bucket')} /></div>
    <div className={styles.settingsRow}><span><strong>{t('sync.prefix')}</strong></span><Input className="!h-8 w-[180px]" value={config.prefix} placeholder="larder" onChange={(event) => setConfig({ ...config, prefix: event.target.value })} aria-label={t('sync.prefix')} /></div>
    <div className={styles.settingsRow}><span><strong>{t('sync.accessKey')}</strong></span><Input className="!h-8 w-[180px]" value={config.accessKey} onChange={(event) => setConfig({ ...config, accessKey: event.target.value })} aria-label={t('sync.accessKey')} /></div></>}
    <div className={styles.settingsRow}><span><strong>{config.provider === 's3' ? t('sync.secretKey') : t('sync.password')}</strong><small>{config.passwordSet ? t('sync.passwordSaved') : ''}</small></span><Input className="!h-8 w-[180px]" type="password" value={password} placeholder={config.passwordSet ? '••••••••' : ''} onChange={(event) => setPassword(event.target.value)} aria-label={t('sync.password')} /></div>
    <div className={styles.settingsRow}><span><strong>{t('sync.filename')}</strong></span><Input className="!h-8 w-[180px]" value={config.filename} onChange={(event) => setConfig({ ...config, filename: event.target.value })} aria-label={t('sync.filename')} /></div>
    <label className={styles.settingsRow}><span><strong>{t('sync.encrypt')}</strong><small>{t('sync.encryptHint')}</small></span><Checkbox checked={config.encryptionEnabled} onCheckedChange={(checked) => setConfig({ ...config, encryptionEnabled: Boolean(checked) })} /></label>
    {config.encryptionEnabled && <div className={styles.settingsRow}><span><strong>{t('sync.encryptionPassword')}</strong><small>{config.encryptionPassphraseSet ? t('sync.passwordSaved') : t('sync.encryptionPasswordHint')}</small></span><Input className="!h-8 w-[180px]" type="password" value={encryptionPassword} placeholder={config.encryptionPassphraseSet ? '••••••••' : ''} onChange={(event) => setEncryptionPassword(event.target.value)} aria-label={t('sync.encryptionPassword')} /></div>}
    {!config.encryptionEnabled && <label className={styles.settingsRow}><span><strong>{t('sync.allowPlaintext')}</strong><small>{t('sync.allowPlaintextHint')}</small></span><Checkbox checked={allowPlaintext} onCheckedChange={(checked) => setAllowPlaintext(Boolean(checked))} /></label>}
    <div className={styles.settingsRow}><span><strong>{t('sync.connection')}</strong><small>{status || (config.lastSyncAt ? t('sync.lastSync', { time: formatRelativeTime(config.lastSyncAt), kind: t(`sync.kind.${config.lastSyncKind}`) }) : t('sync.hint'))}</small></span><div className="flex items-center gap-1.5"><Button variant="outline" size="sm" disabled={busy} onClick={() => void saveConfig()}>{t('sync.save')}</Button><Button variant="outline" size="sm" disabled={busy} onClick={() => void test()}><PlugZap size={14} />{t('sync.test')}</Button><Button variant="outline" size="sm" disabled={busy || (!config.encryptionEnabled && !allowPlaintext)} onClick={() => void upload()}><CloudUpload size={14} />{t('sync.upload')}</Button><Button variant="outline" size="sm" disabled={busy} onClick={() => void download()}><CloudDownload size={14} />{t('sync.download')}</Button></div></div>
    <Dialog open={conflict} onOpenChange={setConflict}><DialogContent className="max-w-sm"><DialogHeader><DialogTitle>{t('sync.conflictTitle')}</DialogTitle><DialogDescription>{t('sync.conflictHint')}</DialogDescription></DialogHeader><DialogFooter><Button size="sm" variant="outline" onClick={() => setConflict(false)}>{t('common.cancel')}</Button><Button size="sm" variant="destructive" onClick={() => { if (pendingSelection) void (async () => { setBusy(true); try { await backend.uploadSyncPackage(pendingSelection, allowPlaintext, true); setConfig(await backend.syncConfig()); toastSuccess(t('sync.uploaded')); setConflict(false) } catch (error) { toastError(error, t('sync.failed')) } finally { setBusy(false); setDialog(undefined) } })() }}>{t('sync.overwriteRemote')}</Button></DialogFooter></DialogContent></Dialog>
    <DataPackageDialog mode={dialog?.mode || 'export'} open={Boolean(dialog)} info={dialog?.info} busy={busy} title={dialog?.mode === 'import' ? t('sync.importTitle') : t('sync.uploadTitle')} hint={dialog?.mode === 'import' ? t('sync.importHint') : t('sync.uploadHint')} confirmLabel={dialog?.mode === 'import' ? t('sync.importConfirm') : t('sync.uploadConfirm')} onClose={() => { if (!busy) setDialog(undefined) }} onConfirm={(selection) => void confirmDialog(selection)} />
  </section>
}
