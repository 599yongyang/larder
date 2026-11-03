import { render, screen } from '@testing-library/react'
import { expect, test } from 'vitest'
import { MarkdownPreview } from './MarkdownPreview'

test('renders GFM tables', () => {
  render(<MarkdownPreview source={'|类型| 说明 |\n|---|---|\n| doc | 文档 |'} />)
  expect(screen.getByRole('table')).toBeTruthy()
  expect(screen.getByRole('columnheader', { name: '类型' })).toBeTruthy()
  expect(screen.getByRole('cell', { name: '文档' })).toBeTruthy()
})
