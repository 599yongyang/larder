import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeEach, expect, test, vi } from 'vitest'
import { InstructionEditorDialog } from './InstructionEditorDialog'

const saveInstruction = vi.fn()
const createInstruction = vi.fn()
vi.mock('../backend', () => ({ backend: {
  instructionContent: async () => '# Rules\n\nold',
  saveInstruction: (...args: unknown[]) => saveInstruction(...args),
  createInstruction: (...args: unknown[]) => createInstruction(...args),
} }))

beforeEach(() => { saveInstruction.mockReset().mockResolvedValue(undefined); createInstruction.mockReset().mockResolvedValue('/tmp/new.md') })

test('edits an existing instruction file', async () => {
  const onSaved = vi.fn()
  const file = { id: 'i1', name: 'AGENTS.md', path: '/p/AGENTS.md', agent: '通用', scope: 'project', project: 'demo', size: 10, modifiedAt: '2026-01-01T00:00:00Z', hash: 'a', duplicate: false }
  render(<InstructionEditorDialog file={file} projects={[]} onClose={() => {}} onSaved={onSaved} />)
  const editor = await screen.findByLabelText('指令文件内容')
  await userEvent.clear(editor)
  await userEvent.type(editor, '# Rules\n\nnew')
  await userEvent.click(screen.getByRole('button', { name: '保存' }))
  await waitFor(() => expect(saveInstruction).toHaveBeenCalledWith('/p/AGENTS.md', '# Rules\n\nnew'))
  expect(onSaved).toHaveBeenCalled()
})

test('creates a new project instruction file', async () => {
  const onSaved = vi.fn()
  render(<InstructionEditorDialog projects={['/p/demo']} onClose={() => {}} onSaved={onSaved} />)
  expect(screen.getByText('新建指令文件')).toBeTruthy()
  expect(screen.getByLabelText('指令文件内容')).toBeTruthy()
})
