import { useEffect, useMemo, useState } from 'react'
import { useSearchParams } from 'react-router-dom'
import {
  Search,
  SlidersHorizontal,
  ArrowUpDown,
  Circle,
  LoaderCircle,
  CircleDot,
  CheckCircle2,
  Ban,
  Clock,
  X,
  ChevronDown,
} from 'lucide-react'
import type { Task, TaskStatus, Project } from '../../api/types'
import { agentsApi, type Agent } from '../../api/agents'
import { ActionDialog } from '../../components/dialogs/ActionDialog'
import {
  TASK_STATUSES,
  taskStatus,
  taskRef,
  relativeTime,
  getQueuedTasksPositions,
} from './task-ui'
import { TaskItemCard } from './TaskItemCard'
import { HairlineList } from '../../components/layout/HairlineList'
import { cn } from '../../lib/utils'
import { tasksApi } from '../../api/tasks'
import { useLanguage } from '../../context/LanguageContext'
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
  onDeleteTask?: (id: string) => Promise<void> | void
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
  onDeleteTask,
  onRunAgent,
  loading = false,
  error = '',
  onRetry,
}: Props) {
  const { t } = useLanguage()
  const [params, setParams] = useSearchParams(),
    [filtersOpen, setFiltersOpen] = useState(false),
    [sortOpen, setSortOpen] = useState(false),
    [saving, setSaving] = useState(''),
    [actionError, setActionError] = useState(''),
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
  const requestPreview = (t: Task) => {
    const text = (t.description || t.prompt || t.body || '')
      .trim()
      .split('\n')[0]
      .slice(0, 240)
    return text && text.toLowerCase() !== t.title.trim().toLowerCase()
      ? text
      : ''
  }
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
  const isTaskActive = (t: Task) => {
    const s = taskStatus(t.status)
    return (
      ['doing', 'queued', 'todo', 'backlog'].includes(s) ||
      ['running', 'queued'].includes(t.latest_attempt?.status || '')
    )
  }
  const isTaskDone = (t: Task) => taskStatus(t.status) === 'done'
  const isTaskReviewOrCancelled = (t: Task) => {
    const s = taskStatus(t.status)
    return (
      ['review', 'blocked', 'failed', 'cancelled'].includes(s) ||
      ['failed', 'canceled', 'cancelled'].includes(
        t.latest_attempt?.status || '',
      )
    )
  }

  const matchesStatus = (t: Task) => {
    if (!status || status === 'all') return true
    if (status === 'active' || status === 'aktif') {
      return isTaskActive(t)
    }
    if (status === 'done' || status === 'selesai') {
      return isTaskDone(t)
    }
    if (
      status === 'dibatalkan_review' ||
      status === 'dibatalkan' ||
      status === 'review_or_blocked'
    ) {
      return isTaskReviewOrCancelled(t)
    }
    return taskStatus(t.status) === status
  }

  const mobileFilterChips = [
    {
      id: 'all',
      label: t('Semua'),
      count: scoped.length,
    },
    {
      id: 'active',
      label: t('Aktif'),
      count: scoped.filter(isTaskActive).length,
    },
    {
      id: 'done',
      label: t('Selesai'),
      count: scoped.filter(isTaskDone).length,
    },
    {
      id: 'dibatalkan_review',
      label: t('Dibatalkan/Review'),
      count: scoped.filter(isTaskReviewOrCancelled).length,
    },
  ]

  const rows = scoped
    .filter(
      (t) =>
        matchesStatus(t) &&
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
  const sortOptions = (
    <>
      <option value="updated">Updated</option>
      <option value="created">Newest</option>
      <option value="priority">Priority</option>
      <option value="title">Title</option>
    </>
  )
  const chips = [
    ...(!activeProject && project !== 'all'
      ? [
          {
            key: 'project',
            label:
              'Project: ' +
              (project === 'none'
                ? 'No project'
                : projects.find((p) => p.id === project)?.name || project),
          },
        ]
      : []),
    ...(priority !== 'all'
      ? [{ key: 'priority', label: 'Priority: ' + priority }]
      : []),
    ...(assignee !== 'all'
      ? [
          {
            key: 'assignee',
            label:
              'Assignee: ' +
              (assignee === 'unassigned'
                ? 'Unassigned'
                : names.get(assignee) || assignee),
          },
        ]
      : []),
  ]
  const saveRow = async (
    t: Task,
    patch: import('../../api/types').UpdateTaskRequest,
  ) => {
    if (saving) return
    setSaving(t.id)
    setActionError('')
    try {
      await tasksApi.update(t.id, {
        ...patch,
        expected_updated_at: t.updated_at,
      })
      await onRunAgent(t.id)
    } catch (e) {
      setActionError(
        e instanceof Error ? e.message : 'Could not save the task. Try again.',
      )
      await onRunAgent(t.id)
    } finally {
      setSaving('')
    }
  }
  const statusTone = (value: string) =>
    ({
      doing: 'text-blue-600 dark:text-blue-400',
      queued: 'text-amber-600 dark:text-amber-400',
      review: 'text-purple-600 dark:text-purple-400',
      done: 'text-emerald-700 dark:text-emerald-400',
      blocked: 'text-destructive dark:text-red-400',
    })[taskStatus(value)] || 'text-muted-foreground'
  const statusIcon = (value: string) => {
    const Icon =
      {
        doing: LoaderCircle,
        queued: Clock,
        review: CircleDot,
        done: CheckCircle2,
        blocked: Ban,
      }[taskStatus(value)] || Circle
    return (
      <Icon
        aria-hidden
        className={cn(
          'h-3.5 w-3.5 shrink-0',
          statusTone(value),
          value === 'doing' && 'animate-spin',
          value === 'backlog' && 'opacity-60',
        )}
      />
    )
  }
  const statusPicker = (t: Task) => (
    <span className="relative inline-flex max-w-full items-center gap-1.5 rounded-md border px-2 text-xs">
      {statusIcon(t.status)}
      <select
        aria-label={`Change status for ${t.title}`}
        className="h-9 min-w-0 max-w-full appearance-none bg-transparent pr-4 text-xs outline-none focus-visible:ring-2 focus-visible:ring-ring disabled:cursor-default"
        value={taskStatus(t.status)}
        disabled={
          !!saving ||
          ['running', 'queued'].includes(t.latest_attempt?.status || '') ||
          t.status === 'done'
        }
        onClick={(e) => e.stopPropagation()}
        onChange={(e) => {
          if (e.target.value === 'done') onSelectTask(t)
          else void saveRow(t, { status: e.target.value as TaskStatus })
        }}
      >
        {TASK_STATUSES.map((item) => (
          <option
            key={item.id}
            value={item.id}
            disabled={
              item.id !== taskStatus(t.status) &&
              (['doing', 'review'].includes(item.id) ||
                (item.id === 'done' &&
                  !(
                    t.status === 'review' &&
                    t.latest_attempt?.status === 'succeeded' &&
                    t.summary &&
                    t.complete_note
                  )))
            }
          >
            {item.label}
            {item.id !== taskStatus(t.status) &&
              (item.id === 'doing'
                ? ' · automatic'
                : item.id === 'review'
                  ? ' · after result'
                  : item.id === 'done'
                    ? ' · completed work'
                    : '')}
          </option>
        ))}
      </select>
      <ChevronDown
        aria-hidden
        className="pointer-events-none absolute right-2 h-3 w-3 text-muted-foreground"
      />
    </span>
  )
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
        <label className="relative min-w-0 flex-1 sm:max-w-sm lg:max-w-md">
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
          className="action-secondary ml-auto h-11 px-3"
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
        <button
          className="icon-action sm:hidden"
          aria-label="Sort tasks"
          onClick={() => setSortOpen(true)}
        >
          <ArrowUpDown className="h-4 w-4" />
        </button>
        <label className="relative hidden sm:block">
          <ArrowUpDown
            aria-hidden
            className="pointer-events-none absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground"
          />
          <select
            aria-label="Sort tasks"
            value={sort}
            onChange={(e) => update('sort', e.target.value)}
            className="h-11 w-36 appearance-none rounded-lg border border-input bg-background pl-9 pr-8 text-sm focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
          >
            {sortOptions}
          </select>
          <ChevronDown
            aria-hidden
            className="pointer-events-none absolute right-3 top-1/2 h-3.5 w-3.5 -translate-y-1/2 text-muted-foreground"
          />
        </label>
      </div>
      {/* Mobile Horizontal Scrollable Filter Chips */}
      <div
        role="tablist"
        aria-label={t('Task status filter chips')}
        className="flex sm:hidden items-center gap-2 overflow-x-auto py-1 -mx-1 px-1 [scrollbar-width:none] [-ms-overflow-style:none] [&::-webkit-scrollbar]:hidden"
      >
        {mobileFilterChips.map((chip) => {
          const isSelected =
            (chip.id === 'all' && (status === 'all' || !status)) ||
            (chip.id === 'active' &&
              (status === 'active' ||
                status === 'aktif' ||
                ['doing', 'queued', 'todo', 'backlog'].includes(status))) ||
            (chip.id === 'done' &&
              (status === 'done' || status === 'selesai')) ||
            (chip.id === 'dibatalkan_review' &&
              (status === 'dibatalkan_review' ||
                status === 'review_or_blocked' ||
                ['review', 'blocked', 'failed', 'cancelled'].includes(status)))

          return (
            <button
              key={chip.id}
              type="button"
              role="tab"
              aria-selected={isSelected}
              onClick={() => {
                update('status', chip.id)
                onStatusFilterChange?.(chip.id)
              }}
              className={cn(
                'inline-flex min-h-11 min-h-9 shrink-0 items-center gap-1.5 rounded-full px-3.5 py-1.5 text-xs font-medium transition-all select-none',
                isSelected
                  ? 'bg-primary text-primary-foreground shadow-xs font-semibold'
                  : 'bg-muted/70 text-muted-foreground hover:bg-muted hover:text-foreground border border-border/50',
              )}
            >
              <span>{chip.label}</span>
              <span
                className={cn(
                  'rounded-full px-1.5 py-0.5 text-[10px] font-mono tabular-nums leading-none',
                  isSelected
                    ? 'bg-primary-foreground/20 text-primary-foreground font-semibold'
                    : 'bg-background/80 text-muted-foreground',
                )}
              >
                {chip.count}
              </span>
            </button>
          )
        })}
      </div>

      {/* Desktop Horizontal Category Chips */}
      <nav
        aria-label="Filter task status"
        className="hidden sm:flex shrink-0 items-center gap-1.5 overflow-x-auto py-1 [scrollbar-width:none] [-ms-overflow-style:none] [&::-webkit-scrollbar]:hidden"
      >
        {[{ id: 'all', label: 'All' }, ...TASK_STATUSES].map((item) => {
          const isSelected = status === item.id
          const count =
            item.id === 'all'
              ? scoped.length
              : scoped.filter((t) => taskStatus(t.status) === item.id).length
          return (
            <button
              key={item.id}
              type="button"
              role="tab"
              aria-selected={isSelected}
              aria-pressed={isSelected}
              onClick={() => {
                update('status', item.id)
                onStatusFilterChange?.(item.id)
              }}
              className={cn(
                'inline-flex min-h-11 shrink-0 items-center justify-center gap-1.5 rounded-full px-3.5 py-1.5 text-xs font-medium transition-all focus-visible:outline-2 focus-visible:outline-ring',
                isSelected
                  ? 'bg-primary text-primary-foreground font-semibold shadow-sm'
                  : 'bg-muted/60 text-muted-foreground hover:bg-muted hover:text-foreground border border-border/40',
              )}
            >
              <span>{item.label}</span>
              <span
                className={cn(
                  'inline-flex min-w-4 items-center justify-center rounded-full px-1.5 text-[10px] font-mono leading-none',
                  isSelected
                    ? 'bg-primary-foreground/20 text-primary-foreground font-semibold'
                    : 'bg-muted-foreground/15 text-muted-foreground',
                )}
              >
                {count}
              </span>
            </button>
          )
        })}
      </nav>
      {chips.length > 0 && (
        <div
          aria-label="Active task filters"
          className="flex flex-wrap items-center gap-1.5 pt-0.5"
        >
          {chips.map((chip) => (
            <button
              key={chip.key}
              aria-label={`Remove ${chip.label} filter`}
              className="inline-flex min-h-9 max-w-full items-center gap-1.5 rounded-full border border-border/60 bg-muted/40 px-2.5 py-1 text-xs text-foreground hover:bg-muted transition-colors"
              onClick={() => update(chip.key, 'all')}
            >
              <span className="truncate">{chip.label}</span>
              <X className="h-3 w-3 shrink-0 text-muted-foreground" />
            </button>
          ))}
          <button
            onClick={clear}
            className="min-h-9 px-2 text-xs text-muted-foreground hover:text-foreground underline underline-offset-2"
          >
            Clear all
          </button>
        </div>
      )}
      <div
        className={cn(
          'flex items-center justify-between gap-2 text-xs text-muted-foreground',
          !filtered && !saving && 'sr-only',
        )}
      >
        <span role="status">
          {saving
            ? 'Saving changes…'
            : `${rows.length} ${rows.length === 1 ? 'task' : 'tasks'}${filtered ? ` of ${scoped.length}` : ''}`}
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
      {actionError && (
        <div
          role="alert"
          className="rounded-lg border border-destructive/40 p-3 text-sm text-destructive"
        >
          {actionError}
          <button className="ml-3 underline" onClick={() => setActionError('')}>
            Dismiss
          </button>
        </div>
      )}
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
        <div className="min-h-0 flex-1 overflow-auto">
          <div className="hidden overflow-hidden rounded-xl border border-border/70 lg:block">
            <table className="w-full table-fixed text-left text-sm">
              <thead className="sticky top-0 z-10 bg-muted/30 text-xs font-medium text-muted-foreground">
                <tr>
                  <th className="w-[48%] px-4 py-3">Task</th>
                  <th className="w-[14%] px-3 py-3">Status</th>
                  <th className="w-[10%] px-3 py-3">Priority</th>
                  <th className="w-[17%] px-3 py-3">Agent</th>
                  <th className="w-[11%] px-3 py-3 text-right">Updated</th>
                </tr>
              </thead>
              <tbody className="divide-y">
                {rows.slice(0, limit).map((t) => (
                  <tr
                    key={t.id}
                    className="group cursor-pointer transition-colors hover:bg-muted/30"
                    onClick={() => onSelectTask(t)}
                  >
                    <td className="px-4 py-3">
                      <button
                        className="block w-full text-left font-medium leading-5 hover:underline focus-visible:outline focus-visible:outline-2 focus-visible:outline-ring"
                        onClick={(e) => {
                          e.stopPropagation()
                          onSelectTask(t)
                        }}
                      >
                        <span
                          className="line-clamp-1 break-words"
                          title={t.title}
                        >
                          {t.title}
                        </span>
                      </button>
                      {requestPreview(t) && (
                        <p
                          className="mt-1 truncate text-xs text-muted-foreground"
                          title={requestPreview(t)}
                        >
                          <span className="font-medium">Request:</span>{' '}
                          {requestPreview(t)}
                        </p>
                      )}
                      <div className="mt-1 flex min-w-0 items-center gap-2 text-xs text-muted-foreground">
                        <span className="shrink-0 font-mono">
                          {taskRef(t.id)}
                        </span>
                        {!activeProject && projectName(t) !== 'No project' && (
                          <>
                            <span>·</span>
                            <span className="truncate">{projectName(t)}</span>
                          </>
                        )}
                      </div>
                    </td>
                    <td className="px-3 py-2">{statusPicker(t)}</td>
                    <td className="px-3 py-2">
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
                      <div className="relative">
                        <span
                          aria-hidden
                          className="pointer-events-none absolute left-1 top-1/2 flex h-5 w-5 -translate-y-1/2 items-center justify-center rounded-full bg-muted text-[10px] font-medium text-muted-foreground"
                        >
                          {t.assignee_id || t.assignee
                            ? name(t)
                                .replace(/^@/, '')
                                .slice(0, 1)
                                .toUpperCase()
                            : '–'}
                        </span>
                        <select
                          aria-label={`Change assignee for ${t.title}`}
                          className="h-9 w-full appearance-none truncate rounded-md border border-transparent bg-transparent pl-8 pr-5 text-xs text-foreground hover:border-input focus-visible:outline focus-visible:outline-2 focus-visible:outline-ring"
                          value={t.assignee_id || t.assignee || ''}
                          disabled={
                            !!saving ||
                            ['running', 'queued'].includes(
                              t.latest_attempt?.status || '',
                            )
                          }
                          onClick={(e) => e.stopPropagation()}
                          onChange={(e) =>
                            void saveRow(t, { assignee_id: e.target.value })
                          }
                        >
                          <option value="">Unassigned</option>
                          {!names.has(t.assignee_id || t.assignee || '') &&
                            (t.assignee_id || t.assignee) && (
                              <option value={t.assignee_id || t.assignee}>
                                {name(t)}
                              </option>
                            )}
                          {agents
                            .filter(
                              (a) =>
                                a.enabled !== false ||
                                a.id === (t.assignee_id || t.assignee),
                            )
                            .map((a) => (
                              <option key={a.id} value={a.id}>
                                {a.name}
                              </option>
                            ))}
                        </select>
                        <ChevronDown
                          aria-hidden
                          className="pointer-events-none absolute right-1 top-1/2 h-3 w-3 -translate-y-1/2 text-muted-foreground"
                        />
                      </div>
                    </td>
                    <td className="whitespace-nowrap px-3 py-3 text-right text-xs text-muted-foreground">
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
          </div>
          <HairlineList className="lg:hidden px-2">
            {(() => {
              const queuedPositions = getQueuedTasksPositions(rows)
              return rows.slice(0, limit).map((t) => (
                <TaskItemCard
                  key={t.id}
                  task={t}
                  variant="row"
                  queuePosition={queuedPositions.get(t.id)}
                  showProject={!activeProject}
                  onClick={onSelectTask}
                  onDelete={onDeleteTask ? () => onDeleteTask(t.id) : undefined}
                />
              ))
            })()}
          </HairlineList>
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
        open={sortOpen}
        title="Sort tasks"
        onClose={() => setSortOpen(false)}
      >
        <fieldset className="grid gap-1">
          <legend className="sr-only">Sort order</legend>
          {[
            { id: 'updated', label: 'Recently updated' },
            { id: 'created', label: 'Newest first' },
            { id: 'priority', label: 'Highest priority' },
            { id: 'title', label: 'Title A–Z' },
          ].map((item) => (
            <label
              key={item.id}
              className="flex min-h-11 cursor-pointer items-center gap-3 rounded-md px-2 hover:bg-muted"
            >
              <input
                type="radio"
                name="task-sort"
                value={item.id}
                checked={sort === item.id}
                onChange={() => {
                  update('sort', item.id)
                  setSortOpen(false)
                }}
              />
              {item.label}
            </label>
          ))}
        </fieldset>
      </ActionDialog>
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
