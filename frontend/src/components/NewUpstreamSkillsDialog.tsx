import { useState } from 'react'
import { ChevronDown, Download, Plus } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { backend } from '../backend'
import styles from '../App.module.css'
import { t } from '../i18n'
import { toastError } from '../lib/toast'
import type { Plan, UpdateItem } from '../types'

export function NewUpstreamSkillsDialog({ items, onClose, onPlan }: { items: UpdateItem[]; onClose: () => void; onPlan: (plan: Plan) => void }) {
  const [busy, setBusy] = useState('')
  const [expanded, setExpanded] = useState<Set<string>>(() => new Set())
  const keyOf = (item: UpdateItem) => `${item.package}:${item.skillPath}`
  const install = async (item: UpdateItem) => {
    if (!item.package || !item.skillPath || busy) return
    setBusy(item.skillPath)
    try {
      const plan = await backend.planInstallSkill(item.package, item.skillPath, item.name) as Plan
      onPlan(plan)
      onClose()
    } catch (error) {
      toastError(error, t('skills.installNewFailed'))
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
  return <Dialog open onOpenChange={(open) => { if (!open && !busy) onClose() }}>
    <DialogContent className="flex max-h-[70vh] w-[min(520px,calc(100vw-2rem))] flex-col gap-0 overflow-x-hidden p-0">
      <DialogHeader className="border-b px-6 py-4 pr-14"><DialogTitle className="text-base">{t('skills.newUpstreamSkills')}</DialogTitle><DialogDescription className="text-xs leading-5">{t('skills.newUpstreamSkillsDescription')}</DialogDescription></DialogHeader>
      <div className="min-h-0 flex-1 overflow-x-hidden overflow-y-auto px-6 py-4">{Array.from(items.reduce((map, item) => { const key = item.package || t('common.noSource'); if (!map.has(key)) map.set(key, []); map.get(key)!.push(item); return map }, new Map<string, UpdateItem[]>())).map(([repo, repoItems]) => <section key={repo} className={styles.newSkillGroup}><h3>{repo}</h3><div className={styles.newSkillList}>{repoItems.map((item) => {
        const key = keyOf(item)
        const isExpanded = expanded.has(key)
        return <div key={key} className={`${styles.newSkillRow} ${isExpanded ? styles.newSkillRowExpanded : ''}`} title={item.skillPath}>
          <button type="button" className={styles.newSkillToggle} aria-expanded={isExpanded} onClick={() => toggle(key)}>
            <span className={styles.newSkillIcon}><Plus size={13} aria-hidden="true" /></span>
            <span className={styles.newSkillInfo}><strong>{item.name}</strong>{item.description && <small>{item.description}</small>}{isExpanded && <code>{item.skillPath}</code>}</span>
            <ChevronDown size={12} className={styles.newSkillChevron} aria-hidden="true" />
          </button>
          <Button size="xs" variant="outline" className={styles.newSkillInstall} disabled={Boolean(busy)} onClick={() => void install(item)}><Download size={12} />{busy === item.skillPath ? t('skills.installing') : t('common.installSkill')}</Button>
        </div>
      })}</div></section>)}</div>
      <DialogFooter className="border-t px-6 py-3"><Button size="sm" variant="outline" onClick={onClose} disabled={Boolean(busy)}>{t('common.close')}</Button></DialogFooter>
    </DialogContent>
  </Dialog>
}
