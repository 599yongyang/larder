import type { ReactNode } from 'react'
import { cn } from '@/lib/utils'

export type SegmentedOption<T extends string> = { value: T; label: ReactNode; icon?: ReactNode; disabled?: boolean }

type Props<T extends string> = {
  value: T
  options: Array<SegmentedOption<T>>
  onChange: (value: T) => void
  ariaLabel: string
  columns?: number
  className?: string
}

export function SegmentedControl<T extends string>({ value, options, onChange, ariaLabel, columns, className }: Props<T>) {
  return <div role="group" aria-label={ariaLabel} className={cn(columns ? 'grid gap-1' : 'inline-flex gap-1', className)} style={columns ? { gridTemplateColumns: `repeat(${columns}, minmax(0, 1fr))` } : undefined}>
    {options.map((option) => <button key={option.value} type="button" disabled={option.disabled} aria-pressed={value === option.value} onClick={() => onChange(option.value)} className={cn('flex h-8 items-center justify-center gap-1.5 rounded-[7px] border px-2.5 text-[10.5px] transition-colors', value === option.value ? 'border-[var(--brand)] bg-[var(--brand-soft)] text-[var(--ink)]' : 'border-[var(--line)] bg-[var(--paper)] text-[var(--ink-soft)] hover:border-[var(--line-strong)]')}>
      {option.icon}<span>{option.label}</span>
    </button>)}
  </div>
}
