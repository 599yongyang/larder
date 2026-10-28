import { copyText as writeClipboard } from '../../lib/clipboard'
import { useEffect, useRef, useState } from 'react'
import { Cloud, Copy, Database, Download, Info, Keyboard, Monitor, Moon, Palette, RefreshCw, Settings2, Sparkles, Sun, Upload, Waves, X } from 'lucide-react'
import { GitHubIcon } from '../../components/GitHubIcon'
import { Button } from '@/components/ui/button'
import { Kbd } from '@/components/ui/kbd'
import { Checkbox } from '@/components/ui/checkbox'
import { backend } from '../../backend'
import { toastError, toastSuccess } from '../../lib/toast'
import { NumberField } from '@/components/ui/number-field'
import { ShortcutRecorder } from '@/components/ui/shortcut-recorder'
import { SegmentedControl } from '@/components/ui/segmented-control'
import { DataPackageDialog } from '../../components/DataPackageDialog'
import { SyncSettings } from '../../components/SyncSettings'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import styles from '../../App.module.css'
import type { AppInfo, DataPackageInfo, DataPackageSelection, Settings } from '../../types'
import type { ThemeMode } from '../../lib/theme'
import { getLanguage, getLanguageSetting, languages, setLanguage, t } from '../../i18n'
import { openExternal } from '../../lib/appUpdate'
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

type SettingsSectionId = 'appearance' | 'general' | 'shortcuts' | 'updates' | 'sync' | 'data' | 'about'

const settingsSections: Array<{ id: SettingsSectionId; label: string; icon: typeof Palette }> = [
  { id: 'appearance', label: 'settings.nav.appearance', icon: Palette },
  { id: 'general', label: 'settings.nav.general', icon: Settings2 },
  { id: 'shortcuts', label: 'settings.nav.shortcuts', icon: Keyboard },
  { id: 'updates', label: 'settings.nav.updates', icon: RefreshCw },
  { id: 'sync', label: 'settings.nav.sync', icon: Cloud },
  { id: 'data', label: 'settings.nav.data', icon: Database },
  { id: 'about', label: 'settings.nav.about', icon: Info },
]

