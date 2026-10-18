import { useState, useRef, useEffect } from 'react'
import { Project, TaskPriority, TaskStatus, DoDItem } from '../../api/types'
import { X, CheckSquare, AlertCircle, FolderGit2, Flag, CornerDownLeft } from 'lucide-react'

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
  const [title, setTitle] = useState('')
  const [description, setDescription] = useState('')
  const [priority, setPriority] = useState<TaskPriority>('normal')
  const [status] = useState<TaskStatus>('todo')
  const [selectedProjectId, setSelectedProjectId] = useState(activeProjectId || (projects[0]?.id || ''))
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)

  const titleInputRef = useRef<HTMLInputElement>(null)

  // Reset and autofocus title whenever modal opens
  useEffect(() => {
    if (isOpen) {
      setTitle('')
      setDescription('')
      setError(null)
      setSelectedProjectId(activeProjectId || (projects[0]?.id || ''))
      setPriority('normal')
      setTimeout(() => {
        titleInputRef.current?.focus()
      }, 50)
    }
  }, [isOpen, activeProjectId, projects])

  // Handle escape key
  useEffect(() => {
    const handleKeyDown = (e: KeyboardEvent) => {
      if (e.key === 'Escape' && isOpen) {
        onClose()
      }
    }
    window.addEventListener('keydown', handleKeyDown)
    return () => window.removeEventListener('keydown', handleKeyDown)
  }, [isOpen, onClose])

  if (!isOpen) return null

  const submitForm = async () => {
    if (!title.trim() || loading) {
      if (!title.trim()) setError('Judul task tidak boleh kosong')
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
        status,
      })
      onClose()
    } catch (err: any) {
      setError(err.message || 'Gagal membuat task')
    } finally {
      setLoading(false)
    }
  }

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault()
    submitForm()
  }

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/60 backdrop-blur-sm p-3 sm:p-4 select-none">
      <div className="w-full max-w-lg bg-card border border-border shadow-2xl rounded-xl p-5 text-card-foreground animate-in fade-in-0 zoom-in-95 duration-150">
        {/* Header: Minimal title & Close */}
        <div className="flex items-center justify-between pb-3 border-b border-border mb-3">
          <div className="flex items-center gap-2">
            <CheckSquare className="h-4 w-4 text-primary" />
            <h2 className="text-xs font-bold tracking-tight text-foreground uppercase">
              Tugas Baru (Quick Task)
            </h2>
          </div>
          <button
            type="button"
            onClick={onClose}
            className="p-1 text-muted-foreground hover:text-foreground rounded transition-colors"
          >
            <X className="h-4 w-4" />
          </button>
        </div>

        {error && (
          <div className="mb-3 p-2 bg-destructive/10 border border-destructive/30 text-destructive text-xs flex items-center gap-2 rounded-md">
            <AlertCircle className="h-3.5 w-3.5 shrink-0" />
            <span>{error}</span>
          </div>
        )}

        <form onSubmit={handleSubmit} className="space-y-3">
          {/* Title Input with autofocus & Enter shortcut */}
          <div>
            <input
              ref={titleInputRef}
              type="text"
              autoFocus
              required
              placeholder="Apa yang ingin dikerjakan?..."
              value={title}
              onChange={(e) => setTitle(e.target.value)}
              onKeyDown={(e) => {
                if (e.key === 'Enter') {
                  e.preventDefault()
                  submitForm()
                }
              }}
              className="w-full px-3.5 py-2.5 text-sm font-medium bg-background border border-input text-foreground placeholder:text-muted-foreground focus:outline-none focus:ring-1 focus:ring-ring rounded-lg shadow-2xs"
            />
          </div>

          {/* Description Textarea (Optional, Ctrl+Enter to submit) */}
          <div>
            <textarea
              rows={3}
              placeholder="Deskripsi atau instruksi tambahan (opsional)..."
              value={description}
              onChange={(e) => setDescription(e.target.value)}
              onKeyDown={(e) => {
                if (e.key === 'Enter' && (e.ctrlKey || e.metaKey)) {
                  e.preventDefault()
                  submitForm()
                }
              }}
              className="w-full px-3.5 py-2 text-xs bg-background border border-input text-foreground placeholder:text-muted-foreground focus:outline-none focus:ring-1 focus:ring-ring rounded-lg resize-none leading-relaxed"
            />
          </div>

          {/* Sub-panel Controls: Project dropdown & Priority */}
          <div className="flex flex-wrap items-center justify-between gap-2 pt-1 border-t border-border">
            <div className="flex items-center gap-2 flex-1 min-w-[200px]">
              {/* Project selector */}
              <div className="flex items-center gap-1.5 flex-1 min-w-0">
                <FolderGit2 className="h-3.5 w-3.5 text-muted-foreground shrink-0" />
                <select
                  value={selectedProjectId}
                  onChange={(e) => setSelectedProjectId(e.target.value)}
                  className="w-full px-2 py-1 text-xs bg-background border border-input text-foreground focus:outline-none focus:ring-1 focus:ring-ring rounded-md truncate cursor-pointer"
                >
                  <option value="">(Global / Tanpa Project)</option>
                  {projects.map((p) => (
                    <option key={p.id} value={p.id}>
                      {p.name}
                    </option>
                  ))}
                </select>
              </div>

              {/* Priority selector */}
              <div className="flex items-center gap-1 shrink-0">
                <Flag className="h-3.5 w-3.5 text-muted-foreground shrink-0" />
                <select
                  value={priority}
                  onChange={(e) => setPriority(e.target.value as TaskPriority)}
                  className="px-2 py-1 text-xs bg-background border border-input text-foreground focus:outline-none focus:ring-1 focus:ring-ring rounded-md cursor-pointer"
                >
                  <option value="low">Low</option>
                  <option value="normal">Normal</option>
                  <option value="high">High</option>
                  <option value="urgent">Urgent</option>
                </select>
              </div>
            </div>

            {/* Actions */}
            <div className="flex items-center gap-2 ml-auto shrink-0">
              <button
                type="button"
                onClick={onClose}
                className="px-3 py-1.5 text-xs text-muted-foreground hover:text-foreground transition-colors"
              >
                Batal
              </button>
              <button
                type="submit"
                disabled={loading || !title.trim()}
                className="px-3.5 py-1.5 bg-primary text-primary-foreground font-semibold text-xs hover:bg-primary/90 transition-all disabled:opacity-50 rounded-md flex items-center gap-1.5 shadow-xs cursor-pointer"
              >
                <span>{loading ? 'Menyimpan...' : 'Buat Tugas'}</span>
                <CornerDownLeft className="h-3 w-3 opacity-70" />
              </button>
            </div>
          </div>
        </form>
      </div>
    </div>
  )
}
