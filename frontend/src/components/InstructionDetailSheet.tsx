import { useEffect, useState } from 'react'
import { Copy, FileText } from 'lucide-react'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Sheet, SheetContent, SheetDescription, SheetHeader, SheetTitle } from '@/components/ui/sheet'
import { backend } from '../backend'
import { MarkdownPreview, splitFrontmatter } from './MarkdownPreview'
import styles from '../App.module.css'
import instructionStyles from './InstructionDetailSheet.module.css'
import { toastSuccess } from '../lib/toast'
import type { InstructionFile } from '../types'

export const instructionScopeLabel = (scope: string) => scope === 'global' ? '全局' : '项目'
export const formatInstructionSize = (size: number) => size < 1024 ? `${size} B` : `${(size / 1024).toFixed(1)} KB`
export const formatInstructionDate = (value: string) => { const date = new Date(value); return Number.isNaN(date.getTime()) ? '—' : date.toLocaleDateString('zh-CN') }

export function InstructionDetailSheet({ file, onClose, onEdit }: { file: InstructionFile; onClose: () => void; onEdit?: () => void }) {
  const [tab, setTab] = useState<'preview' | 'raw'>('preview')
  const [content, setContent] = useState(file.excerpt ?? '')
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')
  useEffect(() => {
    let active = true
    setLoading(true); setError('')
    backend.instructionContent(file.path).then((next) => { if (active) setContent(next) }).catch((cause) => { if (active) setError(cause instanceof Error ? cause.message : '无法读取完整内容') }).finally(() => { if (active) setLoading(false) })
    return () => { active = false }
  }, [file.path])
  const { meta, body } = splitFrontmatter(content)
  const copyPath = async () => { await navigator.clipboard.writeText(file.path); toastSuccess('路径已复制') }
  return <Sheet open onOpenChange={(open) => { if (!open) onClose() }}>
    <SheetContent side="right" className="w-[540px] sm:max-w-[540px]">
      <SheetHeader className="gap-0 border-b px-5 py-4"><div className="flex items-start gap-3"><div className={styles.mcpHero}><FileText size={17} /></div><div className="min-w-0 flex-1"><SheetTitle className="truncate text-[15px]">{file.name}</SheetTitle><SheetDescription className="mt-1 flex flex-wrap items-center gap-x-1.5 text-[11px]"><span>{file.agent}</span><span>·</span><span>{instructionScopeLabel(file.scope)}</span><span>·</span><span>{file.project || '用户目录'}</span></SheetDescription></div><Badge variant={file.scope === 'global' ? 'outline' : 'info'}>{instructionScopeLabel(file.scope)}</Badge></div></SheetHeader>
      <div className="min-h-0 flex-1 overflow-y-auto p-0">
        <section className={styles.detailSection}><h3>文件</h3><dl className={styles.definition}><dt>路径</dt><dd><code className={styles.detailPath}>{file.path}</code></dd><dt>范围</dt><dd>{instructionScopeLabel(file.scope)}{file.project ? ` · ${file.project}` : ''}</dd><dt>大小</dt><dd>{formatInstructionSize(file.size)}</dd><dt>修改时间</dt><dd>{formatInstructionDate(file.modifiedAt)}</dd><dt>内容哈希</dt><dd><code>{file.hash.slice(0, 16)}</code></dd></dl><div className={styles.detailPathActions}><Button size="xs" variant="outline" onClick={() => void copyPath()}><Copy size={12} />复制路径</Button>{onEdit && <Button size="xs" variant="ghost" onClick={onEdit}>编辑</Button>}</div></section>
        <section className={styles.detailSection}>
          <div className={instructionStyles.contentHead}><h3>内容</h3><div className={styles.segmentedControl} role="group" aria-label="内容展示方式"><Button size="xs" variant={tab === 'preview' ? 'secondary' : 'ghost'} onClick={() => setTab('preview')}>预览</Button><Button size="xs" variant={tab === 'raw' ? 'secondary' : 'ghost'} onClick={() => setTab('raw')}>原文</Button></div></div>
          {loading && <p className={instructionStyles.contentHint}>读取中…</p>}
          {error && <p className={styles.planError} role="alert">{error}</p>}
          {!loading && tab === 'preview' && <>
            {meta.length > 0 && <dl className={instructionStyles.frontmatter}>{meta.map(([key, value]) => <div key={key}><dt>{key}</dt><dd>{value}</dd></div>)}</dl>}
            <MarkdownPreview source={body} />
          </>}
          {!loading && tab === 'raw' && <pre className={instructionStyles.instructionPreview}>{content || '（空文件）'}</pre>}
        </section>
      </div>
    </SheetContent>
  </Sheet>
}
