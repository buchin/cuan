import { useState, useEffect, useMemo } from 'react'
import { Card, CardHeader } from '../ui/card'
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
  RotateCcw,
  Terminal,
  Globe,
  FileCode,
  Cpu,
  Layers,
  ArrowLeft,
  Sparkles,
  AlertCircle,
  FolderGit2,
  UploadCloud,
  Check,
  Copy,
  Server
} from 'lucide-react'
import { DeploymentItem } from './types'
import { DeployWizardModal } from './DeployWizardModal'
import { ProjectDrawer } from './ProjectDrawer'

interface StudioDashboardProps {
  onNavigate?: (path: string) => void
  baseDomain?: string
}

export function StudioDashboard({
  onNavigate,
  baseDomain = 'cuan.dojo.cc'
}: StudioDashboardProps) {
  const [deployments, setDeployments] = useState<DeploymentItem[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [notification, setNotification] = useState<{ type: 'success' | 'error'; message: string } | null>(null)

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

  const getRuntimeIcon = (runtime: string) => {
    switch (runtime) {
      case 'php':
        return <FileCode className="h-4 w-4 text-blue-500" />
      case 'nodejs':
        return <Cpu className="h-4 w-4 text-emerald-500" />
      case 'static':
        return <Globe className="h-4 w-4 text-amber-500" />
      default:
        return <Layers className="h-4 w-4 text-slate-400" />
    }
  }

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
                </div>
                <p className="text-[11px] text-slate-400 hidden sm:block">Universal App Deployer & Virtual Host Manager</p>
              </div>
            </div>
          </div>

          <div className="flex items-center gap-2 sm:gap-3">
            <Button
              variant="outline"
              size="sm"
              onClick={fetchDeployments}
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
        ) : filteredDeployments.length === 0 ? (
          /* Empty State */
          <div className="h-80 flex flex-col items-center justify-center text-center p-8 bg-white dark:bg-slate-900 rounded-2xl border border-slate-200 dark:border-slate-800 space-y-4">
            <div className="h-14 w-14 rounded-2xl bg-emerald-100 dark:bg-emerald-950/60 text-emerald-600 dark:text-emerald-400 flex items-center justify-center mx-auto">
              <Sparkles className="h-7 w-7" />
            </div>
            <div className="max-w-md space-y-1">
              <h3 className="text-base font-bold text-slate-900 dark:text-white">
                {searchQuery || runtimeFilter !== 'all' || statusFilter !== 'all'
                  ? 'Tidak ada proyek yang cocok dengan filter'
                  : 'Belum Ada Aplikasi yang Di-Deploy'}
              </h3>
              <p className="text-xs text-slate-500 dark:text-slate-400">
                {searchQuery || runtimeFilter !== 'all' || statusFilter !== 'all'
                  ? 'Coba atur ulang kata kunci pencarian atau filter stack Anda.'
                  : 'Gunakan tombol Deploy New Project untuk mengunggah berkas ZIP (Static, PHP, Node.js) atau clone repositori Git.'}
              </p>
            </div>
            {!searchQuery && runtimeFilter === 'all' && statusFilter === 'all' && (
              <Button
                onClick={() => setIsWizardOpen(true)}
                className="bg-emerald-600 hover:bg-emerald-500 text-white font-semibold text-xs shadow-md shadow-emerald-600/20 flex items-center gap-1.5"
              >
                <Plus className="h-4 w-4" /> Deploy Project Pertama Anda
              </Button>
            )}
          </div>
        ) : (
          <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-5">
            {filteredDeployments.map((dep) => {
              const liveURL = dep.domain
                ? `http://${dep.domain}`
                : `http://${dep.slug}.${baseDomain}`
              const isActionLoading = actionLoadingId === dep.id

              return (
                <Card
                  key={dep.id}
                  className="bg-white dark:bg-slate-900 border-slate-200 dark:border-slate-800 shadow-xs hover:shadow-md transition-all flex flex-col justify-between overflow-hidden group"
                >
                  <CardHeader className="p-5 pb-3 space-y-2.5">
                    <div className="flex items-start justify-between gap-2">
                      <div className="space-y-0.5 min-w-0">
                        <div className="flex items-center gap-2">
                          <h3
                            onClick={() => handleOpenDrawer(dep)}
                            className="font-bold text-slate-900 dark:text-white text-base hover:text-emerald-600 dark:hover:text-emerald-400 cursor-pointer truncate"
                          >
                            {dep.project_name}
                          </h3>
                        </div>
                        <p className="text-xs font-mono text-slate-400">/{dep.slug}</p>
                      </div>

                      {/* Status Badge */}
                      {dep.status === 'running' ? (
                        <Badge variant="success" className="gap-1 py-0.5 px-2 text-[11px] shrink-0">
                          <span className="h-1.5 w-1.5 rounded-full bg-white animate-pulse" />
                          Running
                        </Badge>
                      ) : dep.status === 'stopped' ? (
                        <Badge variant="secondary" className="gap-1 py-0.5 px-2 text-[11px] shrink-0">
                          <span className="h-1.5 w-1.5 rounded-full bg-slate-400" />
                          Stopped
                        </Badge>
                      ) : (
                        <Badge variant="outline" className="text-[11px] shrink-0">
                          {dep.status}
                        </Badge>
                      )}
                    </div>

                    {/* Live Subdomain Link Row */}
                    <div className="flex items-center justify-between p-2.5 bg-slate-50 dark:bg-slate-800/60 rounded-xl border border-slate-100 dark:border-slate-800 text-xs">
                      <a
                        href={liveURL}
                        target="_blank"
                        rel="noreferrer"
                        className="font-mono text-[11px] text-emerald-600 dark:text-emerald-400 hover:underline flex items-center gap-1 truncate"
                      >
                        {liveURL.replace('http://', '')}
                        <ExternalLink className="h-3 w-3 shrink-0" />
                      </a>
                      <button
                        onClick={() => copyUrl(dep.id, liveURL)}
                        className="text-slate-400 hover:text-slate-600 dark:hover:text-slate-200 p-1"
                        title="Salin Subdomain URL"
                      >
                        {copiedId === dep.id ? (
                          <Check className="h-3.5 w-3.5 text-emerald-500" />
                        ) : (
                          <Copy className="h-3.5 w-3.5" />
                        )}
                      </button>
                    </div>

                    {/* Metadata tags: Stack, Port, Source */}
                    <div className="flex items-center gap-2 flex-wrap pt-1 text-[11px] text-slate-500 dark:text-slate-400">
                      <span className="inline-flex items-center gap-1 font-semibold capitalize bg-slate-100 dark:bg-slate-800 px-2 py-0.5 rounded-md">
                        {getRuntimeIcon(dep.runtime)}
                        {dep.runtime}
                      </span>
                      {dep.internal_port > 0 && (
                        <span className="font-mono bg-slate-100 dark:bg-slate-800 px-2 py-0.5 rounded-md">
                          :{dep.internal_port}
                        </span>
                      )}
                      <span className="inline-flex items-center gap-1 bg-slate-100 dark:bg-slate-800 px-2 py-0.5 rounded-md">
                        {dep.source_type === 'git' ? (
                          <>
                            <FolderGit2 className="h-3 w-3 text-blue-500" /> Git
                          </>
                        ) : (
                          <>
                            <UploadCloud className="h-3 w-3 text-emerald-500" /> ZIP
                          </>
                        )}
                      </span>
                    </div>
                  </CardHeader>

                  {/* Card Action Controls Footer */}
                  <div className="p-4 pt-3 border-t border-slate-100 dark:border-slate-800/80 bg-slate-50/50 dark:bg-slate-900/50 flex items-center justify-between gap-2">
                    <div className="flex items-center gap-1.5">
                      {dep.status === 'running' ? (
                        <Button
                          size="sm"
                          variant="outline"
                          onClick={() => handleQuickStop(dep)}
                          disabled={isActionLoading}
                          className="h-8 text-xs font-semibold border-amber-300 dark:border-amber-800 text-amber-700 dark:text-amber-300 hover:bg-amber-50 dark:hover:bg-amber-950 flex items-center gap-1"
                        >
                          {isActionLoading ? (
                            <RefreshCw className="h-3 w-3 animate-spin" />
                          ) : (
                            <Square className="h-3 w-3 fill-current" />
                          )}
                          Stop
                        </Button>
                      ) : (
                        <Button
                          size="sm"
                          onClick={() => handleQuickStart(dep)}
                          disabled={isActionLoading}
                          className="h-8 text-xs font-semibold bg-emerald-600 hover:bg-emerald-500 text-white flex items-center gap-1"
                        >
                          {isActionLoading ? (
                            <RefreshCw className="h-3 w-3 animate-spin" />
                          ) : (
                            <Play className="h-3 w-3 fill-current" />
                          )}
                          Start
                        </Button>
                      )}

                      <Button
                        size="sm"
                        variant="outline"
                        onClick={() => handleQuickRestart(dep)}
                        disabled={isActionLoading}
                        className="h-8 text-xs font-semibold text-slate-700 dark:text-slate-300 hover:bg-slate-100 dark:hover:bg-slate-800 px-2"
                        title="Restart proses"
                      >
                        <RotateCcw className="h-3 w-3" />
                      </Button>
                    </div>

                    <Button
                      size="sm"
                      variant="outline"
                      onClick={() => handleOpenDrawer(dep)}
                      className="h-8 text-xs font-semibold flex items-center gap-1 text-slate-700 dark:text-slate-300 hover:bg-slate-100 dark:hover:bg-slate-800"
                    >
                      <Terminal className="h-3.5 w-3.5 text-emerald-500" />
                      <span>Logs & Drawer</span>
                    </Button>
                  </div>
                </Card>
              )
            })}
          </div>
        )}
      </main>

      {/* Deploy Wizard Modal */}
      <DeployWizardModal
        isOpen={isWizardOpen}
        onClose={() => setIsWizardOpen(false)}
        onSuccess={handleDeploymentCreated}
        baseDomain={baseDomain}
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
    </div>
  )
}
