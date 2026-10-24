import { useEffect, useState } from 'react'
import { Button } from '@/components/ui/button'
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { backend } from '../backend'
import styles from '../App.module.css'
import type { InstructionFile } from '../types'

const instructionNames = ['AGENTS.md', 'CLAUDE.md', 'GEMINI.md', '.cursorrules', '.claude/CLAUDE.md', '.cursor/rules/project.mdc', '.github/copilot-instructions.md']

export function InstructionEditorDialog({ file, projects, project, onClose, onSaved }: { file?: InstructionFile; projects: string[]; project?: string; onClose: () => void; onSaved: () => void }) {
  const [content, setContent] = useState('')
  const [loading, setLoading] = useState(Boolean(file))
  const [saving, setSaving] = useState(false)
  const [error, setError] = useState('')
  const [selectedProject, setSelectedProject] = useState(project || projects[0] || '')
  const [name, setName] = useState('AGENTS.md')
  useEffect(() => {
    if (!file) return
    let active = true
    backend.instructionContent(file.path).then((next) => { if (active) setContent(next) }).catch((cause) => { if (active) setError(cause instanceof Error ? cause.message : '无法读取文件') }).finally(() => { if (active) setLoading(false) })
    return () => { active = false }
  }, [file])
  const save = async () => {
    if (saving) return
    setSaving(true); setError('')
    try {
      if (file) await backend.saveInstruction(file.path, content)
      else await backend.createInstruction(selectedProject, name, content)
      onSaved(); onClose()
    } catch (cause) { setError(cause instanceof Error ? cause.message : '保存失败') } finally { setSaving(false) }
  }
  const valid = Boolean(file || (selectedProject && name)) && !loading
  return <Dialog open onOpenChange={(open) => { if (!open && !saving) onClose() }}>
    <DialogContent className="max-w-2xl gap-0 p-0">
      <DialogHeader className="border-b px-6 py-4 pr-12"><DialogTitle className="text-base">{file ? `编辑 ${file.name}` : '新建指令文件'}</DialogTitle><DialogDescription className="text-xs">{file ? file.path : '选择项目和文件名；保存前会备份已有内容。'}</DialogDescription></DialogHeader>
      <div className={styles.instructionEditorBody}>
        {!file && <div className={styles.instructionEditorRow}><Select value={selectedProject} onValueChange={setSelectedProject}><SelectTrigger size="sm"><SelectValue placeholder="选择项目" /></SelectTrigger><SelectContent>{projects.map((item) => <SelectItem key={item} value={item}>{item.split('/').filter(Boolean).pop()}</SelectItem>)}</SelectContent></Select><Select value={name} onValueChange={setName}><SelectTrigger size="sm"><SelectValue /></SelectTrigger><SelectContent>{instructionNames.map((item) => <SelectItem key={item} value={item}>{item}</SelectItem>)}</SelectContent></Select></div>}
        {loading ? <p className={styles.instructionEditorHint}>读取中…</p> : <textarea className={styles.instructionEditor} value={content} onChange={(event) => setContent(event.target.value)} spellCheck={false} aria-label="指令文件内容" />}
        {error && <p className={styles.planError} role="alert">{error}</p>}
        <div className={styles.instructionEditorMeta}><span>{content.length.toLocaleString()} 字符</span><span>保存时会保留一份备份</span></div>
      </div>
      <DialogFooter className="border-t px-6 py-2.5"><Button size="sm" variant="outline" disabled={saving} onClick={onClose}>取消</Button><Button size="sm" disabled={saving || !valid} onClick={() => void save()}>{saving ? '保存中…' : '保存'}</Button></DialogFooter>
    </DialogContent>
  </Dialog>
}
