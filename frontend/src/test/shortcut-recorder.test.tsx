import { fireEvent, render, screen } from '@testing-library/react'
import { describe, expect, test, vi } from 'vitest'
import { ShortcutRecorder } from '../components/ui/shortcut-recorder'

describe('ShortcutRecorder', () => {
  test('captures a new accelerator and exposes reset', () => {
    const onChange = vi.fn()
    const onReset = vi.fn()
    render(<ShortcutRecorder value="CmdOrCtrl+Shift+L" recordingLabel="Press shortcut…" resetLabel="Reset shortcut" ariaLabel="Global shortcut" onChange={onChange} onReset={onReset} />)
    const field = screen.getByRole('button', { name: 'Global shortcut' })
    fireEvent.click(field)
    fireEvent.keyDown(field, { key: 'K', metaKey: true, shiftKey: true })
    expect(onChange).toHaveBeenCalledWith('Cmd+Shift+K')
    fireEvent.click(screen.getByRole('button', { name: 'Reset shortcut' }))
    expect(onReset).toHaveBeenCalledOnce()
  })
})
