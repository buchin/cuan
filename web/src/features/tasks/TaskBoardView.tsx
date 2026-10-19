import { useEffect, useMemo, useState } from 'react'
import { useSearchParams } from 'react-router-dom'
import {
  Search,
  SlidersHorizontal,
  ArrowUpDown,
  Circle,
  LoaderCircle,
} from 'lucide-react'
import type { Task, TaskStatus, Project } from '../../api/types'
import { agentsApi, type Agent } from '../../api/agents'
import { ActionDialog } from '../../components/dialogs/ActionDialog'
import {
  TASK_STATUSES,
  taskStatus,
  statusLabel,
  taskRef,
  executionLabel,
  relativeTime,
} from './task-ui'
import { cn } from '../../lib/utils'
interface Props {
  loading?: boolean
  error?: string
  onRetry?: () => void
  tasks: Task[]
  projects: Project[]
  activeProject?: Project
  selectedStatusFilter?: string
  onStatusFilterChange?: (status: string) => void
  onSelectTask: (task: Task) => void
  onUpdateTaskStatus: (id: string, status: TaskStatus) => void
  onRunAgent: (id: string) => void
}
export const TASK_FILTERS = ['All', ...TASK_STATUSES.map((s) => s.label)]
export const visibleTaskStatus = taskStatus
export function TaskBoardView({
  tasks,
  projects,
  activeProject,
  onSelectTask,
  onStatusFilterChange,
  loading = false,
  error = '',
  onRetry,
}: Props) {
  const [params, setParams] = useSearchParams(),
    [filtersOpen, setFiltersOpen] = useState(false),
    [agents, setAgents] = useState<Agent[]>([]),
    [limit, setLimit] = useState(100)
  const query = params.get('q') || '',
    status = params.get('status') || 'all',
    project = activeProject?.id || params.get('project') || 'all',
    priority = params.get('priority') || 'all',
    assignee = params.get('assignee') || 'all',
    sort = params.get('sort') || 'updated'
  const update = (key: string, value: string) =>
    setParams(
      (old) => {
        const next = new URLSearchParams(old)
        if (
          !value ||
          value === 'all' ||
          (key === 'sort' && value === 'updated')
        )
          next.delete(key)
        else next.set(key, value)
        return next
      },
      { replace: true },
    )
  useEffect(() => {
    let alive = true
    agentsApi
      .list()
      .then((list) => {
        if (alive) setAgents(list)
      })
      .catch(() => {})
    return () => {
      alive = false
    }
  }, [])
  useEffect(
    () => setLimit(100),
    [query, status, project, priority, assignee, sort],
  )
  const names = new Map(agents.map((a) => [a.id, a.name]))
  const name = (t: Task) =>
    names.get(t.assignee_id || t.assignee || '') ||
    t.assignee_id ||
    t.assignee ||
    'Unassigned'
  const projectName = (t: Task) =>
    t.project_name ||
    projects.find((p) => p.id === t.project_id || t.project_ids?.includes(p.id))
      ?.name ||
    'No project'
  const scoped = useMemo(
    () =>
      tasks.filter(
        (t) =>
          project === 'all' ||
          (project === 'none'
            ? !t.project_id && !t.project_ids?.length
            : t.project_id === project || t.project_ids?.includes(project)),
      ),
    [tasks, project],
  )
  const rows = scoped
    .filter(
      (t) =>
        (status === 'all' || taskStatus(t.status) === status) &&
        (priority === 'all' || t.priority === priority) &&
        (assignee === 'all' ||
          (assignee === 'unassigned'
            ? !t.assignee_id && !t.assignee
            : (t.assignee_id || t.assignee) === assignee)) &&
        [t.id, taskRef(t.id), t.title, t.description, name(t), projectName(t)]
          .join(' ')
          .toLowerCase()
          .includes(query.trim().toLowerCase()),
    )
    .sort((a, b) => {
      if (sort === 'priority') {
        const ranks = { urgent: 0, high: 1, normal: 2, low: 3 }
        const diff = ranks[a.priority] - ranks[b.priority]
        if (diff) return diff
      }
      if (sort === 'title') return a.title.localeCompare(b.title)
      const key = sort === 'created' ? 'created_at' : 'updated_at'
      return (
        (Date.parse(b[key]) || 0) - (Date.parse(a[key]) || 0) ||
        a.id.localeCompare(b.id)
      )
    })
  const filterCount =
    Number(project !== 'all' && !activeProject) +
    Number(priority !== 'all') +
    Number(assignee !== 'all')
  const filtered = !!query || status !== 'all' || filterCount > 0
  const clear = () => {
    setParams(
      (old) => {
        const next = new URLSearchParams(old)
        for (const key of ['q', 'status', 'priority', 'assignee', 'project'])
          next.delete(key)
        return next
      },
      { replace: true },
    )
    onStatusFilterChange?.('all')
  }
  const fields = (
    <>
      {!activeProject && (
        <label className="grid gap-1 text-sm">
          Project
          <select
            aria-label="Filter project"
            className="field-control"
            value={project}
            onChange={(e) => update('project', e.target.value)}
          >
            <option value="all">All projects</option>
            <option value="none">No project</option>
            {projects.map((p) => (
              <option key={p.id} value={p.id}>
                {p.name}
                {p.is_archived ? ' (archived)' : ''}
              </option>
            ))}
          </select>
        </label>
      )}
      <label className="grid gap-1 text-sm">
        Priority
        <select
          aria-label="Filter priority"
          className="field-control"
          value={priority}
          onChange={(e) => update('priority', e.target.value)}
        >
          <option value="all">All priorities</option>
          {['urgent', 'high', 'normal', 'low'].map((p) => (
            <option key={p} value={p}>
              {p[0].toUpperCase() + p.slice(1)}
            </option>
          ))}
        </select>
      </label>
      <label className="grid gap-1 text-sm">
        Assignee
        <select
          aria-label="Filter assignee"
          className="field-control"
          value={assignee}
          onChange={(e) => update('assignee', e.target.value)}
        >
          <option value="all">All assignees</option>
          <option value="unassigned">Unassigned</option>
          {agents.map((a) => (
            <option key={a.id} value={a.id}>
              {a.name}
            </option>
          ))}
        </select>
      </label>
    </>
  )
  return (
    <section
      className="flex h-full min-h-0 flex-col gap-3"
      aria-label="Task list"
    >
      <div className="flex items-center gap-2">
        <label className="relative min-w-0 flex-1">
          <Search
            aria-hidden
            className="absolute left-3 top-3.5 h-4 w-4 text-muted-foreground"
          />
          <input
            aria-label="Search tasks"
            placeholder="Search tasks…"
            value={query}
            onChange={(e) => update('q', e.target.value)}
            className="field-control pl-9"
          />
        </label>
        <button
          className="action-secondary px-3"
          onClick={() => setFiltersOpen(true)}
          aria-label={
            filterCount ? `Filters (${filterCount} active)` : 'Filters'
          }
        >
          <SlidersHorizontal className="h-4 w-4" />
          <span className="hidden sm:inline">Filters</span>
          {filterCount > 0 && (
            <span className="rounded bg-accent px-1.5 text-xs">
              {filterCount}
            </span>
          )}
        </button>
        <label className="relative">
          <ArrowUpDown
            aria-hidden
            className="pointer-events-none absolute left-2 top-3.5 h-4 w-4 text-muted-foreground"
          />
          <select
            aria-label="Sort tasks"
            value={sort}
            onChange={(e) => update('sort', e.target.value)}
            className="min-h-11 max-w-[118px] rounded-md border border-input bg-background pl-7 pr-1 text-sm sm:max-w-none"
          >
            <option value="updated">Updated</option>
            <option value="created">Newest</option>
            <option value="priority">Priority</option>
            <option value="title">Title</option>
          </select>
        </label>
      </div>
      <nav
        aria-label="Filter task status"
        className="flex shrink-0 gap-1 overflow-x-auto border-b pb-1"
      >
        {[{ id: 'all', label: 'All' }, ...TASK_STATUSES].map((item) => (
          <button
            key={item.id}
            aria-pressed={status === item.id}
            onClick={() => {
              update('status', item.id)
              onStatusFilterChange?.(item.id)
            }}
            className={cn(
              'min-h-11 shrink-0 whitespace-nowrap rounded-md px-3 text-sm',
              status === item.id
                ? 'bg-accent font-semibold'
                : 'text-muted-foreground hover:bg-muted',
            )}
          >
            {item.label}
            <span className="ml-1.5 text-xs opacity-60">
              {item.id === 'all'
                ? scoped.length
                : scoped.filter((t) => taskStatus(t.status) === item.id).length}
            </span>
          </button>
        ))}
      </nav>
      <div className="flex min-h-6 items-center justify-between gap-2 text-xs text-muted-foreground">
        <span role="status">
          {rows.length} {rows.length === 1 ? 'task' : 'tasks'}
          {filtered ? ` of ${scoped.length}` : ''}
        </span>
        {filtered && (
          <button
            onClick={clear}
            className="min-h-8 px-1 text-sm underline underline-offset-2"
          >
            Clear filters
          </button>
        )}
      </div>
      {error && (
        <div
          role="alert"
          className="rounded-lg border border-destructive/40 p-3 text-sm text-destructive"
        >
          {error}
          <button className="action-secondary ml-2" onClick={onRetry}>
            Try again
          </button>
        </div>
      )}
      {loading ? (
        <p role="status" className="p-4 text-sm text-muted-foreground">
          Loading tasks…
        </p>
      ) : rows.length === 0 ? (
        <div className="flex flex-1 flex-col items-center justify-center gap-2 rounded-lg border border-dashed p-6 text-center">
          <Circle className="h-7 w-7 text-muted-foreground" />
          <h2 className="font-medium">
            {tasks.length === 0 ? 'No tasks yet' : 'No matching tasks'}
          </h2>
          <p className="max-w-sm text-sm text-muted-foreground">
            {tasks.length === 0
              ? 'Select New task to add work to your backlog.'
              : 'Try another search or clear your filters.'}
          </p>
          {filtered && (
            <button className="action-secondary mt-2" onClick={clear}>
              Clear filters
            </button>
          )}
        </div>
      ) : (
        <div className="min-h-0 flex-1 overflow-auto rounded-lg border">
          <table className="hidden w-full table-fixed text-left text-sm md:table">
            <thead className="sticky top-0 z-10 bg-card text-xs text-muted-foreground">
              <tr>
                <th className="w-[42%] px-4 py-3">Task</th>
                <th className="px-3 py-3">Status</th>
                <th className="px-3 py-3">Priority</th>
                <th className="px-3 py-3">Assignee</th>
                <th className="px-3 py-3">Updated</th>
              </tr>
            </thead>
            <tbody className="divide-y">
              {rows.slice(0, limit).map((t) => (
                <tr
                  key={t.id}
                  className="cursor-pointer hover:bg-muted/50"
                  onClick={() => onSelectTask(t)}
                >
                  <td className="px-4 py-3">
                    <button
                      className="min-h-11 w-full text-left font-medium hover:underline"
                      onClick={(e) => {
                        e.stopPropagation()
                        onSelectTask(t)
                      }}
                    >
                      <span className="line-clamp-2 break-words">
                        {t.title}
                      </span>
                    </button>
                    <div className="mt-1 flex min-w-0 items-center gap-2 text-xs text-muted-foreground">
                      <span className="shrink-0 font-mono">
                        {taskRef(t.id)}
                      </span>
                      {!activeProject && (
                        <>
                          <span>·</span>
                          <span className="truncate">{projectName(t)}</span>
                        </>
                      )}
                    </div>
                  </td>
                  <td className="px-3 py-3">
                    <span className="rounded border px-2 py-1 text-xs">
                      {statusLabel(t.status)}
                    </span>
                    {executionLabel(t) && (
                      <span className="mt-2 flex items-center gap-1 text-xs text-muted-foreground">
                        {['queued', 'running'].includes(
                          t.latest_attempt?.status || '',
                        ) && <LoaderCircle className="h-3 w-3 animate-spin" />}
                        {executionLabel(t)}
                      </span>
                    )}
                  </td>
                  <td className="px-3 py-3">
                    <span
                      className={cn(
                        'text-xs capitalize',
                        t.priority === 'urgent' &&
                          'font-semibold text-destructive',
                        t.priority === 'high' && 'font-semibold',
                      )}
                    >
                      {t.priority}
                    </span>
                  </td>
                  <td className="break-words px-3 py-3 text-xs text-muted-foreground">
                    {name(t)}
                  </td>
                  <td className="px-3 py-3 text-xs text-muted-foreground">
                    <time
                      dateTime={t.updated_at}
                      title={new Date(t.updated_at).toLocaleString('en-US')}
                    >
                      {relativeTime(t.updated_at)}
                    </time>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
          <div role="list" className="divide-y md:hidden">
            {rows.slice(0, limit).map((t) => (
              <div key={t.id} role="listitem">
                <button
                  className="block min-h-24 w-full p-3 text-left hover:bg-muted/50 focus-visible:outline focus-visible:outline-2 focus-visible:outline-ring"
                  onClick={() => onSelectTask(t)}
                >
                  <span className="mb-1.5 flex items-center justify-between gap-2">
                    <span className="font-mono text-xs text-muted-foreground">
                      {taskRef(t.id)}
                    </span>
                    <span className="rounded border px-2 py-1 text-xs">
                      {statusLabel(t.status)}
                    </span>
                  </span>
                  <span className="line-clamp-2 break-words text-base font-medium">
                    {t.title}
                  </span>
                  <span className="mt-2 block truncate text-xs text-muted-foreground">
                    {!activeProject && `${projectName(t)} · `}
                    {name(t)} ·{' '}
                    <span
                      className={
                        t.priority === 'urgent' ? 'text-destructive' : ''
                      }
                    >
                      {t.priority}
                    </span>
                  </span>
                  {executionLabel(t) && (
                    <span className="mt-1 block text-xs text-muted-foreground">
                      {executionLabel(t)}
                    </span>
                  )}
                </button>
              </div>
            ))}
          </div>
          {rows.length > limit && (
            <button
              className="action-secondary m-3"
              onClick={() => setLimit((n) => n + 100)}
            >
              Show more ({rows.length - limit} remaining)
            </button>
          )}
        </div>
      )}
      <ActionDialog
        open={filtersOpen}
        title="Filter tasks"
        onClose={() => setFiltersOpen(false)}
      >
        <div className="space-y-4">
          {fields}
          <label className="grid gap-1 text-sm">
            Status
            <select
              className="field-control"
              aria-label="Filter status"
              value={status}
              onChange={(e) => update('status', e.target.value)}
            >
              <option value="all">All statuses</option>
              {TASK_STATUSES.map((s) => (
                <option key={s.id} value={s.id}>
                  {s.label}
                </option>
              ))}
            </select>
          </label>
          <div className="flex justify-between pt-2">
            <button className="action-secondary" onClick={clear}>
              Clear filters
            </button>
            <button
              className="action-primary"
              onClick={() => setFiltersOpen(false)}
            >
              Show {rows.length} {rows.length === 1 ? 'task' : 'tasks'}
            </button>
          </div>
        </div>
      </ActionDialog>
    </section>
  )
}
