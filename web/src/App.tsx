import {
  NotificationsView,
  NotificationsNavigation,
  NotificationsActions,
} from './features/notifications/NotificationsView'
import { useEffect, useState, useCallback, useMemo, lazy, Suspense } from 'react'
import { Link, useLocation, useNavigate } from 'react-router-dom'
import { useAuth } from './context/AuthContext'
import { AppShell, AppTab } from './components/layout/AppShell'
import { ActionDialog } from './components/dialogs/ActionDialog'
import { agentsApi, type Agent } from './api/agents'
import { projectsApi } from './api/projects'
import { tasksApi } from './api/tasks'
import { chatApi, ChatContact } from './api/chat'
import { InboxSubPanel, ContactFilterTab } from './features/inbox/InboxSubPanel'
import { Project, Task, CreateTaskRequest, ProjectType } from './api/types'
import { useMediaQuery } from './hooks/useMediaQuery'
import { Loader2 } from 'lucide-react'

// Code-splitting via dynamic imports for heavy routes and dialogs
const LoginScreen = lazy(() =>
  import('./components/auth/LoginScreen').then((m) => ({
    default: m.LoginScreen,
  })),
)
const AcceptInvitePage = lazy(() =>
  import('./components/auth/AcceptInvitePage').then((m) => ({
    default: m.AcceptInvitePage,
  })),
)
const InboxView = lazy(() =>
  import('./features/inbox/InboxView').then((m) => ({
    default: m.InboxView,
  })),
)
const StudioView = lazy(() =>
  import('./features/inbox/StudioView').then((m) => ({
    default: m.StudioView,
  })),
)
const ContactsView = lazy(() =>
  import('./features/contacts/ContactsView').then((m) => ({
    default: m.ContactsView,
  })),
)
const TaskStartingPage = lazy(() =>
  import('./features/tasks/TaskStartingPage').then((m) => ({
    default: m.TaskStartingPage,
  })),
)
const TaskSessionPage = lazy(() =>
  import('./features/tasks/TaskSessionPage').then((m) => ({
    default: m.TaskSessionPage,
  })),
)
const TaskBoardView = lazy(() =>
  import('./features/tasks/TaskBoardView').then((m) => ({
    default: m.TaskBoardView,
  })),
)
const ProjectsWorkspace = lazy(() =>
  import('./features/projects/ProjectsWorkspace').then((m) => ({
    default: m.ProjectsWorkspace,
  })),
)
const SettingsView = lazy(() =>
  import('./features/settings/SettingsView').then((m) => ({
    default: m.SettingsView,
  })),
)
const SchedulesView = lazy(() =>
  import('./features/schedules/SchedulesView').then((m) => ({
    default: m.SchedulesView,
  })),
)
const NewContactDialog = lazy(() =>
  import('./features/contacts/NewContactDialog').then((m) => ({
    default: m.NewContactDialog,
  })),
)
const CreateProjectModal = lazy(() =>
  import('./features/projects/CreateProjectModal').then((m) => ({
    default: m.CreateProjectModal,
  })),
)
const CreateTaskModal = lazy(() =>
  import('./features/tasks/CreateTaskModal').then((m) => ({
    default: m.CreateTaskModal,
  })),
)
const TaskSidePeek = lazy(() =>
  import('./features/tasks/TaskSidePeek').then((m) => ({
    default: m.TaskSidePeek,
  })),
)

