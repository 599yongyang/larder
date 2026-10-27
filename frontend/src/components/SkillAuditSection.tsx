import { useEffect, useState } from 'react'
import { ShieldCheck, ShieldAlert, ShieldQuestion, ExternalLink, Loader2 } from 'lucide-react'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { backend } from '../backend'
import { backendErrorMessage } from '../lib/backendError'
import { openExternal } from '../lib/appUpdate'
import { getLanguage, t } from '../i18n'
import type { Skill, SkillAudit } from '../types'
import styles from '../App.module.css'

const verdictTone = (verdict: string) => verdict === 'pass' ? 'success' : verdict === 'warn' ? 'warning' : verdict === 'fail' ? 'error' : 'outline'
const verdictLabel = (verdict: string) => verdict === 'pass' ? t('skills.auditStatusPass') : verdict === 'warn' ? t('skills.auditStatusWarn') : verdict === 'fail' ? t('skills.auditStatusFail') : t('skills.auditStatusUnreviewed')
const riskLabel = (risk?: string) => {
  const key = ({ NONE: 'skills.auditRiskNone', SAFE: 'skills.auditRiskNone', LOW: 'skills.auditRiskLow', MEDIUM: 'skills.auditRiskMedium', HIGH: 'skills.auditRiskHigh', CRITICAL: 'skills.auditRiskCritical' } as Record<string, string>)[risk ?? '']
  return key ? t(key) : ''
}
const formatDate = (value?: string) => value ? new Date(value).toLocaleDateString(getLanguage() === 'en' ? 'en-US' : 'zh-CN') : ''
const verdictIcon = (audit?: SkillAudit) => audit?.verdict === 'pass' ? ShieldCheck : audit?.verdict === 'fail' ? ShieldAlert : ShieldQuestion
const summaryText = (audit: SkillAudit) => !audit.available ? t('skills.auditSimpleUnreviewed') : audit.verdict === 'pass' ? t('skills.auditSimplePass') : audit.verdict === 'warn' ? t('skills.auditSimpleWarn') : t('skills.auditSimpleFail')

export function SkillAuditSection({ skill }: { skill: Skill }) {
  const [audit, setAudit] = useState<SkillAudit>()
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')
  const [reload, setReload] = useState(0)

  useEffect(() => {
    let active = true
    setLoading(true); setError('')
    backend.skillAudit(skill.id).then((next) => { if (active) setAudit(next) }).catch((cause) => { if (active) setError(backendErrorMessage(cause, t('skills.auditUnavailable'))) }).finally(() => { if (active) setLoading(false) })
    return () => { active = false }
  }, [skill.id, reload])

  const Icon = verdictIcon(audit)
  const entries = audit?.entries ?? []
  const latest = entries.reduce((value, entry) => entry.auditedAt && (!value || entry.auditedAt > value) ? entry.auditedAt : value, '')
  const meta = [audit?.available ? t('skills.auditProviderCount', { n: entries.length }) : '', latest ? t('skills.auditLatest', { date: formatDate(latest) }) : '', audit?.available ? (audit.localChanged ? t('skills.auditLocalChangedShort') : t('skills.auditLocalUnchangedShort')) : ''].filter(Boolean).join(' · ')
  return <section className={styles.detailSection}>
    <div className={styles.auditHead}>
      <div className={styles.auditTitle} data-tone={audit?.verdict}><Icon size={15} /><h3>{t('skills.securityAudit')}</h3>{audit && <Badge variant={verdictTone(audit.verdict)}>{verdictLabel(audit.verdict)}</Badge>}{audit?.available && audit.riskLevel && <span className={styles.auditRisk}>{riskLabel(audit.riskLevel)}</span>}</div>
      {audit?.source && <Button size="xs" variant="ghost" className={styles.auditLink} onClick={() => void openExternal(`https://www.skills.sh/${audit.source}/${audit.skill}/security`)}><ExternalLink size={11} />skills.sh</Button>}
    </div>
    {loading && <p className={styles.auditHint}><Loader2 size={13} className="animate-spin" />{t('skills.auditLoading')}</p>}
    {error && <div className={styles.auditError}><span>{error}</span><Button size="xs" variant="outline" onClick={() => setReload((value) => value + 1)}>{t('common.retry')}</Button></div>}
    {!loading && !error && audit && <p className={styles.auditSummaryText}>{summaryText(audit)}{meta && <small>{meta}</small>}</p>}
  </section>
}

// Compact variant for update and install plans.
export function SkillAuditInline({ skillID, source, skill, onAudit }: { skillID?: string; source?: string; skill?: string; onAudit?: (audit: SkillAudit | undefined) => void }) {
  const [audit, setAudit] = useState<SkillAudit>()
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')
  useEffect(() => {
    let active = true
    if (!skillID && (!source || !skill)) { setLoading(false); onAudit?.(undefined); return () => { active = false } }
    (skillID ? backend.skillAudit(skillID) : backend.skillAuditForSource(source ?? '', skill ?? '')).then((next) => { if (active) { setAudit(next); onAudit?.(next) } }).catch(() => { if (active) { setError(t('skills.auditUnavailable')); onAudit?.(undefined) } }).finally(() => { if (active) setLoading(false) })
    return () => { active = false }
  }, [skillID, source, skill, onAudit])
  if (loading) return <div className={styles.auditInline}>{t('skills.auditLoading')}</div>
  if (error || !audit) return <div className={styles.auditInline}>{t('skills.auditUnavailable')}</div>
  const Icon = verdictIcon(audit)
  return <div className={styles.auditInline} data-tone={audit.verdict}><Icon size={14} /><strong>{t('skills.securityAudit')}：{verdictLabel(audit.verdict)}</strong>{audit.available && audit.riskLevel && <small>{riskLabel(audit.riskLevel)}</small>}</div>
}
