import { useEffect, useState } from 'react'
import { Loader2, RefreshCw } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { AgentIcon } from './AgentIcon'
import { findAgentCLI } from '../lib/agentCLI'
import { backend } from '../backend'
import { t } from '../i18n'
import { gooeyToast } from 'goey-toast'
import { backendErrorMessage } from '../lib/backendError'
import styles from '../App.module.css'
import type { CLIInfo, NativeCLIUpdateInfo, Root, UpdateReport } from '../types'

export function AgentCLIBar({ root, clis, updates, loading, onCheck, onUpdated }: { root: Root; clis: CLIInfo[]; updates?: UpdateReport; loading: boolean; onCheck: () => void; onUpdated: () => void }) {
  const cli = findAgentCLI(root, clis)
  const update = cli ? updates?.items.find((item) => item.kind === 'cli' && (item.cliId === cli.id || item.name === cli.name)) : undefined
  const checkable = cli?.owner === 'Homebrew' || cli?.owner === 'npm' || cli?.owner === 'winget'
  const broken = cli?.status === 'broken'
  const [probedVersion, setProbedVersion] = useState('')
  const [nativeInfo, setNativeInfo] = useState<NativeCLIUpdateInfo>()
  const [nativeChecking, setNativeChecking] = useState(false)
  const [nativeError, setNativeError] = useState(false)
  const [updating, setUpdating] = useState(false)
  const [refreshKey, setRefreshKey] = useState(0)

  useEffect(() => {
    if (!cli || cli.version || broken) return
    let active = true
    backend.cliVersion(cli.id).then((next) => { if (active) setProbedVersion(next) }).catch(() => {})
    return () => { active = false }
  }, [cli?.id, cli?.version, broken])

  useEffect(() => {
    setNativeInfo(undefined)
    setNativeError(false)
    setNativeChecking(false)
    if (!cli || broken) return
    if (update && update.state !== 'unknown' && update.state !== 'error') return
    let active = true
    setNativeChecking(true)
    backend.nativeCLIUpdateCheck(cli.id).then((info) => { if (!active) return; if (info.error) { setNativeError(true); setNativeInfo(undefined) } else { setNativeInfo(info) } }).catch(() => { if (active) setNativeError(true) }).finally(() => { if (active) setNativeChecking(false) })
    return () => { active = false }
  }, [cli?.id, broken, update?.state, refreshKey])

  const version = cli?.version || probedVersion
  const normalizeVersion = (value?: string) => (value || '').trim().replace(/^v/i, '')
  const isNewer = (latest?: string) => Boolean(latest && version && normalizeVersion(latest) !== normalizeVersion(version))
  const canUpdate = (update?.state === 'available' || update?.state === 'changed') && isNewer(update.latestVersion)
  const nativeAvailable = Boolean(nativeInfo?.available && isNewer(nativeInfo.latest))
  const nativeCanUpdate = Boolean(nativeAvailable && nativeInfo?.canUpdate)
  const hasUpdate = canUpdate || nativeAvailable
  const latest = canUpdate ? update?.latestVersion : nativeAvailable ? nativeInfo?.latest : undefined
  const checked = Boolean(nativeInfo || (update && update.state !== 'unknown' && update.state !== 'error'))
  const busy = loading || nativeChecking

  const check = () => {
    if (!cli || broken) return
    if (checkable) onCheck()
    setNativeChecking(true)
    setNativeError(false)
    backend.nativeCLIUpdateCheck(cli.id).then((info) => { if (info.error) { setNativeError(true); setNativeInfo(undefined) } else { setNativeInfo(info) } }).catch(() => setNativeError(true)).finally(() => setNativeChecking(false))
  }

  const updateNow = async () => {
    if (!cli) return
    setUpdating(true)
    const toast = gooeyToast(t('agent.updating', { name: cli.name }), { duration: 120000 })
    try {
      if (canUpdate && update?.latestVersion) {
        const plan = await backend.planCLI(cli.id, false, update.latestVersion)
        await backend.applyPlan(plan.id)
      } else {
        await backend.nativeCLIUpdate(cli.id)
      }
      gooeyToast.update(toast, { title: t('agent.updated', { name: cli.name }), type: 'success' })
      setNativeInfo(undefined)
      setRefreshKey((value) => value + 1)
      onUpdated()
    } catch (cause) {
      gooeyToast.update(toast, { title: t('agent.updateFailed', { name: cli.name }), type: 'error', description: backendErrorMessage(cause, t('agent.updateFailed', { name: cli.name })) })
    } finally {
      setUpdating(false)
    }
  }

  const status = broken
    ? t('agent.cliBroken')
    : !cli
      ? t('agent.cliNotFound')
      : !version
        ? t('agent.versionUnavailable')
        : hasUpdate
          ? t('agent.nativeHasUpdate')
          : busy
            ? t('agent.nativeChecking')
            : checked
              ? t('agent.upToDate')
              : nativeError
                ? t('agent.checkUnavailable')
                : t('agent.notChecked')

  return <div className={styles.agentTopStatus}>
    <AgentIcon name={root.logicalAgent || root.label} className={styles.agentTopIcon} />
    <div className={styles.agentTopCopy}>
      <span className={styles.agentTopName}>{root.logicalAgent || root.label}</span>
      <span className={styles.agentTopMeta}>
        {!broken && cli && version && <span>{version}</span>}
        {hasUpdate && latest && <><span className={styles.agentCLIArrow}>→</span><strong>{latest}</strong></>}
        {(version || hasUpdate) && <span className={styles.agentTopDot}>·</span>}
        <span className={styles.agentTopStatusText}>{status}</span>
        <span className={styles.agentTopActions}>
          {hasUpdate && (canUpdate || nativeCanUpdate) && <Button size="xs" disabled={updating} onClick={() => void updateNow()}>{updating ? t('common.updating') : t('agent.update')}</Button>}
          {hasUpdate && !canUpdate && !nativeCanUpdate && <span className={styles.agentCLIUpToDate}>{t('agent.manualUpdate')}</span>}
          {!broken && cli && <Button size="icon-xs" variant="ghost" disabled={busy || updating} title={t('agent.checkUpdate')} aria-label={t('agent.checkUpdate')} onClick={check}>{busy ? <Loader2 size={12} className="animate-spin" /> : <RefreshCw size={12} />}</Button>}
        </span>
      </span>
    </div>
  </div>
}
