import { useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { Project, Task } from '../../api/types'
import { tasksApi } from '../../api/tasks'
import { agentsApi } from '../../api/agents'
import { workStatus } from '../tasks/task-ui'
import { useLanguage } from '../../context/LanguageContext'
import { HairlineList, HairlineListItem } from '../../components/layout/HairlineList'
import { StatusDot } from '../../components/ui/StatusDot'
import { SlideDrawer } from '../../components/layout/SlideDrawer'
import {
  FolderGit2,
  GitBranch,
  ArrowRight,
  Archive,
  Sparkles,
  Terminal,
} from 'lucide-react'

interface ProjectsViewProps {
  projects: Project[]
  tasks?: Task[]
  onSelectProject: (project: Project) => void
  onArchiveProject?: (id: string, isArchived: boolean) => Promise<void>
  onCreateProject?: () => void
}

export function ProjectsView({
  projects,
  tasks = [],
  onSelectProject,
  onArchiveProject,
  onCreateProject,
}: ProjectsViewProps) {
  const [activeTab, setActiveTab] = useState<'active' | 'archived'>('active')
  const [search, setSearch] = useState('')
  const [busy, setBusy] = useState('')
  const [error, setError] = useState('')
  const [drawerProject, setDrawerProject] = useState<Project | null>(null)
  const navigate = useNavigate()
  const { t } = useLanguage()

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
        `/projects/${encodeURIComponent(projId)}/tasks/${encodeURIComponent(activeTerminalTask.id)}/sessions/latest?terminal=open`,
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
      setBusy(projId)
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

      const targetProj = projects.find((p) => p.id === projId)
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
            `/projects/${encodeURIComponent(projId)}/tasks/${encodeURIComponent(created.id)}/sessions/${encodeURIComponent(res.attempt.id)}?terminal=open`,
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
        `/projects/${encodeURIComponent(projId)}/tasks/${encodeURIComponent(created.id)}/sessions/start?terminal=open`,
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
      setBusy('')
    }
  }

  const archive = async (project: Project) => {
    if (
      !onArchiveProject ||
      busy ||
      !confirm(
        `${project.is_archived ? 'Restore' : 'Archive'} project "${project.name}"?`,
      )
    )
      return
    setBusy(project.id)
    setError('')
    try {
      await onArchiveProject(project.id, !project.is_archived)
      if (drawerProject?.id === project.id) {
        setDrawerProject((prev) =>
          prev ? { ...prev, is_archived: !prev.is_archived } : null,
        )
      }
    } catch {
      setError(
        'Could not save changes. Try again; your project data is still available.',
      )
    } finally {
      setBusy('')
    }
  }

  const activeProjects = projects.filter((p) => !p.is_archived)
  const archivedProjects = projects.filter((p) => p.is_archived)

  const displayedProjects = (
    activeTab === 'active' ? activeProjects : archivedProjects
  ).filter(
    (p) =>
      (p.name || '').toLowerCase().includes(search.toLowerCase()) ||
      (p.repo_url && p.repo_url.toLowerCase().includes(search.toLowerCase())),
  )

  const getTaskCountForProject = (projectId: string) => {
    return tasks.filter(
      (t) => t.project_id === projectId || t.project_ids?.includes(projectId),
    ).length
  }

  const getProjectStatus = (projectId: string) => {
    const projTasks = tasks.filter(
      (t) => t.project_id === projectId || t.project_ids?.includes(projectId),
    )
    if (projTasks.length === 0) {
      return { status: 'neutral' as const, label: t('No tasks yet'), pulse: false }
    }
    const hasRunning = projTasks.some(
      (t) =>
        t.latest_attempt?.status === 'running' ||
        workStatus(t) === 'Working' ||
        t.latest_attempt?.status === 'queued',
    )
    if (hasRunning) {
      return { status: 'running' as const, label: t('Agent working'), pulse: true }
    }
    const hasFailed = projTasks.some(
      (t) =>
        t.latest_attempt?.status === 'failed' ||
        workStatus(t) === 'Needs attention',
    )
    if (hasFailed) {
      return { status: 'exception' as const, label: t('Needs attention'), pulse: false }
    }
    return { status: 'healthy' as const, label: t('Active & ready'), pulse: false }
  }

  return (
    <div className="max-w-4xl mx-auto space-y-5">
      {error && (
        <p role="alert" className="text-sm text-destructive">
          {error}
        </p>
      )}

      {/* Search Bar */}
      <div className="flex items-center gap-2">
        <input
          type="search"
          aria-label="Search projects"
          placeholder={t('Search projects…')}
          value={search}
          onChange={(e) => setSearch(e.target.value)}
          className="field-control min-w-0 flex-1"
        />
      </div>

      {search.trim() && displayedProjects.length > 0 && (
        <div className="flex items-center justify-between gap-3 text-sm">
          <span className="text-muted-foreground">
            {displayedProjects.length} {t('projects')}
          </span>
          <button className="action-secondary" onClick={() => setSearch('')}>
            {t('Clear search')}
          </button>
        </div>
      )}

      {/* Category Filter Chips */}
      <nav
        aria-label="Project tabs"
        className="flex items-center gap-1.5 overflow-x-auto py-1 [scrollbar-width:none] [-ms-overflow-style:none] [&::-webkit-scrollbar]:hidden"
      >
        <button
          type="button"
          aria-pressed={activeTab === 'active'}
          onClick={() => setActiveTab('active')}
          className={`inline-flex min-h-11 shrink-0 items-center justify-center gap-1.5 rounded-full px-3.5 py-1.5 text-xs font-medium transition-all focus-visible:outline-2 focus-visible:outline-ring ${
            activeTab === 'active'
              ? 'bg-primary text-primary-foreground font-semibold shadow-sm'
              : 'bg-muted/60 text-muted-foreground hover:bg-muted hover:text-foreground border border-border/40'
          }`}
        >
          <span>{t('Active Projects')}</span>
          <span
            className={`inline-flex min-w-4 items-center justify-center rounded-full px-1.5 text-[10px] font-mono leading-none ${
              activeTab === 'active'
                ? 'bg-primary-foreground/20 text-primary-foreground font-semibold'
                : 'bg-muted-foreground/15 text-muted-foreground'
            }`}
          >
            {activeProjects.length}
          </span>
        </button>
        <button
          type="button"
          aria-pressed={activeTab === 'archived'}
          onClick={() => setActiveTab('archived')}
          className={`inline-flex min-h-11 shrink-0 items-center justify-center gap-1.5 rounded-full px-3.5 py-1.5 text-xs font-medium transition-all focus-visible:outline-2 focus-visible:outline-ring ${
            activeTab === 'archived'
              ? 'bg-primary text-primary-foreground font-semibold shadow-sm'
              : 'bg-muted/60 text-muted-foreground hover:bg-muted hover:text-foreground border border-border/40'
          }`}
        >
          <span>{t('Archived')}</span>
          <span
            className={`inline-flex min-w-4 items-center justify-center rounded-full px-1.5 text-[10px] font-mono leading-none ${
              activeTab === 'archived'
                ? 'bg-primary-foreground/20 text-primary-foreground font-semibold'
                : 'bg-muted-foreground/15 text-muted-foreground'
            }`}
          >
            {archivedProjects.length}
          </span>
        </button>
      </nav>

      {/* Projects Display: HairlineList Canvas (Zero nested cards) */}
      {displayedProjects.length === 0 ? (
        <div className="border border-dashed border-border/40 px-4 py-12 text-center bg-muted/10 rounded-xl">
          <FolderGit2 className="h-10 w-10 text-muted-foreground/60 mx-auto mb-3" />
          <h3 className="text-sm font-semibold text-foreground">
            {search.trim()
              ? t('No projects match your search')
              : activeTab === 'active'
                ? t('No active projects yet')
                : t('No archived projects')}
          </h3>
          <p className="text-xs text-muted-foreground max-w-sm mx-auto mt-1 mb-5">
            {search.trim()
              ? t('Try another name or repository URL.')
              : activeTab === 'active'
                ? t('Select New project in the header to create your first project.')
                : t('Archived projects will appear here.')}
          </p>
          {search.trim() ? (
            <button className="action-secondary" onClick={() => setSearch('')}>
              {t('Clear search')}
            </button>
          ) : activeTab === 'active' && onCreateProject ? (
            <button
              type="button"
              id="projects-empty-create-btn"
              className="action-primary inline-flex items-center gap-2"
              onClick={onCreateProject}
            >
              <Sparkles className="h-4 w-4 text-amber-400" />
              <span>{t('Create project')}</span>
            </button>
          ) : null}
        </div>
      ) : (
        <div className="rounded-xl border border-border/40 bg-card/40 overflow-hidden transition-colors">
          <HairlineList>
            {displayedProjects.map((project) => {
              const taskCount = getTaskCountForProject(project.id)
              const statusInfo = getProjectStatus(project.id)

              return (
                <HairlineListItem
                  key={project.id}
                  interactive
                  onClick={() => setDrawerProject(project)}
                  leading={
                    <div className="flex items-center gap-3">
                      <StatusDot
                        status={statusInfo.status}
                        pulse={statusInfo.pulse}
                        title={statusInfo.label}
                      />
                      <FolderGit2 className="h-4 w-4 text-primary shrink-0" />
                    </div>
                  }
                  title={
                    <div className="flex items-center gap-2 min-w-0">
                      <span className="font-semibold text-foreground group-hover:text-primary transition-colors truncate">
                        {project.name}
                      </span>
                    </div>
                  }
                  description={
                    <div className="flex items-center gap-2 text-xs text-muted-foreground mt-0.5">
                      <span>
                        {taskCount} {taskCount === 1 ? t('task') : t('tasks')}
                      </span>
                      <span>·</span>
                      <span className="font-mono uppercase text-[10px] tracking-wider text-muted-foreground/80">
                        {project.project_type || 'custom'}
                      </span>
                      {statusInfo.status === 'running' && (
                        <>
                          <span>·</span>
                          <span className="text-sky-500 font-medium">
                            {statusInfo.label}
                          </span>
                        </>
                      )}
                      {statusInfo.status === 'exception' && (
                        <>
                          <span>·</span>
                          <span className="text-rose-500 font-medium">
                            {statusInfo.label}
                          </span>
                        </>
                      )}
                    </div>
                  }
                  trailing={
                    <div className="flex items-center gap-2">
                      <button
                        type="button"
                        onClick={(e) => {
                          e.stopPropagation()
                          void handleOpenProjectTerminal(project.id)
                        }}
                        className="action-secondary text-xs px-2.5 py-1.5 inline-flex items-center gap-1.5 min-h-[36px]"
                        title={t('Open terminal for this project')}
                      >
                        <Terminal className="h-3.5 w-3.5 text-emerald-500" />
                        <span className="hidden sm:inline">{t('Terminal')}</span>
                      </button>

                      <button
                        type="button"
                        onClick={(e) => {
                          e.stopPropagation()
                          onSelectProject(project)
                        }}
                        className="icon-action text-primary min-h-[36px] min-w-[36px]"
                        title={t('Open Board')}
                        aria-label={`Open project ${project.name}`}
                      >
                        <ArrowRight className="h-4 w-4" />
                      </button>
                    </div>
                  }
                />
              )
            })}
          </HairlineList>
        </div>
      )}

      {/* Progressive Disclosure SlideDrawer for Deep Technical Details */}
      <SlideDrawer
        isOpen={Boolean(drawerProject)}
        onClose={() => setDrawerProject(null)}
        title={drawerProject?.name || t('Project details')}
        description={t('Technical details & workspace configuration')}
        size="md"
        badge={
          drawerProject && (
            <span className="text-[10px] uppercase font-mono px-2 py-0.5 bg-primary/10 text-primary border border-primary/20 rounded">
              {drawerProject.project_type || 'custom'}
            </span>
          )
        }
        footer={
          drawerProject && (
            <div className="flex items-center justify-between w-full gap-2">
              <div className="flex items-center gap-2">
                {onArchiveProject && (
                  <button
                    type="button"
                    onClick={() => void archive(drawerProject)}
                    className="action-secondary text-xs inline-flex items-center gap-1.5"
                    disabled={!!busy}
                  >
                    <Archive className="h-3.5 w-3.5 text-muted-foreground" />
                    <span>
                      {drawerProject.is_archived
                        ? t('Restore project')
                        : t('Archive project')}
                    </span>
                  </button>
                )}
              </div>
              <button
                type="button"
                onClick={() => {
                  onSelectProject(drawerProject)
                  setDrawerProject(null)
                }}
                className="action-primary inline-flex items-center gap-1.5 text-xs px-3.5 py-2"
              >
                <span>{t('Open Project Board')}</span>
                <ArrowRight className="h-3.5 w-3.5" />
              </button>
            </div>
          )
        }
      >
        {drawerProject && (
          <div className="space-y-5 text-sm">
            {/* Project Status Summary */}
            <div className="rounded-lg border border-border/40 bg-muted/20 p-3.5 space-y-2">
              <div className="flex items-center justify-between">
                <span className="text-xs font-semibold text-muted-foreground uppercase tracking-wider">
                  {t('Status')}
                </span>
                <div className="flex items-center gap-2">
                  <StatusDot
                    status={getProjectStatus(drawerProject.id).status}
                    pulse={getProjectStatus(drawerProject.id).pulse}
                  />
                  <span className="text-xs font-medium text-foreground">
                    {getProjectStatus(drawerProject.id).label}
                  </span>
                </div>
              </div>
              <div className="flex items-center justify-between text-xs text-muted-foreground">
                <span>{t('Total tasks')}</span>
                <span className="font-mono font-medium text-foreground">
                  {getTaskCountForProject(drawerProject.id)}
                </span>
              </div>
            </div>

            {/* Technical Git Repository & Branch Details */}
            <div className="space-y-2">
              <h4 className="text-xs font-semibold text-muted-foreground uppercase tracking-wider">
                {t('Repository & Branch')}
              </h4>
              {drawerProject.repo_url ? (
                <div className="rounded-lg border border-border/40 bg-card p-3 space-y-2">
                  <div className="flex items-start gap-2 text-xs">
                    <GitBranch className="h-4 w-4 text-primary shrink-0 mt-0.5" />
                    <div className="min-w-0 flex-1">
                      <p className="font-mono text-xs break-all text-foreground">
                        {drawerProject.repo_url}
                      </p>
                    </div>
                  </div>
                </div>
              ) : (
                <div className="rounded-lg border border-border/40 bg-muted/10 p-3 text-xs text-muted-foreground">
                  {t('Local workspace project without external git repository.')}
                </div>
              )}
            </div>

            {/* Dedicated Interactive Terminal Action */}
            <div className="space-y-2">
              <h4 className="text-xs font-semibold text-muted-foreground uppercase tracking-wider">
                {t('Developer & Agent Shell')}
              </h4>
              <p className="text-xs text-muted-foreground">
                {t('Launch an interactive terminal session with AI agent assistance.')}
              </p>
              <button
                type="button"
                onClick={() => void handleOpenProjectTerminal(drawerProject.id)}
                className="action-secondary w-full text-xs py-2 inline-flex items-center justify-center gap-2"
                title={t('Open terminal for this project')}
              >
                <Terminal className="h-3.5 w-3.5 text-emerald-500" />
                <span>{t('Open terminal for this project')}</span>
              </button>
            </div>
          </div>
        )}
      </SlideDrawer>
    </div>
  )
}
