import { ListSearch, ListTabs, ListTab, ListRowContent } from '../../components/layout/ListControls'
import { ListCreateAction } from '../../components/layout/ListCreateAction'
import { ItemActions, copyItemLink } from '../../components/ui/ItemActions'
import { useEffect, useState, type ReactNode } from 'react'
import { Link, useLocation, useNavigate } from 'react-router-dom'
import { Folder, FolderGit2, ChevronLeft, Archive, MoreHorizontal, Sparkles, Terminal, ExternalLink, Loader2 } from 'lucide-react'
import type { Project, Task } from '../../api/types'
import { tasksApi } from '../../api/tasks'
import { agentsApi } from '../../api/agents'
import { useLanguage } from '../../context/LanguageContext'
import { TaskProgress, taskProgressModel } from '../../components/ui/TaskProgress'
import { ProjectIcon } from './ProjectIcon'
import { ProjectWorkView } from './ProjectWorkView'

export function ProjectsWorkspace({
  projects,
  project,
  tasks = [],
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
  tasks?: Task[]
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
  const location = useLocation()
  const pathSegments = location.pathname.split('/').filter(Boolean)
  const repositoryView = pathSegments[2] === 'repository'
  const environmentView = pathSegments[2] === 'environment'
  const [query, setQuery] = useState('')
  const [archived, setArchived] = useState(!!project?.is_archived)
  useEffect(() => {
    setArchived(!!project?.is_archived)
    setQuery('')
  }, [project?.id, project?.is_archived])
  const [busy, setBusy] = useState(false)
  const [openingTerminalId, setOpeningTerminalId] = useState('')
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

  useEffect(() => {
    const shouldOpen =
      new URLSearchParams(location.search).get('terminal') === 'open' ||
      (location.state as { openTerminal?: boolean } | null)?.openTerminal
    if (shouldOpen && project?.id && !openingTerminalId && !taskId) {
      void handleOpenProjectTerminal(project.id)
    }
  }, [location.search, location.state, project?.id, taskId])

  const handleOpenProjectTerminal = async (projId: string, popout = false) => {
    if (openingTerminalId) return
    setOpeningTerminalId(projId)
    setSaveError('')
    try {
      const targetProj =
        project?.id === projId
          ? project
          : projects.find((p) => p.id === projId)
      const projName = targetProj?.name || 'Workspace'

      // 1. Reconnect to an existing dedicated project terminal task in memory if available
      let targetTask = tasks.find(
        (item) =>
          (item.project_id === projId || item.project_ids?.includes(projId)) &&
          item.title.startsWith('Terminal ·'),
      )

      // 2. Query API if not cached in memory (without restrictive status filter so any previous terminal task is reused)
      if (!targetTask) {
        try {
          const res = await tasksApi.page({
            project_id: projId,
            search: 'Terminal ·',
            page_size: 20,
          })
          targetTask = res.tasks.find((item) =>
            item.title.startsWith('Terminal ·'),
          )
        } catch {}
      }

      // 3. Create dedicated terminal task if none exists yet
      let assignedAgentId = 'default'
      if (!targetTask) {
        try {
          const agentList = await agentsApi.list()
          const available = agentList.filter(
            (a) => a.enabled !== false && a.status !== 'inactive',
          )
          const selected =
            available.find((a) => a.connection_state === 'verified') ||
            available[0]
          if (selected) assignedAgentId = selected.id
        } catch {}

        targetTask = await tasksApi.create({
          title: `Terminal · ${projName}`,
          project_ids: [projId],
          description: `Interactive terminal workspace session with AI agent for ${projName}.`,
          assignee_id: assignedAgentId,
        })
      }

      try {
        localStorage.setItem('cuan_terminal_pin', 'true')
        localStorage.setItem(
          'cuan-task-launch:' + targetTask.id,
          JSON.stringify({
            request_id: crypto.randomUUID(),
            agent_id: assignedAgentId,
            runner_id: 'local_terminal',
          }),
        )
      } catch {}

      const targetSessionId = targetTask.latest_attempt?.id || 'latest'

      if (popout) {
        const popoutUrl = `/tasks/${encodeURIComponent(targetTask.id)}/sessions/${encodeURIComponent(targetSessionId)}?terminal=open&popout=true`
        const winName = `cuan_terminal_${targetTask.id.replace(/[^a-zA-Z0-9_]/g, '_')}`
        const w = 980
        const h = 680
        const left = Math.max(0, Math.round(window.screenX + (window.outerWidth - w) / 2))
        const top = Math.max(0, Math.round(window.screenY + (window.outerHeight - h) / 2))
        window.open(popoutUrl, winName, `width=${w},height=${h},left=${left},top=${top},resizable=yes,scrollbars=no,status=no,toolbar=no,menubar=no,location=no`)
        return
      }

      navigate(
        `/tasks/${encodeURIComponent(targetTask.id)}/sessions/${encodeURIComponent(targetSessionId)}?terminal=open`,
        {
          state: {
            taskReturnTo: `/projects/${encodeURIComponent(projId)}/tasks`,
            openTerminal: true,
          },
        },
      )
    } catch {
      setSaveError(t('Could not load project terminal. Try again.'))
    } finally {
      setOpeningTerminalId('')
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
        <div className="shrink-0 border-b border-border/40 px-4 pt-4">
          <ListCreateAction label="New project" kind="projects" onClick={onNewProject} />
          <ListSearch aria-label={t('Search projects')} placeholder={t('Search projects…')} value={query} onChange={event => setQuery(event.target.value)} />
          <ListTabs className="mt-3 border-b-0" aria-label={t('Project filters')}>
            {[false,true].map(value => <ListTab key={String(value)} selected={archived===value} count={projects.filter(item=>!!item.is_archived===value).length} onClick={()=>setArchived(value)}>{t(value?'Archived':'Active')}</ListTab>)}
          </ListTabs>
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
            <div data-project-row className="relative">
            <Link
              key={item.id}
              to={`/projects/${encodeURIComponent(item.id)}/settings`}
              aria-current={project?.id === item.id ? 'page' : undefined}
              className={`project-list-row group flex rounded-lg min-h-[4.5rem] items-center gap-3 px-3 py-2.5 transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring ${project?.id === item.id ? 'bg-accent' : ''}`}
            >
              <ListRowContent leading={<ProjectIcon repoUrl={item.repo_url} />} title={<span className={`block truncate ${taskProgressModel(item.task_progress || {}) ? 'pl-9' : ''}`}>{item.name}</span>} description={String(item.metadata?.cuan_settings?.goal || '') || (item.repo_url ? t('GitHub connected') : t('No repository needed'))} />
            </Link>
            <div className="absolute left-[50px] top-1 z-10"><TaskProgress {...item.task_progress} /></div>
            </div>
            </ItemActions>
          ))}
          {(error || saveError) && !project && (
            <div role="alert" className="p-3 text-sm">
              <p>{t(error || saveError)}</p>
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
                <TaskProgress {...project.task_progress} />
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
                  disabled={Boolean(openingTerminalId)}
                  onClick={() => void handleOpenProjectTerminal(project.id)}
                  className="action-secondary text-xs px-2.5 py-1 inline-flex items-center gap-1.5"
                  title={t('Open terminal for this project')}
                >
                  {openingTerminalId === project.id ? (
                    <Loader2 className="h-3.5 w-3.5 animate-spin text-emerald-500" />
                  ) : (
                    <Terminal className="h-3.5 w-3.5 text-emerald-500" />
                  )}
                  <span>
                    {openingTerminalId === project.id ? t('Opening…') : t('Terminal')}
                  </span>
                </button>
                <details className="relative">
                  <summary className="icon-action cursor-pointer list-none [&::-webkit-details-marker]:hidden" aria-label={t('Project actions')}><MoreHorizontal className="h-4 w-4" /></summary>
                  <div className="absolute right-0 top-full z-20 mt-1 min-w-56 rounded-lg border bg-popover p-1 text-popover-foreground shadow-lg">
                    <button
                      className="flex min-h-11 w-full items-center gap-2 rounded-md px-3 text-left text-sm hover:bg-accent cursor-pointer disabled:opacity-50"
                      disabled={Boolean(openingTerminalId)}
                      onClick={() => void handleOpenProjectTerminal(project.id, true)}
                    >
                      {openingTerminalId === project.id ? (
                        <Loader2 className="h-4 w-4 animate-spin text-muted-foreground" />
                      ) : (
                        <ExternalLink className="h-4 w-4 text-muted-foreground" />
                      )}
                      {t('Buka terminal di jendela terpisah')}
                    </button>
                    {onArchiveProject && <button className="flex min-h-11 w-full items-center gap-2 rounded-md px-3 text-left text-sm hover:bg-accent cursor-pointer" disabled={busy} onClick={() => void archive()}><Archive className="h-4 w-4" />{t(project.is_archived ? 'Restore project' : 'Archive project')}</button>}
                  </div>
                </details>
              </div>
            </div>}
            {!taskId && <nav aria-label={t('Project navigation')} className="flex shrink-0 gap-5 border-b bg-transparent px-4 sm:px-5">
              {[
                {label: 'Settings', path: 'settings', active: !!settingsContent && !repositoryView && !environmentView},
                {label: 'Environment', path: 'environment', active: !!settingsContent && environmentView},
                ...(project.id !== 'default' ? [{label: 'Repository', path: 'repository', active: !!settingsContent && repositoryView}] : []),
                {label: 'Activity', path: 'tasks', active: !settingsContent}
              ].map(item => <Link
                key={item.path}
                to={`/projects/${encodeURIComponent(project.id)}/${item.path}`}
                aria-current={item.active ? 'page' : undefined}
                className={`inline-flex min-h-11 items-center border-b-2 text-sm bg-transparent transition-colors focus-visible:outline focus-visible:outline-2 focus-visible:outline-ring ${item.active ? 'border-primary font-medium text-primary' : 'border-transparent text-muted-foreground hover:text-foreground'}`}
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
