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
    <div className="fixed inset-y-0 right-0 z-40 w-full sm:w-[540px] bg-[#e2e7f1] dark:bg-[#16161e] border-l border-[#cfd6e6] dark:border-[#24283b] shadow-2xl flex flex-col font-mono select-none">
      {/* Header */}
      <div className="h-14 border-b border-[#cfd6e6] dark:border-[#24283b] px-6 flex items-center justify-between shrink-0">
        <div className="flex items-center gap-2">
          <span className="text-[10px] text-[#565f89] uppercase tracking-wider font-bold">
            Task
          </span>
          <span className="text-xs font-mono text-[#7aa2f7]">
            #{task.id.slice(0, 8)}
          </span>
          {linkedProject && (
            <span className="text-[10px] px-1.5 py-0.5 bg-[#7aa2f7]/10 text-[#7aa2f7] border border-[#7aa2f7]/20 truncate max-w-[120px]">
              {linkedProject.name}
            </span>
          )}
        </div>

        <div className="flex items-center gap-2">
          <button
            onClick={handleTriggerAgent}
            disabled={agentRunning}
            className="flex items-center gap-1.5 px-2.5 py-1 bg-[#9ece6a] text-[#16161e] font-bold text-xs hover:bg-[#a9e27b] transition-colors disabled:opacity-50"
            title="Execute task with AI Agent"
          >
            <Play className="h-3 w-3 fill-current" />
            <span>{agentRunning ? 'Running...' : 'Run Agent'}</span>
          </button>
          <button
            onClick={onClose}
            className="p-1.5 text-[#565f89] hover:text-[#f7768e] transition-colors"
          >
            <X className="h-4 w-4" />
          </button>
        </div>
      </div>

      {/* Tabs */}
      <div className="flex border-b border-[#cfd6e6] dark:border-[#24283b] text-xs px-6 bg-[#f0f3f8] dark:bg-[#1f2335]/40 shrink-0">
        <button
          onClick={() => setActiveTab('details')}
          className={`py-2.5 px-3 font-semibold border-b-2 transition-colors ${
            activeTab === 'details'
              ? 'border-[#7aa2f7] text-[#7aa2f7]'
              : 'border-transparent text-[#565f89] hover:text-[#1a1b26] dark:hover:text-[#c0caf5]'
          }`}
        >
          Task Brief & DoD
        </button>
        <button
          onClick={() => setActiveTab('runner')}
          className={`py-2.5 px-3 font-semibold border-b-2 flex items-center gap-1.5 transition-colors ${
            activeTab === 'runner'
              ? 'border-[#7aa2f7] text-[#7aa2f7]'
              : 'border-transparent text-[#565f89] hover:text-[#1a1b26] dark:hover:text-[#c0caf5]'
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
              <label className="block text-[11px] font-bold text-[#565f89] uppercase tracking-wider mb-1">
                Title
              </label>
              <input
                type="text"
                value={title}
                onChange={(e) => setTitle(e.target.value)}
                className="w-full px-3 py-2 text-xs bg-[#d5dce8] dark:bg-[#1f2335] border border-[#bcc5d8] dark:border-[#292e42] text-[#1a1b26] dark:text-[#c0caf5] font-semibold focus:outline-none focus:border-[#7aa2f7]"
              />
            </div>

            <div className="grid grid-cols-2 gap-4">
              <div>
                <label className="block text-[11px] font-bold text-[#565f89] uppercase tracking-wider mb-1">
                  Status
                </label>
                <select
                  value={status}
                  onChange={(e) => setStatus(e.target.value as TaskStatus)}
                  className="w-full px-3 py-1.5 text-xs bg-[#d5dce8] dark:bg-[#1f2335] border border-[#bcc5d8] dark:border-[#292e42] text-[#1a1b26] dark:text-[#c0caf5] focus:outline-none focus:border-[#7aa2f7]"
                >
                  <option value="backlog">Backlog</option>
                  <option value="todo">To Do</option>
                  <option value="doing">In Progress</option>
                  <option value="review">In Review</option>
                  <option value="done">Completed</option>
                </select>
              </div>

              <div>
                <label className="block text-[11px] font-bold text-[#565f89] uppercase tracking-wider mb-1">
                  Priority
                </label>
                <select
                  value={priority}
                  onChange={(e) => setPriority(e.target.value as TaskPriority)}
                  className="w-full px-3 py-1.5 text-xs bg-[#d5dce8] dark:bg-[#1f2335] border border-[#bcc5d8] dark:border-[#292e42] text-[#1a1b26] dark:text-[#c0caf5] focus:outline-none focus:border-[#7aa2f7]"
                >
                  <option value="low">Low</option>
                  <option value="normal">Normal</option>
                  <option value="high">High</option>
                  <option value="urgent">Urgent</option>
                </select>
              </div>
            </div>

            <div>
              <label className="block text-[11px] font-bold text-[#565f89] uppercase tracking-wider mb-1">
                Prompt / Task Description
              </label>
              <textarea
                rows={8}
                value={description}
                onChange={(e) => setDescription(e.target.value)}
                placeholder="Describe instructions for the human or AI Agent..."
                className="w-full px-3 py-2 text-xs bg-[#d5dce8] dark:bg-[#1f2335] border border-[#bcc5d8] dark:border-[#292e42] text-[#1a1b26] dark:text-[#c0caf5] focus:outline-none focus:border-[#7aa2f7]"
              />
            </div>

            {task.dod_checklist && task.dod_checklist.length > 0 && (
              <div>
                <label className="block text-[11px] font-bold text-[#565f89] uppercase tracking-wider mb-2">
                  Definition of Done (DoD)
                </label>
                <div className="space-y-1.5 bg-[#d5dce8]/40 dark:bg-[#1f2335]/30 p-3 border border-[#cfd6e6] dark:border-[#24283b]">
                  {task.dod_checklist.map((item, idx) => (
                    <div key={idx} className="flex items-start gap-2 text-xs">
                      <CheckSquare className="h-3.5 w-3.5 text-[#9ece6a] shrink-0 mt-0.5" />
                      <span className="text-[#343b58] dark:text-[#a9b1d6]">{item}</span>
                    </div>
                  ))}
                </div>
              </div>
            )}
          </>
        ) : (
          <div className="space-y-4">
            <div className="flex items-center justify-between">
              <span className="text-xs font-semibold text-[#1a1b26] dark:text-[#c0caf5] flex items-center gap-1.5">
                <Bot className="h-4 w-4 text-[#7aa2f7]" />
                <span>Agent Execution Terminal</span>
              </span>
              <span className="text-[10px] text-[#565f89]">WebSocket: Online</span>
            </div>

            <div className="bg-[#101016] border border-[#24283b] p-3 text-[11px] font-mono text-[#a9b1d6] h-80 overflow-y-auto space-y-1">
              {agentLogs.map((log, index) => (
                <div key={index} className="leading-relaxed">
                  {log.includes('error') ? (
                    <span className="text-[#f7768e]">{log}</span>
                  ) : log.includes('successfully') ? (
                    <span className="text-[#9ece6a]">{log}</span>
                  ) : (
                    <span>{log}</span>
                  )}
                </div>
              ))}
            </div>

            <button
              onClick={handleTriggerAgent}
              disabled={agentRunning}
              className="w-full py-2 bg-[#7aa2f7] text-[#16161e] font-bold text-xs hover:bg-[#89b4fa] transition-colors flex items-center justify-center gap-2 disabled:opacity-50"
            >
              <Play className="h-3.5 w-3.5 fill-current" />
              <span>{agentRunning ? 'Agent is working...' : 'Run Agent on Task'}</span>
            </button>
          </div>
        )}
      </div>

      {/* Footer */}
      {activeTab === 'details' && (
        <div className="h-14 border-t border-[#cfd6e6] dark:border-[#24283b] px-6 flex items-center justify-between shrink-0">
          <button
            onClick={() => onDelete(task.id)}
            className="flex items-center gap-1.5 text-xs text-[#f7768e] hover:underline"
          >
            <Trash2 className="h-3.5 w-3.5" />
            <span>Delete</span>
          </button>

          <button
            onClick={handleSave}
            disabled={saving}
            className="flex items-center gap-1.5 px-4 py-2 bg-[#7aa2f7] text-[#16161e] font-semibold text-xs hover:bg-[#89b4fa] transition-colors disabled:opacity-50"
          >
            <Save className="h-3.5 w-3.5" />
            <span>{saving ? 'Saving...' : 'Save Changes'}</span>
          </button>
        </div>
      )}
    </div>
  )
}
