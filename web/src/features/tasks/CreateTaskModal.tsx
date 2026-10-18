import { useState } from 'react'
import { Project, TaskPriority, TaskStatus, DoDItem } from '../../api/types'
import { X, CheckSquare, Trash2, AlertCircle } from 'lucide-react'

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
  const [status, setStatus] = useState<TaskStatus>('todo')
  const [selectedProjectId, setSelectedProjectId] = useState(activeProjectId || (projects[0]?.id || ''))
  const [dodItems, setDodItems] = useState<string[]>([])
  const [newDod, setNewDod] = useState('')
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)

  if (!isOpen) return null

  const handleAddDod = () => {
    if (newDod.trim()) {
      setDodItems([...dodItems, newDod.trim()])
      setNewDod('')
    }
  }

  const handleRemoveDod = (index: number) => {
    setDodItems(dodItems.filter((_, i) => i !== index))
  }

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    if (!title.trim()) {
      setError('Task title is required')
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
        dod_checklist: dodItems.length > 0 ? dodItems.map((text) => ({ text, completed: false })) : undefined,
      })
      onClose()
      setTitle('')
      setDescription('')
      setDodItems([])
    } catch (err: any) {
      setError(err.message || 'Failed to create task')
    } finally {
      setLoading(false)
    }
  }

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/60 backdrop-blur-sm p-4 font-mono select-none">
      <div className="w-full max-w-lg bg-card border border-border shadow-2xl p-6 max-h-[90vh] overflow-y-auto rounded-lg text-card-foreground">
        <div className="flex items-center justify-between pb-4 border-b border-border mb-4">
          <div className="flex items-center gap-2">
            <CheckSquare className="h-5 w-5 text-primary" />
            <h2 className="text-sm font-bold text-foreground">
              Create New Task
            </h2>
          </div>
          <button
            onClick={onClose}
            className="text-muted-foreground hover:text-foreground transition-colors"
          >
            <X className="h-4 w-4" />
          </button>
        </div>

        {error && (
          <div className="mb-4 p-2.5 bg-destructive/10 border border-destructive/30 text-destructive text-xs flex items-center gap-2 rounded">
            <AlertCircle className="h-4 w-4 shrink-0" />
            <span>{error}</span>
          </div>
        )}

        <form onSubmit={handleSubmit} className="space-y-4">
          <div>
            <label className="block text-xs font-semibold text-muted-foreground mb-1">
              Task Title *
            </label>
            <input
              type="text"
              required
              placeholder="e.g. Implement user login, Draft YouTube script"
              value={title}
              onChange={(e) => setTitle(e.target.value)}
              className="w-full px-3 py-2 text-xs bg-background border border-input text-foreground focus:outline-none focus:ring-1 focus:ring-ring rounded"
            />
          </div>

          <div className="grid grid-cols-3 gap-3">
            <div>
              <label className="block text-xs font-semibold text-muted-foreground mb-1">
                Project Link
              </label>
              <select
                value={selectedProjectId}
                onChange={(e) => setSelectedProjectId(e.target.value)}
                className="w-full px-2.5 py-1.5 text-xs bg-background border border-input text-foreground focus:outline-none focus:ring-1 focus:ring-ring rounded"
              >
                <option value="">(Global)</option>
                {projects.map((p) => (
                  <option key={p.id} value={p.id}>
                    {p.name}
                  </option>
                ))}
              </select>
            </div>

            <div>
              <label className="block text-xs font-semibold text-muted-foreground mb-1">
                Priority
              </label>
              <select
                value={priority}
                onChange={(e) => setPriority(e.target.value as TaskPriority)}
                className="w-full px-2.5 py-1.5 text-xs bg-background border border-input text-foreground focus:outline-none focus:ring-1 focus:ring-ring rounded"
              >
                <option value="low">Low</option>
                <option value="normal">Normal</option>
                <option value="high">High</option>
                <option value="urgent">Urgent</option>
              </select>
            </div>

            <div>
              <label className="block text-xs font-semibold text-muted-foreground mb-1">
                Status
              </label>
              <select
                value={status}
                onChange={(e) => setStatus(e.target.value as TaskStatus)}
                className="w-full px-2.5 py-1.5 text-xs bg-background border border-input text-foreground focus:outline-none focus:ring-1 focus:ring-ring rounded"
              >
                <option value="backlog">Backlog</option>
                <option value="todo">To Do</option>
                <option value="doing">In Progress</option>
              </select>
            </div>
          </div>

          <div>
            <label className="block text-xs font-semibold text-muted-foreground mb-1">
              Task Prompt / Description
            </label>
            <textarea
              rows={4}
              placeholder="Provide detailed instructions or prompt for this task..."
              value={description}
              onChange={(e) => setDescription(e.target.value)}
              className="w-full px-3 py-2 text-xs bg-background border border-input text-foreground focus:outline-none focus:ring-1 focus:ring-ring rounded"
            />
          </div>

          <div>
            <label className="block text-xs font-semibold text-muted-foreground mb-1">
              Definition of Done (DoD) Checklist
            </label>
            <div className="flex gap-2 mb-2">
              <input
                type="text"
                placeholder="Add acceptance criterion..."
                value={newDod}
                onChange={(e) => setNewDod(e.target.value)}
                onKeyDown={(e) => {
                  if (e.key === 'Enter') {
                    e.preventDefault()
                    handleAddDod()
                  }
                }}
                className="flex-1 px-3 py-1.5 text-xs bg-background border border-input text-foreground focus:outline-none focus:ring-1 focus:ring-ring rounded"
              />
              <button
                type="button"
                onClick={handleAddDod}
                className="px-3 py-1.5 bg-secondary text-secondary-foreground hover:bg-secondary/80 text-xs transition-colors rounded"
              >
                Add
              </button>
            </div>

            {dodItems.length > 0 && (
              <div className="space-y-1 bg-muted/40 p-2.5 border border-border max-h-32 overflow-y-auto rounded">
                {dodItems.map((item, idx) => (
                  <div key={idx} className="flex items-center justify-between text-xs py-0.5">
                    <span className="text-foreground truncate">• {item}</span>
                    <button
                      type="button"
                      onClick={() => handleRemoveDod(idx)}
                      className="text-destructive hover:opacity-80 ml-2"
                    >
                      <Trash2 className="h-3 w-3" />
                    </button>
                  </div>
                ))}
              </div>
            )}
          </div>

          <div className="pt-2 flex items-center justify-end gap-2 border-t border-border">
            <button
              type="button"
              onClick={onClose}
              className="px-4 py-2 text-xs text-muted-foreground hover:text-foreground transition-colors"
            >
              Cancel
            </button>
            <button
              type="submit"
              disabled={loading}
              className="px-4 py-2 bg-primary text-primary-foreground font-semibold text-xs hover:bg-primary/90 transition-colors disabled:opacity-50 rounded"
            >
              {loading ? 'Creating...' : 'Create Task'}
            </button>
          </div>
        </form>
      </div>
    </div>
  )
}
