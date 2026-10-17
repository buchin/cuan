import { useState, useEffect } from 'react'
import { Card } from '../ui/card'
import { Button } from '../ui/button'
import { Input } from '../ui/input'
import { Badge } from '../ui/badge'
import { Alert, AlertTitle, AlertDescription } from '../ui/alert'
import {
  Kanban,
  Plus,
  RefreshCw,
  Search,
  CheckCircle2,
  AlertCircle,
  Check,
  ChevronRight,
  ChevronLeft,
  Trash2,
  Layers,
  MessageSquare,
  ListTodo,
  Zap,
  Terminal as TerminalIcon,
  X,
  Bot,
  Sparkles,
  Globe,
  ExternalLink,
  Video,
  KeyRound,
  Send,
  Lock
} from 'lucide-react'
import { ProjectTaskDTO, ExecuteTaskAgentResponse, WordPressIntegrationDTO, PublishTaskResult } from './types'
import { DistillTaskModal } from './DistillTaskModal'
import { TaskTerminal } from './TaskTerminal'
import { TeleprompterModal } from './TeleprompterModal'
import { MarkdownViewer } from '../ui/MarkdownViewer'
import { ScriptDeployerModal, ScriptDeployerControls } from './deployer'
import { WordPressDeployerModal, WordPressControls } from './wordpress'

interface ProjectTasksBoardProps {
  projectId?: string
  projects?: Array<{ id: string; name: string }>
  onOpenDiscussions?: (projectId: string) => void
  onProjectChange?: (projectId: string) => void
}

const COLUMNS: Array<{ id: 'backlog' | 'doing' | 'review' | 'done'; label: string; color: string; badgeColor: string }> = [
  { id: 'backlog', label: 'Backlog', color: 'border-slate-300 dark:border-slate-700', badgeColor: 'bg-slate-100 text-slate-700 dark:bg-slate-800 dark:text-slate-300' },
  { id: 'doing', label: 'Doing', color: 'border-blue-400 dark:border-blue-600', badgeColor: 'bg-blue-100 text-blue-700 dark:bg-blue-950 dark:text-blue-300' },
  { id: 'review', label: 'Review', color: 'border-amber-400 dark:border-amber-600', badgeColor: 'bg-amber-100 text-amber-700 dark:bg-amber-950 dark:text-amber-300' },
  { id: 'done', label: 'Done', color: 'border-emerald-400 dark:border-emerald-600', badgeColor: 'bg-emerald-100 text-emerald-700 dark:bg-emerald-950 dark:text-emerald-300' },
]

