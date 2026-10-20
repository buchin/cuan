import { ItemActions, copyItemLink } from '../../components/ui/ItemActions'
import { useEffect, useMemo, useState } from 'react'
import { Link, useNavigate } from 'react-router-dom'
import { Folder, Search, ChevronLeft, Settings, Archive, MoreHorizontal, Plus } from 'lucide-react'
import type { Project, Task } from '../../api/types'
import { useLanguage } from '../../context/LanguageContext'
import { tasksForProject, workStatus } from '../tasks/task-ui'
import { ProjectWorkView } from './ProjectWorkView'

export function ProjectsWorkspace({
  projects,
  project,
  tasks,
  taskId,
  sessionId,
  loading,
  error,
  onRetry,
  onArchiveProject,
  onNewTask,
}: {
  projects: Project[]
  project?: Project
  tasks: Task[]
  taskId?: string
  sessionId?: string
  loading: boolean
  error: string
  onRetry: () => void
  onArchiveProject?: (id: string, archived: boolean) => Promise<void>
  onNewTask?: () => void
}) {
  const { t } = useLanguage()
  const navigate = useNavigate()
  const [query, setQuery] = useState('')
  const [archived, setArchived] = useState(!!project?.is_archived)
  useEffect(() => {
    setArchived(!!project?.is_archived)
    setQuery('')
  }, [project?.id, project?.is_archived])
  const [busy, setBusy] = useState(false)
  const [saveError, setSaveError] = useState('')
  const visible = projects.filter(
    (item) =>
      !!item.is_archived === archived &&
      item.name.toLowerCase().includes(query.toLowerCase().trim()),
  )
  const summaries = useMemo(() => new Map(projects.map((item) => {
    const work = tasksForProject(tasks, item.id)
    const states = work.map(workStatus)
    const latest = [item.updated_at, ...work.map((task) => task.updated_at)]
      .filter((value) => Number.isFinite(Date.parse(value)))
      .sort((a, b) => Date.parse(b) - Date.parse(a))[0]
    return [item.id, {
      count: work.length,
      blocked: states.filter((state) => state === 'Needs attention' || state === 'Not running').length,
      working: states.filter((state) => state === 'Working').length,
      review: states.filter((state) => state === 'Ready for review').length,
      completed: states.filter((state) => state === 'Completed').length,
      stopped: states.filter((state) => state === 'Not running').length,
      latest,
    }]
  })), [projects, tasks])
  async function archive() {
    if (!project || !onArchiveProject || busy) return
    if (
      !confirm(
        `${t(project.is_archived ? 'Restore project' : 'Archive project')} “${project.name}”?`,
      )
    )
      return
    setBusy(true)
    setSaveError('')
    try {
      await onArchiveProject(project.id, !project.is_archived)
    } catch {
      setSaveError(
        'Could not save changes. Try again; your project data is still available.',
      )
    } finally {
      setBusy(false)
    }
  }
  return (
    <section
      className="flex h-full min-h-0 min-w-0 overflow-hidden"
      aria-label={t('Projects')}
    >
      <aside
        aria-label={t('Project list')}
        className={`min-h-0 w-full min-w-0 flex-col min-[1200px]:w-72 min-[1200px]:shrink-0 min-[1200px]:border-r ${project ? 'hidden min-[1200px]:flex' : 'flex'}`}
      >
        <div className="shrink-0 border-b px-3 pt-3">
          <label className="flex h-10 items-center gap-2 rounded-md border bg-muted/20 px-3 focus-within:ring-2 focus-within:ring-ring">
            <Search className="h-4 w-4 shrink-0 text-muted-foreground" />
            <input
              aria-label={t('Search projects')}
              placeholder={t('Search projects…')}
              value={query}
              onChange={(event) => setQuery(event.target.value)}
              className="w-full min-w-0 bg-transparent text-base outline-none sm:text-sm"
            />
          </label>
          <div
            className="mt-1 flex gap-4"
            role="group"
            aria-label={t('Project filters')}
          >
            {[false, true].map((value) => (
              <button
                key={String(value)}
                aria-pressed={archived === value}
                onClick={() => setArchived(value)}
                className={`min-h-11 border-b-2 px-1 text-xs transition-colors ${archived === value ? 'border-foreground font-medium text-foreground' : 'border-transparent text-muted-foreground hover:text-foreground'}`}
              >
                {t(value ? 'Archived' : 'Active')}
                <span className="ml-1.5 text-[11px] tabular-nums text-muted-foreground">{projects.filter((item) => !!item.is_archived === value).length}</span>
              </button>
            ))}
          </div>
        </div>
        <div className="min-h-0 flex-1 overflow-y-auto p-2">
          {visible.map((item) => (
            <ItemActions key={item.id} label={item.name} actions={[
              { label: 'Open project', run: () => navigate('/projects/' + encodeURIComponent(item.id) + '/tasks') },
              { label: 'Project settings', run: () => navigate('/projects/' + encodeURIComponent(item.id) + '/settings') },
              { label: 'Copy link', run: () => copyItemLink('/projects/' + encodeURIComponent(item.id) + '/tasks') },
              ...(onArchiveProject ? [{ label: item.is_archived ? 'Restore project' : 'Archive project', run: () => onArchiveProject(item.id, !item.is_archived) }] : []),
            ]}>
            <Link
              key={item.id}
              to={`/projects/${encodeURIComponent(item.id)}/tasks`}
              aria-current={project?.id === item.id ? 'page' : undefined}
              className={`group flex min-h-16 items-center gap-2 rounded-[6px] px-2 py-2 transition-colors hover:bg-accent focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring ${project?.id === item.id ? 'bg-accent' : ''}`}
            >
              <span className="flex h-9 w-9 shrink-0 items-center justify-center rounded-lg bg-muted text-muted-foreground"><Folder className="h-4 w-4" /></span>
              <span className="min-w-0 flex-1">
                <span className="block truncate text-sm font-medium">{item.name}</span>
                <span className="mt-1 block truncate text-xs text-muted-foreground">
                  {summaries.get(item.id)?.count || 0} {t(summaries.get(item.id)?.count === 1 ? 'task' : 'tasks')}
                  {!!summaries.get(item.id)?.blocked ? <span className="text-amber-700 dark:text-amber-400"> · {summaries.get(item.id)?.blocked} {t('needs attention')}</span>
                    : !!summaries.get(item.id)?.working ? <span className="text-primary"> · {summaries.get(item.id)?.working} {t('working')}</span>
                    : !!summaries.get(item.id)?.review ? <> · {summaries.get(item.id)?.review} {t('in review')}</>
                    : !!summaries.get(item.id)?.stopped ? <> · {summaries.get(item.id)?.stopped} {t('not running')}</>
                    : !!summaries.get(item.id)?.completed ? <> · {summaries.get(item.id)?.completed} {t('completed')}</> : null}
                </span>
              </span>
            </Link>
            </ItemActions>
          ))}
          {error && !project && (
            <div role="alert" className="p-3 text-sm">
              <p>{t(error)}</p>
              <button className="action-secondary mt-2" onClick={onRetry}>
                {t('Try again')}
              </button>
            </div>
          )}
          {loading && !projects.length && (
            <p role="status" className="p-3 text-sm">
              {t('Loading…')}
            </p>
          )}
          {!loading && !error && !visible.length && (
            <p className="p-4 text-sm text-muted-foreground">
              {t(
                query
                  ? 'No matching projects.'
                  : archived
                    ? 'No archived projects.'
                    : 'No projects yet.',
              )}
            </p>
          )}
        </div>
      </aside>
      <div
        className={`min-h-0 min-w-0 flex-1 flex-col ${project ? 'flex' : 'hidden min-[1200px]:flex'}`}
      >
        {project ? (
          <>
            {!taskId && <div className="flex shrink-0 items-center justify-between gap-2 border-b px-3 py-2 sm:px-5">
              <div className="flex min-w-0 items-center gap-2">
                <Link
                  to="/projects"
                  className="icon-action min-[1200px]:hidden"
                  aria-label={t('Back to projects')}
                >
                  <ChevronLeft className="h-4 w-4" />
                </Link>
                <h2 className="truncate text-sm font-semibold">
                  {project.name}
                </h2>
                {project.is_archived && (
                  <span className="text-xs text-muted-foreground">
                    {t('Archived')}
                  </span>
                )}
              </div>
              <div className="flex shrink-0 items-center gap-1">
                {onNewTask && !project.is_archived && <button className="action-primary min-h-11 gap-1.5 px-3 text-sm" onClick={onNewTask}><Plus className="h-4 w-4" />{t('New task')}</button>}
                <details className="relative">
                  <summary className="icon-action cursor-pointer list-none [&::-webkit-details-marker]:hidden" aria-label={t('Project actions')}><MoreHorizontal className="h-4 w-4" /></summary>
                  <div className="absolute right-0 top-full z-20 mt-1 min-w-48 rounded-lg border bg-popover p-1 text-popover-foreground shadow-lg">
                    <Link className="flex min-h-11 items-center gap-2 rounded-md px-3 text-sm hover:bg-accent" to={`/projects/${encodeURIComponent(project.id)}/settings`}><Settings className="h-4 w-4" />{t('Project settings')}</Link>
                    {onArchiveProject && <button className="flex min-h-11 w-full items-center gap-2 rounded-md px-3 text-left text-sm hover:bg-accent" disabled={busy} onClick={() => void archive()}><Archive className="h-4 w-4" />{t(project.is_archived ? 'Restore project' : 'Archive project')}</button>}
                  </div>
                </details>
              </div>
            </div>}
            {saveError && (
              <p role="alert" className="p-3 text-sm text-destructive">
                {t(saveError)}
              </p>
            )}
            <div className="min-h-0 flex-1 overflow-hidden">
              <ProjectWorkView
                project={project}
                tasks={tasks}
                taskId={taskId}
                sessionId={sessionId}
                loading={loading}
                error={error}
                onRetry={onRetry}
              />
            </div>
          </>
        ) : (
          <div className="flex h-full items-center justify-center gap-3 p-6 text-sm text-muted-foreground">
            <Folder className="h-6 w-6" />
            <p>{t('Select a project to view its tasks.')}</p>
          </div>
        )}
      </div>
    </section>
  )
}
