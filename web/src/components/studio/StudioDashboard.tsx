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
  Server
} from 'lucide-react'
import { DeploymentItem, ServerItem, CustomDomainItem } from './types'
import { DeployWizardModal } from './DeployWizardModal'
import { ProjectDrawer } from './ProjectDrawer'
import { ServersView } from './ServersView'
import { DomainsView } from './DomainsView'
import { CustomDomainsModal } from './CustomDomainsModal'
import { ProjectCard } from './ProjectCard'

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

  // Tab state: 'projects' | 'servers' | 'domains'
  const [activeTab, setActiveTab] = useState<'projects' | 'servers' | 'domains'>(() => {
    if (typeof window !== 'undefined') {
      if (window.location.pathname.startsWith('/studio/domains')) {
        return 'domains'
      }
      if (window.location.pathname.startsWith('/studio/servers')) {
        return 'servers'
      }
    }
    return 'projects'
  })

  // Synchronize when currentPath prop changes
  useEffect(() => {
    if (currentPath === '/studio/domains') {
      setActiveTab('domains')
    } else if (currentPath === '/studio/servers') {
      setActiveTab('servers')
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

          {/* Navigation Tabs (Desktop) */}
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
              <span>Servers (BYOS)</span>
              <span className="h-4 px-1.5 rounded-full bg-indigo-100 dark:bg-indigo-950 text-indigo-600 dark:text-indigo-400 text-[10px] font-mono flex items-center justify-center">
                {servers.length}
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
              }}
              className="h-9 text-xs flex items-center gap-1.5"
            >
              <RefreshCw className={`h-3.5 w-3.5 ${loading ? 'animate-spin' : ''}`} />
              <span className="hidden sm:inline">Refresh</span>
            </Button>

            <Button
              onClick={() => setIsWizardOpen(true)}
              className="h-9 bg-emerald-600 hover:bg-emerald-500 text-white font-semibold text-xs shadow-md shadow-emerald-600/20 flex items-center gap-1.5"
            >
              <Plus className="h-4 w-4" />
              <span>Deploy New Project</span>
            </Button>
          </div>
        </div>
      </header>

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

        {/* Mobile Navigation Tab Bar */}
        <div className="flex md:hidden items-center rounded-xl bg-slate-100 dark:bg-slate-800 p-1 border border-slate-200 dark:border-slate-700/60 text-xs">
          <button
            onClick={() => {
              setActiveTab('projects')
              onNavigate?.('/studio')
            }}
            className={`flex-1 flex items-center justify-center gap-1.5 py-2 px-2 rounded-lg font-semibold transition-all ${
              activeTab === 'projects'
                ? 'bg-white dark:bg-slate-900 text-slate-900 dark:text-white shadow-xs'
                : 'text-slate-500 hover:text-slate-900 dark:text-slate-400'
            }`}
          >
            <Layers className="h-3.5 w-3.5 text-emerald-500" />
            <span>Projects ({deployments.length})</span>
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

        {/* Tab View Switching: Domains Hub vs Servers Hub vs Projects Dashboard */}
        {activeTab === 'domains' ? (
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

      {/* Project Control Drawer */}
      <ProjectDrawer
        deployment={selectedDeployment}
        isOpen={isDrawerOpen}
        onClose={() => setIsDrawerOpen(false)}
        onUpdate={handleDeploymentUpdated}
        onDelete={handleDeploymentDeleted}
        baseDomain={baseDomain}
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
    </div>
  )
}

