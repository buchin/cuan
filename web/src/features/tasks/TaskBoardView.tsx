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

const COLUMNS: { id: TaskStatus; label: string; dotClass: string }[] = [
  { id: 'backlog', label: 'Backlog', dotClass: 'bg-muted-foreground' },
  { id: 'todo', label: 'To Do', dotClass: 'bg-blue-500' },
  { id: 'doing', label: 'In Progress', dotClass: 'bg-yellow-500' },
  { id: 'review', label: 'In Review', dotClass: 'bg-purple-500' },
  { id: 'done', label: 'Completed', dotClass: 'bg-green-500' },
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
          <h2 className="text-base font-bold text-foreground flex items-center gap-2">
            <CheckSquare className="h-4 w-4 text-primary" />
            <span>Task Board</span>
          </h2>
          {!activeProject && projects.length > 0 && (
            <select
              value={filterProject}
              onChange={(e) => setFilterProject(e.target.value)}
              className="px-2.5 py-1 text-xs bg-background border border-input text-foreground focus:outline-none rounded-md"
            >
              <option value="all">All Projects</option>
              {projects.map((p) => (
                <option key={p.id} value={p.id}>
                  {p.name}
                </option>
              ))}
            </select>
          )}
          <span className="text-xs text-muted-foreground">
            {filteredTasks.length} {filteredTasks.length === 1 ? 'task' : 'tasks'}
          </span>
        </div>

        <button
          onClick={onCreateTask}
          className="flex items-center gap-1.5 px-3 py-1.5 bg-primary text-primary-foreground font-semibold text-xs hover:bg-primary/90 transition-colors rounded-md self-start sm:self-auto"
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
              className="flex flex-col bg-muted/40 border border-border min-w-[220px] rounded-lg"
            >
              {/* Column Header */}
              <div className="p-3 border-b border-border flex items-center justify-between">
                <div className="flex items-center gap-2">
                  <div className={`h-2 w-2 rounded-full ${col.dotClass}`} />
                  <span className="text-xs font-semibold text-foreground">
                    {col.label}
                  </span>
                </div>
                <span className="text-[10px] font-mono px-1.5 py-0.5 bg-background border border-border text-muted-foreground rounded">
                  {colTasks.length}
                </span>
              </div>

              {/* Task Cards List */}
              <div className="flex-1 p-2 space-y-2 overflow-y-auto max-h-[calc(100vh-220px)]">
                {colTasks.length === 0 ? (
                  <div className="p-4 text-center text-[11px] text-muted-foreground/60 italic">
                    No tasks
                  </div>
                ) : (
                  colTasks.map((task) => {
                    const next = getNextStatus(task.status)

                    return (
                      <div
                        key={task.id}
                        onClick={() => onSelectTask(task)}
                        className="group p-3 bg-card border border-border hover:border-primary/50 cursor-pointer transition-all space-y-2 rounded-md shadow-sm"
                      >
                        <div className="flex items-start justify-between gap-1">
                          <span
                            className={`text-[9px] uppercase font-mono px-1.5 py-0.5 rounded ${
                              task.priority === 'urgent'
                                ? 'bg-destructive/20 text-destructive'
                                : task.priority === 'high'
                                ? 'bg-orange-500/20 text-orange-600'
                                : 'bg-primary/10 text-primary'
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
                              className="opacity-0 group-hover:opacity-100 p-1 bg-primary/10 text-primary hover:bg-primary hover:text-primary-foreground transition-all rounded"
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
                                className="opacity-0 group-hover:opacity-100 p-1 bg-muted text-muted-foreground hover:text-primary transition-all rounded"
                                title={`Move to ${next}`}
                              >
                                <ChevronRight className="h-3 w-3" />
                              </button>
                            )}
                          </div>
                        </div>

                        <h4 className="text-xs font-semibold text-foreground line-clamp-2">
                          {task.title}
                        </h4>

                        {task.description && (
                          <p className="text-[11px] text-muted-foreground line-clamp-2">
                            {task.description}
                          </p>
                        )}

                        <div className="pt-2 border-t border-border/50 flex items-center justify-between text-[10px] text-muted-foreground">
                          <span className="truncate">
                            {new Date(task.created_at).toLocaleDateString()}
                          </span>
                          <div className="flex items-center gap-2">
                            {task.dod_checklist && task.dod_checklist.length > 0 && (
                              <span className="flex items-center gap-1 font-mono text-[10px] text-green-600" title="Definition of Done">
                                <CheckSquare className="h-3 w-3" />
                                <span>
                                  {task.dod_checklist.filter((d: any) => typeof d === 'object' && d.completed).length}/{task.dod_checklist.length}
                                </span>
                              </span>
                            )}
                            {task.assignee && (
                              <span className="flex items-center gap-1 font-mono text-primary">
                                <Bot className="h-3 w-3" />
                                <span className="truncate max-w-[80px]">{task.assignee}</span>
                              </span>
                            )}
                          </div>
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
