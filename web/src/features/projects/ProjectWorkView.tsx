import { ItemActions, copyItemLink } from '../../components/ui/ItemActions'
import {
  ExtensionBoundary,
  projectViews,
  useExtensionRevision,
} from '../../extensions/sdk'
import { useLanguage } from '../../context/LanguageContext'
import { useEffect, useState } from 'react'
import { useLocation, useNavigate } from 'react-router-dom'
import { Search } from 'lucide-react'
import type { Project, Task } from '../../api/types'
import { TaskSessionPage } from '../tasks/TaskSessionPage'
import { TaskStartingPage } from '../tasks/TaskStartingPage'
import { TaskItemCard } from '../tasks/TaskItemCard'
import { tasksForProject, workStatus, taskRef, taskSummaryText, getQueuedTasksPositions } from '../tasks/task-ui'

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
  useExtensionRevision()
  const views = projectViews()
  const [viewId, setViewId] = useState('list')
  const navigate = useNavigate()
  const extensionView = views.find((view) => view.id === viewId)
  const [query, setQuery] = useState('')
  const [filter, setFilter] = useState('all')
  const location = useLocation()
  const base = '/projects/' + encodeURIComponent(project.id) + '/tasks'
  const scoped = tasksForProject(tasks, project.id)
  const filters = [
    { id: 'all', label: 'All', matches: (_task: Task) => true },
    { id: 'active', label: 'Active', matches: (task: Task) => ['Working', 'Waiting to start', 'Not started'].includes(workStatus(task)) },
    { id: 'attention', label: 'Needs attention', matches: (task: Task) => ['Needs attention', 'Not running', 'Ready for review'].includes(workStatus(task)) },
    { id: 'completed', label: 'Completed', matches: (task: Task) => ['Completed', 'Answered'].includes(workStatus(task)) },
  ]
  const searched = scoped.filter(task =>
    [task.title, taskRef(task.id), task.status, workStatus(task), taskSummaryText(task), task.latest_attempt?.agent_name]
      .join(' ').toLowerCase().includes(query.toLowerCase().trim()))
  const visible = searched.filter(task => (filters.find(item => item.id === filter) || filters[0]).matches(task))
  const selected = scoped.find((t) => t.id === taskId)
  const relocated = taskId && !selected ? tasks.find(task => task.id === taskId && task.project_id && task.project_id !== project.id) : undefined
  useEffect(() => {
    if (loading || error || !relocated) return
    const target = '/projects/' + encodeURIComponent(relocated.project_id!) + '/tasks/' + encodeURIComponent(relocated.id)
    navigate(target + (sessionId ? '/sessions/' + encodeURIComponent(sessionId) : '') + location.search, { replace: true, state: location.state })
  }, [loading, error, relocated?.id, relocated?.project_id, sessionId, location.search, location.state, navigate])

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
              {t(loading || relocated ? 'Loading work…' : 'Work not found in this project.')}
            </p>
          )}
        </div>
      </div>
    )
  if (!taskId && extensionView) {
    if (loading || error) return (
      <div className="p-5 text-sm" role={error ? 'alert' : 'status'}>
        <p>{error || t('Loading work…')}</p>
        {error && <button className="action-secondary mt-3" onClick={onRetry}>{t('Try again')}</button>}
      </div>
    )
    const View = extensionView.component
    return (
      <div className="flex h-full min-h-0 flex-col">
        <div className="flex min-h-11 shrink-0 gap-2 p-3">
          <button
            className="action-secondary"
            onClick={() => setViewId('list')}
          >
            {t('Tasks')}
          </button>
          {views.map((view) => (
            <button
              key={view.id}
              aria-pressed={view.id === viewId}
              className="action-secondary"
              onClick={() => setViewId(view.id)}
            >
              {view.label}
            </button>
          ))}
        </div>
        <div className="min-h-0 flex-1 overflow-auto p-3">
          <ExtensionBoundary
            resetKey={viewId + ':' + project.id}
            fallback={
              <div>
                <p>{t('This view is unavailable.')}</p>
                <button
                  className="action-secondary"
                  onClick={() => setViewId('list')}
                >
                  {t('Back to tasks')}
                </button>
              </div>
            }
          >
            <View
              projectId={project.id}
              tasks={visible.map((task) => ({
                id: task.id,
                title: task.title,
                status: task.status,
              }))}
              openTask={(id) =>
                navigate(base + '/' + encodeURIComponent(id), {
                  state: { ...location.state, taskReturnTo: base },
                })
              }
            />
          </ExtensionBoundary>
        </div>
      </div>
    )
  }
  return (
    <section
      className="flex h-full min-h-0 flex-col"
      aria-label={t('Project tasks')}
    >
      <div className="flex shrink-0 flex-wrap items-center justify-between gap-3 p-3 sm:p-5">
        {!!views.length && (
          <div className="flex gap-2">
            {views.map((view) => (
              <button
                key={view.id}
                className="action-secondary"
                onClick={() => setViewId(view.id)}
              >
                {view.label}
              </button>
            ))}
          </div>
        )}
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
      <div role="group" aria-label={t('Task filters')} className="flex shrink-0 items-center gap-1.5 overflow-x-auto border-b px-3 pb-2 sm:px-5 [scrollbar-width:none] [-ms-overflow-style:none] [&::-webkit-scrollbar]:hidden">
        {filters.filter(item => item.id === 'all' || item.id === filter || searched.some(item.matches)).map(item => {
          const isSelected = filter === item.id
          const count = searched.filter(item.matches).length
          return (
            <button
              key={item.id}
              type="button"
              aria-pressed={isSelected}
              className={`inline-flex min-h-11 shrink-0 items-center justify-center gap-1.5 rounded-full px-3.5 py-1.5 text-xs font-medium transition-all focus-visible:outline-2 focus-visible:outline-ring ${
                isSelected
                  ? 'bg-primary text-primary-foreground font-semibold shadow-sm'
                  : 'bg-muted/60 text-muted-foreground hover:bg-muted hover:text-foreground border border-border/40'
              }`}
              onClick={() => setFilter(item.id)}
            >
              <span>{t(item.label)}</span>
              <span
                className={`inline-flex min-w-4 items-center justify-center rounded-full px-1.5 text-[10px] font-mono leading-none ${
                  isSelected
                    ? 'bg-primary-foreground/20 text-primary-foreground font-semibold'
                    : 'bg-muted-foreground/15 text-muted-foreground'
                }`}
              >
                {count}
              </span>
            </button>
          )
        })}
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
          <div className="space-y-3 pt-3">
            {(() => {
              const queuedPositions = getQueuedTasksPositions(visible)
              return visible.map((item) => (
                <ItemActions
                  key={item.id}
                  label={item.title}
                  actions={[
                    {
                      label: 'Open task',
                      run: () =>
                        navigate(base + '/' + encodeURIComponent(item.id), {
                          state: { ...location.state, taskReturnTo: base },
                        }),
                    },
                    {
                      label: 'Copy link',
                      run: () => copyItemLink(base + '/' + encodeURIComponent(item.id)),
                    },
                  ]}
                >
                  <TaskItemCard
                    task={item}
                    queuePosition={queuedPositions.get(item.id)}
                    showProject={false}
                    showActions={true}
                    returnTo={base}
                  />
                </ItemActions>
              ))
            })()}
          </div>
        )}
      </div>
    </section>
  )
}
