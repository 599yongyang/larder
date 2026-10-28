import { Minus, Plus } from 'lucide-react'
import { Button } from './button'
import { cn } from '@/lib/utils'

type NumberFieldProps = {
  value: number
  min?: number
  max?: number
  step?: number
  suffix?: string
  ariaLabel: string
  decreaseLabel?: string
  increaseLabel?: string
  disabled?: boolean
  onChange: (value: number) => void
  className?: string
}

export function NumberField({ value, min = 0, max = Number.MAX_SAFE_INTEGER, step = 1, suffix, ariaLabel, decreaseLabel = 'Decrease', increaseLabel = 'Increase', disabled, onChange, className }: NumberFieldProps) {
  const clamp = (next: number) => Math.min(max, Math.max(min, next))
  return <div className={cn('inline-flex h-8 shrink-0 items-center rounded-md border border-input bg-transparent shadow-xs', className)} role="group" aria-label={ariaLabel}>
    <Button type="button" variant="ghost" size="icon-xs" className="rounded-r-none" disabled={disabled || value <= min} aria-label={decreaseLabel} onClick={() => onChange(clamp(value - step))}><Minus size={12} /></Button>
    <span className="min-w-9 px-1 text-center font-mono text-[10.5px] tabular-nums text-foreground">{value}{suffix ? <span className="ml-0.5 text-muted-foreground">{suffix}</span> : null}</span>
    <Button type="button" variant="ghost" size="icon-xs" className="rounded-l-none" disabled={disabled || value >= max} aria-label={increaseLabel} onClick={() => onChange(clamp(value + step))}><Plus size={12} /></Button>
  </div>
}
