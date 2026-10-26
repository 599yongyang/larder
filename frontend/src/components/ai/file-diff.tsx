import { Check, ChevronRight, Copy, FileCode2, UnfoldVertical } from 'lucide-react'
import { AnimatePresence, MotionConfig, motion } from 'framer-motion'
import * as React from 'react'
import { cn } from '@/lib/utils'

export interface DiffLine {
  content: string
  newNumber?: number
  oldNumber?: number
  type: 'addition' | 'context' | 'deletion'
}

export interface DiffHunk {
  header?: string
  hidden?: DiffLine[]
  id: string
  lines: DiffLine[]
}

interface FileDiffProps extends Omit<React.ComponentProps<'figure'>, 'onCopy'> {
  additions?: number
  defaultOpen?: boolean
  deletions?: number
  filename: string
  hunks?: DiffHunk[]
  lines?: DiffLine[]
  onCopy?: (patch: string) => void
  onOpenChange?: (open: boolean) => void
  loading?: boolean
  loadingLabel?: string
}

const ease = [0.23, 1, 0.32, 1] as const
const sign = { addition: '+', context: ' ', deletion: '−' }

function Row({ line }: { line: DiffLine }) {
  return (
    <span
      data-diff-line={line.type}
      className={cn(
        'grid min-w-max grid-cols-[2rem_2rem_1rem_1fr] px-2 transition-colors duration-100 hover:brightness-95 dark:hover:brightness-125',
        line.type === 'addition' && 'bg-emerald-500/10',
        line.type === 'deletion' && 'bg-red-500/10',
      )}
    >
      <span className="text-right text-muted-foreground/60 select-none">{line.oldNumber ?? ''}</span>
      <span className="text-right text-muted-foreground/60 select-none">{line.newNumber ?? ''}</span>
      <span className="text-center text-muted-foreground select-none">{sign[line.type]}</span>
      <span className="pr-4 text-foreground">{line.content || ' '}</span>
    </span>
  )
}

