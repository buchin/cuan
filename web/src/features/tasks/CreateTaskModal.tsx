import { useState, useRef, useEffect } from 'react'
import { Project, TaskPriority, TaskStatus, DoDItem } from '../../api/types'
import { X, AlertCircle } from 'lucide-react'
import { trapDialogFocus } from '../../lib/dialog'

interface CreateTaskModalProps {
  isOpen: boolean
  projects: Project[]
  activeProjectId?: string
  onClose: () => void
  onSubmit: (data: {
    title: string
    description?: string
    project_ids?: string[]
    priority: TaskPriority
    status: TaskStatus
    dod_checklist?: DoDItem[]
  }) => Promise<void>
}

export function CreateTaskModal({
  isOpen,
  projects,
  activeProjectId,
  onClose,
  onSubmit,
}: CreateTaskModalProps) {
  const defaultProject =
    projects.find((p) => p.id === activeProjectId && !p.is_archived)?.id ||
    projects.find((p) => !p.is_archived)?.id ||
    ''
  const [title, setTitle] = useState('')
  const [description, setDescription] = useState('')
  const [priority, setPriority] = useState<TaskPriority>('normal')
  const [selectedProjectId, setSelectedProjectId] = useState(defaultProject)
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const dialogRef = useRef<HTMLDialogElement>(null)
  const titleRef = useRef<HTMLInputElement>(null)
  useEffect(() => {
    if (!isOpen) return
    setTitle('')
    setDescription('')
    setError(null)
    setPriority('normal')
    setSelectedProjectId(defaultProject)
  }, [isOpen, defaultProject])
  useEffect(() => {
    if (!isOpen) return
    const previous = document.activeElement as HTMLElement | null
    const dialog = dialogRef.current
    dialog?.showModal()
    titleRef.current?.focus()
    return () => {
      dialog?.close()
      if (previous?.isConnected) previous.focus()
    }
  }, [isOpen])
  if (!isOpen) return null
  return (
    <dialog
      ref={dialogRef}
      onKeyDown={trapDialogFocus}
      aria-labelledby="create-task-heading"
      onCancel={(e) => {
        e.preventDefault()
        if (!loading) onClose()
      }}
      className="m-auto w-[calc(100%_-_24px)] max-w-lg max-h-[90dvh] overflow-y-auto rounded-xl border border-border bg-card p-5 text-card-foreground shadow-2xl backdrop:bg-black/60"
    >
      <header className="mb-4 flex items-center justify-between gap-3">
        <h2 id="create-task-heading" className="text-xl font-semibold">
          New task
        </h2>
        <button
          type="button"
          aria-label="Tutup form task"
          className="icon-action"
          disabled={loading}
          onClick={onClose}
        >
          <X className="h-5 w-5" />
        </button>
      </header>
      {error && (
        <div
          role="alert"
          className="mb-4 flex items-start gap-2 rounded-md bg-destructive/10 p-3 text-sm text-destructive"
        >
          <AlertCircle className="h-4 w-4 shrink-0" />
          <span>{error}</span>
        </div>
      )}
      <form
        className="space-y-4"
        onSubmit={async (e) => {
          e.preventDefault()
          if (loading) return
          if (!title.trim()) {
            setError('Judul task tidak boleh kosong')
            return
          }
          setLoading(true)
          setError(null)
          try {
            await onSubmit({
              title: title.trim(),
              description: description.trim() || undefined,
              project_ids: selectedProjectId ? [selectedProjectId] : undefined,
              priority,
              status: 'todo',
            })
            onClose()
          } catch (err) {
            setError(err instanceof Error ? err.message : 'Gagal membuat task')
          } finally {
            setLoading(false)
          }
        }}
      >
        <div>
          <label htmlFor="create-task-title" className="text-sm font-medium">
            Judul task
          </label>
          <input
            id="create-task-title"
            ref={titleRef}
            className="field-control mt-1"
            required
            maxLength={200}
            placeholder="Apa yang ingin dikerjakan?"
            value={title}
            disabled={loading}
            onChange={(e) => setTitle(e.target.value)}
          />
        </div>
        <div>
          <label
            htmlFor="create-task-description"
            className="text-sm font-medium"
          >
            Deskripsi <span className="text-muted-foreground">(opsional)</span>
          </label>
          <textarea
            id="create-task-description"
            rows={4}
            className="field-control mt-1 resize-y"
            placeholder="Tujuan, scope, dan kriteria selesai"
            value={description}
            disabled={loading}
            onChange={(e) => setDescription(e.target.value)}
            onKeyDown={(e) => {
              if (e.key === 'Enter' && (e.ctrlKey || e.metaKey)) {
                e.preventDefault()
                e.currentTarget.form?.requestSubmit()
              }
            }}
          />
        </div>
        <div className="grid gap-3 sm:grid-cols-2">
          <div className="min-w-0">
            <label
              htmlFor="create-task-project"
              className="text-sm font-medium"
            >
              Project
            </label>
            <select
              id="create-task-project"
              className="field-control mt-1"
              value={selectedProjectId}
              disabled={loading}
              onChange={(e) => setSelectedProjectId(e.target.value)}
            >
              <option value="">Global / Tanpa project</option>
              {projects
                .filter((p) => !p.is_archived)
                .map((p) => (
                  <option key={p.id} value={p.id}>
                    {p.name}
                  </option>
                ))}
            </select>
          </div>
          <div className="min-w-0">
            <label
              htmlFor="create-task-priority"
              className="text-sm font-medium"
            >
              Prioritas
            </label>
            <select
              id="create-task-priority"
              className="field-control mt-1"
              value={priority}
              disabled={loading}
              onChange={(e) => setPriority(e.target.value as TaskPriority)}
            >
              <option value="low">Low</option>
              <option value="normal">Normal</option>
              <option value="high">High</option>
              <option value="urgent">Urgent</option>
            </select>
          </div>
        </div>
        <div className="flex justify-end gap-2 border-t border-border pt-4">
          <button
            type="button"
            className="action-secondary"
            disabled={loading}
            onClick={onClose}
          >
            Batal
          </button>
          <button
            type="submit"
            className="action-primary"
            disabled={loading || !title.trim()}
          >
            {loading ? 'Menyimpan…' : 'Buat task'}
          </button>
        </div>
      </form>
    </dialog>
  )
}
