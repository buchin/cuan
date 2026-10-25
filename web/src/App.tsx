import { LandingPage } from './features/landing/LandingPage'
import {
  NotificationsView,
  NotificationsNavigation,
  NotificationsActions,
} from './features/notifications/NotificationsView'
import { useEffect, useState, useCallback, useMemo } from 'react'
import { Link, useLocation, useNavigate } from 'react-router-dom'
import { useAuth } from './context/AuthContext'
import { LoginScreen } from './components/auth/LoginScreen'
import { AppShell, AppTab } from './components/layout/AppShell'
import { ProjectsWorkspace } from './features/projects/ProjectsWorkspace'
import { CreateProjectModal } from './features/projects/CreateProjectModal'
import { CreateTaskModal } from './features/tasks/CreateTaskModal'
import { ActionDialog } from './components/dialogs/ActionDialog'
import { NewContactDialog } from './features/contacts/NewContactDialog'
import { TaskSidePeek } from './features/tasks/TaskSidePeek'
import { TaskBoardView } from './features/tasks/TaskBoardView'
import { TaskSessionPage } from './features/tasks/TaskSessionPage'
import { TaskStartingPage } from './features/tasks/TaskStartingPage'
import { agentsApi, type Agent } from './api/agents'
import { SettingsView } from './features/settings/SettingsView'
import { projectsApi } from './api/projects'
import { tasksApi } from './api/tasks'
import { chatApi, ChatContact } from './api/chat'
import { InboxSubPanel, ContactFilterTab } from './features/inbox/InboxSubPanel'
import { InboxView } from './features/inbox/InboxView'
import { Project, Task, CreateTaskRequest, ProjectType } from './api/types'
import { useMediaQuery } from './hooks/useMediaQuery'
import { StudioView } from './features/inbox/StudioView'
import { ContactsView } from './features/contacts/ContactsView'
import { Loader2 } from 'lucide-react'