function SettingsView({ settings, onToggle, onUsageToggle, onCheckAppUpdate, checkingAppUpdate, onGlobalShortcutToggle, onGlobalShortcutChange, onAutoCheckToggle, onInterval, onRetention, onTrashRetention, themeMode, onThemeMode, info, onRestored }: { settings: Settings; onToggle: (value: boolean) => void; onUsageToggle: (value: boolean) => void; onCheckAppUpdate: () => void; checkingAppUpdate: boolean; onGlobalShortcutToggle: (value: boolean) => void; onGlobalShortcutChange: (value: string) => void; onAutoCheckToggle: (value: boolean) => void; onInterval: (value: number) => void; onRetention: (value: number) => void; onTrashRetention: (value: number) => void; themeMode: ThemeMode; onThemeMode: (value: ThemeMode, event?: React.MouseEvent<HTMLElement>) => void; info?: AppInfo; onRestored: () => void }) {
  const [motion, setMotion] = useState<MotionMode>(getMotionMode)
  const [zoom, setZoomState] = useState<ZoomLevel>(getZoom)
  const [autostart, setAutostart] = useState(false)
  const [autostartLoading, setAutostartLoading] = useState(true)
  useEffect(() => {
    backend.autostartEnabled().then(setAutostart).catch(() => {}).finally(() => setAutostartLoading(false))
  }, [])
  const toggleAutostart = async (checked: boolean) => {
    setAutostartLoading(true)
    try { await backend.setAutostart(checked); setAutostart(checked) } catch (error) { toastError(error, t('settings.autostartFailed')) } finally { setAutostartLoading(false) }
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
    try { await writeClipboard(JSON.stringify(payload, null, 2)); toastSuccess(t('settings.diagnosticsCopied')) } catch (error) { toastError(error, t('settings.diagnosticsCopyFailed')) }
  }
  const [dataPackage, setDataPackage] = useState<{ mode: 'export' | 'import'; path?: string; info?: DataPackageInfo }>()
  const [dataPackageBusy, setDataPackageBusy] = useState(false)
  const startDataImport = async () => {
    try {
      const path = await backend.pickDataPackage()
      if (!path) return
      const info = await backend.inspectDataPackage(path)
      setDataPackage({ mode: 'import', path, info })
    } catch (error) { toastError(error, t('dataPackage.inspectFailed')) }
  }
  const applyDataPackage = async (selection: DataPackageSelection) => {
    if (!dataPackage) return
    setDataPackageBusy(true)
    try {
      if (dataPackage.mode === 'export') {
        const path = await backend.exportDataPackage(selection)
        if (path) toastSuccess(t('dataPackage.exported'), path)
      } else if (dataPackage.path) {
        const result = await backend.importDataPackage(dataPackage.path, selection)
        toastSuccess(t('dataPackage.imported', { n: result.restored }), result.snapshotDir)
        onRestored()
      }
      setDataPackage(undefined)
    } catch (error) { toastError(error, t('dataPackage.failed')) } finally { setDataPackageBusy(false) }
  }
  const aboutSectionRef = useRef<HTMLElement>(null)
  useEffect(() => {
    const section = aboutSectionRef.current
    if (!section) return
    const scrollContainer = section.closest('[class*="viewBody"]') as HTMLElement | null
    const sync = () => {
      if (scrollContainer) section.style.minHeight = `${scrollContainer.clientHeight}px`
    }
    sync()
    const observer = typeof ResizeObserver !== 'undefined' && scrollContainer ? new ResizeObserver(sync) : null
    observer?.observe(scrollContainer as Element)
    window.addEventListener('resize', sync)
    return () => { observer?.disconnect(); window.removeEventListener('resize', sync) }
  }, [])
  const [activeSettingsSection, setActiveSettingsSection] = useState<SettingsSectionId>('appearance')
  useEffect(() => {
    if (typeof IntersectionObserver === 'undefined') return
    const sections = settingsSections.map(({ id }) => document.getElementById(`settings-${id}`)).filter((element): element is HTMLElement => Boolean(element))
    const observer = new IntersectionObserver((entries) => {
      const visible = entries.filter((entry) => entry.isIntersecting).sort((a, b) => a.boundingClientRect.top - b.boundingClientRect.top)
      const id = visible[0]?.target.getAttribute('data-settings-section') as SettingsSectionId | null
      if (id) setActiveSettingsSection(id)
    }, { rootMargin: '-12% 0px -70% 0px', threshold: 0 })
    sections.forEach((section) => observer.observe(section))
    return () => observer.disconnect()
  }, [])
  const scrollToSettingsSection = (id: SettingsSectionId) => {
    setActiveSettingsSection(id)
    const section = document.getElementById(`settings-${id}`)
    if (!section) return
    const reduceMotion = window.matchMedia?.('(prefers-reduced-motion: reduce)').matches
    section.scrollIntoView({ behavior: reduceMotion ? 'auto' : 'smooth', block: 'start' })
  }
  return <section className={styles.settingsPanel}>
    <div className={styles.settingsLayout}>
      <nav className={styles.settingsNav} aria-label={t('common.settings')}>
        {settingsSections.map(({ id, label, icon: Icon }) => <button type="button" key={id} id={`settings-tab-${id}`} aria-controls="settings-panel" className={`${styles.settingsNavButton} ${activeSettingsSection === id ? styles.settingsNavButtonActive : ''}`} aria-current={activeSettingsSection === id ? 'true' : undefined} onClick={() => scrollToSettingsSection(id)}><Icon size={15} aria-hidden="true" /><span>{t(label)}</span></button>)}
      </nav>
      <div className={styles.settingsContent} id="settings-panel">
        <section id="settings-appearance" className={styles.settingsSection} data-settings-section="appearance" aria-label={t('settings.nav.appearance')}>
          <div className={styles.settingsGrid}>
          <section className={styles.settingsGroup}><h2>{t('settings.appearance')}</h2><SegmentedControl columns={3} ariaLabel={t('settings.appearance')} value={themeMode} onChange={(value) => onThemeMode(value)} options={themeOptions.map(({ value, labelKey, icon: Icon }) => ({ value, label: t(labelKey), icon: <Icon size={14} aria-hidden="true" /> }))} /></section>
          <section className={styles.settingsGroup}><h2>{t('settings.language')}</h2><SegmentedControl columns={3} ariaLabel={t('settings.language')} value={getLanguageSetting()} onChange={(value) => { setLanguage(value); void backend.setNativeLocale(value === 'system' ? getLanguage() : value).catch(() => {}) }} options={languages.map(({ value, label }) => ({ value, label: value === 'system' ? t(label) : label }))} /></section>
          <section className={styles.settingsGroup}><h2>{t('settings.display')}</h2><label className={styles.settingsRow}><span><strong>{t('settings.showAgentLocalSkills')}</strong><small>{t('settings.onlySharedSkillsAreShownByDefaultLocalSkillsStillAppearInsideAnAgent')}</small></span><Checkbox checked={settings.showAgentBuiltins} onCheckedChange={(checked) => onToggle(Boolean(checked))} /></label></section>
          <section className={styles.settingsGroup}><h2>{t('settings.interfaceMotion')}</h2><SegmentedControl columns={3} ariaLabel={t('settings.interfaceMotion')} value={motion} onChange={(value) => { setMotionMode(value); setMotion(value) }} options={motionOptions.map(({ value, labelKey, icon: Icon }) => ({ value, label: t(labelKey), icon: <Icon size={14} aria-hidden="true" /> }))} /></section>
          <section className={styles.settingsGroup}><h2>{t('settings.uiScale')}</h2><SegmentedControl columns={3} className="max-w-[280px]" ariaLabel={t('settings.uiScale')} value={String(zoom)} onChange={(value) => changeZoom(Number(value) as typeof zoom)} options={zoomLevels.map((value) => ({ value: String(value), label: `${value}%` }))} /></section>
          </div>
        </section>
        <section id="settings-general" className={styles.settingsSection} data-settings-section="general" aria-label={t('settings.nav.general')}>
          <div className={styles.settingsGrid}>
          <section className={styles.settingsGroup}><h2>{t('settings.startup')}</h2><label className={styles.settingsRow}><span><strong>{t('settings.openAtLogin')}</strong><small>{t('settings.openAtLoginHint')}</small></span><Checkbox checked={autostart} disabled={autostartLoading} onCheckedChange={(checked) => void toggleAutostart(Boolean(checked))} /></label></section>
          <section className={styles.settingsGroup}><h2>{t('settings.localUsageStats')}</h2><label className={styles.settingsRow}><span><strong>{t('settings.enableLocalUsageStats')}</strong><small>{t('settings.readsLocalZshHistoryOnlyNothingIsUploadedOrWrittenBack')}</small></span><Checkbox checked={settings.usageStatsEnabled} onCheckedChange={(checked) => onUsageToggle(Boolean(checked))} /></label></section>
          </div>
        </section>
        <section id="settings-shortcuts" className={styles.settingsSection} data-settings-section="shortcuts" aria-label={t('settings.nav.shortcuts')}>
          <div className={styles.settingsGrid}>
          <section className={styles.settingsGroup}><h2>{t('settings.keyboardShortcuts')}</h2><div className={styles.shortcutList}><div className={styles.shortcutRow}><span>{t('command.searchAndJump')}</span><Kbd className={styles.shortcutKbd}>{mod} K</Kbd></div><div className={styles.shortcutRow}><span>{t('settings.shortcutFocusSearch')}</span><Kbd className={styles.shortcutKbd}>{mod} F</Kbd></div><div className={styles.shortcutRow}><span>{t('common.rescan')}</span><Kbd className={styles.shortcutKbd}>{mod} R</Kbd></div><div className={styles.shortcutRow}><span>{t('settings.shortcutSwitchSections')}</span><Kbd className={styles.shortcutKbd}>{mod} 1–6</Kbd></div><div className={styles.shortcutRow}><span>{t('settings.shortcutZoomIn')}</span><Kbd className={styles.shortcutKbd}>{mod} +</Kbd></div><div className={styles.shortcutRow}><span>{t('settings.shortcutZoomOut')}</span><Kbd className={styles.shortcutKbd}>{mod} −</Kbd></div><div className={styles.shortcutRow}><span>{t('settings.shortcutResetZoom')}</span><Kbd className={styles.shortcutKbd}>{mod} 0</Kbd></div></div></section>
          <section className={styles.settingsGroup}><h2>{t('settings.globalShortcut')}</h2><label className={styles.settingsRow}><span><strong>{t('settings.enableGlobalShortcut')}</strong><small>{t('settings.globalShortcutEnabledHint')}</small></span><Checkbox checked={settings.globalShortcutEnabled !== false} onCheckedChange={(checked) => onGlobalShortcutToggle(Boolean(checked))} /></label><div className={styles.settingsRow}><span><strong>{t('settings.globalShortcut')}</strong><small>{t('settings.globalShortcutHint')}</small></span><ShortcutRecorder value={settings.globalShortcut || 'CmdOrCtrl+Shift+L'} disabled={settings.globalShortcutEnabled === false} ariaLabel={t('settings.globalShortcut')} recordingLabel={t('settings.pressShortcut')} resetLabel={t('settings.resetShortcut')} onChange={onGlobalShortcutChange} onReset={() => onGlobalShortcutChange('CmdOrCtrl+Shift+L')} /></div></section>
          </div>
        </section>
        <section id="settings-updates" className={styles.settingsSection} data-settings-section="updates" aria-label={t('settings.nav.updates')}>
          <div className={styles.settingsGrid}>
          <section className={styles.settingsGroup}><h2>{t('settings.autoCheck')}</h2><label className={styles.settingsRow}><span><strong>{t('settings.enableAutoCheck')}</strong><small>{t('settings.manualChecksAreStillAvailableWhenOff')}</small></span><Checkbox checked={settings.autoCheckUpdates} onCheckedChange={(checked) => onAutoCheckToggle(Boolean(checked))} /></label><div className={styles.settingsRow}><span><strong>{t('settings.checkInterval')}</strong><small>{t('settings.hours')}</small></span><NumberField value={settings.updateCheckIntervalHours || 24} min={1} max={168} suffix="h" ariaLabel={t('settings.checkInterval')} decreaseLabel={t('common.decrease')} increaseLabel={t('common.increase')} onChange={onInterval} /></div></section>
          </div>
        </section>
        <section id="settings-sync" className={styles.settingsSection} data-settings-section="sync" aria-label={t('settings.nav.sync')}>
          <div className={styles.settingsGrid}>
            <SyncSettings onRestored={onRestored} />
            </div>
        </section>
        <section id="settings-data" className={styles.settingsSection} data-settings-section="data" aria-label={t('settings.nav.data')}>
          <div className={styles.settingsGrid}>
          <section className={styles.settingsGroup}><h2>{t('settings.data')}</h2><div className={styles.settingsRow}><span><strong>{t('settings.snapshotRetention')}</strong></span><NumberField value={settings.snapshotRetention || 50} min={5} max={500} suffix={t('settings.itemUnit')} ariaLabel={t('settings.snapshotRetention')} decreaseLabel={t('common.decrease')} increaseLabel={t('common.increase')} onChange={onRetention} /></div><div className={styles.settingsRow}><span><strong>{t('settings.trashRetention')}</strong></span><Select value={String(settings.trashRetentionDays ?? 30)} onValueChange={(value) => onTrashRetention(Number(value))}><SelectTrigger size="sm" className="w-[104px] text-[11px]" aria-label={t('settings.trashRetention')}><SelectValue /></SelectTrigger><SelectContent><SelectItem value="7">{t('settings.days', { n: 7 })}</SelectItem><SelectItem value="30">{t('settings.days', { n: 30 })}</SelectItem><SelectItem value="90">{t('settings.days', { n: 90 })}</SelectItem><SelectItem value="-1">{t('common.forever')}</SelectItem></SelectContent></Select></div></section>
          <section className={styles.settingsGroup}><h2>{t('settings.dataPackage')}</h2><div className={styles.settingsRow}><span><strong>{t('dataPackage.exportTitle')}</strong><small>{t('dataPackage.exportHint')}</small></span><Button variant="outline" size="sm" onClick={() => setDataPackage({ mode: 'export' })}><Download size={14} />{t('dataPackage.export')}</Button></div><div className={styles.settingsRow}><span><strong>{t('dataPackage.importTitle')}</strong><small>{t('dataPackage.importHint')}</small></span><Button variant="outline" size="sm" onClick={() => void startDataImport()}><Upload size={14} />{t('dataPackage.import')}</Button></div></section>
          <section className={`${styles.settingsGroup} ${styles.settingsWide}`}><h2>{t('settings.diagnostics')}</h2><div className={styles.settingsRow}><span><strong>{t('settings.copyDiagnostics')}</strong></span><Button variant="outline" size="sm" onClick={() => void copyDiagnostics()}><Copy size={14} />{t('settings.copyDiagnostics')}</Button></div></section>
          </div>
        </section>
        <section ref={aboutSectionRef} id="settings-about" className={styles.settingsSection} data-settings-section="about" aria-label={t('settings.nav.about')}>
          <div className={styles.settingsGrid}>
          <section className={`${styles.settingsGroup} ${styles.settingsWide}`}><div className={styles.settingsGroupHead}><h2>{t('settings.about')}</h2><Button variant="ghost" size="icon-sm" title={t('settings.openGithub')} aria-label={t('settings.openGithub')} onClick={() => void openExternal('https://github.com/599yongyang/larder')}><GitHubIcon size={14} /></Button></div><div className={styles.aboutIdentity}><div><strong>Larder</strong><small>{t('common.version')} {info?.version || 'dev'}</small></div></div><dl className={`${styles.settingsDefinition} ${styles.aboutDefinition}`}><dt>{t('settings.dataDirectory')}</dt><dd><code>{info?.dataDir || '—'}</code></dd><dt>Schema</dt><dd><code>{info?.schemaVersion || '—'}</code></dd><dt>{t('settings.runtime')}</dt><dd>{info ? `${info.platform} · ${info.goVersion}` : '—'}</dd></dl><div className={`${styles.settingsRow} ${styles.aboutUpdate}`}><span><strong>{t('settings.appUpdates')}</strong><small>{t('settings.appUpdatesHint')}</small></span><Button variant="outline" size="sm" disabled={checkingAppUpdate} onClick={onCheckAppUpdate}>{checkingAppUpdate ? t('settings.checkingForAppUpdates') : t('settings.checkForAppUpdates')}</Button></div></section>
          </div>
        </section>

    </div>
    </div>
    <DataPackageDialog mode={dataPackage?.mode || 'export'} open={Boolean(dataPackage)} info={dataPackage?.info} busy={dataPackageBusy} onClose={() => { if (!dataPackageBusy) setDataPackage(undefined) }} onConfirm={(selection) => void applyDataPackage(selection)} />
  </section>
}

export { SettingsView }