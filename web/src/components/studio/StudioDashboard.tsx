import { useState, useEffect, useMemo } from 'react'
import { Card } from '../ui/card'
import { Button } from '../ui/button'
import { Input } from '../ui/input'
import { Badge } from '../ui/badge'
import { Alert, AlertTitle, AlertDescription } from '../ui/alert'
import {
  Plus,
  RefreshCw,
  Search,
  ExternalLink,
  Play,
  Square,
  Globe,
  Layers,
  ArrowLeft,
  Sparkles,
  AlertCircle,
  UploadCloud,
  Server,
  Activity,
  MessageSquare,
  Kanban,
  Rocket,
  Bot,
  Settings2,
  GitBranch,
  Share2,
  HelpCircle,
  ChevronDown
} from 'lucide-react'
import {
  DeploymentItem,
  ServerItem,
  CustomDomainItem,
  InstantiateCreatorProjectResponse,
  ProjectItemDTO,
  CreateFromZipResponse
} from './types'
import { DeployWizardModal } from './DeployWizardModal'
import { CreatorWizardModal } from './CreatorWizardModal'
import { ProjectDrawer } from './ProjectDrawer'
import { ServersView } from './ServersView'
import { DomainsView } from './DomainsView'
import { ObservabilityView } from './ObservabilityView'
import { CustomDomainsModal } from './CustomDomainsModal'
import { ProjectCard } from './ProjectCard'
import { ProjectDiscussionsView } from './ProjectDiscussionsView'
import { ProjectTasksBoard } from './ProjectTasksBoard'
import { BatchCampaignModal } from './BatchCampaignModal'
import { CampaignDashboardView } from './CampaignDashboardView'
import { CampaignDetailView } from './CampaignDetailView'
import { AgentsHubView } from './AgentsHubView'
import { ModuleToggleModal } from './ModuleToggleModal'
import { ZipUploadWizardModal } from './ZipUploadWizardModal'
import { ScriptDeployerView } from './deployer'

export type StudioTab =
  | 'projects'
  | 'deployer'
  | 'discussions'
  | 'tasks'
  | 'campaigns'
  | 'agents'
  | 'servers'
  | 'domains'
  | 'observability'
  | 'repositories'
  | 'publishing'
  | 'support'

interface StudioDashboardProps {
  currentPath?: string
  onNavigate?: (path: string) => void
  baseDomain?: string
  appMode?: 'marketing' | 'studio'
  isStudioModeActive?: boolean
  onToggleMode?: (newMode: 'marketing' | 'studio') => void
}

