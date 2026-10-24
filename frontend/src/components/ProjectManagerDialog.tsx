import { useState } from 'react'
import { Folder, FolderOpen, Trash2 } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { backend } from '../backend'
import styles from '../App.module.css'
import { toastError } from '../lib/toast'

type Project = { path: string; label: string; count: number }
export function ProjectManagerDialog({ projects, onClose, onAdd, onRemove }: { projects: Project[]; onClose: () => void; onAdd: (path: string) => Promise<void>; onRemove: (path: string) => Promise<void> }) {
  const [busy, setBusy] = useState('')
  const [error, setError] = useState('')
  const browse = async () => {
    setError('')
    try {
      const path = await backend.pickFolder()
      if (!path) return
      if (projects.some((project) => project.path === path)) { setError('这个项目已经添加过了'); return }
      setBusy('add')
      await onAdd(path)
    } catch (cause) {
      toastError(cause, '添加项目失败')
    } finally {
      setBusy('')
    }
  }
  const remove = async (path: string) => {
    setBusy(path)
    setError('')
    try { await onRemove(path) } catch { setError('移除失败') } finally { setBusy('') }
  }
  return <Dialog open onOpenChange={(open) => { if (!open && !busy) onClose() }}>
    <DialogContent className="max-w-2xl gap-0 p-0">
      <DialogHeader className="border-b px-6 py-4 pr-14"><DialogTitle className="text-base">管理项目</DialogTitle><DialogDescription className="max-w-xl text-xs leading-5">项目 Skills 只读取你主动添加的工作区；移除项目不会删除任何文件。</DialogDescription></DialogHeader>
      <div className="max-h-[56vh] overflow-y-auto px-6 py-5">
        <Button variant="outline" size="sm" disabled={busy === 'add'} onClick={() => void browse()}><FolderOpen size={14} />{busy === 'add' ? '添加中…' : '选择项目文件夹'}</Button>
        {error && <p className={styles.planError} role="alert">{error}</p>}
        <div className={styles.projectManagerList}>{projects.map((project) => <div key={project.path} className={styles.projectManagerRow}><Folder size={16} aria-hidden="true" /><div><strong>{project.label}</strong><small>{project.path}</small></div><span>{project.count} 个 Skill</span><Button variant="ghost" size="xs" className="text-destructive" disabled={busy === project.path} onClick={() => void remove(project.path)}><Trash2 size={13} />移除</Button></div>)}{projects.length === 0 && <p className={styles.footerHint}>还没有项目工作区，点击上方按钮选择一个文件夹。</p>}</div>
      </div>
      <DialogFooter className="border-t px-6 py-3"><Button size="sm" variant="outline" onClick={onClose} disabled={Boolean(busy)}>关闭</Button></DialogFooter>
    </DialogContent>
  </Dialog>
}
