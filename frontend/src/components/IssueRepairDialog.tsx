import { useState } from 'react'
import { CheckCircle2, Wrench } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { Badge } from '@/components/ui/badge'
import { SkillIcon } from './SkillIcon'
import { BatchPlanDialog } from './BatchPlanDialog'
import { backend } from '../backend'
import styles from '../App.module.css'
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
      else setError('当前没有可以自动处理的项目')
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : '预览失败')
    } finally {
      setPlanning(false)
    }
  }

  if (plans.length > 0) return <BatchPlanDialog plans={plans} onClose={() => setPlans([])} onApplied={onApplied} />

  return <Dialog open onOpenChange={(open) => { if (!open) onClose() }}>
    <DialogContent className="max-w-3xl gap-0 p-0">
      <DialogHeader className="border-b px-6 py-5 pr-14">
        <DialogTitle className="text-lg">需要处理</DialogTitle>
        <DialogDescription className="max-w-xl leading-relaxed">按问题类型集中处理。操作会先预览，确认后才会修改；每个 Skill 执行前都会写快照。</DialogDescription>
      </DialogHeader>
      <div className="max-h-[60vh] overflow-y-auto px-6 py-5">
        <div className={styles.issueStats}>
          <div className={styles.issueStat}><strong>{issues.length}</strong><span>需要处理</span></div>
          <div className={styles.issueStat}><strong>{copyIssues.length}</strong><span>内容不一致</span></div>
          <div className={styles.issueStat}><strong>{brokenIssues.length}</strong><span>链接失效</span></div>
        </div>

        {copyIssues.length > 0 && <section className={styles.issueSection}>
          <div className={styles.issueSectionHead}>
            <div><h3>内容不一致</h3><p>Agent 目录里的内容和共享版本不同。修复时会用共享版本覆盖这些副本。</p></div>
          </div>
          <div className={styles.issueList}>{copyIssues.map((issue) => <button type="button" key={issue.skill.id} className={styles.issueRow} onClick={() => onLocate(issue.skill.id)}><SkillIcon skill={issue.skill} className="size-6" /><div><strong>{issue.skill.name}</strong><small>{issue.copyLabels.join('、')}</small></div><Badge variant="warning">内容不一致</Badge></button>)}</div>
        </section>}

        {brokenIssues.length > 0 && <section className={styles.issueSection}>
          <div className={styles.issueSectionHead}>
            <div><h3>链接失效</h3><p>Agent 目录里的链接已经打不开。修复会重新创建指向共享版本的链接。</p></div>
          </div>
          <div className={styles.issueList}>{brokenIssues.map((issue) => <button type="button" key={issue.skill.id} className={styles.issueRow} onClick={() => onLocate(issue.skill.id)}><SkillIcon skill={issue.skill} className="size-6" /><div><strong>{issue.skill.name}</strong><small>{issue.brokenLabels.join('、')}</small></div><Badge variant="warning">链接失效</Badge></button>)}</div>
        </section>}

        {manualIssues.length > 0 && <section className={styles.issueSection}>
          <div className={styles.issueSectionHead}>
            <div><h3>SKILL.md 需要处理</h3><p>文件缺失或 frontmatter 格式无效，需要修复文件或从来源重新安装/更新。</p></div>
            <Wrench size={16} className="text-muted-foreground" />
          </div>
          <div className={styles.issueList}>{manualIssues.map((issue) => <button type="button" key={issue.skill.id} className={styles.issueRow} onClick={() => onLocate(issue.skill.id)}><SkillIcon skill={issue.skill} className="size-6" /><div><strong>{issue.skill.name}</strong><small>{issue.reasons.filter((reason) => reason.includes('SKILL.md')).join('、')}</small></div><Badge variant="outline">手动处理</Badge></button>)}</div>
        </section>}

        {issues.length === 0 && <div className={styles.emptyState}><CheckCircle2 size={24} /><strong>当前没有需要处理的问题</strong></div>}
        {error && <p className={styles.planError} role="alert">{error}</p>}
      </div>
      <DialogFooter className="items-center border-t px-6 py-4"><span className={styles.footerHint}>{autoCount > 0 ? `${autoCount} 项可以自动修复` : "没有可自动修复的项目"}</span><Button size="sm" variant="outline" onClick={onClose}>关闭</Button><Button size="sm" disabled={autoCount === 0 || planning} onClick={() => void createPlans()}>{planning ? "准备中…" : "修复全部"}</Button></DialogFooter>
    </DialogContent>
  </Dialog>
}