export function StudioDashboard({
  currentPath,
  onNavigate,
  baseDomain = 'cuan.dojo.cc',
  appMode = 'studio',
  isStudioModeActive = false,
  onToggleMode,
}: StudioDashboardProps) {
  const [deployments, setDeployments] = useState<DeploymentItem[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [notification, setNotification] = useState<{ type: 'success' | 'error'; message: string } | null>(null)

  // Drawer initial tab state
  const [drawerInitialTab, setDrawerInitialTab] = useState<'overview' | 'discussions' | 'tasks'>('overview')

  // Tab state: StudioTab
  const [activeTab, setActiveTab] = useState<StudioTab>(() => {
    if (typeof window !== 'undefined') {
      if (window.location.pathname.startsWith('/studio/agents') || window.location.pathname.startsWith('/agents')) {
        return 'agents'
      }
      if (window.location.pathname.startsWith('/studio/deployer') || window.location.pathname.startsWith('/deployer')) {
        return 'deployer'
      }
      if (window.location.pathname.startsWith('/studio/campaigns') || window.location.pathname.startsWith('/campaigns')) {
        return 'campaigns'
      }
      if (window.location.pathname.startsWith('/studio/discussions')) {
        return 'discussions'
      }
      if (window.location.pathname.startsWith('/studio/tasks')) {
        return 'tasks'
      }
      if (window.location.pathname.startsWith('/studio/observability')) {
        return 'observability'
      }
      if (window.location.pathname.startsWith('/studio/domains')) {
        return 'domains'
      }
      if (window.location.pathname.startsWith('/studio/servers')) {
        return 'servers'
      }
      if (window.location.pathname.startsWith('/studio/repositories')) {
        return 'repositories'
      }
      if (window.location.pathname.startsWith('/studio/publishing')) {
        return 'publishing'
      }
      if (window.location.pathname.startsWith('/studio/support')) {
        return 'support'
      }
    }
    return 'projects'
  })

  // Selected Campaign ID for Detail View
  const [selectedCampaignId, setSelectedCampaignId] = useState<string | null>(() => {
    if (typeof window !== 'undefined') {
      const m = window.location.pathname.match(/\/(?:studio\/)?campaigns\/([a-zA-Z0-9_-]+)/)
      return m ? m[1] : null
    }
    return null
  })

  // Batch Campaign Modal state
  const [isBatchModalOpen, setIsBatchModalOpen] = useState(false)

  // Preset Workspaces for Dynamic Navigation (S11.4 DoD 1)
  const DEFAULT_PRESET_PROJECTS: ProjectItemDTO[] = useMemo(() => [
    {
      id: 'saas-dojo',
      name: 'SaaS / Dojo Workspace',
      slug: 'saas-dojo',
      project_type: 'saas',
      enabled_modules: ['tasks', 'deployer', 'discussions', 'repositories', 'servers'],
    },
    {
      id: 'creator-hub',
      name: 'Creator Hub Workspace',
      slug: 'creator-hub',
      project_type: 'creator',
      enabled_modules: ['tasks', 'discussions', 'publishing'],
    },
    {
      id: 'blank-workspace',
      name: 'Blank Workspace',
      slug: 'blank-workspace',
      project_type: 'blank',
      enabled_modules: ['tasks', 'discussions'],
    },
  ], [])

  const [projectsList, setProjectsList] = useState<ProjectItemDTO[]>(DEFAULT_PRESET_PROJECTS)
  const [activeProjectId, setActiveProjectId] = useState<string>('saas-dojo')
  const [isModuleToggleModalOpen, setIsModuleToggleModalOpen] = useState(false)
  const [isZipWizardOpen, setIsZipWizardOpen] = useState(false)

  const fetchProjectsList = async () => {
    try {
      const res = await fetch('/api/v1/projects')
      if (res.ok) {
        const data = await res.json()
        const serverProjects: ProjectItemDTO[] = data.projects || []
        const existingIds = new Set(serverProjects.map((p) => p.id))
        const combined = [
          ...serverProjects,
          ...DEFAULT_PRESET_PROJECTS.filter((p) => !existingIds.has(p.id)),
        ]
        setProjectsList(combined)
      }
    } catch {}
  }

  useEffect(() => {
    fetchProjectsList()
  }, [])

  const activeProject = useMemo<ProjectItemDTO>(() => {
    return (
      projectsList.find((p) => p.id === activeProjectId) ||
      projectsList[0] ||
      DEFAULT_PRESET_PROJECTS[0]
    )
  }, [projectsList, activeProjectId, DEFAULT_PRESET_PROJECTS])

  const enabledModules = useMemo<string[]>(() => {
    return activeProject.enabled_modules && activeProject.enabled_modules.length > 0
      ? activeProject.enabled_modules
      : ['tasks', 'discussions']
  }, [activeProject])

  const handleSelectProject = (projectId: string) => {
    setActiveProjectId(projectId)
    const targetProj = projectsList.find((p) => p.id === projectId)
    const targetModules = targetProj?.enabled_modules || ['tasks', 'discussions']
    if (
      ['tasks', 'deployer', 'discussions', 'repositories', 'servers', 'publishing', 'support'].includes(activeTab) &&
      !targetModules.includes(activeTab)
    ) {
      if (targetModules.includes('tasks')) {
        setActiveTab('tasks')
      } else if (targetModules.includes('deployer')) {
        setActiveTab('deployer')
      } else if (targetModules.includes('discussions')) {
        setActiveTab('discussions')
      } else {
        setActiveTab('projects')
      }
    }
    showToast('success', `Beralih ke workspace "${targetProj?.name || projectId}"`)
  }

  const handleSaveModules = async (updatedModules: string[]) => {
    setProjectsList((prev) =>
      prev.map((p) => (p.id === activeProject.id ? { ...p, enabled_modules: updatedModules } : p))
    )
    try {
      await fetch(`/api/v1/projects/${activeProject.id}/modules`, {
        method: 'PATCH',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ enabled_modules: updatedModules }),
      })
    } catch {}
    showToast(
      'success',
      `Modul proyek "${activeProject.name}" berhasil diperbarui: ${updatedModules.join(', ')}`
    )
    if (
      ['tasks', 'deployer', 'discussions', 'repositories', 'servers', 'publishing', 'support'].includes(activeTab) &&
      !updatedModules.includes(activeTab)
    ) {
      if (updatedModules.includes('tasks')) {
        setActiveTab('tasks')
      } else if (updatedModules.includes('deployer')) {
        setActiveTab('deployer')
      } else if (updatedModules.includes('discussions')) {
        setActiveTab('discussions')
      } else {
        setActiveTab('projects')
      }
    }
  }

  // Agents count for header badge
  const [agentsCount, setAgentsCount] = useState<number>(0)
  const fetchAgentsCount = async () => {
    try {
      const res = await fetch('/api/agents')
      if (res.ok) {
        const data = await res.json()
        setAgentsCount(data.total ?? data.count ?? (data.agents?.length || 0))
      }
    } catch {}
  }

  useEffect(() => {
    fetchAgentsCount()
  }, [activeTab])

  // Synchronize when currentPath prop changes
  useEffect(() => {
    if (currentPath === '/studio/agents' || currentPath?.startsWith('/studio/agents') || currentPath === '/agents' || currentPath?.startsWith('/agents')) {
      setActiveTab('agents')
    } else if (currentPath === '/studio/deployer' || currentPath?.startsWith('/studio/deployer') || currentPath === '/deployer' || currentPath?.startsWith('/deployer')) {
      setActiveTab('deployer')
    } else if (currentPath === '/studio/campaigns' || currentPath?.startsWith('/studio/campaigns') || currentPath === '/campaigns' || currentPath?.startsWith('/campaigns')) {
      setActiveTab('campaigns')
      const m = currentPath.match(/\/(?:studio\/)?campaigns\/([a-zA-Z0-9_-]+)/)
      if (m && m[1] && m[1] !== 'new') {
        setSelectedCampaignId(m[1])
      } else {
        setSelectedCampaignId(null)
      }
    } else if (currentPath === '/studio/discussions' || currentPath?.startsWith('/studio/discussions')) {
      setActiveTab('discussions')
    } else if (currentPath === '/studio/tasks' || currentPath?.startsWith('/studio/tasks')) {
      setActiveTab('tasks')
    } else if (currentPath === '/studio/observability') {
      setActiveTab('observability')
    } else if (currentPath === '/studio/domains') {
      setActiveTab('domains')
    } else if (currentPath === '/studio/servers') {
      setActiveTab('servers')
    } else if (currentPath === '/studio/repositories' || currentPath?.startsWith('/studio/repositories')) {
      setActiveTab('repositories')
    } else if (currentPath === '/studio/publishing' || currentPath?.startsWith('/studio/publishing')) {
      setActiveTab('publishing')
    } else if (currentPath === '/studio/support' || currentPath?.startsWith('/studio/support')) {
      setActiveTab('support')
    } else if (currentPath === '/studio') {
      setActiveTab('projects')
    }
  }, [currentPath])

  // Custom domains state (S05.4)
  const [domains, setDomains] = useState<CustomDomainItem[]>([])
  const [selectedDomainDeployment, setSelectedDomainDeployment] = useState<DeploymentItem | null>(null)
  const [isCustomDomainsModalOpen, setIsCustomDomainsModalOpen] = useState(false)

  const fetchDomains = async () => {
    try {
      const res = await fetch('/api/domains')
      if (res.ok) {
        const data = await res.json()
        setDomains(data.domains || [])
      }
    } catch {}
  }

  const handleOpenCustomDomains = (dep: DeploymentItem) => {
    setSelectedDomainDeployment(dep)
    setIsCustomDomainsModalOpen(true)
  }

  // Remote servers state
  const [servers, setServers] = useState<ServerItem[]>([])

  const fetchServers = async () => {
    try {
      const res = await fetch('/api/servers')
      if (res.ok) {
        const data = await res.json()
        setServers(data.servers || [])
      }
    } catch {}
  }

  useEffect(() => {
    fetchServers()
    fetchDomains()
    const interval = setInterval(() => {
      fetchServers()
      fetchDomains()
    }, 8000)
    return () => clearInterval(interval)
  }, [])

  // Filters & Search
  const [searchQuery, setSearchQuery] = useState('')
  const [runtimeFilter, setRuntimeFilter] = useState<'all' | 'php' | 'nodejs' | 'static'>('all')
  const [statusFilter, setStatusFilter] = useState<'all' | 'running' | 'stopped' | 'failed'>('all')

  // Modals & Drawer
  const [isWizardOpen, setIsWizardOpen] = useState(false)
  const [isCreatorWizardOpen, setIsCreatorWizardOpen] = useState(false)
  const [selectedDeployment, setSelectedDeployment] = useState<DeploymentItem | null>(null)
  const [isDrawerOpen, setIsDrawerOpen] = useState(false)

  // Fast action loading states per ID
  const [actionLoadingId, setActionLoadingId] = useState<string | null>(null)
  const [copiedId, setCopiedId] = useState<string | null>(null)

  const fetchDeployments = async () => {
    setError(null)
    try {
      const res = await fetch('/api/deployments')
      if (!res.ok) {
        throw new Error(`Gagal mengambil data deployment (HTTP ${res.status})`)
      }
      const data = await res.json()
      setDeployments(data.deployments || [])
    } catch (err: any) {
      setError(err.message || 'Terjadi kesalahan jaringan saat memuat deployment.')
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    fetchDeployments()
    // Poll list every 5 seconds for status updates
    const interval = setInterval(fetchDeployments, 5000)
    return () => clearInterval(interval)
  }, [])

  // Show auto-dismiss notification
  const showToast = (type: 'success' | 'error', message: string) => {
    setNotification({ type, message })
    setTimeout(() => setNotification(null), 4000)
  }

  // 1-Click quick actions on card
  const handleQuickStart = async (dep: DeploymentItem) => {
    setActionLoadingId(dep.id)
    try {
      const res = await fetch(`/api/deployments/${dep.id}/start`, { method: 'POST' })
      if (!res.ok) {
        const err = await res.json().catch(() => ({}))
        throw new Error(err.error || `HTTP ${res.status}`)
      }
      const data = await res.json()
      handleDeploymentUpdated(data.deployment)
      showToast('success', `Aplikasi "${dep.project_name}" berhasil dijalankan.`)
    } catch (err: any) {
      showToast('error', `Gagal memulai: ${err.message}`)
    } finally {
      setActionLoadingId(null)
    }
  }

  const handleQuickStop = async (dep: DeploymentItem) => {
    setActionLoadingId(dep.id)
    try {
      const res = await fetch(`/api/deployments/${dep.id}/stop`, { method: 'POST' })
      if (!res.ok) {
        const err = await res.json().catch(() => ({}))
        throw new Error(err.error || `HTTP ${res.status}`)
      }
      const data = await res.json()
      handleDeploymentUpdated(data.deployment)
      showToast('success', `Aplikasi "${dep.project_name}" berhasil dihentikan.`)
    } catch (err: any) {
      showToast('error', `Gagal menghentikan: ${err.message}`)
    } finally {
      setActionLoadingId(null)
    }
  }

  const handleQuickRestart = async (dep: DeploymentItem) => {
    setActionLoadingId(dep.id)
    try {
      const res = await fetch(`/api/deployments/${dep.id}/restart`, { method: 'POST' })
      if (!res.ok) {
        const err = await res.json().catch(() => ({}))
        throw new Error(err.error || `HTTP ${res.status}`)
      }
      const data = await res.json()
      handleDeploymentUpdated(data.deployment)
      showToast('success', `Aplikasi "${dep.project_name}" berhasil di-restart.`)
    } catch (err: any) {
      showToast('error', `Gagal restart: ${err.message}`)
    } finally {
      setActionLoadingId(null)
    }
  }

  const handleDeploymentCreated = (newDep: DeploymentItem) => {
    setDeployments([newDep, ...deployments])
    setIsWizardOpen(false)
    showToast('success', `Proyek "${newDep.project_name}" berhasil dibuat!`)
    // Automatically open drawer to inspect or start
    setSelectedDeployment(newDep)
    setIsDrawerOpen(true)
  }

  const handleCreatorProjectCreated = (result: InstantiateCreatorProjectResponse) => {
    setIsCreatorWizardOpen(false)
    fetchDeployments()
    showToast(
      'success',
      `🎉 Proyek Creator "${result.project?.name || 'Baru'}" berhasil dibuat! Membuka Tasks Board...`
    )
    setActiveTab('tasks')
    onNavigate?.('/studio/tasks')
  }

  const handleDeploymentUpdated = (updated: DeploymentItem) => {
    setDeployments((prev) => prev.map((d) => (d.id === updated.id ? updated : d)))
    if (selectedDeployment?.id === updated.id) {
      setSelectedDeployment(updated)
    }
  }

  const handleDeploymentDeleted = (id: string) => {
    setDeployments((prev) => prev.filter((d) => d.id !== id))
    if (selectedDeployment?.id === id) {
      setIsDrawerOpen(false)
      setSelectedDeployment(null)
    }
    showToast('success', 'Aplikasi berhasil dihapus.')
  }

  const handleOpenDrawer = (dep: DeploymentItem) => {
    setSelectedDeployment(dep)
    setDrawerInitialTab('overview')
    setIsDrawerOpen(true)
  }

  const handleOpenDiscussionsDrawer = (dep: DeploymentItem) => {
    setSelectedDeployment(dep)
    setDrawerInitialTab('discussions')
    setIsDrawerOpen(true)
  }

  const handleOpenTasksDrawer = (dep: DeploymentItem) => {
    setSelectedDeployment(dep)
    setDrawerInitialTab('tasks')
    setIsDrawerOpen(true)
  }

  const copyUrl = (id: string, url: string) => {
    navigator.clipboard.writeText(url)
    setCopiedId(id)
    setTimeout(() => setCopiedId(null), 2000)
  }

  // Statistics calculation
  const stats = useMemo(() => {
    const total = deployments.length
    const running = deployments.filter((d) => d.status === 'running').length
    const stopped = deployments.filter((d) => d.status === 'stopped').length
    const activePorts = deployments.filter((d) => d.status === 'running' && d.internal_port > 0).length
    return { total, running, stopped, activePorts }
  }, [deployments])

  // Filtered list
  const filteredDeployments = useMemo(() => {
    return deployments.filter((dep) => {
      // Runtime filter
      if (runtimeFilter !== 'all' && dep.runtime !== runtimeFilter) {
        return false
      }
      // Status filter
      if (statusFilter !== 'all' && dep.status !== statusFilter) {
        return false
      }
      // Search query
      if (searchQuery.trim()) {
        const q = searchQuery.toLowerCase()
        const matchName = dep.project_name.toLowerCase().includes(q)
        const matchSlug = dep.slug.toLowerCase().includes(q)
        const matchId = dep.id.toLowerCase().includes(q)
        if (!matchName && !matchSlug && !matchId) {
          return false
        }
      }
      return true
    })
  }, [deployments, runtimeFilter, statusFilter, searchQuery])

  // Combined project list for Tasks Board & Discussions (deployments + projects)
  const combinedProjectsForTasks = useMemo(() => {
    const list = deployments.map((d) => ({ id: d.id, name: d.project_name }))
    const existingIds = new Set(list.map((p) => p.id))
    for (const p of projectsList) {
      if (!existingIds.has(p.id)) {
        list.push({ id: p.id, name: p.name })
        existingIds.add(p.id)
      }
    }
    return list
  }, [deployments, projectsList])

  return (
    <div className="min-h-screen bg-slate-50 dark:bg-slate-950 text-slate-900 dark:text-slate-100 flex flex-col">
      {/* Top Navigation Bar */}
      <header className="sticky top-0 z-30 border-b border-slate-200 dark:border-slate-800 bg-white/90 dark:bg-slate-900/90 backdrop-blur-md">
        <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 h-16 flex items-center justify-between gap-4">
          <div className="flex items-center gap-3">
            {onNavigate && (
              <Button
                variant="outline"
                size="sm"
                onClick={() => onNavigate('/')}
                className="h-8 px-2.5 text-xs text-slate-600 dark:text-slate-300 gap-1.5"
              >
                <ArrowLeft className="h-3.5 w-3.5" /> Beranda
              </Button>
            )}
            <div className="flex items-center gap-2.5">
              <div className="h-9 w-9 rounded-xl bg-gradient-to-br from-emerald-500 to-teal-600 text-white font-black text-lg flex items-center justify-center shadow-md shadow-emerald-500/20">
                S
              </div>
              <div>
                <div className="flex items-center gap-2">
                  <h1 className="font-extrabold text-base tracking-tight text-slate-900 dark:text-white">
                    Cuan Studio
                  </h1>
                  <Badge variant="outline" className="border-emerald-500/50 text-emerald-600 dark:text-emerald-400 text-[10px] py-0 px-1.5 font-mono">
                    Runtime S3
                  </Badge>
                  <Badge
                    variant="outline"
                    className={`text-[10px] py-0 px-2 font-mono flex items-center gap-1 ${
                      appMode === 'studio'
                        ? 'border-emerald-500/50 bg-emerald-500/10 text-emerald-600 dark:text-emerald-400'
                        : 'border-blue-500/50 bg-blue-500/10 text-blue-600 dark:text-blue-400'
                    }`}
                  >
                    <span className={`h-1.5 w-1.5 rounded-full ${appMode === 'studio' ? 'bg-emerald-500 animate-pulse' : 'bg-blue-500'}`} />
                    Mode: {appMode}
                  </Badge>
                </div>
                <p className="text-[11px] text-slate-400 hidden sm:block">Universal App Deployer & Virtual Host Manager</p>
              </div>
            </div>
          </div>

          {/* Navigation Tabs (Desktop - Filtered by activeProject.enabled_modules) */}
          <div className="hidden md:flex items-center rounded-xl bg-slate-100 dark:bg-slate-800 p-1 border border-slate-200 dark:border-slate-700/60 text-xs">
            <button
              onClick={() => {
                setActiveTab('projects')
                onNavigate?.('/studio')
              }}
              className={`flex items-center gap-1.5 py-1.5 px-3 rounded-lg font-semibold transition-all ${
                activeTab === 'projects'
                  ? 'bg-white dark:bg-slate-900 text-slate-900 dark:text-white shadow-xs'
                  : 'text-slate-500 hover:text-slate-900 dark:text-slate-400 dark:hover:text-white'
              }`}
            >
              <Layers className="h-3.5 w-3.5 text-emerald-500" />
              <span>Projects</span>
              <span className="h-4 px-1.5 rounded-full bg-slate-200 dark:bg-slate-700 text-[10px] font-mono flex items-center justify-center">
                {deployments.length}
              </span>
            </button>

            {enabledModules.includes('discussions') && (
              <button
                onClick={() => {
                  setActiveTab('discussions')
                  onNavigate?.('/studio/discussions')
                }}
                className={`flex items-center gap-1.5 py-1.5 px-3 rounded-lg font-semibold transition-all ${
                  activeTab === 'discussions'
                    ? 'bg-white dark:bg-slate-900 text-slate-900 dark:text-white shadow-xs'
                    : 'text-slate-500 hover:text-slate-900 dark:text-slate-400 dark:hover:text-white'
                }`}
              >
                <MessageSquare className="h-3.5 w-3.5 text-emerald-500" />
                <span>Discussions</span>
              </button>
            )}

            {enabledModules.includes('tasks') && (
              <button
                onClick={() => {
                  setActiveTab('tasks')
                  onNavigate?.('/studio/tasks')
                }}
                className={`flex items-center gap-1.5 py-1.5 px-3 rounded-lg font-semibold transition-all ${
                  activeTab === 'tasks'
                    ? 'bg-white dark:bg-slate-900 text-slate-900 dark:text-white shadow-xs'
                    : 'text-slate-500 hover:text-slate-900 dark:text-slate-400 dark:hover:text-white'
                }`}
              >
                <Kanban className="h-3.5 w-3.5 text-indigo-500" />
                <span>Tasks</span>
              </button>
            )}

            {enabledModules.includes('deployer') && (
              <button
                onClick={() => {
                  setActiveTab('deployer')
                  onNavigate?.('/studio/deployer')
                }}
                className={`flex items-center gap-1.5 py-1.5 px-3 rounded-lg font-semibold transition-all ${
                  activeTab === 'deployer'
                    ? 'bg-white dark:bg-slate-900 text-slate-900 dark:text-white shadow-xs'
                    : 'text-slate-500 hover:text-slate-900 dark:text-slate-400 dark:hover:text-white'
                }`}
              >
                <Rocket className="h-3.5 w-3.5 text-cyan-500" />
                <span>Deployer</span>
              </button>
            )}

            {enabledModules.includes('repositories') && (
              <button
                onClick={() => {
                  setActiveTab('repositories')
                  onNavigate?.('/studio/repositories')
                }}
                className={`flex items-center gap-1.5 py-1.5 px-3 rounded-lg font-semibold transition-all ${
                  activeTab === 'repositories'
                    ? 'bg-white dark:bg-slate-900 text-slate-900 dark:text-white shadow-xs'
                    : 'text-slate-500 hover:text-slate-900 dark:text-slate-400 dark:hover:text-white'
                }`}
              >
                <GitBranch className="h-3.5 w-3.5 text-amber-500" />
                <span>Repositories</span>
              </button>
            )}

            {enabledModules.includes('servers') && (
              <button
                onClick={() => {
                  setActiveTab('servers')
                  onNavigate?.('/studio/servers')
                }}
                className={`flex items-center gap-1.5 py-1.5 px-3 rounded-lg font-semibold transition-all ${
                  activeTab === 'servers'
                    ? 'bg-white dark:bg-slate-900 text-slate-900 dark:text-white shadow-xs'
                    : 'text-slate-500 hover:text-slate-900 dark:text-slate-400 dark:hover:text-white'
                }`}
              >
                <Server className="h-3.5 w-3.5 text-indigo-500" />
                <span>Servers</span>
                <span className="h-4 px-1.5 rounded-full bg-indigo-100 dark:bg-indigo-950 text-indigo-600 dark:text-indigo-400 text-[10px] font-mono flex items-center justify-center">
                  {servers.length}
                </span>
              </button>
            )}

            {enabledModules.includes('publishing') && (
              <button
                onClick={() => {
                  setActiveTab('publishing')
                  onNavigate?.('/studio/publishing')
                }}
                className={`flex items-center gap-1.5 py-1.5 px-3 rounded-lg font-semibold transition-all ${
                  activeTab === 'publishing'
                    ? 'bg-white dark:bg-slate-900 text-slate-900 dark:text-white shadow-xs'
                    : 'text-slate-500 hover:text-slate-900 dark:text-slate-400 dark:hover:text-white'
                }`}
              >
                <Share2 className="h-3.5 w-3.5 text-blue-500" />
                <span>Publishing</span>
              </button>
            )}

            {enabledModules.includes('support') && (
              <button
                onClick={() => {
                  setActiveTab('support')
                  onNavigate?.('/studio/support')
                }}
                className={`flex items-center gap-1.5 py-1.5 px-3 rounded-lg font-semibold transition-all ${
                  activeTab === 'support'
                    ? 'bg-white dark:bg-slate-900 text-slate-900 dark:text-white shadow-xs'
                    : 'text-slate-500 hover:text-slate-900 dark:text-slate-400 dark:hover:text-white'
                }`}
              >
                <HelpCircle className="h-3.5 w-3.5 text-purple-500" />
                <span>Support</span>
              </button>
            )}

            {/* Platform Services Tabs */}
            <button
              onClick={() => {
                setActiveTab('agents')
                onNavigate?.('/studio/agents')
              }}
              className={`flex items-center gap-1.5 py-1.5 px-3 rounded-lg font-semibold transition-all ${
                activeTab === 'agents'
                  ? 'bg-white dark:bg-slate-900 text-slate-900 dark:text-white shadow-xs'
                  : 'text-slate-500 hover:text-slate-900 dark:text-slate-400 dark:hover:text-white'
              }`}
            >
              <Bot className="h-3.5 w-3.5 text-indigo-500" />
              <span>Agents</span>
              <span className="h-4 px-1.5 rounded-full bg-indigo-100 dark:bg-indigo-950 text-indigo-600 dark:text-indigo-400 text-[10px] font-mono flex items-center justify-center">
                {agentsCount}
              </span>
            </button>

            <button
              onClick={() => {
                setActiveTab('campaigns')
                setSelectedCampaignId(null)
                onNavigate?.('/studio/campaigns')
              }}
              className={`flex items-center gap-1.5 py-1.5 px-3 rounded-lg font-semibold transition-all ${
                activeTab === 'campaigns'
                  ? 'bg-white dark:bg-slate-900 text-slate-900 dark:text-white shadow-xs'
                  : 'text-slate-500 hover:text-slate-900 dark:text-slate-400 dark:hover:text-white'
              }`}
            >
              <Rocket className="h-3.5 w-3.5 text-emerald-500" />
              <span>Campaigns</span>
              <span className="h-4 px-1.5 rounded-full bg-emerald-100 dark:bg-emerald-950 text-emerald-600 dark:text-emerald-400 text-[10px] font-mono flex items-center justify-center">
                Batch
              </span>
            </button>

            <button
              onClick={() => {
                setActiveTab('domains')
                onNavigate?.('/studio/domains')
              }}
              className={`flex items-center gap-1.5 py-1.5 px-3 rounded-lg font-semibold transition-all ${
                activeTab === 'domains'
                  ? 'bg-white dark:bg-slate-900 text-slate-900 dark:text-white shadow-xs'
                  : 'text-slate-500 hover:text-slate-900 dark:text-slate-400 dark:hover:text-white'
              }`}
            >
              <Globe className="h-3.5 w-3.5 text-emerald-500" />
              <span>Domains</span>
              <span className="h-4 px-1.5 rounded-full bg-emerald-100 dark:bg-emerald-950 text-emerald-600 dark:text-emerald-400 text-[10px] font-mono flex items-center justify-center">
                {domains.length}
              </span>
            </button>

            <button
              onClick={() => {
                setActiveTab('observability')
                onNavigate?.('/studio/observability')
              }}
              className={`flex items-center gap-1.5 py-1.5 px-3 rounded-lg font-semibold transition-all ${
                activeTab === 'observability'
                  ? 'bg-white dark:bg-slate-900 text-slate-900 dark:text-white shadow-xs'
                  : 'text-slate-500 hover:text-slate-900 dark:text-slate-400 dark:hover:text-white'
              }`}
            >
              <Activity className="h-3.5 w-3.5 text-emerald-500" />
              <span>Observability</span>
              <span className="h-4 px-1.5 rounded-full bg-emerald-100 dark:bg-emerald-950 text-emerald-600 dark:text-emerald-400 text-[10px] font-mono flex items-center justify-center">
                Live
              </span>
            </button>
          </div>

          <div className="flex items-center gap-2 sm:gap-3">
            {onToggleMode ? (
              <Button
                variant="outline"
                size="sm"
                onClick={() => onToggleMode(appMode === 'studio' ? 'marketing' : 'studio')}
                className="h-9 text-xs flex items-center gap-1.5 border-slate-300 dark:border-slate-700 hover:bg-slate-100 dark:hover:bg-slate-800"
                title="Ganti mode aplikasi"
              >
                <Layers className="h-3.5 w-3.5 text-emerald-500" />
                <span className="hidden sm:inline">Switch to {appMode === 'studio' ? 'Marketing' : 'Studio'}</span>
              </Button>
            ) : onNavigate ? (
              <Button
                variant="outline"
                size="sm"
                onClick={() => onNavigate('/marketing')}
                className="h-9 text-xs flex items-center gap-1.5 border-slate-300 dark:border-slate-700"
              >
                <Layers className="h-3.5 w-3.5 text-emerald-500" />
                <span className="hidden sm:inline">Landing Page</span>
              </Button>
            ) : null}

            <Button
              variant="outline"
              size="sm"
              onClick={() => {
                fetchDeployments()
                fetchServers()
                fetchProjectsList()
              }}
              className="h-9 text-xs flex items-center gap-1.5"
            >
              <RefreshCw className={`h-3.5 w-3.5 ${loading ? 'animate-spin' : ''}`} />
              <span className="hidden sm:inline">Refresh</span>
            </Button>

            <Button
              onClick={() => setIsCreatorWizardOpen(true)}
              className="h-9 bg-gradient-to-r from-emerald-600 to-teal-600 hover:from-emerald-500 hover:to-teal-500 text-white font-bold text-xs shadow-md shadow-emerald-600/20 flex items-center gap-1.5"
              title="Buat Proyek Creator Baru dari 4 Blueprint (Blog, YouTube, Olshop, Affiliate)"
            >
              <Sparkles className="h-3.5 w-3.5" />
              <span>Creator Wizard</span>
            </Button>

            <Button
              onClick={() => setIsWizardOpen(true)}
              className="h-9 bg-slate-800 hover:bg-slate-700 text-white font-semibold text-xs border border-slate-700 flex items-center gap-1.5"
            >
              <Plus className="h-4 w-4" />
              <span>Deploy Project</span>
            </Button>
          </div>
        </div>
      </header>

      {/* Dynamic Workspace Switcher & Module Bar (S11.4 DoD 1) */}
      <div className="border-b border-slate-200 dark:border-slate-800 bg-white dark:bg-slate-900 py-2.5 px-4 sm:px-6 lg:px-8 shadow-xs">
        <div className="max-w-7xl mx-auto flex flex-wrap items-center justify-between gap-3 text-xs">
          {/* Active Workspace Switcher */}
          <div className="flex items-center gap-2.5">
            <span className="text-xs font-semibold text-slate-500 dark:text-slate-400">Workspace Aktif:</span>
            <div className="relative">
              <select
                value={activeProjectId}
                onChange={(e) => handleSelectProject(e.target.value)}
                className="h-8 pl-3 pr-8 text-xs font-bold bg-slate-50 dark:bg-slate-800 border border-slate-300 dark:border-slate-700 rounded-lg text-slate-900 dark:text-white cursor-pointer hover:border-emerald-500 focus:outline-none focus:ring-1 focus:ring-emerald-500 transition-colors appearance-none shadow-xs"
              >
                <optgroup label="Preset Workspaces">
                  <option value="saas-dojo">⚡ SaaS / Dojo Workspace (Full Stack)</option>
                  <option value="creator-hub">🎨 Creator Hub Workspace (Publishing)</option>
                  <option value="blank-workspace">📄 Blank Workspace (Lean)</option>
                </optgroup>
                {projectsList.filter((p) => !['saas-dojo', 'creator-hub', 'blank-workspace'].includes(p.id)).length > 0 && (
                  <optgroup label="Proyek Terhubung / Hasil Import">
                    {projectsList
                      .filter((p) => !['saas-dojo', 'creator-hub', 'blank-workspace'].includes(p.id))
                      .map((p) => (
                        <option key={p.id} value={p.id}>
                          📦 {p.name}
                        </option>
                      ))}
                  </optgroup>
                )}
              </select>
              <ChevronDown className="h-3 w-3 text-slate-400 absolute right-2.5 top-1/2 -translate-y-1/2 pointer-events-none" />
            </div>

            {/* Active Modules Badges */}
            <div className="hidden sm:flex items-center gap-1.5 ml-2">
              <span className="text-[11px] text-slate-400">Modul Aktif:</span>
              <div className="flex items-center gap-1">
                {enabledModules.map((m) => (
                  <Badge
                    key={m}
                    variant="outline"
                    className="text-[10px] font-mono uppercase bg-emerald-500/10 text-emerald-600 dark:text-emerald-400 border-emerald-500/20 py-0 px-1.5"
                  >
                    {m}
                  </Badge>
                ))}
              </div>
            </div>
          </div>

          {/* Quick Actions: + Add Modules & + New Project from ZIP */}
          <div className="flex items-center gap-2">
            <Button
              variant="outline"
              size="sm"
              onClick={() => setIsModuleToggleModalOpen(true)}
              className="h-8 px-2.5 text-xs text-slate-700 dark:text-slate-300 border-slate-300 dark:border-slate-700 hover:bg-slate-100 dark:hover:bg-slate-800 gap-1.5 font-semibold"
              title="Atur modul aktif pada project ini"
            >
              <Settings2 className="h-3.5 w-3.5 text-emerald-500" />
              <span>+ Add Modules</span>
            </Button>

            <Button
              variant="outline"
              size="sm"
              onClick={() => setIsZipWizardOpen(true)}
              className="h-8 px-2.5 text-xs text-teal-700 dark:text-teal-300 border-teal-500/40 bg-teal-500/10 hover:bg-teal-500/20 gap-1.5 font-bold shadow-xs"
              title="Upload Script ZIP & AI Inspection Wizard"
            >
              <UploadCloud className="h-3.5 w-3.5 text-teal-500" />
              <span>+ New Project from ZIP</span>
            </Button>
          </div>
        </div>
      </div>

      {/* Main Content Container */}
      <main className="flex-1 max-w-7xl w-full mx-auto px-4 sm:px-6 lg:px-8 py-8 space-y-6">
        {/* Toast Notification Banner */}
        {notification && (
          <div
            className={`p-4 rounded-xl flex items-center justify-between text-xs font-semibold shadow-md animate-in slide-in-from-top duration-200 ${
              notification.type === 'success'
                ? 'bg-emerald-500 text-white shadow-emerald-500/20'
                : 'bg-red-500 text-white shadow-red-500/20'
            }`}
          >
            <span>{notification.message}</span>
            <button onClick={() => setNotification(null)} className="opacity-80 hover:opacity-100">
              <ExternalLink className="h-3.5 w-3.5" />
            </button>
          </div>
        )}

        {/* Mobile Navigation Tab Bar - Dynamic according to enabledModules */}
        <div className="flex md:hidden items-center rounded-xl bg-slate-100 dark:bg-slate-800 p-1 border border-slate-200 dark:border-slate-700/60 text-xs overflow-x-auto gap-1">
          <button
            onClick={() => {
              setActiveTab('projects')
              onNavigate?.('/studio')
            }}
            className={`flex-none flex items-center justify-center gap-1.5 py-1.5 px-2.5 rounded-lg font-semibold transition-all ${
              activeTab === 'projects'
                ? 'bg-white dark:bg-slate-900 text-slate-900 dark:text-white shadow-xs'
                : 'text-slate-500 hover:text-slate-900 dark:text-slate-400'
            }`}
          >
            <Layers className="h-3.5 w-3.5 text-emerald-500" />
            <span>Projects ({deployments.length})</span>
          </button>

          {enabledModules.includes('discussions') && (
            <button
              onClick={() => {
                setActiveTab('discussions')
                onNavigate?.('/studio/discussions')
              }}
              className={`flex-none flex items-center justify-center gap-1.5 py-1.5 px-2.5 rounded-lg font-semibold transition-all ${
                activeTab === 'discussions'
                  ? 'bg-white dark:bg-slate-900 text-slate-900 dark:text-white shadow-xs'
                  : 'text-slate-500 hover:text-slate-900 dark:text-slate-400'
              }`}
            >
              <MessageSquare className="h-3.5 w-3.5 text-emerald-500" />
              <span>Discussions</span>
            </button>
          )}

          {enabledModules.includes('tasks') && (
            <button
              onClick={() => {
                setActiveTab('tasks')
                onNavigate?.('/studio/tasks')
              }}
              className={`flex-none flex items-center justify-center gap-1.5 py-1.5 px-2.5 rounded-lg font-semibold transition-all ${
                activeTab === 'tasks'
                  ? 'bg-white dark:bg-slate-900 text-slate-900 dark:text-white shadow-xs'
                  : 'text-slate-500 hover:text-slate-900 dark:text-slate-400'
              }`}
            >
              <Kanban className="h-3.5 w-3.5 text-indigo-500" />
              <span>Tasks</span>
            </button>
          )}

          {enabledModules.includes('deployer') && (
            <button
              onClick={() => {
                setActiveTab('deployer')
                onNavigate?.('/studio/deployer')
              }}
              className={`flex-none flex items-center justify-center gap-1.5 py-1.5 px-2.5 rounded-lg font-semibold transition-all ${
                activeTab === 'deployer'
                  ? 'bg-white dark:bg-slate-900 text-slate-900 dark:text-white shadow-xs'
                  : 'text-slate-500 hover:text-slate-900 dark:text-slate-400'
              }`}
            >
              <Rocket className="h-3.5 w-3.5 text-cyan-500" />
              <span>Deployer</span>
            </button>
          )}

          {enabledModules.includes('repositories') && (
            <button
              onClick={() => {
                setActiveTab('repositories')
                onNavigate?.('/studio/repositories')
              }}
              className={`flex-none flex items-center justify-center gap-1.5 py-1.5 px-2.5 rounded-lg font-semibold transition-all ${
                activeTab === 'repositories'
                  ? 'bg-white dark:bg-slate-900 text-slate-900 dark:text-white shadow-xs'
                  : 'text-slate-500 hover:text-slate-900 dark:text-slate-400'
              }`}
            >
              <GitBranch className="h-3.5 w-3.5 text-amber-500" />
              <span>Repos</span>
            </button>
          )}

          {enabledModules.includes('servers') && (
            <button
              onClick={() => {
                setActiveTab('servers')
                onNavigate?.('/studio/servers')
              }}
              className={`flex-none flex items-center justify-center gap-1.5 py-1.5 px-2.5 rounded-lg font-semibold transition-all ${
                activeTab === 'servers'
                  ? 'bg-white dark:bg-slate-900 text-slate-900 dark:text-white shadow-xs'
                  : 'text-slate-500 hover:text-slate-900 dark:text-slate-400'
              }`}
            >
              <Server className="h-3.5 w-3.5 text-indigo-500" />
              <span>Servers ({servers.length})</span>
            </button>
          )}

          {enabledModules.includes('publishing') && (
            <button
              onClick={() => {
                setActiveTab('publishing')
                onNavigate?.('/studio/publishing')
              }}
              className={`flex-none flex items-center justify-center gap-1.5 py-1.5 px-2.5 rounded-lg font-semibold transition-all ${
                activeTab === 'publishing'
                  ? 'bg-white dark:bg-slate-900 text-slate-900 dark:text-white shadow-xs'
                  : 'text-slate-500 hover:text-slate-900 dark:text-slate-400'
              }`}
            >
              <Share2 className="h-3.5 w-3.5 text-blue-500" />
              <span>Publishing</span>
            </button>
          )}

          {enabledModules.includes('support') && (
            <button
              onClick={() => {
                setActiveTab('support')
                onNavigate?.('/studio/support')
              }}
              className={`flex-none flex items-center justify-center gap-1.5 py-1.5 px-2.5 rounded-lg font-semibold transition-all ${
                activeTab === 'support'
                  ? 'bg-white dark:bg-slate-900 text-slate-900 dark:text-white shadow-xs'
                  : 'text-slate-500 hover:text-slate-900 dark:text-slate-400'
              }`}
            >
              <HelpCircle className="h-3.5 w-3.5 text-purple-500" />
              <span>Support</span>
            </button>
          )}

          <button
            onClick={() => {
              setActiveTab('agents')
              onNavigate?.('/studio/agents')
            }}
            className={`flex-none flex items-center justify-center gap-1.5 py-1.5 px-2.5 rounded-lg font-semibold transition-all ${
              activeTab === 'agents'
                ? 'bg-white dark:bg-slate-900 text-slate-900 dark:text-white shadow-xs'
                : 'text-slate-500 hover:text-slate-900 dark:text-slate-400'
            }`}
          >
            <Bot className="h-3.5 w-3.5 text-indigo-500" />
            <span>Agents ({agentsCount})</span>
          </button>

          <button
            onClick={() => {
              setActiveTab('campaigns')
              setSelectedCampaignId(null)
              onNavigate?.('/studio/campaigns')
            }}
            className={`flex-none flex items-center justify-center gap-1.5 py-1.5 px-2.5 rounded-lg font-semibold transition-all ${
              activeTab === 'campaigns'
                ? 'bg-white dark:bg-slate-900 text-slate-900 dark:text-white shadow-xs'
                : 'text-slate-500 hover:text-slate-900 dark:text-slate-400'
            }`}
          >
            <Rocket className="h-3.5 w-3.5 text-emerald-500" />
            <span>Campaigns</span>
          </button>

          <button
            onClick={() => {
              setActiveTab('domains')
              onNavigate?.('/studio/domains')
            }}
            className={`flex-1 flex items-center justify-center gap-1.5 py-2 px-2 rounded-lg font-semibold transition-all ${
              activeTab === 'domains'
                ? 'bg-white dark:bg-slate-900 text-slate-900 dark:text-white shadow-xs'
                : 'text-slate-500 hover:text-slate-900 dark:text-slate-400'
            }`}
          >
            <Globe className="h-3.5 w-3.5 text-emerald-500" />
            <span>Domains ({domains.length})</span>
          </button>

          <button
            onClick={() => {
              setActiveTab('servers')
              onNavigate?.('/studio/servers')
            }}
            className={`flex-1 flex items-center justify-center gap-1.5 py-2 px-2 rounded-lg font-semibold transition-all ${
              activeTab === 'servers'
                ? 'bg-white dark:bg-slate-900 text-slate-900 dark:text-white shadow-xs'
                : 'text-slate-500 hover:text-slate-900 dark:text-slate-400'
            }`}
          >
            <Server className="h-3.5 w-3.5 text-indigo-500" />
            <span>Servers ({servers.length})</span>
          </button>

          <button
            onClick={() => {
              setActiveTab('observability')
              onNavigate?.('/studio/observability')
            }}
            className={`flex-1 flex items-center justify-center gap-1.5 py-2 px-2 rounded-lg font-semibold transition-all ${
              activeTab === 'observability'
                ? 'bg-white dark:bg-slate-900 text-slate-900 dark:text-white shadow-xs'
                : 'text-slate-500 hover:text-slate-900 dark:text-slate-400'
            }`}
          >
            <Activity className="h-3.5 w-3.5 text-emerald-500" />
            <span>Observability</span>
          </button>
        </div>

        {/* Studio Mode Active Banner (APP_MODE=studio) */}
        {(isStudioModeActive || appMode === 'studio') && (
          <div className="p-4 sm:p-5 rounded-2xl bg-gradient-to-r from-emerald-950/60 via-slate-900 to-slate-900 border border-emerald-500/30 text-white shadow-lg shadow-emerald-950/20 flex flex-col sm:flex-row items-start sm:items-center justify-between gap-4 animate-in fade-in duration-300">
            <div className="flex items-start gap-3.5">
              <div className="h-10 w-10 rounded-xl bg-emerald-500/20 border border-emerald-500/40 text-emerald-400 flex items-center justify-center shrink-0 mt-0.5">
                <Sparkles className="h-5 w-5" />
              </div>
              <div className="space-y-1">
                <div className="flex items-center gap-2 flex-wrap">
                  <h2 className="text-sm sm:text-base font-bold text-white tracking-tight">
                    Studio Mode Active - Welcome to your self-hosted digital project studio
                  </h2>
                  <Badge className="bg-emerald-500/20 text-emerald-300 border-emerald-500/40 text-[10px] py-0 px-1.5 font-mono">
                    APP_MODE=studio
                  </Badge>
                </div>
                <p className="text-xs text-slate-300">
                  Root <code className="text-emerald-400 bg-slate-950/60 px-1.5 py-0.5 rounded font-mono">/</code> langsung membuka Studio Dashboard. Kelola intake script ZIP/Git, dynamic port, dan virtual host subdomain otomatis Anda di sini.
                </p>
              </div>
            </div>
            <div className="flex items-center gap-2 self-end sm:self-center shrink-0">
              {onNavigate && (
                <Button
                  variant="outline"
                  size="sm"
                  onClick={() => onNavigate('/marketing')}
                  className="h-8 text-xs border-slate-700 bg-slate-800/80 hover:bg-slate-700 text-slate-200"
                >
                  Lihat Landing Page (/marketing)
                </Button>
              )}
            </div>
          </div>
        )}

        {/* Tab View Switching: Campaigns vs Discussions vs Tasks vs Observability Hub vs Domains Hub vs Servers Hub vs Projects Dashboard */}
        {activeTab === 'campaigns' ? (
          selectedCampaignId ? (
            <CampaignDetailView
              campaignId={selectedCampaignId}
              onBack={() => {
                setSelectedCampaignId(null)
                onNavigate?.('/studio/campaigns')
              }}
              onNavigate={onNavigate}
            />
          ) : (
            <CampaignDashboardView
              onSelectCampaign={(id) => {
                setSelectedCampaignId(id)
                onNavigate?.(`/studio/campaigns/${id}`)
              }}
              onOpenCreateModal={() => setIsBatchModalOpen(true)}
              onNavigate={onNavigate}
            />
          )
        ) : activeTab === 'discussions' ? (
          <ProjectDiscussionsView
            projects={combinedProjectsForTasks}
            onOpenTaskBoard={() => {
              setActiveTab('tasks')
              onNavigate?.('/studio/tasks')
            }}
          />
        ) : activeTab === 'deployer' ? (
          <ScriptDeployerView
            projectId={activeProjectId}
            projects={projectsList}
            servers={servers}
            baseDomain={baseDomain}
            onToast={(type, message) => showToast(type, message)}
            onProjectChange={(id) => handleSelectProject(id)}
          />
        ) : activeTab === 'tasks' ? (
          <ProjectTasksBoard
            projects={combinedProjectsForTasks}
            onOpenDiscussions={() => {
              setActiveTab('discussions')
              onNavigate?.('/studio/discussions')
            }}
          />
        ) : activeTab === 'publishing' ? (
          <div className="space-y-6 animate-in fade-in duration-200">
            <div className="p-6 rounded-2xl bg-white dark:bg-slate-900 border border-slate-200 dark:border-slate-800 shadow-xs space-y-5">
              <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-4 border-b border-slate-100 dark:border-slate-800 pb-5">
                <div className="flex items-center gap-3">
                  <div className="h-11 w-11 rounded-2xl bg-blue-500/10 text-blue-500 border border-blue-500/20 flex items-center justify-center">
                    <Share2 className="h-6 w-6" />
                  </div>
                  <div>
                    <div className="flex items-center gap-2">
                      <h2 className="text-base font-bold text-slate-900 dark:text-white">
                        WordPress CMS Publishing Center
                      </h2>
                      <Badge className="bg-blue-500/10 text-blue-600 dark:text-blue-400 border border-blue-500/20 text-[10px] font-mono">
                        REST API v2
                      </Badge>
                    </div>
                    <p className="text-xs text-slate-500 dark:text-slate-400 mt-0.5">
                      Publikasikan konten naskah, deliverable blog, dan artikel secara langsung ke WordPress via REST API berenkripsi AES-256-GCM.
                    </p>
                  </div>
                </div>

                <div className="flex items-center gap-2">
                  <Button
                    size="sm"
                    onClick={() => {
                      setActiveTab('tasks')
                      onNavigate?.('/studio/tasks')
                    }}
                    className="bg-blue-600 hover:bg-blue-500 text-white text-xs font-semibold gap-1.5 shadow-md shadow-blue-600/20"
                  >
                    <Kanban className="h-3.5 w-3.5" />
                    <span>Pilih Deliverable di Tasks Board</span>
                  </Button>
                </div>
              </div>

              {/* Status Integration Cards */}
              <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
                <div className="p-4 rounded-xl bg-slate-50 dark:bg-slate-950/60 border border-slate-200 dark:border-slate-800">
                  <span className="text-[11px] font-medium text-slate-500 dark:text-slate-400 uppercase tracking-wider block">
                    Workspace Aktif
                  </span>
                  <p className="text-sm font-bold text-slate-900 dark:text-white mt-1">
                    {activeProject.name}
                  </p>
                  <span className="text-[10px] text-slate-400 font-mono">ID: {activeProject.id}</span>
                </div>

                <div className="p-4 rounded-xl bg-emerald-500/5 border border-emerald-500/20">
                  <span className="text-[11px] font-medium text-emerald-600 dark:text-emerald-400 uppercase tracking-wider block">
                    Enkripsi Kredensial Application Password
                  </span>
                  <p className="text-sm font-bold text-emerald-700 dark:text-emerald-300 mt-1 flex items-center gap-1.5">
                    <span className="h-2 w-2 rounded-full bg-emerald-500 animate-pulse" />
                    AES-256-GCM Active
                  </p>
                  <span className="text-[10px] text-slate-400">Zero plaintext leakage terjamin</span>
                </div>

                <div className="p-4 rounded-xl bg-slate-50 dark:bg-slate-950/60 border border-slate-200 dark:border-slate-800">
                  <span className="text-[11px] font-medium text-slate-500 dark:text-slate-400 uppercase tracking-wider block">
                    Target CMS Integrasi
                  </span>
                  <p className="text-sm font-bold text-slate-900 dark:text-white mt-1">
                    WordPress REST API
                  </p>
                  <span className="text-[10px] text-slate-400">Endpoint: /wp-json/wp/v2/posts</span>
                </div>
              </div>

              <div className="p-4 rounded-xl bg-blue-500/5 border border-blue-500/20 text-xs text-blue-900 dark:text-blue-300 flex flex-col sm:flex-row items-start sm:items-center justify-between gap-3">
                <div>
                  <strong>Alur 1-Click Publishing Cuan:</strong>
                  <p className="text-[11px] text-blue-800/80 dark:text-blue-300/80 mt-0.5">
                    Buka Task Detail Drawer di Kanban Tasks Board, pilih tab <strong>"Publish to WordPress"</strong> untuk melakukan pengujian kredensial atau menerbitkan naskah/deliverable dalam status <code>draft</code> atau <code>publish</code>.
                  </p>
                </div>
                <Button
                  variant="outline"
                  size="sm"
                  onClick={() => {
                    setActiveTab('tasks')
                    onNavigate?.('/studio/tasks')
                  }}
                  className="border-blue-500/30 text-blue-600 dark:text-blue-300 text-xs h-8 shrink-0 hover:bg-blue-500/10"
                >
                  Buka Tasks Board
                </Button>
              </div>
            </div>
          </div>
        ) : activeTab === 'repositories' ? (
          <div className="space-y-6 animate-in fade-in duration-200">
            <div className="p-6 rounded-2xl bg-white dark:bg-slate-900 border border-slate-200 dark:border-slate-800 shadow-xs space-y-5">
              <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-4 border-b border-slate-100 dark:border-slate-800 pb-5">
                <div className="flex items-center gap-3">
                  <div className="h-11 w-11 rounded-2xl bg-amber-500/10 text-amber-500 border border-amber-500/20 flex items-center justify-center">
                    <GitBranch className="h-6 w-6" />
                  </div>
                  <div>
                    <h2 className="text-base font-bold text-slate-900 dark:text-white">
                      Connected Git Repositories & Branches
                    </h2>
                    <p className="text-xs text-slate-500 dark:text-slate-400 mt-0.5">
                      Kelola repository kode sumber, cabang Git aktif, dan sinkronisasi runner pada workspace {activeProject.name}.
                    </p>
                  </div>
                </div>
              </div>

              <div className="p-4 rounded-xl bg-slate-50 dark:bg-slate-950/60 border border-slate-200 dark:border-slate-800 flex items-center justify-between">
                <div>
                  <h4 className="text-xs font-bold text-slate-900 dark:text-white">
                    Repository Workspace: {activeProject.name}
                  </h4>
                  <p className="text-[11px] text-slate-400 mt-0.5 font-mono">
                    {activeProject.repo_url || 'Repo default terhubung via Cuan Git Runner'}
                  </p>
                </div>
                <Badge className="bg-emerald-500/10 text-emerald-600 dark:text-emerald-400 border border-emerald-500/20 text-[10px] font-mono">
                  Branch: {activeProject.default_branch || 'main'}
                </Badge>
              </div>
            </div>
          </div>
        ) : activeTab === 'support' ? (
          <div className="space-y-6 animate-in fade-in duration-200">
            <div className="p-6 rounded-2xl bg-white dark:bg-slate-900 border border-slate-200 dark:border-slate-800 shadow-xs space-y-5">
              <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-4 border-b border-slate-100 dark:border-slate-800 pb-5">
                <div className="flex items-center gap-3">
                  <div className="h-11 w-11 rounded-2xl bg-purple-500/10 text-purple-500 border border-purple-500/20 flex items-center justify-center">
                    <HelpCircle className="h-6 w-6" />
                  </div>
                  <div>
                    <h2 className="text-base font-bold text-slate-900 dark:text-white">
                      Support & Customer License Management Hub
                    </h2>
                    <p className="text-xs text-slate-500 dark:text-slate-400 mt-0.5">
                      Pantau tiket support pelanggan, investigasi kendala lisensi/aktivasi AGC Dracin, dan audit log aktivasi.
                    </p>
                  </div>
                </div>
              </div>

              <div className="p-4 rounded-xl bg-slate-50 dark:bg-slate-950/60 border border-slate-200 dark:border-slate-800">
                <h4 className="text-xs font-bold text-slate-900 dark:text-white">
                  SLA Support & License Status: Aktif
                </h4>
                <p className="text-[11px] text-slate-400 mt-0.5">
                  Modul support aktif untuk workspace {activeProject.name}. Tiket masuk otomatis disinkronkan ke room diskusi studio.
                </p>
              </div>
            </div>
          </div>
        ) : activeTab === 'agents' ? (
          <AgentsHubView onNavigate={onNavigate} />
        ) : activeTab === 'observability' ? (
          <ObservabilityView
            onNavigate={onNavigate}
            baseDomain={baseDomain}
            deployments={deployments}
            onOpenDrawer={handleOpenDrawer}
          />
        ) : activeTab === 'domains' ? (
          <DomainsView
            onNavigate={onNavigate}
            baseDomain={baseDomain}
            deployments={deployments}
            onOpenDeployWizard={() => setIsWizardOpen(true)}
          />
        ) : activeTab === 'servers' ? (
          <ServersView onNavigate={onNavigate} baseDomain={baseDomain} />
        ) : (
          <>
            {error && (
              <Alert variant="destructive">
                <AlertCircle className="h-4 w-4" />
                <AlertTitle>Kesalahan</AlertTitle>
                <AlertDescription>{error}</AlertDescription>
              </Alert>
            )}

            {/* Creator Blueprint Fast Launcher Banner (S10.4) */}
            <div className="p-4 sm:p-5 rounded-2xl bg-gradient-to-r from-emerald-950/40 via-slate-900 to-slate-900 border border-emerald-500/30 flex flex-col sm:flex-row items-start sm:items-center justify-between gap-4 shadow-sm">
              <div className="flex items-start gap-3.5">
                <div className="h-10 w-10 rounded-xl bg-emerald-500/20 text-emerald-400 border border-emerald-500/30 flex items-center justify-center shrink-0 mt-0.5">
                  <Sparkles className="h-5 w-5" />
                </div>
                <div className="space-y-0.5">
                  <div className="flex items-center gap-2">
                    <h3 className="text-sm font-bold text-white tracking-tight">
                      Creator Project Blueprints (Blog, YouTube, Olshop, Affiliate)
                    </h3>
                    <Badge className="bg-emerald-500/20 text-emerald-300 border-emerald-500/30 text-[10px] font-mono py-0 px-1.5">
                      Instant Pipeline
                    </Badge>
                  </div>
                  <p className="text-xs text-slate-300">
                    Mulai proyek kreator instan dengan alur kerja nyata: rooms diskusi, alur pipeline task, dan AI Agent persona terpasang.
                  </p>
                </div>
              </div>
              <div className="flex items-center gap-2 self-end sm:self-center shrink-0">
                <Button
                  onClick={() => setIsZipWizardOpen(true)}
                  size="sm"
                  variant="outline"
                  className="border-teal-500/40 bg-teal-500/10 hover:bg-teal-500/20 text-teal-300 font-bold text-xs gap-1.5"
                >
                  <UploadCloud className="h-3.5 w-3.5" />
                  <span>Upload Script ZIP</span>
                </Button>
                <Button
                  onClick={() => setIsCreatorWizardOpen(true)}
                  size="sm"
                  className="bg-emerald-600 hover:bg-emerald-500 text-white font-bold text-xs gap-1.5 shadow-md shadow-emerald-600/20"
                >
                  <Sparkles className="h-3.5 w-3.5" />
                  <span>Buka Creator Wizard</span>
                </Button>
              </div>
            </div>

        {/* 1. Stat Summary Cards */}
        <div className="grid grid-cols-2 lg:grid-cols-4 gap-4">
          <Card className="p-4 bg-white dark:bg-slate-900 border-slate-200 dark:border-slate-800">
            <div className="flex items-center justify-between">
              <div>
                <p className="text-xs font-medium text-slate-500 dark:text-slate-400">Total Proyek</p>
                <p className="text-2xl font-bold tracking-tight text-slate-900 dark:text-white mt-0.5">
                  {stats.total}
                </p>
              </div>
              <div className="h-10 w-10 rounded-xl bg-slate-100 dark:bg-slate-800 text-slate-600 dark:text-slate-300 flex items-center justify-center">
                <Layers className="h-5 w-5" />
              </div>
            </div>
          </Card>

          <Card className="p-4 bg-white dark:bg-slate-900 border-slate-200 dark:border-slate-800">
            <div className="flex items-center justify-between">
              <div>
                <p className="text-xs font-medium text-slate-500 dark:text-slate-400">Aktif Berjalan</p>
                <div className="flex items-center gap-2 mt-0.5">
                  <p className="text-2xl font-bold tracking-tight text-emerald-600 dark:text-emerald-400">
                    {stats.running}
                  </p>
                  {stats.running > 0 && <span className="h-2.5 w-2.5 rounded-full bg-emerald-500 animate-pulse" />}
                </div>
              </div>
              <div className="h-10 w-10 rounded-xl bg-emerald-100 dark:bg-emerald-950/60 text-emerald-600 dark:text-emerald-400 flex items-center justify-center">
                <Play className="h-5 w-5 fill-current" />
              </div>
            </div>
          </Card>

          <Card className="p-4 bg-white dark:bg-slate-900 border-slate-200 dark:border-slate-800">
            <div className="flex items-center justify-between">
              <div>
                <p className="text-xs font-medium text-slate-500 dark:text-slate-400">Dihentikan (Stopped)</p>
                <p className="text-2xl font-bold tracking-tight text-slate-600 dark:text-slate-400 mt-0.5">
                  {stats.stopped}
                </p>
              </div>
              <div className="h-10 w-10 rounded-xl bg-slate-100 dark:bg-slate-800 text-slate-400 flex items-center justify-center">
                <Square className="h-5 w-5" />
              </div>
            </div>
          </Card>

          <Card className="p-4 bg-white dark:bg-slate-900 border-slate-200 dark:border-slate-800">
            <div className="flex items-center justify-between">
              <div>
                <p className="text-xs font-medium text-slate-500 dark:text-slate-400">Port Terpakai</p>
                <p className="text-2xl font-bold tracking-tight text-blue-600 dark:text-blue-400 mt-0.5">
                  {stats.activePorts} <span className="text-xs font-normal text-slate-400">/ 1000 pool</span>
                </p>
              </div>
              <div className="h-10 w-10 rounded-xl bg-blue-100 dark:bg-blue-950/60 text-blue-600 dark:text-blue-400 flex items-center justify-center">
                <Server className="h-5 w-5" />
              </div>
            </div>
          </Card>
        </div>

        {/* 2. Search & Filter Bar */}
        <div className="flex flex-col sm:flex-row gap-3 items-center justify-between bg-white dark:bg-slate-900 p-3 rounded-2xl border border-slate-200 dark:border-slate-800 shadow-xs">
          <div className="relative w-full sm:w-72">
            <Search className="absolute left-3 top-1/2 -translate-y-1/2 h-4 w-4 text-slate-400" />
            <Input
              type="text"
              placeholder="Cari nama atau slug..."
              value={searchQuery}
              onChange={(e) => setSearchQuery(e.target.value)}
              className="pl-9 h-9 text-xs bg-slate-50 dark:bg-slate-800/50"
            />
          </div>

          <div className="flex flex-wrap items-center gap-2 w-full sm:w-auto">
            {/* Runtime Filter */}
            <div className="flex rounded-lg bg-slate-100 dark:bg-slate-800 p-0.5 text-xs">
              {(['all', 'static', 'php', 'nodejs'] as const).map((rt) => (
                <button
                  key={rt}
                  onClick={() => setRuntimeFilter(rt)}
                  className={`px-2.5 py-1 rounded-md font-semibold capitalize transition-all ${
                    runtimeFilter === rt
                      ? 'bg-white dark:bg-slate-900 text-slate-900 dark:text-white shadow-xs'
                      : 'text-slate-500 hover:text-slate-900 dark:text-slate-400'
                  }`}
                >
                  {rt === 'all' ? 'All Stacks' : rt}
                </button>
              ))}
            </div>

            {/* Status Filter */}
            <div className="flex rounded-lg bg-slate-100 dark:bg-slate-800 p-0.5 text-xs">
              {(['all', 'running', 'stopped'] as const).map((st) => (
                <button
                  key={st}
                  onClick={() => setStatusFilter(st)}
                  className={`px-2.5 py-1 rounded-md font-semibold capitalize transition-all ${
                    statusFilter === st
                      ? 'bg-white dark:bg-slate-900 text-slate-900 dark:text-white shadow-xs'
                      : 'text-slate-500 hover:text-slate-900 dark:text-slate-400'
                  }`}
                >
                  {st === 'all' ? 'All Status' : st}
                </button>
              ))}
            </div>
          </div>
        </div>

        {/* 3. Projects Grid / Cards View */}
        {loading && deployments.length === 0 ? (
          <div className="h-64 flex flex-col items-center justify-center gap-3 bg-white dark:bg-slate-900 rounded-2xl border border-slate-200 dark:border-slate-800">
            <RefreshCw className="h-8 w-8 text-emerald-500 animate-spin" />
            <p className="text-sm text-slate-500">Memuat daftar aplikasi Studio...</p>
          </div>
        ) : deployments.length === 0 ? (
          /* Zero-Friction Welcome Banner Onboarding (empty state) */
          <div className="rounded-3xl border border-dashed border-emerald-500/40 bg-gradient-to-b from-emerald-950/20 via-slate-900/60 to-slate-900/80 p-8 sm:p-12 text-center space-y-6 shadow-xl">
            <div className="relative inline-block">
              <div className="h-20 w-20 rounded-3xl bg-gradient-to-tr from-emerald-500 to-teal-400 text-slate-950 flex items-center justify-center mx-auto shadow-xl shadow-emerald-500/25">
                <UploadCloud className="h-10 w-10 stroke-[2.2]" />
              </div>
              <div className="absolute -top-1 -right-1 h-7 w-7 rounded-full bg-emerald-400 text-slate-950 flex items-center justify-center font-bold text-xs shadow-md">
                ⚡
              </div>
            </div>

            <div className="max-w-2xl mx-auto space-y-2">
              <Badge className="bg-emerald-500/10 text-emerald-400 border-emerald-500/30 text-xs py-0.5 px-2.5 font-semibold">
                Zero-Friction Quick Start
              </Badge>
              <h2 className="text-xl sm:text-2xl font-black text-white tracking-tight">
                Deploy your first app in 60 seconds — drag and drop a ZIP or paste a Git URL
              </h2>
              <p className="text-xs sm:text-sm text-slate-400 leading-relaxed max-w-xl mx-auto">
                Unggah arsip ZIP (PHP/WordPress, Node.js, atau Static HTML) atau hubungkan repository Git publik/privat. Intake engine akan mendeteksi stack otomatis, mengalokasikan port terisolasi, dan mengarahkan subdomain live instan via Caddy reverse proxy.
              </p>
            </div>

            {/* 3 Step Features */}
            <div className="grid grid-cols-1 md:grid-cols-3 gap-4 max-w-3xl mx-auto text-left pt-2">
              <div className="p-4 rounded-2xl bg-slate-900/90 border border-slate-800 text-xs space-y-1.5">
                <div className="flex items-center gap-2 text-emerald-400 font-bold">
                  <span className="h-5 w-5 rounded-md bg-emerald-500/20 text-emerald-300 flex items-center justify-center text-[11px]">1</span>
                  <span>Anti-ZipSlip Defense</span>
                </div>
                <p className="text-slate-400 text-[11px] leading-normal">
                  Ekstraksi aman dengan verifikasi batas path traversal ketat terisolasi di direktori sandboxing.
                </p>
              </div>

              <div className="p-4 rounded-2xl bg-slate-900/90 border border-slate-800 text-xs space-y-1.5">
                <div className="flex items-center gap-2 text-teal-400 font-bold">
                  <span className="h-5 w-5 rounded-md bg-teal-500/20 text-teal-300 flex items-center justify-center text-[11px]">2</span>
                  <span>Auto Stack Detection</span>
                </div>
                <p className="text-slate-400 text-[11px] leading-normal">
                  Deteksi instan file index.php, wp-config.php, package.json, atau index.html tanpa konfigurasi manual.
                </p>
              </div>

              <div className="p-4 rounded-2xl bg-slate-900/90 border border-slate-800 text-xs space-y-1.5">
                <div className="flex items-center gap-2 text-blue-400 font-bold">
                  <span className="h-5 w-5 rounded-md bg-blue-500/20 text-blue-300 flex items-center justify-center text-[11px]">3</span>
                  <span>Subdomain & Auto-SSL</span>
                </div>
                <p className="text-slate-400 text-[11px] leading-normal">
                  Alokasi subdomain &#123;slug&#125;.{baseDomain} otomatis ter-route ke port aplikasi sandbox.
                </p>
              </div>
            </div>

            <div className="pt-2 flex flex-col sm:flex-row items-center justify-center gap-3">
              <Button
                onClick={() => setIsWizardOpen(true)}
                className="h-11 px-6 bg-emerald-500 hover:bg-emerald-400 text-slate-950 font-bold text-sm shadow-xl shadow-emerald-500/20 gap-2 active:scale-95 transition-all"
              >
                <Plus className="h-4 w-4" />
                Mulai Deploy Proyek Pertama
              </Button>
              {onNavigate && (
                <Button
                  variant="outline"
                  onClick={() => onNavigate(appMode === 'studio' ? '/marketing' : '/')}
                  className="h-11 px-4 text-xs text-slate-300 border-slate-700 bg-slate-800/40 hover:bg-slate-800"
                >
                  Eksplorasi Landing Page & Sponsorship
                </Button>
              )}
            </div>
          </div>
        ) : filteredDeployments.length === 0 ? (
          /* Filtered Empty State */
          <div className="h-80 flex flex-col items-center justify-center text-center p-8 bg-white dark:bg-slate-900 rounded-2xl border border-slate-200 dark:border-slate-800 space-y-4">
            <div className="h-14 w-14 rounded-2xl bg-emerald-100 dark:bg-emerald-950/60 text-emerald-600 dark:text-emerald-400 flex items-center justify-center mx-auto">
              <Sparkles className="h-7 w-7" />
            </div>
            <div className="max-w-md space-y-1">
              <h3 className="text-base font-bold text-slate-900 dark:text-white">
                Tidak ada proyek yang cocok dengan filter
              </h3>
              <p className="text-xs text-slate-500 dark:text-slate-400">
                Coba atur ulang kata kunci pencarian atau filter stack Anda.
              </p>
            </div>
            <Button
              variant="outline"
              onClick={() => {
                setSearchQuery('')
                setRuntimeFilter('all')
                setStatusFilter('all')
              }}
              className="text-xs"
            >
              Reset Filter
            </Button>
          </div>
        ) : (
          <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-5">
            {filteredDeployments.map((dep) => (
              <ProjectCard
                key={dep.id}
                deployment={dep}
                baseDomain={baseDomain}
                servers={servers}
                customDomains={domains}
                onOpenDrawer={handleOpenDrawer}
                onOpenCustomDomains={handleOpenCustomDomains}
                onOpenDiscussions={handleOpenDiscussionsDrawer}
                onOpenTasks={handleOpenTasksDrawer}
                onQuickStart={handleQuickStart}
                onQuickStop={handleQuickStop}
                onQuickRestart={handleQuickRestart}
                isActionLoading={actionLoadingId === dep.id}
                copiedId={copiedId}
                onCopyUrl={copyUrl}
              />
            ))}
          </div>
        )}
          </>
        )}
      </main>

      {/* Deploy Wizard Modal */}
      <DeployWizardModal
        isOpen={isWizardOpen}
        onClose={() => setIsWizardOpen(false)}
        onSuccess={handleDeploymentCreated}
        baseDomain={baseDomain}
        servers={servers}
      />

      {/* Creator Project Wizard Modal (S10.4) */}
      <CreatorWizardModal
        isOpen={isCreatorWizardOpen}
        onClose={() => setIsCreatorWizardOpen(false)}
        onSuccess={handleCreatorProjectCreated}
        onOpenZipWizard={() => setIsZipWizardOpen(true)}
      />

      {/* Dynamic Project Module Configuration Modal (S11.4) */}
      <ModuleToggleModal
        isOpen={isModuleToggleModalOpen}
        onClose={() => setIsModuleToggleModalOpen(false)}
        currentModules={enabledModules}
        projectName={activeProject.name}
        projectId={activeProject.id}
        onSaveModules={handleSaveModules}
      />

      {/* AI Script ZIP Onboarding Wizard Modal (S11.4) */}
      <ZipUploadWizardModal
        isOpen={isZipWizardOpen}
        onClose={() => setIsZipWizardOpen(false)}
        onProjectCreated={(response: CreateFromZipResponse) => {
          setIsZipWizardOpen(false)
          fetchProjectsList()
          fetchDeployments()
          if (response.project) {
            setActiveProjectId(response.project.id)
            setActiveTab('tasks')
            onNavigate?.('/studio/tasks')
          }
          showToast(
            'success',
            `🎉 Project "${response.project?.name || 'Baru'}" berhasil dibuat via AI ZIP Onboarding!`
          )
        }}
      />

      {/* Project Control Drawer */}
      <ProjectDrawer
        deployment={selectedDeployment}
        isOpen={isDrawerOpen}
        onClose={() => setIsDrawerOpen(false)}
        onUpdate={handleDeploymentUpdated}
        onDelete={handleDeploymentDeleted}
        baseDomain={baseDomain}
        initialTab={drawerInitialTab}
      />

      {/* Custom Domains Modal (S05.4) */}
      <CustomDomainsModal
        deployment={selectedDomainDeployment}
        isOpen={isCustomDomainsModalOpen}
        onClose={() => {
          setIsCustomDomainsModalOpen(false)
          fetchDomains()
        }}
        baseDomain={baseDomain}
        onDomainsUpdated={fetchDomains}
      />

      {/* Batch Campaign Studio Modal (S08.4) */}
      <BatchCampaignModal
        isOpen={isBatchModalOpen}
        onClose={() => setIsBatchModalOpen(false)}
        onCreated={(newCampaign) => {
          setIsBatchModalOpen(false)
          setActiveTab('campaigns')
          setSelectedCampaignId(newCampaign.id)
          onNavigate?.(`/studio/campaigns/${newCampaign.id}`)
          showToast('success', `Kampanye batch "${newCampaign.title}" berhasil dibuat!`)
        }}
        projects={deployments.map((d) => ({
          id: d.id,
          name: d.project_name,
          tags: [
            d.runtime,
            d.project_name.toLowerCase().includes('wp') || d.runtime === 'php' ? 'wordpress' : '',
            d.project_name.toLowerCase().includes('ad') ? 'adsense' : '',
          ].filter(Boolean),
          repo_url: d.domain,
        }))}
      />
    </div>
  )
}

