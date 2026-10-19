import { useEffect, useState, useCallback, useMemo } from 'react'
import { Link, useLocation, useNavigate } from 'react-router-dom'
import { useAuth } from './context/AuthContext'
import { LoginScreen } from './components/auth/LoginScreen'
import { AppShell, AppTab } from './components/layout/AppShell'
import { ProjectsView } from './features/projects/ProjectsView'
import { CreateProjectModal } from './features/projects/CreateProjectModal'
import { CreateTaskModal } from './features/tasks/CreateTaskModal'
import { ActionDialog } from './components/dialogs/ActionDialog'
import { NewContactDialog } from './features/contacts/NewContactDialog'
import { NewMessageDialog } from './features/inbox/NewMessageDialog'
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
  CreateTaskRequest,
  ProjectType,
} from './api/types'
import { useMediaQuery } from './hooks/useMediaQuery'
import { ContactsView } from './features/contacts/ContactsView'
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
  const [tasksLoading,setTasksLoading]=useState(true)
  const [tasksError,setTasksError]=useState('')
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
  const [newFlow, setNewFlow] = useState<'message' | 'contact' | 'menu' | null>(
    null,
  )
  const [contactRevision, setContactRevision] = useState(0)
  const [agentCreateRequested, setAgentCreateRequested] = useState(false)
  const openNew = (
    kind: 'message' | 'contact' | 'task' | 'project' | 'agent' | 'menu',
  ) => {
    setNewFlow(null)
    if (kind === 'task') setIsCreateTaskOpen(true)
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
        'Could not load projects. Check your connection and try again.',
      )
    } finally {
      setProjectsLoading(false)
    }
  }, [])

  const refreshTasks = useCallback(async () => {
    try {
      const data = await tasksApi.list()
      setTasks(data);setTasksError('')
    } catch {setTasksError('Could not load tasks. Check your connection and try again.')}finally{setTasksLoading(false)}
  }, [])

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
        'Could not load contacts. Check your connection and try again.',
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
    const search = new URLSearchParams(location.search);search.delete('view')
    const suffix=search.toString()?'?'+search.toString():''
    const returnTo=location.pathname.startsWith('/projects/')?location.pathname+suffix:typeof location.state?.taskReturnTo==='string'?location.state.taskReturnTo:'/tasks'+suffix
    navigate(`/tasks/${encodeURIComponent(t.id)}${suffix}`,{state:{taskReturnTo:returnTo}})
  }
  const handleCloseSidePeek = () => {
    setIsSidePeekOpen(false);setSelectedTask(null);void refreshTasks()
    const search=new URLSearchParams(location.search);search.delete('view')
    const fallback='/tasks'+(search.toString()?'?'+search.toString():'')
    const target=location.state?.taskReturnTo
    navigate(typeof target==='string'&&/^\/(?:tasks|projects\/[^/?#]+)(?:\?[^#]*)?$/.test(target)?target:fallback)
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
  const handleCreateTask = async (data: CreateTaskRequest) => {
    const created=await tasksApi.create(data)
    await refreshTasks()
    handleSelectTask(created)
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
  const projectSettings = projectDetail && pathSegments[2] === 'settings'
  const settingTitles: Record<string, string> = {
    agents: 'Agents',
    integrations: 'Integrations',
    appearance: 'Appearance',
  }
  const settingsDetail = currentTab === 'settings' && !!settingTitles[subParam]
  const headerTitle = projectDetail
    ? projectSettings
      ? 'Settings'
      : headerProject?.name || 'Project'
    : settingsDetail
      ? pathSegments[2] === 'live-chat'
        ? 'Live Chat'
        : settingTitles[subParam]
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
          { label: 'Settings', to: '/settings' },
          ...(pathSegments[2] === 'live-chat'
            ? [{ label: 'Integrations', to: '/settings/integrations' }]
            : []),
        ]
      : []

  // 2. Unauthenticated: Auth Gatekeeper
  if (!isAuthenticated) {
    return <LoginScreen />
  }

  // 3. Authenticated: Sidebar-09 App Shell
  return (
    <>
      <AppShell
        currentTab={currentTab}
        pageTitle={headerTitle}
        breadcrumbs={headerBreadcrumbs}
        inboxUnread={contacts.reduce(
          (sum, contact) => sum + Math.max(0, contact.unread_count || 0),
          0,
        )}
        onSelectTab={handleSelectTab}
        newAction={{
          label:
            currentTab === 'inbox'
              ? 'New message'
              : currentTab === 'contacts'
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
                : currentTab === 'contacts'
                  ? 'contact'
                  : currentTab === 'tasks' || projectDetail
                    ? 'task'
                    : currentTab === 'projects'
                      ? 'project'
                      : subParam === 'agents'
                        ? 'agent'
                        : 'menu',
            ),
        }}
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

        {/* 2. Global Task Board View — Tasks */}
        {currentTab === 'contacts' && (
          <ContactsView refreshKey={contactRevision} />
        )}
        {currentTab === 'tasks' && (
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
              onRetry={()=>void refreshTasks()}
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
                    className="action-secondary md:hidden"
                    onClick={() => {
                      setActiveProject(undefined)
                      navigate('/projects')
                    }}
                  >
                    ← All projects
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
                    loading={tasksLoading}
                    error={tasksError}
                    onRetry={()=>void refreshTasks()}
                    projects={projects}
                    activeProject={activeProject}
                    onSelectTask={handleSelectTask}
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
              onArchiveProject={handleArchiveProject}
            />
          ))}

        {/* 5. Settings Configuration */}
        {currentTab === 'settings' && (
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
      <NewMessageDialog
        open={newFlow === 'message'}
        onClose={() => setNewFlow(null)}
        contacts={contacts}
        loading={contactsLoading}
        error={contactsError}
        onRetry={() => void refreshContacts()}
        onSelect={handleSelectContact}
      />
      <NewContactDialog
        open={newFlow === 'contact'}
        onClose={() => setNewFlow(null)}
        onCreated={(contact) => {
          setNewFlow(null)
          setContactRevision((n) => n + 1)
          navigate('/contacts?contact=' + encodeURIComponent(contact.id))
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
        activeProjectId={projectDetail ? headerProject?.id : undefined}
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
