import { useEffect, useState, useCallback } from 'react'
import { useAuth } from './context/AuthContext'
import { LoginScreen } from './components/auth/LoginScreen'
import { AppShell, AppTab } from './components/layout/AppShell'
import { ProjectsView } from './features/projects/ProjectsView'
import { CreateProjectModal } from './features/projects/CreateProjectModal'
import { CreateTaskModal } from './features/tasks/CreateTaskModal'
import { TaskSidePeek } from './features/tasks/TaskSidePeek'
import { AgentsView, AgentsSubPanel } from './features/agents/AgentsView'
import { TeamView, TeamSubPanel } from './features/team/TeamView'
import { projectsApi } from './api/projects'
import { tasksApi } from './api/tasks'
import { inboxApi, InboxThreadItem } from './api/inbox'
import { InboxSubPanel } from './features/inbox/InboxSubPanel'
import { InboxView } from './features/inbox/InboxView'
import { CreateThreadModal } from './features/inbox/CreateThreadModal'
import { Project, Task, TaskStatus, TaskPriority, ProjectType, DoDItem } from './api/types'
import { Loader2 } from 'lucide-react'

export function App() {
  const { isAuthenticated, isLoading: authLoading } = useAuth()

  // Navigation State
  const [currentTab, setCurrentTab] = useState<AppTab>('inbox')
  const [activeProject, setActiveProject] = useState<Project | undefined>(undefined)

  // Data States
  const [projects, setProjects] = useState<Project[]>([])
  const [tasks, setTasks] = useState<Task[]>([])

  // Modals & Panels
  const [isCreateProjectOpen, setIsCreateProjectOpen] = useState(false)
  const [isCreateTaskOpen, setIsCreateTaskOpen] = useState(false)
  const [selectedTask, setSelectedTask] = useState<Task | null>(null)
  const [isSidePeekOpen, setIsSidePeekOpen] = useState(false)

  // Inbox State
  const [threads, setThreads] = useState<InboxThreadItem[]>([])
  const [activeThread, setActiveThread] = useState<InboxThreadItem | undefined>(undefined)
  const [inboxStatusFilter, setInboxStatusFilter] = useState('all')
  const [isCreateThreadOpen, setIsCreateThreadOpen] = useState(false)
  const [selectedTeamRole, setSelectedTeamRole] = useState('all')

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
      const data = await tasksApi.list(activeProject?.id)
      setTasks(data)
    } catch (err) {
      console.error('Failed to load tasks:', err)
    }
  }, [activeProject])

  const refreshThreads = useCallback(async (status?: string) => {
    try {
      const data = await inboxApi.list({ status: status ?? inboxStatusFilter })
      setThreads(data)
      if (activeThread) {
        const updated = data.find((t) => t.id === activeThread.id)
        if (updated) setActiveThread(updated)
      } else if (data.length > 0) {
        setActiveThread(data[0])
      }
    } catch (err) {
      console.error('Failed to load threads:', err)
    }
  }, [activeThread, inboxStatusFilter])

  useEffect(() => {
    if (isAuthenticated) {
      Promise.allSettled([refreshProjects(), refreshTasks(), refreshThreads()])
    }
  }, [isAuthenticated, refreshProjects, refreshTasks, refreshThreads])

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
    setCurrentTab('projects')
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
    setIsSidePeekOpen(false)
    setSelectedTask(null)
  }

  const handleRunAgent = async (taskId: string) => {
    await tasksApi.runAgent(taskId)
    await refreshTasks()
  }

  const handleCreateThread = async (data: { title: string; message: string; project_id?: string }) => {
    const res = await inboxApi.create(data)
    await refreshThreads()
    if (res.thread) {
      setActiveThread(res.thread)
    }
    setCurrentTab('inbox')
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
        onSelectTab={setCurrentTab}
        activeProject={activeProject}
        projects={projects}
        onSelectProject={(p) => {
          setActiveProject(p)
          if (p) setCurrentTab('projects')
        }}
        onCreateProjectClick={() => setIsCreateProjectOpen(true)}
        inboxSubPanel={
          <InboxSubPanel
            threads={threads}
            activeThreadId={activeThread?.id}
            onSelectThread={(t) => setActiveThread(t)}
            onCreateThreadClick={() => setIsCreateThreadOpen(true)}
            statusFilter={inboxStatusFilter}
            onStatusFilterChange={(status) => {
              setInboxStatusFilter(status)
              refreshThreads(status)
            }}
          />
        }
        agentsSubPanel={<AgentsSubPanel />}
        teamSubPanel={
          <TeamSubPanel
            selectedRole={selectedTeamRole}
            onSelectRole={setSelectedTeamRole}
          />
        }
      >
        {/* Inbox View — S17.2 & S17.3 */}
        {currentTab === 'inbox' && (
          <InboxView
            activeThread={activeThread}
            onThreadUpdated={refreshThreads}
            onCreateThreadClick={() => setIsCreateThreadOpen(true)}
          />
        )}

        {currentTab === 'projects' && (
          <ProjectsView
            projects={projects}
            tasks={tasks}
            onSelectProject={(p) => {
              setActiveProject(p)
              setCurrentTab('projects')
            }}
            onCreateProject={() => setIsCreateProjectOpen(true)}
            onArchiveProject={handleArchiveProject}
          />
        )}

        {currentTab === 'agents' && <AgentsView />}

        {/* Team Directory — S17.4 */}
        {currentTab === 'team' && <TeamView selectedRole={selectedTeamRole} />}
      </AppShell>

      {/* Modals & Drawers */}
      <CreateThreadModal
        isOpen={isCreateThreadOpen}
        projects={projects}
        onClose={() => setIsCreateThreadOpen(false)}
        onSubmit={handleCreateThread}
      />

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
        onClose={() => {
          setIsSidePeekOpen(false)
          setSelectedTask(null)
        }}
        onUpdate={handleUpdateTask}
        onDelete={handleDeleteTask}
        onRunAgent={handleRunAgent}
      />
    </>
  )
}

export default App
