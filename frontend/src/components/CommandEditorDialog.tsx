import { useEffect, useMemo, useState } from 'react'
import { Command as CommandIcon, FileWarning, Loader2 } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { Textarea } from '@/components/ui/textarea'
import { Input } from '@/components/ui/input'
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { AgentIcon } from './AgentIcon'
import { backend } from '../backend'
import { t } from '../i18n'
import { backendErrorMessage } from '../lib/backendError'
import { toastSuccess } from '../lib/toast'
import styles from '../App.module.css'
import type { Command, CommandTarget, CommandWritePlan } from '../types'

const targetKey = (target: CommandTarget) => [target.agent, target.scope, target.project ?? '', target.rootPath, target.format].join('|')
const targetForCommand = (command: Command): CommandTarget => ({ agent: command.agent, scope: command.scope, project: command.project, rootPath: command.rootPath, format: command.format ?? 'markdown' })
const targetLabel = (target: CommandTarget) => `${target.agent} · ${target.scope === 'global' ? t('common.global') : target.project || t('common.projects')} · ${target.format === 'toml' ? 'TOML' : 'Markdown'}`
const template = (format: string) => format === 'toml' ? 'description = ""\nprompt = """\n\n"""\n' : '---\ndescription: \n---\n\n'

const unsupported = (warning: string) => {
  const match = warning.match(/^(argument-hint|allowed-tools|model) is not supported by (.+) TOML commands$/)
  if (!match) return warning
  const field = match[1] === 'argument-hint' ? t('commands.argumentHint') : match[1] === 'allowed-tools' ? t('commands.allowedTools') : t('commands.model')
  return t('commands.warningUnsupported', { field, agent: match[2] })
}

const warningText = (warning: string) => {
  if (warning === 'target already exists; applying will overwrite it') return ''
  if (warning === 'command name is empty') return t('commands.nameRequired')
  if (warning.startsWith('conversion between ')) return t('commands.warningConvertUnavailable')
  return unsupported(warning)
}

type Props = { command?: Command; convertFrom?: Command; targets: CommandTarget[]; onClose: () => void; onSaved: () => Promise<void> | void }

