import { useEffect, useState, useCallback, useMemo } from 'react'
import { useLocation, useNavigate } from 'react-router-dom'
import { useAuth } from './context/AuthContext'
import { LoginScreen } from './components/auth/LoginScreen'
import { AppShell, AppTab } from './components/layout/AppShell'
import { ProjectsView } from './features/projects/ProjectsView'
import { CreateProjectModal } from './features/projects/CreateProjectModal'
import { CreateTaskModal } from './features/tasks/CreateTaskModal'
import { TaskSidePeek } from './features/tasks/TaskSidePeek'
import { TaskBoardView } from './features/tasks/TaskBoardView'
import { TasksSubPanel } from './features/tasks/TasksSubPanel'
import { TeamView, TeamSubPanel } from './features/team/TeamView'
import { SettingsView } from './features/settings/SettingsView'
import { projectsApi } from './api/projects'
import { tasksApi } from './api/tasks'
import { chatApi, ChatContact } from './api/chat'
import { InboxSubPanel, ContactFilterTab } from './features/inbox/InboxSubPanel'
import { InboxView } from './features/inbox/InboxView'
import { Project, Task, TaskStatus, TaskPriority, ProjectType, DoDItem } from './api/types'
import { Loader2 } from 'lucide-react'

export function App() {
  const { isAuthenticated, isLoading: authLoading } = useAuth()
  const location = useLocation()
  const navigate = useNavigate()

  // Parse URL pathname segments: /:tab/:id
  const pathSegments = useMemo(() => location.pathname.split('/').filter(Boolean), [location.pathname])
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
        return 'team'
      case 'settings':
        return 'settings'
      case 'inbox':
      default:
        return 'inbox'
    }
  }, [rootSegment])

  // Data States
  const [projects, setProjects] = useState<Project[]>([])
  const [tasks, setTasks] = useState<Task[]>([])
  const [contacts, setContacts] = useState<ChatContact[]>([])

  // Active selections
  const [activeProject, setActiveProject] = useState<Project | undefined>(undefined)
  const [activeContact, setActiveContact] = useState<ChatContact | undefined>(undefined)
  const [selectedTask, setSelectedTask] = useState<Task | null>(null)
  const [isSidePeekOpen, setIsSidePeekOpen] = useState(false)

  // Modals
  const [isCreateProjectOpen, setIsCreateProjectOpen] = useState(false)
  const [isCreateTaskOpen, setIsCreateTaskOpen] = useState(false)

  // Filters & Mobile
  const [contactTab, setContactTab] = useState<ContactFilterTab>('all')
  const [contactSearch, setContactSearch] = useState('')
  const [selectedTeamRole, setSelectedTeamRole] = useState('all')
  const [selectedTaskStatus, setSelectedTaskStatus] = useState('all')
  const [selectedTaskProject, setSelectedTaskProject] = useState('all')
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
      if (activeProject) {
        const updated = data.find((p) => p.id === activeProject.id)
        if (updated) setActiveProject(updated)
      }
    } catch (err) {
      console.error('Failed to load projects:', err)
    }
  }, [activeProject])

  const refreshTasks = useCallback(async () => {
    try {
      const projId = currentTab === 'projects' ? activeProject?.id : undefined
      const data = await tasksApi.list(projId)
      setTasks(data)
    } catch (err) {
      console.error('Failed to load tasks:', err)
    }
  }, [currentTab, activeProject])

  const refreshContacts = useCallback(async () => {
    try {
      const data = await chatApi.listContacts()
      setContacts(data)
    } catch (err) {
      console.error('Failed to load contacts:', err)
    }
  }, [])

  useEffect(() => {
    if (isAuthenticated) {
      Promise.allSettled([refreshProjects(), refreshTasks(), refreshContacts()])
      const interval = setInterval(refreshContacts, 5000)
      return () => clearInterval(interval)
    }
  }, [isAuthenticated, refreshProjects, refreshTasks, refreshContacts])

  // Deep Link Synchronization: /inbox/:conversationId
  useEffect(() => {
    if (rootSegment === 'inbox' && contacts.length > 0) {
      if (subParam) {
        const matched = contacts.find(
          (c) =>
            c.conversation_id === subParam ||
            c.id === subParam ||
            c.id === 'contact_' + subParam ||
            c.name.toLowerCase() === subParam.toLowerCase() ||
            ('@' + c.name.toLowerCase()) === subParam.toLowerCase()
        )
        if (matched) {
          setActiveContact(matched)
        }
      } else if (!activeContact) {
        const sm = contacts.find((c) => c.id === 'agent-sm' || c.name.toLowerCase().includes('scrum master'))
        setActiveContact(sm || contacts[0])
      }
    }
  }, [rootSegment, subParam, contacts, activeContact])

  // Deep Link Synchronization: /tasks/:taskId
  useEffect(() => {
    if (rootSegment === 'tasks' && tasks.length > 0) {
      if (subParam) {
        const matched = tasks.find((t) => t.id === subParam)
        if (matched) {
          setSelectedTask(matched)
          setIsSidePeekOpen(true)
        }
      } else {
        setIsSidePeekOpen(false)
        setSelectedTask(null)
      }
    }
  }, [rootSegment, subParam, tasks])

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
      navigate(activeContact ? `/inbox/${activeContact.conversation_id || activeContact.id}` : '/inbox')
    } else if (tab === 'tasks') {
      navigate(selectedTask && isSidePeekOpen ? `/tasks/${selectedTask.id}` : '/tasks')
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
    navigate(`/tasks/${t.id}`)
  }

  const handleCloseSidePeek = () => {
    setIsSidePeekOpen(false)
    setSelectedTask(null)
    navigate('/tasks')
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
    await tasksApi.runAgent(taskId)
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
        activeProject={activeProject}
        projects={projects}
        onSelectProject={handleSelectProject}
        onCreateProjectClick={() => setIsCreateProjectOpen(true)}
        hideBottomNav={currentTab === 'inbox' && isMobileChatActive}
        inboxSubPanel={
          <InboxSubPanel
            contacts={contacts}
            activeContactId={activeContact?.id}
            onSelectContact={handleSelectContact}
            selectedTab={contactTab}
            onTabChange={setContactTab}
            searchQuery={contactSearch}
            onSearchChange={setContactSearch}
          />
        }
        tasksSubPanel={
          <TasksSubPanel
            tasks={tasks}
            projects={projects}
            selectedStatus={selectedTaskStatus}
            onSelectStatus={setSelectedTaskStatus}
            selectedProject={selectedTaskProject}
            onSelectProject={setSelectedTaskProject}
            onCreateTaskClick={() => setIsCreateTaskOpen(true)}
          />
        }
        teamSubPanel={
          <TeamSubPanel
            selectedRole={selectedTeamRole}
            onSelectRole={setSelectedTeamRole}
          />
        }
      >
        {/* 1. Messenger View — Inbox */}
        {currentTab === 'inbox' && (
          <InboxView
            contacts={contacts}
            activeContact={activeContact}
            onSelectContact={handleSelectContact}
            onContactUpdated={refreshContacts}
            contactTab={contactTab}
            onContactTabChange={setContactTab}
            contactSearch={contactSearch}
            onContactSearchChange={setContactSearch}
            onMobileChatActiveChange={setIsMobileChatActive}
            onOpenMobileDrawer={() => setIsMobileDrawerOpen(true)}
          />
        )}

        {/* 2. Global Task Board View — Tasks */}
        {currentTab === 'tasks' && (
          <div className="h-full p-4 md:p-6 overflow-hidden">
            <TaskBoardView
              tasks={tasks}
              projects={projects}
              activeProject={selectedTaskProject !== 'all' ? projects.find((p) => p.id === selectedTaskProject) : undefined}
              selectedStatusFilter={selectedTaskStatus}
              onSelectTask={handleSelectTask}
              onCreateTask={() => setIsCreateTaskOpen(true)}
              onUpdateTaskStatus={(taskId, status) => handleUpdateTask(taskId, { status })}
              onRunAgent={handleRunAgent}
            />
          </div>
        )}

        {/* 3. Projects Workspace & Repositories */}
        {currentTab === 'projects' && (
          <ProjectsView
            projects={projects}
            tasks={tasks}
            onSelectProject={handleSelectProject}
            onCreateProject={() => setIsCreateProjectOpen(true)}
            onArchiveProject={handleArchiveProject}
          />
        )}

        {/* 4. Unified Team Directory — Humans + AI Agents */}
        {currentTab === 'team' && (
          <div className="h-full p-4 md:p-6 overflow-y-auto">
            <TeamView
              selectedRole={selectedTeamRole}
              onAgentAdded={refreshContacts}
            />
          </div>
        )}

        {/* 5. Settings Configuration */}
        {currentTab === 'settings' && (
          <div className="h-full p-4 md:p-6 overflow-y-auto">
            <SettingsView
              activeProject={activeProject}
              onUpdateProject={async (id, data) => {
                await projectsApi.update(id, data)
                await refreshProjects()
              }}
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
        projects={projects}
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
