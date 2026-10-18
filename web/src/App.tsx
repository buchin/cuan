import { useEffect, useState, useCallback } from 'react'
import { useAuth } from './context/AuthContext'
import { LoginScreen } from './components/auth/LoginScreen'
import { AppShell } from './components/layout/AppShell'
import { ProjectsView } from './features/projects/ProjectsView'
import { CreateProjectModal } from './features/projects/CreateProjectModal'
import { TaskBoardView } from './features/tasks/TaskBoardView'
import { CreateTaskModal } from './features/tasks/CreateTaskModal'
import { TaskSidePeek } from './features/tasks/TaskSidePeek'
import { AgentsView } from './features/agents/AgentsView'
import { SettingsView } from './features/settings/SettingsView'
import { projectsApi } from './api/projects'
import { tasksApi } from './api/tasks'
import { Project, Task, TaskStatus, TaskPriority, ProjectType } from './api/types'
import { Loader2 } from 'lucide-react'

export function App() {
  const { isAuthenticated, isLoading: authLoading } = useAuth()

  // Navigation State
  const [currentTab, setCurrentTab] = useState<'projects' | 'tasks' | 'agents' | 'settings'>('projects')
  const [activeProject, setActiveProject] = useState<Project | undefined>(undefined)

  // Data States
  const [projects, setProjects] = useState<Project[]>([])
  const [tasks, setTasks] = useState<Task[]>([])

  // Modals & Panels
  const [isCreateProjectOpen, setIsCreateProjectOpen] = useState(false)
  const [isCreateTaskOpen, setIsCreateTaskOpen] = useState(false)
  const [selectedTask, setSelectedTask] = useState<Task | null>(null)
  const [isSidePeekOpen, setIsSidePeekOpen] = useState(false)

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

  useEffect(() => {
    if (isAuthenticated) {
      Promise.allSettled([refreshProjects(), refreshTasks()])
    }
  }, [isAuthenticated, refreshProjects, refreshTasks])

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
    setCurrentTab('tasks')
  }

  const handleArchiveProject = async (id: string, isArchived: boolean) => {
    await projectsApi.archive(id, isArchived)
    await refreshProjects()
  }

  const handleDeleteProject = async (id: string) => {
    await projectsApi.delete(id)
    if (activeProject?.id === id) {
      setActiveProject(undefined)
    }
    await refreshProjects()
    setCurrentTab('projects')
  }

  const handleUpdateProject = async (id: string, updates: any) => {
    await projectsApi.update(id, updates)
    await refreshProjects()
  }

  // Handlers for Tasks
  const handleCreateTask = async (data: {
    title: string
    description?: string
    project_ids?: string[]
    priority: TaskPriority
    status: TaskStatus
    dod_checklist?: string[]
  }) => {
    await tasksApi.create(data)
    await refreshTasks()
  }

  const handleUpdateTaskStatus = async (taskId: string, status: TaskStatus) => {
    // Optimistic update
    setTasks((prev) =>
      prev.map((t) => (t.id === taskId ? { ...t, status } : t))
    )
    try {
      await tasksApi.updateStatus(taskId, status)
    } catch (err) {
      await refreshTasks()
    }
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

  const handleOpenTaskDetail = (task: Task) => {
    setSelectedTask(task)
    setIsSidePeekOpen(true)
  }

  // 1. Loading Auth State
  if (authLoading) {
    return (
      <div className="h-screen w-screen flex items-center justify-center bg-[#eaeef6] dark:bg-[#1a1b26] text-[#7aa2f7] font-mono">
        <div className="flex flex-col items-center gap-3">
          <Loader2 className="h-8 w-8 animate-spin" />
          <span className="text-xs uppercase tracking-widest text-[#565f89]">
            Initializing Cuan Workplace...
          </span>
        </div>
      </div>
    )
  }

  // 2. Unauthenticated: Auth Gatekeeper
  if (!isAuthenticated) {
    return <LoginScreen />
  }

  // 3. Authenticated: Linear-Class App Shell
  return (
    <>
      <AppShell
        currentTab={currentTab}
        onSelectTab={setCurrentTab}
        activeProject={activeProject}
        projects={projects}
        onSelectProject={(p) => {
          setActiveProject(p)
          if (p) setCurrentTab('tasks')
        }}
        onCreateProjectClick={() => setIsCreateProjectOpen(true)}
      >
        {currentTab === 'projects' && (
          <ProjectsView
            projects={projects}
            tasks={tasks}
            onSelectProject={(p) => {
              setActiveProject(p)
              setCurrentTab('tasks')
            }}
            onCreateProject={() => setIsCreateProjectOpen(true)}
            onArchiveProject={handleArchiveProject}
          />
        )}

        {currentTab === 'tasks' && (
          <TaskBoardView
            tasks={tasks}
            projects={projects}
            activeProject={activeProject}
            onSelectTask={handleOpenTaskDetail}
            onCreateTask={() => setIsCreateTaskOpen(true)}
            onUpdateTaskStatus={handleUpdateTaskStatus}
            onRunAgent={handleRunAgent}
          />
        )}

        {currentTab === 'agents' && <AgentsView />}

        {currentTab === 'settings' && (
          <SettingsView
            activeProject={activeProject}
            onUpdateProject={handleUpdateProject}
            onDeleteProject={handleDeleteProject}
          />
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
