import { useState } from 'react'
import { Archive, Monitor, Moon, RotateCcw, Sun } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Checkbox } from '@/components/ui/checkbox'
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { backend } from '../../backend'
import { toastError, toastSuccess } from '../../lib/toast'
import { Input } from '@/components/ui/input'
import styles from '../../App.module.css'
import type { AppInfo, BackupInfo, Settings } from '../../types'
import type { ThemeMode } from '../../lib/theme'

const themeOptions: Array<{ value: ThemeMode; label: string; icon: typeof Sun }> = [
  { value: 'system', label: '系统', icon: Monitor },
  { value: 'light', label: '明亮', icon: Sun },
  { value: 'dark', label: '暗黑', icon: Moon },
]

function SettingsView({ settings, onToggle, onUsageToggle, onAutoCheckToggle, onInterval, onRetention, themeMode, onThemeMode, info, onBackup, onRestored }: { settings: Settings; onToggle: (value: boolean) => void; onUsageToggle: (value: boolean) => void; onAutoCheckToggle: (value: boolean) => void; onInterval: (value: number) => void; onRetention: (value: number) => void; themeMode: ThemeMode; onThemeMode: (value: ThemeMode, event?: React.MouseEvent<HTMLElement>) => void; info?: AppInfo; onBackup: () => void; onRestored: () => void }) {
  const [restore, setRestore] = useState<{ path: string; info: BackupInfo }>()
  const [restoring, setRestoring] = useState(false)
  const chooseBackup = async () => { setRestoring(true); try { const path = await backend.pickBackupFile(); if (!path) return; const info = await backend.inspectBackup(path); setRestore({ path, info }) } catch (error) { toastError(error, '读取备份失败') } finally { setRestoring(false) } }
  const applyRestore = async () => { if (!restore) return; setRestoring(true); try { const result = await backend.restoreBackup(restore.path); toastSuccess(`已恢复 ${result.restored} 个文件`, result.snapshotDir); setRestore(undefined); onRestored() } catch (error) { toastError(error, '恢复备份失败') } finally { setRestoring(false) } }
  return <section className={styles.settingsPanel}>
    <div className={styles.settingsGrid}>
      <section className={styles.settingsGroup}><h2>外观</h2><div className={styles.themeOptions} role="group" aria-label="外观模式">{themeOptions.map(({ value, label, icon: Icon }) => <button type="button" key={value} className={`${styles.themeOption} ${themeMode === value ? styles.themeOptionActive : ''}`} aria-pressed={themeMode === value} onClick={(event) => onThemeMode(value, event)}><Icon size={14} aria-hidden="true" /><span>{label}</span></button>)}</div><p className={styles.settingsHint}>跟随系统会随 macOS 自动切换。</p></section>
      <section className={styles.settingsGroup}><h2>显示</h2><label className={styles.settingsRow}><span><strong>显示 Agent 本地 Skills</strong><small>默认只显示共享 Skills；进入 Agent 后仍显示本地 Skill。</small></span><Checkbox checked={settings.showAgentBuiltins} onCheckedChange={(checked) => onToggle(Boolean(checked))} /></label></section>
      <section className={styles.settingsGroup}><h2>本地使用统计</h2><label className={styles.settingsRow}><span><strong>启用本地使用统计</strong><small>只读取本机 zsh history，不上传、不写回。</small></span><Checkbox checked={settings.usageStatsEnabled} onCheckedChange={(checked) => onUsageToggle(Boolean(checked))} /></label></section>
      <section className={styles.settingsGroup}><h2>备份</h2><div className={styles.settingsRow}><span><strong>导出配置备份</strong><small>把 MCP 配置、指令文件和应用元数据打包到本地。</small></span><Button variant="outline" size="sm" onClick={onBackup}><Archive size={14} />导出备份</Button></div><div className={styles.settingsRow}><span><strong>恢复配置备份</strong><small>恢复前会把当前文件备份到 restore-snapshots。</small></span><Button variant="outline" size="sm" disabled={restoring} onClick={() => void chooseBackup()}><RotateCcw size={14} />恢复备份</Button></div><p className={styles.settingsHint}>备份中的 MCP 环境变量和 Header 为明文，请妥善保管；恢复会覆盖备份内对应的配置文件。</p></section>
      <section className={styles.settingsGroup}><h2>关于</h2><dl className={styles.settingsDefinition}><dt>版本</dt><dd>{info?.version || 'dev'}</dd><dt>数据目录</dt><dd><code>{info?.dataDir || '—'}</code></dd><dt>Schema</dt><dd><code>{info?.schemaVersion || '—'}</code></dd><dt>运行环境</dt><dd>{info ? `${info.platform} · ${info.goVersion}` : '—'}</dd></dl></section>
      <section className={styles.settingsGroup}><h2>自动检查</h2><label className={styles.settingsRow}><span><strong>启用自动检查</strong><small>关闭后仍可手动检查。</small></span><Checkbox checked={settings.autoCheckUpdates} onCheckedChange={(checked) => onAutoCheckToggle(Boolean(checked))} /></label><div className={styles.settingsRow}><span><strong>检查间隔</strong><small>小时</small></span><div className={styles.intervalField}><Input type="number" min={1} value={String(settings.updateCheckIntervalHours)} onChange={(event) => onInterval(Math.max(1, Number(event.target.value) || 24))} aria-label="检查间隔小时" /><span>h</span></div></div></section>
      <section className={styles.settingsGroup}><h2>数据</h2><div className={styles.settingsRow}><span><strong>快照保留数量</strong><small>超出后自动清理最旧的快照。</small></span><div className={styles.intervalField}><Input type="number" min={5} max={500} value={String(settings.snapshotRetention || 50)} onChange={(event) => onRetention(Math.max(5, Number(event.target.value) || 50))} aria-label="快照保留数量" /><span>个</span></div></div></section>
    </div>
    {restore && <Dialog open onOpenChange={(open) => { if (!open && !restoring) setRestore(undefined) }}>
      <DialogContent className="max-w-md"><DialogHeader><DialogTitle>恢复这份备份？</DialogTitle><DialogDescription>会覆盖备份中的 MCP 配置和指令文件，当前文件会先保存到 restore-snapshots。</DialogDescription></DialogHeader><div className="px-6 py-2"><dl className={styles.settingsDefinition}><dt>创建时间</dt><dd>{new Date(restore.info.createdAt).toLocaleString('zh-CN')}</dd><dt>MCP 配置</dt><dd>{restore.info.configs} 个</dd><dt>指令文件</dt><dd>{restore.info.instructions} 个</dd><dt>应用文件</dt><dd>{restore.info.apps} 个</dd></dl></div><DialogFooter><Button size="sm" variant="outline" disabled={restoring} onClick={() => setRestore(undefined)}>取消</Button><Button size="sm" disabled={restoring} onClick={() => void applyRestore()}>{restoring ? '恢复中…' : '确认恢复'}</Button></DialogFooter></DialogContent>
    </Dialog>}
    <p className={styles.settingsFootnote}>所有扫描与配置都保存在本机。</p>
  </section>
}

export { SettingsView }
