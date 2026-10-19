import { useEffect, useState, useCallback, useMemo } from 'react'
import { Link, useLocation, useNavigate } from 'react-router-dom'
import { useAuth } from './context/AuthContext'
import { LoginScreen } from './components/auth/LoginScreen'
import { AppShell, AppTab } from './components/layout/AppShell'
import { ProjectsView } from './features/projects/ProjectsView'
import { CreateProjectModal } from './features/projects/CreateProjectModal'
import { CreateTaskModal } from './features/tasks/CreateTaskModal'
import { TaskSidePeek } from './features/tasks/TaskSidePeek'
import { TaskBoardView } from './features/tasks/TaskBoardView'
import { agentsApi, type Agent } from './api/agents'
import { SettingsView } from './features/settings/SettingsView'
import { projectsApi } from './api/projects'
import { tasksApi } from './api/tasks'
import { chatApi, ChatContact } from './api/chat'
import { InboxSubPanel, ContactFilterTab } from './features/inbox/InboxSubPanel'
import { InboxView } from './features/inbox/InboxView'
import {
  Project,
  Task,
  TaskStatus,
  TaskPriority,
  ProjectType,
  DoDItem,
} from './api/types'
import { useMediaQuery } from './hooks/useMediaQuery'
import { Loader2 } from 'lucide-react'

