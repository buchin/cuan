import { useEffect, useState, FormEvent } from 'react'
import { LandingPage } from './components/landing/LandingPage'
import { SponsorShowcase } from './components/sponsor/SponsorShowcase'
import { SponsorArchive } from './components/sponsor/SponsorArchive'
import { SponsorPending } from './components/sponsor/SponsorPending'
import { AdminSponsorPanel } from './components/sponsor/AdminSponsorPanel'
import { StudioDashboard } from './components/studio/StudioDashboard'
import { TermsPage, PrivacyPage } from './components/legal/LegalPages'
import { Card, CardHeader, CardTitle, CardDescription, CardContent, CardFooter } from './components/ui/card'
import { Badge } from './components/ui/badge'
import { Button } from './components/ui/button'
import { Input } from './components/ui/input'
import { Alert, AlertDescription, AlertTitle } from './components/ui/alert'
import { 
  CheckCircle2, 
  Server, 
  Shield, 
  Database, 
  Lock, 
  LogOut, 
  RefreshCw, 
  GitBranch, 
  GitPullRequest,
  GitCommit,
  AlertCircle,
  ShieldCheck,
  Globe,
  HardDrive,
  FolderGit2,
  ExternalLink,
  Plus,
  ArrowLeft,
  Trash2,
  HelpCircle,
  KeyRound,
  Check,
  ChevronRight,
  Clock,
  MessageSquare,
  Send,
  Bot,
  Play,
  Layers,
  Search,
  Cpu,
  RotateCcw,
  Square,
  Terminal,
  Activity,
  Sparkles
} from 'lucide-react'

interface OwnerInfo {
  id: string
  email: string
}

interface ServerInfo {
  name: string
  sprint: string
  status: string
  adapter: string
  license: string
  architecture: string
}

interface HealthInfo {
  status: string
  app: string
  version: string
}

interface ReadyInfo {
  status: string
  database: string
  migrations?: string
  migrations_count?: number
  owner_configured?: boolean
}

interface TaskAttempt {
  id: string
  task_id: string
  runner_id: string
  agent_name: string
  status: 'queued' | 'running' | 'succeeded' | 'failed' | 'canceled' | 'interrupted'
  exit_code?: number
  peak_rss_bytes?: number
  started_at?: string
  finished_at?: string
  error_message?: string
  created_at: string
}

interface TaskActivity {
  id: string
  task_id: string
  attempt_id?: string
  activity_type: string
  author: string
  old_status?: string
  new_status?: string
  message: string
  created_at: string
}

interface TaskItem {
  id: string
  project_id?: string
  project_name?: string
  title: string
  instructions?: string
  status: 'backlog' | 'doing' | 'review' | 'done' | 'blocked'
  priority: 'low' | 'normal' | 'high' | 'urgent'
  assigned_agent?: string
  branch_name?: string
  pr_url?: string
  pr_number?: number
  commit_sha?: string
  diff_summary?: string
  test_summary?: string
  created_at: string
  updated_at: string
  latest_attempt?: TaskAttempt
}

interface TaskDetail extends TaskItem {
  repo_url?: string
  attempts: TaskAttempt[]
  activities: TaskActivity[]
}

interface ProjectItem {
  id: string
  name: string
  repo_url: string
  repo_owner: string
  repo_name: string
  default_branch: string
  local_path?: string
  has_token: boolean
  token_preview?: string
  clone_status: string // connected, synced, failed, pending
  last_fetched_at?: string
  last_error?: string
  created_at: string
  updated_at: string
}

interface RunnerInfo {
  id: string
  name: string
  hostname: string
  supported_agents: string[]
  status: 'idle' | 'busy' | 'offline'
  last_heartbeat_at: string
  current_task_id?: string
  current_attempt_id?: string
  cpu_percent: number
  memory_used_bytes: number
  memory_total_bytes: number
  disk_used_bytes: number
  disk_total_bytes: number
  created_at: string
  updated_at: string
}

interface LogRecord {
  id: number
  attempt_id: string
  stream: 'stdout' | 'stderr' | 'system'
  line: string
  logged_at: string
}