export function CommandEditorDialog({ command, convertFrom, targets, onClose, onSaved }: Props) {
  const source = command ?? convertFrom
  const sourceTarget = source ? targetForCommand(source) : undefined
  const initialTarget = sourceTarget ? targets.find((target) => targetKey(target) !== targetKey(sourceTarget)) ?? targets[0] : targets[0]
  const [selectedKey, setSelectedKey] = useState(() => command ? targetKey(targetForCommand(command)) : (initialTarget ? targetKey(initialTarget) : ''))
  const [name, setName] = useState(source?.name ?? '')
  const [content, setContent] = useState(() => source ? '' : template(targets[0]?.format ?? 'markdown'))
  const [loading, setLoading] = useState(Boolean(source))
  const [plan, setPlan] = useState<CommandWritePlan>()
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')

  const allTargets = useMemo(() => {
    if (command) return [targetForCommand(command), ...targets.filter((target) => targetKey(target) !== targetKey(targetForCommand(command)))]
    if (convertFrom) return targets.filter((target) => targetKey(target) !== targetKey(targetForCommand(convertFrom)))
    return targets
  }, [command, convertFrom, targets])
  const selected = allTargets.find((target) => targetKey(target) === selectedKey)

  useEffect(() => {
    if (!source) return
    let active = true
    setLoading(true)
    backend.commandContent(source.path).then((next) => { if (active) setContent(next) }).catch((cause) => { if (active) setError(backendErrorMessage(cause, t('commands.failedToReadContent'))) }).finally(() => { if (active) setLoading(false) })
    return () => { active = false }
  }, [source])

  const planWrite = async () => {
    if (!selected) { setError(t('commands.targetRequired')); return }
    if (!name.trim()) { setError(t('commands.nameRequired')); return }
    setBusy(true); setError('')
    try {
      const request = command
        ? { path: command.path, content }
        : convertFrom
          ? { sourcePath: convertFrom.path, agent: selected.agent, scope: selected.scope, project: selected.project, rootPath: selected.rootPath, name: name.trim() }
          : { agent: selected.agent, scope: selected.scope, project: selected.project, rootPath: selected.rootPath, name: name.trim(), content }
      const next = await backend.planCommandWrite(request) as CommandWritePlan
      setPlan(next)
    } catch (cause) {
      setError(backendErrorMessage(cause, t('commands.planFailed')))
    } finally { setBusy(false) }
  }

  const apply = async () => {
    if (!plan) return
    setBusy(true); setError('')
    try {
      await backend.applyCommandWrite(plan.id)
      await onSaved()
      toastSuccess(t('commands.saved'))
      onClose()
    } catch (cause) {
      setError(backendErrorMessage(cause, t('commands.saveFailed')))
      setBusy(false)
    }
  }

  const close = () => { if (plan && !busy) void backend.cancelCommandWrite(plan.id); onClose() }

  return <Dialog open onOpenChange={(open) => { if (!open && !busy) close() }}>
    <DialogContent className="max-w-3xl gap-0 p-0">
      <DialogHeader className="border-b px-6 py-4 pr-14"><DialogTitle className="flex items-center gap-2 text-base"><CommandIcon size={17} />{command ? t('commands.editCommand') : convertFrom ? t('commands.convertCommand') : t('commands.newCommand')}</DialogTitle><DialogDescription className="text-xs">{command ? t('commands.editHint') : convertFrom ? t('commands.convertHint') : t('commands.createHint')}</DialogDescription></DialogHeader>
      <div className="max-h-[68vh] overflow-y-auto px-6 py-5">
        {!plan ? <>
          {!command && <label className={styles.commandField}><span>{t('commands.target')}</span><Select value={selectedKey} onValueChange={(value) => { setSelectedKey(value); const next = allTargets.find((target) => targetKey(target) === value); if (next && !convertFrom) setContent(template(next.format)) }}><SelectTrigger size="sm" className="w-full"><SelectValue /></SelectTrigger><SelectContent>{allTargets.map((target) => <SelectItem key={targetKey(target)} value={targetKey(target)}><span title={target.rootPath}>{targetLabel(target)}</span></SelectItem>)}</SelectContent></Select>{selected && <small className={styles.commandTargetPath}>{selected.rootPath}</small>}</label>}
          {command && selected && <div className={styles.commandTargetStatic}><AgentIcon name={selected.agent} className="size-4" /><span>{targetLabel(selected)}</span><code>{command.path}</code></div>}
          <div className={styles.commandFieldRow}>
            <label className={styles.commandField}><span>{t('commands.name')}</span><Input className="!h-8" value={name} disabled={Boolean(command)} onChange={(event) => setName(event.target.value)} placeholder="review" /></label>
            {selected && <div className={styles.commandFormatHint}><span>{t('commands.format')}</span><strong>{selected.format === 'toml' ? 'TOML' : 'Markdown'}</strong></div>}
          </div>
          <label className={styles.commandField}><span>{t('commands.content')}</span>{loading ? <div className={styles.commandLoading}><Loader2 size={14} className="animate-spin" />{t('common.loading')}</div> : <Textarea className={styles.commandTextarea} value={content} readOnly={Boolean(convertFrom)} onChange={(event) => setContent(event.target.value)} spellCheck={false} aria-label={t('commands.content')} />}</label>
          {error && <p className={styles.planError} role="alert">{error}</p>}
        </> : <div className={styles.commandPlan}>
          <div className={styles.commandPlanHead}><div><strong>{plan.operation === 'command_create' ? t('commands.createPlan') : plan.operation === 'command_convert' ? t('commands.convertPlan') : t('commands.updatePlan')}</strong><small>{plan.path}</small></div>{plan.exists && <span className={styles.commandPlanWarn}>{t('commands.overwrite')}</span>}</div>
          {plan.warnings && plan.warnings.filter((warning) => warningText(warning)).length > 0 && <div className={styles.commandWarnings}>{plan.warnings.map((warning) => warningText(warning)).filter(Boolean).map((warning, index) => <p key={`${warning}-${index}`}><FileWarning size={12} aria-hidden="true" />{warning}</p>)}</div>}
          {plan.exists && <details className={styles.commandCurrent}><summary>{t('commands.currentContent')}</summary><pre>{plan.currentContent || t('commands.emptyFile')}</pre></details>}
          <pre className={styles.commandPlanPreview}>{plan.content || t('commands.emptyFile')}</pre>
          {error && <p className={styles.planError} role="alert">{error}</p>}
        </div>}
      </div>
      <DialogFooter className="border-t px-6 py-3">
        <Button size="sm" variant="outline" onClick={close} disabled={busy}>{t('common.cancel')}</Button>
        {!plan ? <Button size="sm" disabled={busy || loading} onClick={() => void planWrite()}>{busy && <Loader2 size={13} className="animate-spin" />}{t('commands.previewPlan')}</Button> : <><Button size="sm" variant="outline" onClick={() => setPlan(undefined)} disabled={busy}>{t('commands.backToEdit')}</Button><Button size="sm" onClick={() => void apply()} disabled={busy}>{busy && <Loader2 size={13} className="animate-spin" />}{t('commands.apply')}</Button></>}
      </DialogFooter>
    </DialogContent>
  </Dialog>
}