export function App() {
  const { isAuthenticated, isLoading: authLoading } = useAuth()
  const location = useLocation()
  const navigate = useNavigate()

  // Parse URL pathname segments: /:tab/:id
  const pathSegments = useMemo(
    () => location.pathname.split('/').filter(Boolean),
    [location.pathname],
  )
  const rootSegment = pathSegments[0] || ''
  const subParam = pathSegments[1] || ''

  // Current tab derived from URL route
  const currentTab: AppTab = useMemo(() => {
    switch (rootSegment) {
      case 'tasks':
        return 'tasks'
      case 'projects':
        return 'projects'
      case 'team':
        return 'settings'
      case 'settings':
        return 'settings'
      case 'inbox':
      default:
        return 'inbox'
    }
  }, [rootSegment])

  // Data States
  const [projects, setProjects] = useState<Project[]>([])
  const [projectsLoading, setProjectsLoading] = useState(true)
  const [projectsError, setProjectsError] = useState('')
  const [tasks, setTasks] = useState<Task[]>([])
  const [contacts, setContacts] = useState<ChatContact[]>([])
  const [directoryAgents, setDirectoryAgents] = useState<Agent[]>([])
  const [contactsLoading, setContactsLoading] = useState(true)
  const [contactsError, setContactsError] = useState('')
  const [now, setNow] = useState(Date.now())
  useEffect(() => {
    const timer = setInterval(() => setNow(Date.now()), 1000)
    return () => clearInterval(timer)
  }, [])
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
  const [selectedTask, setSelectedTask] = useState<Task | null>(null)
  const [isSidePeekOpen, setIsSidePeekOpen] = useState(false)

  // Modals
  const [isCreateProjectOpen, setIsCreateProjectOpen] = useState(false)
  const [isCreateTaskOpen, setIsCreateTaskOpen] = useState(false)

  // Filters & Mobile
  const [contactTab, setContactTab] = useState<ContactFilterTab>('all')
  const [contactSearch, setContactSearch] = useState('')
  const [selectedTaskStatus, setSelectedTaskStatus] = useState('all')
  const [isMobileChatActive, setIsMobileChatActive] = useState(false)
  const [isMobileDrawerOpen, setIsMobileDrawerOpen] = useState(false)

  // Default redirect from / to /inbox
  useEffect(() => {
    if (location.pathname === '/' || location.pathname === '') {
      navigate('/inbox', { replace: true })
    }
  }, [location.pathname, navigate])

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
        'Project belum dapat dimuat. Periksa koneksi dan coba lagi.',
      )
    } finally {
      setProjectsLoading(false)
    }
  }, [])

  const refreshTasks = useCallback(async () => {
    try {
      const data = await tasksApi.list()
      setTasks(data)
    } catch (err) {
      console.error('Failed to load tasks:', err)
    }
  }, [currentTab, activeProject?.id])

  const refreshContacts = useCallback(async () => {
    const [list, agents] = await Promise.allSettled([
      chatApi.listContacts(),
      agentsApi.list(),
    ])
    if (list.status === 'fulfilled') {
      setContacts(list.value)
      setContactsError('')
    } else
      setContactsError(
        'Kontak belum dapat dimuat. Periksa koneksi dan coba lagi.',
      )
    setDirectoryAgents(agents.status === 'fulfilled' ? agents.value : [])
    setContactsLoading(false)
  }, [])

  useEffect(() => {
    if (isAuthenticated) {
      Promise.allSettled([refreshProjects(), refreshTasks(), refreshContacts()])
      const interval = setInterval(refreshContacts, 5000)
      return () => clearInterval(interval)
    }
  }, [isAuthenticated, refreshProjects, refreshTasks, refreshContacts])

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
    if (rootSegment !== 'tasks' || !subParam) {
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
  }, [rootSegment, subParam, tasks, isAuthenticated])

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
    setSelectedTask(t)
    setIsSidePeekOpen(true)
    navigate(`/tasks/${t.id}`, {
      state: {
        taskReturnTo: location.pathname.startsWith('/projects/')
          ? location.pathname
          : '/tasks',
      },
    })
  }

  const handleCloseSidePeek = () => {
    setIsSidePeekOpen(false)
    setSelectedTask(null)
    const target = location.state?.taskReturnTo
    navigate(
      typeof target === 'string' && /^\/projects\/[^/?#]+$/.test(target)
        ? target
        : '/tasks',
    )
  }

  const handleSelectProject = (p?: Project) => {
    setActiveProject(p)
    if (p) {
      navigate(`/projects/${p.id}`)
    } else {
      navigate('/projects')
    }
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
    handleSelectProject(created)
  }

  const handleArchiveProject = async (id: string, isArchived: boolean) => {
    await projectsApi.archive(id, isArchived)
    await refreshProjects()
  }

  // Handlers for Tasks
  const handleCreateTask = async (data: {
    title: string
    description?: string
    project_ids?: string[]
    priority: TaskPriority
    status: TaskStatus
    dod_checklist?: DoDItem[]
  }) => {
    await tasksApi.create(data)
    await refreshTasks()
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

  const handleRunAgent = async (taskId: string) => {
    const task = tasks.find((item) => item.id === taskId)
    if (task) handleSelectTask(task)
    await refreshTasks()
  }

  // 1. Loading Auth State
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

  // 2. Unauthenticated: Auth Gatekeeper
  if (!isAuthenticated) {
    return <LoginScreen />
  }

  // 3. Authenticated: Sidebar-09 App Shell
  return (
    <>
      <AppShell
        currentTab={currentTab}
        onSelectTab={handleSelectTab}
        isMobileDrawerOpen={isMobileDrawerOpen}
        onMobileDrawerOpenChange={setIsMobileDrawerOpen}
        onCreateProjectClick={() => setIsCreateProjectOpen(true)}
        hideBottomNav={currentTab === 'inbox' && isMobileChatActive}
        inboxSubPanel={
          <InboxSubPanel
            contacts={contacts}
            agents={directoryAgents}
            now={now}
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
        {/* 1. Messenger View — Inbox */}
        {currentTab === 'inbox' && (
          <InboxView
            contacts={contacts}
            agents={directoryAgents}
            now={now}
            loading={contactsLoading}
            error={contactsError}
            onRetry={refreshContacts}
            activeContact={visibleContact}
            onSelectContact={handleSelectContact}
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

        {/* 2. Global Task Board View — Tasks */}
        {currentTab === 'tasks' && (
          <div className="h-full overflow-hidden">
            {taskRouteError && (
              <p role="alert" className="mb-3 text-sm text-destructive">
                {taskRouteError}
              </p>
            )}
            <TaskBoardView
              tasks={tasks}
              projects={projects}

              selectedStatusFilter={selectedTaskStatus}
              onStatusFilterChange={setSelectedTaskStatus}
              onSelectTask={handleSelectTask}
              onCreateTask={() => setIsCreateTaskOpen(true)}
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
                <p role="status">Memuat project…</p>
              ) : (
                <>
                  <p role="alert">
                    {projectsError ||
                      'Project tidak ditemukan atau tidak tersedia.'}
                  </p>
                  <button
                    className="action-secondary"
                    onClick={() => {
                      void refreshProjects()
                    }}
                  >
                    Coba lagi
                  </button>
                  <Link to="/projects" className="action-secondary">
                    Semua project
                  </Link>
                </>
              )}
            </section>
          ) : subParam && activeProject?.id === subParam ? (
            pathSegments[2] === 'settings' ? (
              <SettingsView
                key={activeProject.id}
                activeProject={activeProject}
                onDeleteProject={async (id) => {
                  await projectsApi.archive(id, true)
                  await refreshProjects()
                }}
              />
            ) : (
              <section className="flex h-full min-h-0 flex-col gap-4">
                <div className="flex flex-wrap items-center gap-3">
                  <button
                    className="action-secondary"
                    onClick={() => {
                      setActiveProject(undefined)
                      navigate('/projects')
                    }}
                  >
                    ← Semua project
                  </button>
                  <Link
                    className="action-secondary"
                    to={`/projects/${encodeURIComponent(activeProject.id)}/settings`}
                  >
                    Project settings
                  </Link>
                </div>
                <div className="min-h-0 flex-1">
                  <TaskBoardView
                    tasks={tasks}
                    projects={projects}
                    activeProject={activeProject}
                    onSelectTask={handleSelectTask}
                    onCreateTask={() => setIsCreateTaskOpen(true)}
                    onUpdateTaskStatus={(id, status) =>
                      handleUpdateTask(id, { status })
                    }
                    onRunAgent={handleRunAgent}
                  />
                </div>
              </section>
            )
          ) : (
            <ProjectsView
              projects={projects}
              tasks={tasks}
              onSelectProject={handleSelectProject}
              onCreateProject={() => setIsCreateProjectOpen(true)}
              onArchiveProject={handleArchiveProject}
            />
          ))}

        {/* 5. Settings Configuration */}
        {currentTab === 'settings' && (
          <div className="h-full">
            <SettingsView
              projects={projects}
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

      <CreateProjectModal
        isOpen={isCreateProjectOpen}
        onClose={() => setIsCreateProjectOpen(false)}
        onSubmit={handleCreateProject}
      />

      <CreateTaskModal
        isOpen={isCreateTaskOpen}
        projects={projects.filter((project) => !project.is_archived)}
        activeProjectId={activeProject?.id}
        onClose={() => setIsCreateTaskOpen(false)}
        onSubmit={handleCreateTask}
      />

      <TaskSidePeek
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
