import { useState, useEffect, useMemo } from 'react'
import { Card } from '../ui/card'
import { Button } from '../ui/button'
import { Badge } from '../ui/badge'
import { cn } from '../../lib/utils'
import {
  Plus,
  RefreshCw,
  ExternalLink,
  Globe,
  Layers,
  Sparkles,
  UploadCloud,
  Server,
  Activity,
  MessageSquare,
  Kanban,
  Rocket,
  Bot,
  Settings2,
  GitBranch,
  ChevronDown,
  FolderGit2
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
import { ProjectDiscussionsView } from './ProjectDiscussionsView'
import { ProjectTasksBoard } from './ProjectTasksBoard'
import { AgentsHubView } from './AgentsHubView'
import { ModuleToggleModal } from './ModuleToggleModal'
import { ZipUploadWizardModal } from './ZipUploadWizardModal'
import { ScriptDeployerView } from './deployer'
import { WordPressDashboardView } from './wordpress'
import { TeamView } from './TeamView'
import { DashboardEmptyState } from './DashboardEmptyState'
import { CreateProjectModal } from './CreateProjectModal'
import { ProjectsDirectoryView } from './ProjectsDirectoryView'

export type StudioTab =
  | 'dashboard'
  | 'projects'
  | 'deployer'
  | 'wordpress'
  | 'discussions'
  | 'tasks'
  | 'agents'
  | 'team'
  | 'servers'
  | 'domains'
  | 'observability'
  | 'repositories'

interface StudioDashboardProps {
  currentPath?: string
  onNavigate?: (path: string) => void
  baseDomain?: string
  hideHeader?: boolean
}

export function StudioDashboard({
  currentPath,
  onNavigate,
  baseDomain = 'cuan.dojo.cc',
  hideHeader = false,
}: StudioDashboardProps) {
  const [deployments, setDeployments] = useState<DeploymentItem[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [notification, setNotification] = useState<{ type: 'success' | 'error'; message: string } | null>(null)

  // Drawer initial tab state
  const [drawerInitialTab, setDrawerInitialTab] = useState<'overview' | 'discussions' | 'tasks'>('overview')

  // Tab state: StudioTab
  // Tab state: StudioTab
  const [activeTab, setActiveTab] = useState<StudioTab>(() => {
    if (typeof window !== 'undefined') {
      const p = window.location.pathname
      if (p.startsWith('/studio/agents') || p.startsWith('/agents')) return 'agents'
      if (p.startsWith('/studio/team') || p.startsWith('/team')) return 'team'
      if (p.startsWith('/studio/deployer') || p.startsWith('/deployer')) return 'deployer'
      if (p.startsWith('/studio/wordpress') || p.startsWith('/wordpress')) return 'wordpress'
      if (p.startsWith('/studio/discussions') || p.startsWith('/discussions')) return 'discussions'
      if (p.startsWith('/studio/tasks') || p.startsWith('/tasks')) return 'tasks'
      if (p.startsWith('/studio/observability') || p.startsWith('/observability')) return 'observability'
      if (p.startsWith('/studio/domains') || p.startsWith('/domains') || p.startsWith('/studio/settings') || p.startsWith('/settings')) return 'domains'
      if (p.startsWith('/studio/servers') || p.startsWith('/servers')) return 'servers'
      if (p.startsWith('/studio/repositories') || p.startsWith('/repositories')) return 'repositories'
      if (p.startsWith('/studio/projects') || p === '/projects') return 'projects'
      if (p === '/' || p === '/dashboard' || p === '/studio' || p === '/studio/dashboard') return 'dashboard'
    }
    return 'dashboard'
  })

  // Fallback project representation when projectsList is empty
  const FALLBACK_EMPTY_PROJECT: ProjectItemDTO = useMemo(() => ({
    id: 'default',
    name: 'Default Workspace',
    slug: 'default',
    project_type: 'saas',
    enabled_modules: ['tasks', 'discussions'],
  }), [])

  const [projectsList, setProjectsList] = useState<ProjectItemDTO[]>([])
  const [activeProjectId, setActiveProjectId] = useState<string>('')
  const [isModuleToggleModalOpen, setIsModuleToggleModalOpen] = useState(false)
  const [isZipWizardOpen, setIsZipWizardOpen] = useState(false)
  const [isCreateProjectModalOpen, setIsCreateProjectModalOpen] = useState(false)

  const handleCreateProjectCreated = (newProj: any) => {
    setProjectsList((prev) => [newProj, ...prev])
    setActiveProjectId(newProj.id)
    showToast('success', `Proyek "${newProj.name}" berhasil dibuat dan diaktifkan.`)
    setIsCreateProjectModalOpen(false)
  }

  const fetchProjectsList = async () => {
    try {
      const res = await fetch('/api/v1/projects?include_archived=true')
      if (res.ok) {
        const data = await res.json()
        const serverProjects: ProjectItemDTO[] = data.projects || []
        setProjectsList(serverProjects)
        if (serverProjects.length > 0 && !activeProjectId) {
          const firstActive = serverProjects.find((p) => !p.is_archived && p.status !== 'archived')
          if (firstActive) {
            setActiveProjectId(firstActive.id)
          }
        }
      }
    } catch {}
  }

  const handleArchiveProject = async (id: string, name: string) => {
    try {
      const res = await fetch(`/api/v1/projects/${id}/archive`, {
        method: 'PATCH',
      })
      if (!res.ok) {
        throw new Error(`HTTP ${res.status}: Gagal mengarsipkan proyek`)
      }
      showToast('success', `Proyek "${name}" berhasil diarsipkan.`)
      await fetchProjectsList()
      if (activeProjectId === id) {
        const remaining = projectsList.filter(
          (p) => p.id !== id && !p.is_archived && p.status !== 'archived'
        )
        if (remaining.length > 0) {
          setActiveProjectId(remaining[0].id)
        }
      }
    } catch (err: any) {
      showToast('error', err.message || 'Gagal mengarsipkan proyek')
    }
  }

  const handleUnarchiveProject = async (id: string, name: string) => {
    try {
      const res = await fetch(`/api/v1/projects/${id}/unarchive`, {
        method: 'PATCH',
      })
      if (!res.ok) {
        throw new Error(`HTTP ${res.status}: Gagal memulihkan proyek`)
      }
      showToast('success', `Proyek "${name}" berhasil dipulihkan ke proyek aktif.`)
      await fetchProjectsList()
    } catch (err: any) {
      showToast('error', err.message || 'Gagal memulihkan proyek')
    }
  }

  useEffect(() => {
    fetchProjectsList()
  }, [])

  const activeProjects = useMemo(() => {
    return projectsList.filter((p) => !p.is_archived && p.status !== 'archived')
  }, [projectsList])

  const activeProject = useMemo<ProjectItemDTO>(() => {
    return (
      activeProjects.find((p) => p.id === activeProjectId) ||
      activeProjects[0] ||
      FALLBACK_EMPTY_PROJECT
    )
  }, [activeProjects, activeProjectId, FALLBACK_EMPTY_PROJECT])

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
      ['tasks', 'deployer', 'wordpress', 'discussions', 'repositories', 'servers'].includes(activeTab) &&
      !targetModules.includes(activeTab)
    ) {
      if (targetModules.includes('tasks')) {
        setActiveTab('tasks')
      } else if (targetModules.includes('deployer')) {
        setActiveTab('deployer')
      } else if (targetModules.includes('wordpress')) {
        setActiveTab('wordpress')
      } else if (targetModules.includes('discussions')) {
        setActiveTab('discussions')
      } else {
        setActiveTab('dashboard')
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
      ['tasks', 'deployer', 'wordpress', 'discussions', 'repositories', 'servers'].includes(activeTab) &&
      !updatedModules.includes(activeTab)
    ) {
      if (updatedModules.includes('tasks')) {
        setActiveTab('tasks')
      } else if (updatedModules.includes('deployer')) {
        setActiveTab('deployer')
      } else if (updatedModules.includes('wordpress')) {
        setActiveTab('wordpress')
      } else if (updatedModules.includes('discussions')) {
        setActiveTab('discussions')
      } else {
        setActiveTab('dashboard')
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
    if (!currentPath) return
    if (currentPath === '/studio/agents' || currentPath.startsWith('/studio/agents') || currentPath === '/agents' || currentPath.startsWith('/agents')) {
      setActiveTab('agents')
    } else if (currentPath === '/studio/team' || currentPath.startsWith('/studio/team') || currentPath === '/team' || currentPath.startsWith('/team')) {
      setActiveTab('team')
    } else if (currentPath === '/studio/deployer' || currentPath.startsWith('/studio/deployer') || currentPath === '/deployer' || currentPath.startsWith('/deployer')) {
      setActiveTab('deployer')
    } else if (currentPath === '/studio/wordpress' || currentPath.startsWith('/studio/wordpress') || currentPath === '/wordpress' || currentPath.startsWith('/wordpress')) {
      setActiveTab('wordpress')
    } else if (currentPath === '/studio/discussions' || currentPath.startsWith('/studio/discussions') || currentPath === '/discussions' || currentPath.startsWith('/discussions')) {
      setActiveTab('discussions')
    } else if (currentPath === '/studio/tasks' || currentPath.startsWith('/studio/tasks') || currentPath === '/tasks' || currentPath.startsWith('/tasks')) {
      setActiveTab('tasks')
    } else if (currentPath === '/studio/observability' || currentPath === '/observability') {
      setActiveTab('observability')
    } else if (currentPath === '/studio/domains' || currentPath === '/domains' || currentPath === '/studio/settings' || currentPath === '/settings') {
      setActiveTab('domains')
    } else if (currentPath === '/studio/servers' || currentPath === '/servers') {
      setActiveTab('servers')
    } else if (currentPath === '/studio/repositories' || currentPath.startsWith('/studio/repositories') || currentPath === '/repositories' || currentPath.startsWith('/repositories')) {
      setActiveTab('repositories')
    } else if (currentPath === '/studio/projects' || currentPath === '/projects' || currentPath.startsWith('/projects')) {
      setActiveTab('projects')
    } else if (
      currentPath === '/' ||
      currentPath === '/studio' ||
      currentPath === '/studio/dashboard' ||
      currentPath === '/dashboard'
    ) {
      setActiveTab('dashboard')
    }
  }, [currentPath])

  // Custom domains state (S05.4)
  const [domains, setDomains] = useState<CustomDomainItem[]>([])

  const fetchDomains = async () => {
    try {
      const res = await fetch('/api/domains')
      if (res.ok) {
        const data = await res.json()
        setDomains(data.domains || [])
      }
    } catch {}
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

  // Modals & Drawer
  const [isWizardOpen, setIsWizardOpen] = useState(false)
  const [isCreatorWizardOpen, setIsCreatorWizardOpen] = useState(false)
  const [selectedDeployment, setSelectedDeployment] = useState<DeploymentItem | null>(null)
  const [isDrawerOpen, setIsDrawerOpen] = useState(false)

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

  const handleDeploymentCreated = (newDep: DeploymentItem) => {
    setDeployments([newDep, ...deployments])
    setIsWizardOpen(false)
    showToast('success', `Proyek "${newDep.project_name}" berhasil dibuat!`)
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

  // Combined project list for Tasks Board & Discussions (deployments + active projects)
  const combinedProjectsForTasks = useMemo(() => {
    const list = deployments.map((d) => ({ id: d.id, name: d.project_name }))
    const existingIds = new Set(list.map((p) => p.id))
    for (const p of projectsList) {
      if (!p.is_archived && p.status !== 'archived') {
        if (!existingIds.has(p.id)) {
          list.push({ id: p.id, name: p.name })
          existingIds.add(p.id)
        }
      }
    }
    return list
  }, [deployments, projectsList])

  return (
    <div className={cn(
      "flex flex-col",
      hideHeader ? "bg-transparent text-foreground w-full" : "min-h-screen bg-slate-50 dark:bg-slate-950 text-slate-900 dark:text-slate-100"
    )}>
      {/* Top Navigation Bar */}
      {!hideHeader && (
      <header className="sticky top-0 z-30 border-b border-slate-200 dark:border-slate-800 bg-white/90 dark:bg-slate-900/90 backdrop-blur-md">
        <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 h-16 flex items-center justify-between gap-4">
          <div className="flex items-center gap-3">

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

            {enabledModules.includes('wordpress') && (
              <button
                onClick={() => {
                  setActiveTab('wordpress')
                  onNavigate?.('/studio/wordpress')
                }}
                className={`flex items-center gap-1.5 py-1.5 px-3 rounded-lg font-semibold transition-all ${
                  activeTab === 'wordpress'
                    ? 'bg-white dark:bg-slate-900 text-slate-900 dark:text-white shadow-xs'
                    : 'text-slate-500 hover:text-slate-900 dark:text-slate-400 dark:hover:text-white'
                }`}
              >
                <Globe className="h-3.5 w-3.5 text-blue-500" />
                <span>WordPress</span>
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
      )}

      {/* Dynamic Workspace Switcher & Module Bar (Only shown in standalone mode) */}
      {!hideHeader && (
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
                  {projectsList.filter((p) => !p.is_archived && p.status !== 'archived').map((p) => (
                    <option key={p.id} value={p.id}>
                      📦 {p.name}
                    </option>
                  ))}
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
      )}

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

        {error && (
          <div className="p-4 rounded-xl flex items-center justify-between text-xs font-semibold shadow-md bg-red-500 text-white shadow-red-500/20">
            <span>{error}</span>
            <button onClick={() => setError(null)} className="opacity-80 hover:opacity-100 font-bold">
              ✕
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

          {enabledModules.includes('wordpress') && (
            <button
              onClick={() => {
                setActiveTab('wordpress')
                onNavigate?.('/studio/wordpress')
              }}
              className={`flex-none flex items-center justify-center gap-1.5 py-1.5 px-2.5 rounded-lg font-semibold transition-all ${
                activeTab === 'wordpress'
                  ? 'bg-white dark:bg-slate-900 text-slate-900 dark:text-white shadow-xs'
                  : 'text-slate-500 hover:text-slate-900 dark:text-slate-400'
              }`}
            >
              <Globe className="h-3.5 w-3.5 text-blue-500" />
              <span>WordPress</span>
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



        {/* Tab View Switching: Dashboard vs Discussions vs Tasks vs Projects vs Contextual Modules */}
        {activeTab === 'dashboard' ? (
          activeProjects.length === 0 ? (
            <DashboardEmptyState
              onCreateProjectClick={() => setIsCreateProjectModalOpen(true)}
              onExploreGlobalClick={() => {
                setActiveTab('projects')
                onNavigate?.('/projects')
              }}
            />
          ) : (
            <div className="space-y-6 animate-in fade-in duration-200">
              {/* Minimalist Tokyo Night Dashboard Header */}
              <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-4">
                <div>
                  <h1 className="text-xl sm:text-2xl font-bold tracking-tight text-foreground">
                    Workspace Overview
                  </h1>
                  <p className="text-xs sm:text-sm text-muted-foreground mt-0.5">
                    Ringkasan performa proyek aktif, status pengerjaan task, dan ekosistem autonomous agent.
                  </p>
                </div>
                <div className="flex items-center gap-2">
                  <Button
                    size="sm"
                    onClick={() => setIsCreateProjectModalOpen(true)}
                    className="h-8 text-xs font-semibold gap-1.5 bg-primary text-primary-foreground hover:bg-primary/90"
                  >
                    <Plus className="h-3.5 w-3.5" />
                    <span>Create New Project</span>
                  </Button>
                </div>
              </div>

              {/* 4 Essential Metric Cards */}
              <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-4">
                <Card className="p-4 bg-card text-card-foreground border-border space-y-1">
                  <div className="flex items-center justify-between text-xs text-muted-foreground">
                    <span>Active Projects</span>
                    <FolderGit2 className="h-4 w-4 text-blue-400" />
                  </div>
                  <div className="text-2xl font-bold text-foreground">
                    {activeProjects.length}
                  </div>
                  <p className="text-[11px] text-muted-foreground">
                    Workspace terdaftar & aktif
                  </p>
                </Card>

                <Card className="p-4 bg-card text-card-foreground border-border space-y-1">
                  <div className="flex items-center justify-between text-xs text-muted-foreground">
                    <span>AI Agents</span>
                    <Bot className="h-4 w-4 text-purple-400" />
                  </div>
                  <div className="text-2xl font-bold text-foreground">
                    {agentsCount}
                  </div>
                  <p className="text-[11px] text-muted-foreground">
                    Autonomous AI personas
                  </p>
                </Card>

                <Card className="p-4 bg-card text-card-foreground border-border space-y-1">
                  <div className="flex items-center justify-between text-xs text-muted-foreground">
                    <span>Running Deployments</span>
                    <Rocket className="h-4 w-4 text-cyan-400" />
                  </div>
                  <div className="text-2xl font-bold text-foreground">
                    {deployments.length}
                  </div>
                  <p className="text-[11px] text-muted-foreground">
                    Container & static instances
                  </p>
                </Card>

                <Card className="p-4 bg-card text-card-foreground border-border space-y-1">
                  <div className="flex items-center justify-between text-xs text-muted-foreground">
                    <span>Connected Servers</span>
                    <Server className="h-4 w-4 text-emerald-400" />
                  </div>
                  <div className="text-2xl font-bold text-foreground">
                    {servers.length}
                  </div>
                  <p className="text-[11px] text-muted-foreground">
                    Node & host remote aktif
                  </p>
                </Card>
              </div>

              {/* Active Projects Grid Overview */}
              <div className="space-y-3">
                <div className="flex items-center justify-between">
                  <h2 className="text-sm font-bold text-foreground flex items-center gap-2">
                    <Layers className="h-4 w-4 text-primary" />
                    <span>Active Projects ({activeProjects.length})</span>
                  </h2>
                  <button
                    type="button"
                    onClick={() => {
                      setActiveTab('projects')
                      onNavigate?.('/projects')
                    }}
                    className="text-xs font-semibold text-primary hover:underline"
                  >
                    Buka Direktori Proyek →
                  </button>
                </div>

                <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
                  {activeProjects.map((p) => (
                    <Card
                      key={p.id}
                      className="p-4 bg-card text-card-foreground border-border hover:border-primary/50 transition-colors space-y-3 flex flex-col justify-between"
                    >
                      <div className="space-y-1.5">
                        <div className="flex items-center justify-between gap-2">
                          <span className="font-bold text-sm text-foreground truncate">
                            {p.name}
                          </span>
                          <Badge variant="outline" className="text-[10px] font-mono py-0 px-1.5 uppercase">
                            {p.project_type || 'saas'}
                          </Badge>
                        </div>
                        <p className="text-xs text-muted-foreground font-mono truncate">
                          ID: #{p.id}
                        </p>
                        {p.enabled_modules && p.enabled_modules.length > 0 && (
                          <div className="flex flex-wrap gap-1 pt-1">
                            {p.enabled_modules.map((m) => (
                              <span
                                key={m}
                                className="text-[9.5px] font-mono px-1.5 py-0.2 bg-muted text-muted-foreground rounded-xs uppercase"
                              >
                                {m}
                              </span>
                            ))}
                          </div>
                        )}
                      </div>

                      <div className="pt-2 border-t border-border flex items-center justify-between gap-2">
                        <button
                          type="button"
                          onClick={() => {
                            handleSelectProject(p.id)
                            setActiveTab('tasks')
                            onNavigate?.('/tasks')
                          }}
                          className="text-xs font-semibold text-primary hover:underline flex items-center gap-1"
                        >
                          <Kanban className="h-3 w-3" />
                          <span>Tasks</span>
                        </button>
                        <button
                          type="button"
                          onClick={() => {
                            handleSelectProject(p.id)
                            setActiveTab('discussions')
                            onNavigate?.('/discussions')
                          }}
                          className="text-xs font-semibold text-cyan-400 hover:underline flex items-center gap-1"
                        >
                          <MessageSquare className="h-3 w-3" />
                          <span>Rooms</span>
                        </button>
                      </div>
                    </Card>
                  ))}
                </div>
              </div>
            </div>
          )
        ) : activeTab === 'discussions' ? (
          <ProjectDiscussionsView
            projects={combinedProjectsForTasks}
            onOpenTaskBoard={() => {
              setActiveTab('tasks')
              onNavigate?.('/studio/tasks')
            }}
          />
        ) : activeTab === 'projects' ? (
          <ProjectsDirectoryView
            projects={projectsList}
            activeProjectId={activeProjectId}
            onSelectProject={(id) => handleSelectProject(id)}
            onArchiveProject={handleArchiveProject}
            onUnarchiveProject={handleUnarchiveProject}
            onCreateProjectClick={() => setIsCreateProjectModalOpen(true)}
            onNavigate={onNavigate}
          />
        ) : activeTab === 'deployer' ? (
          <ScriptDeployerView
            projectId={activeProjectId}
            projects={projectsList.filter((p) => !p.is_archived && p.status !== 'archived')}
            servers={servers}
            baseDomain={baseDomain}
            onToast={(type, message) => showToast(type, message)}
            onProjectChange={(id) => handleSelectProject(id)}
          />
        ) : activeTab === 'wordpress' ? (
          <WordPressDashboardView
            projectId={activeProjectId}
            projects={projectsList.filter((p) => !p.is_archived && p.status !== 'archived')}
            servers={servers}
            baseDomain={baseDomain}
            onToast={(type, message) => showToast(type, message)}
          />
        ) : activeTab === 'tasks' ? (
          <ProjectTasksBoard
            projects={combinedProjectsForTasks}
            onOpenDiscussions={() => {
              setActiveTab('discussions')
              onNavigate?.('/studio/discussions')
            }}
          />
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
        ) : activeTab === 'team' ? (
          <TeamView />
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
          activeProjects.length === 0 ? (
            <DashboardEmptyState
              onCreateProjectClick={() => setIsCreateProjectModalOpen(true)}
              onExploreGlobalClick={() => {
                setActiveTab('projects')
                onNavigate?.('/projects')
              }}
            />
          ) : (
            <ProjectsDirectoryView
              projects={projectsList}
              activeProjectId={activeProjectId}
              onSelectProject={(id) => handleSelectProject(id)}
              onArchiveProject={handleArchiveProject}
              onUnarchiveProject={handleUnarchiveProject}
              onCreateProjectClick={() => setIsCreateProjectModalOpen(true)}
              onNavigate={onNavigate}
            />
          )
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

      {/* Global Interactive Create Project Modal */}
      <CreateProjectModal
        isOpen={isCreateProjectModalOpen}
        onClose={() => setIsCreateProjectModalOpen(false)}
        onCreateProject={handleCreateProjectCreated}
      />
    </div>
  )
}

