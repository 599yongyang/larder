import { useState } from 'react'
import { CheckCircle2, Wrench } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { Badge } from '@/components/ui/badge'
import { SkillIcon } from './SkillIcon'
import { BatchPlanDialog } from './BatchPlanDialog'
import { backend } from '../backend'
import styles from '../App.module.css'
import { t, tValue } from '../i18n'
import { backendErrorMessage } from '../lib/backendError'
import type { Plan, Skill } from '../types'

export type IssueItem = { skill: Skill; reasons: string[]; copyRoots: string[]; copyLabels: string[]; brokenRoots: string[]; brokenLabels: string[] }

export function IssueRepairDialog({ issues, onClose, onLocate, onApplied }: { issues: IssueItem[]; onClose: () => void; onLocate: (id: string) => void; onApplied: () => void }) {
  const [plans, setPlans] = useState<Plan[]>([])
  const [planning, setPlanning] = useState(false)
  const [error, setError] = useState('')
  const copyIssues = issues.filter((issue) => issue.copyRoots.length > 0)
  const brokenIssues = issues.filter((issue) => issue.brokenRoots.length > 0)
  const manualIssues = issues.filter((issue) => (issue.reasons.includes('缺少 SKILL.md') || issue.reasons.includes('SKILL.md 格式无效')) && issue.copyRoots.length === 0 && issue.brokenRoots.length === 0)
  const autoCount = copyIssues.length + brokenIssues.length

  const createPlans = async () => {
    setPlanning(true)
    setError('')
    try {
      const created: Plan[] = []
      for (const issue of copyIssues) created.push(await backend.planSyncCopy(issue.skill.id, issue.copyRoots))
      for (const issue of brokenIssues) created.push(await backend.planLink(issue.skill.id, issue.brokenRoots))
      if (created.length > 0) setPlans(created)
      else setError(t('common.nothingCanBeFixedAutomaticallyRightNow'))
    } catch (cause) {
      setError(backendErrorMessage(cause, t('common.previewFailed')))
    } finally {
      setPlanning(false)
    }
  }

  if (plans.length > 0) return <BatchPlanDialog plans={plans} onClose={() => setPlans([])} onApplied={onApplied} />

  return <Dialog open onOpenChange={(open) => { if (!open) onClose() }}>
    <DialogContent className="max-w-3xl gap-0 p-0">
      <DialogHeader className="border-b px-6 py-5 pr-14">
        <DialogTitle className="text-lg">{t('common.needsAttention')}</DialogTitle>
        <DialogDescription className="max-w-xl leading-relaxed">{t('common.fixIssuesByTypeOperationsArePreviewedFirstEachSkillIsSnapshottedBeforeChanges')}</DialogDescription>
      </DialogHeader>
      <div className="max-h-[60vh] overflow-y-auto px-6 py-5">
        <div className={styles.issueStats}>
          <div className={styles.issueStat}><strong>{issues.length}</strong><span>{t('common.needsAttention')}</span></div>
          <div className={styles.issueStat}><strong>{copyIssues.length}</strong><span>{t('common.contentDrift')}</span></div>
          <div className={styles.issueStat}><strong>{brokenIssues.length}</strong><span>{t('common.brokenLink')}</span></div>
        </div>

        {copyIssues.length > 0 && <section className={styles.issueSection}>
          <div className={styles.issueSectionHead}>
            <div><h3>{t('common.contentDrift')}</h3><p>{t('common.contentInAgentDirectoriesDiffersFromTheSharedVersionRepairReplacesThoseCopiesWithTheSharedVersion')}</p></div>
          </div>
          <div className={styles.issueList}>{copyIssues.map((issue) => <button type="button" key={issue.skill.id} className={styles.issueRow} onClick={() => onLocate(issue.skill.id)}><SkillIcon skill={issue.skill} className="size-6" /><div><strong>{issue.skill.name}</strong><small>{issue.copyLabels.join('、')}</small></div><Badge variant="warning">{t('common.contentDrift')}</Badge></button>)}</div>
        </section>}

        {brokenIssues.length > 0 && <section className={styles.issueSection}>
          <div className={styles.issueSectionHead}>
            <div><h3>{t('common.brokenLink')}</h3><p>{t('common.linksInAgentDirectoriesAreBrokenRepairRecreatesLinksToTheSharedVersion')}</p></div>
          </div>
          <div className={styles.issueList}>{brokenIssues.map((issue) => <button type="button" key={issue.skill.id} className={styles.issueRow} onClick={() => onLocate(issue.skill.id)}><SkillIcon skill={issue.skill} className="size-6" /><div><strong>{issue.skill.name}</strong><small>{issue.brokenLabels.join('、')}</small></div><Badge variant="warning">{t('common.brokenLink')}</Badge></button>)}</div>
        </section>}

        {manualIssues.length > 0 && <section className={styles.issueSection}>
          <div className={styles.issueSectionHead}>
            <div><h3>{t('common.skillMdNeedsAttention')}</h3><p>{t('common.theFileIsMissingOrItsFrontmatterIsInvalidRepairItOrReinstallUpdateFromTheSource')}</p></div>
            <Wrench size={16} className="text-muted-foreground" />
          </div>
          <div className={styles.issueList}>{manualIssues.map((issue) => <button type="button" key={issue.skill.id} className={styles.issueRow} onClick={() => onLocate(issue.skill.id)}><SkillIcon skill={issue.skill} className="size-6" /><div><strong>{issue.skill.name}</strong><small>{issue.reasons.filter((reason) => reason.includes('SKILL.md')).map((reason) => tValue(reason)).join('、')}</small></div><Badge variant="outline">{t('common.manual')}</Badge></button>)}</div>
        </section>}

        {issues.length === 0 && <div className={styles.emptyState}><CheckCircle2 size={24} /><strong>{t('common.noIssuesNeedAttention')}</strong></div>}
        {error && <p className={styles.planError} role="alert">{error}</p>}
      </div>
      <DialogFooter className="items-center border-t px-6 py-4"><span className={styles.footerHint}>{autoCount > 0 ? t('common.canBeFixedAutomatically', { n: autoCount }) : t('common.nothingCanBeFixedAutomatically')}</span><Button size="sm" variant="outline" onClick={onClose}>{t('common.close')}</Button><Button size="sm" disabled={autoCount === 0 || planning} onClick={() => void createPlans()}>{planning ? t('common.preparing') : t('common.fixAll')}</Button></DialogFooter>
    </DialogContent>
  </Dialog>
}
