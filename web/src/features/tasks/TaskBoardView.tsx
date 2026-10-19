import { useEffect, useMemo, useState } from 'react'
import { Search } from 'lucide-react'
import { Task, TaskStatus, Project } from '../../api/types'
import { agentsApi } from '../../api/agents'
import { cn } from '../../lib/utils'

interface TaskBoardViewProps {
  tasks: Task[]
  projects: Project[]
  activeProject?: Project
  selectedStatusFilter?: string
  onStatusFilterChange?: (status: string) => void
  onSelectTask: (task: Task) => void
  onUpdateTaskStatus: (taskId: string, status: TaskStatus) => void
  onRunAgent: (taskId: string) => void
}

export const TASK_FILTERS = [
  'All',
  'Todo',
  'Doing',
  'Review',
  'Done',
  'Blocked',
]
export const visibleTaskStatus = (status: string) =>
  status === 'backlog' ? 'todo' : status === 'failed' ? 'blocked' : status

export function TaskBoardView({
  tasks,
  projects,
  activeProject,
  selectedStatusFilter = 'all',
  onStatusFilterChange,
  onSelectTask,
}: TaskBoardViewProps) {
  const [localStatus, setLocalStatus] = useState(selectedStatusFilter)
  const [project, setProject] = useState('all')
  const [query, setQuery] = useState('')
  const [agents, setAgents] = useState<Map<string, string>>(new Map())
  useEffect(() => {
    setLocalStatus(selectedStatusFilter)
  }, [selectedStatusFilter])
  useEffect(() => {
    agentsApi
      .list()
      .then((list) => setAgents(new Map(list.map((a) => [a.id, a.name]))))
      .catch(() => {})
  }, [])
  const status = onStatusFilterChange ? selectedStatusFilter : localStatus
  const projectTasks = useMemo(
    () =>
      tasks.filter((task) => {
        const id = activeProject?.id || project
        return (
          id === 'all' ||
          task.project_id === id ||
          task.project_ids?.includes(id)
        )
      }),
    [tasks, activeProject, project],
  )
  const rows = useMemo(
    () =>
      projectTasks.filter((task) => {
        const matchesStatus =
          status === 'all' ||
          visibleTaskStatus(task.status) === visibleTaskStatus(status)
        const assignee = task.assignee_id || task.assignee || ''
        const text = [
          task.id,
          task.title,
          task.description,
          agents.get(assignee),
          assignee,
        ]
          .join(' ')
          .toLowerCase()
        return matchesStatus && text.includes(query.toLowerCase().trim())
      }),
    [projectTasks, status, query, agents],
  )
  const filtersActive =
    status !== 'all' || !!query.trim() || (!activeProject && project !== 'all')
  const emptyScope = activeProject
    ? projectTasks.length === 0
    : tasks.length === 0
  const clearFilters = () => {
    setQuery('')
    setLocalStatus('all')
    onStatusFilterChange?.('all')
    if (!activeProject) setProject('all')
  }
  return (
    <section className="h-full flex flex-col gap-4">
      <header className="flex flex-wrap items-center justify-between gap-3">
        <div>
          <p className="text-xs text-muted-foreground">{rows.length} tasks</p>
        </div>
      </header>
      <div className="flex flex-wrap gap-3">
        <label className="relative flex-1 min-w-[180px]">
          <Search className="absolute left-3 top-3.5 h-4 w-4 text-muted-foreground" />
          <input
            aria-label="Search tasks"
            placeholder="Search tasks…"
            value={query}
            onChange={(event) => setQuery(event.target.value)}
            className="w-full min-h-11 rounded-md border border-input bg-background py-2.5 pl-9 pr-3 text-sm"
          />
        </label>
        {!activeProject && (
          <select
            aria-label="Filter project"
            value={project}
            onChange={(event) => setProject(event.target.value)}
            className="min-h-11 w-full max-w-full rounded-md border border-input bg-background px-3 text-base sm:w-auto md:text-sm"
          >
            <option value="all">All projects</option>
            {projects.map((p) => (
              <option key={p.id} value={p.id}>
                {p.name}
              </option>
            ))}
          </select>
        )}
      </div>
      <nav
        aria-label="Filter task status"
        className="flex gap-1 overflow-x-auto border-b border-border pb-2 shrink-0"
      >
        {TASK_FILTERS.map((label) => {
          const id = label.toLowerCase()
          const count =
            id === 'all'
              ? projectTasks.length
              : projectTasks.filter((t) => visibleTaskStatus(t.status) === id)
                  .length
          return (
            <button
              key={id}
              aria-pressed={visibleTaskStatus(status) === id}
              onClick={() => {
                setLocalStatus(id)
                onStatusFilterChange?.(id)
              }}
              className={cn(
                'min-h-11 whitespace-nowrap rounded-md px-3 py-2 text-sm',
                visibleTaskStatus(status) === id
                  ? 'bg-accent text-accent-foreground font-semibold'
                  : 'text-muted-foreground hover:bg-muted',
              )}
            >
              {label} <span className="ml-1 opacity-60">{count}</span>
            </button>
          )
        })}
      </nav>
      {filtersActive && (
        <div className="flex items-center justify-between gap-3 text-sm">
          <p className="text-muted-foreground">
            {rows.length} of {projectTasks.length} task
          </p>
          <button className="action-secondary" onClick={clearFilters}>
            Clear filters
          </button>
        </div>
      )}
      <div className="overflow-auto flex-1 rounded-lg border border-border">
        <table className="w-full table-fixed text-left text-sm sm:table-auto">
          <thead className="sticky top-0 bg-card text-xs text-muted-foreground">
            <tr>
              <th className="hidden p-3 sm:table-cell">Code</th>
              <th className="p-3">Title</th>
              <th className="w-24 p-3 sm:w-auto">Status</th>
              <th className="p-3 hidden sm:table-cell">Assignee</th>
              <th className="p-3 hidden md:table-cell">Updated</th>
            </tr>
          </thead>
          <tbody className="divide-y divide-border">
            {rows.map((task) => {
              const assignee = task.assignee_id || task.assignee || ''
              const updated = new Date(task.updated_at || task.created_at)
              const displayStatus = visibleTaskStatus(task.status)
              return (
                <tr key={task.id} className="hover:bg-muted/50">
                  <td className="hidden p-3 font-mono text-xs text-muted-foreground sm:table-cell">
                    #
                    {task.id
                      .replace(/^task_/, '')
                      .slice(0, 6)
                      .toUpperCase()}
                  </td>
                  <td className="p-3">
                    <button
                      onClick={() => onSelectTask(task)}
                      className="block min-h-11 w-full break-words text-left text-base font-medium hover:underline focus:underline sm:text-sm"
                    >
                      <span className="line-clamp-3 sm:line-clamp-none">
                        {task.title}
                      </span>
                    </button>
                    <span className="mt-1 block break-words text-xs text-muted-foreground sm:hidden">
                      #
                      {task.id
                        .replace(/^(task_|msg_)/, '')
                        .slice(0, 6)
                        .toUpperCase()}{' '}
                      · {agents.get(assignee) || assignee || 'Unassigned'}
                    </span>
                  </td>
                  <td className="p-3">
                    <span
                      className={cn(
                        'rounded border border-border px-2 py-1 text-xs capitalize',
                        displayStatus === 'blocked' && 'text-destructive',
                        displayStatus === 'done' && 'text-emerald-600',
                      )}
                    >
                      {displayStatus}
                    </span>
                  </td>
                  <td className="p-3 text-xs hidden sm:table-cell">
                    {agents.get(assignee) || assignee || 'Unassigned'}
                  </td>
                  <td className="p-3 text-xs text-muted-foreground hidden md:table-cell">
                    <time
                      dateTime={
                        Number.isNaN(updated.getTime())
                          ? undefined
                          : updated.toISOString()
                      }
                    >
                      {Number.isNaN(updated.getTime())
                        ? '—'
                        : updated.toLocaleString('en-US')}
                    </time>
                  </td>
                </tr>
              )
            })}
            {rows.length === 0 && (
              <tr>
                <td
                  colSpan={5}
                  className="p-10 text-center text-muted-foreground"
                >
                  <p className="font-medium">
                    {emptyScope
                      ? 'No tasks yet'
                      : 'No tasks match these filters.'}
                  </p>
                  <p className="mt-2 text-sm">
                    {emptyScope
                      ? 'Select New task in the header to create a task.'
                      : 'Try another search or reset the status filter.'}
                  </p>
                </td>
              </tr>
            )}
          </tbody>
        </table>
      </div>
    </section>
  )
}