export function FileDiff({
  additions,
  className,
  defaultOpen = true,
  deletions,
  filename,
  hunks,
  lines = [],
  onCopy,
  onOpenChange,
  loading = false,
  loadingLabel = 'Loading…',
  ...props
}: FileDiffProps) {
  const bodyId = React.useId()
  const groups = hunks ?? [{ id: 'all', lines }]
  const all = groups.flatMap((hunk) => hunk.lines)
  const added = additions ?? all.filter((line) => line.type === 'addition').length
  const removed = deletions ?? all.filter((line) => line.type === 'deletion').length
  const [open, setOpen] = React.useState(defaultOpen)
  const [expanded, setExpanded] = React.useState<Set<string>>(() => new Set())
  const [copied, setCopied] = React.useState(false)
  const preRef = React.useRef<HTMLPreElement>(null)
  const diffKey = groups.map((hunk) => `${hunk.id}:${hunk.lines.length}`).join('|')

  React.useEffect(() => {
    if (!open || loading) return
    const pre = preRef.current
    if (!pre) return
    const timer = window.setTimeout(() => {
      const target = pre.querySelector<HTMLElement>('[data-diff-line="addition"], [data-diff-line="deletion"]')
      if (!target) return
      const delta = target.getBoundingClientRect().top - pre.getBoundingClientRect().top
      // Keep roughly four context lines above the first change.
      const lead = Math.min(80, pre.clientHeight * 0.3)
      pre.scrollTop = Math.max(0, pre.scrollTop + delta - lead)
    }, 180)
    return () => window.clearTimeout(timer)
  }, [open, loading, diffKey])

  React.useEffect(() => {
    if (!copied) return
    const timer = setTimeout(() => setCopied(false), 1400)
    return () => clearTimeout(timer)
  }, [copied])

  function copy() {
    const patch = all.map((line) => `${sign[line.type] === '−' ? '-' : sign[line.type]}${line.content}`).join('\n')
    void navigator.clipboard?.writeText(patch).catch(() => undefined)
    setCopied(true)
    onCopy?.(patch)
  }

  const iconButton = 'flex size-6 items-center justify-center rounded-md text-muted-foreground outline-none transition-colors hover:bg-muted focus-visible:ring-2 focus-visible:ring-ring/40'

  return (
    <MotionConfig reducedMotion="user">
      <figure
        data-slot="file-diff"
        className={cn('overflow-hidden rounded-[12px] border border-[var(--line)] bg-[var(--panel)] font-sans text-sm font-[450] tracking-[-0.05px] text-muted-foreground', className)}
        {...props}
      >
        <figcaption className="flex h-10 items-center gap-2 border-b border-[var(--line)] bg-[var(--paper)] pr-2 pl-1.5 text-[10px] uppercase tracking-[.06em]">
          <button aria-controls={bodyId} aria-expanded={open} aria-label={open ? `Collapse ${filename}` : `Expand ${filename}`} className={iconButton} onClick={() => setOpen((value) => { const next = !value; onOpenChange?.(next); return next })} type="button">
            <ChevronRight className={cn('size-3.5 transition-[rotate] duration-200 ease-[cubic-bezier(0.23,1,0.32,1)] motion-reduce:transition-none', open && 'rotate-90')} />
          </button>
          <FileCode2 aria-hidden="true" className="size-4 shrink-0" />
          <span className="min-w-0 truncate normal-case tracking-normal text-foreground/80">{filename}</span>
          {all.length > 0 || additions !== undefined || deletions !== undefined ? <><span className="ml-auto text-emerald-600 tabular-nums">+{added}</span><span className="text-red-500 tabular-nums">−{removed}</span></> : <span className="ml-auto" />}
          <button aria-label={copied ? 'Copied' : 'Copy patch'} className={iconButton} onClick={copy} type="button">
            <span className="grid *:col-start-1 *:row-start-1 *:transition-[opacity,scale] *:duration-150 motion-reduce:*:transition-opacity">
              <Copy className={cn('size-3.5', copied && 'scale-75 opacity-0')} />
              <Check className={cn('size-3.5', !copied && 'scale-75 opacity-0')} />
            </span>
          </button>
        </figcaption>
        <AnimatePresence initial={false}>
          {open && (
            <motion.div layout animate={{ height: 'auto', opacity: 1 }} className="overflow-hidden" exit={{ height: 0, opacity: 0 }} id={bodyId} initial={{ height: 0, opacity: 0 }} transition={{ layout: { duration: 0.2, ease }, duration: 0.22, ease }}>
              {loading ? <div className="flex items-center justify-center gap-2 border-t border-[var(--line)] px-3 py-5 text-[11px] text-muted-foreground"><span className="size-1.5 animate-pulse rounded-full bg-current" />{loadingLabel}</div> : <pre ref={preRef} className="max-h-[320px] overflow-auto border-t border-[var(--line)] py-2 text-[12px] leading-5">
                <code className="block">
                  {groups.map((hunk) => {
                    const showHidden = expanded.has(hunk.id)
                    return (
                      <React.Fragment key={hunk.id}>
                        {hunk.header && (
                          <span className="flex min-w-max items-center gap-2 px-2 text-muted-foreground/60">
                            {hunk.hidden?.length ? (
                              <button aria-expanded={showHidden} className="-ml-0.5 flex items-center gap-1.5 rounded-md px-1 text-muted-foreground outline-none transition-colors hover:bg-muted focus-visible:ring-2 focus-visible:ring-ring/40" onClick={() => setExpanded((current) => { const next = new Set(current); if (next.has(hunk.id)) next.delete(hunk.id); else next.add(hunk.id); return next })} type="button">
                                <UnfoldVertical aria-hidden="true" className="size-3.5" />
                                {showHidden ? 'Hide' : 'Show'} {hunk.hidden.length} unchanged lines
                              </button>
                            ) : null}
                            <span className="select-none">{hunk.header}</span>
                          </span>
                        )}
                        <AnimatePresence initial={false}>
                          {showHidden && hunk.hidden && (
                            <motion.span animate={{ height: 'auto', opacity: 1 }} className="block overflow-hidden" exit={{ height: 0, opacity: 0 }} initial={{ height: 0, opacity: 0 }} transition={{ duration: 0.2, ease }}>
                              {hunk.hidden.map((line, index) => <Row key={`hidden-${index}-${line.content}`} line={line} />)}
                            </motion.span>
                          )}
                        </AnimatePresence>
                        {hunk.lines.map((line, index) => <Row key={`${line.type}-${index}-${line.content}`} line={line} />)}
                      </React.Fragment>
                    )
                  })}
                </code>
              </pre>}
            </motion.div>
          )}
        </AnimatePresence>
      </figure>
    </MotionConfig>
  )
}
