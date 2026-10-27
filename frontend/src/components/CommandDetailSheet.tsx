import { copyText as writeClipboard } from '../lib/clipboard'
import { useEffect, useState } from 'react'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Sheet, SheetContent, SheetDescription, SheetHeader, SheetTitle } from '@/components/ui/sheet'
import { Copy, Terminal, Trash2 } from 'lucide-react'
import { AgentIcon } from './AgentIcon'
import { MarkdownPreview, splitFrontmatter } from './MarkdownPreview'
import { backend } from '../backend'
import { backendErrorMessage } from '../lib/backendError'
import { t, tValue } from '../i18n'
import { toastSuccess } from '../lib/toast'
import styles from '../App.module.css'
import instructionStyles from './InstructionDetailSheet.module.css'
import type { Command } from '../types'

export function commandScopeLabel(scope: string) {
  return scope === 'global' ? t('common.global') : t('common.projects')
}

export function CommandDetailSheet({ command, onClose, onEdit, onConvert, onDelete }: { command: Command; onClose: () => void; onEdit?: () => void; onConvert?: () => void; onDelete?: () => void }) {
  const [tab, setTab] = useState<'preview' | 'raw'>(command.format === 'toml' ? 'raw' : 'preview')
  const [content, setContent] = useState('')
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')

  useEffect(() => {
    let active = true
    setLoading(true)
    setError('')
    backend.commandContent(command.path).then((next) => {
      if (active) setContent(next)
    }).catch((cause) => {
      if (active) setError(backendErrorMessage(cause, t('commands.failedToReadContent')))
    }).finally(() => {
      if (active) setLoading(false)
    })
    return () => { active = false }
  }, [command.path])

  const { meta, body } = splitFrontmatter(content)
  const copyPath = async () => {
    await writeClipboard(command.path)
    toastSuccess(t('commands.pathCopied'))
  }
  const copyName = async () => {
    await writeClipboard(`/${command.name}`)
    toastSuccess(t('commands.nameCopied'))
  }

  return <Sheet open onOpenChange={(open) => { if (!open) onClose() }}>
    <SheetContent side="right" className="w-[560px] sm:max-w-[560px]">
      <SheetHeader className="gap-0 border-b px-5 py-4">
        <div className="flex items-start gap-3 pr-9">
          <div className={styles.mcpHero}><Terminal size={17} /></div>
          <div className="min-w-0 flex-1">
            <SheetTitle className="truncate text-[15px]">/{command.name}</SheetTitle>
            <SheetDescription className="mt-1 flex flex-wrap items-center gap-x-1.5 text-[11px]">
              <AgentIcon name={command.agent} className="size-4" />
              <span>{tValue(command.agent)}</span>
              <span>·</span>
              <span>{commandScopeLabel(command.scope)}</span>
              {command.project && <><span>·</span><span>{command.project}</span></>}
            </SheetDescription>
          </div>
          <Badge variant={command.scope === 'global' ? 'outline' : 'info'}>{commandScopeLabel(command.scope)}</Badge>
        </div>
      </SheetHeader>
      <div className="min-h-0 flex-1 overflow-y-auto p-0">
        {command.description && <section className={styles.detailSection}><p className={styles.commandDetailDescription}>{command.description}</p></section>}
        <section className={styles.detailSection}>
          <h3>{t('common.file')}</h3>
          <dl className={styles.definition}>
            <dt>{t('common.path')}</dt><dd><code className={styles.detailPath}>{command.path}</code></dd>
            <dt>{t('common.agent')}</dt><dd>{tValue(command.agent)}</dd>
            <dt>{t('common.scope')}</dt><dd>{commandScopeLabel(command.scope)}{command.project ? ` · ${command.project}` : ''}</dd>
            <dt>{t('common.type')}</dt><dd>{command.format === 'toml' ? t('commands.formatToml') : t('commands.formatMarkdown')}</dd>
            {command.argumentHint && <><dt>{t('commands.argumentHint')}</dt><dd><code>{command.argumentHint}</code></dd></>}
            {command.model && <><dt>{t('commands.model')}</dt><dd><code>{command.model}</code></dd></>}
            {command.allowedTools && command.allowedTools.length > 0 && <><dt>{t('commands.allowedTools')}</dt><dd><code>{command.allowedTools.join(', ')}</code></dd></>}
          </dl>
          <div className={styles.detailPathActions}><Button size="xs" variant="outline" onClick={() => void copyName()}><Terminal size={12} />{t('commands.copyName')}</Button><Button size="xs" variant="outline" onClick={() => void copyPath()}><Copy size={12} />{t('common.copyPath')}</Button>{onEdit && <Button size="xs" variant="ghost" onClick={onEdit}>{t('commands.edit')}</Button>}{onConvert && <Button size="xs" variant="ghost" onClick={onConvert}>{t('commands.convert')}</Button>}{onDelete && <Button size="xs" variant="ghost" className="text-destructive" onClick={onDelete}><Trash2 size={12} />{t('common.delete')}</Button>}</div>
        </section>
        <section className={styles.detailSection}>
          <div className={instructionStyles.contentHead}>
            <h3>{t('commands.content')}</h3>
            <div className={styles.segmentedControl} role="group" aria-label={t('commands.contentDisplay')}>
              <Button size="xs" variant={tab === 'preview' ? 'secondary' : 'ghost'} onClick={() => setTab('preview')}>{t('commands.preview')}</Button>
              <Button size="xs" variant={tab === 'raw' ? 'secondary' : 'ghost'} onClick={() => setTab('raw')}>{t('commands.raw')}</Button>
            </div>
          </div>
          {loading && <p className={instructionStyles.contentHint}>{t('common.loading')}</p>}
          {error && <p className={styles.planError} role="alert">{error}</p>}
          {!loading && !error && tab === 'preview' && <>
            {meta.length > 0 && <dl className={instructionStyles.frontmatter}>{meta.map(([key, value]) => <div key={key}><dt>{key}</dt><dd>{value}</dd></div>)}</dl>}
            <MarkdownPreview source={body} />
          </>}
          {!loading && !error && tab === 'raw' && <pre className={instructionStyles.instructionPreview}>{content || t('commands.emptyFile')}</pre>}
        </section>
      </div>
    </SheetContent>
  </Sheet>
}
