import { useState } from 'react'
import { RotateCcw } from 'lucide-react'
import { Button } from './button'
import { Kbd, KbdGroup } from './kbd'
import { isMacPlatform } from '../../lib/platform'
import { cn } from '@/lib/utils'

type Props = {
  value: string
  disabled?: boolean
  recordingLabel: string
  resetLabel: string
  ariaLabel: string
  onChange: (accelerator: string) => void
  onReset: () => void
  className?: string
}

function splitAccelerator(value: string) {
  return value.split('+').map((part) => part.trim()).filter(Boolean).map((part) => {
    const mac = isMacPlatform()
    if (part === 'CmdOrCtrl') return mac ? '⌘' : 'Ctrl'
    if (part === 'Cmd' || part === 'Command') return '⌘'
    if (part === 'Ctrl' || part === 'Control') return mac ? '⌃' : 'Ctrl'
    if (part === 'Shift') return '⇧'
    if (part === 'Alt' || part === 'Option') return mac ? '⌥' : 'Alt'
    return part.length === 1 ? part.toUpperCase() : part
  })
}

function acceleratorFromEvent(event: React.KeyboardEvent) {
  if (['Meta', 'Control', 'Alt', 'Shift'].includes(event.key)) return ''
  if (!event.metaKey && !event.ctrlKey && !event.altKey) return ''
  const parts: string[] = []
  if (event.metaKey) parts.push('Cmd')
  if (event.ctrlKey) parts.push('Ctrl')
  if (event.altKey) parts.push('Alt')
  if (event.shiftKey) parts.push('Shift')
  const key = event.key === ' ' ? 'Space' : event.key.length === 1 ? event.key.toUpperCase() : event.key
  return [...parts, key].join('+')
}

export function ShortcutRecorder({ value, disabled, recordingLabel, resetLabel, ariaLabel, onChange, onReset, className }: Props) {
  const [recording, setRecording] = useState(false)
  const keys = splitAccelerator(value)
  const handleKeyDown = (event: React.KeyboardEvent<HTMLButtonElement>) => {
    if (!recording) return
    event.preventDefault()
    event.stopPropagation()
    if (event.key === 'Escape') {
      setRecording(false)
      event.currentTarget.blur()
      return
    }
    const next = acceleratorFromEvent(event)
    if (!next) return
    setRecording(false)
    event.currentTarget.blur()
    if (next !== value) onChange(next)
  }
  return <div className={cn('flex items-center gap-1.5', className)}>
    <Button type="button" variant="outline" size="sm" disabled={disabled} aria-label={ariaLabel} aria-pressed={recording} className={cn('min-w-[148px] justify-center gap-1', recording && 'border-ring ring-[3px] ring-ring/20')} onClick={() => setRecording(true)} onKeyDown={handleKeyDown} onBlur={() => setRecording(false)}>
      {recording ? <span className="text-[10.5px] text-muted-foreground">{recordingLabel}</span> : <KbdGroup>{keys.map((key, index) => <Kbd key={`${key}-${index}`} className="h-5 min-w-5 px-1 text-[10.5px]">{key}</Kbd>)}</KbdGroup>}
    </Button>
    <Button type="button" variant="ghost" size="icon-sm" disabled={disabled} aria-label={resetLabel} title={resetLabel} onClick={onReset}><RotateCcw size={13} /></Button>
  </div>
}
