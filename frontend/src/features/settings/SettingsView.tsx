import { useEffect, useState } from 'react'
import { Archive, Copy, Download, Monitor, Moon, RotateCcw, Sparkles, Sun, Waves, X } from 'lucide-react'
import { GitHubIcon } from '../../components/GitHubIcon'
import { Button } from '@/components/ui/button'
import { Kbd } from '@/components/ui/kbd'
import { Checkbox } from '@/components/ui/checkbox'
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { backend } from '../../backend'
import { toastError, toastSuccess } from '../../lib/toast'
import { Input } from '@/components/ui/input'
import styles from '../../App.module.css'
import type { AppInfo, BackupInfo, Settings } from '../../types'
import type { ThemeMode } from '../../lib/theme'
import { getLanguage, getLanguageSetting, languages, setLanguage, t } from '../../i18n'
import { checkAppUpdate, openExternal, type AppUpdate } from '../../lib/appUpdate'
import { getMotionMode, setMotionMode, type MotionMode } from '../../lib/motion'
import { applyZoom, getZoom, setZoom, zoomLevels, type ZoomLevel } from '../../lib/zoom'
import { modifierLabel } from '../../lib/platform'

const motionOptions: Array<{ value: MotionMode; labelKey: string; icon: typeof Sun }> = [
  { value: 'full', labelKey: 'settings.motionFull', icon: Sparkles },
  { value: 'subtle', labelKey: 'settings.motionSubtle', icon: Waves },
  { value: 'off', labelKey: 'settings.motionOff', icon: X },
]

const themeOptions: Array<{ value: ThemeMode; labelKey: string; icon: typeof Sun }> = [
  { value: 'system', labelKey: 'common.system', icon: Monitor },
  { value: 'light', labelKey: 'common.light', icon: Sun },
  { value: 'dark', labelKey: 'common.dark', icon: Moon },
]

