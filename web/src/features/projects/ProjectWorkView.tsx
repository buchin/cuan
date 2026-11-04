import { useTaskPage } from '../tasks/useTaskPage'
import { TaskPagePager } from '../tasks/TaskPagePager'
import { ListSearch, ListTabs, ListTab } from '../../components/layout/ListControls'
import { ItemActions, copyItemLink } from '../../components/ui/ItemActions'
import {
  ExtensionBoundary,
  projectViews,
  useExtensionRevision,
} from '../../extensions/sdk'
import { useLanguage } from '../../context/LanguageContext'
import { useEffect, useState } from 'react'
import { Link, useLocation, useNavigate } from 'react-router-dom'
import { AlertCircle, ArrowLeft, RefreshCw } from 'lucide-react'
import type { Project, Task } from '../../api/types'
import { tasksApi } from '../../api/tasks'
import { TaskSessionPage } from '../tasks/TaskSessionPage'
import { TaskStartingPage } from '../tasks/TaskStartingPage'
import { TaskItemCard } from '../tasks/TaskItemCard'
import { workStatus, taskRef, taskSummaryText } from '../tasks/task-ui'

export function ProjectWorkView({
  project,
  tasks,
  taskId,
  sessionId,
  loading: projectLoading,
  error: projectError,
  onRetry: retryProject,
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
  const [page, setPage] = useState(1)
  const [extensionRevision, setExtensionRevision] = useState(0)
  useEffect(() => setPage(1), [project.id, query, filter])
  const result = useTaskPage({ project_id: project.id, search: query, status: filter, page, page_size: 25, exclude_terminals: true })
  const loading = projectLoading || result.loading
  const error = projectError || result.error
  const onRetry = () => { retryProject(); result.refresh(); setExtensionRevision(value => value + 1) }
  const scoped = result.data?.tasks || []
  const [extensionTasks, setExtensionTasks] = useState<Task[] | null>(null)
  const [extensionError, setExtensionError] = useState('')
  useEffect(() => {
    if (!extensionView) { setExtensionTasks(null); setExtensionError(''); return }
    let current = true
    void tasksApi.list(project.id).then(items => { if (current) { setExtensionTasks(items); setExtensionError('') } }).catch(() => { if (current) setExtensionError('Could not load this project view. Try again.') })
    return () => { current = false }
  }, [extensionView?.id, project.id, result.data, extensionRevision])
  const visible = extensionView ? (extensionTasks || []).filter(task => !task.title.startsWith('Terminal ·')).filter(task =>
    [task.title, taskRef(task.id), task.status, workStatus(task), taskSummaryText(task), task.latest_attempt?.agent_name].join(' ').toLowerCase().includes(query.toLowerCase().trim()) &&
    (filter === 'all' || (filter === 'work_active' && ['Working','Waiting to start','Not started'].includes(workStatus(task))) || (filter === 'attention' && ['Needs attention','Not running','Ready for review'].includes(workStatus(task))) || (filter === 'completed' && ['Completed','Answered'].includes(workStatus(task))))) : scoped
  const counts = result.counts
  const total = result.data?.total || 0
  const currentPage = result.data?.page || page
  const pager = <TaskPagePager className="shrink-0 px-3 sm:px-5" page={currentPage} pageSize={25} total={total} onPage={setPage} />
  const filters = [{id:'all',label:'All'}, {id:'work_active',label:'Active'}, {id:'attention',label:'Needs attention'}, {id:'completed',label:'Completed'}]
  const selected = scoped.find((t) => t.id === taskId)
  const relocated = taskId && !selected ? tasks.find(task => task.id === taskId && task.project_id && task.project_id !== project.id) : undefined

  const isDetailView = ['detail', 'full'].includes(
    new URLSearchParams(location.search).get('view') || '',
  )
  const backURL =
    typeof location.state?.taskReturnTo === 'string' &&
    /^\/(?:tasks|projects\/[^/?#]+(?:\/tasks)?|inbox\/[^/?#]+)(?:\?[^#]*)?$/.test(
      location.state.taskReturnTo,
    )
      ? location.state.taskReturnTo
      : `/projects/${encodeURIComponent(project.id)}/tasks`

  useEffect(() => {
    if (taskId) {
      navigate(`/tasks/${encodeURIComponent(taskId)}${sessionId ? '/sessions/' + encodeURIComponent(sessionId) : ''}${location.search}`, {
        replace: true,
        state: { ...location.state, taskReturnTo: backURL },
      })
    }
  }, [taskId, sessionId, location.search, location.state, backURL, navigate])

  const [directTask, setDirectTask] = useState<Task | null>(null)
  const [directLoading, setDirectLoading] = useState(false)
  const [directError, setDirectError] = useState('')

  useEffect(() => {
    if (!taskId || selected || isDetailView) return
    let active = true
    setDirectLoading(true)
    setDirectError('')
    tasksApi.get(taskId)
      .then((t) => {
        if (!active) return
        setDirectTask(t)
        if (t.project_id && t.project_id !== project.id) {
          const target = `/projects/${encodeURIComponent(t.project_id)}/tasks/${encodeURIComponent(t.id)}`
          navigate(target + (sessionId ? '/sessions/' + encodeURIComponent(sessionId) : '') + location.search, {
            replace: true,
            state: location.state,
          })
        }
      })
      .catch((err) => {
        if (active) {
          setDirectError(err instanceof Error ? err.message : 'Work not found or session ended.')
        }
      })
      .finally(() => {
        if (active) setDirectLoading(false)
      })
    return () => {
      active = false
    }
  }, [taskId, selected, project.id, sessionId, location.search, location.state, navigate, isDetailView])

  useEffect(() => {
    if (loading || error || !relocated) return
    const target = '/projects/' + encodeURIComponent(relocated.project_id!) + '/tasks/' + encodeURIComponent(relocated.id)
    navigate(target + (sessionId ? '/sessions/' + encodeURIComponent(sessionId) : '') + location.search, { replace: true, state: location.state })
  }, [loading, error, relocated?.id, relocated?.project_id, sessionId, location.search, location.state, navigate])

  if (taskId && !isDetailView) {
    const activeTask = selected || directTask
    const isFetching = (loading || directLoading) && !activeTask
    const hasError = !isFetching && !activeTask

    return (
      <div className="flex h-full min-h-0 flex-col bg-background">
        {hasError ? (
          <div className="flex flex-col h-full">
            <header className="flex h-11 sm:h-12 shrink-0 items-center justify-between border-b bg-background px-3 gap-2">
              <Link
                to={backURL}
                state={location.state}
                className="icon-action shrink-0"
                aria-label={t('Back')}
                title={t('Back')}
              >
                <ArrowLeft className="h-4 w-4" />
              </Link>
              <span className="text-xs font-semibold px-2 py-0.5 rounded-full border border-destructive/20 bg-destructive/10 text-destructive">
                {t('Not Found')}
              </span>
            </header>

            <div className="flex flex-1 items-center justify-center p-4">
              <div
                role="alert"
                className="flex flex-col items-center justify-center max-w-sm rounded-xl border border-border bg-card p-6 text-center text-card-foreground shadow-sm space-y-3"
              >
                <div className="flex h-12 w-12 items-center justify-center rounded-full bg-destructive/10 text-destructive">
                  <AlertCircle className="h-6 w-6" />
                </div>
                <div className="space-y-1">
                  <h3 className="text-sm font-semibold text-foreground">
                    {t('Work or session not found')}
                  </h3>
                  <p className="text-xs text-muted-foreground">
                    {directError || t('The requested task was not found in this project or its terminal session has ended.')}
                  </p>
                </div>
                <div className="flex flex-wrap items-center justify-center gap-2 pt-2">
                  <Link
                    to={backURL}
                    state={location.state}
                    className="action-primary text-xs px-3 py-1.5"
                  >
                    {t('Back to project')}
                  </Link>
                  <Link
                    to="/tasks"
                    className="action-secondary text-xs px-3 py-1.5"
                  >
                    {t('All tasks')}
                  </Link>
                  <button
                    type="button"
                    onClick={() => {
                      setDirectError('')
                      setDirectTask(null)
                      onRetry?.()
                    }}
                    className="action-secondary text-xs px-3 py-1.5 inline-flex items-center gap-1"
                  >
                    <RefreshCw className="h-3 w-3" />
                    <span>{t('Try again')}</span>
                  </button>
                </div>
              </div>
            </div>
          </div>
        ) : isFetching ? (
          <div className="flex flex-col h-full">
            <header className="flex h-11 sm:h-12 shrink-0 items-center border-b bg-background px-3 gap-2">
              <Link to={backURL} state={location.state} className="icon-action shrink-0" aria-label={t('Back')}>
                <ArrowLeft className="h-4 w-4" />
              </Link>
              <span className="text-xs text-muted-foreground">{t('Loading work…')}</span>
            </header>
            <div className="flex flex-1 items-center justify-center p-4 text-xs text-muted-foreground">
              <RefreshCw className="h-4 w-4 animate-spin mr-2" />
              <span>{t('Loading session…')}</span>
            </div>
          </div>
        ) : (
          <div className="min-h-0 flex-1">
            {sessionId === 'start' ? (
              <TaskStartingPage key={taskId} taskId={taskId} />
            ) : (
              <TaskSessionPage
                key={taskId}
                taskId={taskId}
                sessionId={sessionId || 'latest'}
              />
            )}
          </div>
        )}
      </div>
    )
  }
  if (!taskId && extensionView) {
    if (loading || error || extensionError || !extensionTasks) return (
      <div className="p-5 text-sm" role={error || extensionError ? 'alert' : 'status'}>
        <p>{error || extensionError || t('Loading work…')}</p>
        {(error || extensionError) && <button className="action-secondary mt-3" onClick={onRetry}>{t('Try again')}</button>}
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
            {result.scopeTotal}
          </span>
        </h3>
        <ListSearch containerClassName="sm:max-w-64" aria-label={t('Search project work')} placeholder={t('Search tasks…')} value={query} onChange={event=>setQuery(event.target.value)} />
      </div>
      <ListTabs aria-label={t('Task filters')} className="px-4">
        {filters.filter(item=>item.id==='all'||item.id===filter||(counts[item.id] || 0)>0).map(item=><ListTab key={item.id} selected={filter===item.id} count={counts[item.id] || 0} onClick={()=>setFilter(item.id)}>{t(item.label)}</ListTab>)}
      </ListTabs>
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
              return visible.map((item) => (
                <ItemActions
                  key={item.id}
                  label={item.title}
                  actions={[
                    {
                      label: 'Open task',
                      run: () =>
                        navigate('/tasks/' + encodeURIComponent(item.id), {
                          state: { ...location.state, taskReturnTo: base },
                        }),
                    },
                    {
                      label: 'Copy link',
                      run: () => copyItemLink('/tasks/' + encodeURIComponent(item.id)),
                    },
                  ]}
                >
                  <TaskItemCard
                    task={item}
                    managedUpdates
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
      {pager}
    </section>
  )
}
