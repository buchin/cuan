import { useState } from 'react'
import { Task, TaskStatus, Project } from '../../api/types'
import { Plus, Bot, CheckSquare, Play, ChevronRight } from 'lucide-react'

interface TaskBoardViewProps {
  tasks: Task[]
  projects: Project[]
  activeProject?: Project
  onSelectTask: (task: Task) => void
  onCreateTask: () => void
  onUpdateTaskStatus: (taskId: string, status: TaskStatus) => void
  onRunAgent: (taskId: string) => void
}

const COLUMNS: { id: TaskStatus; label: string; color: string }[] = [
  { id: 'backlog', label: 'Backlog', color: '#565f89' },
  { id: 'todo', label: 'To Do', color: '#7aa2f7' },
  { id: 'doing', label: 'In Progress', color: '#e0af68' },
  { id: 'review', label: 'In Review', color: '#bb9af7' },
  { id: 'done', label: 'Completed', color: '#9ece6a' },
]

export function TaskBoardView({
  tasks,
  projects,
  activeProject,
  onSelectTask,
  onCreateTask,
  onUpdateTaskStatus,
  onRunAgent,
}: TaskBoardViewProps) {
  const [filterProject, setFilterProject] = useState<string>(activeProject ? activeProject.id : 'all')

  const filteredTasks = tasks.filter((t) => {
    if (activeProject) return t.project_id === activeProject.id || t.project_ids?.includes(activeProject.id)
    if (filterProject !== 'all') return t.project_id === filterProject || t.project_ids?.includes(filterProject)
    return true
  })

  const getNextStatus = (current: TaskStatus): TaskStatus | null => {
    if (current === 'backlog') return 'todo'
    if (current === 'todo') return 'doing'
    if (current === 'doing') return 'review'
    if (current === 'review') return 'done'
    return null
  }

  return (
    <div className="h-full flex flex-col space-y-4">
      {/* Board Controls */}
      <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-3 shrink-0">
        <div className="flex items-center gap-3">
          <h2 className="text-base font-bold text-[#1a1b26] dark:text-[#c0caf5] flex items-center gap-2">
            <CheckSquare className="h-4 w-4 text-[#7aa2f7]" />
            <span>Task Board</span>
          </h2>
          {!activeProject && projects.length > 0 && (
            <select
              value={filterProject}
              onChange={(e) => setFilterProject(e.target.value)}
              className="px-2.5 py-1 text-xs bg-[#e2e7f1] dark:bg-[#16161e] border border-[#cfd6e6] dark:border-[#24283b] text-[#1a1b26] dark:text-[#c0caf5] focus:outline-none"
            >
              <option value="all">All Projects</option>
              {projects.map((p) => (
                <option key={p.id} value={p.id}>
                  {p.name}
                </option>
              ))}
            </select>
          )}
          <span className="text-xs text-[#565f89]">
            {filteredTasks.length} {filteredTasks.length === 1 ? 'task' : 'tasks'}
          </span>
        </div>

        <button
          onClick={onCreateTask}
          className="flex items-center gap-1.5 px-3 py-1.5 bg-[#7aa2f7] text-[#16161e] font-semibold text-xs hover:bg-[#89b4fa] transition-colors self-start sm:self-auto"
        >
          <Plus className="h-3.5 w-3.5" />
          <span>New Task</span>
        </button>
      </div>

      {/* Kanban Columns */}
      <div className="flex-1 grid grid-cols-1 md:grid-cols-5 gap-3 overflow-x-auto pb-4">
        {COLUMNS.map((col) => {
          const colTasks = filteredTasks.filter((t) => (t.status || 'todo') === col.id)

          return (
            <div
              key={col.id}
              className="flex flex-col bg-[#e2e7f1]/60 dark:bg-[#16161e]/40 border border-[#cfd6e6] dark:border-[#24283b] min-w-[220px]"
            >
              {/* Column Header */}
              <div className="p-3 border-b border-[#cfd6e6] dark:border-[#24283b] flex items-center justify-between">
                <div className="flex items-center gap-2">
                  <div className="h-2 w-2 rounded-none" style={{ backgroundColor: col.color }} />
                  <span className="text-xs font-bold text-[#1a1b26] dark:text-[#c0caf5]">
                    {col.label}
                  </span>
                </div>
                <span className="text-[10px] font-mono px-1.5 py-0.2 bg-[#cfd6e6] dark:bg-[#24283b] text-[#565f89]">
                  {colTasks.length}
                </span>
              </div>

              {/* Task Cards List */}
              <div className="flex-1 p-2 space-y-2 overflow-y-auto max-h-[calc(100vh-220px)]">
                {colTasks.length === 0 ? (
                  <div className="p-4 text-center text-[11px] text-[#565f89]/60 italic">
                    No tasks
                  </div>
                ) : (
                  colTasks.map((task) => {
                    const next = getNextStatus(task.status)

                    return (
                      <div
                        key={task.id}
                        onClick={() => onSelectTask(task)}
                        className="group p-3 bg-[#f0f3f8] dark:bg-[#1f2335] border border-[#cfd6e6] dark:border-[#292e42] hover:border-[#7aa2f7] dark:hover:border-[#7aa2f7] cursor-pointer transition-all space-y-2"
                      >
                        <div className="flex items-start justify-between gap-1">
                          <span
                            className={`text-[9px] uppercase font-mono px-1.5 py-0.2 ${
                              task.priority === 'urgent'
                                ? 'bg-[#f7768e]/20 text-[#f7768e]'
                                : task.priority === 'high'
                                ? 'bg-[#ff9e64]/20 text-[#ff9e64]'
                                : 'bg-[#7aa2f7]/10 text-[#7aa2f7]'
                            }`}
                          >
                            {task.priority || 'normal'}
                          </span>

                          <div className="flex items-center gap-1">
                            <button
                              onClick={(e) => {
                                e.stopPropagation()
                                onRunAgent(task.id)
                              }}
                              className="opacity-0 group-hover:opacity-100 p-1 bg-[#7aa2f7]/20 text-[#7aa2f7] hover:bg-[#7aa2f7] hover:text-[#16161e] transition-all"
                              title="Run AI Agent on this task"
                            >
                              <Play className="h-3 w-3" />
                            </button>
                            {next && (
                              <button
                                onClick={(e) => {
                                  e.stopPropagation()
                                  onUpdateTaskStatus(task.id, next)
                                }}
                                className="opacity-0 group-hover:opacity-100 p-1 bg-[#cfd6e6] dark:bg-[#24283b] text-[#565f89] hover:text-[#7aa2f7] transition-all"
                                title={`Move to ${next}`}
                              >
                                <ChevronRight className="h-3 w-3" />
                              </button>
                            )}
                          </div>
                        </div>

                        <h4 className="text-xs font-semibold text-[#1a1b26] dark:text-[#c0caf5] line-clamp-2">
                          {task.title}
                        </h4>

                        {task.description && (
                          <p className="text-[11px] text-[#565f89] line-clamp-2">
                            {task.description}
                          </p>
                        )}

                        <div className="pt-2 border-t border-[#cfd6e6]/50 dark:border-[#24283b] flex items-center justify-between text-[10px] text-[#565f89]">
                          <span className="truncate">
                            {new Date(task.created_at).toLocaleDateString()}
                          </span>
                          {task.assignee && (
                            <span className="flex items-center gap-1 font-mono text-[#7aa2f7]">
                              <Bot className="h-3 w-3" />
                              <span className="truncate max-w-[80px]">{task.assignee}</span>
                            </span>
                          )}
                        </div>
                      </div>
                    )
                  })
                )}
              </div>
            </div>
          )
        })}
      </div>
    </div>
  )
}