function SettingsView({ settings, onToggle, onUsageToggle, onAutoCheckToggle, onInterval, onRetention, themeMode, onThemeMode, info, onBackup, onRestored }: { settings: Settings; onToggle: (value: boolean) => void; onUsageToggle: (value: boolean) => void; onAutoCheckToggle: (value: boolean) => void; onInterval: (value: number) => void; onRetention: (value: number) => void; themeMode: ThemeMode; onThemeMode: (value: ThemeMode, event?: React.MouseEvent<HTMLElement>) => void; info?: AppInfo; onBackup: () => void; onRestored: () => void }) {
  const [restore, setRestore] = useState<{ path: string; info: BackupInfo }>()
  const [motion, setMotion] = useState<MotionMode>(getMotionMode)
  const [zoom, setZoomState] = useState<ZoomLevel>(getZoom)
  const [appUpdate, setAppUpdate] = useState<AppUpdate>()
  const [checkingAppUpdate, setCheckingAppUpdate] = useState(false)
  const handleCheckAppUpdate = async () => {
    setCheckingAppUpdate(true)
    try {
      const result = await checkAppUpdate(info?.version ?? '0.0.0')
      setAppUpdate(result)
      if (!result.available) toastSuccess(t('settings.appUpToDate'))
    } catch (error) {
      toastError(error, t('settings.appUpdateCheckFailed'))
    } finally { setCheckingAppUpdate(false) }
  }
  const changeZoom = (value: ZoomLevel) => { setZoom(value); setZoomState(value); void applyZoom(value) }
  useEffect(() => {
    const sync = () => setZoomState(getZoom())
    window.addEventListener('larder:zoom', sync)
    return () => window.removeEventListener('larder:zoom', sync)
  }, [])
  const mod = modifierLabel()
  const copyDiagnostics = async () => {
    const payload = { generatedAt: new Date().toISOString(), app: info, settings, language: getLanguage(), motion, zoom, runtime: { userAgent: navigator.userAgent, language: navigator.language, languages: navigator.languages, hardwareConcurrency: navigator.hardwareConcurrency } }
    try { await navigator.clipboard.writeText(JSON.stringify(payload, null, 2)); toastSuccess(t('settings.diagnosticsCopied')) } catch (error) { toastError(error, t('settings.diagnosticsCopyFailed')) }
  }
  const [restoring, setRestoring] = useState(false)
  const chooseBackup = async () => { setRestoring(true); try { const path = await backend.pickBackupFile(); if (!path) return; const info = await backend.inspectBackup(path); setRestore({ path, info }) } catch (error) { toastError(error, t('settings.failedToReadBackup')) } finally { setRestoring(false) } }
  const applyRestore = async () => { if (!restore) return; setRestoring(true); try { const result = await backend.restoreBackup(restore.path); toastSuccess(t('settings.restoredFiles', { count: result.restored }), result.snapshotDir); setRestore(undefined); onRestored() } catch (error) { toastError(error, t('settings.failedToRestoreBackup')) } finally { setRestoring(false) } }
  return <section className={styles.settingsPanel}>
    <div className={styles.settingsGrid}>
      <section className={styles.settingsGroup}><h2>{t('settings.appearance')}</h2><div className={styles.themeOptions} role="group" aria-label={t('settings.appearance')}>{themeOptions.map(({ value, labelKey, icon: Icon }) => <button type="button" key={value} className={`${styles.themeOption} ${themeMode === value ? styles.themeOptionActive : ''}`} aria-pressed={themeMode === value} onClick={(event) => onThemeMode(value, event)}><Icon size={14} aria-hidden="true" /><span>{t(labelKey)}</span></button>)}</div><p className={styles.settingsHint}>{t('settings.systemFollowsYourDeviceAppearance')}</p></section>
      <section className={styles.settingsGroup}><h2>{t('settings.language')}</h2><div className={styles.themeOptions} role="group" aria-label={t('settings.language')}>{languages.map(({ value, label }) => <button type="button" key={value} className={`${styles.themeOption} ${getLanguageSetting() === value ? styles.themeOptionActive : ''}`} aria-pressed={getLanguageSetting() === value} onClick={() => setLanguage(value)}><span>{value === 'system' ? t(label) : label}</span></button>)}</div><p className={styles.settingsHint}>{t('settings.languageChangesImmediately')}</p></section>
      <section className={styles.settingsGroup}><h2>{t('settings.display')}</h2><label className={styles.settingsRow}><span><strong>{t('settings.showAgentLocalSkills')}</strong><small>{t('settings.onlySharedSkillsAreShownByDefaultLocalSkillsStillAppearInsideAnAgent')}</small></span><Checkbox checked={settings.showAgentBuiltins} onCheckedChange={(checked) => onToggle(Boolean(checked))} /></label></section>
      <section className={styles.settingsGroup}><h2>{t('settings.interfaceMotion')}</h2><div className={styles.themeOptions} role="group" aria-label={t('settings.interfaceMotion')}>{motionOptions.map(({ value, labelKey, icon: Icon }) => <button type="button" key={value} className={`${styles.themeOption} ${motion === value ? styles.themeOptionActive : ''}`} aria-pressed={motion === value} onClick={() => { setMotionMode(value); setMotion(value) }}><Icon size={14} aria-hidden="true" /><span>{t(labelKey)}</span></button>)}</div><p className={styles.settingsHint}>{t('settings.interfaceMotionHint')}</p></section>
      <section className={styles.settingsGroup}><h2>{t('settings.uiScale')}</h2><div className={`${styles.themeOptions} ${styles.scaleOptions}`} role="group" aria-label={t('settings.uiScale')}>{zoomLevels.map((value) => <button type="button" key={value} className={`${styles.themeOption} ${zoom === value ? styles.themeOptionActive : ''}`} aria-pressed={zoom === value} onClick={() => changeZoom(value)}><span>{value}%</span></button>)}</div><p className={styles.settingsHint}>{t('settings.uiScaleHint')}</p></section>
      <section className={styles.settingsGroup}><h2>{t('settings.keyboardShortcuts')}</h2><div className={styles.shortcutList}><div className={styles.shortcutRow}><span>{t('command.searchAndJump')}</span><Kbd className={styles.shortcutKbd}>{mod} K</Kbd></div><div className={styles.shortcutRow}><span>{t('settings.shortcutFocusSearch')}</span><Kbd className={styles.shortcutKbd}>{mod} F</Kbd></div><div className={styles.shortcutRow}><span>{t('common.rescan')}</span><Kbd className={styles.shortcutKbd}>{mod} R</Kbd></div><div className={styles.shortcutRow}><span>{t('settings.shortcutSwitchSections')}</span><Kbd className={styles.shortcutKbd}>{mod} 1–6</Kbd></div><div className={styles.shortcutRow}><span>{t('settings.shortcutZoomIn')}</span><Kbd className={styles.shortcutKbd}>{mod} +</Kbd></div><div className={styles.shortcutRow}><span>{t('settings.shortcutZoomOut')}</span><Kbd className={styles.shortcutKbd}>{mod} −</Kbd></div><div className={styles.shortcutRow}><span>{t('settings.shortcutResetZoom')}</span><Kbd className={styles.shortcutKbd}>{mod} 0</Kbd></div></div></section>
      <section className={styles.settingsGroup}><h2>{t('settings.autoCheck')}</h2><label className={styles.settingsRow}><span><strong>{t('settings.enableAutoCheck')}</strong><small>{t('settings.manualChecksAreStillAvailableWhenOff')}</small></span><Checkbox checked={settings.autoCheckUpdates} onCheckedChange={(checked) => onAutoCheckToggle(Boolean(checked))} /></label><div className={styles.settingsRow}><span><strong>{t('settings.checkInterval')}</strong><small>{t('settings.hours')}</small></span><div className={styles.intervalField}><Input type="number" min={1} value={String(settings.updateCheckIntervalHours)} onChange={(event) => onInterval(Math.max(1, Number(event.target.value) || 24))} aria-label={t('settings.checkInterval')} /><span>h</span></div></div></section>
      <section className={styles.settingsGroup}><h2>{t('settings.localUsageStats')}</h2><label className={styles.settingsRow}><span><strong>{t('settings.enableLocalUsageStats')}</strong><small>{t('settings.readsLocalZshHistoryOnlyNothingIsUploadedOrWrittenBack')}</small></span><Checkbox checked={settings.usageStatsEnabled} onCheckedChange={(checked) => onUsageToggle(Boolean(checked))} /></label></section>
      <section className={styles.settingsGroup}><h2>{t('settings.data')}</h2><div className={styles.settingsRow}><span><strong>{t('settings.snapshotRetention')}</strong><small>{t('settings.oldestSnapshotsAreRemovedAfterThisLimit')}</small></span><div className={styles.intervalField}><Input type="number" min={5} max={500} value={String(settings.snapshotRetention || 50)} onChange={(event) => onRetention(Math.max(5, Number(event.target.value) || 50))} aria-label={t('settings.snapshotRetention')} /><span>{t('settings.itemUnit')}</span></div></div></section>
      <section className={styles.settingsGroup}><h2>{t('settings.backup')}</h2><div className={styles.settingsRow}><span><strong>{t('settings.exportConfigurationBackup')}</strong><small>{t('settings.packageMcpConfigsInstructionFilesAndAppMetadataLocally')}</small></span><Button variant="outline" size="sm" onClick={onBackup}><Archive size={14} />{t('settings.exportBackup')}</Button></div><div className={styles.settingsRow}><span><strong>{t('settings.restoreConfigurationBackup')}</strong><small>{t('settings.currentFilesAreSnapshottedToRestoreSnapshotsBeforeRestoring')}</small></span><Button variant="outline" size="sm" disabled={restoring} onClick={() => void chooseBackup()}><RotateCcw size={14} />{t('settings.restoreBackup')}</Button></div><p className={styles.settingsHint}>{t('settings.mcpEnvironmentVariablesAndHeadersAreStoredInPlainTextRestoringOverwritesTheMatchingConfigFiles')}</p></section>
      <section className={styles.settingsGroup}><h2>{t('settings.diagnostics')}</h2><div className={styles.settingsRow}><span><strong>{t('settings.copyDiagnostics')}</strong><small>{t('settings.diagnosticsHint')}</small></span><Button variant="outline" size="sm" onClick={() => void copyDiagnostics()}><Copy size={14} />{t('settings.copyDiagnostics')}</Button></div></section>
      <section className={styles.settingsGroup}><div className={styles.settingsGroupHead}><h2>{t('settings.about')}</h2><Button variant="ghost" size="icon-sm" title={t('settings.openGithub')} aria-label={t('settings.openGithub')} onClick={() => void openExternal('https://github.com/599yongyang/larder')}><GitHubIcon size={14} /></Button></div><div className={styles.aboutIdentity}><div><strong>Larder</strong><small>{t('common.version')} {info?.version || 'dev'}</small></div></div><dl className={`${styles.settingsDefinition} ${styles.aboutDefinition}`}><dt>{t('settings.dataDirectory')}</dt><dd><code>{info?.dataDir || '—'}</code></dd><dt>Schema</dt><dd><code>{info?.schemaVersion || '—'}</code></dd><dt>{t('settings.runtime')}</dt><dd>{info ? `${info.platform} · ${info.goVersion}` : '—'}</dd></dl><div className={`${styles.settingsRow} ${styles.aboutUpdate}`}><span><strong>{t('settings.appUpdates')}</strong><small>{appUpdate ? (appUpdate.available ? t('settings.appUpdateAvailable', { version: appUpdate.latest }) : t('settings.appUpToDate')) : t('settings.appUpdatesHint')}</small></span>{appUpdate?.available ? <Button variant="default" size="sm" onClick={() => void openExternal(appUpdate.url)}><Download size={14} />{t('settings.downloadUpdate')}</Button> : <Button variant="outline" size="sm" disabled={checkingAppUpdate} onClick={() => void handleCheckAppUpdate()}>{checkingAppUpdate ? t('settings.checkingForAppUpdates') : t('settings.checkForAppUpdates')}</Button>}</div></section>
    </div>
    {restore && <Dialog open onOpenChange={(open) => { if (!open && !restoring) setRestore(undefined) }}>
      <DialogContent className="max-w-md"><DialogHeader><DialogTitle>{t('settings.restoreThisBackup')}</DialogTitle><DialogDescription>{t('settings.mcpConfigsAndInstructionFilesInTheBackupWillOverwriteCurrentFilesCurrentFilesAreSnapshottedFirst')}</DialogDescription></DialogHeader><div className="px-6 py-2"><dl className={styles.settingsDefinition}><dt>{t('settings.created')}</dt><dd>{new Date(restore.info.createdAt).toLocaleString(getLanguage() === 'en' ? 'en-US' : 'zh-CN')}</dd><dt>{t('settings.mcpConfigs')}</dt><dd>{t('settings.message', { n: restore.info.configs })}</dd><dt>{t('common.instructionFiles')}</dt><dd>{t('settings.message', { n: restore.info.instructions })}</dd><dt>{t('settings.appFiles')}</dt><dd>{t('settings.message', { n: restore.info.apps })}</dd></dl></div><DialogFooter><Button size="sm" variant="outline" disabled={restoring} onClick={() => setRestore(undefined)}>{t('common.cancel')}</Button><Button size="sm" disabled={restoring} onClick={() => void applyRestore()}>{restoring ? t('common.restoring') : t('settings.restore')}</Button></DialogFooter></DialogContent>
    </Dialog>}
    <p className={styles.settingsFootnote}>{t('settings.allScansAndConfigurationStayOnThisDevice')}</p>
  </section>
}

export { SettingsView }
