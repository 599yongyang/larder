import { useEffect, useRef, useState } from 'react'
import { ChevronDown, Download, Loader2 } from 'lucide-react'
import { Events } from '@wailsio/runtime'
import { Checkbox } from '@/components/ui/checkbox'
import { RepoIcon } from './SkillIcon'
import { Button } from '@/components/ui/button'
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { backend } from '../backend'
import styles from '../App.module.css'
import { t } from '../i18n'
import { backendErrorMessage } from '../lib/backendError'
import { toastError, toastSuccess } from '../lib/toast'
import type { Plan, UpdateItem } from '../types'

export function NewUpstreamSkillsDialog({ items, onClose, onPlan, onInstalled }: { items: UpdateItem[]; onClose: () => void; onPlan: (plan: Plan) => void; onInstalled?: () => void | Promise<void> }) {
  const [busy, setBusy] = useState('')
  const [selected, setSelected] = useState<Set<string>>(() => new Set())
  const [batchProgress, setBatchProgress] = useState<{ done: number; total: number }>()
  const batchBase = useRef(0)
  const batchTotal = useRef(0)
  const [expanded, setExpanded] = useState<Set<string>>(() => new Set())
  useEffect(() => {
    const off = Events.On('larder:install-progress', (event: { data?: { done?: number; total?: number } }) => {
      const data = event?.data
      if (!data || typeof data.done !== 'number' || typeof data.total !== 'number' || batchTotal.current === 0) return
      setBatchProgress({ done: Math.min(batchTotal.current, batchBase.current + data.done), total: batchTotal.current })
    })
    return off
  }, [])
  const keyOf = (item: UpdateItem) => `${item.package}:${item.skillPath}`
  const sourceSkillPath = (item: UpdateItem) => item.skillPath?.endsWith('/SKILL.md') ? item.skillPath : `${item.skillPath ?? ''}/SKILL.md`
  const displayPath = (item: UpdateItem) => (item.skillPath ?? '').replace(/\/SKILL\.md$/i, '')
  const toggleSelected = (key: string) => setSelected((current) => {
    const next = new Set(current)
    if (next.has(key)) next.delete(key)
    else next.add(key)
    return next
  })
  const toggleRepo = (repoItems: UpdateItem[]) => {
    const keys = repoItems.map(keyOf)
    const all = keys.every((key) => selected.has(key))
    setSelected((current) => {
      const next = new Set(current)
      for (const key of keys) { if (all) next.delete(key); else next.add(key) }
      return next
    })
  }
  const installSelected = async () => {
    if (batchProgress || busy || selected.size === 0) return
    const targets = items.filter((item) => selected.has(keyOf(item)))
    const byRepo = new Map<string, Array<{ name: string; skillPath: string }>>()
    for (const item of targets) {
      if (!item.package || !item.skillPath) continue
      if (!byRepo.has(item.package)) byRepo.set(item.package, [])
      byRepo.get(item.package)!.push({ name: item.name, skillPath: sourceSkillPath(item) })
    }
    let done = 0
    let installed = 0
    batchBase.current = 0
    batchTotal.current = targets.length
    setBatchProgress({ done, total: targets.length })
    try {
      for (const [repo, entries] of byRepo) {
        batchBase.current = done
        const operations = await backend.installUpstreamSkills(repo, entries) as Array<unknown> | null
        installed += operations?.length ?? 0
        done += entries.length
        setBatchProgress({ done, total: targets.length })
      }
      setBatchProgress(undefined)
      batchTotal.current = 0
      await onInstalled?.()
      onClose()
      if (installed === 0) toastSuccess(t('skills.alreadyInstalled'))
      else toastSuccess(t('skills.batchInstallSuccess', { n: installed }))
    } catch (error) {
      setBatchProgress(undefined)
      batchTotal.current = 0
      await onInstalled?.()
      onClose()
      toastError(error, t('skills.batchInstallFailed', { done, total: targets.length }))
    }
  }
  const install = async (item: UpdateItem) => {
    if (!item.package || !item.skillPath || busy || batchProgress) return
    setBusy(item.skillPath)
    try {
      const plan = await backend.planInstallSkill(item.package, sourceSkillPath(item), item.name) as Plan
      onPlan(plan)
      onClose()
    } catch (error) {
      const message = backendErrorMessage(error, t('skills.installNewFailed'))
      if (/already exists/i.test(message)) {
        try { await backend.installUpstreamSkills(item.package, [{ name: item.name, skillPath: sourceSkillPath(item) }]) } catch {}
        await onInstalled?.()
        onClose()
        toastSuccess(t('skills.alreadyInstalled'))
      } else {
        toastError(error, t('skills.installNewFailed'))
      }
    } finally {
      setBusy('')
    }
  }
  const toggle = (key: string) => setExpanded((current) => {
    const next = new Set(current)
    if (next.has(key)) next.delete(key)
    else next.add(key)
    return next
  })
  return <Dialog open onOpenChange={(open) => { if (!open && !busy && !batchProgress) onClose() }}>
    <DialogContent className="flex max-h-[70vh] w-[min(520px,calc(100vw-2rem))] flex-col gap-0 overflow-x-hidden p-0">
      <DialogHeader className="border-b px-6 py-4 pr-14"><DialogTitle className="text-base">{t('skills.newUpstreamSkills')}</DialogTitle><DialogDescription className="text-xs leading-5">{t('skills.newUpstreamSkillsDescription')}</DialogDescription></DialogHeader>
      {batchProgress && <div className={styles.installProgress} role="status" aria-live="polite"><div><Loader2 size={13} className="animate-spin" aria-hidden="true" /><span>{t('skills.installingSelected', batchProgress)}</span></div><div className={styles.installProgressBar}><span style={{ width: `${batchProgress.total ? Math.max(4, (batchProgress.done / batchProgress.total) * 100) : 0}%` }} /></div>{batchProgress.total > 0 && <small>{Math.round((batchProgress.done / batchProgress.total) * 100)}%</small>}</div>}
      <div className="min-h-0 flex-1 overflow-x-hidden overflow-y-auto px-6 py-4">{Array.from(items.reduce((map, item) => { const key = item.package || t('common.noSource'); if (!map.has(key)) map.set(key, []); map.get(key)!.push(item); return map }, new Map<string, UpdateItem[]>())).map(([repo, repoItems]) => <section key={repo} className={styles.newSkillGroup}><div className={styles.newSkillGroupHead}><RepoIcon repo={repo} className={styles.newSkillRepoIcon} /><h3>{repo}</h3><Checkbox className={styles.newSkillRepoCheck} checked={repoItems.every((item) => selected.has(keyOf(item))) ? true : repoItems.some((item) => selected.has(keyOf(item))) ? 'indeterminate' : false} onCheckedChange={() => toggleRepo(repoItems)} disabled={Boolean(batchProgress)} aria-label={t('skills.selectAllRepo')} /></div><div className={styles.newSkillList}>{repoItems.map((item) => {
        const key = keyOf(item)
        const isExpanded = expanded.has(key)
        return <div key={key} className={`${styles.newSkillRow} ${isExpanded ? styles.newSkillRowExpanded : ''}`} title={displayPath(item)}>
          <Checkbox checked={selected.has(key)} onCheckedChange={() => toggleSelected(key)} disabled={Boolean(batchProgress)} aria-label={`${t('skills.selectForInstall')} ${item.name}`} />
          <button type="button" className={styles.newSkillToggle} aria-expanded={isExpanded} onClick={() => toggle(key)}>
              <span className={styles.newSkillInfo}><strong>{item.name}</strong>{item.description && <small>{item.description}</small>}{isExpanded && <code>{displayPath(item)}</code>}{isExpanded && (item.copies ?? 1) > 1 && <small>{t('skills.upstreamAgentCopies', { n: item.copies ?? 1 })}</small>}</span>
            <ChevronDown size={12} className={styles.newSkillChevron} aria-hidden="true" />
          </button>
          <Button size="xs" variant="outline" className={styles.newSkillInstall} disabled={Boolean(busy) || Boolean(batchProgress)} onClick={() => void install(item)}><Download size={12} />{busy === item.skillPath ? t('skills.installing') : t('common.installSkill')}</Button>
        </div>
      })}</div></section>)}</div>
      <DialogFooter className="border-t px-6 py-3"><span className={styles.batchInstallHint}>{t('skills.batchInstallHint')}</span><Button size="sm" variant="outline" onClick={onClose} disabled={Boolean(batchProgress)}>{t('common.close')}</Button><Button size="sm" disabled={selected.size === 0 || Boolean(batchProgress)} onClick={() => void installSelected()}>{batchProgress ? t('skills.installingSelected', batchProgress) : t('skills.installSelected', { n: selected.size })}</Button></DialogFooter>
    </DialogContent>
  </Dialog>
}
