import { fireEvent, render, screen } from '@testing-library/react'
import { describe, expect, test, vi } from 'vitest'
import { NumberField } from '../components/ui/number-field'

describe('NumberField', () => {
  test('clamps through explicit increment and decrement buttons', () => {
    const onChange = vi.fn()
    const { rerender } = render(<NumberField value={5} min={5} max={8} ariaLabel="Retention" onChange={onChange} />)
    expect((screen.getByLabelText('Decrease') as HTMLButtonElement).disabled).toBe(true)
    fireEvent.click(screen.getByLabelText('Increase'))
    expect(onChange).toHaveBeenCalledWith(6)
    rerender(<NumberField value={8} min={5} max={8} ariaLabel="Retention" onChange={onChange} />)
    expect((screen.getByLabelText('Increase') as HTMLButtonElement).disabled).toBe(true)
    fireEvent.click(screen.getByLabelText('Decrease'))
    expect(onChange).toHaveBeenLastCalledWith(7)
  })

  test('renders the suffix without a native number input', () => {
    render(<NumberField value={24} suffix="h" ariaLabel="Interval" onChange={() => {}} />)
    expect(screen.getByText('h')).toBeTruthy()
    expect(document.querySelector('input[type="number"]')).toBeNull()
  })
})
