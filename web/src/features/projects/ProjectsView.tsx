import { TaskProgress } from '../../components/ui/TaskProgress'
import { ListSearch, ListTabs, ListTab } from '../../components/layout/ListControls'
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
  ExternalLink,
  Loader2,
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
  const [openingTerminalId, setOpeningTerminalId] = useState('')
  const [error, setError] = useState('')
  const [drawerProject, setDrawerProject] = useState<Project | null>(null)
  const navigate = useNavigate()
  const { t } = useLanguage()

  const handleOpenProjectTerminal = async (projId: string, popout = false) => {
    if (openingTerminalId) return
    setOpeningTerminalId(projId)
    setError('')
    try {
      const targetProj = projects.find((p) => p.id === projId)
      const projName = targetProj?.name || 'Workspace'

      // 1. Reconnect to an existing dedicated project terminal task in memory if available
      let targetTask = tasks.find(
        (item) =>
          (item.project_id === projId || item.project_ids?.includes(projId)) &&
          item.title.startsWith('Terminal ·') &&
          (item.status as string) !== 'cancelled',
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
            item.title.startsWith('Terminal ·') &&
            (item.status as string) !== 'cancelled',
          )
        } catch {}
      }

      // 3. Resolve active agent and create dedicated terminal task if none exists yet
      let assignedAgentId = targetTask?.assignee_id || (targetTask as any)?.assigned_agent || ''
      if (!assignedAgentId) {
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
      }

      if (!targetTask) {
        targetTask = await tasksApi.create({
          title: `Terminal · ${projName}`,
          project_ids: [projId],
          description: `Interactive terminal workspace session with AI agent for ${projName}.`,
          assignee_id: assignedAgentId,
        })
      }

      const launchPayload = {
        request_id: crypto.randomUUID(),
        agent_id: assignedAgentId,
        runner_id: 'local_terminal',
      }
      try {
        localStorage.setItem('cuan_terminal_pin', 'true')
        localStorage.setItem('cuan-task-launch:' + targetTask.id, JSON.stringify(launchPayload))
      } catch {}

      let targetSessionId = 'latest'
      try {
        const runRes = await tasksApi.runAgent(targetTask.id, launchPayload)
        if (runRes.attempt?.id) {
          targetSessionId = runRes.attempt.id
        }
      } catch (err) {
        console.warn('Direct runAgent attempt fallback to session route:', err)
      }

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
          },
        },
      )
    } catch {
      setError(t('Could not load project terminal. Try again.'))
    } finally {
      setOpeningTerminalId('')
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
  )
    .filter(
      (p) =>
        (p.name || '').toLowerCase().includes(search.toLowerCase()) ||
        (p.repo_url && p.repo_url.toLowerCase().includes(search.toLowerCase())),
    )
    .sort((a, b) => {
      // 1. Projects with active / queued tasks first
      const aActive = (a.task_progress?.active || 0) + (a.task_progress?.queued || 0)
      const bActive = (b.task_progress?.active || 0) + (b.task_progress?.queued || 0)
      if (aActive > 0 && bActive === 0) return -1
      if (bActive > 0 && aActive === 0) return 1

      // 2. Latest updated_at (or created_at)
      const aTime = Date.parse(a.updated_at || a.created_at || '') || 0
      const bTime = Date.parse(b.updated_at || b.created_at || '') || 0
      if (bTime !== aTime) return bTime - aTime

      // 3. Alphabetical tie-breaker
      return (a.name || '').localeCompare(b.name || '')
    })

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
        <ListSearch aria-label={t('Search projects')} placeholder={t('Search projects…')} value={search} onChange={event=>setSearch(event.target.value)} />
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
      <ListTabs aria-label={t('Project filters')}>
        <ListTab selected={activeTab==='active'} count={activeProjects.length} onClick={()=>setActiveTab('active')}>{t('Active Projects')}</ListTab>
        <ListTab selected={activeTab==='archived'} count={archivedProjects.length} onClick={()=>setActiveTab('archived')}>{t('Archived')}</ListTab>
      </ListTabs>

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
                      <TaskProgress {...project.task_progress} />
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
                        disabled={Boolean(openingTerminalId)}
                        onClick={(e) => {
                          e.stopPropagation()
                          void handleOpenProjectTerminal(project.id)
                        }}
                        className="action-secondary text-xs px-2.5 py-1.5 inline-flex items-center gap-1.5 min-h-[36px]"
                        title={t('Open terminal for this project')}
                      >
                        {openingTerminalId === project.id ? (
                          <Loader2 className="h-3.5 w-3.5 animate-spin text-emerald-500" />
                        ) : (
                          <Terminal className="h-3.5 w-3.5 text-emerald-500" />
                        )}
                        <span className="hidden sm:inline">
                          {openingTerminalId === project.id ? t('Opening…') : t('Terminal')}
                        </span>
                      </button>

                      <button
                        type="button"
                        disabled={Boolean(openingTerminalId)}
                        onClick={(e) => {
                          e.stopPropagation()
                          void handleOpenProjectTerminal(project.id, true)
                        }}
                        className="icon-action text-muted-foreground hover:text-foreground hidden sm:inline-flex min-h-[36px] min-w-[36px] disabled:opacity-50"
                        title={t('Open terminal in separate window')}
                        aria-label={t('Open terminal in separate window')}
                      >
                        {openingTerminalId === project.id ? (
                          <Loader2 className="h-3.5 w-3.5 animate-spin text-muted-foreground" />
                        ) : (
                          <ExternalLink className="h-3.5 w-3.5" />
                        )}
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
                disabled={Boolean(openingTerminalId)}
                onClick={() => void handleOpenProjectTerminal(drawerProject.id)}
                className="action-secondary w-full text-xs py-2 inline-flex items-center justify-center gap-2"
                title={t('Open terminal for this project')}
              >
                {openingTerminalId === drawerProject.id ? (
                  <Loader2 className="h-3.5 w-3.5 animate-spin text-emerald-500" />
                ) : (
                  <Terminal className="h-3.5 w-3.5 text-emerald-500" />
                )}
                <span>
                  {openingTerminalId === drawerProject.id
                    ? t('Opening terminal…')
                    : t('Open terminal for this project')}
                </span>
              </button>
            </div>
          </div>
        )}
      </SlideDrawer>
    </div>
  )
}