export function App() {
  const { isAuthenticated, isLoading: authLoading, token, setupRequired } = useAuth()
  const location = useLocation()
  const navigate = useNavigate()

  // Parse URL pathname segments: /:tab/:id
  const pathSegments = useMemo(
    () => location.pathname.split('/').filter(Boolean),
    [location.pathname],
  )
  const rootSegment = pathSegments[0] || ''
  const subParam = pathSegments[1] || ''
  const sessionRoute =
    rootSegment === 'tasks' &&
    !!subParam &&
    !['detail', 'full'].includes(
      new URLSearchParams(location.search).get('view') || '',
    )
  const isProjectTaskSession =
    rootSegment === 'projects' &&
    pathSegments[2] === 'tasks' &&
    Boolean(pathSegments[3]) &&
    !['detail', 'full'].includes(
      new URLSearchParams(location.search).get('view') || '',
    )
  const isTerminalSession = sessionRoute || isProjectTaskSession

  // Current tab derived from URL route
  const currentTab: AppTab = useMemo(() => {
    switch (rootSegment) {
      case 'notifications':
        return 'notifications'
      case 'contacts':
        return 'contacts'
      case 'tasks':
        return 'tasks'
      case 'projects':
        return 'projects'
      case 'team':
        return 'settings'
      case 'settings':
        return 'settings'
      case 'account':
        return 'account'
      case 'inbox':
        return 'inbox'
      case 'studio':
      default:
        return 'studio'
    }
  }, [rootSegment])

  // Data States
  const [projects, setProjects] = useState<Project[]>([])
  const [projectsLoading, setProjectsLoading] = useState(true)
  const [projectsError, setProjectsError] = useState('')
  const [tasks, setTasks] = useState<Task[]>([])
  const [tasksLoading, setTasksLoading] = useState(true)
  const [tasksError, setTasksError] = useState('')
  const [contacts, setContacts] = useState<ChatContact[]>([])
  const [directoryAgents, setDirectoryAgents] = useState<Agent[]>([])
  const [agentsLoaded, setAgentsLoaded] = useState(false)
  const [agentsError, setAgentsError] = useState(false)
  const [contactsLoading, setContactsLoading] = useState(true)
  const [contactsError, setContactsError] = useState('')
  useEffect(() => {
    if (rootSegment === 'team')
      navigate('/settings/agents' + location.search, { replace: true })
  }, [rootSegment, location.search, navigate])

  // Active selections
  const [activeProject, setActiveProject] = useState<Project | undefined>(
    undefined,
  )
  const [activeContact, setActiveContact] = useState<ChatContact | undefined>(
    undefined,
  )
  const [taskRouteError, setTaskRouteError] = useState('')
  const [taskLaunchError, setTaskLaunchError] = useState('')
  const [selectedTask, setSelectedTask] = useState<Task | null>(null)
  const [isSidePeekOpen, setIsSidePeekOpen] = useState(false)

  // Modals
  const [isCreateProjectOpen, setIsCreateProjectOpen] = useState(false)
  const [isCreateTaskOpen, setIsCreateTaskOpen] = useState(false)
  const [newFlow, setNewFlow] = useState<'contact' | 'menu' | null>(null)
  const [contactRevision, setContactRevision] = useState(0)
  const [agentCreateRequested, setAgentCreateRequested] = useState(false)
  const openNew = (
    kind: 'message' | 'contact' | 'task' | 'project' | 'agent' | 'menu',
  ) => {
    setNewFlow(null)
    if (kind === 'message') navigate('/studio')
    else if (kind === 'task') setIsCreateTaskOpen(true)
    else if (kind === 'project') setIsCreateProjectOpen(true)
    else if (kind === 'agent') {
      navigate('/settings/agents')
      setAgentCreateRequested(true)
    } else setNewFlow(kind)
  }

  // Filters & Mobile
  const [contactTab, setContactTab] = useState<ContactFilterTab>('all')
  const [contactSearch, setContactSearch] = useState('')
  const [selectedTaskStatus, setSelectedTaskStatus] = useState('all')
  const [isMobileChatActive, setIsMobileChatActive] = useState(false)

  // Studio is the default entry point.
  useEffect(() => {
    if (
      isAuthenticated &&
      (location.pathname === '/' ||
        location.pathname === '' ||
        location.pathname === '/login')
    ) {
      navigate('/studio', { replace: true })
    }
  }, [location.pathname, navigate, isAuthenticated])

  // Fetch Projects and Tasks
  const refreshProjects = useCallback(async () => {
    try {
      const data = await projectsApi.list()
      setProjects(data)
      setProjectsError('')
      setActiveProject((previous) =>
        previous
          ? data.find((p) => p.id === previous.id) || previous
          : undefined,
      )
    } catch (err) {
      setProjectsError(
        'Could not load projects. Check your connection and try again.',
      )
    } finally {
      setProjectsLoading(false)
    }
  }, [])

  const refreshTasks = useCallback(async () => {
    try {
      const data = await tasksApi.list()
      setTasks(data)
      setTasksError('')
    } catch {
      setTasksError(
        'Could not load tasks. Check your connection and try again.',
      )
    } finally {
      setTasksLoading(false)
    }
  }, [])

  const refreshContacts = useCallback(async () => {
    const [list, agents] = await Promise.allSettled([
      chatApi.listContacts(),
      agentsApi.list(),
    ])
    if (list.status === 'fulfilled') {
      setContacts((previous) =>
        JSON.stringify(previous) === JSON.stringify(list.value)
          ? previous
          : list.value,
      )
      setContactsError('')
    } else
      setContactsError(
        'Could not load contacts. Check your connection and try again.',
      )
    if (agents.status === 'fulfilled') {
      setAgentsError(false)
      setDirectoryAgents((previous) =>
        JSON.stringify(previous) === JSON.stringify(agents.value)
          ? previous
          : agents.value,
      )
    } else setAgentsError(true)
    setAgentsLoaded(true)
    setContactsLoading(false)
  }, [])

  useEffect(() => {
    if (isAuthenticated) {
      Promise.allSettled([refreshProjects(), refreshTasks(), refreshContacts()])
      let refreshing = false
      const refresh = async () => {
        if (document.hidden || refreshing) return
        refreshing = true
        try {
          await refreshContacts()
        } finally {
          refreshing = false
        }
      }
      const interval = setInterval(() => void refresh(), 5000)
      document.addEventListener('visibilitychange', refresh)
      return () => {
        clearInterval(interval)
        document.removeEventListener('visibilitychange', refresh)
      }
    }
  }, [isAuthenticated, refreshProjects, refreshTasks, refreshContacts])

  // Task/session completion must become visible without reopening the list.
  useEffect(() => {
    if (
      !isAuthenticated ||
      isTerminalSession ||
      !['tasks', 'projects'].includes(currentTab)
    )
      return
    let refreshing = false
    const refresh = async () => {
      if (document.visibilityState !== 'visible' || refreshing) return
      refreshing = true
      try {
        await refreshTasks()
      } finally {
        refreshing = false
      }
    }
    if (!tasksLoading) void refresh()
    const timer = setInterval(() => void refresh(), 5000)
    const resume = () => void refresh()
    window.addEventListener('focus', resume)
    document.addEventListener('visibilitychange', resume)
    return () => {
      clearInterval(timer)
      window.removeEventListener('focus', resume)
      document.removeEventListener('visibilitychange', resume)
    }
  }, [isAuthenticated, currentTab, isTerminalSession, refreshTasks])

  // Read the visible conversation directly from the URL. Back/forward and
  // responsive list mode cannot race against an effect restoring old selection.
  const wide = useMediaQuery('(min-width: 1200px)')
  const visibleContact =
    subParam && rootSegment === 'inbox'
      ? contacts.find(
          (c) =>
            c.conversation_id === subParam ||
            c.id === subParam ||
            c.id === 'contact_' + subParam ||
            c.name.toLowerCase() === subParam.toLowerCase(),
        )
      : wide && rootSegment === 'inbox'
        ? contacts.find((c) => c.id === activeContact?.id) || contacts[0]
        : undefined

  // URL drives the SidePeek so refresh/back/forward cannot leave a stale panel.
  useEffect(() => {
    let current = true
    setTaskRouteError('')
    if (rootSegment !== 'tasks' || !subParam || sessionRoute) {
      setIsSidePeekOpen(false)
      if (rootSegment === 'tasks') setSelectedTask(null)
      return
    }
    if (!isAuthenticated) return
    const matched = tasks.find((task) => task.id === subParam)
    if (matched) {
      setSelectedTask(matched)
      setIsSidePeekOpen(true)
    } else
      tasksApi
        .get(subParam)
        .then((task) => {
          if (current) {
            setSelectedTask(task)
            setIsSidePeekOpen(true)
          }
        })
        .catch(() => {
          if (current) {
            setSelectedTask(null)
            setIsSidePeekOpen(false)
            setTaskRouteError('Task not found or unavailable.')
          }
        })
    return () => {
      current = false
    }
  }, [rootSegment, subParam, tasks, isAuthenticated, sessionRoute])

  // Deep Link Synchronization: /projects/:projectId
  useEffect(() => {
    if (rootSegment === 'projects' && projects.length > 0) {
      if (subParam) {
        const matched = projects.find((p) => p.id === subParam)
        if (matched) {
          setActiveProject(matched)
        }
      }
    }
  }, [rootSegment, subParam, projects])

  useEffect(() => {
    if (rootSegment === 'projects' && subParam && pathSegments.length === 2)
      navigate(
        '/projects/' +
          encodeURIComponent(subParam) +
          '/tasks' +
          location.search,
        { replace: true, state: location.state },
      )
  }, [rootSegment, subParam, location.pathname, navigate])

  // Navigation handlers
  const handleSelectTab = (tab: AppTab) => {
    if (tab === 'inbox') {
      navigate(
        activeContact
          ? `/inbox/${activeContact.conversation_id || activeContact.id}`
          : '/inbox',
      )
    } else if (tab === 'tasks') {
      navigate(
        selectedTask && isSidePeekOpen ? `/tasks/${selectedTask.id}` : '/tasks',
      )
    } else if (tab === 'projects') {
      navigate(activeProject ? `/projects/${activeProject.id}` : '/projects')
    } else if (tab === 'account') {
      navigate('/account')
    } else {
      navigate(`/${tab}`)
    }
    if (tab !== 'inbox') {
      setIsMobileChatActive(false)
    }
  }

  const handleSelectContact = (c: ChatContact) => {
    setActiveContact(c)
    navigate(`/inbox/${c.conversation_id || c.id}`)
  }

  const handleSelectTask = (t: Task) => {
    setTaskLaunchError('')
    setSelectedTask(t)
    setIsSidePeekOpen(false)
    const search = new URLSearchParams(location.search)
    search.delete('view')
    const suffix = search.toString() ? '?' + search.toString() : ''
    const returnTo = location.pathname.startsWith('/projects/')
      ? location.pathname + suffix
      : typeof location.state?.taskReturnTo === 'string'
        ? location.state.taskReturnTo
        : '/tasks' + suffix
    navigate(
      `${t.project_id ? `/projects/${encodeURIComponent(t.project_id)}/tasks/` : '/tasks/'}${encodeURIComponent(t.id)}${suffix}`,
      {
        state: { taskReturnTo: returnTo },
      },
    )
  }
  const handleCloseSidePeek = () => {
    setIsSidePeekOpen(false)
    setSelectedTask(null)
    void refreshTasks()
    const search = new URLSearchParams(location.search)
    search.delete('view')
    if (rootSegment === 'tasks' && subParam) {
      const target = location.state?.workspaceReturnTo
      navigate(
        typeof target === 'string' &&
          (target.startsWith('/tasks/' + encodeURIComponent(subParam)) ||
            new RegExp(
              '^/projects/[^/?#]+/tasks/' +
                encodeURIComponent(subParam) +
                '(?:/|$)',
            ).test(target)) &&
          !/[?&]view=/.test(target)
          ? target
          : '/tasks/' +
              encodeURIComponent(subParam) +
              (search.toString() ? '?' + search.toString() : ''),
        { state: location.state },
      )
      return
    }
    const fallback =
      '/tasks' + (search.toString() ? '?' + search.toString() : '')
    const target = location.state?.taskReturnTo
    navigate(
      typeof target === 'string' &&
        /^\/(?:tasks|projects\/[^/?#]+(?:\/tasks)?|inbox\/[^/?#]+)(?:\?[^#]*)?$/.test(
          target,
        )
        ? target
        : fallback,
    )
  }

  // Handlers for Projects
  const handleCreateProject = async (data: {
    name: string
    project_type: ProjectType
    repo_url?: string
    github_token?: string
  }) => {
    const created = await projectsApi.create(data)
    await refreshProjects()
    setActiveProject(created)
    navigate(`/studio?project=${encodeURIComponent(created.id)}`)
  }

  const handleArchiveProject = async (id: string, isArchived: boolean) => {
    await projectsApi.archive(id, isArchived)
    await refreshProjects()
  }

  // Handlers for Tasks
  const handleCreateTask = async (
    data: CreateTaskRequest,
    startAgentId?: string,
  ) => {
    const created = await tasksApi.create(data)
    void refreshTasks()
    if (!startAgentId) {
      handleSelectTask(created)
      return
    }
    const payload = { request_id: crypto.randomUUID(), agent_id: startAgentId }
    try {
      localStorage.setItem(
        'cuan-task-launch:' + created.id,
        JSON.stringify(payload),
      )
    } catch {
      handleSelectTask(created)
      setTaskLaunchError(
        'Task saved. Browser storage is unavailable; enable it before starting a session.',
      )
      return
    }
    const next = new URLSearchParams(location.search)
    next.delete('view')
    const suffix = next.toString() ? '?' + next.toString() : ''
    const returnTo = location.pathname.startsWith('/projects/')
      ? location.pathname + suffix
      : '/tasks' + suffix
    setIsSidePeekOpen(false)
    setSelectedTask(null)
    navigate(
      `${created.project_id ? `/projects/${encodeURIComponent(created.project_id)}/tasks/` : '/tasks/'}${encodeURIComponent(created.id)}/sessions/start${suffix}`,
      { state: { taskReturnTo: returnTo, startRequestId: payload.request_id } },
    )
  }

  const handleUpdateTask = async (taskId: string, updates: Partial<Task>) => {
    await tasksApi.update(taskId, updates)
    await refreshTasks()
    if (selectedTask?.id === taskId) {
      setSelectedTask((prev) => (prev ? { ...prev, ...updates } : null))
    }
  }

  const handleDeleteTask = async (taskId: string) => {
    await tasksApi.delete(taskId)
    await refreshTasks()
    handleCloseSidePeek()
  }

  const handleRunAgent = async (_taskId: string) => {
    await refreshTasks()
  }

  if (
    rootSegment === 'about' ||
    (location.pathname === '/' && !token && !isAuthenticated && setupRequired !== true)
  )
    return <LandingPage />

  if (authLoading) {
    return (
      <div className="h-screen w-screen flex items-center justify-center bg-background text-primary font-mono">
        <div className="flex flex-col items-center gap-3">
          <Loader2 className="h-8 w-8 animate-spin" />
          <span className="text-xs uppercase tracking-widest text-muted-foreground">
            Initializing Cuan...
          </span>
        </div>
      </div>
    )
  }

  const headerProject =
    currentTab === 'projects' && subParam
      ? projects.find((p) => p.id === subParam)
      : undefined
  const projectDetail = currentTab === 'projects' && !!subParam
  const projectSettings = projectDetail && pathSegments[2] !== 'tasks'
  const settingTitles: Record<string, string> = {
    agents: 'Agents',
    integrations: 'Integrations',
    appearance: 'Appearance',
    profile: 'Profile',
    notifications: 'Notifications',
    contacts: 'Contacts',
    general: 'General',
  }
  const isAccountOrSettings = currentTab === 'settings' || currentTab === 'account'
  const settingsDetail = isAccountOrSettings && !!settingTitles[subParam]
  const headerTitle = projectDetail
    ? projectSettings
      ? pathSegments[2] === 'repository' ? 'Repository' : 'Settings'
      : headerProject?.name || 'Project'
    : settingsDetail
      ? pathSegments[2] === 'live-chat'
        ? 'Live Chat'
        : pathSegments[2] === 'github' ? 'GitHub' : settingTitles[subParam]
      : currentTab === 'account'
        ? 'Account'
        : undefined
  const headerBreadcrumbs = projectDetail
    ? [
        { label: 'Projects', to: '/projects' },
        ...(projectSettings && headerProject
          ? [
              {
                label: headerProject.name,
                to: `/projects/${encodeURIComponent(headerProject.id)}`,
              },
            ]
          : []),
      ]
    : settingsDetail
      ? [
          {
            label: currentTab === 'account' ? 'Account' : 'Settings',
            to: currentTab === 'account' ? '/account' : '/settings',
          },
          ...(['live-chat', 'github'].includes(pathSegments[2])
            ? [{ label: 'Integrations', to: '/settings/integrations' }]
            : []),
        ]
      : []

  // 2. Unauthenticated: Auth Gatekeeper
  if (!isAuthenticated) {
    if (location.pathname === '/' && setupRequired !== true) return <LandingPage />
    return <LoginScreen />
  }

  // 3. Authenticated: Sidebar-09 App Shell
  return (
    <>
      <AppShell
        currentTab={currentTab}
        headerActions={
          currentTab === 'notifications' ? <NotificationsActions /> : undefined
        }
        secondaryNavigation={
          currentTab === 'notifications' ? (
            <NotificationsNavigation />
          ) : undefined
        }
        pageTitle={isTerminalSession ? 'Task session' : headerTitle}
        pageTitleIsContent={
          projectDetail && !!headerProject
        }
        breadcrumbs={headerBreadcrumbs}
        inboxUnread={contacts.reduce(
          (sum, contact) => sum + Math.max(0, contact.unread_count || 0),
          0,
        )}
        onSelectTab={handleSelectTab}
        newAction={
          currentTab === 'studio' ||
          currentTab === 'notifications' ||
          projectDetail ||
          (currentTab === 'inbox' && (!!subParam || isMobileChatActive)) ||
          (currentTab === 'contacts' && !!new URLSearchParams(location.search).get('contact')) ||
          ((currentTab === 'settings' || currentTab === 'account') && subParam !== 'agents' && subParam !== 'contacts') ||
          (currentTab === 'account' && subParam === 'contacts' && !!new URLSearchParams(location.search).get('contact'))
            ? undefined
            : {
                label:
                  currentTab === 'inbox'
                    ? 'New message'
                    : currentTab === 'contacts' || (currentTab === 'account' && subParam === 'contacts')
                      ? 'New contact'
                      : currentTab === 'tasks' || projectDetail
                        ? 'New task'
                        : currentTab === 'projects'
                          ? 'New project'
                          : subParam === 'agents'
                            ? 'New agent'
                            : 'New',
                onClick: () =>
                  openNew(
                    currentTab === 'inbox'
                      ? 'message'
                      : currentTab === 'contacts' || (currentTab === 'account' && subParam === 'contacts')
                        ? 'contact'
                        : currentTab === 'tasks' || projectDetail
                          ? 'task'
                          : currentTab === 'projects'
                            ? 'project'
                            : subParam === 'agents'
                              ? 'agent'
                              : 'menu',
                  ),
              }
        }
        contentHeader={projectDetail && !!headerProject}
        projectWorkspace={currentTab === 'projects' && !isProjectTaskSession}
        sessionWorkspace={isTerminalSession}
        hideBottomNav={
          isTerminalSession || (currentTab === 'inbox' && isMobileChatActive)
        }
        inboxSubPanel={
          <InboxSubPanel
            contacts={contacts}
            onNewMessage={() => openNew('message')}
            agents={directoryAgents}
            loading={contactsLoading}
            error={contactsError}
            onRetry={refreshContacts}
            activeContactId={visibleContact?.id}
            onSelectContact={handleSelectContact}
            selectedTab={contactTab}
            onTabChange={setContactTab}
            searchQuery={contactSearch}
            onSearchChange={setContactSearch}
          />
        }
      >
        {currentTab === 'notifications' && <NotificationsView />}
        {currentTab === 'studio' && (
          <StudioView
            onAccepted={() => void refreshContacts()}
            checkRuntime={!directoryAgents.some((a) =>
              a.enabled !== false && a.status !== 'inactive' &&
              ['codex', 'claude'].includes(a.harness || a.provider || '')
            )}
            agentSetup={
              !agentsLoaded ? 'loading' : agentsError ? 'error' :
              directoryAgents.some((a) => a.enabled !== false && a.status !== 'inactive')
                ? 'ready' : 'needed'
            }
            firstGoal={!tasksLoading && !tasksError && tasks.length === 0}
            hasProject={
              projectsLoading || !!projectsError ||
              projects.some((p) => p.id !== 'default' || !!p.repo_url)
            }
            onAgentConnected={() => void refreshContacts()}
            onCreateProject={() => setIsCreateProjectOpen(true)}
            onRetrySetup={() => void refreshContacts()}
          />
        )}
        {/* 1. Messenger View — Inbox */}
        {currentTab === 'inbox' && (
          <InboxView
            contacts={contacts}
            agents={directoryAgents}
            loading={contactsLoading}
            error={contactsError}
            onRetry={refreshContacts}
            activeContact={visibleContact}
            onSelectContact={handleSelectContact}
            onNewMessage={() => openNew('message')}
            onContactUpdated={refreshContacts}
            contactTab={contactTab}
            onContactTabChange={setContactTab}
            contactSearch={contactSearch}
            onContactSearchChange={setContactSearch}
            onBackToContacts={() => {
              setActiveContact(undefined)
              navigate('/inbox')
            }}
            onMobileChatActiveChange={setIsMobileChatActive}
          />
        )}

        {/* 2. Contacts View */}
        {(currentTab === 'contacts' ||
          ((currentTab === 'settings' || currentTab === 'account') &&
            subParam === 'contacts')) && (
          <ContactsView refreshKey={contactRevision} onNewContact={() => openNew('contact')} />
        )}
        {sessionRoute && pathSegments[3] === 'start' && (
          <TaskStartingPage key={subParam} taskId={subParam} />
        )}
        {sessionRoute && pathSegments[3] !== 'start' && (
          <TaskSessionPage
            key={subParam}
            taskId={subParam}
            sessionId={pathSegments[3] || 'latest'}
          />
        )}
        {currentTab === 'tasks' && !sessionRoute && (
          <div className="h-full overflow-hidden">
            {taskRouteError && (
              <p role="alert" className="mb-3 text-sm text-destructive">
                {taskRouteError}
              </p>
            )}
            <TaskBoardView
              tasks={tasks}
              loading={tasksLoading}
              error={tasksError}
              onRetry={() => void refreshTasks()}
              projects={projects}
              selectedStatusFilter={selectedTaskStatus}
              onStatusFilterChange={setSelectedTaskStatus}
              onSelectTask={handleSelectTask}
              onUpdateTaskStatus={(taskId, status) =>
                handleUpdateTask(taskId, { status })
              }
              onRunAgent={handleRunAgent}
            />
          </div>
        )}

        {/* 3. Projects Workspace & Repositories */}
        {currentTab === 'projects' &&
          (subParam && activeProject?.id !== subParam ? (
            <section className="page-content space-y-4">
              {projectsLoading ? (
                <p role="status">Loading projects…</p>
              ) : (
                <>
                  <p role="alert">
                    {projectsError || 'Project not found or unavailable.'}
                  </p>
                  <button
                    className="action-secondary"
                    onClick={() => {
                      void refreshProjects()
                    }}
                  >
                    Try again
                  </button>
                  <Link to="/projects" className="action-secondary">
                    All projects
                  </Link>
                </>
              )}
            </section>
          ) : subParam && activeProject?.id === subParam ? (
            pathSegments[2] !== 'tasks' ? (
              <ProjectsWorkspace
                onNewProject={() => openNew('project')}
                projects={projects}
                project={activeProject}
                tasks={tasks}
                loading={projectsLoading}
                error={projectsError}
                onRetry={() => void refreshProjects()}
                onArchiveProject={handleArchiveProject}
                settingsContent={<SettingsView
                  key={activeProject.id}
                  activeProject={activeProject}
                  embeddedProject
                  onProjectUpdated={refreshProjects}
                  onDeleteProject={async (id) => {
                    await projectsApi.archive(id, true)
                    await refreshProjects()
                  }}
                />}
              />
            ) : (
              <ProjectsWorkspace
                onNewProject={() => openNew('project')}
                projects={projects}
                onArchiveProject={handleArchiveProject}
                onNewTask={() => openNew('task')}
                project={activeProject}
                tasks={tasks}
                taskId={
                  pathSegments[2] === 'tasks' ? pathSegments[3] : undefined
                }
                sessionId={
                  pathSegments[4] === 'sessions' ? pathSegments[5] : undefined
                }
                loading={tasksLoading}
                error={tasksError}
                onRetry={() => void refreshTasks()}
              />
            )
          ) : (
            <ProjectsWorkspace
                onNewProject={() => openNew('project')}
              projects={projects}
              tasks={tasks}
              loading={projectsLoading || tasksLoading}
              error={projectsError || tasksError}
              onRetry={() => {
                void refreshProjects()
                void refreshTasks()
              }}
              onArchiveProject={handleArchiveProject}
            />
          ))}

        {/* 5. Settings / Account Configuration */}
        {(currentTab === 'settings' || currentTab === 'account') &&
          subParam !== 'contacts' && (
            <div className="min-h-full">
              <SettingsView
                projects={projects}
                createAgentRequested={agentCreateRequested}
                onCreateAgentHandled={() => setAgentCreateRequested(false)}
                onAgentAdded={refreshContacts}
                onDeleteProject={async (id) => {
                  await projectsApi.archive(id, true)
                  await refreshProjects()
                }}
              />
            </div>
          )}
      </AppShell>

      {/* Modals & Drawers */}
      <NewContactDialog
        open={newFlow === 'contact'}
        onClose={() => setNewFlow(null)}
        onCreated={(contact) => {
          setNewFlow(null)
          setContactRevision((n) => n + 1)
          const target = location.pathname.startsWith('/account')
            ? '/account/contacts?contact=' + encodeURIComponent(contact.id)
            : '/contacts?contact=' + encodeURIComponent(contact.id)
          navigate(target)
          void refreshContacts()
        }}
      />
      <ActionDialog
        open={newFlow === 'menu'}
        title="New"
        onClose={() => setNewFlow(null)}
      >
        <p className="mb-3 text-sm text-muted-foreground">
          What would you like to create?
        </p>
        <div className="grid gap-2">
          {(['message', 'contact', 'task', 'project', 'agent'] as const).map(
            (kind) => (
              <button
                key={kind}
                className="action-secondary justify-start"
                onClick={() => openNew(kind)}
              >
                New {kind}
              </button>
            ),
          )}
        </div>
      </ActionDialog>

      <CreateProjectModal
        isOpen={isCreateProjectOpen}
        onClose={() => setIsCreateProjectOpen(false)}
        onSubmit={handleCreateProject}
      />

      <CreateTaskModal
        isOpen={isCreateTaskOpen}
        projects={projects.filter((project) => !project.is_archived)}
        activeProjectId={projectDetail ? subParam : undefined}
        onClose={() => setIsCreateTaskOpen(false)}
        onSubmit={handleCreateTask}
      />

      <TaskSidePeek
        initialError={taskLaunchError}
        task={selectedTask}
        projects={projects}
        isOpen={isSidePeekOpen}
        onClose={handleCloseSidePeek}
        onUpdate={handleUpdateTask}
        onDelete={handleDeleteTask}
        onRunAgent={handleRunAgent}
      />
    </>
  )
}

export default App
