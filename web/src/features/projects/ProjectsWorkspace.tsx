import { useEffect, useState } from 'react'
import { Link } from 'react-router-dom'
import { Folder, Search, ChevronLeft, Settings, Archive } from 'lucide-react'
import type { Project, Task } from '../../api/types'
import { useLanguage } from '../../context/LanguageContext'
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
}) {
  const { t } = useLanguage()
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
        <div className="shrink-0 border-b p-3 space-y-3">
          <label className="flex min-h-11 items-center gap-2 rounded-lg border px-3">
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
            className="flex gap-1"
            role="group"
            aria-label={t('Project filters')}
          >
            {[false, true].map((value) => (
              <button
                key={String(value)}
                aria-pressed={archived === value}
                onClick={() => setArchived(value)}
                className={`min-h-11 rounded-md px-3 text-xs ${archived === value ? 'bg-accent font-medium' : 'text-muted-foreground hover:bg-muted'}`}
              >
                {t(value ? 'Archived' : 'Active')}
              </button>
            ))}
          </div>
        </div>
        <div className="min-h-0 flex-1 overflow-y-auto p-2">
          {visible.map((item) => (
            <Link
              key={item.id}
              to={`/projects/${encodeURIComponent(item.id)}/tasks`}
              aria-current={project?.id === item.id ? 'page' : undefined}
              className={`flex min-h-16 items-center gap-3 rounded-md p-3 hover:bg-accent ${project?.id === item.id ? 'bg-accent' : ''}`}
            >
              <Folder className="h-4 w-4 shrink-0 text-muted-foreground" />
              <span className="min-w-0 flex-1 truncate text-sm font-medium">
                {item.name}
              </span>
              <span className="text-xs tabular-nums text-muted-foreground">
                {
                  tasks.filter(
                    (task) =>
                      task.project_id === item.id ||
                      task.project_ids?.includes(item.id) ||
                      (item.id === 'default' &&
                        !task.project_id &&
                        !task.project_ids?.length),
                  ).length
                }
              </span>
            </Link>
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
            <div className="flex shrink-0 items-center justify-between gap-2 border-b px-3 py-2 sm:px-5">
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
                {onArchiveProject && (
                  <button
                    className="icon-action"
                    disabled={busy}
                    aria-label={t(
                      project.is_archived
                        ? 'Restore project'
                        : 'Archive project',
                    )}
                    onClick={() => void archive()}
                  >
                    <Archive className="h-4 w-4" />
                  </button>
                )}
                <Link
                  className="icon-action"
                  aria-label={t('Project settings')}
                  to={`/projects/${encodeURIComponent(project.id)}/settings`}
                >
                  <Settings className="h-4 w-4" />
                </Link>
              </div>
            </div>
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
