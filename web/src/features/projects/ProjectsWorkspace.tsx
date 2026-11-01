import { ListCreateAction } from '../../components/layout/ListCreateAction'
import { ItemActions, copyItemLink } from '../../components/ui/ItemActions'
import { useEffect, useState, type ReactNode } from 'react'
import { Link, useLocation, useNavigate } from 'react-router-dom'
import { Folder, FolderGit2, Search, ChevronLeft, Archive, MoreHorizontal, Sparkles, Terminal } from 'lucide-react'
import type { Project, Task } from '../../api/types'
import { tasksApi } from '../../api/tasks'
import { agentsApi } from '../../api/agents'
import { workStatus } from '../tasks/task-ui'
import { useLanguage } from '../../context/LanguageContext'
import { ProjectIcon } from './ProjectIcon'
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
  settingsContent,
  onNewProject,
}: {
  onNewProject?: () => void
  settingsContent?: ReactNode
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
  const repositoryView = useLocation().pathname.split('/').filter(Boolean)[2] === 'repository'
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

  const handleOpenProjectTerminal = async (projId: string) => {
    const projTasks = tasks.filter((item) => item.project_id === projId)
    // 1. Reconnect to an already active dedicated project terminal session if alive
    const activeTerminalTask = projTasks.find(
      (item) =>
        item.title.startsWith('Terminal ·') &&
        (['Working', 'Waiting to start'].includes(workStatus(item)) ||
          ['running', 'queued'].includes(item.latest_attempt?.status || '')),
    )
    if (activeTerminalTask) {
      try {
        localStorage.setItem('cuan_terminal_pin', 'true')
      } catch {}
      navigate(
        `/tasks/${encodeURIComponent(activeTerminalTask.id)}/sessions/latest?terminal=open`,
        {
          state: {
            taskReturnTo: `/projects/${encodeURIComponent(projId)}/tasks`,
            openTerminal: true,
          },
        },
      )
      return
    }

    // 2. Independent from regular tasks / old completed tasks. Always launch a dedicated interactive terminal with agent.
    try {
      setBusy(true)
      let agentId = 'default'
      try {
        const agentList = await agentsApi.list()
        const available = agentList.filter(
          (a) => a.enabled !== false && a.status !== 'inactive',
        )
        const selected =
          available.find((a) => a.connection_state === 'verified') ||
          available[0]
        if (selected) agentId = selected.id
      } catch {}

      const targetProj =
        project?.id === projId
          ? project
          : projects.find((p) => p.id === projId)
      const projName = targetProj?.name || 'Workspace'
      const created = await tasksApi.create({
        title: `Terminal · ${projName}`,
        project_ids: [projId],
        description: `Interactive terminal workspace session with AI agent for ${projName}.`,
      })

      try {
        localStorage.setItem('cuan_terminal_pin', 'true')
      } catch {}

      const launchPayload = {
        request_id: crypto.randomUUID(),
        agent_id: agentId,
      }

      try {
        localStorage.setItem(
          'cuan-task-launch:' + created.id,
          JSON.stringify(launchPayload),
        )
      } catch {}

      try {
        const res = await tasksApi.runAgent(created.id, launchPayload)
        if (res?.attempt?.id) {
          navigate(
            `/tasks/${encodeURIComponent(created.id)}/sessions/${encodeURIComponent(res.attempt.id)}?terminal=open`,
            {
              state: {
                taskReturnTo: `/projects/${encodeURIComponent(projId)}/tasks`,
                openTerminal: true,
              },
            },
          )
          return
        }
      } catch {}

      navigate(
        `/tasks/${encodeURIComponent(created.id)}/sessions/start?terminal=open`,
        {
          state: {
            taskReturnTo: `/projects/${encodeURIComponent(projId)}/tasks`,
            startRequestId: launchPayload.request_id,
            openTerminal: true,
          },
        },
      )
    } catch {
      navigate(`/projects/${encodeURIComponent(projId)}/tasks`)
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
          <ListCreateAction label="New project" kind="projects" onClick={onNewProject} />
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
              { label: 'Open project', run: () => navigate('/projects/' + encodeURIComponent(item.id) + '/settings') },
              { label: 'Activity', run: () => navigate('/projects/' + encodeURIComponent(item.id) + '/tasks') },
              { label: 'Terminal', run: () => void handleOpenProjectTerminal(item.id) },
              { label: 'Copy link', run: () => copyItemLink('/projects/' + encodeURIComponent(item.id) + '/settings') },
              ...(onArchiveProject ? [{ label: item.is_archived ? 'Restore project' : 'Archive project', run: () => onArchiveProject(item.id, !item.is_archived) }] : []),
            ]}>
            <Link
              key={item.id}
              to={`/projects/${encodeURIComponent(item.id)}/settings`}
              aria-current={project?.id === item.id ? 'page' : undefined}
              className={`project-list-row group flex rounded-lg min-h-[4.5rem] items-center gap-3 px-3 py-2.5 transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring ${project?.id === item.id ? 'bg-accent' : ''}`}
            >
              <ProjectIcon repoUrl={item.repo_url} />
              <span className="min-w-0 flex-1">
                <span className="block truncate text-sm font-medium">{item.name}</span>
                <span className="mt-1 block truncate text-xs text-muted-foreground">
                  {String(item.metadata?.cuan_settings?.goal || '') || (item.repo_url ? t('GitHub connected') : t('No repository needed'))}
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
                {project.repo_url ? (
                  <FolderGit2 className="h-4 w-4 text-primary shrink-0" />
                ) : (
                  <Folder className="h-4 w-4 text-muted-foreground shrink-0" />
                )}
                <h2 className="truncate text-sm font-semibold">
                  {project.name}
                </h2>
                {project.is_archived && (
                  <span className="text-xs text-muted-foreground">
                    {t('Archived')}
                  </span>
                )}
              </div>
              <div className="flex shrink-0 items-center gap-1.5 sm:gap-2">
                <button
                  type="button"
                  onClick={() => void handleOpenProjectTerminal(project.id)}
                  className="action-secondary text-xs px-2.5 py-1 inline-flex items-center gap-1.5"
                  title={t('Open terminal for this project')}
                >
                  <Terminal className="h-3.5 w-3.5 text-emerald-500" />
                  <span>{t('Terminal')}</span>
                </button>
                <details className="relative">
                  <summary className="icon-action cursor-pointer list-none [&::-webkit-details-marker]:hidden" aria-label={t('Project actions')}><MoreHorizontal className="h-4 w-4" /></summary>
                  <div className="absolute right-0 top-full z-20 mt-1 min-w-48 rounded-lg border bg-popover p-1 text-popover-foreground shadow-lg">
                    {onArchiveProject && <button className="flex min-h-11 w-full items-center gap-2 rounded-md px-3 text-left text-sm hover:bg-accent" disabled={busy} onClick={() => void archive()}><Archive className="h-4 w-4" />{t(project.is_archived ? 'Restore project' : 'Archive project')}</button>}
                  </div>
                </details>
              </div>
            </div>}
            {!taskId && <nav aria-label={t('Project navigation')} className="flex shrink-0 gap-5 border-b px-4 sm:px-5">
              {[{label: 'Settings', path: 'settings', active: !!settingsContent && !repositoryView}, ...(project.id !== 'default' ? [{label: 'Repository', path: 'repository', active: !!settingsContent && repositoryView}] : []), {label: 'Activity', path: 'tasks', active: !settingsContent}].map(item => <Link
                key={item.path}
                to={`/projects/${encodeURIComponent(project.id)}/${item.path}`}
                aria-current={item.active ? 'page' : undefined}
                className={`inline-flex min-h-11 items-center border-b-2 text-sm transition-colors focus-visible:outline focus-visible:outline-2 focus-visible:outline-ring ${item.active ? 'border-primary font-medium text-primary' : 'border-transparent text-muted-foreground hover:text-foreground'}`}
              >{t(item.label)}</Link>)}
            </nav>}
            {saveError && (
              <p role="alert" className="p-3 text-sm text-destructive">
                {t(saveError)}
              </p>
            )}
            <div className={`min-h-0 flex-1 ${settingsContent ? 'overflow-y-auto' : 'overflow-hidden'}`}>
              {settingsContent || <ProjectWorkView
                project={project}
                tasks={tasks}
                taskId={taskId}
                sessionId={sessionId}
                loading={loading}
                error={error}
                onRetry={onRetry}
              />}
            </div>
          </>
        ) : (
          <div className="flex h-full flex-col items-center justify-center gap-3 p-6 text-center text-sm text-muted-foreground">
            <Folder className="h-8 w-8 text-muted-foreground/60" />
            <p className="max-w-xs">{t('Select a project from the left or create a new workspace.')}</p>
            {onNewProject && (
              <button
                type="button"
                id="projects-workspace-empty-create-btn"
                onClick={onNewProject}
                className="action-secondary inline-flex items-center gap-1.5 text-xs py-1.5 px-3 mt-1"
              >
                <Sparkles className="h-3.5 w-3.5 text-amber-500" />
                <span>{t('Create project')}</span>
              </button>
            )}
          </div>
        )}
      </div>
    </section>
  )
}