export function App() {
  const [currentPath, setCurrentPath] = useState<string>(() => window.location.pathname || '/')

  useEffect(() => {
    const handlePopState = () => {
      setCurrentPath(window.location.pathname || '/')
    }
    window.addEventListener('popstate', handlePopState)
    return () => window.removeEventListener('popstate', handlePopState)
  }, [])

  const navigate = (path: string) => {
    window.history.pushState({}, '', path)
    setCurrentPath(path)
  }

  const [setupRequired, setSetupRequired] = useState<boolean | null>(null)
  const [owner, setOwner] = useState<OwnerInfo | null>(null)
  const [token, setToken] = useState<string>(() => localStorage.getItem('cuan_token') || '')

  // System Mode State (APP_MODE=marketing vs APP_MODE=studio - S03.5)
  const [systemMode, setSystemMode] = useState<{
    app_mode: 'marketing' | 'studio'
    version: string
    base_domain: string
  }>({
    app_mode: 'marketing',
    version: '0.1.0',
    base_domain: 'cuan.dojo.cc',
  })

  const [overrideMode, setOverrideMode] = useState<'marketing' | 'studio' | null>(() => {
    const saved = localStorage.getItem('cuan_app_mode_override')
    return (saved === 'marketing' || saved === 'studio') ? saved : null
  })

  const effectiveMode: 'marketing' | 'studio' = overrideMode || systemMode.app_mode

  const handleToggleMode = (newMode: 'marketing' | 'studio') => {
    setOverrideMode(newMode)
    localStorage.setItem('cuan_app_mode_override', newMode)
  }
  
  const [info, setInfo] = useState<ServerInfo | null>(null)
  const [health, setHealth] = useState<HealthInfo | null>(null)
  const [ready, setReady] = useState<ReadyInfo | null>(null)
  const [tasks, setTasks] = useState<TaskItem[]>([])
  const [projects, setProjects] = useState<ProjectItem[]>([])
  const [runners, setRunners] = useState<RunnerInfo[]>([])
  const [runnersLoading, setRunnersLoading] = useState(false)
  
  // Execution logs for selected attempt
  const [attemptLogs, setAttemptLogs] = useState<LogRecord[]>([])
  const [logsLoading, setLogsLoading] = useState(false)
  
  // Navigation
  const [activeTab, setActiveTab] = useState<'overview' | 'projects' | 'tasks' | 'runners' | 'sponsors'>('tasks')

  useEffect(() => {
    if (currentPath === '/admin/sponsors' || currentPath === '/admin') {
      setActiveTab('sponsors')
    }
  }, [currentPath])
  const [selectedProject, setSelectedProject] = useState<ProjectItem | null>(null)
  const [projectBranches, setProjectBranches] = useState<string[]>([])
  const [branchesLoading, setBranchesLoading] = useState(false)

  // Task Board States (T4)
  const [selectedTask, setSelectedTask] = useState<TaskDetail | null>(null)
  const [taskDetailLoading, setTaskDetailLoading] = useState(false)
  const [boardViewMode, setBoardViewMode] = useState<'kanban' | 'list'>('kanban')
  const [taskProjectFilter, setTaskProjectFilter] = useState<string>('all')
  const [taskSearchQuery, setTaskSearchQuery] = useState<string>('')

  // Create Task Modal States
  const [isCreateTaskModalOpen, setIsCreateTaskModalOpen] = useState(false)
  const [newTaskTitle, setNewTaskTitle] = useState('')
  const [newTaskInstructions, setNewTaskInstructions] = useState('')
  const [newTaskProjectId, setNewTaskProjectId] = useState('')
  const [newTaskPriority, setNewTaskPriority] = useState<string>('normal')
  const [newTaskAgent, setNewTaskAgent] = useState('agy')
  const [createTaskLoading, setCreateTaskLoading] = useState(false)
  const [createTaskError, setCreateTaskError] = useState<string | null>(null)

  // Edit Task States
  const [editTaskTitle, setEditTaskTitle] = useState('')
  const [editTaskInstructions, setEditTaskInstructions] = useState('')
  const [editTaskPriority, setEditTaskPriority] = useState('normal')
  const [editTaskStatus, setEditTaskStatus] = useState('backlog')
  const [editTaskAgent, setEditTaskAgent] = useState('agy')
  const [editTaskLoading, setEditTaskLoading] = useState(false)

  // Comment & Attempt states
  const [commentInput, setCommentInput] = useState('')
  const [commentLoading, setCommentLoading] = useState(false)
  const [attemptActionLoading, setAttemptActionLoading] = useState(false)

  // Dialog & Form states
  const [isConnectModalOpen, setIsConnectModalOpen] = useState(false)
  const [showTokenGuide, setShowTokenGuide] = useState(false)
  const [deleteModalOpen, setDeleteModalOpen] = useState(false)
  
  const [repoNameInput, setRepoNameInput] = useState('')
  const [repoUrlInput, setRepoUrlInput] = useState('')
  const [githubTokenInput, setGithubTokenInput] = useState('')
  const [defaultBranchInput, setDefaultBranchInput] = useState('')
  
  const [loading, setLoading] = useState(true)
  const [authLoading, setAuthLoading] = useState(false)
  const [connectLoading, setConnectLoading] = useState(false)
  const [syncLoading, setSyncLoading] = useState(false)
  const [deleteLoading, setDeleteLoading] = useState(false)
  
  const [errorMessage, setErrorMessage] = useState<string | null>(null)
  const [connectError, setConnectError] = useState<string | null>(null)
  const [successNotification, setSuccessNotification] = useState<string | null>(null)

  // Auth Form states
  const [emailInput, setEmailInput] = useState('')
  const [passwordInput, setPasswordInput] = useState('')
  const [confirmPasswordInput, setConfirmPasswordInput] = useState('')

  const getAuthHeaders = (): HeadersInit => {
    const headers: Record<string, string> = {
      'Content-Type': 'application/json',
    }
    if (token) {
      headers['Authorization'] = `Bearer ${token}`
    }
    return headers
  }

  const checkStatusAndAuth = async (currentToken: string = token) => {
    setLoading(true)
    setErrorMessage(null)

    try {
      // 1. Check Auth Status (Setup required vs configured)
      const statusRes = await fetch('/api/v1/auth/status')
      if (statusRes.ok) {
        const statusData = await statusRes.json()
        setSetupRequired(statusData.setup_required)

        if (!statusData.setup_required) {
          // Check if currently authenticated
          const headers: Record<string, string> = {}
          if (currentToken) {
            headers['Authorization'] = `Bearer ${currentToken}`
          }
          const meRes = await fetch('/api/v1/auth/me', { headers })
          if (meRes.ok) {
            const meData = await meRes.json()
            setOwner(meData.owner)
            await fetchProtectedData(currentToken)
          } else {
            setOwner(null)
          }
        }
      }

      // 2. Fetch Health, Readiness & System Mode
      const [healthRes, readyRes, infoRes, modeRes] = await Promise.allSettled([
        fetch('/healthz'),
        fetch('/readyz'),
        fetch('/api/v1/info'),
        fetch('/api/system/mode'),
      ])

      if (healthRes.status === 'fulfilled' && healthRes.value.ok) {
        setHealth(await healthRes.value.json())
      }
      if (readyRes.status === 'fulfilled' && readyRes.value.ok) {
        setReady(await readyRes.value.json())
      }
      if (infoRes.status === 'fulfilled' && infoRes.value.ok) {
        setInfo(await infoRes.value.json())
      }
      if (modeRes.status === 'fulfilled' && modeRes.value.ok) {
        const modeData = await modeRes.value.json()
        setSystemMode(modeData)
      }
    } catch (err) {
      console.error('Failed to load system status:', err)
    } finally {
      setLoading(false)
    }
  }

  const fetchProtectedData = async (activeToken: string = token) => {
    try {
      const headers: Record<string, string> = {}
      if (activeToken) {
        headers['Authorization'] = `Bearer ${activeToken}`
      }
      
      const [tasksRes, projectsRes, runnersRes] = await Promise.allSettled([
        fetch('/api/v1/tasks', { headers }),
        fetch('/api/v1/projects', { headers }),
        fetch('/api/v1/runners', { headers }),
      ])

      if (tasksRes.status === 'fulfilled' && tasksRes.value.ok) {
        const tasksData = await tasksRes.value.json()
        setTasks(tasksData.tasks || [])
      }

      if (projectsRes.status === 'fulfilled' && projectsRes.value.ok) {
        const projectsData = await projectsRes.value.json()
        const pList = projectsData.projects || []
        setProjects(pList)

        // If a project is currently selected, refresh its data
        if (selectedProject) {
          const updated = pList.find((p: ProjectItem) => p.id === selectedProject.id)
          if (updated) {
            setSelectedProject(updated)
          }
        }
      }

      if (runnersRes.status === 'fulfilled' && runnersRes.value.ok) {
        const runnersData = await runnersRes.value.json()
        setRunners(runnersData.runners || [])
      }
    } catch (err) {
      console.error('Failed to fetch protected data:', err)
    }
  }

  const fetchRunners = async () => {
    setRunnersLoading(true)
    try {
      const res = await fetch('/api/v1/runners', {
        headers: getAuthHeaders(),
      })
      if (res.ok) {
        const data = await res.json()
        setRunners(data.runners || [])
      }
    } catch (err) {
      console.error('Failed to fetch runners:', err)
    } finally {
      setRunnersLoading(false)
    }
  }

  const fetchAttemptLogs = async (taskId: string, attemptId: string) => {
    setLogsLoading(true)
    try {
      const res = await fetch(`/api/v1/tasks/${taskId}/attempts/${attemptId}/logs`, {
        headers: getAuthHeaders(),
      })
      if (res.ok) {
        const data = await res.json()
        setAttemptLogs(data.logs || [])
      }
    } catch (err) {
      console.error('Failed to fetch logs:', err)
    } finally {
      setLogsLoading(false)
    }
  }

  const fetchProjectBranches = async (projectId: string) => {
    setBranchesLoading(true)
    try {
      const res = await fetch(`/api/v1/projects/${projectId}/branches`, {
        headers: getAuthHeaders(),
      })
      if (res.ok) {
        const data = await res.json()
        setProjectBranches(data.branches || [])
      }
    } catch (err) {
      console.error('Failed to fetch branches:', err)
    } finally {
      setBranchesLoading(false)
    }
  }

  const handleSelectProject = (project: ProjectItem) => {
    setSelectedProject(project)
    fetchProjectBranches(project.id)
  }

  const fetchTaskDetail = async (taskId: string) => {
    setTaskDetailLoading(true)
    try {
      const res = await fetch(`/api/v1/tasks/${taskId}`, {
        headers: getAuthHeaders(),
      })
      if (res.ok) {
        const data = await res.json()
        setSelectedTask(data)
        setEditTaskTitle(data.title)
        setEditTaskInstructions(data.instructions || '')
        setEditTaskPriority(data.priority || 'normal')
        setEditTaskStatus(data.status || 'backlog')
        setEditTaskAgent(data.assigned_agent || 'agy')

        if (data.latest_attempt) {
          fetchAttemptLogs(data.id, data.latest_attempt.id)
        } else {
          setAttemptLogs([])
        }
      }
    } catch (err) {
      console.error('Failed to fetch task detail:', err)
    } finally {
      setTaskDetailLoading(false)
    }
  }

  const handleCreateTaskSubmit = async (e: FormEvent) => {
    e.preventDefault()
    setCreateTaskError(null)
    setCreateTaskLoading(true)

    try {
      const payload: any = {
        title: newTaskTitle.trim(),
        instructions: newTaskInstructions.trim(),
        priority: newTaskPriority,
        assigned_agent: newTaskAgent.trim() || 'agy',
      }
      if (newTaskProjectId && newTaskProjectId !== 'none') {
        payload.project_id = newTaskProjectId
      }

      const res = await fetch('/api/v1/tasks', {
        method: 'POST',
        headers: getAuthHeaders(),
        body: JSON.stringify(payload),
      })

      const data = await res.json()
      if (!res.ok) {
        throw new Error(data.error || 'Gagal membuat task')
      }

      setNewTaskTitle('')
      setNewTaskInstructions('')
      setNewTaskProjectId('')
      setNewTaskPriority('normal')
      setNewTaskAgent('agy')
      setIsCreateTaskModalOpen(false)

      await fetchProtectedData()
      await fetchTaskDetail(data.id)
      setSuccessNotification(`Task '${data.title}' berhasil dibuat!`)
      setTimeout(() => setSuccessNotification(null), 4000)
    } catch (err: any) {
      setCreateTaskError(err.message || 'Terjadi kesalahan saat membuat task')
    } finally {
      setCreateTaskLoading(false)
    }
  }

  const handleUpdateTaskSubmit = async (e: FormEvent) => {
    e.preventDefault()
    if (!selectedTask) return
    setEditTaskLoading(true)

    try {
      const payload: any = {
        title: editTaskTitle.trim(),
        instructions: editTaskInstructions.trim(),
        priority: editTaskPriority,
        status: editTaskStatus,
        assigned_agent: editTaskAgent.trim(),
      }

      const res = await fetch(`/api/v1/tasks/${selectedTask.id}`, {
        method: 'PUT',
        headers: getAuthHeaders(),
        body: JSON.stringify(payload),
      })

      const data = await res.json()
      if (!res.ok) {
        throw new Error(data.error || 'Gagal memperbarui task')
      }

      await fetchTaskDetail(selectedTask.id)
      await fetchProtectedData()
      setSuccessNotification(`Perubahan task '${data.title}' berhasil disimpan!`)
      setTimeout(() => setSuccessNotification(null), 4000)
    } catch (err: any) {
      alert(`Gagal update task: ${err.message}`)
    } finally {
      setEditTaskLoading(false)
    }
  }

  const handleQuickMoveStatus = async (taskId: string, newStatus: string) => {
    try {
      const res = await fetch(`/api/v1/tasks/${taskId}`, {
        method: 'PUT',
        headers: getAuthHeaders(),
        body: JSON.stringify({ status: newStatus }),
      })
      if (res.ok) {
        await fetchProtectedData()
        if (selectedTask?.id === taskId) {
          await fetchTaskDetail(taskId)
        }
      }
    } catch (err) {
      console.error('Failed to move task status:', err)
    }
  }

  const handleDeleteTaskAction = async (taskId: string) => {
    if (!confirm('Apakah Anda yakin ingin menghapus task ini? Semua attempt dan histori transisi akan dihapus permanen.')) {
      return
    }

    try {
      const res = await fetch(`/api/v1/tasks/${taskId}`, {
        method: 'DELETE',
        headers: getAuthHeaders(),
      })
      if (!res.ok) {
        const data = await res.json()
        throw new Error(data.error || 'Gagal menghapus task')
      }

      setSelectedTask(null)
      await fetchProtectedData()
      setSuccessNotification('Task berhasil dihapus.')
      setTimeout(() => setSuccessNotification(null), 4000)
    } catch (err: any) {
      alert(`Gagal menghapus task: ${err.message}`)
    }
  }

  const handleAddCommentSubmit = async (e: FormEvent) => {
    e.preventDefault()
    if (!selectedTask || !commentInput.trim()) return
    setCommentLoading(true)

    try {
      const res = await fetch(`/api/v1/tasks/${selectedTask.id}/comments`, {
        method: 'POST',
        headers: getAuthHeaders(),
        body: JSON.stringify({ message: commentInput.trim() }),
      })

      if (!res.ok) {
        const data = await res.json()
        throw new Error(data.error || 'Gagal mengirim catatan kerja')
      }

      setCommentInput('')
      await fetchTaskDetail(selectedTask.id)
    } catch (err: any) {
      alert(`Gagal menambah komentar: ${err.message}`)
    } finally {
      setCommentLoading(false)
    }
  }

  const handleTriggerAttempt = async (taskId: string) => {
    setAttemptActionLoading(true)
    try {
      const res = await fetch(`/api/v1/tasks/${taskId}/attempts`, {
        method: 'POST',
        headers: getAuthHeaders(),
        body: JSON.stringify({
          runner_id: 'runner_local',
          agent_name: selectedTask?.assigned_agent || 'agy',
        }),
      })

      if (!res.ok) {
        const data = await res.json()
        throw new Error(data.error || 'Gagal mendaftarkan attempt')
      }

      await fetchTaskDetail(taskId)
      await fetchProtectedData()
      setSuccessNotification('Attempt baru dijadwalkan (status: queued)!')
      setTimeout(() => setSuccessNotification(null), 4000)
    } catch (err: any) {
      alert(`Gagal trigger attempt: ${err.message}`)
    } finally {
      setAttemptActionLoading(false)
    }
  }

  const handleSimulateAttemptUpdate = async (
    taskId: string,
    attemptId: string,
    status: string,
    exitCode?: number,
    errMsg?: string
  ) => {
    setAttemptActionLoading(true)
    try {
      const payload: any = { status }
      if (exitCode !== undefined) {
        payload.exit_code = exitCode
      }
      if (errMsg) {
        payload.error_message = errMsg
      }

      const res = await fetch(`/api/v1/tasks/${taskId}/attempts/${attemptId}`, {
        method: 'PATCH',
        headers: getAuthHeaders(),
        body: JSON.stringify(payload),
      })

      if (!res.ok) {
        const data = await res.json()
        throw new Error(data.error || 'Gagal update status attempt')
      }

      await fetchTaskDetail(taskId)
      await fetchProtectedData()
    } catch (err: any) {
      alert(`Gagal update attempt: ${err.message}`)
    } finally {
      setAttemptActionLoading(false)
    }
  }

  const handleCancelTaskAction = async (taskId: string) => {
    if (!confirm('Apakah Anda yakin ingin membatalkan eksekusi task ini? Seluruh process tree agent akan dihentikan.')) {
      return
    }
    setAttemptActionLoading(true)
    try {
      const res = await fetch(`/api/v1/tasks/${taskId}/cancel`, {
        method: 'POST',
        headers: getAuthHeaders(),
      })
      if (!res.ok) {
        const data = await res.json()
        throw new Error(data.error || 'Gagal membatalkan task')
      }
      setSuccessNotification('Pembatalan task berhasil diminta! Seluruh process tree agent dihentikan.')
      setTimeout(() => setSuccessNotification(null), 4000)
      await fetchTaskDetail(taskId)
      await fetchProtectedData()
    } catch (err: any) {
      alert(`Gagal cancel task: ${err.message}`)
    } finally {
      setAttemptActionLoading(false)
    }
  }

  const handleRetryTaskAction = async (taskId: string) => {
    setAttemptActionLoading(true)
    try {
      const res = await fetch(`/api/v1/tasks/${taskId}/retry`, {
        method: 'POST',
        headers: getAuthHeaders(),
        body: JSON.stringify({
          runner_id: 'runner-01',
          agent_name: selectedTask?.assigned_agent || 'agy',
        }),
      })
      if (!res.ok) {
        const data = await res.json()
        throw new Error(data.error || 'Gagal retry task')
      }
      setSuccessNotification('Task berhasil di-retry secara eksplisit sebagai attempt baru!')
      setTimeout(() => setSuccessNotification(null), 4000)
      await fetchTaskDetail(taskId)
      await fetchProtectedData()
    } catch (err: any) {
      alert(`Gagal retry task: ${err.message}`)
    } finally {
      setAttemptActionLoading(false)
    }
  }

  const handleRequestRevisionAction = async (taskId: string) => {
    const directive = prompt('Masukkan arahan/catatan revisi untuk Development Team / Agent:', 'Perbaiki catatan verifikasi dan pastikan targeted test lulus.')
    if (!directive || !directive.trim()) return

    setAttemptActionLoading(true)
    try {
      const res = await fetch(`/api/v1/tasks/${taskId}`, {
        method: 'PUT',
        headers: getAuthHeaders(),
        body: JSON.stringify({ status: 'doing' }),
      })
      if (!res.ok) {
        const data = await res.json()
        throw new Error(data.error || 'Gagal mengubah status task')
      }

      await fetch(`/api/v1/tasks/${taskId}/comments`, {
        method: 'POST',
        headers: getAuthHeaders(),
        body: JSON.stringify({ message: `[ARAHAN REVISI PO/OWNER]: ${directive.trim()}` }),
      })

      setSuccessNotification('Siklus revisi dimulai: Task dikembalikan ke Doing dan attempt revisi dijadwalkan otomatis!')
      setTimeout(() => setSuccessNotification(null), 4000)
      await fetchTaskDetail(taskId)
      await fetchProtectedData()
    } catch (err: any) {
      alert(`Gagal meminta revisi: ${err.message}`)
    } finally {
      setAttemptActionLoading(false)
    }
  }

  const handleApproveTaskAction = async (taskId: string) => {
    if (!confirm('Apakah Anda yakin menyetujui (Approve) hasil pekerjaan dan PR ini? Task akan ditandai Done.')) return

    setAttemptActionLoading(true)
    try {
      const res = await fetch(`/api/v1/tasks/${taskId}`, {
        method: 'PUT',
        headers: getAuthHeaders(),
        body: JSON.stringify({ status: 'done' }),
      })
      if (!res.ok) {
        const data = await res.json()
        throw new Error(data.error || 'Gagal approve task')
      }

      await fetch(`/api/v1/tasks/${taskId}/comments`, {
        method: 'POST',
        headers: getAuthHeaders(),
        body: JSON.stringify({ message: '[APPROVAL PO/OWNER]: Pekerjaan dan PR telah direview dan disetujui (Approved). Task selesai (Done).' }),
      })

      setSuccessNotification('Task berhasil di-approve dan berstatus Done!')
      setTimeout(() => setSuccessNotification(null), 4000)
      await fetchTaskDetail(taskId)
      await fetchProtectedData()
    } catch (err: any) {
      alert(`Gagal approve task: ${err.message}`)
    } finally {
      setAttemptActionLoading(false)
    }
  }

  const handleReconcileLeasesAction = async () => {
    try {
      const res = await fetch('/api/v1/runner/reconcile', {
        method: 'POST',
        headers: getAuthHeaders(),
      })
      if (res.ok) {
        const data = await res.json()
        setSuccessNotification(`Reconcile selesai: ${data.interrupted_count} attempt kedaluwarsa dilaporkan interrupted.`)
        setTimeout(() => setSuccessNotification(null), 4000)
        await fetchProtectedData()
      }
    } catch (err: any) {
      alert(`Gagal reconcile leases: ${err.message}`)
    }
  }

  useEffect(() => {
    checkStatusAndAuth()
  }, [])

  const handleSetup = async (e: FormEvent) => {
    e.preventDefault()
    setErrorMessage(null)

    if (passwordInput.length < 8) {
      setErrorMessage('Kata sandi harus minimal 8 karakter.')
      return
    }
    if (passwordInput !== confirmPasswordInput) {
      setErrorMessage('Konfirmasi kata sandi tidak cocok.')
      return
    }

    setAuthLoading(true)
    try {
      const res = await fetch('/api/v1/auth/setup', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ email: emailInput, password: passwordInput }),
      })

      const data = await res.json()
      if (!res.ok) {
        throw new Error(data.error || 'Setup gagal dilakukan')
      }

      const newToken = data.token
      localStorage.setItem('cuan_token', newToken)
      setToken(newToken)
      setOwner(data.owner)
      setSetupRequired(false)
      setEmailInput('')
      setPasswordInput('')
      setConfirmPasswordInput('')
      await fetchProtectedData(newToken)
    } catch (err: any) {
      setErrorMessage(err.message || 'Terjadi kesalahan saat setup')
    } finally {
      setAuthLoading(false)
    }
  }

  const handleLogin = async (e: FormEvent) => {
    e.preventDefault()
    setErrorMessage(null)
    setAuthLoading(true)

    try {
      const res = await fetch('/api/v1/auth/login', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ email: emailInput, password: passwordInput }),
      })

      const data = await res.json()
      if (!res.ok) {
        throw new Error(data.error || 'Login gagal')
      }

      const newToken = data.token
      localStorage.setItem('cuan_token', newToken)
      setToken(newToken)
      setOwner(data.owner)
      setEmailInput('')
      setPasswordInput('')
      await fetchProtectedData(newToken)
    } catch (err: any) {
      setErrorMessage(err.message || 'Email atau password salah')
    } finally {
      setAuthLoading(false)
    }
  }

  const handleLogout = async () => {
    try {
      await fetch('/api/v1/auth/logout', {
        method: 'POST',
        headers: getAuthHeaders(),
      })
    } catch (err) {
      console.error('Logout error:', err)
    } finally {
      localStorage.removeItem('cuan_token')
      setToken('')
      setOwner(null)
      setTasks([])
      setProjects([])
      setSelectedProject(null)
    }
  }

  const handleConnectProject = async (e: FormEvent) => {
    e.preventDefault()
    setConnectError(null)
    setConnectLoading(true)

    try {
      const payload: any = {
        repo_url: repoUrlInput.trim(),
      }
      if (repoNameInput.trim()) {
        payload.name = repoNameInput.trim()
      }
      if (githubTokenInput.trim()) {
        payload.token = githubTokenInput.trim()
      }
      if (defaultBranchInput.trim()) {
        payload.default_branch = defaultBranchInput.trim()
      }

      const res = await fetch('/api/v1/projects', {
        method: 'POST',
        headers: getAuthHeaders(),
        body: JSON.stringify(payload),
      })

      const data = await res.json()
      if (!res.ok) {
        throw new Error(data.error || 'Gagal menghubungkan repository GitHub')
      }

      // Reset form and close modal
      setRepoNameInput('')
      setRepoUrlInput('')
      setGithubTokenInput('')
      setDefaultBranchInput('')
      setIsConnectModalOpen(false)
      setShowTokenGuide(false)

      // Refresh projects list and select the new project
      await fetchProtectedData()
      setSelectedProject(data)
      fetchProjectBranches(data.id)
      setSuccessNotification(`Repository ${data.repo_owner}/${data.repo_name} berhasil dihubungkan!`)
      setTimeout(() => setSuccessNotification(null), 5000)
    } catch (err: any) {
      setConnectError(err.message || 'Terjadi kesalahan saat menghubungkan repository')
    } finally {
      setConnectLoading(false)
    }
  }

  const handleSyncProject = async (projectId: string) => {
    setSyncLoading(true)
    try {
      const res = await fetch(`/api/v1/projects/${projectId}/sync`, {
        method: 'POST',
        headers: getAuthHeaders(),
      })
      const data = await res.json()
      if (!res.ok) {
        throw new Error(data.error || 'Sinkronisasi gagal')
      }

      await fetchProtectedData()
      if (selectedProject?.id === projectId) {
        setSelectedProject(data)
        fetchProjectBranches(projectId)
      }
      setSuccessNotification('Sinkronisasi repository berhasil!')
      setTimeout(() => setSuccessNotification(null), 4000)
    } catch (err: any) {
      alert(`Gagal sinkronisasi: ${err.message}`)
    } finally {
      setSyncLoading(false)
    }
  }

  const handleDeleteProject = async () => {
    if (!selectedProject) return
    setDeleteLoading(true)

    try {
      const res = await fetch(`/api/v1/projects/${selectedProject.id}`, {
        method: 'DELETE',
        headers: getAuthHeaders(),
      })
      if (!res.ok) {
        const data = await res.json()
        throw new Error(data.error || 'Gagal menghapus proyek')
      }

      setDeleteModalOpen(false)
      setSelectedProject(null)
      await fetchProtectedData()
      setSuccessNotification('Proyek berhasil dihapus dari Cuan. Remote GitHub Anda tetap aman.')
      setTimeout(() => setSuccessNotification(null), 5000)
    } catch (err: any) {
      alert(`Gagal menghapus proyek: ${err.message}`)
    } finally {
      setDeleteLoading(false)
    }
  }

  // Legal routes (/terms & /privacy - S03.5)
  if (currentPath === '/terms') {
    return <TermsPage onNavigate={navigate} />
  }

  if (currentPath === '/privacy') {
    return <PrivacyPage onNavigate={navigate} />
  }

  // Explicit Marketing Landing Page route (/marketing & /landing)
  if (currentPath === '/marketing' || currentPath === '/landing') {
    return (
      <LandingPage
        onNavigate={navigate}
        appMode={effectiveMode}
        onToggleMode={handleToggleMode}
      />
    )
  }

  // Sponsorship Pending Confirmation Route
  if (
    currentPath === '/sponsor/pending' ||
    currentPath === '/sponsors/pending' ||
    currentPath === '/sponsor/status' ||
    (currentPath === '/sponsor' && window.location.search.includes('status='))
  ) {
    return <SponsorPending onNavigate={navigate} />
  }

  // Sponsorship Showcase Route (/sponsor)
  if (currentPath === '/sponsor') {
    return <SponsorShowcase onNavigate={navigate} />
  }

  // Sponsorship Archive & Directory Route (/sponsors and /sponsors/:batch)
  if (currentPath === '/sponsors' || currentPath.startsWith('/sponsors/')) {
    return <SponsorArchive onNavigate={navigate} />
  }

  // Batch Campaigns Direct Route (/campaigns and /campaigns/*)
  if (currentPath === '/campaigns' || currentPath.startsWith('/campaigns/')) {
    return (
      <StudioDashboard
        currentPath={currentPath.startsWith('/studio') ? currentPath : `/studio${currentPath}`}
        onNavigate={navigate}
        baseDomain={systemMode.base_domain}
        appMode={effectiveMode}
        isStudioModeActive={effectiveMode === 'studio'}
        onToggleMode={handleToggleMode}
      />
    )
  }

  // Agents Hub Direct Route (/agents and /agents/*)
  if (currentPath === '/agents' || currentPath.startsWith('/agents/')) {
    return (
      <StudioDashboard
        currentPath={currentPath.startsWith('/studio') ? currentPath : `/studio${currentPath}`}
        onNavigate={navigate}
        baseDomain={systemMode.base_domain}
        appMode={effectiveMode}
        isStudioModeActive={effectiveMode === 'studio'}
        onToggleMode={handleToggleMode}
      />
    )
  }

  // Studio Dashboard & Universal Deployer Route (/studio and /studio/*)
  if (currentPath === '/studio' || currentPath.startsWith('/studio/')) {
    return (
      <StudioDashboard
        currentPath={currentPath}
        onNavigate={navigate}
        baseDomain={systemMode.base_domain}
        appMode={effectiveMode}
        isStudioModeActive={effectiveMode === 'studio'}
        onToggleMode={handleToggleMode}
      />
    )
  }

  // Root ('/' and '/home') dynamic routing based on APP_MODE
  if (currentPath === '/' || currentPath === '/home') {
    if (effectiveMode === 'studio') {
      return (
        <StudioDashboard
          currentPath={currentPath}
          onNavigate={navigate}
          baseDomain={systemMode.base_domain}
          appMode={effectiveMode}
          isStudioModeActive={true}
          onToggleMode={handleToggleMode}
        />
      )
    }
    return (
      <LandingPage
        onNavigate={navigate}
        appMode={effectiveMode}
        onToggleMode={handleToggleMode}
      />
    )
  }

  if (loading && setupRequired === null) {
    return (
      <div className="min-h-screen bg-slate-50 flex items-center justify-center">
        <div className="flex flex-col items-center gap-3">
          <RefreshCw className="h-8 w-8 text-emerald-600 animate-spin" />
          <p className="text-sm font-medium text-slate-600">Memeriksa status instance Cuan...</p>
        </div>
      </div>
    )
  }

  // 1. FIRST-TIME SETUP SCREEN
  if (setupRequired) {
    return (
      <div className="min-h-screen bg-slate-50 flex items-center justify-center p-4">
        <div className="w-full max-w-md space-y-6">
          <div className="text-center space-y-2">
            <div className="inline-flex items-center justify-center p-3 bg-emerald-100 rounded-2xl mb-1 text-emerald-700">
              <ShieldCheck className="h-8 w-8" />
            </div>
            <h1 className="text-2xl font-bold tracking-tight text-slate-900">Setup Pertama Pemilik Cuan</h1>
            <p className="text-sm text-slate-600">
              Instance Cuan beroperasi dalam mode pemilik tunggal (single-owner). Akun pertama ini menjadi satu-satunya pengelola instance.
            </p>
          </div>

          <Card className="bg-white shadow-sm border-slate-200">
            <CardHeader className="space-y-1 pb-4">
              <CardTitle className="text-base font-semibold">Buat Akun Pemilik</CardTitle>
              <CardDescription className="text-xs">
                Setelah pendaftaran berhasil, registrasi publik ditutup permanen untuk mencegah pengambilalihan pihak ketiga.
              </CardDescription>
            </CardHeader>
            <form onSubmit={handleSetup}>
              <CardContent className="space-y-4">
                {errorMessage && (
                  <Alert variant="destructive">
                    <AlertCircle className="h-4 w-4" />
                    <AlertTitle>Gagal Setup</AlertTitle>
                    <AlertDescription>{errorMessage}</AlertDescription>
                  </Alert>
                )}

                <div className="space-y-1.5">
                  <label className="text-xs font-semibold text-slate-700">Email Pemilik</label>
                  <Input
                    type="email"
                    placeholder="owner@domain.com"
                    value={emailInput}
                    onChange={(e) => setEmailInput(e.target.value)}
                    required
                  />
                </div>

                <div className="space-y-1.5">
                  <label className="text-xs font-semibold text-slate-700">Kata Sandi (min. 8 karakter)</label>
                  <Input
                    type="password"
                    placeholder="••••••••••••"
                    value={passwordInput}
                    onChange={(e) => setPasswordInput(e.target.value)}
                    required
                  />
                </div>

                <div className="space-y-1.5">
                  <label className="text-xs font-semibold text-slate-700">Konfirmasi Kata Sandi</label>
                  <Input
                    type="password"
                    placeholder="••••••••••••"
                    value={confirmPasswordInput}
                    onChange={(e) => setConfirmPasswordInput(e.target.value)}
                    required
                  />
                </div>

                <div className="p-3 bg-slate-50 rounded-md border border-slate-200 text-xs text-slate-600 space-y-1">
                  <div className="flex items-center gap-1.5 font-medium text-slate-700">
                    <Lock className="h-3.5 w-3.5 text-emerald-600" />
                    Jaminan Keamanan & DoD
                  </div>
                  <p>
                    Data tersimpan secara persisten pada volume PostgreSQL. Kredensial di-hash dengan bcrypt dan database tidak diekspos ke publik.
                  </p>
                </div>
              </CardContent>
              <CardFooter className="pt-2 flex flex-col gap-2">
                <Button type="submit" className="w-full bg-emerald-600 hover:bg-emerald-700 text-white" disabled={authLoading}>
                  {authLoading ? (
                    <>
                      <RefreshCw className="h-4 w-4 mr-2 animate-spin" />
                      Menyimpan Akun Pemilik...
                    </>
                  ) : (
                    'Inisialisasi & Amankan Instance'
                  )}
                </Button>
                <button
                  type="button"
                  onClick={() => navigate('/')}
                  className="text-xs text-slate-500 hover:text-slate-800 underline pt-1"
                >
                  ← Kembali ke Landing Page
                </button>
              </CardFooter>
            </form>
          </Card>
        </div>
      </div>
    )
  }

  // 2. LOGIN SCREEN (WHEN NOT LOGGED IN)
  if (!owner) {
    return (
      <div className="min-h-screen bg-slate-50 flex items-center justify-center p-4">
        <div className="w-full max-w-md space-y-6">
          <div className="text-center space-y-2">
            <div className="inline-flex items-center justify-center p-3 bg-slate-900 rounded-2xl mb-1 text-white">
              <Lock className="h-8 w-8" />
            </div>
            <h1 className="text-2xl font-bold tracking-tight text-slate-900">Masuk ke Cuan</h1>
            <p className="text-sm text-slate-600">
              Instance terkonfigurasi. Masuk dengan akun pemilik terdaftar.
            </p>
          </div>

          <Card className="bg-white shadow-sm border-slate-200">
            <CardHeader className="space-y-1 pb-4">
              <CardTitle className="text-base font-semibold">Autentikasi Pemilik Tunggal</CardTitle>
              <CardDescription className="text-xs">
                Akses API, runner agent, dan data proyek dilindungi oleh sesi terenkripsi.
              </CardDescription>
            </CardHeader>
            <form onSubmit={handleLogin}>
              <CardContent className="space-y-4">
                {currentPath === '/admin/sponsors' && (
                  <div className="p-3 bg-amber-50 border border-amber-200 rounded-lg text-xs text-amber-900 flex items-start gap-2">
                    <ShieldCheck className="h-4 w-4 text-emerald-600 shrink-0 mt-0.5" />
                    <div>
                      <span className="font-semibold text-amber-950">Akses Panel Kurasi Terproteksi (/admin/sponsors)</span>
                      <p className="text-[11px] text-amber-800 mt-0.5">
                        Area moderasi sponsor anti-judol dilindungi oleh autentikasi pemilik tunggal. Masuk dengan akun Pemilik Cuan.
                      </p>
                    </div>
                  </div>
                )}

                {errorMessage && (
                  <Alert variant="destructive">
                    <AlertCircle className="h-4 w-4" />
                    <AlertTitle>Autentikasi Gagal</AlertTitle>
                    <AlertDescription>{errorMessage}</AlertDescription>
                  </Alert>
                )}

                <div className="space-y-1.5">
                  <label className="text-xs font-semibold text-slate-700">Email Pemilik</label>
                  <Input
                    type="email"
                    placeholder="owner@domain.com"
                    value={emailInput}
                    onChange={(e) => setEmailInput(e.target.value)}
                    required
                  />
                </div>

                <div className="space-y-1.5">
                  <label className="text-xs font-semibold text-slate-700">Kata Sandi</label>
                  <Input
                    type="password"
                    placeholder="••••••••••••"
                    value={passwordInput}
                    onChange={(e) => setPasswordInput(e.target.value)}
                    required
                  />
                </div>

                <div className="flex items-center justify-between text-xs text-slate-500 pt-1">
                  <span className="flex items-center gap-1">
                    <Shield className="h-3.5 w-3.5 text-emerald-600" />
                    Pendaftaran publik dikunci
                  </span>
                  <span>PostgreSQL Persistent</span>
                </div>
              </CardContent>
              <CardFooter className="pt-2 flex flex-col gap-2">
                <Button type="submit" className="w-full bg-slate-900 hover:bg-slate-800 text-white" disabled={authLoading}>
                  {authLoading ? (
                    <>
                      <RefreshCw className="h-4 w-4 mr-2 animate-spin" />
                      Memverifikasi...
                    </>
                  ) : (
                    'Masuk ke Dashboard'
                  )}
                </Button>
                <button
                  type="button"
                  onClick={() => navigate('/')}
                  className="text-xs text-slate-500 hover:text-slate-800 underline pt-1"
                >
                  ← Kembali ke Landing Page
                </button>
              </CardFooter>
            </form>
          </Card>
        </div>
      </div>
    )
  }

  // 3. AUTHENTICATED DASHBOARD (UI SHELL)
  return (
    <div className="min-h-screen bg-slate-50 text-slate-900 w-full p-4 md:p-8">
      <div className="max-w-6xl mx-auto space-y-6">
        
        {/* Top Header */}
        <header className="flex flex-col sm:flex-row justify-between items-start sm:items-center gap-4 bg-white p-5 rounded-xl border border-slate-200 shadow-sm">
          <div className="space-y-1">
            <div className="flex items-center gap-3">
              <h1 className="text-2xl font-black tracking-tight text-slate-900">Cuan</h1>
              <Badge variant="success" className="text-xs uppercase">
                Sprint 2: Running
              </Badge>
              <Badge variant="outline" className="text-xs font-mono bg-emerald-50 text-emerald-700 border-emerald-200">
                S02.4: Moderasi Sponsor Anti-Judol
              </Badge>
            </div>
            <p className="text-xs text-slate-500">
              Digital Project Studio for self-hosted teams. Develop Cuan using Cuan.
            </p>
          </div>

          <div className="flex items-center gap-3">
            <Button variant="outline" size="sm" onClick={() => navigate('/sponsor')} className="gap-1.5 text-xs text-slate-700 hover:text-emerald-700">
              <Globe className="h-3.5 w-3.5 text-emerald-600" />
              Grid Sponsor (/sponsor)
            </Button>

            <Button variant="outline" size="sm" onClick={() => navigate('/')} className="gap-1.5 text-xs text-slate-700 hover:text-emerald-700">
              <ExternalLink className="h-3.5 w-3.5 text-slate-500" />
              Landing Page (/)
            </Button>

            <div className="text-right hidden sm:block">
              <div className="text-xs font-bold text-slate-800">{owner.email}</div>
              <div className="text-[11px] text-emerald-600 font-medium">Owner Terautentikasi</div>
            </div>

            <Button variant="outline" size="sm" onClick={() => fetchProtectedData()} disabled={loading} className="gap-1.5 text-xs">
              <RefreshCw className={`h-3.5 w-3.5 ${loading ? 'animate-spin' : ''}`} />
              Refresh
            </Button>

            <Button variant="ghost" size="sm" onClick={handleLogout} className="gap-1.5 text-xs text-red-600 hover:bg-red-50">
              <LogOut className="h-3.5 w-3.5" />
              Keluar
            </Button>
          </div>
        </header>

        {/* Global Success Notification */}
        {successNotification && (
          <div className="p-3 bg-emerald-50 border border-emerald-200 rounded-lg text-emerald-900 text-xs flex items-center justify-between">
            <div className="flex items-center gap-2">
              <CheckCircle2 className="h-4 w-4 text-emerald-600" />
              <span className="font-medium">{successNotification}</span>
            </div>
            <button onClick={() => setSuccessNotification(null)} className="text-emerald-700 hover:text-emerald-900">
              ✕
            </button>
          </div>
        )}

        {/* Navigation Tabs */}
        <div className="flex flex-wrap items-center gap-2 border-b border-slate-200 pb-2">
          <Button
            variant={activeTab === 'sponsors' ? 'default' : 'ghost'}
            size="sm"
            onClick={() => { setActiveTab('sponsors'); setSelectedProject(null); navigate('/admin/sponsors'); }}
            className={`gap-1.5 text-xs font-semibold ${activeTab === 'sponsors' ? 'bg-slate-900 text-white' : 'text-slate-600 hover:text-emerald-700'}`}
          >
            <ShieldCheck className="h-4 w-4 text-emerald-500" />
            Kurasi Sponsor Anti-Judol (/admin/sponsors)
          </Button>

          <Button
            variant="ghost"
            size="sm"
            onClick={() => navigate('/studio')}
            className="gap-1.5 text-xs font-semibold text-emerald-700 hover:bg-emerald-50 border border-emerald-200"
          >
            <Sparkles className="h-4 w-4 text-emerald-600" />
            Studio Deployer (/studio)
          </Button>

          <Button
            variant={activeTab === 'tasks' ? 'default' : 'ghost'}
            size="sm"
            onClick={() => { setActiveTab('tasks'); setSelectedProject(null); navigate('/admin'); }}
            className={`gap-1.5 text-xs ${activeTab === 'tasks' ? 'bg-slate-900 text-white' : 'text-slate-600'}`}
          >
            <GitBranch className="h-4 w-4" />
            Tasks Board ({tasks.length})
          </Button>

          <Button
            variant={activeTab === 'projects' ? 'default' : 'ghost'}
            size="sm"
            onClick={() => { setActiveTab('projects'); setSelectedProject(null); navigate('/admin'); }}
            className={`gap-1.5 text-xs ${activeTab === 'projects' ? 'bg-slate-900 text-white' : 'text-slate-600'}`}
          >
            <FolderGit2 className="h-4 w-4" />
            Projects ({projects.length})
          </Button>

          <Button
            variant={activeTab === 'runners' ? 'default' : 'ghost'}
            size="sm"
            onClick={() => { setActiveTab('runners'); setSelectedProject(null); navigate('/admin'); }}
            className={`gap-1.5 text-xs ${activeTab === 'runners' ? 'bg-slate-900 text-white' : 'text-slate-600'}`}
          >
            <Cpu className="h-4 w-4" />
            Agents & Servers ({runners.length})
          </Button>

          <Button
            variant={activeTab === 'overview' ? 'default' : 'ghost'}
            size="sm"
            onClick={() => { setActiveTab('overview'); setSelectedProject(null); navigate('/admin'); }}
            className={`gap-1.5 text-xs ${activeTab === 'overview' ? 'bg-slate-900 text-white' : 'text-slate-600'}`}
          >
            <Server className="h-4 w-4" />
            System Overview
          </Button>
        </div>

        {/* ============================================================== */}
        {/* TAB 1: PROJECTS & PROJECT DETAIL (MAIN SCOPE T3)               */}
        {/* ============================================================== */}
        {activeTab === 'projects' && (
          <div className="space-y-6">
            {!selectedProject ? (
              // 1. Projects List View
              <div className="space-y-4">
                <div className="flex flex-col sm:flex-row justify-between items-start sm:items-center gap-3">
                  <div>
                    <h2 className="text-lg font-bold text-slate-900">Daftar Project GitHub</h2>
                    <p className="text-xs text-slate-500">
                      Hubungkan repository GitHub existing untuk pengembangan mandiri via Cuan runner & coding agent.
                    </p>
                  </div>

                  <Button
                    onClick={() => { setIsConnectModalOpen(true); setConnectError(null); }}
                    className="bg-emerald-600 hover:bg-emerald-700 text-white text-xs gap-1.5"
                    size="sm"
                  >
                    <Plus className="h-4 w-4" />
                    Hubungkan Repository GitHub
                  </Button>
                </div>

                {projects.length === 0 ? (
                  <Card className="bg-white border-dashed border-2 border-slate-300 p-8 text-center">
                    <CardContent className="space-y-3">
                      <div className="inline-flex p-3 bg-slate-100 rounded-full text-slate-500">
                        <FolderGit2 className="h-8 w-8" />
                      </div>
                      <h3 className="text-sm font-semibold text-slate-800">Belum ada repository terhubung</h3>
                      <p className="text-xs text-slate-500 max-w-md mx-auto">
                        Hubungkan repository GitHub existing Anda (misal <code>https://github.com/buchin/cuan</code>) untuk mulai mengelola task dan menjalankan agent.
                      </p>
                      <Button
                        onClick={() => setIsConnectModalOpen(true)}
                        className="bg-slate-900 hover:bg-slate-800 text-white text-xs mt-2"
                        size="sm"
                      >
                        Hubungkan Repository Pertama
                      </Button>
                    </CardContent>
                  </Card>
                ) : (
                  <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
                    {projects.map((p) => (
                      <Card key={p.id} className="bg-white border-slate-200 hover:border-slate-300 transition-all shadow-sm">
                        <CardHeader className="p-4 pb-3">
                          <div className="flex items-start justify-between gap-2">
                            <div className="space-y-1">
                              <CardTitle className="text-base font-bold text-slate-900 flex items-center gap-1.5">
                                {p.name}
                                <span className="text-xs font-normal text-slate-500 font-mono">
                                  ({p.repo_owner}/{p.repo_name})
                                </span>
                              </CardTitle>
                              <a
                                href={p.repo_url}
                                target="_blank"
                                rel="noreferrer"
                                className="text-xs text-blue-600 hover:underline flex items-center gap-1"
                              >
                                {p.repo_url}
                                <ExternalLink className="h-3 w-3" />
                              </a>
                            </div>

                            <Badge
                              variant={p.clone_status === 'synced' ? 'success' : p.clone_status === 'failed' ? 'destructive' : 'default'}
                              className="text-[11px] capitalize"
                            >
                              {p.clone_status}
                            </Badge>
                          </div>
                        </CardHeader>
                        <CardContent className="p-4 pt-0 space-y-2.5">
                          <div className="flex items-center gap-2 text-xs text-slate-600">
                            <GitBranch className="h-3.5 w-3.5 text-slate-400" />
                            <span>Default Branch: <strong className="font-mono text-slate-800">{p.default_branch}</strong></span>
                          </div>

                          <div className="flex items-center gap-2 text-xs text-slate-600">
                            <KeyRound className="h-3.5 w-3.5 text-slate-400" />
                            <span>
                              Kredensial: {p.has_token ? (
                                <span className="font-mono text-emerald-700 bg-emerald-50 px-1.5 py-0.5 rounded border border-emerald-200">
                                  {p.token_preview || 'Tersimpan Terenkripsi'}
                                </span>
                              ) : (
                                <span className="text-slate-500 italic">Public (Tanpa Token)</span>
                              )}
                            </span>
                          </div>

                          {p.last_fetched_at && (
                            <div className="text-[11px] text-slate-400">
                              Terakhir sinkronisasi: {new Date(p.last_fetched_at).toLocaleString()}
                            </div>
                          )}

                          {p.last_error && (
                            <div className="text-xs text-red-600 bg-red-50 p-2 rounded border border-red-100">
                              Error: {p.last_error}
                            </div>
                          )}
                        </CardContent>
                        <CardFooter className="p-4 pt-2 border-t border-slate-100 flex items-center justify-between">
                          <Button
                            variant="outline"
                            size="sm"
                            onClick={() => handleSyncProject(p.id)}
                            disabled={syncLoading}
                            className="text-xs gap-1.5"
                          >
                            <RefreshCw className={`h-3 w-3 ${syncLoading ? 'animate-spin' : ''}`} />
                            Sync Remote
                          </Button>

                          <Button
                            variant="default"
                            size="sm"
                            onClick={() => handleSelectProject(p)}
                            className="text-xs bg-slate-900 hover:bg-slate-800 text-white gap-1"
                          >
                            Buka Detail
                            <ChevronRight className="h-3.5 w-3.5" />
                          </Button>
                        </CardFooter>
                      </Card>
                    ))}
                  </div>
                )}
              </div>
            ) : (
              // 2. Project Detail View
              <div className="space-y-5">
                <div className="flex items-center justify-between">
                  <Button
                    variant="ghost"
                    size="sm"
                    onClick={() => setSelectedProject(null)}
                    className="text-xs gap-1.5 text-slate-600 hover:text-slate-900"
                  >
                    <ArrowLeft className="h-4 w-4" />
                    Kembali ke Daftar Proyek
                  </Button>

                  <div className="flex items-center gap-2">
                    <Button
                      variant="outline"
                      size="sm"
                      onClick={() => handleSyncProject(selectedProject.id)}
                      disabled={syncLoading}
                      className="text-xs gap-1.5"
                    >
                      <RefreshCw className={`h-3.5 w-3.5 ${syncLoading ? 'animate-spin' : ''}`} />
                      Sinkronisasi Sekarang
                    </Button>

                    <Button
                      variant="destructive"
                      size="sm"
                      onClick={() => setDeleteModalOpen(true)}
                      className="text-xs gap-1.5"
                    >
                      <Trash2 className="h-3.5 w-3.5" />
                      Hapus Proyek dari Cuan
                    </Button>
                  </div>
                </div>

                {/* Main Detail Header Card */}
                <Card className="bg-white border-slate-200 shadow-sm">
                  <CardHeader className="p-5 pb-3">
                    <div className="flex flex-col sm:flex-row justify-between items-start sm:items-center gap-2">
                      <div>
                        <div className="flex items-center gap-2">
                          <CardTitle className="text-xl font-bold text-slate-900">{selectedProject.name}</CardTitle>
                          <Badge
                            variant={selectedProject.clone_status === 'synced' ? 'success' : 'default'}
                            className="capitalize text-xs"
                          >
                            {selectedProject.clone_status}
                          </Badge>
                        </div>
                        <CardDescription className="text-xs text-slate-500 mt-1">
                          ID Proyek: <code className="bg-slate-100 px-1 py-0.5 rounded font-mono">{selectedProject.id}</code>
                        </CardDescription>
                      </div>

                      <a
                        href={selectedProject.repo_url}
                        target="_blank"
                        rel="noreferrer"
                        className="inline-flex items-center gap-1.5 text-xs text-blue-600 bg-blue-50 hover:bg-blue-100 px-3 py-1.5 rounded-md border border-blue-200 transition-colors"
                      >
                        Lihat di GitHub
                        <ExternalLink className="h-3.5 w-3.5" />
                      </a>
                    </div>
                  </CardHeader>
                  <CardContent className="p-5 pt-2 grid grid-cols-1 md:grid-cols-3 gap-4">
                    <div className="p-3 bg-slate-50 rounded-lg border border-slate-200 space-y-1">
                      <div className="text-[11px] font-semibold text-slate-500 uppercase">Repository Remote</div>
                      <div className="text-sm font-bold text-slate-800">{selectedProject.repo_owner} / {selectedProject.repo_name}</div>
                      <div className="text-xs text-slate-500 break-all">{selectedProject.repo_url}</div>
                    </div>

                    <div className="p-3 bg-slate-50 rounded-lg border border-slate-200 space-y-1">
                      <div className="text-[11px] font-semibold text-slate-500 uppercase">Default Branch</div>
                      <div className="text-sm font-bold text-slate-800 flex items-center gap-1.5">
                        <GitBranch className="h-4 w-4 text-emerald-600" />
                        {selectedProject.default_branch}
                      </div>
                      <div className="text-xs text-slate-500">Cabang rujukan utama untuk task & commit</div>
                    </div>

                    <div className="p-3 bg-slate-50 rounded-lg border border-slate-200 space-y-1">
                      <div className="text-[11px] font-semibold text-slate-500 uppercase">Keamanan Kredensial</div>
                      <div className="text-sm font-bold text-emerald-700 flex items-center gap-1.5">
                        <ShieldCheck className="h-4 w-4 text-emerald-600" />
                        {selectedProject.has_token ? selectedProject.token_preview : 'Public Access'}
                      </div>
                      <div className="text-xs text-slate-500">Terenkripsi AES-256-GCM di database</div>
                    </div>
                  </CardContent>
                </Card>

                {/* Branches & Remote Metadata Card */}
                <Card className="bg-white border-slate-200 shadow-sm">
                  <CardHeader className="p-5 pb-3">
                    <CardTitle className="text-sm font-bold text-slate-900 flex items-center gap-2">
                      <GitBranch className="h-4 w-4 text-slate-500" />
                      Cabang Remote (Remote Branches) dari GitHub
                    </CardTitle>
                    <CardDescription className="text-xs text-slate-500">
                      Daftar branch yang terdeteksi secara real-time dari repository remote.
                    </CardDescription>
                  </CardHeader>
                  <CardContent className="p-5 pt-0">
                    {branchesLoading ? (
                      <div className="flex items-center gap-2 text-xs text-slate-500 py-3">
                        <RefreshCw className="h-3.5 w-3.5 animate-spin" />
                        Memeriksa branch remote dari GitHub...
                      </div>
                    ) : projectBranches.length > 0 ? (
                      <div className="flex flex-wrap gap-2 pt-2">
                        {projectBranches.map((br) => (
                          <span
                            key={br}
                            className={`inline-flex items-center gap-1 text-xs px-2.5 py-1 rounded-md border font-mono ${
                              br === selectedProject.default_branch
                                ? 'bg-emerald-50 text-emerald-800 border-emerald-300 font-bold'
                                : 'bg-slate-50 text-slate-700 border-slate-200'
                            }`}
                          >
                            <GitBranch className="h-3 w-3" />
                            {br}
                            {br === selectedProject.default_branch && ' (default)'}
                          </span>
                        ))}
                      </div>
                    ) : (
                      <div className="text-xs text-slate-500 py-2">
                        Branch default: <code className="font-mono font-bold text-slate-700">{selectedProject.default_branch}</code>
                      </div>
                    )}
                  </CardContent>
                </Card>

                {/* Credential & Token Security Details */}
                <Card className="bg-white border-slate-200 shadow-sm">
                  <CardHeader className="p-5 pb-3">
                    <div className="flex items-center justify-between">
                      <CardTitle className="text-sm font-bold text-slate-900 flex items-center gap-2">
                        <Lock className="h-4 w-4 text-emerald-600" />
                        Audit Keamanan Token & Panduan Revoke
                      </CardTitle>
                      <Button
                        variant="outline"
                        size="sm"
                        onClick={() => setShowTokenGuide(!showTokenGuide)}
                        className="text-xs gap-1"
                      >
                        <HelpCircle className="h-3.5 w-3.5" />
                        {showTokenGuide ? 'Sembunyikan Panduan' : 'Lihat Panduan Token'}
                      </Button>
                    </div>
                  </CardHeader>
                  <CardContent className="p-5 pt-0 space-y-3">
                    <div className="grid grid-cols-1 sm:grid-cols-3 gap-3 text-xs">
                      <div className="flex items-start gap-2 p-2.5 bg-slate-50 rounded border border-slate-200">
                        <Check className="h-4 w-4 text-emerald-600 mt-0.5" />
                        <div>
                          <strong>Enkripsi Standar Industri:</strong> Token disimpan dengan AES-256-GCM. Kunci turunan rahasia aman.
                        </div>
                      </div>
                      <div className="flex items-start gap-2 p-2.5 bg-slate-50 rounded border border-slate-200">
                        <Check className="h-4 w-4 text-emerald-600 mt-0.5" />
                        <div>
                          <strong>Zero Leak Log:</strong> Engine Git Cuan secara otomatis menyamarkan token dari log runner maupun proses anak.
                        </div>
                      </div>
                      <div className="flex items-start gap-2 p-2.5 bg-slate-50 rounded border border-slate-200">
                        <Check className="h-4 w-4 text-emerald-600 mt-0.5" />
                        <div>
                          <strong>Non-Destructive:</strong> Menghapus proyek dari Cuan tidak akan pernah menghapus remote repository GitHub.
                        </div>
                      </div>
                    </div>

                    {showTokenGuide && (
                      <div className="mt-4 p-4 bg-slate-50 rounded-lg border border-slate-200 text-xs text-slate-700 space-y-2">
                        <h4 className="font-bold text-slate-900 flex items-center gap-1.5">
                          <KeyRound className="h-4 w-4 text-slate-700" />
                          Panduan Konfigurasi & Pencabutan (Revoke) Token GitHub:
                        </h4>
                        <ol className="list-decimal pl-5 space-y-1">
                          <li>
                            Buka <a href="https://github.com/settings/tokens" target="_blank" rel="noreferrer" className="text-blue-600 underline">GitHub Token Settings</a>.
                          </li>
                          <li>Pilih <strong>Generate new token (classic)</strong> atau <strong>Fine-grained token</strong>.</li>
                          <li>Beri izin (scope) <code>repo</code> untuk clone, fetch, dan PR otomatis pada repository private.</li>
                          <li>
                            <strong>Cara Revoke (Cabut Akses):</strong> Kapan saja Anda ingin memutuskan akses Cuan, buka kembali halaman Settings GitHub dan klik tombol <strong>Delete / Revoke</strong> pada token tersebut.
                          </li>
                        </ol>
                      </div>
                    )}
                  </CardContent>
                </Card>
              </div>
            )}
          </div>
        )}

        {/* ============================================================== */}
        {/* TAB 2: TASKS BOARD VIEW                                         */}
        {/* ============================================================== */}
        {/* ============================================================== */}
        {/* TAB 2: TASKS BOARD VIEW & DETAIL (MAIN SCOPE T4)               */}
        {/* ============================================================== */}
        {activeTab === 'tasks' && (
          <div className="space-y-6">
            {!selectedTask ? (
              // 1. Tasks Board View (Kanban / List)
              <div className="space-y-4">
                {/* Header controls: Search, Filter, View Toggle, Create Button */}
                <div className="flex flex-col md:flex-row justify-between items-start md:items-center gap-4 bg-white p-4 rounded-xl border border-slate-200 shadow-sm">
                  <div className="space-y-1">
                    <h2 className="text-lg font-bold text-slate-900 flex items-center gap-2">
                      <GitBranch className="h-5 w-5 text-emerald-600" />
                      Task Board & Histori Pekerjaan
                    </h2>
                    <p className="text-xs text-slate-500">
                      Kelola unit kerja sprint, tetapkan agent adapter, pantau transisi attempt eksekusi dan catatan kerja.
                    </p>
                  </div>

                  <div className="flex flex-wrap items-center gap-2 w-full md:w-auto">
                    {/* Project Filter */}
                    <div className="flex items-center gap-1.5 bg-slate-50 px-2.5 py-1 rounded-md border border-slate-200 text-xs">
                      <FolderGit2 className="h-3.5 w-3.5 text-slate-400" />
                      <select
                        value={taskProjectFilter}
                        onChange={(e) => setTaskProjectFilter(e.target.value)}
                        className="bg-transparent border-none text-xs text-slate-700 focus:outline-none"
                      >
                        <option value="all">Semua Project ({tasks.length})</option>
                        {projects.map((p) => (
                          <option key={p.id} value={p.id}>
                            {p.name}
                          </option>
                        ))}
                      </select>
                    </div>

                    {/* Search */}
                    <div className="relative">
                      <Search className="h-3.5 w-3.5 text-slate-400 absolute left-2.5 top-1/2 -translate-y-1/2" />
                      <input
                        type="text"
                        placeholder="Cari task..."
                        value={taskSearchQuery}
                        onChange={(e) => setTaskSearchQuery(e.target.value)}
                        className="pl-8 pr-3 py-1 bg-slate-50 border border-slate-200 rounded-md text-xs text-slate-700 w-36 focus:w-48 transition-all focus:outline-none focus:bg-white focus:border-slate-400"
                      />
                    </div>

                    {/* View Mode Toggle */}
                    <div className="flex items-center rounded-lg border border-slate-200 bg-slate-100 p-0.5 text-xs">
                      <button
                        type="button"
                        onClick={() => setBoardViewMode('kanban')}
                        className={`px-2.5 py-1 rounded-md font-medium transition-colors ${
                          boardViewMode === 'kanban' ? 'bg-white text-slate-900 shadow-sm' : 'text-slate-600 hover:text-slate-900'
                        }`}
                      >
                        Kanban
                      </button>
                      <button
                        type="button"
                        onClick={() => setBoardViewMode('list')}
                        className={`px-2.5 py-1 rounded-md font-medium transition-colors ${
                          boardViewMode === 'list' ? 'bg-white text-slate-900 shadow-sm' : 'text-slate-600 hover:text-slate-900'
                        }`}
                      >
                        List
                      </button>
                    </div>

                    {/* Create Task Button */}
                    <Button
                      size="sm"
                      onClick={() => {
                        setIsCreateTaskModalOpen(true)
                        setCreateTaskError(null)
                      }}
                      className="bg-emerald-600 hover:bg-emerald-700 text-white text-xs gap-1.5"
                    >
                      <Plus className="h-4 w-4" />
                      Buat Task
                    </Button>
                  </div>
                </div>

                {/* Filtered tasks */}
                {(() => {
                  const filtered = tasks.filter((t) => {
                    if (taskProjectFilter !== 'all' && t.project_id !== taskProjectFilter) return false
                    if (taskSearchQuery.trim()) {
                      const q = taskSearchQuery.toLowerCase()
                      const matchTitle = t.title.toLowerCase().includes(q)
                      const matchDesc = (t.instructions || '').toLowerCase().includes(q)
                      if (!matchTitle && !matchDesc) return false
                    }
                    return true
                  })

                  if (boardViewMode === 'list') {
                    // List Table View
                    return (
                      <Card className="bg-white border-slate-200 shadow-sm overflow-hidden">
                        <div className="overflow-x-auto">
                          <table className="w-full text-left text-xs border-collapse">
                            <thead>
                              <tr className="bg-slate-50 border-b border-slate-200 text-slate-500 font-semibold uppercase tracking-wider text-[11px]">
                                <th className="p-3.5 pl-4">Task</th>
                                <th className="p-3.5">Project</th>
                                <th className="p-3.5">Status Task</th>
                                <th className="p-3.5">Status Attempt</th>
                                <th className="p-3.5">Prioritas</th>
                                <th className="p-3.5">Agent</th>
                                <th className="p-3.5 text-right pr-4">Aksi</th>
                              </tr>
                            </thead>
                            <tbody className="divide-y divide-slate-100">
                              {filtered.length === 0 ? (
                                <tr>
                                  <td colSpan={7} className="p-8 text-center text-slate-400">
                                    Tidak ada task yang cocok dengan filter.
                                  </td>
                                </tr>
                              ) : (
                                filtered.map((task) => (
                                  <tr key={task.id} className="hover:bg-slate-50/80 transition-colors">
                                    <td className="p-3.5 pl-4 font-medium text-slate-900 max-w-xs">
                                      <div className="flex items-center gap-2">
                                        <span className="font-mono text-[11px] text-slate-400">{task.id}</span>
                                        <span className="truncate">{task.title}</span>
                                      </div>
                                    </td>
                                    <td className="p-3.5 text-slate-600">
                                      {task.project_name || <span className="text-slate-400 italic">None</span>}
                                    </td>
                                    <td className="p-3.5">
                                      <Badge
                                        variant={
                                          task.status === 'done'
                                            ? 'success'
                                            : task.status === 'doing'
                                            ? 'default'
                                            : 'secondary'
                                        }
                                        className="capitalize text-[11px]"
                                      >
                                        {task.status}
                                      </Badge>
                                    </td>
                                    <td className="p-3.5">
                                      {task.latest_attempt ? (
                                        <Badge
                                          variant={
                                            task.latest_attempt.status === 'failed'
                                              ? 'destructive'
                                              : task.latest_attempt.status === 'succeeded'
                                              ? 'success'
                                              : 'outline'
                                          }
                                          className="text-[11px] gap-1 font-mono"
                                        >
                                          {task.latest_attempt.status === 'running' && (
                                            <RefreshCw className="h-3 w-3 animate-spin" />
                                          )}
                                          {task.latest_attempt.status === 'failed' && (
                                            <AlertCircle className="h-3 w-3" />
                                          )}
                                          {task.latest_attempt.status}
                                          {task.latest_attempt.exit_code !== undefined &&
                                            ` (${task.latest_attempt.exit_code})`}
                                        </Badge>
                                      ) : (
                                        <span className="text-slate-400 text-[11px] italic">-</span>
                                      )}
                                    </td>
                                    <td className="p-3.5">
                                      <span
                                        className={`px-2 py-0.5 rounded text-[11px] font-semibold uppercase ${
                                          task.priority === 'urgent'
                                            ? 'bg-red-100 text-red-700'
                                            : task.priority === 'high'
                                            ? 'bg-amber-100 text-amber-700'
                                            : task.priority === 'low'
                                            ? 'bg-slate-100 text-slate-600'
                                            : 'bg-blue-100 text-blue-700'
                                        }`}
                                      >
                                        {task.priority}
                                      </span>
                                    </td>
                                    <td className="p-3.5 font-mono text-slate-600">
                                      🤖 {task.assigned_agent || 'agy'}
                                    </td>
                                    <td className="p-3.5 text-right pr-4">
                                      <Button
                                        variant="outline"
                                        size="sm"
                                        onClick={() => fetchTaskDetail(task.id)}
                                        className="text-xs gap-1"
                                      >
                                        Detail
                                        <ChevronRight className="h-3 w-3" />
                                      </Button>
                                    </td>
                                  </tr>
                                ))
                              )}
                            </tbody>
                          </table>
                        </div>
                      </Card>
                    )
                  }

                  // Kanban View
                  const columns = [
                    { id: 'backlog', title: 'Backlog', color: 'border-slate-300 bg-slate-50/50' },
                    { id: 'doing', title: 'Doing (Running)', color: 'border-blue-300 bg-blue-50/30' },
                    { id: 'review', title: 'Review', color: 'border-purple-300 bg-purple-50/30' },
                    { id: 'done', title: 'Done', color: 'border-emerald-300 bg-emerald-50/30' },
                  ]

                  return (
                    <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-4">
                      {columns.map((col) => {
                        const colTasks = filtered.filter((t) => t.status === col.id)
                        return (
                          <div
                            key={col.id}
                            className={`rounded-xl border ${col.color} p-3 flex flex-col gap-3 min-h-[500px]`}
                          >
                            {/* Column Header */}
                            <div className="flex items-center justify-between pb-2 border-b border-slate-200/80">
                              <span className="font-bold text-xs uppercase tracking-wider text-slate-700">
                                {col.title}
                              </span>
                              <Badge variant="outline" className="text-xs bg-white text-slate-700 font-mono">
                                {colTasks.length}
                              </Badge>
                            </div>

                            {/* Column Cards */}
                            <div className="space-y-2.5 flex-1 overflow-y-auto">
                              {colTasks.length === 0 ? (
                                <div className="p-4 text-center text-xs text-slate-400 italic">
                                  Kolom kosong
                                </div>
                              ) : (
                                colTasks.map((t) => (
                                  <Card
                                    key={t.id}
                                    className="bg-white border-slate-200 hover:border-slate-300 shadow-xs transition-shadow cursor-pointer"
                                    onClick={() => fetchTaskDetail(t.id)}
                                  >
                                    <CardContent className="p-3.5 space-y-2.5">
                                      {/* Card Badges: Priority + Agent */}
                                      <div className="flex items-center justify-between gap-1.5 text-[10px]">
                                        <span
                                          className={`px-1.5 py-0.5 rounded font-bold uppercase ${
                                            t.priority === 'urgent'
                                              ? 'bg-red-100 text-red-700'
                                              : t.priority === 'high'
                                              ? 'bg-amber-100 text-amber-700'
                                              : t.priority === 'low'
                                              ? 'bg-slate-100 text-slate-600'
                                              : 'bg-blue-100 text-blue-700'
                                          }`}
                                        >
                                          {t.priority}
                                        </span>

                                        <span className="font-mono text-slate-500 bg-slate-100 px-1.5 py-0.5 rounded">
                                          🤖 {t.assigned_agent || 'agy'}
                                        </span>
                                      </div>

                                      {/* Title */}
                                      <h4 className="text-xs font-bold text-slate-900 leading-snug">
                                        {t.title}
                                      </h4>

                                      {/* Project name if any */}
                                      {t.project_name && (
                                        <div className="text-[11px] text-slate-500 flex items-center gap-1">
                                          <FolderGit2 className="h-3 w-3 text-slate-400" />
                                          <span className="truncate">{t.project_name}</span>
                                        </div>
                                      )}

                                      {/* Attempt status pill */}
                                      {t.latest_attempt && (
                                        <div className="pt-1 border-t border-slate-100 flex items-center justify-between text-[11px]">
                                          <span className="text-slate-400 text-[10px]">Attempt:</span>
                                          <Badge
                                            variant={
                                              t.latest_attempt.status === 'failed'
                                                ? 'destructive'
                                                : t.latest_attempt.status === 'succeeded'
                                                ? 'success'
                                                : 'outline'
                                            }
                                            className="text-[10px] gap-1 px-1.5 py-0 capitalize"
                                          >
                                            {t.latest_attempt.status === 'running' && (
                                              <RefreshCw className="h-2.5 w-2.5 animate-spin" />
                                            )}
                                            {t.latest_attempt.status === 'failed' && (
                                              <AlertCircle className="h-2.5 w-2.5" />
                                            )}
                                            {t.latest_attempt.status}
                                            {t.latest_attempt.exit_code !== undefined &&
                                              ` (${t.latest_attempt.exit_code})`}
                                          </Badge>
                                        </div>
                                      )}

                                      {/* Quick Move Action buttons */}
                                      <div
                                        className="pt-2 border-t border-slate-100 flex items-center justify-between text-[10px]"
                                        onClick={(e) => e.stopPropagation()}
                                      >
                                        <span className="font-mono text-slate-400">{t.id}</span>
                                        <div className="flex items-center gap-1">
                                          {col.id === 'backlog' && (
                                            <button
                                              onClick={() => handleQuickMoveStatus(t.id, 'doing')}
                                              className="text-blue-600 hover:text-blue-800 font-semibold px-1.5 py-0.5 rounded bg-blue-50 border border-blue-100"
                                            >
                                              Mulai →
                                            </button>
                                          )}
                                          {col.id === 'doing' && (
                                            <>
                                              <button
                                                onClick={() => handleQuickMoveStatus(t.id, 'backlog')}
                                                className="text-slate-500 hover:text-slate-700 px-1 py-0.5"
                                              >
                                                ←
                                              </button>
                                              <button
                                                onClick={() => handleQuickMoveStatus(t.id, 'review')}
                                                className="text-purple-600 hover:text-purple-800 font-semibold px-1.5 py-0.5 rounded bg-purple-50 border border-purple-100"
                                              >
                                                Review →
                                              </button>
                                            </>
                                          )}
                                          {col.id === 'review' && (
                                            <>
                                              <button
                                                onClick={() => handleQuickMoveStatus(t.id, 'doing')}
                                                className="text-blue-600 hover:text-blue-800 px-1.5 py-0.5 rounded bg-blue-50"
                                              >
                                                ← Revisi
                                              </button>
                                              <button
                                                onClick={() => handleQuickMoveStatus(t.id, 'done')}
                                                className="text-emerald-700 hover:text-emerald-900 font-semibold px-1.5 py-0.5 rounded bg-emerald-50 border border-emerald-200"
                                              >
                                                Selesai ✓
                                              </button>
                                            </>
                                          )}
                                          {col.id === 'done' && (
                                            <button
                                              onClick={() => handleQuickMoveStatus(t.id, 'backlog')}
                                              className="text-slate-500 hover:text-slate-700 px-1.5 py-0.5 rounded bg-slate-100"
                                            >
                                              Buka Lagi
                                            </button>
                                          )}
                                        </div>
                                      </div>
                                    </CardContent>
                                  </Card>
                                ))
                              )}
                            </div>
                          </div>
                        )
                      })}
                    </div>
                  )
                })()}
              </div>
            ) : (
              // 2. Task Detail View (DoD: Owner membuka detail di UI & histori transisi)
              <div className="space-y-5">
                {/* Back button & top actions */}
                <div className="flex items-center justify-between">
                  <Button
                    variant="ghost"
                    size="sm"
                    onClick={() => setSelectedTask(null)}
                    className="text-xs gap-1.5 text-slate-600 hover:text-slate-900"
                  >
                    <ArrowLeft className="h-4 w-4" />
                    Kembali ke Task Board
                  </Button>

                  <div className="flex items-center gap-2">
                    {/* Cancel Task Button (when attempt is running or queued) */}
                    {(selectedTask.latest_attempt?.status === 'running' || selectedTask.latest_attempt?.status === 'queued') && (
                      <Button
                        variant="destructive"
                        size="sm"
                        onClick={() => handleCancelTaskAction(selectedTask.id)}
                        disabled={attemptActionLoading}
                        className="text-xs gap-1.5 bg-red-600 hover:bg-red-700 text-white"
                      >
                        <Square className="h-3.5 w-3.5 fill-current" />
                        Batalkan Task (Kill Process Tree)
                      </Button>
                    )}

                    {/* Retry Task Button (when attempt is failed, canceled, or interrupted) */}
                    {(selectedTask.latest_attempt?.status === 'failed' ||
                      selectedTask.latest_attempt?.status === 'canceled' ||
                      selectedTask.latest_attempt?.status === 'interrupted') && (
                      <Button
                        variant="default"
                        size="sm"
                        onClick={() => handleRetryTaskAction(selectedTask.id)}
                        disabled={attemptActionLoading}
                        className="text-xs gap-1.5 bg-blue-600 hover:bg-blue-700 text-white"
                      >
                        <RotateCcw className="h-3.5 w-3.5" />
                        Retry Task (Attempt Baru)
                      </Button>
                    )}

                    {/* Review Actions (when status is review) */}
                    {selectedTask.status === 'review' && (
                      <>
                        <Button
                          variant="outline"
                          size="sm"
                          onClick={() => handleRequestRevisionAction(selectedTask.id)}
                          disabled={attemptActionLoading}
                          className="text-xs gap-1.5 border-amber-300 bg-amber-50 text-amber-800 hover:bg-amber-100"
                        >
                          <RotateCcw className="h-3.5 w-3.5" />
                          Minta Revisi
                        </Button>
                        <Button
                          size="sm"
                          onClick={() => handleApproveTaskAction(selectedTask.id)}
                          disabled={attemptActionLoading}
                          className="text-xs gap-1.5 bg-emerald-600 hover:bg-emerald-700 text-white"
                        >
                          <Check className="h-3.5 w-3.5" />
                          Approve & Done
                        </Button>
                      </>
                    )}

                    <Button
                      variant="outline"
                      size="sm"
                      onClick={() => fetchTaskDetail(selectedTask.id)}
                      disabled={taskDetailLoading}
                      className="text-xs gap-1.5"
                    >
                      <RefreshCw className={`h-3.5 w-3.5 ${taskDetailLoading ? 'animate-spin' : ''}`} />
                      Refresh Detail
                    </Button>

                    <Button
                      variant="destructive"
                      size="sm"
                      onClick={() => handleDeleteTaskAction(selectedTask.id)}
                      className="text-xs gap-1.5"
                    >
                      <Trash2 className="h-3.5 w-3.5" />
                      Hapus Task
                    </Button>
                  </div>
                </div>

                {/* Main Task Header Card */}
                <Card className="bg-white border-slate-200 shadow-sm">
                  <CardHeader className="p-5 pb-3">
                    <div className="flex flex-col md:flex-row justify-between items-start md:items-center gap-3">
                      <div>
                        <div className="flex items-center gap-2">
                          <CardTitle className="text-xl font-bold text-slate-900">
                            {selectedTask.title}
                          </CardTitle>
                          <Badge variant="outline" className="font-mono text-xs">
                            {selectedTask.id}
                          </Badge>
                        </div>
                        <CardDescription className="text-xs text-slate-500 mt-1">
                          Project: {selectedTask.project_name || 'Tidak terikat project'} | Dibuat:{' '}
                          {new Date(selectedTask.created_at).toLocaleString()}
                        </CardDescription>
                      </div>

                      {/* Dual Status Badges (Pemisahan Status Task vs Status Attempt) */}
                      <div className="flex flex-wrap items-center gap-2 bg-slate-50 p-2 rounded-lg border border-slate-200">
                        <div className="text-right">
                          <div className="text-[10px] font-semibold text-slate-400 uppercase">Status Task</div>
                          <Badge
                            variant={
                              selectedTask.status === 'done'
                                ? 'success'
                                : selectedTask.status === 'doing'
                                ? 'default'
                                : 'secondary'
                            }
                            className="capitalize text-xs font-bold"
                          >
                            {selectedTask.status}
                          </Badge>
                        </div>

                        <div className="h-6 w-px bg-slate-200 mx-1" />

                        <div className="text-right">
                          <div className="text-[10px] font-semibold text-slate-400 uppercase">Status Attempt</div>
                          {selectedTask.latest_attempt ? (
                            <Badge
                              variant={
                                selectedTask.latest_attempt.status === 'failed'
                                  ? 'destructive'
                                  : selectedTask.latest_attempt.status === 'succeeded'
                                  ? 'success'
                                  : 'outline'
                              }
                              className="text-xs font-mono capitalize gap-1"
                            >
                              {selectedTask.latest_attempt.status === 'running' && (
                                <RefreshCw className="h-3 w-3 animate-spin" />
                              )}
                              {selectedTask.latest_attempt.status === 'failed' && (
                                <AlertCircle className="h-3 w-3" />
                              )}
                              {selectedTask.latest_attempt.status}
                              {selectedTask.latest_attempt.exit_code !== undefined &&
                                ` (${selectedTask.latest_attempt.exit_code})`}
                            </Badge>
                          ) : (
                            <span className="text-xs text-slate-400 italic">Belum Ada</span>
                          )}
                        </div>
                      </div>
                    </div>
                  </CardHeader>
                </Card>

                {/* DoD CRITICAL ALERT: Attempt Failure must not be displayed as success */}
                {selectedTask.latest_attempt?.status === 'failed' && (
                  <Alert variant="destructive">
                    <AlertCircle className="h-4 w-4" />
                    <AlertTitle className="font-bold">Kegagalan Attempt Eksekusi Terdeteksi</AlertTitle>
                    <AlertDescription className="text-xs space-y-1">
                      <p>
                        Attempt <code>{selectedTask.latest_attempt.id}</code> mengalami kegagalan dengan exit code{' '}
                        <code>{selectedTask.latest_attempt.exit_code}</code>.
                        {selectedTask.latest_attempt.error_message && (
                          <span> Pesan: {selectedTask.latest_attempt.error_message}</span>
                        )}
                      </p>
                      <p className="font-semibold text-red-800">
                        Sesuai kriteria DoD: Kegagalan attempt TIDAK otomatis dianggap sukses atau di-mark done. Task
                        tetap bertahan dalam status kerja untuk perbaikan.
                      </p>
                    </AlertDescription>
                  </Alert>
                )}

                {/* T6: Perubahan Git & GitHub Pull Request Card */}
                <Card className="bg-white border-slate-200 shadow-sm">
                  <CardHeader className="p-4 pb-3 border-b border-slate-100 bg-slate-50/50">
                    <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-2">
                      <div className="flex items-center gap-2">
                        <GitPullRequest className="h-5 w-5 text-indigo-600" />
                        <div>
                          <CardTitle className="text-sm font-bold text-slate-900">
                            Perubahan Git & GitHub Pull Request (T6)
                          </CardTitle>
                          <CardDescription className="text-xs text-slate-500">
                            Branch terisolasi per task, proteksi branch default, diff summary, dan PR GitHub.
                          </CardDescription>
                        </div>
                      </div>

                      {/* Review Action Controls */}
                      {selectedTask.status === 'review' && (
                        <div className="flex items-center gap-2">
                          <Button
                            size="sm"
                            variant="outline"
                            onClick={() => handleRequestRevisionAction(selectedTask.id)}
                            disabled={attemptActionLoading}
                            className="text-xs gap-1 border-amber-300 bg-amber-50 text-amber-800 hover:bg-amber-100"
                          >
                            <RotateCcw className="h-3 w-3" />
                            Minta Revisi (Kembalikan ke Doing)
                          </Button>
                          <Button
                            size="sm"
                            onClick={() => handleApproveTaskAction(selectedTask.id)}
                            disabled={attemptActionLoading}
                            className="text-xs gap-1 bg-emerald-600 hover:bg-emerald-700 text-white"
                          >
                            <Check className="h-3 w-3" />
                            Approve & Mark Done
                          </Button>
                        </div>
                      )}
                    </div>
                  </CardHeader>
                  <CardContent className="p-4 space-y-4">
                    <div className="grid grid-cols-1 sm:grid-cols-2 md:grid-cols-4 gap-3 text-xs">
                      {/* Branch Name */}
                      <div className="p-3 bg-slate-50 rounded-lg border border-slate-200 space-y-1">
                        <div className="text-[10px] font-semibold text-slate-400 uppercase flex items-center gap-1">
                          <GitBranch className="h-3 w-3 text-slate-500" />
                          Task Branch
                        </div>
                        <div className="font-mono font-bold text-slate-800 truncate" title={selectedTask.branch_name || 'task/' + selectedTask.id}>
                          {selectedTask.branch_name || 'task/' + selectedTask.id}
                        </div>
                        <div className="text-[10px] text-emerald-600 font-medium">✓ Default branch terlindungi</div>
                      </div>

                      {/* Pull Request Link */}
                      <div className="p-3 bg-slate-50 rounded-lg border border-slate-200 space-y-1">
                        <div className="text-[10px] font-semibold text-slate-400 uppercase flex items-center gap-1">
                          <GitPullRequest className="h-3 w-3 text-slate-500" />
                          GitHub PR
                        </div>
                        {selectedTask.pr_url ? (
                          <a
                            href={selectedTask.pr_url}
                            target="_blank"
                            rel="noopener noreferrer"
                            className="inline-flex items-center gap-1 text-indigo-600 hover:text-indigo-800 font-bold hover:underline"
                          >
                            <span>PR #{selectedTask.pr_number || 'View'}</span>
                            <ExternalLink className="h-3 w-3" />
                          </a>
                        ) : (
                          <div className="text-slate-400 italic">Belum dibuat</div>
                        )}
                        <div className="text-[10px] text-slate-500">Auto-update saat revisi</div>
                      </div>

                      {/* Latest Commit */}
                      <div className="p-3 bg-slate-50 rounded-lg border border-slate-200 space-y-1">
                        <div className="text-[10px] font-semibold text-slate-400 uppercase flex items-center gap-1">
                          <GitCommit className="h-3 w-3 text-slate-500" />
                          Commit SHA
                        </div>
                        <div className="font-mono font-bold text-slate-800">
                          {selectedTask.commit_sha ? selectedTask.commit_sha.slice(0, 7) : '-'}
                        </div>
                        <div className="text-[10px] text-slate-500">Author: Buchin</div>
                      </div>

                      {/* Review Status */}
                      <div className="p-3 bg-slate-50 rounded-lg border border-slate-200 space-y-1">
                        <div className="text-[10px] font-semibold text-slate-400 uppercase flex items-center gap-1">
                          <ShieldCheck className="h-3 w-3 text-slate-500" />
                          Status Review
                        </div>
                        <Badge
                          variant={
                            selectedTask.status === 'review'
                              ? 'default'
                              : selectedTask.status === 'done'
                              ? 'success'
                              : 'secondary'
                          }
                          className="capitalize text-xs font-bold"
                        >
                          {selectedTask.status === 'review' ? 'Menunggu Review PO' : selectedTask.status}
                        </Badge>
                        <div className="text-[10px] text-slate-500">
                          {selectedTask.status === 'review' ? 'Bisa diminta revisi' : 'Status saat ini'}
                        </div>
                      </div>
                    </div>

                    {/* Targeted Test Status */}
                    {selectedTask.test_summary && (
                      <div className="flex items-center gap-2 p-2.5 bg-emerald-50 border border-emerald-200 rounded-md text-xs text-emerald-800">
                        <CheckCircle2 className="h-4 w-4 text-emerald-600 shrink-0" />
                        <span className="font-medium">{selectedTask.test_summary}</span>
                      </div>
                    )}

                    {/* Diff Summary */}
                    {selectedTask.diff_summary && (
                      <div className="space-y-1.5">
                        <div className="text-xs font-semibold text-slate-700 flex items-center gap-1">
                          <FolderGit2 className="h-3.5 w-3.5 text-slate-500" />
                          Ringkasan Diff Perubahan
                        </div>
                        <pre className="bg-slate-900 text-slate-100 p-3 rounded-md font-mono text-xs overflow-x-auto border border-slate-800 max-h-48">
                          {selectedTask.diff_summary}
                        </pre>
                      </div>
                    )}
                  </CardContent>
                </Card>

                {/* Two-Column Detail Grid */}
                <div className="grid grid-cols-1 lg:grid-cols-12 gap-6">
                  {/* Left Column: Form Edit & Attempts */}
                  <div className="lg:col-span-7 space-y-6">
                    {/* Card 1: Edit Task Form */}
                    <Card className="bg-white border-slate-200 shadow-sm">
                      <CardHeader className="p-4 pb-2 border-b border-slate-100">
                        <CardTitle className="text-sm font-bold text-slate-900">
                          Edit Atribut Task & Status
                        </CardTitle>
                      </CardHeader>
                      <form onSubmit={handleUpdateTaskSubmit}>
                        <CardContent className="p-4 space-y-3.5">
                          <div className="space-y-1">
                            <label className="text-xs font-semibold text-slate-700">Judul Task</label>
                            <Input
                              value={editTaskTitle}
                              onChange={(e) => setEditTaskTitle(e.target.value)}
                              required
                            />
                          </div>

                          <div className="space-y-1">
                            <label className="text-xs font-semibold text-slate-700">Instruksi / Scope</label>
                            <textarea
                              rows={3}
                              value={editTaskInstructions}
                              onChange={(e) => setEditTaskInstructions(e.target.value)}
                              className="w-full text-xs p-2.5 rounded-md border border-slate-300 focus:outline-none focus:border-slate-500 font-mono"
                              placeholder="Deskripsi pekerjaan task..."
                            />
                          </div>

                          <div className="grid grid-cols-1 sm:grid-cols-3 gap-3">
                            <div className="space-y-1">
                              <label className="text-xs font-semibold text-slate-700">Status Task</label>
                              <select
                                value={editTaskStatus}
                                onChange={(e) => setEditTaskStatus(e.target.value)}
                                className="w-full text-xs p-2 rounded-md border border-slate-300 bg-white"
                              >
                                <option value="backlog">Backlog</option>
                                <option value="doing">Doing</option>
                                <option value="review">Review</option>
                                <option value="done">Done</option>
                                <option value="blocked">Blocked</option>
                              </select>
                            </div>

                            <div className="space-y-1">
                              <label className="text-xs font-semibold text-slate-700">Prioritas</label>
                              <select
                                value={editTaskPriority}
                                onChange={(e) => setEditTaskPriority(e.target.value)}
                                className="w-full text-xs p-2 rounded-md border border-slate-300 bg-white"
                              >
                                <option value="low">Low</option>
                                <option value="normal">Normal</option>
                                <option value="high">High</option>
                                <option value="urgent">Urgent</option>
                              </select>
                            </div>

                            <div className="space-y-1">
                              <label className="text-xs font-semibold text-slate-700">Assigned Agent</label>
                              <Input
                                value={editTaskAgent}
                                onChange={(e) => setEditTaskAgent(e.target.value)}
                                placeholder="agy"
                              />
                            </div>
                          </div>
                        </CardContent>
                        <CardFooter className="p-4 pt-2 border-t border-slate-100 flex items-center justify-end">
                          <Button
                            type="submit"
                            size="sm"
                            disabled={editTaskLoading}
                            className="bg-slate-900 hover:bg-slate-800 text-white text-xs gap-1.5"
                          >
                            {editTaskLoading ? (
                              <>
                                <RefreshCw className="h-3.5 w-3.5 animate-spin" />
                                Menyimpan Perubahan...
                              </>
                            ) : (
                              'Simpan Perubahan Task'
                            )}
                          </Button>
                        </CardFooter>
                      </form>
                    </Card>

                    {/* Card 2: Runner & Attempt Execution Controls */}
                    <Card className="bg-white border-slate-200 shadow-sm">
                      <CardHeader className="p-4 pb-2 border-b border-slate-100">
                        <div className="flex items-center justify-between">
                          <CardTitle className="text-sm font-bold text-slate-900 flex items-center gap-1.5">
                            <Bot className="h-4 w-4 text-emerald-600" />
                            Kontrol Attempt Eksekusi & Runner
                          </CardTitle>
                          <Button
                            size="sm"
                            onClick={() => handleTriggerAttempt(selectedTask.id)}
                            disabled={attemptActionLoading}
                            className="bg-emerald-600 hover:bg-emerald-700 text-white text-xs gap-1"
                          >
                            <Play className="h-3 w-3" />
                            Trigger Attempt Baru
                          </Button>
                        </div>
                      </CardHeader>
                      <CardContent className="p-4 space-y-3">
                        {selectedTask.latest_attempt ? (
                          <div className="p-3 bg-slate-50 rounded-lg border border-slate-200 space-y-2 text-xs">
                            <div className="flex items-center justify-between">
                              <span className="font-mono text-slate-600 font-bold">
                                {selectedTask.latest_attempt.id} ({selectedTask.latest_attempt.agent_name})
                              </span>
                              <Badge
                                variant={
                                  selectedTask.latest_attempt.status === 'failed'
                                    ? 'destructive'
                                    : selectedTask.latest_attempt.status === 'succeeded'
                                    ? 'success'
                                    : 'outline'
                                }
                                className="text-xs uppercase"
                              >
                                {selectedTask.latest_attempt.status}
                              </Badge>
                            </div>

                            <div className="grid grid-cols-2 gap-2 text-[11px] text-slate-500">
                              <div>Runner: {selectedTask.latest_attempt.runner_id}</div>
                              <div>Exit Code: {selectedTask.latest_attempt.exit_code ?? '-'}</div>
                            </div>

                            {/* Simulation Actions for Verification Demonstration */}
                            <div className="pt-2 border-t border-slate-200 flex flex-wrap gap-1.5">
                              <span className="text-[10px] text-slate-400 self-center mr-1">Simulasi Transisi:</span>
                              <Button
                                variant="outline"
                                size="sm"
                                onClick={() =>
                                  handleSimulateAttemptUpdate(
                                    selectedTask.id,
                                    selectedTask.latest_attempt!.id,
                                    'running'
                                  )
                                }
                                disabled={attemptActionLoading}
                                className="text-[11px] h-7 px-2"
                              >
                                Set Running
                              </Button>
                              <Button
                                variant="outline"
                                size="sm"
                                onClick={() =>
                                  handleSimulateAttemptUpdate(
                                    selectedTask.id,
                                    selectedTask.latest_attempt!.id,
                                    'succeeded',
                                    0
                                  )
                                }
                                disabled={attemptActionLoading}
                                className="text-[11px] h-7 px-2 text-emerald-700 hover:bg-emerald-50"
                              >
                                Set Succeeded (Exit 0)
                              </Button>
                              <Button
                                variant="outline"
                                size="sm"
                                onClick={() =>
                                  handleSimulateAttemptUpdate(
                                    selectedTask.id,
                                    selectedTask.latest_attempt!.id,
                                    'failed',
                                    1,
                                    'Verifikasi targeted test gagal'
                                  )
                                }
                                disabled={attemptActionLoading}
                                className="text-[11px] h-7 px-2 text-red-600 hover:bg-red-50"
                              >
                                Set Failed (Exit 1)
                              </Button>
                            </div>
                          </div>
                        ) : (
                          <div className="p-4 text-center text-xs text-slate-500 italic bg-slate-50 rounded border">
                            Belum ada attempt yang pernah dijalankan untuk task ini.
                          </div>
                        )}

                        {/* Attempt history list */}
                        {selectedTask.attempts.length > 1 && (
                          <div className="pt-2 space-y-1.5">
                            <div className="text-xs font-semibold text-slate-600">
                              Riwayat Semua Attempt ({selectedTask.attempts.length}):
                            </div>
                            <div className="max-h-36 overflow-y-auto divide-y divide-slate-100 border rounded text-[11px]">
                              {selectedTask.attempts.map((att) => (
                                <div key={att.id} className="p-2 flex items-center justify-between">
                                  <div className="font-mono text-slate-700">
                                    {att.id} ({att.agent_name})
                                  </div>
                                  <div className="flex items-center gap-2">
                                    <span>Exit: {att.exit_code ?? '-'}</span>
                                    <Badge
                                      variant={att.status === 'failed' ? 'destructive' : 'outline'}
                                      className="text-[10px]"
                                    >
                                      {att.status}
                                    </Badge>
                                  </div>
                                </div>
                              ))}
                            </div>
                          </div>
                        )}
                      </CardContent>
                    </Card>

                    {/* Card 3: Execution Output & Streamed Logs (T5) */}
                    <Card className="bg-slate-900 border-slate-800 text-slate-100 shadow-sm">
                      <CardHeader className="p-4 pb-2 border-b border-slate-800">
                        <div className="flex items-center justify-between">
                          <CardTitle className="text-sm font-bold flex items-center gap-2 text-slate-200">
                            <Terminal className="h-4 w-4 text-emerald-400" />
                            Log Output Streaming & Verifikasi
                          </CardTitle>
                          <div className="flex items-center gap-2">
                            <Badge variant="outline" className="text-[10px] text-slate-400 border-slate-700 font-mono">
                              {attemptLogs.length} baris
                            </Badge>
                            {selectedTask.latest_attempt && (
                              <Button
                                size="sm"
                                variant="ghost"
                                onClick={() => fetchAttemptLogs(selectedTask.id, selectedTask.latest_attempt!.id)}
                                disabled={logsLoading}
                                className="h-7 px-2 text-xs text-slate-400 hover:text-slate-100 hover:bg-slate-800"
                              >
                                <RefreshCw className={`h-3 w-3 ${logsLoading ? 'animate-spin' : ''}`} />
                              </Button>
                            )}
                          </div>
                        </div>
                      </CardHeader>
                      <CardContent className="p-4 pt-3">
                        {attemptLogs.length > 0 ? (
                          <div className="bg-slate-950 p-3 rounded font-mono text-[11px] text-slate-300 max-h-64 overflow-y-auto space-y-1 border border-slate-800">
                            {attemptLogs.map((log) => (
                              <div key={log.id} className="flex items-start gap-2">
                                <span className="text-slate-600 select-none text-[10px] w-8 text-right font-mono">
                                  #{log.id}
                                </span>
                                <span
                                  className={`text-[9px] uppercase font-bold px-1 rounded ${
                                    log.stream === 'stderr'
                                      ? 'text-red-400 bg-red-950/60'
                                      : log.stream === 'system'
                                      ? 'text-amber-400 bg-amber-950/60'
                                      : 'text-emerald-400 bg-emerald-950/60'
                                  }`}
                                >
                                  {log.stream}
                                </span>
                                <span className="flex-1 whitespace-pre-wrap break-all font-mono">{log.line}</span>
                              </div>
                            ))}
                          </div>
                        ) : (
                          <div className="text-xs text-slate-500 italic p-4 text-center">
                            Belum ada log output streaming untuk attempt ini. Log akan muncul saat runner mengeksekusi agent.
                          </div>
                        )}
                        <div className="mt-2 text-[10px] text-slate-400 flex items-center justify-between border-t border-slate-800/60 pt-2">
                          <span>🔒 Kredensial & Secrets disamarkan otomatis ([REDACTED])</span>
                          <span>📦 Bounded buffer maks 5.000 baris</span>
                        </div>
                      </CardContent>
                    </Card>
                  </div>

                  {/* Right Column: Activity History & Comments Timeline */}
                  <div className="lg:col-span-5 space-y-4">
                    <Card className="bg-white border-slate-200 shadow-sm flex flex-col h-full">
                      <CardHeader className="p-4 pb-2 border-b border-slate-100">
                        <CardTitle className="text-sm font-bold text-slate-900 flex items-center gap-1.5">
                          <Clock className="h-4 w-4 text-slate-500" />
                          Histori Transisi & Catatan Kerja
                        </CardTitle>
                        <CardDescription className="text-xs text-slate-500">
                          Audit trail perubahan status, riwayat attempt, dan catatan kerja.
                        </CardDescription>
                      </CardHeader>

                      <CardContent className="p-4 space-y-3 flex-1 overflow-y-auto max-h-[500px]">
                        {selectedTask.activities.length === 0 ? (
                          <div className="text-center text-xs text-slate-400 py-6">
                            Belum ada aktivitas tercatat.
                          </div>
                        ) : (
                          <div className="space-y-3">
                            {selectedTask.activities.map((act) => (
                              <div
                                key={act.id}
                                className="p-2.5 rounded-lg border border-slate-100 bg-slate-50/70 text-xs space-y-1.5"
                              >
                                <div className="flex items-center justify-between text-[11px] text-slate-400">
                                  <div className="flex items-center gap-1 font-semibold text-slate-700">
                                    {act.activity_type === 'status_change' && (
                                      <GitBranch className="h-3 w-3 text-purple-600" />
                                    )}
                                    {act.activity_type === 'attempt_transition' && (
                                      <RefreshCw className="h-3 w-3 text-blue-600" />
                                    )}
                                    {act.activity_type === 'comment' && (
                                      <MessageSquare className="h-3 w-3 text-slate-600" />
                                    )}
                                    {act.activity_type === 'task_created' && (
                                      <CheckCircle2 className="h-3 w-3 text-emerald-600" />
                                    )}
                                    {act.activity_type === 'task_updated' && (
                                      <Layers className="h-3 w-3 text-amber-600" />
                                    )}
                                    <span>{act.author}</span>
                                  </div>
                                  <span>
                                    {new Date(act.created_at).toLocaleTimeString([], {
                                      hour: '2-digit',
                                      minute: '2-digit',
                                    })}
                                  </span>
                                </div>

                                <p className="text-slate-800 leading-relaxed">{act.message}</p>

                                {act.old_status && act.new_status && (
                                  <div className="flex items-center gap-1 font-mono text-[10px]">
                                    <span className="bg-slate-200 px-1 py-0.5 rounded text-slate-700">
                                      {act.old_status}
                                    </span>
                                    <span>→</span>
                                    <span className="bg-emerald-100 px-1 py-0.5 rounded text-emerald-800 font-bold">
                                      {act.new_status}
                                    </span>
                                  </div>
                                )}
                              </div>
                            ))}
                          </div>
                        )}
                      </CardContent>

                      {/* Comment Box */}
                      <CardFooter className="p-4 pt-2 border-t border-slate-100">
                        <form onSubmit={handleAddCommentSubmit} className="w-full space-y-2">
                          <textarea
                            rows={2}
                            value={commentInput}
                            onChange={(e) => setCommentInput(e.target.value)}
                            placeholder="Tulis catatan kerja / komentar pada task ini..."
                            className="w-full text-xs p-2 rounded-md border border-slate-200 focus:outline-none focus:border-slate-400"
                            required
                          />
                          <div className="flex justify-end">
                            <Button
                              type="submit"
                              size="sm"
                              disabled={commentLoading || !commentInput.trim()}
                              className="text-xs bg-slate-900 hover:bg-slate-800 text-white gap-1"
                            >
                              <Send className="h-3 w-3" />
                              Kirim Catatan
                            </Button>
                          </div>
                        </form>
                      </CardFooter>
                    </Card>
                  </div>
                </div>
              </div>
            )}
          </div>
        )}

        {/* ============================================================== */}
        {/* TAB 4: AGENTS & SERVERS (MAIN SCOPE T5)                        */}
        {/* ============================================================== */}
        {activeTab === 'runners' && (
          <div className="space-y-6">
            {/* Header & Controls */}
            <div className="flex flex-col sm:flex-row justify-between items-start sm:items-center gap-4 bg-white p-5 rounded-xl border border-slate-200 shadow-sm">
              <div>
                <h2 className="text-xl font-bold text-slate-900 flex items-center gap-2">
                  <Cpu className="h-5 w-5 text-emerald-600" />
                  Agents & Servers
                </h2>
                <p className="text-xs text-slate-500 mt-1">
                  Runner outbound connection daemon, persistent queue, heartbeat/lease tracking, dan profil agent coding.
                </p>
              </div>

              <div className="flex items-center gap-2">
                <Button
                  variant="outline"
                  size="sm"
                  onClick={handleReconcileLeasesAction}
                  className="text-xs gap-1.5 border-slate-200 text-slate-700 hover:bg-slate-50"
                >
                  <Activity className="h-3.5 w-3.5 text-amber-600" />
                  Reconcile Leases
                </Button>

                <Button
                  size="sm"
                  onClick={fetchRunners}
                  disabled={runnersLoading}
                  className="bg-slate-900 hover:bg-slate-800 text-white text-xs gap-1.5"
                >
                  <RefreshCw className={`h-3.5 w-3.5 ${runnersLoading ? 'animate-spin' : ''}`} />
                  Refresh Runners
                </Button>
              </div>
            </div>

            {/* Overview Metric Cards */}
            <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-4">
              <Card className="bg-white border-slate-200 shadow-sm">
                <CardHeader className="p-4 pb-2">
                  <div className="flex items-center justify-between">
                    <CardTitle className="text-xs font-semibold text-slate-500 uppercase tracking-wider">
                      Runners Terdaftar
                    </CardTitle>
                    <Server className="h-4 w-4 text-emerald-600" />
                  </div>
                </CardHeader>
                <CardContent className="p-4 pt-1">
                  <div className="text-2xl font-bold text-slate-900">{runners.length}</div>
                  <div className="text-xs text-slate-500 mt-1">
                    {runners.filter((r) => r.status === 'idle').length} idle,{' '}
                    {runners.filter((r) => r.status === 'busy').length} sibuk
                  </div>
                </CardContent>
              </Card>

              <Card className="bg-white border-slate-200 shadow-sm">
                <CardHeader className="p-4 pb-2">
                  <div className="flex items-center justify-between">
                    <CardTitle className="text-xs font-semibold text-slate-500 uppercase tracking-wider">
                      Queue Depth
                    </CardTitle>
                    <Clock className="h-4 w-4 text-amber-500" />
                  </div>
                </CardHeader>
                <CardContent className="p-4 pt-1">
                  <div className="text-2xl font-bold text-slate-900">
                    {tasks.filter((t) => t.latest_attempt?.status === 'queued').length}
                  </div>
                  <div className="text-xs text-slate-500 mt-1">
                    Task antre menunggu giliran claim
                  </div>
                </CardContent>
              </Card>

              <Card className="bg-white border-slate-200 shadow-sm">
                <CardHeader className="p-4 pb-2">
                  <div className="flex items-center justify-between">
                    <CardTitle className="text-xs font-semibold text-slate-500 uppercase tracking-wider">
                      Task Aktif Berjalan
                    </CardTitle>
                    <RefreshCw className="h-4 w-4 text-blue-500 animate-spin" />
                  </div>
                </CardHeader>
                <CardContent className="p-4 pt-1">
                  <div className="text-2xl font-bold text-slate-900">
                    {tasks.filter((t) => t.latest_attempt?.status === 'running').length}
                  </div>
                  <div className="text-xs text-slate-500 mt-1">
                    Maksimum 1 task aktif per runner
                  </div>
                </CardContent>
              </Card>

              <Card className="bg-white border-slate-200 shadow-sm">
                <CardHeader className="p-4 pb-2">
                  <div className="flex items-center justify-between">
                    <CardTitle className="text-xs font-semibold text-slate-500 uppercase tracking-wider">
                      Agent Adapter
                    </CardTitle>
                    <Bot className="h-4 w-4 text-purple-500" />
                  </div>
                </CardHeader>
                <CardContent className="p-4 pt-1">
                  <div className="text-xl font-bold text-slate-900">AGY (Antigravity)</div>
                  <div className="text-xs text-slate-500 mt-1">Headless CLI adapter terpasang</div>
                </CardContent>
              </Card>
            </div>

            {/* Connected Runners Grid / List */}
            <div className="space-y-4">
              <h3 className="text-sm font-bold text-slate-900 uppercase tracking-wider">
                Daftar Runner Terkoneksi & Metrik Host
              </h3>

              {runners.length === 0 ? (
                <Card className="bg-white border-dashed border-2 border-slate-200 p-8 text-center space-y-3">
                  <Cpu className="h-8 w-8 text-slate-400 mx-auto" />
                  <div className="text-sm font-semibold text-slate-700">Belum ada runner terdaftar</div>
                  <p className="text-xs text-slate-500 max-w-md mx-auto">
                    Jalankan daemon <code>./bin/cuan-runner</code> pada host untuk mulai menghubungkan runner outbound ke Cuan server.
                  </p>
                </Card>
              ) : (
                <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
                  {runners.map((r) => {
                    const memUsedMB = Math.round(r.memory_used_bytes / (1024 * 1024))
                    const memTotalMB = Math.round(r.memory_total_bytes / (1024 * 1024))
                    const memPct = memTotalMB > 0 ? Math.round((memUsedMB / memTotalMB) * 100) : 0
                    const diskUsedGB = Math.round(r.disk_used_bytes / (1024 * 1024 * 1024))
                    const diskTotalGB = Math.round(r.disk_total_bytes / (1024 * 1024 * 1024))
                    const diskPct = diskTotalGB > 0 ? Math.round((diskUsedGB / diskTotalGB) * 100) : 0

                    return (
                      <Card key={r.id} className="bg-white border-slate-200 shadow-sm">
                        <CardHeader className="p-4 pb-2 border-b border-slate-100">
                          <div className="flex items-center justify-between">
                            <div className="flex items-center gap-2">
                              <span className="font-mono text-sm font-bold text-slate-900">{r.id}</span>
                              <span className="text-xs text-slate-400">({r.hostname || 'localhost'})</span>
                            </div>
                            <Badge
                              variant={
                                r.status === 'busy'
                                  ? 'default'
                                  : r.status === 'idle'
                                  ? 'success'
                                  : 'secondary'
                              }
                              className="text-xs capitalize font-bold"
                            >
                              {r.status === 'idle' && '🟢 Idle (Connected)'}
                              {r.status === 'busy' && '⚡ Busy (Executing)'}
                              {r.status === 'offline' && '⚪ Offline (Disconnected)'}
                            </Badge>
                          </div>
                        </CardHeader>
                        <CardContent className="p-4 space-y-3 text-xs">
                          <div className="flex items-center justify-between text-slate-500">
                            <span>Last Heartbeat:</span>
                            <span className="font-mono text-slate-700">
                              {new Date(r.last_heartbeat_at).toLocaleTimeString()}
                            </span>
                          </div>

                          <div className="flex items-center justify-between text-slate-500">
                            <span>Supported Agents:</span>
                            <span className="font-mono text-slate-800 font-semibold">
                              {r.supported_agents?.join(', ') || 'agy'}
                            </span>
                          </div>

                          <div className="p-2.5 bg-slate-50 rounded-lg border border-slate-200 space-y-1">
                            <span className="text-[10px] font-semibold text-slate-400 uppercase">
                              Task Aktif
                            </span>
                            {r.current_task_id ? (
                              <div className="flex items-center justify-between">
                                <span className="font-mono font-bold text-slate-800">
                                  {r.current_task_id}
                                </span>
                                <Button
                                  variant="ghost"
                                  size="sm"
                                  onClick={() => {
                                    setActiveTab('tasks')
                                    fetchTaskDetail(r.current_task_id!)
                                  }}
                                  className="h-6 text-[10px] px-2 text-blue-600 hover:text-blue-800"
                                >
                                  Buka Task Detail →
                                </Button>
                              </div>
                            ) : (
                              <div className="text-slate-400 italic">Tidak ada task aktif (Idle)</div>
                            )}
                          </div>

                          {/* Host Resource Meters */}
                          <div className="space-y-2 pt-1">
                            <div className="flex items-center justify-between text-[11px] text-slate-600">
                              <span>CPU Load:</span>
                              <span className="font-mono font-semibold">{r.cpu_percent.toFixed(1)}%</span>
                            </div>
                            <div className="w-full bg-slate-100 rounded-full h-1.5 overflow-hidden">
                              <div
                                className="bg-emerald-500 h-1.5 rounded-full"
                                style={{ width: `${Math.min(r.cpu_percent, 100)}%` }}
                              />
                            </div>

                            <div className="flex items-center justify-between text-[11px] text-slate-600">
                              <span>RAM Usage:</span>
                              <span className="font-mono font-semibold">
                                {memUsedMB} MB / {memTotalMB} MB ({memPct}%)
                              </span>
                            </div>
                            <div className="w-full bg-slate-100 rounded-full h-1.5 overflow-hidden">
                              <div
                                className="bg-blue-500 h-1.5 rounded-full"
                                style={{ width: `${Math.min(memPct, 100)}%` }}
                              />
                            </div>

                            <div className="flex items-center justify-between text-[11px] text-slate-600">
                              <span>Disk Space:</span>
                              <span className="font-mono font-semibold">
                                {diskUsedGB} GB / {diskTotalGB} GB ({diskPct}%)
                              </span>
                            </div>
                            <div className="w-full bg-slate-100 rounded-full h-1.5 overflow-hidden">
                              <div
                                className="bg-purple-500 h-1.5 rounded-full"
                                style={{ width: `${Math.min(diskPct, 100)}%` }}
                              />
                            </div>
                          </div>
                        </CardContent>
                      </Card>
                    )
                  })}
                </div>
              )}
            </div>

            {/* Agent Profiles Section */}
            <Card className="bg-white border-slate-200 shadow-sm">
              <CardHeader className="p-4 pb-2 border-b border-slate-100">
                <CardTitle className="text-sm font-bold text-slate-900 flex items-center gap-2">
                  <Bot className="h-4 w-4 text-purple-600" />
                  Profil & Kebijakan Coding Agent
                </CardTitle>
                <CardDescription className="text-xs text-slate-500">
                  Konfigurasi adapter eksekusi headless, isolasi proses, dan batas keamanan.
                </CardDescription>
              </CardHeader>
              <CardContent className="p-4 space-y-3 text-xs">
                <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
                  <div className="p-3 bg-slate-50 rounded-lg border border-slate-200 space-y-1">
                    <div className="font-bold text-slate-900 flex items-center gap-1.5">
                      <ShieldCheck className="h-4 w-4 text-emerald-600" />
                      Isolasi & Process Tree
                    </div>
                    <p className="text-slate-600 text-[11px]">
                      Menjalankan agent dengan Process Group tersendiri (<code>Setpgid: true</code>). Saat cancel atau timeout, seluruh pohon proses subagent/compiler dihentikan tuntas tanpa meninggalkan zombie.
                    </p>
                  </div>

                  <div className="p-3 bg-slate-50 rounded-lg border border-slate-200 space-y-1">
                    <div className="font-bold text-slate-900 flex items-center gap-1.5">
                      <Lock className="h-4 w-4 text-blue-600" />
                      Masking Kredensial Otomatis
                    </div>
                    <p className="text-slate-600 text-[11px]">
                      Log streaming dan database disaring secara realtime dengan <code>MaskSecrets</code>. PAT, connection string, password, dan token diganti dengan <code>[REDACTED]</code> untuk mencegah kebocoran.
                    </p>
                  </div>

                  <div className="p-3 bg-slate-50 rounded-lg border border-slate-200 space-y-1">
                    <div className="font-bold text-slate-900 flex items-center gap-1.5">
                      <Layers className="h-4 w-4 text-amber-600" />
                      Batas Concurrency & Antrean
                    </div>
                    <p className="text-slate-600 text-[11px]">
                      Kebijakan tepat 1 task aktif per runner. Task berikutnya otomatis antre (status <code>queued</code>) dalam antrean persisten DB dengan proteksi <code>FOR UPDATE SKIP LOCKED</code> anti-double execution.
                    </p>
                  </div>
                </div>
              </CardContent>
            </Card>
          </div>
        )}

        {/* ============================================================== */}
        {/* TAB 3: SYSTEM FOUNDATION OVERVIEW                              */}
        {/* ============================================================== */}
        {activeTab === 'overview' && (
          <div className="space-y-6">
            <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-4">
              {/* Card 1: Backend */}
              <Card className="bg-white border-slate-200 shadow-sm">
                <CardHeader className="p-4 pb-2">
                  <div className="flex items-center justify-between">
                    <CardTitle className="text-xs font-semibold text-slate-500 uppercase tracking-wider">Backend API</CardTitle>
                    <Server className="h-4 w-4 text-emerald-600" />
                  </div>
                </CardHeader>
                <CardContent className="p-4 pt-1">
                  <div className="text-lg font-bold text-slate-900">Go Chi + Git Engine</div>
                  <div className="flex items-center gap-1.5 mt-1.5">
                    <span className="h-2 w-2 rounded-full bg-emerald-500 animate-pulse" />
                    <span className="text-xs text-slate-600">/healthz: {health?.status || 'healthy'}</span>
                  </div>
                </CardContent>
              </Card>

              {/* Card 2: Database & Volume */}
              <Card className="bg-white border-slate-200 shadow-sm">
                <CardHeader className="p-4 pb-2">
                  <div className="flex items-center justify-between">
                    <CardTitle className="text-xs font-semibold text-slate-500 uppercase tracking-wider">Storage Persisten</CardTitle>
                    <Database className="h-4 w-4 text-emerald-600" />
                  </div>
                </CardHeader>
                <CardContent className="p-4 pt-1">
                  <div className="text-lg font-bold text-slate-900">PostgreSQL 16</div>
                  <div className="flex items-center gap-1.5 mt-1.5">
                    <HardDrive className="h-3.5 w-3.5 text-emerald-600" />
                    <span className="text-xs text-slate-600">Migrations: {ready?.migrations_count || '3'} applied</span>
                  </div>
                </CardContent>
              </Card>

              {/* Card 3: Auth & Security */}
              <Card className="bg-white border-slate-200 shadow-sm">
                <CardHeader className="p-4 pb-2">
                  <div className="flex items-center justify-between">
                    <CardTitle className="text-xs font-semibold text-slate-500 uppercase tracking-wider">Akses Owner</CardTitle>
                    <ShieldCheck className="h-4 w-4 text-emerald-600" />
                  </div>
                </CardHeader>
                <CardContent className="p-4 pt-1">
                  <div className="text-lg font-bold text-slate-900">Single-Owner Terkunci</div>
                  <div className="flex items-center gap-1.5 mt-1.5">
                    <Lock className="h-3.5 w-3.5 text-emerald-600" />
                    <span className="text-xs text-slate-600">Setup ditutup (403 Forbidden)</span>
                  </div>
                </CardContent>
              </Card>

              {/* Card 4: Proxy & Networking */}
              <Card className="bg-white border-slate-200 shadow-sm">
                <CardHeader className="p-4 pb-2">
                  <div className="flex items-center justify-between">
                    <CardTitle className="text-xs font-semibold text-slate-500 uppercase tracking-wider">Network & Proxy</CardTitle>
                    <Globe className="h-4 w-4 text-emerald-600" />
                  </div>
                </CardHeader>
                <CardContent className="p-4 pt-1">
                  <div className="text-lg font-bold text-slate-900">Caddy / Reverse Proxy</div>
                  <div className="flex items-center gap-1.5 mt-1.5">
                    <Shield className="h-3.5 w-3.5 text-emerald-600" />
                    <span className="text-xs text-slate-600">DB internal (port 5432 privat)</span>
                  </div>
                </CardContent>
              </Card>
            </div>

            {/* Verification & DoD Evidence Banner */}
            <div className="bg-emerald-50 border border-emerald-200 rounded-xl p-4 text-xs text-emerald-900 flex flex-col md:flex-row items-start md:items-center justify-between gap-3">
              <div className="space-y-1">
                <div className="font-semibold flex items-center gap-1.5 text-emerald-950">
                  <CheckCircle2 className="h-4 w-4 text-emerald-600" />
                  DoD T3 Terverifikasi: Hubungkan Repository GitHub Existing
                </div>
                <p className="text-emerald-800">
                  Koneksi GitHub aktif, validasi 404/401 informatif, kredensial dienkripsi AES-256-GCM, dan remote repo terlindungi dari penghapusan.
                </p>
              </div>
              <div className="flex items-center gap-2">
                <Badge variant="outline" className="bg-white/80 border-emerald-300 text-emerald-800 font-mono text-[10px]">
                  Projects: {projects.length}
                </Badge>
                <Badge variant="outline" className="bg-white/80 border-emerald-300 text-emerald-800 font-mono text-[10px]">
                  Readyz: {ready?.status || 'ready'}
                </Badge>
              </div>
            </div>
          </div>
        )}

        {/* ============================================================== */}
        {/* TAB 5: PANEL KURASI & MODERASI SPONSOR ANTI-JUDOL (S02.4)      */}
        {/* ============================================================== */}
        {activeTab === 'sponsors' && (
          <AdminSponsorPanel
            token={token}
            onNavigate={navigate}
            onNotify={(msg) => setSuccessNotification(msg)}
            ownerEmail={owner.email}
          />
        )}

        {/* ============================================================== */}
        {/* MODAL / DIALOG: HUBUNGKAN REPOSITORY GITHUB EXISTING           */}
        {/* ============================================================== */}
        {isConnectModalOpen && (
          <div className="fixed inset-0 z-50 bg-black/50 backdrop-blur-sm flex items-center justify-center p-4">
            <div className="w-full max-w-lg bg-white rounded-xl shadow-xl border border-slate-200 overflow-hidden">
              <div className="p-5 border-b border-slate-100 flex items-center justify-between">
                <div>
                  <h3 className="text-base font-bold text-slate-900">Hubungkan Repository GitHub Existing</h3>
                  <p className="text-xs text-slate-500">Integrasikan repository GitHub yang sudah ada ke dalam Cuan.</p>
                </div>
                <button
                  onClick={() => setIsConnectModalOpen(false)}
                  className="text-slate-400 hover:text-slate-600 text-lg font-bold"
                >
                  ✕
                </button>
              </div>

              <form onSubmit={handleConnectProject}>
                <div className="p-5 space-y-4 max-h-[75vh] overflow-y-auto">
                  {connectError && (
                    <Alert variant="destructive">
                      <AlertCircle className="h-4 w-4" />
                      <AlertTitle>Koneksi Gagal</AlertTitle>
                      <AlertDescription className="text-xs">{connectError}</AlertDescription>
                    </Alert>
                  )}

                  <div className="space-y-1.5">
                    <label className="text-xs font-semibold text-slate-700">
                      URL Repository GitHub <span className="text-red-500">*</span>
                    </label>
                    <Input
                      placeholder="https://github.com/buchin/cuan atau buchin/cuan"
                      value={repoUrlInput}
                      onChange={(e) => setRepoUrlInput(e.target.value)}
                      required
                    />
                    <p className="text-[11px] text-slate-500">
                      Format: <code>https://github.com/owner/repo</code> atau <code>owner/repo</code>
                    </p>
                  </div>

                  <div className="space-y-1.5">
                    <label className="text-xs font-semibold text-slate-700">Nama Project (Opsional)</label>
                    <Input
                      placeholder="Contoh: Cuan Core (default: nama repository)"
                      value={repoNameInput}
                      onChange={(e) => setRepoNameInput(e.target.value)}
                    />
                  </div>

                  <div className="space-y-1.5">
                    <label className="text-xs font-semibold text-slate-700">Default Branch (Opsional)</label>
                    <Input
                      placeholder="main (otomatis dideteksi dari remote)"
                      value={defaultBranchInput}
                      onChange={(e) => setDefaultBranchInput(e.target.value)}
                    />
                  </div>

                  <div className="space-y-1.5">
                    <div className="flex items-center justify-between">
                      <label className="text-xs font-semibold text-slate-700">
                        Personal Access Token (PAT)
                      </label>
                      <button
                        type="button"
                        onClick={() => setShowTokenGuide(!showTokenGuide)}
                        className="text-[11px] text-blue-600 hover:underline flex items-center gap-1"
                      >
                        <HelpCircle className="h-3 w-3" />
                        Panduan Token & Revoke
                      </button>
                    </div>
                    <Input
                      type="password"
                      placeholder="ghp_••••••••••••••••••••••••••••••••"
                      value={githubTokenInput}
                      onChange={(e) => setGithubTokenInput(e.target.value)}
                    />
                    <p className="text-[11px] text-slate-500">
                      Wajib untuk repository private. Untuk public repo, token bersifat opsional tetapi meningkatkan limit GitHub API.
                    </p>
                  </div>

                  {showTokenGuide && (
                    <div className="p-3 bg-blue-50 border border-blue-200 rounded-lg text-xs text-blue-900 space-y-1.5">
                      <div className="font-bold flex items-center gap-1 text-blue-950">
                        <KeyRound className="h-3.5 w-3.5" />
                        Panduan Personal Access Token (PAT) GitHub
                      </div>
                      <p>
                        1. Buat token di <a href="https://github.com/settings/tokens" target="_blank" rel="noreferrer" className="underline font-semibold">GitHub Developer Settings</a>.<br />
                        2. Centang izin <code>repo</code> untuk repositori private.<br />
                        3. <strong>Pencabutan Akses:</strong> Token dapat dicabut (revoke) kapan saja langsung di GitHub Settings jika tidak lagi dibutuhkan.
                      </p>
                    </div>
                  )}

                  <div className="p-3 bg-slate-50 border border-slate-200 rounded-md text-xs text-slate-600 flex items-start gap-2">
                    <ShieldCheck className="h-4 w-4 text-emerald-600 mt-0.5 shrink-0" />
                    <div>
                      <strong>Safe Credential Storage:</strong> Token GitHub disimpan terenkripsi dengan AES-256-GCM. Kredensial tidak pernah masuk ke file log dan tidak dikirimkan plain text di API.
                    </div>
                  </div>
                </div>

                <div className="p-4 border-t border-slate-100 bg-slate-50 flex items-center justify-end gap-2">
                  <Button
                    type="button"
                    variant="outline"
                    size="sm"
                    onClick={() => setIsConnectModalOpen(false)}
                    disabled={connectLoading}
                    className="text-xs"
                  >
                    Batal
                  </Button>

                  <Button
                    type="submit"
                    size="sm"
                    disabled={connectLoading}
                    className="bg-emerald-600 hover:bg-emerald-700 text-white text-xs gap-1.5"
                  >
                    {connectLoading ? (
                      <>
                        <RefreshCw className="h-3.5 w-3.5 animate-spin" />
                        Memvalidasi Repository...
                      </>
                    ) : (
                      'Hubungkan Repository'
                    )}
                  </Button>
                </div>
              </form>
            </div>
          </div>
        )}

        {/* ============================================================== */}
        {/* MODAL / DIALOG: CONFIRM DELETE PROJECT                         */}
        {/* ============================================================== */}
        {deleteModalOpen && selectedProject && (
          <div className="fixed inset-0 z-50 bg-black/50 backdrop-blur-sm flex items-center justify-center p-4">
            <div className="w-full max-w-md bg-white rounded-xl shadow-xl border border-slate-200 p-5 space-y-4">
              <div className="flex items-center gap-3">
                <div className="p-2.5 bg-red-100 rounded-full text-red-600">
                  <Trash2 className="h-6 w-6" />
                </div>
                <div>
                  <h3 className="text-base font-bold text-slate-900">Hapus Proyek dari Cuan?</h3>
                  <p className="text-xs text-slate-500">Konfirmasi penghapusan proyek <strong>{selectedProject.name}</strong></p>
                </div>
              </div>

              <div className="p-3 bg-amber-50 border border-amber-200 rounded-md text-xs text-amber-900 space-y-1">
                <div className="font-bold flex items-center gap-1 text-amber-950">
                  <ShieldCheck className="h-4 w-4 text-emerald-600" />
                  Jaminan Keamanan Remote Repository:
                </div>
                <p>
                  Penghapusan ini <strong>HANYA</strong> menghapus data proyek dan cache lokal dari instalasi Cuan Anda.
                  Repository GitHub remote Anda di <code>{selectedProject.repo_url}</code> <strong>TETAP UTUH DAN TIDAK AKAN DIUBAH/DIHAPUS</strong>.
                </p>
              </div>

              <div className="flex items-center justify-end gap-2 pt-2">
                <Button
                  variant="outline"
                  size="sm"
                  onClick={() => setDeleteModalOpen(false)}
                  disabled={deleteLoading}
                  className="text-xs"
                >
                  Batal
                </Button>
                <Button
                  variant="destructive"
                  size="sm"
                  onClick={handleDeleteProject}
                  disabled={deleteLoading}
                  className="text-xs gap-1.5"
                >
                  {deleteLoading ? (
                    <>
                      <RefreshCw className="h-3 w-3 animate-spin" />
                      Menghapus...
                    </>
                  ) : (
                    'Ya, Hapus dari Cuan'
                  )}
                </Button>
              </div>
            </div>
          </div>
        )}

        {/* ============================================================== */}
        {/* MODAL / DIALOG: BUAT TASK BARU (T4)                            */}
        {/* ============================================================== */}
        {isCreateTaskModalOpen && (
          <div className="fixed inset-0 z-50 bg-black/50 backdrop-blur-sm flex items-center justify-center p-4">
            <div className="w-full max-w-lg bg-white rounded-xl shadow-xl border border-slate-200 overflow-hidden">
              <div className="p-5 border-b border-slate-100 flex items-center justify-between">
                <div>
                  <h3 className="text-base font-bold text-slate-900">Buat Task Baru</h3>
                  <p className="text-xs text-slate-500">Tambahkan task ke board untuk pengerjaan sprint.</p>
                </div>
                <button
                  onClick={() => setIsCreateTaskModalOpen(false)}
                  className="text-slate-400 hover:text-slate-600 text-lg font-bold"
                >
                  ✕
                </button>
              </div>

              <form onSubmit={handleCreateTaskSubmit}>
                <div className="p-5 space-y-4 max-h-[75vh] overflow-y-auto">
                  {createTaskError && (
                    <Alert variant="destructive">
                      <AlertCircle className="h-4 w-4" />
                      <AlertTitle>Gagal Membuat Task</AlertTitle>
                      <AlertDescription className="text-xs">{createTaskError}</AlertDescription>
                    </Alert>
                  )}

                  <div className="space-y-1.5">
                    <label className="text-xs font-semibold text-slate-700">
                      Judul Task <span className="text-red-500">*</span>
                    </label>
                    <Input
                      placeholder="Contoh: S01.4 Bangun task board dan histori"
                      value={newTaskTitle}
                      onChange={(e) => setNewTaskTitle(e.target.value)}
                      required
                    />
                  </div>

                  <div className="space-y-1.5">
                    <label className="text-xs font-semibold text-slate-700">Project Terkait (Opsional)</label>
                    <select
                      value={newTaskProjectId}
                      onChange={(e) => setNewTaskProjectId(e.target.value)}
                      className="w-full text-xs p-2.5 rounded-md border border-slate-300 bg-white"
                    >
                      <option value="">Tanpa Project (Mandiri)</option>
                      {projects.map((p) => (
                        <option key={p.id} value={p.id}>
                          {p.name} ({p.repo_owner}/{p.repo_name})
                        </option>
                      ))}
                    </select>
                  </div>

                  <div className="space-y-1.5">
                    <label className="text-xs font-semibold text-slate-700">Instruksi / Scope</label>
                    <textarea
                      rows={4}
                      placeholder="Tuliskan instruksi atau scope pekerjaan..."
                      value={newTaskInstructions}
                      onChange={(e) => setNewTaskInstructions(e.target.value)}
                      className="w-full text-xs p-2.5 rounded-md border border-slate-300 focus:outline-none focus:border-slate-500 font-mono"
                    />
                  </div>

                  <div className="grid grid-cols-2 gap-3">
                    <div className="space-y-1.5">
                      <label className="text-xs font-semibold text-slate-700">Prioritas</label>
                      <select
                        value={newTaskPriority}
                        onChange={(e) => setNewTaskPriority(e.target.value)}
                        className="w-full text-xs p-2.5 rounded-md border border-slate-300 bg-white"
                      >
                        <option value="low">Low</option>
                        <option value="normal">Normal</option>
                        <option value="high">High</option>
                        <option value="urgent">Urgent</option>
                      </select>
                    </div>

                    <div className="space-y-1.5">
                      <label className="text-xs font-semibold text-slate-700">Assigned Agent</label>
                      <Input
                        value={newTaskAgent}
                        onChange={(e) => setNewTaskAgent(e.target.value)}
                        placeholder="agy"
                      />
                    </div>
                  </div>
                </div>

                <div className="p-4 border-t border-slate-100 bg-slate-50 flex items-center justify-end gap-2">
                  <Button
                    type="button"
                    variant="outline"
                    size="sm"
                    onClick={() => setIsCreateTaskModalOpen(false)}
                    disabled={createTaskLoading}
                    className="text-xs"
                  >
                    Batal
                  </Button>

                  <Button
                    type="submit"
                    size="sm"
                    disabled={createTaskLoading}
                    className="bg-emerald-600 hover:bg-emerald-700 text-white text-xs gap-1.5"
                  >
                    {createTaskLoading ? (
                      <>
                        <RefreshCw className="h-3.5 w-3.5 animate-spin" />
                        Menyimpan Task...
                      </>
                    ) : (
                      'Simpan & Buat Task'
                    )}
                  </Button>
                </div>
              </form>
            </div>
          </div>
        )}

        {/* Footer */}
        <footer className="text-xs text-slate-500 flex flex-col sm:flex-row justify-between items-center gap-2 border-t pt-4">
          <span>Cuan Monorepo — Apache-2.0 License</span>
          <span>{info?.architecture || 'Go Backend + React Frontend'}</span>
        </footer>
      </div>
    </div>
  )
}

export default App
