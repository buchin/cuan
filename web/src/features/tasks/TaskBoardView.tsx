import { useState, useMemo, useEffect } from 'react'
import { Task, TaskStatus, Project } from '../../api/types'
import { Plus, Bot, CheckSquare, Play, ChevronRight, Search, X, FolderGit2 } from 'lucide-react'
import { cn } from '../../lib/utils'
import { agentsApi } from '../../api/agents'

interface TaskBoardViewProps {
  tasks: Task[]
  projects: Project[]
  activeProject?: Project
  selectedStatusFilter?: string
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
  selectedStatusFilter = 'all',
  onSelectTask,
  onCreateTask,
  onUpdateTaskStatus,
  onRunAgent,
}: TaskBoardViewProps) {
  const [filterProject, setFilterProject] = useState<string>(activeProject ? activeProject.id : 'all')
  const [statusFilter, setStatusFilter] = useState<string>(selectedStatusFilter)
  const [searchQuery, setSearchQuery] = useState('')

  const [agentsMap, setAgentsMap] = useState<Map<string, { name: string; harness?: string }>>(new Map())

  useEffect(() => {
    agentsApi.list().then((list) => {
      if (list && list.length > 0) {
        const m = new Map<string, { name: string; harness?: string }>()
        for (const ag of list) {
          m.set(ag.id, { name: ag.name, harness: ag.harness || ag.provider || 'agy' })
        }
        setAgentsMap(m)
      }
    }).catch(() => {})
  }, [])

  // Map project ID to Project Name for fast lookup
  const projectMap = useMemo(() => {
    const map = new Map<string, string>()
    for (const p of projects) {
      map.set(p.id, p.name)
    }
    return map
  }, [projects])

  const filteredTasks = useMemo(() => {
    return tasks.filter((t) => {
      // Project filter
      if (activeProject) {
        const matchesActive = t.project_id === activeProject.id || t.project_ids?.includes(activeProject.id)
        if (!matchesActive) return false
      } else if (filterProject !== 'all') {
        const matchesFilter = t.project_id === filterProject || t.project_ids?.includes(filterProject)
        if (!matchesFilter) return false
      }

      // Status filter
      if (statusFilter !== 'all') {
        const taskStatus = t.status || 'todo'
        if (taskStatus !== statusFilter) return false
      }

      // Search query filter
      if (searchQuery.trim() !== '') {
        const q = searchQuery.toLowerCase().trim()
        const matchTitle = t.title.toLowerCase().includes(q)
        const matchDesc = t.description ? t.description.toLowerCase().includes(q) : false
        const matchAssignee = t.assignee ? t.assignee.toLowerCase().includes(q) : false
        if (!matchTitle && !matchDesc && !matchAssignee) return false
      }

      return true
    })
  }, [tasks, activeProject, filterProject, statusFilter, searchQuery])

  const getNextStatus = (current: TaskStatus): TaskStatus | null => {
    if (current === 'backlog') return 'todo'
    if (current === 'todo') return 'doing'
    if (current === 'doing') return 'review'
    if (current === 'review') return 'done'
    return null
  }

  // Active columns to display
  const displayColumns = useMemo(() => {
    if (statusFilter === 'all') return COLUMNS
    return COLUMNS.filter((c) => c.id === statusFilter)
  }, [statusFilter])

  return (
    <div className="h-full flex flex-col space-y-4">
      {/* Board Controls: Header, Filters, Search & New Task */}
      <div className="flex flex-col lg:flex-row lg:items-center justify-between gap-3 shrink-0 pb-1 border-b border-border/60">
        <div className="flex flex-wrap items-center gap-3">
          <h2 className="text-base font-bold text-foreground flex items-center gap-2">
            <CheckSquare className="h-4 w-4 text-primary" />
            <span>{activeProject ? activeProject.name : 'Global Task Board'}</span>
          </h2>

          {!activeProject && projects.length > 0 && (
            <select
              value={filterProject}
              onChange={(e) => setFilterProject(e.target.value)}
              className="px-2.5 py-1 text-xs bg-background border border-input text-foreground focus:outline-none rounded-md"
            >
              <option value="all">Semua Proyek ({projects.length})</option>
              {projects.map((p) => (
                <option key={p.id} value={p.id}>
                  {p.name}
                </option>
              ))}
            </select>
          )}

          <span className="text-xs font-mono px-2 py-0.5 rounded bg-muted text-muted-foreground">
            {filteredTasks.length} {filteredTasks.length === 1 ? 'task' : 'tasks'}
          </span>
        </div>

        <div className="flex flex-wrap items-center gap-2">
          {/* Quick Search */}
          <div className="relative min-w-[180px] sm:min-w-[220px]">
            <Search className="h-3.5 w-3.5 absolute left-2.5 top-1/2 -translate-y-1/2 text-muted-foreground" />
            <input
              type="text"
              placeholder="Cari task..."
              value={searchQuery}
              onChange={(e) => setSearchQuery(e.target.value)}
              className="w-full pl-8 pr-7 py-1 text-xs bg-background border border-input rounded-md text-foreground placeholder:text-muted-foreground focus:outline-none focus:ring-1 focus:ring-primary"
            />
            {searchQuery && (
              <button
                onClick={() => setSearchQuery('')}
                className="absolute right-2 top-1/2 -translate-y-1/2 text-muted-foreground hover:text-foreground"
              >
                <X className="h-3 w-3" />
              </button>
            )}
          </div>

          {/* Status Quick Pills */}
          <div className="hidden sm:flex items-center gap-1 bg-muted/50 p-0.5 rounded-md border border-border">
            {[{ id: 'all', label: 'All' }, ...COLUMNS].map((col) => (
              <button
                key={col.id}
                onClick={() => setStatusFilter(col.id)}
                className={cn(
                  'px-2 py-0.5 text-[11px] font-medium rounded transition-colors',
                  statusFilter === col.id
                    ? 'bg-background text-foreground shadow-xs font-bold'
                    : 'text-muted-foreground hover:text-foreground'
                )}
              >
                {col.label}
              </button>
            ))}
          </div>

          {/* New Task Button */}
          <button
            onClick={onCreateTask}
            className="flex items-center gap-1.5 px-3 py-1.5 bg-primary text-primary-foreground font-semibold text-xs hover:bg-primary/90 transition-colors rounded-md shadow-xs shrink-0"
          >
            <Plus className="h-3.5 w-3.5" />
            <span>+ Task</span>
          </button>
        </div>
      </div>

      {/* Kanban Columns */}
      <div className={cn(
        'flex-1 grid gap-3 overflow-x-auto pb-4',
        displayColumns.length === 1 ? 'grid-cols-1 max-w-2xl' : 'grid-cols-1 md:grid-cols-3 lg:grid-cols-5'
      )}>
        {displayColumns.map((col) => {
          const colTasks = filteredTasks.filter((t) => (t.status || 'todo') === col.id)

          return (
            <div
              key={col.id}
              className="flex flex-col bg-muted/40 border border-border min-w-[240px] rounded-lg shadow-xs"
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
                    Belum ada task
                  </div>
                ) : (
                  colTasks.map((task) => {
                    const next = getNextStatus(task.status)
                    const projectName = task.project_id ? projectMap.get(task.project_id) : undefined

                    return (
                      <div
                        key={task.id}
                        onClick={() => onSelectTask(task)}
                        className="group p-3 bg-card border border-border hover:border-primary/50 cursor-pointer transition-all space-y-2 rounded-md shadow-xs hover:shadow-sm"
                      >
                        <div className="flex items-start justify-between gap-1">
                          <div className="flex items-center gap-1.5 flex-wrap">
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

                            {/* Project Badge if in Global View */}
                            {projectName && !activeProject && (
                              <span className="flex items-center gap-1 text-[9px] font-mono px-1.5 py-0.5 rounded bg-muted text-muted-foreground border border-border max-w-[120px] truncate">
                                <FolderGit2 className="h-2.5 w-2.5 shrink-0" />
                                <span className="truncate">{projectName}</span>
                              </span>
                            )}

                            {/* S22.4 Running Indicator Badge */}
                            {task.execution_status === 'running' && (
                              <span className="flex items-center gap-1 text-[9px] font-mono px-1.5 py-0.5 rounded bg-emerald-500/10 text-emerald-600 dark:text-emerald-400 font-bold border border-emerald-500/20">
                                <span className="relative flex h-1.5 w-1.5">
                                  <span className="animate-ping absolute inline-flex h-full w-full rounded-full bg-emerald-400 opacity-75"></span>
                                  <span className="relative inline-flex rounded-full h-1.5 w-1.5 bg-emerald-500"></span>
                                </span>
                                <span>Running</span>
                              </span>
                            )}
                          </div>

                          <div className="flex items-center gap-1 shrink-0">
                            <button
                              onClick={(e) => {
                                e.stopPropagation()
                                onRunAgent(task.id)
                              }}
                              className="opacity-0 group-hover:opacity-100 p-1 bg-primary/10 text-primary hover:bg-primary hover:text-primary-foreground transition-all rounded"
                              title="Jalankan Worker Agent pada task ini"
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
                                title={`Pindahkan ke ${next}`}
                              >
                                <ChevronRight className="h-3 w-3" />
                              </button>
                            )}
                          </div>
                        </div>

                        <h4 className="text-xs font-semibold text-foreground line-clamp-2 leading-tight">
                          {task.title}
                        </h4>

                        {task.description && (
                          <p className="text-[11px] text-muted-foreground line-clamp-2 leading-relaxed">
                            {task.description}
                          </p>
                        )}

                        <div className="pt-2 border-t border-border/50 flex items-center justify-between text-[10px] text-muted-foreground">
                          <span className="truncate font-mono">
                            {new Date(task.created_at).toLocaleDateString()}
                          </span>
                          <div className="flex items-center gap-2 shrink-0">
                            {task.dod_checklist && task.dod_checklist.length > 0 && (
                              <span className="flex items-center gap-1 font-mono text-[10px] text-emerald-600 dark:text-emerald-400 font-semibold" title="Definition of Done">
                                <CheckSquare className="h-3 w-3" />
                                <span>
                                  {task.dod_checklist.filter((d: any) => typeof d === 'object' && d.completed).length}/{task.dod_checklist.length}
                                </span>
                              </span>
                            )}
                            {(task.assignee || task.assignee_id) && (() => {
                              const rawAssignee = task.assignee || task.assignee_id || ''
                              const agentInfo = agentsMap.get(rawAssignee)
                              const displayName = agentInfo ? `${agentInfo.name} (${agentInfo.harness || 'agy'})` : rawAssignee

                              return (
                                <span
                                  className={cn(
                                    'flex items-center gap-1 font-mono text-[10px] px-1.5 py-0.5 rounded',
                                    task.execution_status === 'running'
                                      ? 'bg-emerald-500/10 text-emerald-600 dark:text-emerald-400 font-bold border border-emerald-500/20'
                                      : task.execution_status === 'completed'
                                      ? 'bg-emerald-500/10 text-emerald-600 dark:text-emerald-400'
                                      : task.execution_status === 'failed'
                                      ? 'bg-destructive/10 text-destructive'
                                      : 'bg-primary/10 text-primary'
                                  )}
                                  title={`Assigned: ${displayName} (${task.execution_status || 'idle'})`}
                                >
                                  <Bot className="h-3 w-3 shrink-0" />
                                  <span className="truncate max-w-[110px]">{displayName}</span>
                                  {task.execution_status === 'running' ? (
                                    <span className="relative flex h-1.5 w-1.5 ml-0.5">
                                      <span className="animate-ping absolute inline-flex h-full w-full rounded-full bg-emerald-400 opacity-75"></span>
                                      <span className="relative inline-flex rounded-full h-1.5 w-1.5 bg-emerald-500"></span>
                                    </span>
                                  ) : task.execution_status === 'completed' ? (
                                    <span className="h-1.5 w-1.5 rounded-full bg-emerald-500 ml-0.5" />
                                  ) : task.execution_status === 'failed' ? (
                                    <span className="h-1.5 w-1.5 rounded-full bg-destructive ml-0.5" />
                                  ) : null}
                                </span>
                              )
                            })()}
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
