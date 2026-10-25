import { useState } from 'react'
import { AlertTriangle, Copy, Download, Info, Link2, RefreshCw, Server, Trash2, Wrench } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Checkbox } from '@/components/ui/checkbox'
import { Dialog, DialogContent, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { backend } from '../backend'
import styles from '../App.module.css'
import { t } from '../i18n'
import { backendErrorMessage } from '../lib/backendError'
import type { Plan } from '../types'
import { shortPath } from '../lib/format'

const operationMeta = (operation: string) => {
  if (operation === 'uninstall_skill' || operation === 'cli_uninstall') return { icon: Trash2, tone: 'danger', confirm: operation === 'uninstall_skill' ? t('common.uninstall') : t('common.uninstallCli') }
  if (operation === 'mcp_remove') return { icon: Server, tone: 'danger', confirm: t('common.remove') }
  if (operation === 'install_skill') return { icon: Download, tone: 'brand', confirm: t('common.install') }
  if (operation.includes('update')) return { icon: RefreshCw, tone: 'brand', confirm: t('common.updateAction') }
  if (operation === 'disable_link') return { icon: Link2, tone: 'neutral', confirm: t('common.disable') }
  if (operation === 'link_skill' || operation === 'replace_copy') return { icon: Link2, tone: 'brand', confirm: t('common.applyChange') }
  if (operation === 'sync_copy') return { icon: Copy, tone: 'brand', confirm: t('common.sync') }
  return { icon: Wrench, tone: 'neutral', confirm: t('common.run') }
}

export function PlanDialog({ plan, onClose, onApplied }: { plan: Plan; onClose: () => void; onApplied: () => void }) {
  const actionLabels: Record<string, string> = { create_symlink: t('common.createLink'), already_linked: t('common.alreadyLinked'), replace_link: t('common.repairBrokenLink'), replace_copy: t('common.useSharedVersion'), sync_copy: t('common.syncContent'), remove_link: t('common.removeLink'), conflict: t('common.targetConflict'), error: t('common.checkFailed'), install_skill: t('common.installFromSource'), uninstall_skill: t('common.removeFromSharedLibrary'), remove_mcp_entry: t('common.removeConfig'), cli_uninstall: t('common.uninstallCliAction') }
  const [confirmed, setConfirmed] = useState(false)
  const [applying, setApplying] = useState(false)
  const [error, setError] = useState('')
  const meta = operationMeta(plan.operation)
  const needsExplicitConfirm = meta.tone === 'danger'
  const OperationIcon = meta.icon
  const warnings = plan.warningMessages?.length ? plan.warningMessages.map((message) => t(message.code, message.params)) : (plan.warnings ?? []).filter((item) => !item.includes('快照') && !item.includes('snapshot'))
  const coreChanges = (plan.changes ?? []).filter((change) => change.action !== 'already_linked')
  const overwritten = coreChanges.filter((change) => change.exists).length
  const changeVerb = (change: { action: string; exists: boolean }) => {
    if (['uninstall_skill', 'remove_mcp_entry', 'cli_uninstall', 'remove_link'].includes(change.action)) return ''
    if (['replace_link', 'replace_copy', 'sync_copy', 'install_skill'].includes(change.action)) return t('plan.overwrite')
    return change.exists ? t('plan.overwrite') : t('plan.create')
  }
  const apply = async () => {
    if (!confirmed || applying) return
    setApplying(true)
    setError('')
    try {
      await backend.applyPlan(plan.id)
      onApplied()
    } catch (cause) {
      setError(backendErrorMessage(cause, t('common.executionFailed')))
    } finally {
      setApplying(false)
    }
  }
  return <Dialog open onOpenChange={(open) => { if (!open && !applying) onClose() }}>
    <DialogContent className={`${styles.planDialog} max-w-[560px] gap-0 p-0`} showCloseButton onOpenAutoFocus={(event) => { event.preventDefault(); (event.target as HTMLElement).focus() }}>
      <DialogHeader className={styles.planHeader}>
        <span className={`${styles.planIcon} ${styles[`planIcon_${meta.tone}`]}`}><OperationIcon size={18} strokeWidth={1.8} aria-hidden="true" /></span>
        <div className={styles.planHeading}>
          <DialogTitle className={styles.planTitle}>{plan.titleMessage?.code ? t(plan.titleMessage.code, plan.titleMessage.params) : plan.title}</DialogTitle><p className={styles.planSummaryText}>{plan.summaryMessage?.code ? t(plan.summaryMessage.code, plan.summaryMessage.params) : plan.summary}</p>
        </div>
      </DialogHeader>
      <div className={styles.planBody}>
        {(coreChanges.length > 0 || plan.command) && <div className={styles.planFlat}>
          {coreChanges.length > 0 && <div className={styles.planSummary}><span>{t('plan.changeSummary', { n: coreChanges.length })}</span>{overwritten > 0 && <span>{t('plan.overwriteSummary', { n: overwritten })}</span>}{warnings.length > 0 && <span>{t('plan.warningSummary', { n: warnings.length })}</span>}</div>}
          {coreChanges.length > 0 && <ul className={styles.planFlatList}>{coreChanges.map((change) => {
            const target = change.target?.split('/').filter(Boolean).pop() || ''
            const label = change.rootLabel || plan.skillName || plan.mcpName || target || t('common.target')
            return <li key={`${change.rootId}:${change.target}`} title={change.target}><span>{label}</span><em>{actionLabels[change.action] ?? change.action}</em>{change.target && <small>{changeVerb(change) ? `${changeVerb(change)} · ` : ''}{shortPath(change.target)}</small>}</li>
          })}</ul>}
          {coreChanges.some((change) => change.action === 'install_skill' && change.source) && <p>{t('common.source')}{coreChanges.find((change) => change.action === 'install_skill' && change.source)?.source}</p>}
          {plan.command && <p className={styles.planFlatCommand}>{t('common.command')}<code>{[plan.command, ...(plan.args ?? [])].join(' ')}</code></p>}
        </div>}
        {warnings.length > 0 && <div className={styles.planWarning}><AlertTriangle size={15} aria-hidden="true" /><div>{warnings.map((item) => <p key={item}>{item}</p>)}</div></div>}
        {needsExplicitConfirm ? <label className={styles.planConfirm}><Checkbox checked={confirmed} onCheckedChange={(checked) => setConfirmed(Boolean(checked))} /><span className={styles.planConfirmLabel}>{t('common.confirmOperationSnapshotFirst')}</span><span className={styles.planConfirmInfo} title={t('common.aSnapshotIsCreatedFirstFailuresFollowTheOperationPolicy')} aria-label={t('common.aSnapshotIsCreatedFirstFailuresFollowTheOperationPolicy')}><Info size={12} /></span></label> : <div className={styles.planConfirm}><Info size={12} aria-hidden="true" /><span className={styles.planConfirmLabel}>{t('common.aSnapshotIsCreatedFirstFailuresFollowTheOperationPolicy')}</span></div>}
        {error && <p className={styles.planError} role="alert">{error}</p>}
      </div>
      <DialogFooter className={styles.planFooter}><Button size="sm" variant="outline" onClick={onClose} disabled={applying}>{t('common.cancel')}</Button><Button size="sm" variant={meta.tone === 'danger' ? 'destructive' : 'default'} disabled={(needsExplicitConfirm && !confirmed) || applying} onClick={() => void apply()}>{applying ? t('common.running') : meta.confirm}</Button></DialogFooter>
    </DialogContent>
  </Dialog>
}
