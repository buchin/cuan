import { Plus, FolderGit2 } from 'lucide-react'
import { Task, Project } from '../../api/types'
import { cn } from '../../lib/utils'

interface TasksSubPanelProps {
  tasks: Task[]
  projects: Project[]
  selectedStatus: string
  onSelectStatus: (status: string) => void
  selectedProject: string
  onSelectProject: (projectId: string) => void
  onCreateTaskClick?: () => void
}

export function TasksSubPanel({
  tasks,
  projects,
  selectedStatus,
  onSelectStatus,
  selectedProject,
  onSelectProject,
  onCreateTaskClick,
}: TasksSubPanelProps) {
  const activeProjects = projects.filter((p) => !p.is_archived)

  const STATUSES: { id: string; label: string; dotClass: string }[] = [
    { id: 'all', label: 'All Tasks', dotClass: 'bg-primary' },
    { id: 'todo', label: 'Todo', dotClass: 'bg-blue-500' },
    { id: 'doing', label: 'Doing', dotClass: 'bg-yellow-500' },
    { id: 'review', label: 'Review', dotClass: 'bg-purple-500' },
    { id: 'done', label: 'Done', dotClass: 'bg-green-500' },
    { id: 'blocked', label: 'Blocked', dotClass: 'bg-destructive' },
  ]

  const getStatusCount = (statusId: string) => {
    let pool = tasks
    if (selectedProject !== 'all') {
      pool = pool.filter(
        (t) =>
          t.project_id === selectedProject ||
          t.project_ids?.includes(selectedProject),
      )
    }
    if (statusId === 'all') return pool.length
    return pool.filter(
      (t) =>
        (t.status === 'backlog' ? 'todo' : t.status || 'todo') === statusId,
    ).length
  }

  const getProjectCount = (projId: string) => {
    if (projId === 'all') return tasks.length
    return tasks.filter(
      (t) => t.project_id === projId || t.project_ids?.includes(projId),
    ).length
  }

  return (
    <div className="flex flex-col h-full bg-sidebar/50">
      {/* Create Task Button */}
      {onCreateTaskClick && (
        <div className="p-3 border-b border-border">
          <button
            onClick={onCreateTaskClick}
            className="flex items-center justify-center gap-2 w-full px-3 py-2 text-xs font-semibold bg-primary text-primary-foreground hover:bg-primary/90 transition-colors rounded-md shadow-xs"
          >
            <Plus className="h-3.5 w-3.5" />
            <span>New Task</span>
          </button>
        </div>
      )}

      <div className="flex-1 overflow-y-auto py-2 space-y-4">
        {/* Status Filters */}
        <div className="px-2 space-y-1">
          <div className="px-2 py-1 text-[10px] font-mono uppercase tracking-wider text-muted-foreground font-semibold">
            Status Backlog
          </div>
          {STATUSES.map((s) => (
            <button
              key={s.id}
              onClick={() => onSelectStatus(s.id)}
              className={cn(
                'w-full px-2.5 py-1.5 text-xs text-left rounded-md flex items-center justify-between transition-colors',
                selectedStatus === s.id
                  ? 'bg-accent text-accent-foreground font-semibold'
                  : 'text-muted-foreground hover:text-foreground hover:bg-accent/40',
              )}
            >
              <div className="flex items-center gap-2">
                <span className={cn('h-2 w-2 rounded-full', s.dotClass)} />
                <span>{s.label}</span>
              </div>
              <span className="text-[10px] font-mono opacity-70">
                {getStatusCount(s.id)}
              </span>
            </button>
          ))}
        </div>

        {/* Project Filters */}
        {activeProjects.length > 0 && (
          <div className="px-2 space-y-1 pt-2 border-t border-border/50">
            <div className="px-2 py-1 text-[10px] font-mono uppercase tracking-wider text-muted-foreground font-semibold flex items-center justify-between">
              <span>Filter Project</span>
            </div>
            <button
              onClick={() => onSelectProject('all')}
              className={cn(
                'w-full px-2.5 py-1.5 text-xs text-left rounded-md flex items-center justify-between transition-colors',
                selectedProject === 'all'
                  ? 'bg-accent text-accent-foreground font-semibold'
                  : 'text-muted-foreground hover:text-foreground hover:bg-accent/40',
              )}
            >
              <span className="truncate">All projects</span>
              <span className="text-[10px] font-mono opacity-70">
                {getProjectCount('all')}
              </span>
            </button>
            {activeProjects.map((p) => (
              <button
                key={p.id}
                onClick={() => onSelectProject(p.id)}
                className={cn(
                  'w-full px-2.5 py-1.5 text-xs text-left rounded-md flex items-center justify-between transition-colors',
                  selectedProject === p.id
                    ? 'bg-accent text-accent-foreground font-semibold'
                    : 'text-muted-foreground hover:text-foreground hover:bg-accent/40',
                )}
              >
                <div className="flex items-center gap-1.5 truncate pr-2">
                  <FolderGit2 className="h-3 w-3 text-muted-foreground shrink-0" />
                  <span className="truncate">{p.name}</span>
                </div>
                <span className="text-[10px] font-mono opacity-70">
                  {getProjectCount(p.id)}
                </span>
              </button>
            ))}
          </div>
        )}
      </div>
    </div>
  )
}
