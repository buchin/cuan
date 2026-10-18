import { useState } from 'react'
import { Task, TaskStatus, TaskPriority, Project } from '../../api/types'
import { X, Play, Bot, CheckSquare, Terminal, Save, Trash2 } from 'lucide-react'

interface TaskSidePeekProps {
  task: Task | null
  projects: Project[]
  isOpen: boolean
  onClose: () => void
  onUpdate: (taskId: string, updates: Partial<Task>) => Promise<void>
  onDelete: (taskId: string) => Promise<void>
  onRunAgent: (taskId: string) => Promise<void>
}

export function TaskSidePeek({
  task,
  projects,
  isOpen,
  onClose,
  onUpdate,
  onDelete,
  onRunAgent,
}: TaskSidePeekProps) {
  if (!isOpen || !task) return null

  const linkedProject = projects.find(p => p.id === task.project_id || task.project_ids?.includes(p.id))
  const [activeTab, setActiveTab] = useState<'details' | 'runner'>('details')
  const [title, setTitle] = useState(task.title)
  const [description, setDescription] = useState(task.description || task.body || task.prompt || '')
  const [status, setStatus] = useState<TaskStatus>(task.status)
  const [priority, setPriority] = useState<TaskPriority>(task.priority || 'normal')
  const [saving, setSaving] = useState(false)
  const [agentRunning, setAgentRunning] = useState(false)
  const [agentLogs, setAgentLogs] = useState<string[]>([
    `[${new Date().toLocaleTimeString()}] Task loaded: ${task.id}`,
    `[${new Date().toLocaleTimeString()}] Current status: ${task.status}`,
  ])

  const handleSave = async () => {
    setSaving(true)
    try {
      await onUpdate(task.id, {
        title,
        description,
        status,
        priority,
      })
      onClose()
    } catch (err) {
      console.error(err)
    } finally {
      setSaving(false)
    }
  }

  const handleTriggerAgent = async () => {
    setAgentRunning(true)
    setActiveTab('runner')
    setAgentLogs((prev) => [
      ...prev,
      `[${new Date().toLocaleTimeString()}] Dispatched agent runner for task ${task.id}...`,
      `[${new Date().toLocaleTimeString()}] Workspace sandbox allocated. Preparing task runner...`,
    ])

    try {
      await onRunAgent(task.id)
      setAgentLogs((prev) => [
        ...prev,
        `[${new Date().toLocaleTimeString()}] Agent runner executed successfully.`,
      ])
    } catch (err: any) {
      setAgentLogs((prev) => [
        ...prev,
        `[${new Date().toLocaleTimeString()}] Execution error: ${err.message || 'Unknown runner error'}`,
      ])
    } finally {
      setAgentRunning(false)
    }
  }

  return (
    <div className="fixed inset-y-0 right-0 z-40 w-full sm:w-[540px] bg-card text-card-foreground border-l border-border shadow-2xl flex flex-col font-mono select-none">
      {/* Header */}
      <div className="h-14 border-b border-border px-6 flex items-center justify-between shrink-0">
        <div className="flex items-center gap-2">
          <span className="text-[10px] text-muted-foreground uppercase tracking-wider font-bold">
            Task
          </span>
          <span className="text-xs font-mono text-primary font-bold">
            #{task.id.slice(0, 8)}
          </span>
          {linkedProject && (
            <span className="text-[10px] px-1.5 py-0.5 bg-primary/10 text-primary border border-primary/20 truncate max-w-[120px] rounded">
              {linkedProject.name}
            </span>
          )}
        </div>

        <div className="flex items-center gap-2">
          <button
            onClick={handleTriggerAgent}
            disabled={agentRunning}
            className="flex items-center gap-1.5 px-2.5 py-1 bg-emerald-500 text-white font-bold text-xs hover:bg-emerald-600 transition-colors disabled:opacity-50 rounded"
            title="Execute task with AI Agent"
          >
            <Play className="h-3 w-3 fill-current" />
            <span>{agentRunning ? 'Running...' : 'Run Agent'}</span>
          </button>
          <button
            onClick={onClose}
            className="p-1.5 text-muted-foreground hover:text-foreground transition-colors"
          >
            <X className="h-4 w-4" />
          </button>
        </div>
      </div>

      {/* Tabs */}
      <div className="flex border-b border-border text-xs px-6 bg-muted/40 shrink-0">
        <button
          onClick={() => setActiveTab('details')}
          className={`py-2.5 px-3 font-semibold border-b-2 transition-colors ${
            activeTab === 'details'
              ? 'border-primary text-primary'
              : 'border-transparent text-muted-foreground hover:text-foreground'
          }`}
        >
          Task Brief & DoD
        </button>
        <button
          onClick={() => setActiveTab('runner')}
          className={`py-2.5 px-3 font-semibold border-b-2 flex items-center gap-1.5 transition-colors ${
            activeTab === 'runner'
              ? 'border-primary text-primary'
              : 'border-transparent text-muted-foreground hover:text-foreground'
          }`}
        >
          <Terminal className="h-3.5 w-3.5" />
          <span>Agent Runner Console</span>
        </button>
      </div>

      {/* Body */}
      <div className="flex-1 overflow-y-auto p-6 space-y-5">
        {activeTab === 'details' ? (
          <>
            <div>
              <label className="block text-[11px] font-bold text-muted-foreground uppercase tracking-wider mb-1">
                Title
              </label>
              <input
                type="text"
                value={title}
                onChange={(e) => setTitle(e.target.value)}
                className="w-full px-3 py-2 text-xs bg-background border border-input text-foreground font-semibold focus:outline-none focus:ring-1 focus:ring-ring rounded"
              />
            </div>

            <div className="grid grid-cols-2 gap-4">
              <div>
                <label className="block text-[11px] font-bold text-muted-foreground uppercase tracking-wider mb-1">
                  Status
                </label>
                <select
                  value={status}
                  onChange={(e) => setStatus(e.target.value as TaskStatus)}
                  className="w-full px-3 py-1.5 text-xs bg-background border border-input text-foreground focus:outline-none focus:ring-1 focus:ring-ring rounded"
                >
                  <option value="backlog">Backlog</option>
                  <option value="todo">To Do</option>
                  <option value="doing">In Progress</option>
                  <option value="review">In Review</option>
                  <option value="done">Completed</option>
                </select>
              </div>

              <div>
                <label className="block text-[11px] font-bold text-muted-foreground uppercase tracking-wider mb-1">
                  Priority
                </label>
                <select
                  value={priority}
                  onChange={(e) => setPriority(e.target.value as TaskPriority)}
                  className="w-full px-3 py-1.5 text-xs bg-background border border-input text-foreground focus:outline-none focus:ring-1 focus:ring-ring rounded"
                >
                  <option value="low">Low</option>
                  <option value="normal">Normal</option>
                  <option value="high">High</option>
                  <option value="urgent">Urgent</option>
                </select>
              </div>
            </div>

            <div>
              <label className="block text-[11px] font-bold text-muted-foreground uppercase tracking-wider mb-1">
                Prompt / Task Description
              </label>
              <textarea
                rows={8}
                value={description}
                onChange={(e) => setDescription(e.target.value)}
                placeholder="Describe instructions for the human or AI Agent..."
                className="w-full px-3 py-2 text-xs bg-background border border-input text-foreground focus:outline-none focus:ring-1 focus:ring-ring rounded"
              />
            </div>

            {task.dod_checklist && task.dod_checklist.length > 0 && (
              <div>
                <label className="block text-[11px] font-bold text-muted-foreground uppercase tracking-wider mb-2">
                  Definition of Done (DoD)
                </label>
                <div className="space-y-1.5 bg-muted/40 p-3 border border-border rounded">
                  {task.dod_checklist.map((item, idx) => {
                    const text = typeof item === 'string' ? item : item.text
                    const completed = typeof item === 'object' && !!item.completed
                    return (
                      <div key={idx} className="flex items-start gap-2 text-xs">
                        <CheckSquare className={`h-3.5 w-3.5 ${completed ? 'text-emerald-500' : 'text-primary'} shrink-0 mt-0.5`} />
                        <span className={completed ? 'line-through text-muted-foreground' : 'text-foreground'}>
                          {text}
                        </span>
                      </div>
                    )
                  })}
                </div>
              </div>
            )}
          </>
        ) : (
          <div className="space-y-4">
            <div className="flex items-center justify-between">
              <span className="text-xs font-semibold text-foreground flex items-center gap-1.5">
                <Bot className="h-4 w-4 text-primary" />
                <span>Agent Execution Terminal</span>
              </span>
              <span className="text-[10px] text-muted-foreground">WebSocket: Online</span>
            </div>

            <div className="bg-zinc-950 border border-border p-3 text-[11px] font-mono text-zinc-200 h-80 overflow-y-auto space-y-1 rounded">
              {agentLogs.map((log, index) => (
                <div key={index} className="leading-relaxed">
                  {log.includes('error') ? (
                    <span className="text-destructive font-semibold">{log}</span>
                  ) : log.includes('successfully') ? (
                    <span className="text-emerald-400 font-semibold">{log}</span>
                  ) : (
                    <span>{log}</span>
                  )}
                </div>
              ))}
            </div>

            <button
              onClick={handleTriggerAgent}
              disabled={agentRunning}
              className="w-full py-2 bg-primary text-primary-foreground font-bold text-xs hover:bg-primary/90 transition-colors flex items-center justify-center gap-2 disabled:opacity-50 rounded"
            >
              <Play className="h-3.5 w-3.5 fill-current" />
              <span>{agentRunning ? 'Agent is working...' : 'Run Agent on Task'}</span>
            </button>
          </div>
        )}
      </div>

      {/* Footer */}
      {activeTab === 'details' && (
        <div className="h-14 border-t border-border px-6 flex items-center justify-between shrink-0">
          <button
            onClick={() => onDelete(task.id)}
            className="flex items-center gap-1.5 text-xs text-destructive hover:underline"
          >
            <Trash2 className="h-3.5 w-3.5" />
            <span>Delete</span>
          </button>

          <button
            onClick={handleSave}
            disabled={saving}
            className="flex items-center gap-1.5 px-4 py-2 bg-primary text-primary-foreground font-semibold text-xs hover:bg-primary/90 transition-colors disabled:opacity-50 rounded"
          >
            <Save className="h-3.5 w-3.5" />
            <span>{saving ? 'Saving...' : 'Save Changes'}</span>
          </button>
        </div>
      )}
    </div>
  )
}