export function ProjectTasksBoard({
  projectId: propProjectId,
  projects: propProjects = [],
  onOpenDiscussions,
  onProjectChange,
}: ProjectTasksBoardProps) {
  // Project selection
  const [internalProjects, setInternalProjects] = useState<Array<{ id: string; name: string }>>(propProjects)
  const effectiveProjects = propProjects.length > 0 ? propProjects : internalProjects
  const [selectedProjectId, setSelectedProjectId] = useState<string>(
    propProjectId || (effectiveProjects.length > 0 ? effectiveProjects[0].id : 'default')
  )

  useEffect(() => {
    if (propProjectId && propProjectId !== selectedProjectId) {
      setSelectedProjectId(propProjectId)
    }
  }, [propProjectId])

  // Fetch projects if none passed
  useEffect(() => {
    if (effectiveProjects.length === 0) {
      const loadProjects = async () => {
        try {
          const res = await fetch('/api/v1/projects')
          if (res.ok) {
            const data = await res.json()
            if (data.projects && data.projects.length > 0) {
              setInternalProjects(data.projects)
              if (!propProjectId) {
                setSelectedProjectId(data.projects[0].id)
              }
            }
          }
        } catch {
          try {
            const res2 = await fetch('/api/deployments')
            if (res2.ok) {
              const data2 = await res2.json()
              if (data2.deployments && data2.deployments.length > 0) {
                const list = data2.deployments.map((d: any) => ({ id: d.id, name: d.project_name }))
                setInternalProjects(list)
                if (!propProjectId) {
                  setSelectedProjectId(list[0].id)
                }
              }
            }
          } catch {}
        }
      }
      loadProjects()
    }
  }, [propProjectId])

  // Tasks state
  const [tasks, setTasks] = useState<ProjectTaskDTO[]>([])
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [notification, setNotification] = useState<{ type: 'success' | 'error'; message: string } | null>(null)

  // Filters & Search
  const [searchQuery, setSearchQuery] = useState('')
  const [tagFilter, setTagFilter] = useState('all')
  const [batchFilter, setBatchFilter] = useState('all')
  const [priorityFilter, setPriorityFilter] = useState('all')
  const [parentTaskFilter, setParentTaskFilter] = useState('all')

  // Expanded DoD cards (map of task ID to boolean)
  const [expandedCards, setExpandedCards] = useState<Record<string, boolean>>({})

  // Distill / Create Task Modal
  const [isCreateModalOpen, setIsCreateModalOpen] = useState(false)

  // Interactive Terminal & Detail Drawer States (S09.4, S10.4, S11.4)
  const [selectedTerminalTask, setSelectedTerminalTask] = useState<ProjectTaskDTO | null>(null)
  const [detailTask, setDetailTask] = useState<ProjectTaskDTO | null>(null)
  const [detailModalTab, setDetailModalTab] = useState<'details' | 'terminal' | 'agent_output' | 'wordpress'>('details')
  const [runningAgentTaskIds, setRunningAgentTaskIds] = useState<Record<string, boolean>>({})
  const [geminiKeyWarning, setGeminiKeyWarning] = useState<string | null>(null)

  // Teleprompter View State (S11.4)
  const [isTeleprompterOpen, setIsTeleprompterOpen] = useState(false)
  const [teleprompterTask, setTeleprompterTask] = useState<ProjectTaskDTO | null>(null)

  // Script Deployer Wizard & Quick Controls State (S12.4)
  const [isDeployerModalOpen, setIsDeployerModalOpen] = useState(false)
  const [deployerInitialStep, setDeployerInitialStep] = useState<1 | 2 | 3 | 4>(1)

  const handleOpenDeployer = (step: 1 | 2 | 3 | 4 = 1) => {
    setDeployerInitialStep(step)
    setIsDeployerModalOpen(true)
  }

  // Turnkey WordPress Cloud Launchpad State (S13.4)
  const [isWordPressModalOpen, setIsWordPressModalOpen] = useState(false)
  const [wordPressInitialStep, setWordPressInitialStep] = useState<1 | 2 | 3 | 4>(1)

  const handleOpenWordPress = (step: 1 | 2 | 3 | 4 = 1) => {
    setWordPressInitialStep(step)
    setIsWordPressModalOpen(true)
  }

  // WordPress Integration & Publishing State (S11.4)
  const [wpIntegration, setWpIntegration] = useState<WordPressIntegrationDTO | null>(null)
  const [wpConfigured, setWpConfigured] = useState<boolean | null>(null)
  const [wpLoading, setWpLoading] = useState(false)
  const [wpSiteURL, setWpSiteURL] = useState('')
  const [wpUsername, setWpUsername] = useState('')
  const [wpAppPassword, setWpAppPassword] = useState('')
  const [wpIsConnecting, setWpIsConnecting] = useState(false)
  const [wpConnectError, setWpConnectError] = useState<string | null>(null)
  const [wpConnectSuccess, setWpConnectSuccess] = useState<string | null>(null)
  const [wpIsReconfiguring, setWpIsReconfiguring] = useState(false)

  // Publish Form State (S11.4)
  const [wpPublishTitle, setWpPublishTitle] = useState('')
  const [wpPublishStatus, setWpPublishStatus] = useState<'draft' | 'publish' | 'pending'>('draft')
  const [wpConvertMarkdown, setWpConvertMarkdown] = useState(true)
  const [wpIsPublishing, setWpIsPublishing] = useState(false)
  const [wpPublishError, setWpPublishError] = useState<string | null>(null)
  const [wpPublishResult, setWpPublishResult] = useState<PublishTaskResult | null>(null)

  // Fetch WordPress integration status
  const fetchWordPressIntegration = async (projId: string) => {
    setWpLoading(true)
    setWpConnectError(null)
    try {
      const res = await fetch(`/api/v1/projects/${projId}/integrations/wordpress`)
      if (res.ok) {
        const data = await res.json()
        setWpIntegration(data)
        setWpSiteURL(data.site_url || '')
        setWpUsername(data.username || '')
        setWpConfigured(true)
      } else if (res.status === 404) {
        setWpIntegration(null)
        setWpConfigured(false)
      } else {
        const err = await res.json().catch(() => ({}))
        setWpConnectError(err.error || `HTTP ${res.status}`)
        setWpConfigured(false)
      }
    } catch {
      setWpConfigured(false)
    } finally {
      setWpLoading(false)
    }
  }

  // Connect or reconfigure WordPress
  const handleConnectWordPress = async () => {
    setWpConnectError(null)
    setWpConnectSuccess(null)
    if (!wpSiteURL.trim() || !wpUsername.trim() || !wpAppPassword.trim()) {
      setWpConnectError('Site URL, Username, dan Application Password wajib diisi.')
      return
    }

    setWpIsConnecting(true)
    try {
      const res = await fetch(`/api/v1/projects/${selectedProjectId}/integrations/wordpress`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          site_url: wpSiteURL.trim(),
          username: wpUsername.trim(),
          application_password: wpAppPassword.trim(),
          test_connection: true,
        }),
      })

      const data = await res.json()
      if (!res.ok) {
        throw new Error(data.error || `HTTP ${res.status}: Gagal menghubungkan ke WordPress`)
      }

      setWpIntegration(data)
      setWpConfigured(true)
      setWpIsReconfiguring(false)
      setWpAppPassword('')
      setWpConnectSuccess(`Koneksi berhasil ke ${data.site_url}! Kredensial tersimpan aman via AES-256-GCM.`)
    } catch (err: any) {
      setWpConnectError(err.message || 'Koneksi ke WordPress REST API gagal.')
    } finally {
      setWpIsConnecting(false)
    }
  }

  // Publish Task to WordPress
  const handlePublishToWordPress = async () => {
    if (!detailTask) return
    setWpPublishError(null)
    setWpPublishResult(null)

    const contentToPublish = detailTask.agent_output || detailTask.description
    if (!contentToPublish || !contentToPublish.trim()) {
      setWpPublishError('Task belum memiliki deliverable konten atau draf agent output untuk dipublikasikan.')
      return
    }

    setWpIsPublishing(true)
    try {
      const payload = {
        title: wpPublishTitle.trim() || detailTask.title,
        content: contentToPublish,
        status: wpPublishStatus,
        tags: detailTask.tags || [],
        convert_markdown: wpConvertMarkdown,
      }

      const res = await fetch(`/api/v1/tasks/${detailTask.id}/publish`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(payload),
      })

      const data: PublishTaskResult = await res.json()
      if (!res.ok) {
        const errData = data as any
        throw new Error(errData.error || errData.message || `HTTP ${res.status}: Gagal mempublikasikan task`)
      }

      setWpPublishResult(data)
      const updatedTask: ProjectTaskDTO = {
        ...detailTask,
        publication_url: data.publication_url,
        published_at: data.published_at || new Date().toISOString(),
      }
      setDetailTask(updatedTask)
      setTasks((prev) => prev.map((t) => (t.id === updatedTask.id ? updatedTask : t)))
      setNotification({
        type: 'success',
        message: `🎉 Berhasil mempublikasikan artikel ke WordPress! Status: ${data.status.toUpperCase()}`,
      })
    } catch (err: any) {
      setWpPublishError(err.message || 'Terjadi kesalahan saat mempublikasikan ke WordPress.')
    } finally {
      setWpIsPublishing(false)
    }
  }

  // Fetch tasks
  const fetchTasks = async (projId: string) => {
    setLoading(true)
    setError(null)
    try {
      const params = new URLSearchParams()
      if (tagFilter !== 'all') params.set('tag', tagFilter)
      if (batchFilter !== 'all') params.set('batch_id', batchFilter)
      if (priorityFilter !== 'all') params.set('priority', priorityFilter)
      if (parentTaskFilter !== 'all') params.set('parent_task_id', parentTaskFilter)

      const queryStr = params.toString() ? `?${params.toString()}` : ''
      const res = await fetch(`/api/projects/${projId}/tasks${queryStr}`)
      if (!res.ok) {
        throw new Error(`HTTP ${res.status}: Gagal memuat tasks`)
      }
      const data = await res.json()
      setTasks(data.tasks || [])
    } catch (err: any) {
      setError(err.message || 'Gagal mengambil data tasks dari server.')
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    if (selectedProjectId) {
      fetchTasks(selectedProjectId)
      onProjectChange?.(selectedProjectId)
    }
  }, [selectedProjectId, tagFilter, batchFilter, priorityFilter, parentTaskFilter])

  // Status Mutation
  const handleUpdateStatus = async (task: ProjectTaskDTO, newStatus: string) => {
    try {
      const res = await fetch(`/api/tasks/${task.id}/status`, {
        method: 'PATCH',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ status: newStatus }),
      })

      if (!res.ok) {
        throw new Error('Gagal memperbarui status task')
      }

      await res.json()
      setTasks((prev) =>
        prev.map((t) => (t.id === task.id ? { ...t, status: newStatus } : t))
      )

      if (newStatus === 'doing') {
        setNotification({
          type: 'success',
          message: `⚡ Task "${task.title}" masuk 'Doing'. Agent execution hook telah ditrigger!`,
        })
      } else {
        setNotification({
          type: 'success',
          message: `Status task "${task.title}" diubah ke ${newStatus.toUpperCase()}.`,
        })
      }
    } catch (err: any) {
      setNotification({ type: 'error', message: err.message || 'Gagal mengubah status task' })
    }
  }

  // Toggle DoD Item Checklist on Card
  const handleToggleDoD = async (task: ProjectTaskDTO, dodIndex: number) => {
    const updatedChecklist = task.dod_checklist.map((item, idx) =>
      idx === dodIndex ? { ...item, completed: !item.completed } : item
    )

    try {
      const res = await fetch(`/api/tasks/${task.id}`, {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ dod_checklist: updatedChecklist }),
      })

      if (res.ok) {
        setTasks((prev) =>
          prev.map((t) => (t.id === task.id ? { ...t, dod_checklist: updatedChecklist } : t))
        )
      }
    } catch {}
  }

  // Delete Task
  const handleDeleteTask = async (taskId: string, title: string) => {
    if (!confirm(`Hapus task "${title}" secara permanen?`)) return

    try {
      const res = await fetch(`/api/tasks/${taskId}`, { method: 'DELETE' })
      if (res.ok) {
        setTasks((prev) => prev.filter((t) => t.id !== taskId))
        setNotification({ type: 'success', message: `Task "${title}" berhasil dihapus.` })
      }
    } catch (err: any) {
      setNotification({ type: 'error', message: 'Gagal menghapus task' })
    }
  }

  // Toggle Card Expansion for DoD checklist
  const toggleExpandCard = (id: string) => {
    setExpandedCards((prev) => ({ ...prev, [id]: !prev[id] }))
  }

  // In-Task Agent Runner (S10.4)
  const handleRunAgent = async (task: ProjectTaskDTO, customInstructions?: string) => {
    setRunningAgentTaskIds((prev) => ({ ...prev, [task.id]: true }))
    setGeminiKeyWarning(null)

    // Optimistic status update
    setTasks((prev) =>
      prev.map((t) => (t.id === task.id ? { ...t, execution_status: 'running' } : t))
    )
    if (detailTask && detailTask.id === task.id) {
      setDetailTask((prev) => (prev ? { ...prev, execution_status: 'running' } : null))
    }

    try {
      const res = await fetch(`/api/v1/tasks/${task.id}/execute-agent`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          instructions: customInstructions || '',
        }),
      })

      const data = await res.json()

      // Handle Missing Gemini Key (422) gracefully
      if (res.status === 422 || data.code === 'MISSING_GEMINI_KEY') {
        const errorMsg =
          data.error ||
          'Google Gemini Pro API key belum terhubung. Silakan hubungkan API key Anda melalui menu Agent Hub (/studio/agents) terlebih dahulu.'
        setGeminiKeyWarning(errorMsg)
        setNotification({
          type: 'error',
          message: errorMsg,
        })
        setTasks((prev) =>
          prev.map((t) => (t.id === task.id ? { ...t, execution_status: 'idle' } : t))
        )
        if (detailTask && detailTask.id === task.id) {
          setDetailTask((prev) => (prev ? { ...prev, execution_status: 'idle' } : null))
        }
        return
      }

      if (!res.ok) {
        throw new Error(data.error || data.message || `HTTP ${res.status}: Gagal mengeksekusi agent pada task`)
      }

      const execResp = data as ExecuteTaskAgentResponse
      const updatedTask = execResp.task || {
        ...task,
        agent_output: execResp.agent_output,
        execution_status: execResp.execution_status || 'completed',
      }

      setTasks((prev) => prev.map((t) => (t.id === task.id ? updatedTask : t)))
      if (detailTask && detailTask.id === task.id) {
        setDetailTask(updatedTask)
        setDetailModalTab('agent_output')
      }

      setNotification({
        type: 'success',
        message: `⚡ Agen ${execResp.agent?.name || 'AI'} berhasil mengeksekusi task! ${execResp.dod_completed_count}/${execResp.dod_total_count} kriteria DoD tercentang. Draf tersimpan di Agent Output.`,
      })
    } catch (err: any) {
      setNotification({
        type: 'error',
        message: err.message || 'Gagal mengeksekusi agent pada task ini.',
      })
      setTasks((prev) =>
        prev.map((t) => (t.id === task.id ? { ...t, execution_status: 'idle' } : t))
      )
      if (detailTask && detailTask.id === task.id) {
        setDetailTask((prev) => (prev ? { ...prev, execution_status: 'idle' } : null))
      }
    } finally {
      setRunningAgentTaskIds((prev) => {
        const next = { ...prev }
        delete next[task.id]
        return next
      })
    }
  }

  // Extract all unique tags, batches, and parent tasks for filters
  const allTags = Array.from(new Set(tasks.flatMap((t) => t.tags || [])))
  const allBatches = Array.from(new Set(tasks.map((t) => t.batch_id).filter(Boolean) as string[]))
  const allParents = Array.from(new Set(tasks.map((t) => t.parent_task_id).filter(Boolean) as string[]))

  // Filtered Tasks for UI
  const filteredTasks = tasks.filter((t) => {
    if (searchQuery.trim()) {
      const q = searchQuery.toLowerCase()
      const matchTitle = t.title.toLowerCase().includes(q)
      const matchDesc = t.description?.toLowerCase().includes(q)
      if (!matchTitle && !matchDesc) return false
    }
    return true
  })

  // Priority Badge Helper
  const getPriorityBadge = (priority: string) => {
    switch (priority) {
      case 'urgent':
        return <Badge variant="destructive" className="text-[10px] py-0 px-1.5 uppercase font-mono">Urgent</Badge>
      case 'high':
        return <Badge className="bg-amber-500 text-white text-[10px] py-0 px-1.5 uppercase font-mono">High</Badge>
      case 'normal':
        return <Badge variant="secondary" className="bg-blue-100 text-blue-700 dark:bg-blue-950 dark:text-blue-300 text-[10px] py-0 px-1.5 uppercase font-mono">Normal</Badge>
      case 'low':
        return <Badge variant="outline" className="text-slate-400 text-[10px] py-0 px-1.5 uppercase font-mono">Low</Badge>
      default:
        return <Badge variant="outline" className="text-[10px] py-0 px-1.5 font-mono">{priority}</Badge>
    }
  }

  return (
    <div className="space-y-6 animate-in fade-in duration-300">
      {/* Toast Notification Banner */}
      {notification && (
        <div
          className={`p-3.5 rounded-xl flex items-center justify-between text-xs font-semibold shadow-md animate-in slide-in-from-top duration-200 ${
            notification.type === 'success'
              ? 'bg-emerald-500 text-white shadow-emerald-500/20'
              : 'bg-red-500 text-white shadow-red-500/20'
          }`}
        >
          <div className="flex items-center gap-2">
            {notification.type === 'success' ? (
              <CheckCircle2 className="h-4 w-4 shrink-0" />
            ) : (
              <AlertCircle className="h-4 w-4 shrink-0" />
            )}
            <span>{notification.message}</span>
          </div>
          <button onClick={() => setNotification(null)} className="opacity-80 hover:opacity-100">
            <X className="h-4 w-4" />
          </button>
        </div>
      )}

      {/* Error Alert Banner */}
      {error && (
        <div className="p-3.5 rounded-xl bg-red-50 dark:bg-red-950/40 border border-red-200 dark:border-red-900 text-red-700 dark:text-red-300 text-xs flex items-center justify-between">
          <div className="flex items-center gap-2">
            <AlertCircle className="h-4 w-4 shrink-0" />
            <span>{error}</span>
          </div>
          <button onClick={() => setError(null)} className="opacity-80 hover:opacity-100">
            <X className="h-4 w-4" />
          </button>
        </div>
      )}

      {/* Top Header Bar */}
      <div className="flex flex-col sm:flex-row items-start sm:items-center justify-between gap-3 p-4 bg-white dark:bg-slate-900 rounded-2xl border border-slate-200 dark:border-slate-800 shadow-xs">
        <div className="flex items-center gap-3">
          <div className="h-10 w-10 rounded-xl bg-indigo-500/10 border border-indigo-500/20 text-indigo-600 dark:text-indigo-400 flex items-center justify-center shrink-0">
            <Kanban className="h-5 w-5" />
          </div>
          <div>
            <div className="flex items-center gap-2">
              <h1 className="text-base sm:text-lg font-bold text-slate-900 dark:text-white tracking-tight">
                Project Tasks Board
              </h1>
              <Badge variant="outline" className="border-indigo-400/40 text-indigo-600 dark:text-indigo-400 text-[10px] font-mono">
                {tasks.length} Tasks
              </Badge>
            </div>
            <p className="text-xs text-slate-400">
              Papan Kanban 4 status (Backlog, Doing, Review, Done) dengan DoD acceptance criteria dan hook agen otonom.
            </p>
          </div>
        </div>

        <div className="flex flex-wrap items-center gap-2.5 w-full sm:w-auto justify-between sm:justify-end">
          {/* Quick Script Deployer Controls (S12.4) */}
          <ScriptDeployerControls
            projectId={selectedProjectId}
            projectName={effectiveProjects.find((p) => p.id === selectedProjectId)?.name || selectedProjectId}
            onOpenDeployerWizard={handleOpenDeployer}
            onToast={(type, message) => setNotification({ type, message })}
          />

          {/* Quick WordPress Cloud Controls (S13.4) */}
          <WordPressControls
            projectId={selectedProjectId}
            projectName={effectiveProjects.find((p) => p.id === selectedProjectId)?.name || selectedProjectId}
            onOpenDeployerWizard={handleOpenWordPress}
            onToast={(type, message) => setNotification({ type, message })}
          />

          {/* Project Selector */}
          <div className="flex items-center gap-1.5">
            <Layers className="h-3.5 w-3.5 text-slate-400 shrink-0" />
            <select
              value={selectedProjectId}
              onChange={(e) => setSelectedProjectId(e.target.value)}
              className="text-xs py-1.5 px-2.5 rounded-lg border border-slate-200 dark:border-slate-800 bg-slate-50 dark:bg-slate-950 text-slate-800 dark:text-slate-200 font-semibold focus:outline-none focus:ring-1 focus:ring-indigo-500 max-w-[180px] truncate"
            >
              {effectiveProjects.length > 0 ? (
                effectiveProjects.map((p) => (
                  <option key={p.id} value={p.id}>
                    {p.name}
                  </option>
                ))
              ) : (
                <option value="default">Default Project</option>
              )}
            </select>
          </div>

          {/* Jump to Discussions Button */}
          {onOpenDiscussions && (
            <Button
              variant="outline"
              size="sm"
              onClick={() => onOpenDiscussions(selectedProjectId)}
              className="h-8 text-xs font-semibold gap-1.5 border-slate-300 dark:border-slate-700"
              title="Buka Ruang Diskusi Project"
            >
              <MessageSquare className="h-3.5 w-3.5 text-emerald-500" />
              <span className="hidden sm:inline">Discussions</span>
            </Button>
          )}

          {/* Refresh Button */}
          <Button
            variant="outline"
            size="sm"
            onClick={() => fetchTasks(selectedProjectId)}
            className="h-8 text-xs px-2.5"
            title="Refresh tasks"
          >
            <RefreshCw className={`h-3.5 w-3.5 ${loading ? 'animate-spin' : ''}`} />
          </Button>

          {/* New Task Button */}
          <Button
            size="sm"
            onClick={() => setIsCreateModalOpen(true)}
            className="h-8 text-xs font-semibold bg-emerald-600 hover:bg-emerald-500 text-white gap-1 shadow-xs"
          >
            <Plus className="h-3.5 w-3.5" />
            <span>Task Baru</span>
          </Button>
        </div>
      </div>

      {/* Filter & Search Bar */}
      <div className="p-3.5 bg-white dark:bg-slate-900 rounded-2xl border border-slate-200 dark:border-slate-800 shadow-xs flex flex-wrap items-center justify-between gap-3 text-xs">
        {/* Search */}
        <div className="relative flex-1 min-w-[200px] max-w-sm">
          <Search className="h-3.5 w-3.5 absolute left-3 top-2.5 text-slate-400" />
          <Input
            type="text"
            placeholder="Cari task berdasarkan judul atau deskripsi..."
            value={searchQuery}
            onChange={(e) => setSearchQuery(e.target.value)}
            className="h-8 pl-8 text-xs"
          />
        </div>

        {/* Filters */}
        <div className="flex items-center gap-2 flex-wrap">
          {/* Tag Filter */}
          <div className="flex items-center gap-1">
            <span className="text-slate-400 text-[11px]">Tag:</span>
            <select
              value={tagFilter}
              onChange={(e) => setTagFilter(e.target.value)}
              className="h-7 text-xs px-2 rounded-lg border border-slate-200 dark:border-slate-800 bg-slate-50 dark:bg-slate-950 text-slate-700 dark:text-slate-300 font-medium"
            >
              <option value="all">Semua Tag</option>
              {allTags.map((t) => (
                <option key={t} value={t}>
                  #{t}
                </option>
              ))}
            </select>
          </div>

          {/* Priority Filter */}
          <div className="flex items-center gap-1">
            <span className="text-slate-400 text-[11px]">Priority:</span>
            <select
              value={priorityFilter}
              onChange={(e) => setPriorityFilter(e.target.value)}
              className="h-7 text-xs px-2 rounded-lg border border-slate-200 dark:border-slate-800 bg-slate-50 dark:bg-slate-950 text-slate-700 dark:text-slate-300 font-medium"
            >
              <option value="all">Semua Prioritas</option>
              <option value="urgent">Urgent</option>
              <option value="high">High</option>
              <option value="normal">Normal</option>
              <option value="low">Low</option>
            </select>
          </div>

          {/* Batch Filter */}
          {allBatches.length > 0 && (
            <div className="flex items-center gap-1">
              <span className="text-slate-400 text-[11px]">Batch:</span>
              <select
                value={batchFilter}
                onChange={(e) => setBatchFilter(e.target.value)}
                className="h-7 text-xs px-2 rounded-lg border border-slate-200 dark:border-slate-800 bg-slate-50 dark:bg-slate-950 text-slate-700 dark:text-slate-300 font-medium"
              >
                <option value="all">Semua Batch</option>
                {allBatches.map((b) => (
                  <option key={b} value={b}>
                    {b}
                  </option>
                ))}
              </select>
            </div>
          )}

          {/* Subtask / Parent Filter */}
          {allParents.length > 0 && (
            <div className="flex items-center gap-1">
              <span className="text-slate-400 text-[11px]">Subtask:</span>
              <select
                value={parentTaskFilter}
                onChange={(e) => setParentTaskFilter(e.target.value)}
                className="h-7 text-xs px-2 rounded-lg border border-slate-200 dark:border-slate-800 bg-slate-50 dark:bg-slate-950 text-slate-700 dark:text-slate-300 font-medium"
              >
                <option value="all">Semua</option>
                {allParents.map((p) => (
                  <option key={p} value={p}>
                    Subtask #{p.slice(0, 8)}
                  </option>
                ))}
              </select>
            </div>
          )}

          {/* Clear Filters Button */}
          {(tagFilter !== 'all' || batchFilter !== 'all' || priorityFilter !== 'all' || parentTaskFilter !== 'all' || searchQuery) && (
            <Button
              variant="ghost"
              size="sm"
              onClick={() => {
                setTagFilter('all')
                setBatchFilter('all')
                setPriorityFilter('all')
                setParentTaskFilter('all')
                setSearchQuery('')
              }}
              className="h-7 text-[11px] text-slate-400 hover:text-slate-700 dark:hover:text-slate-200 px-2"
            >
              Reset Filter
            </Button>
          )}
        </div>
      </div>

      {/* 4-Column Kanban Board */}
      <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-4 items-start">
        {COLUMNS.map((col) => {
          const colTasks = filteredTasks.filter((t) => t.status === col.id)

          return (
            <div
              key={col.id}
              className={`bg-slate-100/70 dark:bg-slate-900/60 rounded-2xl border-t-4 ${col.color} border-slate-200 dark:border-slate-800 p-3 space-y-3 shadow-xs flex flex-col min-h-[500px]`}
            >
              {/* Column Header */}
              <div className="flex items-center justify-between px-1">
                <div className="flex items-center gap-2">
                  <span className="text-xs font-bold text-slate-800 dark:text-slate-100 uppercase tracking-wider">
                    {col.label}
                  </span>
                  <Badge className={`${col.badgeColor} text-[10px] font-mono py-0 px-2 rounded-full font-bold`}>
                    {colTasks.length}
                  </Badge>
                </div>
                {col.id === 'doing' && colTasks.length > 0 && (
                  <span className="flex items-center gap-1 text-[10px] text-blue-600 dark:text-blue-400 font-mono font-semibold animate-pulse">
                    <Zap className="h-3 w-3" /> Auto-Runner
                  </span>
                )}
              </div>

              {/* Tasks List */}
              <div className="flex-1 space-y-2.5 overflow-y-auto max-h-[700px] pr-0.5">
                {colTasks.length === 0 ? (
                  <div className="p-6 text-center text-xs text-slate-400 border border-dashed border-slate-200 dark:border-slate-800 rounded-xl my-2">
                    Tidak ada task di kolom {col.label}.
                  </div>
                ) : (
                  colTasks.map((task) => {
                    const dodList = task.dod_checklist || []
                    const completedDoD = dodList.filter((d) => d.completed).length
                    const totalDoD = dodList.length
                    const dodPercent = totalDoD > 0 ? Math.round((completedDoD / totalDoD) * 100) : 0
                    const isExpanded = !!expandedCards[task.id]

                    return (
                      <Card
                        key={task.id}
                        className="p-3.5 bg-white dark:bg-slate-900 border-slate-200 dark:border-slate-800 shadow-xs hover:shadow-md transition-shadow space-y-2.5 rounded-xl group"
                      >
                        {/* Card Header: Badges & Actions */}
                        <div className="flex items-center justify-between gap-1">
                          <div className="flex items-center gap-1.5 flex-wrap">
                            {getPriorityBadge(task.priority)}
                            {(task.execution_status === 'running' || runningAgentTaskIds[task.id]) && (
                              <Badge className="bg-purple-600 text-white animate-pulse text-[9px] py-0 px-1.5 flex items-center gap-1 font-mono">
                                <RefreshCw className="h-2.5 w-2.5 animate-spin" />
                                Running Agent
                              </Badge>
                            )}
                            {task.agent_output && (
                              <button
                                type="button"
                                onClick={(e) => {
                                  e.stopPropagation()
                                  setDetailTask(task)
                                  setDetailModalTab('agent_output')
                                }}
                                className="inline-flex items-center gap-1 text-[9px] font-mono px-1.5 py-0.5 rounded bg-emerald-500/15 hover:bg-emerald-500/25 text-emerald-600 dark:text-emerald-400 border border-emerald-500/30 transition-colors"
                                title="Lihat draf Agent Output"
                              >
                                <Bot className="h-2.5 w-2.5 text-emerald-500" />
                                <span>Output</span>
                              </button>
                            )}
                            {task.batch_id && (
                              <Badge variant="outline" className="text-[9px] py-0 px-1 border-indigo-400/40 text-indigo-600 dark:text-indigo-400 font-mono">
                                {task.batch_id}
                              </Badge>
                            )}
                            {task.parent_task_id && (
                              <Badge variant="outline" className="text-[9px] py-0 px-1 border-amber-400/40 text-amber-600 dark:text-amber-400 font-mono">
                                Subtask
                              </Badge>
                            )}
                          </div>

                          {/* Quick Access Terminal & Delete Button */}
                          <div className="flex items-center gap-1">
                            <button
                              type="button"
                              onClick={() => setSelectedTerminalTask(task)}
                              className="flex items-center gap-1 px-1.5 py-0.5 rounded text-[10px] font-mono text-emerald-600 dark:text-emerald-400 bg-emerald-500/10 hover:bg-emerald-500/20 border border-emerald-500/30 transition-colors"
                              title="Akses Cepat Interactive PTY Terminal"
                            >
                              <TerminalIcon className="h-3 w-3 text-emerald-500" />
                              <span className="hidden sm:inline">PTY</span>
                            </button>

                            {/* Delete Card Button */}
                            <button
                              type="button"
                              onClick={() => handleDeleteTask(task.id, task.title)}
                              className="opacity-0 group-hover:opacity-100 p-1 text-slate-400 hover:text-red-500 rounded transition-opacity"
                              title="Hapus task"
                            >
                              <Trash2 className="h-3 w-3" />
                            </button>
                          </div>
                        </div>

                        {/* Title (Click to open Detail & Terminal Modal) */}
                        <h3
                          onClick={() => {
                            setDetailTask(task)
                            setDetailModalTab('details')
                          }}
                          className="text-xs font-bold text-slate-900 dark:text-white leading-snug cursor-pointer hover:text-emerald-600 dark:hover:text-emerald-400 transition-colors"
                          title="Klik untuk melihat Detail & Terminal"
                        >
                          {task.title}
                        </h3>

                        {/* Description Preview */}
                        {task.description && (
                          <p className="text-[11px] text-slate-500 line-clamp-2">
                            {task.description}
                          </p>
                        )}

                        {/* Tags */}
                        {task.tags && task.tags.length > 0 && (
                          <div className="flex flex-wrap gap-1">
                            {task.tags.map((t) => (
                              <span
                                key={t}
                                className="text-[10px] font-mono text-slate-600 dark:text-slate-400 bg-slate-100 dark:bg-slate-800 px-1.5 py-0.2 rounded"
                              >
                                #{t}
                              </span>
                            ))}
                          </div>
                        )}

                        {/* DoD Progress Indicator */}
                        {totalDoD > 0 && (
                          <div className="space-y-1 pt-1 border-t border-slate-100 dark:border-slate-800/80">
                            <div className="flex items-center justify-between text-[10px] text-slate-500">
                              <button
                                type="button"
                                onClick={() => toggleExpandCard(task.id)}
                                className="flex items-center gap-1 font-semibold text-slate-700 dark:text-slate-300 hover:text-emerald-500"
                              >
                                <ListTodo className="h-3 w-3 text-emerald-500" />
                                <span>DoD: {completedDoD}/{totalDoD} ({dodPercent}%)</span>
                              </button>
                              <span className="font-mono text-[9px] text-slate-400">
                                {isExpanded ? 'Tutup' : 'Buka'}
                              </span>
                            </div>

                            {/* Progress bar */}
                            <div className="w-full bg-slate-100 dark:bg-slate-800 rounded-full h-1.5 overflow-hidden">
                              <div
                                className={`h-full transition-all duration-300 ${
                                  dodPercent === 100
                                    ? 'bg-emerald-500'
                                    : dodPercent >= 50
                                    ? 'bg-blue-500'
                                    : 'bg-amber-500'
                                }`}
                                style={{ width: `${dodPercent}%` }}
                              />
                            </div>

                            {/* Expanded Interactive DoD Checklist */}
                            {isExpanded && (
                              <div className="space-y-1 pt-2 animate-in fade-in duration-150">
                                {dodList.map((item, idx) => (
                                  <label
                                    key={idx}
                                    className="flex items-start gap-1.5 text-[11px] cursor-pointer text-slate-600 dark:text-slate-400 hover:text-slate-900 dark:hover:text-slate-200"
                                  >
                                    <input
                                      type="checkbox"
                                      checked={item.completed}
                                      onChange={() => handleToggleDoD(task, idx)}
                                      className="rounded text-emerald-600 focus:ring-emerald-500 mt-0.5 h-3 w-3 shrink-0"
                                    />
                                    <span className={`break-words ${item.completed ? 'line-through text-slate-400' : ''}`}>
                                      {item.text}
                                    </span>
                                  </label>
                                ))}
                              </div>
                            )}
                          </div>
                        )}

                        {/* In-Task Agent Runner & Output Card Section (S10.4) */}
                        <div className="pt-2 border-t border-slate-100 dark:border-slate-800/80 space-y-1.5">
                          <button
                            type="button"
                            disabled={runningAgentTaskIds[task.id] || task.execution_status === 'running'}
                            onClick={(e) => {
                              e.stopPropagation()
                              handleRunAgent(task)
                            }}
                            className={`w-full flex items-center justify-center gap-1.5 py-1.5 px-2.5 rounded-lg text-[11px] font-semibold transition-all ${
                              runningAgentTaskIds[task.id] || task.execution_status === 'running'
                                ? 'bg-purple-500/20 text-purple-400 border border-purple-500/30 cursor-not-allowed'
                                : 'bg-purple-50 hover:bg-purple-100 dark:bg-purple-950/40 dark:hover:bg-purple-900/50 text-purple-700 dark:text-purple-300 border border-purple-200 dark:border-purple-800 shadow-2xs hover:shadow-xs active:scale-[0.98]'
                            }`}
                            title="Eksekusi pembuatan konten otomatis dengan AI Agent persona via Google Gemini Pro"
                          >
                            {runningAgentTaskIds[task.id] || task.execution_status === 'running' ? (
                              <>
                                <RefreshCw className="h-3.5 w-3.5 animate-spin text-purple-500" />
                                <span>Menjalankan Agent...</span>
                              </>
                            ) : (
                              <>
                                <Bot className="h-3.5 w-3.5 text-purple-600 dark:text-purple-400" />
                                <span>Run with Agent</span>
                                <Sparkles className="h-3 w-3 text-amber-500 ml-0.5" />
                              </>
                            )}
                          </button>

                          {task.agent_output && (
                            <div className="flex items-center gap-1">
                              <button
                                type="button"
                                onClick={(e) => {
                                  e.stopPropagation()
                                  setDetailTask(task)
                                  setDetailModalTab('agent_output')
                                }}
                                className="flex-1 flex items-center justify-between px-2.5 py-1 rounded-lg bg-emerald-500/10 hover:bg-emerald-500/20 text-emerald-700 dark:text-emerald-300 border border-emerald-500/30 text-[10px] font-medium transition-colors"
                              >
                                <span className="flex items-center gap-1.5 truncate">
                                  <CheckCircle2 className="h-3 w-3 text-emerald-500 shrink-0" />
                                  <span className="truncate">Draf Konten Siap</span>
                                </span>
                                <span className="font-mono text-[9px] text-emerald-600 dark:text-emerald-400 font-bold shrink-0">
                                  Output &rarr;
                                </span>
                              </button>
                              <button
                                type="button"
                                onClick={(e) => {
                                  e.stopPropagation()
                                  setTeleprompterTask(task)
                                  setIsTeleprompterOpen(true)
                                }}
                                className="px-2 py-1 rounded-lg bg-red-500/10 hover:bg-red-500/20 text-red-600 dark:text-red-400 border border-red-500/30 text-[10px] font-medium flex items-center gap-1 shrink-0 transition-colors"
                                title="Buka Naskah di Teleprompter Fullscreen"
                              >
                                <Video className="h-3 w-3 text-red-500" />
                                <span>Prompter</span>
                              </button>
                            </div>
                          )}

                          {/* WordPress Published Indicator */}
                          {task.publication_url && (
                            <a
                              href={task.publication_url}
                              target="_blank"
                              rel="noopener noreferrer"
                              onClick={(e) => e.stopPropagation()}
                              className="w-full flex items-center justify-between px-2.5 py-1 rounded-lg bg-blue-500/10 hover:bg-blue-500/20 text-blue-700 dark:text-blue-300 border border-blue-500/30 text-[10px] font-medium transition-colors"
                              title={`Terbit di WordPress: ${task.publication_url}`}
                            >
                              <span className="flex items-center gap-1.5 truncate">
                                <Globe className="h-3 w-3 text-blue-500 shrink-0" />
                                <span className="truncate">WordPress: {task.publication_url.replace(/^https?:\/\//, '')}</span>
                              </span>
                              <ExternalLink className="h-3 w-3 text-blue-500 shrink-0" />
                            </a>
                          )}
                        </div>

                        {/* Status Mutation Control Bar */}
                        <div className="pt-2 border-t border-slate-100 dark:border-slate-800/80 flex items-center justify-between gap-1 text-[10px]">
                          {/* Move Left */}
                          {col.id === 'doing' && (
                            <Button
                              variant="ghost"
                              size="sm"
                              onClick={() => handleUpdateStatus(task, 'backlog')}
                              className="h-6 px-1.5 text-[10px] text-slate-500 gap-0.5"
                              title="Pindahkan ke Backlog"
                            >
                              <ChevronLeft className="h-3 w-3" /> Backlog
                            </Button>
                          )}
                          {col.id === 'review' && (
                            <Button
                              variant="ghost"
                              size="sm"
                              onClick={() => handleUpdateStatus(task, 'doing')}
                              className="h-6 px-1.5 text-[10px] text-slate-500 gap-0.5"
                              title="Kembalikan ke Doing"
                            >
                              <ChevronLeft className="h-3 w-3" /> Doing
                            </Button>
                          )}
                          {col.id === 'done' && (
                            <Button
                              variant="ghost"
                              size="sm"
                              onClick={() => handleUpdateStatus(task, 'review')}
                              className="h-6 px-1.5 text-[10px] text-slate-500 gap-0.5"
                              title="Kembalikan ke Review"
                            >
                              <ChevronLeft className="h-3 w-3" /> Review
                            </Button>
                          )}

                          <div className="flex-1" />

                          {/* Move Right */}
                          {col.id === 'backlog' && (
                            <Button
                              size="sm"
                              onClick={() => handleUpdateStatus(task, 'doing')}
                              className="h-6 px-2 text-[10px] font-semibold bg-blue-600 hover:bg-blue-500 text-white gap-1"
                              title="Mulai pengerjaan (Memicu Agent Hook)"
                            >
                              <span>Start Doing</span>
                              <ChevronRight className="h-3 w-3" />
                            </Button>
                          )}
                          {col.id === 'doing' && (
                            <Button
                              size="sm"
                              onClick={() => handleUpdateStatus(task, 'review')}
                              className="h-6 px-2 text-[10px] font-semibold bg-amber-600 hover:bg-amber-500 text-white gap-1"
                              title="Pindahkan ke Review"
                            >
                              <span>Submit Review</span>
                              <ChevronRight className="h-3 w-3" />
                            </Button>
                          )}
                          {col.id === 'review' && (
                            <Button
                              size="sm"
                              onClick={() => handleUpdateStatus(task, 'done')}
                              className="h-6 px-2 text-[10px] font-semibold bg-emerald-600 hover:bg-emerald-500 text-white gap-1"
                              title="Tandai selesai (DoD Terpenuhi)"
                            >
                              <Check className="h-3 w-3" />
                              <span>Mark Done</span>
                            </Button>
                          )}
                        </div>
                      </Card>
                    )
                  })
                )}
              </div>
            </div>
          )
        })}
      </div>

      {/* Task Creation Modal */}
      <DistillTaskModal
        isOpen={isCreateModalOpen}
        onClose={() => setIsCreateModalOpen(false)}
        projectId={selectedProjectId}
        onTaskCreated={(newTask) => {
          setTasks((prev) => [newTask, ...prev])
          setNotification({
            type: 'success',
            message: `Task "${newTask.title}" berhasil dibuat dan ditambahkan ke board!`,
          })
        }}
      />

      {/* Standalone Quick-Access Interactive Terminal Modal (S09.4) */}
      {selectedTerminalTask && (
        <div className="fixed inset-0 z-50 flex items-center justify-center p-3 sm:p-6 bg-slate-950/80 backdrop-blur-sm animate-in fade-in duration-200">
          <div className="w-full max-w-5xl h-[620px] max-h-[92vh] flex flex-col shadow-2xl">
            <TaskTerminal
              taskId={selectedTerminalTask.id}
              taskTitle={selectedTerminalTask.title}
              initialMode="watch"
              onClose={() => setSelectedTerminalTask(null)}
              className="w-full h-full"
            />
          </div>
        </div>
      )}

      {/* Task Detail & Terminal Drawer / Modal (S09.4) */}
      {detailTask && (
        <div className="fixed inset-0 z-50 flex items-center justify-center p-4 bg-slate-950/80 backdrop-blur-sm animate-in fade-in duration-200">
          <div className="bg-white dark:bg-slate-900 border border-slate-200 dark:border-slate-800 rounded-2xl w-full max-w-4xl shadow-2xl overflow-hidden flex flex-col max-h-[90vh]">
            {/* Header */}
            <div className="px-6 py-4 border-b border-slate-200 dark:border-slate-800 flex items-center justify-between bg-slate-50 dark:bg-slate-950/50">
              <div className="space-y-1 min-w-0 pr-4">
                <div className="flex items-center gap-2 flex-wrap">
                  {getPriorityBadge(detailTask.priority)}
                  <Badge variant="outline" className="font-mono text-[10px] py-0 px-1.5 uppercase">
                    Status: {detailTask.status}
                  </Badge>
                  <span className="font-mono text-[10px] text-slate-400">
                    ID: #{detailTask.id}
                  </span>
                </div>
                <h2 className="text-base font-bold text-slate-900 dark:text-white truncate">
                  {detailTask.title}
                </h2>
              </div>

              <div className="flex items-center gap-2 shrink-0">
                {/* Tab switcher: Detail vs Agent Output vs Terminal vs WordPress */}
                <div className="flex items-center bg-slate-200 dark:bg-slate-800 p-0.5 rounded-lg text-xs font-semibold">
                  <button
                    type="button"
                    onClick={() => setDetailModalTab('details')}
                    className={`px-3 py-1 rounded-md transition-all ${
                      detailModalTab === 'details'
                        ? 'bg-white dark:bg-slate-900 text-slate-900 dark:text-white shadow-xs'
                        : 'text-slate-500 hover:text-slate-900 dark:text-slate-400'
                    }`}
                  >
                    Detail & DoD
                  </button>

                  <button
                    type="button"
                    onClick={() => setDetailModalTab('agent_output')}
                    className={`flex items-center gap-1.5 px-3 py-1 rounded-md transition-all ${
                      detailModalTab === 'agent_output'
                        ? 'bg-white dark:bg-slate-900 text-purple-600 dark:text-purple-400 shadow-xs'
                        : 'text-slate-500 hover:text-slate-900 dark:text-slate-400'
                    }`}
                  >
                    <Bot className="h-3.5 w-3.5 text-purple-500" />
                    <span>Agent Output</span>
                    {detailTask.agent_output && (
                      <span className="h-2 w-2 rounded-full bg-emerald-500 animate-pulse" />
                    )}
                  </button>

                  <button
                    type="button"
                    onClick={() => {
                      setDetailModalTab('wordpress')
                      fetchWordPressIntegration(selectedProjectId)
                    }}
                    className={`flex items-center gap-1.5 px-3 py-1 rounded-md transition-all ${
                      detailModalTab === 'wordpress'
                        ? 'bg-white dark:bg-slate-900 text-blue-600 dark:text-blue-400 shadow-xs'
                        : 'text-slate-500 hover:text-slate-900 dark:text-slate-400'
                    }`}
                  >
                    <Globe className="h-3.5 w-3.5 text-blue-500" />
                    <span>Publish to WordPress</span>
                    {detailTask.publication_url && (
                      <span className="h-2 w-2 rounded-full bg-blue-500" />
                    )}
                  </button>

                  <button
                    type="button"
                    onClick={() => setDetailModalTab('terminal')}
                    className={`flex items-center gap-1.5 px-3 py-1 rounded-md transition-all ${
                      detailModalTab === 'terminal'
                        ? 'bg-white dark:bg-slate-900 text-emerald-600 dark:text-emerald-400 shadow-xs'
                        : 'text-slate-500 hover:text-slate-900 dark:text-slate-400'
                    }`}
                  >
                    <TerminalIcon className="h-3.5 w-3.5 text-emerald-500" />
                    <span>Terminal (PTY)</span>
                  </button>
                </div>

                {/* Teleprompter Fullscreen Button */}
                <Button
                  size="sm"
                  variant="outline"
                  onClick={() => {
                    setTeleprompterTask(detailTask)
                    setIsTeleprompterOpen(true)
                  }}
                  className="h-7 text-xs font-semibold gap-1.5 border-slate-300 dark:border-slate-700 hover:bg-slate-100 dark:hover:bg-slate-800"
                  title="Buka Naskah di Teleprompter View Fullscreen"
                >
                  <Video className="h-3.5 w-3.5 text-red-500" />
                  <span>Teleprompter</span>
                </Button>

                <button
                  type="button"
                  onClick={() => setDetailTask(null)}
                  className="p-1.5 text-slate-400 hover:text-slate-600 dark:hover:text-slate-200 rounded-lg hover:bg-slate-100 dark:hover:bg-slate-800"
                >
                  <X className="h-4 w-4" />
                </button>
              </div>
            </div>

            {/* Modal Content */}
            <div className="p-6 overflow-y-auto flex-1">
              {detailModalTab === 'wordpress' ? (
                <div className="space-y-6">
                  {/* WordPress Publisher Header Card */}
                  <div className="p-4 rounded-2xl bg-gradient-to-r from-blue-500/10 via-slate-50 to-slate-100 dark:from-blue-950/30 dark:via-slate-900/60 dark:to-slate-900 border border-blue-200 dark:border-blue-800/60 flex flex-col sm:flex-row sm:items-center justify-between gap-4">
                    <div className="space-y-1">
                      <div className="flex items-center gap-2">
                        <span className="p-1.5 rounded-lg bg-blue-500/20 text-blue-600 dark:text-purple-400">
                          <Globe className="h-4 w-4 text-blue-500" />
                        </span>
                        <h3 className="text-sm font-bold text-slate-900 dark:text-white">
                          WordPress 1-Click Publishing Hub
                        </h3>
                        {wpConfigured && (
                          <Badge className="bg-emerald-600 text-white text-[10px] font-mono py-0 px-2">
                            Connected {wpIntegration ? `(${wpIntegration.site_url})` : ''}
                          </Badge>
                        )}
                        {wpLoading && (
                          <span className="text-[11px] text-slate-400 animate-pulse">Memeriksa...</span>
                        )}
                      </div>
                      <p className="text-xs text-slate-500 dark:text-slate-400">
                        Publikasikan draf konten atau deliverable task langsung ke WordPress REST API dengan enkripsi kredensial AES-256-GCM.
                      </p>
                    </div>

                    {wpConfigured && (
                      <Button
                        variant="outline"
                        size="sm"
                        onClick={() => setWpIsReconfiguring(!wpIsReconfiguring)}
                        className="text-xs h-8 border-slate-300 dark:border-slate-700 shrink-0"
                      >
                        <KeyRound className="h-3.5 w-3.5 mr-1" />
                        <span>{wpIsReconfiguring ? 'Tutup Pengaturan' : 'Ganti Kredensial'}</span>
                      </Button>
                    )}
                  </div>

                  {/* Publication Success Banner if published */}
                  {detailTask.publication_url && (
                    <div className="p-4 rounded-2xl bg-emerald-500/10 border border-emerald-500/30 text-emerald-800 dark:text-emerald-200 flex flex-col sm:flex-row sm:items-center justify-between gap-3">
                      <div className="space-y-1">
                        <span className="text-xs font-bold flex items-center gap-1.5 text-emerald-600 dark:text-emerald-400">
                          <CheckCircle2 className="h-4 w-4" /> Artikel Berhasil Terbit di WordPress!
                        </span>
                        <p className="text-xs text-slate-600 dark:text-slate-300">
                          URL: <a href={detailTask.publication_url} target="_blank" rel="noopener noreferrer" className="font-mono text-emerald-600 dark:text-emerald-400 underline">{detailTask.publication_url}</a>
                        </p>
                        {detailTask.published_at && (
                          <span className="text-[10px] text-slate-400 font-mono block">
                            Diterbitkan pada: {new Date(detailTask.published_at).toLocaleString()}
                          </span>
                        )}
                      </div>
                      <a
                        href={detailTask.publication_url}
                        target="_blank"
                        rel="noopener noreferrer"
                        className="inline-flex items-center gap-1.5 px-3 py-1.5 rounded-xl bg-emerald-600 hover:bg-emerald-500 text-white text-xs font-bold shrink-0 transition-colors shadow-sm"
                      >
                        <span>Kunjungi Postingan</span>
                        <ExternalLink className="h-3.5 w-3.5" />
                      </a>
                    </div>
                  )}

                  {/* Connect Form (if not configured or reconfiguring) */}
                  {(!wpConfigured || wpIsReconfiguring) ? (
                    <div className="p-5 rounded-2xl bg-slate-50 dark:bg-slate-950 border border-slate-200 dark:border-slate-800 space-y-4">
                      <div className="flex items-center gap-2 border-b border-slate-200 dark:border-slate-800 pb-3">
                        <Lock className="h-4 w-4 text-amber-500" />
                        <h4 className="text-xs font-bold text-slate-900 dark:text-white uppercase tracking-wider">
                          Konfigurasi WordPress REST API
                        </h4>
                      </div>

                      {wpConnectError && (
                        <Alert className="bg-red-500/10 border-red-500/20 text-red-700 dark:text-red-300">
                          <AlertCircle className="h-4 w-4 text-red-500" />
                          <AlertTitle className="text-xs font-bold">Koneksi Gagal</AlertTitle>
                          <AlertDescription className="text-xs mt-0.5">{wpConnectError}</AlertDescription>
                        </Alert>
                      )}

                      {wpConnectSuccess && (
                        <Alert className="bg-emerald-500/10 border-emerald-500/20 text-emerald-700 dark:text-emerald-300">
                          <CheckCircle2 className="h-4 w-4 text-emerald-500" />
                          <AlertTitle className="text-xs font-bold">Terhubung</AlertTitle>
                          <AlertDescription className="text-xs mt-0.5">{wpConnectSuccess}</AlertDescription>
                        </Alert>
                      )}

                      <div className="grid grid-cols-1 sm:grid-cols-2 gap-3 text-xs">
                        <div className="space-y-1">
                          <label className="font-bold text-slate-700 dark:text-slate-300">
                            WordPress Site URL
                          </label>
                          <Input
                            value={wpSiteURL}
                            onChange={(e) => setWpSiteURL(e.target.value)}
                            placeholder="https://myblog.com"
                            className="text-xs"
                          />
                        </div>

                        <div className="space-y-1">
                          <label className="font-bold text-slate-700 dark:text-slate-300">
                            Username WordPress
                          </label>
                          <Input
                            value={wpUsername}
                            onChange={(e) => setWpUsername(e.target.value)}
                            placeholder="admin"
                            className="text-xs"
                          />
                        </div>
                      </div>

                      <div className="space-y-1 text-xs">
                        <label className="font-bold text-slate-700 dark:text-slate-300">
                          Application Password
                        </label>
                        <Input
                          type="password"
                          value={wpAppPassword}
                          onChange={(e) => setWpAppPassword(e.target.value)}
                          placeholder="xxxx xxxx xxxx xxxx"
                          className="text-xs font-mono"
                        />
                        <p className="text-[11px] text-slate-400 mt-1">
                          Buat di WP Admin &rarr; Users &rarr; Profile &rarr; Application Passwords. Password Anda dienkripsi AES-256-GCM dan tidak pernah bocor.
                        </p>
                      </div>

                      <div className="flex justify-end pt-2">
                        <Button
                          size="sm"
                          disabled={wpIsConnecting}
                          onClick={handleConnectWordPress}
                          className="bg-blue-600 hover:bg-blue-500 text-white text-xs font-bold gap-1.5"
                        >
                          {wpIsConnecting ? (
                            <>
                              <RefreshCw className="h-3.5 w-3.5 animate-spin" />
                              <span>Memverifikasi Koneksi...</span>
                            </>
                          ) : (
                            <>
                              <KeyRound className="h-3.5 w-3.5" />
                              <span>Test & Simpan Koneksi WordPress</span>
                            </>
                          )}
                        </Button>
                      </div>
                    </div>
                  ) : null}

                  {/* Publish Form (when configured) */}
                  {wpConfigured && (
                    <div className="p-5 rounded-2xl bg-white dark:bg-slate-950 border border-slate-200 dark:border-slate-800 space-y-4">
                      <div className="flex items-center justify-between border-b border-slate-200 dark:border-slate-800 pb-3">
                        <div className="space-y-0.5">
                          <h4 className="text-xs font-bold text-slate-900 dark:text-white uppercase tracking-wider flex items-center gap-1.5">
                            <Send className="h-3.5 w-3.5 text-blue-500" />
                            <span>Form Publikasi Konten</span>
                          </h4>
                          <span className="text-[11px] text-slate-400">
                            Target: <strong className="text-slate-700 dark:text-slate-200">{wpSiteURL}</strong> ({wpUsername})
                          </span>
                        </div>

                        <div className="flex items-center gap-2">
                          <label className="flex items-center gap-1.5 text-xs text-slate-600 dark:text-slate-400 cursor-pointer">
                            <input
                              type="checkbox"
                              checked={wpConvertMarkdown}
                              onChange={(e) => setWpConvertMarkdown(e.target.checked)}
                              className="rounded text-blue-600 focus:ring-blue-500 h-3.5 w-3.5"
                            />
                            <span>Convert Markdown to HTML</span>
                          </label>
                        </div>
                      </div>

                      {wpPublishError && (
                        <Alert className="bg-red-500/10 border-red-500/20 text-red-700 dark:text-red-300">
                          <AlertCircle className="h-4 w-4 text-red-500" />
                          <AlertTitle className="text-xs font-bold">Publikasi Gagal</AlertTitle>
                          <AlertDescription className="text-xs mt-0.5">{wpPublishError}</AlertDescription>
                        </Alert>
                      )}

                      {wpPublishResult && (
                        <div className="p-3 rounded-xl bg-emerald-500/10 border border-emerald-500/20 text-xs text-emerald-800 dark:text-emerald-200 flex items-center justify-between">
                          <span>Post berhasil dibuat! ID: #{wpPublishResult.post_id} | Status: {wpPublishResult.status}</span>
                          {wpPublishResult.publication_url && (
                            <a href={wpPublishResult.publication_url} target="_blank" rel="noopener noreferrer" className="underline font-bold text-emerald-600">
                              Buka Link
                            </a>
                          )}
                        </div>
                      )}

                      <div className="grid grid-cols-1 sm:grid-cols-3 gap-3 text-xs">
                        <div className="sm:col-span-2 space-y-1">
                          <label className="font-bold text-slate-700 dark:text-slate-300">
                            Judul Postingan
                          </label>
                          <Input
                            value={wpPublishTitle || detailTask.title}
                            onChange={(e) => setWpPublishTitle(e.target.value)}
                            placeholder="Judul artikel WordPress"
                            className="text-xs"
                          />
                        </div>

                        <div className="space-y-1">
                          <label className="font-bold text-slate-700 dark:text-slate-300">
                            Status Postingan
                          </label>
                          <select
                            value={wpPublishStatus}
                            onChange={(e: any) => setWpPublishStatus(e.target.value)}
                            className="w-full h-9 rounded-md border border-slate-200 dark:border-slate-800 bg-white dark:bg-slate-900 px-3 py-1 text-xs text-slate-900 dark:text-slate-100 focus:outline-hidden focus:ring-2 focus:ring-blue-500"
                          >
                            <option value="draft">Draft (Draf - Aman direview)</option>
                            <option value="publish">Publish (Langsung Live Publik)</option>
                            <option value="pending">Pending Review</option>
                          </select>
                        </div>
                      </div>

                      {/* Content Preview */}
                      <div className="space-y-1 text-xs">
                        <div className="flex items-center justify-between">
                          <label className="font-bold text-slate-700 dark:text-slate-300">
                            Konten yang Akan Diterbitkan
                          </label>
                          <span className="text-[10px] text-slate-400 font-mono">
                            {detailTask.agent_output
                              ? `Output Agent (${detailTask.agent_output.length} karakter)`
                              : `Deskripsi Task (${detailTask.description?.length || 0} karakter)`}
                          </span>
                        </div>

                        <div className="p-3.5 rounded-xl bg-slate-50 dark:bg-slate-900 border border-slate-200 dark:border-slate-800 text-xs text-slate-800 dark:text-slate-200 max-h-48 overflow-y-auto whitespace-pre-wrap font-sans leading-relaxed">
                          {detailTask.agent_output || detailTask.description || (
                            <span className="text-slate-400 italic">Belum ada konten atau draf output agent.</span>
                          )}
                        </div>
                      </div>

                      <div className="flex items-center justify-between pt-2">
                        <Button
                          variant="outline"
                          size="sm"
                          onClick={() => {
                            setTeleprompterTask(detailTask)
                            setIsTeleprompterOpen(true)
                          }}
                          className="text-xs gap-1.5 border-slate-300 dark:border-slate-700"
                        >
                          <Video className="h-3.5 w-3.5 text-red-500" />
                          <span>Buka Teleprompter</span>
                        </Button>

                        <Button
                          size="sm"
                          disabled={wpIsPublishing || (!detailTask.agent_output && !detailTask.description)}
                          onClick={handlePublishToWordPress}
                          className="bg-blue-600 hover:bg-blue-500 text-white text-xs font-bold gap-1.5 shadow-md shadow-blue-600/20 px-5"
                        >
                          {wpIsPublishing ? (
                            <>
                              <RefreshCw className="h-3.5 w-3.5 animate-spin" />
                              <span>Menerbitkan ke WordPress...</span>
                            </>
                          ) : (
                            <>
                              <Globe className="h-3.5 w-3.5" />
                              <span>Publish to WordPress Sekarang</span>
                            </>
                          )}
                        </Button>
                      </div>
                    </div>
                  )}
                </div>
              ) : detailModalTab === 'agent_output' ? (
                <div className="space-y-6">
                  {/* Agent Output Header Card */}
                  <div className="p-4 rounded-2xl bg-gradient-to-r from-purple-500/10 via-slate-50 to-slate-100 dark:from-purple-950/30 dark:via-slate-900/60 dark:to-slate-900 border border-purple-200 dark:border-purple-800/60 flex flex-col sm:flex-row sm:items-center justify-between gap-4">
                    <div className="space-y-1">
                      <div className="flex items-center gap-2">
                        <span className="p-1.5 rounded-lg bg-purple-500/20 text-purple-600 dark:text-purple-400">
                          <Bot className="h-4 w-4" />
                        </span>
                        <h3 className="text-sm font-bold text-slate-900 dark:text-white">
                          Deliverable Draf Konten AI Agent
                        </h3>
                        {detailTask.execution_status && (
                          <Badge
                            className={`text-[10px] font-mono py-0 px-2 uppercase ${
                              detailTask.execution_status === 'running'
                                ? 'bg-purple-600 text-white animate-pulse'
                                : detailTask.execution_status === 'completed'
                                ? 'bg-emerald-600 text-white'
                                : 'bg-slate-200 text-slate-700 dark:bg-slate-800 dark:text-slate-300'
                            }`}
                          >
                            {detailTask.execution_status}
                          </Badge>
                        )}
                      </div>
                      <p className="text-xs text-slate-500 dark:text-slate-400">
                        Draf konten siap pakai hasil eksekusi Google Gemini Pro berdasarkan instruksi dan kriteria DoD task.
                      </p>
                    </div>

                    <div className="flex items-center gap-2 shrink-0">
                      <Button
                        size="sm"
                        variant="outline"
                        onClick={() => {
                          setTeleprompterTask(detailTask)
                          setIsTeleprompterOpen(true)
                        }}
                        className="text-xs font-semibold gap-1.5 border-slate-300 dark:border-slate-700"
                        title="Buka naskah di Teleprompter View Fullscreen"
                      >
                        <Video className="h-3.5 w-3.5 text-red-500" />
                        <span>Teleprompter View</span>
                      </Button>

                      <Button
                        size="sm"
                        onClick={() => {
                          setDetailModalTab('wordpress')
                          fetchWordPressIntegration(selectedProjectId)
                        }}
                        className="bg-blue-600 hover:bg-blue-500 text-white text-xs font-semibold gap-1.5 shadow-sm"
                        title="Publikasikan langsung ke WordPress REST API"
                      >
                        <Globe className="h-3.5 w-3.5" />
                        <span>Publish to WordPress</span>
                      </Button>

                      <Button
                        size="sm"
                        disabled={runningAgentTaskIds[detailTask.id] || detailTask.execution_status === 'running'}
                        onClick={() => handleRunAgent(detailTask)}
                        className="bg-purple-600 hover:bg-purple-500 text-white text-xs font-semibold gap-1.5 shadow-sm"
                      >
                        {runningAgentTaskIds[detailTask.id] || detailTask.execution_status === 'running' ? (
                          <>
                            <RefreshCw className="h-3.5 w-3.5 animate-spin" />
                            <span>Mengeksekusi...</span>
                          </>
                        ) : (
                          <>
                            <Sparkles className="h-3.5 w-3.5" />
                            <span>{detailTask.agent_output ? 'Jalankan Ulang Agent' : 'Run with Agent'}</span>
                          </>
                        )}
                      </Button>
                    </div>
                  </div>

                  {geminiKeyWarning && (
                    <Alert className="bg-amber-500/10 border-amber-500/30 text-amber-800 dark:text-amber-200">
                      <AlertTitle className="text-xs font-bold flex items-center gap-1.5">
                        <AlertCircle className="h-4 w-4 text-amber-500" />
                        <span>Koneksi Google Gemini Belum Ditemukan</span>
                      </AlertTitle>
                      <AlertDescription className="text-xs mt-1">
                        {geminiKeyWarning}
                      </AlertDescription>
                    </Alert>
                  )}

                  {/* Render Markdown Deliverable */}
                  {detailTask.agent_output ? (
                    <div className="space-y-3">
                      <div className="p-5 rounded-2xl bg-white dark:bg-slate-950 border border-slate-200 dark:border-slate-800 shadow-sm">
                        <MarkdownViewer content={detailTask.agent_output} showCopyButton={true} />
                      </div>
                    </div>
                  ) : (
                    <div className="p-10 text-center rounded-2xl bg-slate-50 dark:bg-slate-950/60 border border-dashed border-slate-200 dark:border-slate-800 space-y-4">
                      <div className="h-12 w-12 rounded-2xl bg-purple-500/10 text-purple-600 dark:text-purple-400 mx-auto flex items-center justify-center">
                        <Bot className="h-6 w-6" />
                      </div>
                      <div className="max-w-md mx-auto space-y-1">
                        <h4 className="text-sm font-bold text-slate-800 dark:text-slate-200">
                          Belum Ada Output Agent untuk Task Ini
                        </h4>
                        <p className="text-xs text-slate-500 dark:text-slate-400 leading-relaxed">
                          Tekan tombol di bawah untuk meminta AI Agent membaca instruksi dan kriteria DoD, lalu membuat draft deliverable siap pakai.
                        </p>
                      </div>
                      <Button
                        onClick={() => handleRunAgent(detailTask)}
                        disabled={runningAgentTaskIds[detailTask.id]}
                        className="bg-purple-600 hover:bg-purple-500 text-white text-xs font-bold px-5 gap-2 shadow-md shadow-purple-600/20"
                      >
                        <Sparkles className="h-3.5 w-3.5" />
                        <span>Run with Agent Sekarang</span>
                      </Button>
                    </div>
                  )}
                </div>
              ) : detailModalTab === 'terminal' ? (
                <div className="h-[520px] w-full">
                  <TaskTerminal
                    taskId={detailTask.id}
                    taskTitle={detailTask.title}
                    initialMode="watch"
                    className="w-full h-full"
                  />
                </div>
              ) : (
                <div className="space-y-6">
                  {/* Description */}
                  <div className="space-y-1.5">
                    <h4 className="text-xs font-bold text-slate-500 dark:text-slate-400 uppercase tracking-wider">
                      Deskripsi & Instruksi Pengerjaan
                    </h4>
                    <div className="p-3.5 rounded-xl bg-slate-50 dark:bg-slate-950 border border-slate-200 dark:border-slate-800 text-xs text-slate-800 dark:text-slate-200 leading-relaxed whitespace-pre-wrap">
                      {detailTask.description || 'Tidak ada deskripsi detail untuk task ini.'}
                    </div>
                  </div>

                  {/* DoD Checklist */}
                  <div className="space-y-2">
                    <div className="flex items-center justify-between">
                      <h4 className="text-xs font-bold text-slate-500 dark:text-slate-400 uppercase tracking-wider flex items-center gap-1.5">
                        <ListTodo className="h-3.5 w-3.5 text-emerald-500" />
                        <span>Definition of Done (DoD) Checklist</span>
                      </h4>
                      <span className="text-xs font-mono font-semibold text-emerald-600 dark:text-emerald-400">
                        {(detailTask.dod_checklist || []).filter((d) => d.completed).length} /{' '}
                        {(detailTask.dod_checklist || []).length} Selesai
                      </span>
                    </div>

                    <div className="p-3 rounded-xl bg-slate-50 dark:bg-slate-950 border border-slate-200 dark:border-slate-800 space-y-2">
                      {(detailTask.dod_checklist || []).length === 0 ? (
                        <p className="text-xs text-slate-400 italic">Tidak ada kriteria DoD yang didefinisikan.</p>
                      ) : (
                        detailTask.dod_checklist.map((item, idx) => (
                          <label
                            key={idx}
                            className="flex items-start gap-2.5 text-xs text-slate-700 dark:text-slate-300 cursor-pointer p-1.5 rounded-lg hover:bg-white dark:hover:bg-slate-900 transition-colors"
                          >
                            <input
                              type="checkbox"
                              checked={item.completed}
                              onChange={() => {
                                handleToggleDoD(detailTask, idx)
                                setDetailTask((prev) =>
                                  prev
                                    ? {
                                        ...prev,
                                        dod_checklist: prev.dod_checklist.map((d, i) =>
                                          i === idx ? { ...d, completed: !d.completed } : d
                                        ),
                                      }
                                    : null
                                )
                              }}
                              className="rounded text-emerald-600 focus:ring-emerald-500 mt-0.5 h-3.5 w-3.5 shrink-0"
                            />
                            <span className={item.completed ? 'line-through text-slate-400' : ''}>
                              {item.text}
                            </span>
                          </label>
                        ))
                      )}
                    </div>
                  </div>

                  {/* Meta Information Grid */}
                  <div className="grid grid-cols-2 sm:grid-cols-4 gap-3 text-xs">
                    <div className="p-2.5 rounded-xl bg-slate-50 dark:bg-slate-950 border border-slate-200 dark:border-slate-800">
                      <span className="text-[10px] text-slate-400 block">Status Saat Ini</span>
                      <span className="font-bold text-slate-800 dark:text-slate-200 uppercase mt-0.5 block">
                        {detailTask.status}
                      </span>
                    </div>
                    <div className="p-2.5 rounded-xl bg-slate-50 dark:bg-slate-950 border border-slate-200 dark:border-slate-800">
                      <span className="text-[10px] text-slate-400 block">Prioritas</span>
                      <span className="font-bold text-slate-800 dark:text-slate-200 capitalize mt-0.5 block">
                        {detailTask.priority}
                      </span>
                    </div>
                    <div className="p-2.5 rounded-xl bg-slate-50 dark:bg-slate-950 border border-slate-200 dark:border-slate-800">
                      <span className="text-[10px] text-slate-400 block">Batch ID</span>
                      <span className="font-mono font-medium text-slate-800 dark:text-slate-200 mt-0.5 block">
                        {detailTask.batch_id || '-'}
                      </span>
                    </div>
                    <div className="p-2.5 rounded-xl bg-slate-50 dark:bg-slate-950 border border-slate-200 dark:border-slate-800">
                      <span className="text-[10px] text-slate-400 block">Subtask / Parent</span>
                      <span className="font-mono font-medium text-slate-800 dark:text-slate-200 mt-0.5 block">
                        {detailTask.parent_task_id ? `#${detailTask.parent_task_id.slice(0, 8)}` : 'Root'}
                      </span>
                    </div>
                  </div>

                  {/* Launch Terminal CTA button */}
                  <div className="p-4 rounded-xl bg-gradient-to-r from-emerald-950/30 via-slate-900/60 to-slate-900 border border-emerald-500/30 flex items-center justify-between gap-3">
                    <div className="space-y-0.5">
                      <span className="text-xs font-bold text-emerald-400 flex items-center gap-1.5">
                        <TerminalIcon className="h-3.5 w-3.5" /> In-Browser Pseudo-Terminal
                      </span>
                      <p className="text-[11px] text-slate-400">
                        Buka shell terminal langsung ke direktori kerja Git task untuk memantau agent atau melakukan intervensi perintah.
                      </p>
                    </div>
                    <Button
                      size="sm"
                      onClick={() => setDetailModalTab('terminal')}
                      className="bg-emerald-600 hover:bg-emerald-500 text-white font-semibold text-xs gap-1.5 shrink-0"
                    >
                      <TerminalIcon className="h-3.5 w-3.5" />
                      <span>Buka Terminal Task</span>
                    </Button>
                  </div>
                </div>
              )}
            </div>
          </div>
        </div>
      )}

      {/* Teleprompter Fullscreen View (Sprint 11 S11.4) */}
      {teleprompterTask && (
        <TeleprompterModal
          isOpen={isTeleprompterOpen}
          onClose={() => {
            setIsTeleprompterOpen(false)
            setTeleprompterTask(null)
          }}
          title={teleprompterTask.title}
          content={teleprompterTask.agent_output || teleprompterTask.description}
        />
      )}

      {/* Script Deployer Modal (Sprint 12 S12.4) */}
      <ScriptDeployerModal
        isOpen={isDeployerModalOpen}
        onClose={() => setIsDeployerModalOpen(false)}
        projectId={selectedProjectId}
        projectName={effectiveProjects.find((p) => p.id === selectedProjectId)?.name || selectedProjectId}
        initialStep={deployerInitialStep}
        onDeploymentSuccess={() => {
          setNotification({
            type: 'success',
            message: '🚀 Script deployment berhasil dijalankan via supervisor runner!',
          })
        }}
      />

      {/* WordPress Deployer Modal (Sprint 13 S13.4) */}
      <WordPressDeployerModal
        isOpen={isWordPressModalOpen}
        onClose={() => setIsWordPressModalOpen(false)}
        projectId={selectedProjectId}
        projectName={effectiveProjects.find((p) => p.id === selectedProjectId)?.name || selectedProjectId}
        initialStep={wordPressInitialStep}
        onDeploymentSuccess={() => {
          setNotification({
            type: 'success',
            message: '🎉 Turnkey WordPress Cloud deployment live & healthy!',
          })
        }}
      />
    </div>
  )
}

