import { useMediaQuery } from '../../hooks/useMediaQuery'
import { TaskPagePager } from './TaskPagePager'
import { useTaskPage } from './useTaskPage'
import { TaskRowActions } from './TaskRowActions'
import { ListSearch, ListTabs, ListTab } from '../../components/layout/ListControls'
import { IconButton } from '../../components/ui/IconButton'
import { useEffect, useState } from 'react'
import { useSearchParams } from 'react-router-dom'
import {
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
  PlusCircle,
  ArrowDown,
  ArrowRight,
  ArrowUp,
} from 'lucide-react'
import type { Task, TaskStatus, Project } from '../../api/types'
import { ActionDialog } from '../../components/dialogs/ActionDialog'
import {
  TASK_STATUSES,
  taskStatus,
  taskRef,
  relativeTime,
} from './task-ui'
import { TaskItemCard } from './TaskItemCard'
import { HairlineList } from '../../components/layout/HairlineList'
import { cn } from '../../lib/utils'
import { tasksApi } from '../../api/tasks'
import { useLanguage } from '../../context/LanguageContext'
interface Props {
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
  projects,
  activeProject,
  onSelectTask,
  onStatusFilterChange,
  onDeleteTask,
  onRunAgent,
}: Props) {
  const { t } = useLanguage()
  const isDesktop = useMediaQuery('(min-width: 1024px)')
  const [params, setParams] = useSearchParams(),
    [filtersOpen, setFiltersOpen] = useState(false),
    [sortOpen, setSortOpen] = useState(false),
    [saving, setSaving] = useState(''),
    [actionError, setActionError] = useState(''),
    [selectedRows, setSelectedRows] = useState<Set<string>>(new Set()),
    [currentPage, setCurrentPage] = useState(1)
  useEffect(() => setSelectedRows(new Set()), [currentPage])
  const toggleSelectAll = (totalRows: Task[]) => {
    if (selectedRows.size === totalRows.length) {
      setSelectedRows(new Set())
    } else {
      setSelectedRows(new Set(totalRows.map((r) => r.id)))
    }
  }
  const toggleSelectRow = (id: string, e: React.MouseEvent) => {
    e.stopPropagation()
    setSelectedRows((prev) => {
      const next = new Set(prev)
      if (next.has(id)) next.delete(id)
      else next.add(id)
      return next
    })
  }
  const query = params.get('q') || '',
    status = params.get('status') || 'all',
    project = activeProject?.id || params.get('project') || 'all',
    priority = params.get('priority') || 'all',
    sort = params.get('sort') || 'updated'
  useEffect(() => {
    if (!params.has('assignee') && !/^(agent|assignee)(-desc)?$/.test(params.get('sort') || '')) return
    setParams(old => {
      const next = new URLSearchParams(old)
      next.delete('assignee')
      if (/^(agent|assignee)(-desc)?$/.test(next.get('sort') || '')) next.delete('sort')
      return next
    }, { replace: true })
  }, [params, setParams])
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
  const handleSortColumn = (col: string) => {
    if (sort === col) {
      update('sort', `${col}-desc`)
    } else if (sort === `${col}-desc`) {
      update('sort', col === 'updated' ? 'updated-asc' : 'updated')
    } else {
      update('sort', col)
    }
  }
  useEffect(() => {
    setCurrentPage(1)
    setSelectedRows(new Set())
  }, [query, status, project, priority, sort])
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
  const { data, counts, loading, error, refresh: onRetry } = useTaskPage({ project_id: project, search: query, status, priority, sort, page: currentPage, page_size: 25 })
  const tasks = data?.tasks || []
  const total = data?.total || 0
  useEffect(() => {
    if (data && data.page !== currentPage) setCurrentPage(data.page)
  }, [data?.page, currentPage])
  const mobileFilterChips = [
    {
      id: 'all',
      label: t('Semua'),
      count: counts.all || 0,
    },
    {
      id: 'active',
      label: t('Aktif'),
      count: counts.active || 0,
    },
    {
      id: 'done',
      label: t('Selesai'),
      count: counts.done || 0,
    },
    {
      id: 'dibatalkan_review',
      label: t('Dibatalkan/Review'),
      count: counts.review_or_blocked || 0,
    },
  ]

  const rows = tasks
  const pageSize = data?.page_size || 25
  const paginatedRows = rows
  const filterCount =
    Number(project !== 'all' && !activeProject) +
    Number(priority !== 'all')
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
    <span className="relative inline-flex max-w-full items-center gap-1 rounded-md border-border/40 bg-muted/20 px-2 text-xs hover:bg-muted/40 transition-colors border">
      {statusIcon(t.status)}
      <select
        aria-label={`Change status for ${t.title}`}
        className="h-7 min-w-0 max-w-full appearance-none bg-transparent pr-4 text-xs outline-none focus-visible:ring-1 focus-visible:ring-ring disabled:cursor-default"
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
        className="pointer-events-none absolute right-1.5 h-3 w-3 text-muted-foreground/40 opacity-0 group-hover:opacity-100 transition-opacity"
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

    </>
  )
  return (
    <section
      className="flex h-full min-h-0 flex-col gap-2.5"
      aria-label="Task list"
    >
      <h2 className="sr-only">{t('Tasks')}</h2>
      <div className="flex items-center gap-2">
        <ListSearch containerClassName="sm:max-w-xs lg:max-w-sm" aria-label={t('Search tasks')} placeholder={t('Search tasks…')} value={query} onChange={event=>update('q',event.target.value)} />
        <button
          type="button"
          onClick={() => setFiltersOpen(true)}
          className={cn(
            "hidden sm:inline-flex h-11 items-center gap-1.5 rounded-md border border-dashed border-border/80 px-2.5 text-xs font-medium hover:bg-accent/50 transition-colors",
            status !== 'all' && "border-solid bg-accent/40 text-foreground"
          )}
        >
          <PlusCircle className="h-3.5 w-3.5 text-muted-foreground" />
          <span>Status</span>
          {status !== 'all' && (
            <span className="rounded bg-secondary px-1 font-mono text-[10px] text-secondary-foreground font-semibold">
              {status}
            </span>
          )}
        </button>
        <button
          type="button"
          onClick={() => setFiltersOpen(true)}
          className={cn(
            "hidden sm:inline-flex h-11 items-center gap-1.5 rounded-md border border-dashed border-border/80 px-2.5 text-xs font-medium hover:bg-accent/50 transition-colors",
            priority !== 'all' && "border-solid bg-accent/40 text-foreground"
          )}
        >
          <PlusCircle className="h-3.5 w-3.5 text-muted-foreground" />
          <span>Priority</span>
          {priority !== 'all' && (
            <span className="rounded bg-secondary px-1 font-mono text-[10px] text-secondary-foreground font-semibold">
              {priority}
            </span>
          )}
        </button>
        {(status !== 'all' || priority !== 'all' || query) && (
          <button
            type="button"
            onClick={clear}
            className="hidden sm:inline-flex h-11 items-center gap-1 px-2 text-xs font-medium text-muted-foreground hover:text-foreground transition-colors"
          >
            <span>Reset</span>
            <X className="h-3.5 w-3.5" />
          </button>
        )}
        <IconButton label={filterCount ? t('Filters')+' ('+filterCount+')' : t('Filters')} className="relative ml-auto" active={filterCount>0} onClick={()=>setFiltersOpen(true)}><SlidersHorizontal />{filterCount>0&&<span className="absolute right-0.5 top-0.5 text-[10px] tabular-nums">{filterCount}</span>}</IconButton>
        <IconButton label={t('Sort tasks')} className="sm:hidden" onClick={()=>setSortOpen(true)}><ArrowUpDown /></IconButton>
        <label className="relative hidden sm:block">
          <ArrowUpDown
            aria-hidden
            className="pointer-events-none absolute left-2.5 top-1/2 h-3.5 w-3.5 -translate-y-1/2 text-muted-foreground"
          />
          <select
            aria-label="Sort tasks"
            value={sort}
            onChange={(e) => update('sort', e.target.value)}
            className="h-11 w-32 appearance-none rounded-md border border-input bg-transparent pl-7 pr-6 text-xs hover:bg-accent/30 transition-colors focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-ring"
          >
            {sortOptions}
          </select>
          <ChevronDown
            aria-hidden
            className="pointer-events-none absolute right-2 top-1/2 h-3 w-3 -translate-y-1/2 text-muted-foreground"
          />
        </label>
      </div>
      {/* Mobile Horizontal Scrollable Filter Chips */}
      <ListTabs className="sm:hidden" aria-label={t('Task status filter chips')}>
        {mobileFilterChips.map(chip=>{
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

          return <ListTab key={chip.id} selected={isSelected} count={chip.count} onClick={()=>{update('status',chip.id);onStatusFilterChange?.(chip.id)}}>{chip.label}</ListTab>
        })}
      </ListTabs>

      {/* Desktop Horizontal Category Chips */}
      <ListTabs className="hidden sm:flex" aria-label={t('Filter task status')}>
        {[{id:'all',label:t('All')},...TASK_STATUSES].map(item=><ListTab key={item.id} selected={status===item.id} count={counts[item.id] || 0} onClick={()=>{update('status',item.id);onStatusFilterChange?.(item.id)}}>{t(item.label)}</ListTab>)}
      </ListTabs>
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
            : `${total} ${total === 1 ? 'task' : 'tasks'}`}
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
      ) : error && !data ? null : rows.length === 0 ? (
        <div className="flex flex-1 flex-col items-center justify-center gap-2 rounded-xl border border-dashed border-border/40 bg-muted/10 p-8 text-center">
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
          {isDesktop && <div className="hidden overflow-hidden rounded-md border border-border/40 bg-card/30 lg:block shadow-2xs">
            <table className="w-full table-fixed text-left text-sm">
              <thead className="sticky top-0 z-10 bg-muted/40 text-xs font-medium text-muted-foreground border-b border-border/40">
                <tr>
                  <th className="w-[40px] px-2 py-2 text-center">
                    <input
                      type="checkbox"
                      aria-label="Select all tasks"
                      checked={rows.length > 0 && rows.every(row => selectedRows.has(row.id))}
                      onChange={() => toggleSelectAll(rows)}
                      className="h-3.5 w-3.5 rounded border-border text-primary focus:ring-ring"
                    />
                  </th>
                  <th
                    className="w-[85px] px-2 py-2 font-mono text-xs font-medium cursor-pointer select-none hover:text-foreground transition-colors"
                    onClick={() => handleSortColumn('task')}
                    title="Sort by Task ID"
                  >
                    <span className="inline-flex items-center gap-1">
                      <span>Task</span>
                      {sort === 'task' ? <ArrowUp className="h-3 w-3 text-primary" /> : sort === 'task-desc' ? <ArrowDown className="h-3 w-3 text-primary" /> : null}
                    </span>
                  </th>
                  <th
                    className="w-[49%] px-2 py-2 font-medium cursor-pointer select-none hover:text-foreground transition-colors"
                    onClick={() => handleSortColumn('title')}
                    title="Sort by Title"
                  >
                    <span className="inline-flex items-center gap-1">
                      <span>Title</span>
                      {sort === 'title' ? <ArrowUp className="h-3 w-3 text-primary" /> : sort === 'title-desc' ? <ArrowDown className="h-3 w-3 text-primary" /> : null}
                    </span>
                  </th>
                  <th
                    className="w-[14%] px-2 py-2 font-medium cursor-pointer select-none hover:text-foreground transition-colors"
                    onClick={() => handleSortColumn('status')}
                    title="Sort by Status"
                  >
                    <span className="inline-flex items-center gap-1">
                      <span>Status</span>
                      {sort === 'status' ? <ArrowUp className="h-3 w-3 text-primary" /> : sort === 'status-desc' ? <ArrowDown className="h-3 w-3 text-primary" /> : null}
                    </span>
                  </th>
                  <th
                    className="w-[11%] px-2 py-2 font-medium cursor-pointer select-none hover:text-foreground transition-colors"
                    onClick={() => handleSortColumn('priority')}
                    title="Sort by Priority"
                  >
                    <span className="inline-flex items-center gap-1">
                      <span>Priority</span>
                      {sort === 'priority' ? <ArrowUp className="h-3 w-3 text-primary" /> : sort === 'priority-desc' ? <ArrowDown className="h-3 w-3 text-primary" /> : null}
                    </span>
                  </th>
                  <th
                    className="w-[11%] px-2 py-2 text-right font-medium cursor-pointer select-none hover:text-foreground transition-colors"
                    onClick={() => handleSortColumn('updated')}
                    title="Sort by Updated Time"
                  >
                    <span className="inline-flex items-center gap-1 justify-end w-full">
                      <span>Updated</span>
                      {sort === 'updated-asc' ? <ArrowUp className="h-3 w-3 text-primary" /> : (sort === 'updated' || !sort) ? <ArrowDown className="h-3 w-3 text-primary" /> : null}
                    </span>
                  </th>
                  <th className="w-[40px] px-1 py-2 text-center">
                    <span className="sr-only">Actions</span>
                  </th>
                </tr>
              </thead>
              <tbody className="divide-y divide-border/40">
                {paginatedRows.map((t) => (
                  <tr
                    key={t.id}
                    className={cn(
                      "group h-10 cursor-pointer transition-colors hover:bg-muted/30",
                      selectedRows.has(t.id) && "bg-muted/40"
                    )}
                    onClick={() => onSelectTask(t)}
                  >
                    <td className="w-[40px] px-2 py-2.5 text-center" onClick={(e) => toggleSelectRow(t.id, e)}>
                      <input
                        type="checkbox"
                        aria-label={`Select task ${t.title}`}
                        checked={selectedRows.has(t.id)}
                        onChange={() => {}}
                        className="h-3.5 w-3.5 rounded border-border text-primary focus:ring-ring"
                      />
                    </td>
                    <td className="px-2 py-2.5 font-mono text-xs text-muted-foreground font-medium">
                      {taskRef(t.id)}
                    </td>
                    <td className="px-2 py-2.5">
                      <div className="flex items-center gap-1.5 max-w-full">
                        {!activeProject && projectName(t) !== 'No project' && (
                          <span className="inline-flex shrink-0 items-center rounded border border-border/50 bg-muted/40 px-1.5 py-0.2 text-[10px] font-semibold text-muted-foreground truncate max-w-[110px]">
                            {projectName(t)}
                          </span>
                        )}
                        <button
                          className="block text-left text-sm font-medium leading-6 hover:underline focus-visible:outline focus-visible:outline-2 focus-visible:outline-ring truncate"
                          onClick={(e) => {
                            e.stopPropagation()
                            onSelectTask(t)
                          }}
                        >
                          <span
                            className="truncate"
                            title={requestPreview(t) ? `${t.title} — Request: ${requestPreview(t)}` : t.title}
                          >
                            {t.title}
                          </span>
                        </button>
                      </div>
                    </td>
                    <td className="px-2 py-1">{statusPicker(t)}</td>
                    <td className="px-2 py-1">
                      <span className="inline-flex items-center gap-1 text-sm">
                        {t.priority === 'urgent' || t.priority === 'high' ? (
                          <ArrowUp className="h-3 w-3 text-destructive" />
                        ) : t.priority === 'low' ? (
                          <ArrowDown className="h-3 w-3 text-muted-foreground" />
                        ) : (
                          <ArrowRight className="h-3 w-3 text-muted-foreground" />
                        )}
                        <span
                          className={cn(
                            'capitalize text-sm',
                            t.priority === 'urgent' && 'font-semibold text-destructive',
                            t.priority === 'high' && 'font-semibold text-foreground',
                            t.priority === 'normal' && 'text-muted-foreground',
                            t.priority === 'low' && 'text-muted-foreground',
                          )}
                        >
                          {t.priority}
                        </span>
                      </span>
                    </td>
                    <td className="whitespace-nowrap px-2 py-2.5 text-right text-xs font-mono text-muted-foreground">
                      <time
                        dateTime={t.updated_at}
                        title={new Date(t.updated_at).toLocaleString('en-US')}
                      >
                        {relativeTime(t.updated_at)}
                      </time>
                    </td>
                    <td className="px-1 py-1 text-center" onClick={(e) => e.stopPropagation()}>
                      <TaskRowActions task={t} onDelete={onDeleteTask} onChanged={onRetry} />
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
            {selectedRows.size > 0 && <p className="px-3 py-2 text-xs text-muted-foreground">{selectedRows.size} / {rows.length}</p>}
            <TaskPagePager className="hidden border-t border-border/40 px-3 lg:flex" page={currentPage} pageSize={pageSize} total={total} onPage={setCurrentPage} />
          </div>}
          {!isDesktop && <HairlineList className="lg:hidden">
            {(() => {
              return rows.map((t) => (
                <TaskRowActions key={t.id} task={t} onDelete={onDeleteTask} onChanged={onRetry}>
                <TaskItemCard
                  task={t}
                  managedUpdates
                  variant="row"
                  showProject={!activeProject}
                  onClick={onSelectTask}
                  onDelete={onDeleteTask ? () => onDeleteTask(t.id) : undefined}
                />
                </TaskRowActions>
              ))
            })()}
          </HairlineList>}
          <TaskPagePager className="px-3 lg:hidden" page={currentPage} pageSize={pageSize} total={total} onPage={setCurrentPage} />
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
              Show {total} {total === 1 ? 'task' : 'tasks'}
            </button>
          </div>
        </div>
      </ActionDialog>
    </section>
  )
}
