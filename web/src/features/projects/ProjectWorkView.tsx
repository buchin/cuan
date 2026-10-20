import { ItemActions, copyItemLink } from '../../components/ui/ItemActions'
import { useLanguage } from '../../context/LanguageContext'
import { useState } from 'react'
import { Link, useLocation, useNavigate } from 'react-router-dom'
import { Search } from 'lucide-react'
import type { Project, Task } from '../../api/types'
import { TaskSessionPage } from '../tasks/TaskSessionPage'
import { TaskStartingPage } from '../tasks/TaskStartingPage'
import { relativeTime, tasksForProject, workStatus, taskRef } from '../tasks/task-ui'

export function ProjectWorkView({
  project,
  tasks,
  taskId,
  sessionId,
  loading,
  error,
  onRetry,
}: {
  project: Project
  tasks: Task[]
  taskId?: string
  sessionId?: string
  loading: boolean
  error: string
  onRetry: () => void
}) {
  const { t } = useLanguage()
  const [query, setQuery] = useState('')
  const [filter, setFilter] = useState('all')
  const location = useLocation()
  const navigate = useNavigate()
  const base = '/projects/' + encodeURIComponent(project.id) + '/tasks'
  const scoped = tasksForProject(tasks, project.id)
  const filters = [
    { id: 'all', label: 'All', matches: (_task: Task) => true },
    { id: 'active', label: 'Working', matches: (task: Task) => ['Working', 'Waiting to start'].includes(workStatus(task)) },
    { id: 'attention', label: 'Needs attention', matches: (task: Task) => ['Needs attention', 'Not running'].includes(workStatus(task)) },
    { id: 'review', label: 'Review', matches: (task: Task) => workStatus(task) === 'Ready for review' },
    { id: 'completed', label: 'Completed', matches: (task: Task) => workStatus(task) === 'Completed' },
  ]
  const summaryFor = (task: Task) => {
    const text = (workStatus(task) === 'Completed' ? task.complete_note || task.summary : task.launch?.safe_error || task.summary) || ''
    return text.replace(/\s+/g, ' ').trim()
  }
  const searched = scoped.filter(task =>
    [task.title, taskRef(task.id), task.status, workStatus(task), summaryFor(task), task.latest_attempt?.agent_name]
      .join(' ').toLowerCase().includes(query.toLowerCase().trim()))
  const visible = searched.filter(task => (filters.find(item => item.id === filter) || filters[0]).matches(task))
  const selected = scoped.find((t) => t.id === taskId)
  if (taskId)
    return (
      <div className="flex h-full min-h-0 flex-col">
        <div className="min-h-0 flex-1">
          {selected ? (
            sessionId === 'start' ? (
              <TaskStartingPage key={taskId} taskId={taskId} />
            ) : (
              <TaskSessionPage
                key={taskId}
                taskId={taskId}
                sessionId={sessionId || 'latest'}
              />
            )
          ) : (
            <p className="p-5 text-sm" role={loading ? 'status' : 'alert'}>
              {t(loading ? 'Loading work…' : 'Work not found in this project.')}
            </p>
          )}
        </div>
      </div>
    )
  return (
    <section
      className="flex h-full min-h-0 flex-col"
      aria-label={t('Project tasks')}
    >
      <div className="flex shrink-0 flex-wrap items-center justify-between gap-3 p-3 sm:p-5">
        <h3 className="text-sm font-semibold">
          {t('Tasks')}{' '}
          <span className="ml-1 text-xs font-normal text-muted-foreground">
            {scoped.length}
          </span>
        </h3>
        <label className="flex min-h-11 w-full items-center gap-2 rounded-md border px-3 sm:w-64">
          <Search className="h-4 w-4 text-muted-foreground" />
          <input
            className="w-full min-w-0 bg-transparent text-base outline-none sm:text-sm"
            aria-label={t('Search project work')}
            placeholder={t('Search tasks…')}
            value={query}
            onChange={(event) => setQuery(event.target.value)}
          />
        </label>
      </div>
      <div role="group" aria-label={t('Task filters')} className="flex shrink-0 flex-wrap gap-x-3 border-b px-3 sm:px-5">
        {filters.map(item => <button key={item.id} type="button" aria-pressed={filter === item.id}
          className={`min-h-11 border-b-2 text-xs ${filter === item.id ? 'border-primary font-medium text-primary' : 'border-transparent text-muted-foreground hover:text-foreground'}`}
          onClick={() => setFilter(item.id)}>{t(item.label)} <span className="ml-1 tabular-nums">{searched.filter(item.matches).length}</span></button>)}
      </div>
      <div className="min-h-0 flex-1 overflow-y-auto px-3 pb-4 sm:px-5">
        {error ? (
          <div role="alert" className="p-3 text-sm">
            <p>{t(error)}</p>
            <button className="action-secondary mt-3" onClick={onRetry}>
              {t('Try again')}
            </button>
          </div>
        ) : loading && !scoped.length ? (
          <p className="p-3 text-sm" role="status">
            {t('Loading work…')}
          </p>
        ) : !visible.length ? (
          <p className="p-5 text-sm text-muted-foreground">
            {t(
              query || filter !== 'all'
                ? 'No matching work.'
                : 'No work yet. Ask an agent in Inbox or create a task.',
            )}
          </p>
        ) : (
          visible.map((item) => (
            <ItemActions key={item.id} label={item.title} actions={[
              { label: 'Open task', run: () => navigate(base + '/' + encodeURIComponent(item.id), { state: { ...location.state, taskReturnTo: base } }) },
              { label: 'Copy link', run: () => copyItemLink(base + '/' + encodeURIComponent(item.id)) },
            ]}>
            <Link
              key={item.id}
              to={base + '/' + encodeURIComponent(item.id)}
              state={{ ...location.state, taskReturnTo: base }}
              className="flex min-h-16 items-center gap-3 border-b px-2 py-3 focus-visible:outline focus-visible:outline-2 focus-visible:outline-ring hover:bg-muted"
            >
              <span className="min-w-0 flex-1">
                <span className="line-clamp-2 break-words text-sm font-medium leading-5">{item.title}</span>
                <span className="mt-1 flex flex-wrap items-center gap-x-2 gap-y-1 text-xs text-muted-foreground">
                  <span className={`inline-flex items-center gap-1 rounded-full px-2 py-0.5 ${['Needs attention', 'Not running'].includes(workStatus(item)) ? 'bg-amber-500/10 text-amber-700 dark:text-amber-400' : workStatus(item) === 'Working' ? 'bg-primary/10 text-primary' : 'bg-secondary text-muted-foreground'}`}>{t(workStatus(item))}</span>
                  <span className="tabular-nums">{taskRef(item.id)}</span>
                  {item.latest_attempt?.agent_name && <span>{item.latest_attempt.agent_name}</span>}
                  <time className="sm:ml-auto" dateTime={item.updated_at} title={new Date(item.updated_at).toLocaleString()}>{t(relativeTime(item.updated_at))}</time>
                </span>
                {summaryFor(item) && <span className="mt-1.5 line-clamp-2 break-words text-xs leading-5 text-muted-foreground">{summaryFor(item)}</span>}
              </span>
            </Link>
            </ItemActions>
          ))
        )}
      </div>
    </section>
  )
}