function ViewFallback() {
  return (
    <div className="flex h-full min-h-[300px] w-full items-center justify-center p-8">
      <Loader2 className="h-6 w-6 animate-spin text-muted-foreground" />
    </div>
  )
}

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
  const sessionRoute =
    rootSegment === 'tasks' &&
    !!subParam &&
    (pathSegments[2] === 'sessions' || new URLSearchParams(location.search).get('terminal') === 'open') &&
    !['detail', 'full'].includes(
      new URLSearchParams(location.search).get('view') || '',
    )
  const isProjectTaskSession =
    rootSegment === 'projects' &&
    pathSegments[2] === 'tasks' &&
    Boolean(pathSegments[3]) &&
    (pathSegments[4] === 'sessions' || new URLSearchParams(location.search).get('terminal') === 'open') &&
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
      case 'schedules':
        return 'schedules'
      case 'agents':
        return 'settings'
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
      case 'chat':
        return 'chat'
      case 'studio':
        return 'studio'
      default:
        return 'chat'
    }
  }, [rootSegment])

  // Data States
  const [projects, setProjects] = useState<Project[]>([])
  const [projectsLoading, setProjectsLoading] = useState(true)
  const [projectsError, setProjectsError] = useState('')
  const [contacts, setContacts] = useState<ChatContact[]>([])
  const [directoryAgents, setDirectoryAgents] = useState<Agent[]>([])
  const [agentsLoaded, setAgentsLoaded] = useState(false)
  const [agentsError, setAgentsError] = useState(false)
  const [contactsLoading, setContactsLoading] = useState(true)
  const [contactsError, setContactsError] = useState('')
  useEffect(() => {
    if (rootSegment === 'team')
      navigate('/settings/team' + location.search, { replace: true })
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
      navigate('/settings/agent')
      setAgentCreateRequested(true)
    } else setNewFlow(kind)
  }

  // Filters & Mobile
  const [contactTab, setContactTab] = useState<ContactFilterTab>('all')
  const [contactSearch, setContactSearch] = useState('')
  const [selectedTaskStatus, setSelectedTaskStatus] = useState('all')
  const [isMobileChatActive, setIsMobileChatActive] = useState(false)

  // Chat is the default entry point.
  useEffect(() => {
    if (
      isAuthenticated &&
      (location.pathname === '/' ||
        location.pathname === '' ||
        location.pathname === '/login')
    ) {
      navigate('/chat', { replace: true })
    }
  }, [location.pathname, navigate, isAuthenticated])

  // Fetch Projects and Tasks
  const refreshProjects = useCallback(async () => {
    try {
      const data = await projectsApi.list()
      setProjects(previous => JSON.stringify(previous) === JSON.stringify(data) ? previous : data)
      setProjectsError('')
      setActiveProject(previous => {
        if (!previous) return undefined
        const next = data.find(project => project.id === previous.id) || previous
        return JSON.stringify(previous) === JSON.stringify(next) ? previous : next
      })
    } catch (err) {
      setProjectsError(
        'Could not load projects. Check your connection and try again.',
      )
    } finally {
      setProjectsLoading(false)
    }
  }, [])

  const refreshTasks = useCallback(async () => {
    window.dispatchEvent(new Event('cuan:tasks-changed'))
  }, [])

  const refreshContacts = useCallback(async () => {
    try {
      const list = await chatApi.listContacts()
      setContacts(previous => JSON.stringify(previous) === JSON.stringify(list) ? previous : list)
      setContactsError('')
    } catch { setContactsError('Could not load contacts. Check your connection and try again.') }
    finally { setContactsLoading(false) }
  }, [])
  useEffect(() => {
    const changed = (e?: Event) => {
      const custom = e as CustomEvent<{ roomId?: string; title?: string }> | undefined
      if (custom?.detail?.roomId && custom.detail.title) {
        const targetId = custom.detail.roomId
        const newTitle = custom.detail.title
        setContacts((prev) =>
          prev.map((c) => {
            if (
              c.conversation_id === targetId ||
              c.id === targetId ||
              c.id === 'topic_' + targetId
            ) {
              return { ...c, name: newTitle }
            }
            return c
          }),
        )
      }
      void refreshContacts()
    }
    window.addEventListener('cuan-conversation-resolution', changed)
    window.addEventListener('cuan-conversation-updated', changed)
    return () => {
      window.removeEventListener('cuan-conversation-resolution', changed)
      window.removeEventListener('cuan-conversation-updated', changed)
    }
  }, [refreshContacts])
  const refreshAgents = useCallback(async () => {
    try {
      const agents = await agentsApi.list()
      setAgentsError(false)
      setDirectoryAgents(previous => JSON.stringify(previous) === JSON.stringify(agents) ? previous : agents)
    } catch { setAgentsError(true) }
    finally { setAgentsLoaded(true) }
  }, [])

  useEffect(() => {
    if (isAuthenticated) {
      Promise.allSettled([refreshProjects(), refreshContacts(), refreshAgents()])
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
      const agentInterval = setInterval(() => { if (!document.hidden) void refreshAgents() }, 60000)
      document.addEventListener('visibilitychange', refresh)
      return () => {
        clearInterval(interval)
        clearInterval(agentInterval)
        document.removeEventListener('visibilitychange', refresh)
      }
    }
  }, [isAuthenticated, refreshProjects, refreshContacts, refreshAgents])

  // Task/session completion must become visible without reopening the list.
  useEffect(() => {
    if (
      !isAuthenticated ||
      isTerminalSession ||
      currentTab !== 'projects'
    )
      return
    let refreshing = false
    const refresh = async () => {
      if (document.visibilityState !== 'visible' || refreshing) return
      refreshing = true
      try {
        await Promise.allSettled([refreshTasks(), ...(currentTab === 'projects' ? [refreshProjects()] : [])])
      } finally {
        refreshing = false
      }
    }
    void refresh()
    const timer = setInterval(() => void refresh(), 5000)
    const resume = () => void refresh()
    window.addEventListener('focus', resume)
    document.addEventListener('visibilitychange', resume)
    return () => {
      clearInterval(timer)
      window.removeEventListener('focus', resume)
      document.removeEventListener('visibilitychange', resume)
    }
  }, [isAuthenticated, currentTab, isTerminalSession, refreshTasks, refreshProjects])

  // Read the visible conversation directly from the URL. Back/forward and
  // responsive list mode cannot race against an effect restoring old selection.
  const wide = useMediaQuery('(min-width: 1200px)')
  const visibleContact =
    subParam && (rootSegment === 'inbox' || rootSegment === 'chat')
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
  const activeDetailTaskId = useMemo(() => {
    if (rootSegment === 'tasks' && subParam && !sessionRoute) {
      return subParam
    }
    if (
      rootSegment === 'projects' &&
      pathSegments[2] === 'tasks' &&
      Boolean(pathSegments[3]) &&
      !isProjectTaskSession
    ) {
      return pathSegments[3]
    }
    return null
  }, [rootSegment, subParam, sessionRoute, pathSegments, isProjectTaskSession])

  useEffect(() => {
    let current = true
    setTaskRouteError('')
    if (!activeDetailTaskId) {
      setIsSidePeekOpen(false)
      if (rootSegment === 'tasks') setSelectedTask(null)
      return
    }
    if (!isAuthenticated) return
      tasksApi
        .get(activeDetailTaskId)
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
  }, [activeDetailTaskId, isAuthenticated, rootSegment])

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
    if (rootSegment === 'projects' && subParam && pathSegments[2] === 'terminal')
      navigate(
        '/projects/' +
          encodeURIComponent(subParam) +
          '/tasks?terminal=open',
        { replace: true, state: { ...((location.state as object) || {}), openTerminal: true } },
      )
  }, [rootSegment, subParam, location.pathname, navigate])

  // Deep Link Synchronization & Backward Compatibility Aliases (S59-T3):
  // /chat -> /inbox (and /chat/:roomId -> /inbox/:roomId)
  // /account/contacts -> /contacts
  // /settings/schedules -> /schedules
  useEffect(() => {
    if (rootSegment === 'chat' && subParam) {
      navigate('/inbox/' + encodeURIComponent(subParam) + location.search + location.hash, {
        replace: true,
        state: location.state,
      })
    } else if (rootSegment === 'chat') {
      navigate('/inbox' + location.search + location.hash, {
        replace: true,
        state: location.state,
      })
    }
    if (
      (rootSegment === 'account' || rootSegment === 'settings') &&
      subParam === 'contacts'
    ) {
      navigate('/contacts' + location.search + location.hash, {
        replace: true,
        state: location.state,
      })
    }
    if (rootSegment === 'schedules') {
      navigate('/settings/schedules' + location.search + location.hash, {
        replace: true,
        state: location.state,
      })
    }
    if (rootSegment === 'agents' || rootSegment === 'agent') {
      navigate('/settings/agent' + location.search + location.hash, {
        replace: true,
        state: location.state,
      })
    }
    // Canonical Route Consolidation: /projects/:projectId/tasks/:taskId -> /tasks/:taskId
    if (rootSegment === 'projects' && pathSegments[2] === 'tasks' && pathSegments[3]) {
      const taskId = pathSegments[3]
      const rest = pathSegments.slice(4).join('/')
      const target = `/tasks/${encodeURIComponent(taskId)}${rest ? '/' + rest : ''}${location.search}${location.hash}`
      navigate(target, {
        replace: true,
        state: {
          taskReturnTo: `/projects/${encodeURIComponent(subParam)}/tasks`,
          ...location.state,
        },
      })
    }
    if (rootSegment === 'account' && ['general', 'team', 'ai-providers', 'memory', 'agent', 'agents', 'apps-skills', 'integrations'].includes(subParam)) {
      navigate('/settings/' + encodeURIComponent(subParam) + location.search + location.hash, {
        replace: true,
        state: location.state,
      })
    }
  }, [rootSegment, subParam, location.search, location.hash, location.state, navigate])

  // Navigation handlers
  const handleSelectTab = (tab: AppTab) => {
    if ((tab as string) === 'studio') {
      navigate('/studio')
    } else if (tab === 'chat' || (tab as string) === 'studio' || tab === 'inbox') {
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
    } else if (tab === 'schedules') {
      navigate('/settings/schedules')
    } else if (tab === 'contacts') {
      navigate('/contacts')
    } else if (tab === 'agents') {
      navigate('/settings/agent')
    } else if (tab === 'team') {
      navigate('/settings/team')
    } else if (tab === 'skills') {
      navigate('/settings/apps-skills')
    } else if (tab === 'settings') {
      navigate('/settings')
    } else if (tab === 'account') {
      navigate('/account')
    } else {
      navigate(`/${tab}`)
    }
    if (tab !== 'inbox' && tab !== 'chat') {
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
      `/tasks/${encodeURIComponent(t.id)}${suffix}`,
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

    const taskReturn = location.state?.taskReturnTo
    if (
      typeof taskReturn === 'string' &&
      /^\/(?:tasks|projects\/[^/?#]+(?:\/tasks)?|inbox\/[^/?#]+)(?:\?[^#]*)?$/.test(
        taskReturn,
      ) &&
      !taskReturn.includes('view=')
    ) {
      navigate(taskReturn, { state: location.state })
      return
    }

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
    navigate(`/chat?project=${encodeURIComponent(created.id)}`)
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
      `/tasks/${encodeURIComponent(created.id)}/sessions/start${suffix}`,
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

  useEffect(() => {
    const handleTaskDeleted = () => {
      void refreshTasks()
    }
    window.addEventListener('cuan:task-deleted', handleTaskDeleted)
    return () => {
      window.removeEventListener('cuan:task-deleted', handleTaskDeleted)
    }
  }, [refreshTasks])

  const handleRunAgent = async (_taskId: string) => {
    await refreshTasks()
  }

  if (rootSegment === 'invite' && subParam === 'accept') {
    return (
      <Suspense fallback={<ViewFallback />}>
        <AcceptInvitePage />
      </Suspense>
    )
  }

  if (rootSegment === 'landing' || rootSegment === 'about') {
    return (
      <Suspense fallback={<ViewFallback />}>
        <LoginScreen />
      </Suspense>
    )
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
  const projectSettings = projectDetail && pathSegments[2] !== 'tasks'
  const settingTitles: Record<string, string> = {
    agent: 'Assistant & AI brains',
    'apps-skills': 'Apps & Skills',
    team: 'Team',
    agents: 'Assistant & AI brains',
    'ai-providers': 'Assistant & AI brains',
    memory: 'Agent Memory',
    integrations: 'Integrations',
    appearance: 'Appearance',
    profile: 'Profile',
    notifications: 'Notifications',
    contacts: 'Contacts',
    general: 'General',
    schedules: 'Schedules',
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
        : pathSegments[2] === 'github' ? 'GitHub' : pathSegments[2] === 'resend' ? 'Resend Email' : settingTitles[subParam]
      : currentTab === 'account'
        ? 'Account'
        : currentTab === 'studio'
          ? 'New work'
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
          ...(['live-chat', 'github', 'resend'].includes(pathSegments[2])
            ? [{ label: 'Integrations', to: '/settings/integrations' }]
            : []),
        ]
      : []

  // 2. Unauthenticated: Auth Gatekeeper
  if (!isAuthenticated) {
    return (
      <Suspense fallback={<ViewFallback />}>
        <LoginScreen />
      </Suspense>
    )
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
        inboxUnread={contacts.filter(contact => !contact.resolved && contact.has_messages !== false).length}
        onSelectTab={handleSelectTab}
        newAction={
          currentTab === 'notifications' ||
          projectDetail ||
          (currentTab === 'inbox' && (!!subParam || isMobileChatActive)) ||
          (currentTab === 'contacts' && !!new URLSearchParams(location.search).get('contact')) ||
          ((currentTab === 'settings' || currentTab === 'account') && subParam !== 'contacts') ||
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
            onNewStudio={() => {
              setActiveContact(undefined)
              navigate('/studio')
            }}
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
        <Suspense fallback={<ViewFallback />}>
          {currentTab === 'notifications' && <NotificationsView />}
          {currentTab === 'studio' && (
            <div className="h-full flex-1 overflow-y-auto px-4 py-6 md:px-8 bg-background">
              <StudioView
                onAccepted={() => void refreshContacts()}
                onConversationCreated={(id) => navigate('/inbox/' + encodeURIComponent(id))}
                agentSetup={
                  !agentsLoaded ? 'loading' : agentsError ? 'error' :
                  directoryAgents.some((a) => a.enabled !== false && a.status !== 'inactive')
                    ? 'ready' : 'needed'
                }
                firstGoal={!projectsLoading && !projectsError && projects.every(project => project.task_progress?.total === 0)}
                hasProject={
                  projectsLoading || !!projectsError ||
                  projects.some((p) => p.id !== 'default' || !!p.repo_url)
                }
                onAgentConnected={() => void refreshContacts()}
                onCreateProject={() => setIsCreateProjectOpen(true)}
                onRetrySetup={() => void refreshContacts()}
                checkRuntime={!directoryAgents.some((a) =>
                  a.enabled !== false && a.status !== 'inactive' &&
                  ['codex', 'claude'].includes(a.harness || a.provider || '')
                )}
                autoFocus={true}
              />
            </div>
          )}
          {currentTab === 'chat' && (
            <InboxView
              contacts={contacts}
              agents={directoryAgents}
              loading={contactsLoading}
              error={contactsError}
              onRetry={refreshContacts}
              activeContact={visibleContact}
              onSelectContact={handleSelectContact}
              onNewMessage={() => openNew('message')}
              onNewStudio={() => {
                setActiveContact(undefined)
                navigate('/studio')
              }}
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
              initialMode="studio"
              checkRuntime={!directoryAgents.some((a) =>
                a.enabled !== false && a.status !== 'inactive' &&
                ['codex', 'claude'].includes(a.harness || a.provider || '')
              )}
              agentSetup={
                !agentsLoaded ? 'loading' : agentsError ? 'error' :
                directoryAgents.some((a) => a.enabled !== false && a.status !== 'inactive')
                  ? 'ready' : 'needed'
              }
              firstGoal={!projectsLoading && !projectsError && projects.every(project => project.task_progress?.total === 0)}
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
              onNewStudio={() => {
                setActiveContact(undefined)
                navigate('/studio')
              }}
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
              initialMode={visibleContact ? 'chat' : 'studio'}
              checkRuntime={!directoryAgents.some((a) =>
                a.enabled !== false && a.status !== 'inactive' &&
                ['codex', 'claude'].includes(a.harness || a.provider || '')
              )}
              agentSetup={
                !agentsLoaded ? 'loading' : agentsError ? 'error' :
                directoryAgents.some((a) => a.enabled !== false && a.status !== 'inactive')
                  ? 'ready' : 'needed'
              }
              firstGoal={!projectsLoading && !projectsError && projects.every(project => project.task_progress?.total === 0)}
              hasProject={
                projectsLoading || !!projectsError ||
                projects.some((p) => p.id !== 'default' || !!p.repo_url)
              }
              onAgentConnected={() => void refreshContacts()}
              onCreateProject={() => setIsCreateProjectOpen(true)}
              onRetrySetup={() => void refreshContacts()}
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
                projects={projects}
                selectedStatusFilter={selectedTaskStatus}
                onStatusFilterChange={setSelectedTaskStatus}
                onSelectTask={handleSelectTask}
                onUpdateTaskStatus={(taskId, status) =>
                  handleUpdateTask(taskId, { status })
                }
                onDeleteTask={handleDeleteTask}
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
                  taskId={
                    pathSegments[2] === 'tasks' ? pathSegments[3] : undefined
                  }
                  sessionId={
                    pathSegments[4] === 'sessions' ? pathSegments[5] : undefined
                  }
                  loading={projectsLoading}
                  error={projectsError}
                  onRetry={() => void refreshTasks()}
                />
              )
            ) : (
              <ProjectsWorkspace
                  onNewProject={() => openNew('project')}
                projects={projects}
                loading={projectsLoading}
                error={projectsError}
                onRetry={() => {
                  void refreshProjects()
                  void refreshTasks()
                }}
                onArchiveProject={handleArchiveProject}
              />
            ))}

          {/* 4. Schedules Hub (Standalone fallback for /schedules) */}
          {currentTab === 'schedules' && (
            <SchedulesView />
          )}

          {/* 5. Settings / Account Configuration (Consolidated hub including /settings/schedules) */}
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
        </Suspense>
      </AppShell>

      {/* Modals & Drawers wrapped in Suspense */}
      <Suspense fallback={null}>
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
      </Suspense>
    </>
  )
}

export default App
