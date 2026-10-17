import { useState, useEffect, useRef } from 'react'
import { Button } from '../ui/button'
import { Badge } from '../ui/badge'
import {
  Play,
  Square,
  RotateCcw,
  Trash2,
  ExternalLink,
  Copy,
  Check,
  Terminal,
  Layers,
  Cpu,
  Globe,
  FileCode,
  FolderGit2,
  UploadCloud,
  X,
  AlertCircle,
  RefreshCw,
  Radio,
  Server,
  CheckCircle2,
  Clock
} from 'lucide-react'
import { DeploymentItem, DeploymentLog, HealthStatus, CustomDomainItem } from './types'
import { CustomDomainsModal } from './CustomDomainsModal'

interface ProjectDrawerProps {
  deployment: DeploymentItem | null
  isOpen: boolean
  onClose: () => void
  onUpdate: (updated: DeploymentItem) => void
  onDelete: (id: string) => void
  baseDomain?: string
}

export function ProjectDrawer({
  deployment,
  isOpen,
  onClose,
  onUpdate,
  onDelete,
  baseDomain = 'cuan.dojo.cc'
}: ProjectDrawerProps) {
  if (!isOpen || !deployment) return null

  // Action states
  const [actionLoading, setActionLoading] = useState<string | null>(null)
  const [actionError, setActionError] = useState<string | null>(null)
  const [copiedUrl, setCopiedUrl] = useState(false)
  const [showDeleteConfirm, setShowDeleteConfirm] = useState(false)

  // Live health state
  const [health, setHealth] = useState<HealthStatus | null>(null)

  // Logs state
  const [logs, setLogs] = useState<DeploymentLog[]>([])
  const [logsLoading, setLogsLoading] = useState(false)
  const [streamFilter, setStreamFilter] = useState<'all' | 'stdout' | 'stderr' | 'system'>('all')
  const [autoScroll, setAutoScroll] = useState(true)
  const [copiedLogs, setCopiedLogs] = useState(false)
  const logContainerRef = useRef<HTMLDivElement>(null)

  // Custom domains state (S05.4)
  const [isCustomDomainsModalOpen, setIsCustomDomainsModalOpen] = useState(false)
  const [customDomains, setCustomDomains] = useState<CustomDomainItem[]>([])

  const fetchCustomDomains = async () => {
    if (!deployment) return
    try {
      const res = await fetch(`/api/deployments/${deployment.id}/domains`)
      if (res.ok) {
        const data = await res.json()
        setCustomDomains(data.domains || [])
      }
    } catch {
      // ignore
    }
  }

  // Fetch initial logs & live health
  const fetchLogs = async () => {
    try {
      const res = await fetch(`/api/deployments/${deployment.id}/logs?limit=300`)
      if (res.ok) {
        const data = await res.json()
        setLogs(data.logs || [])
      }
    } catch {
      // ignore transient network errors
    }
  }

  const fetchHealth = async () => {
    try {
      const res = await fetch(`/api/deployments/${deployment.id}/health`)
      if (res.ok) {
        const data = await res.json()
        setHealth(data)
      }
    } catch {
      // ignore
    }
  }

  // Effect to load logs & poll while drawer is open
  useEffect(() => {
    if (!deployment) return

    setLogsLoading(true)
    fetchLogs().finally(() => setLogsLoading(false))
    fetchHealth()
    fetchCustomDomains()

    // Poll logs every 2 seconds while running
    const interval = setInterval(() => {
      fetchLogs()
      if (deployment.status === 'running') {
        fetchHealth()
      }
    }, 2000)

    return () => clearInterval(interval)
  }, [deployment.id, deployment.status])

  // Auto-scroll log viewer to bottom on new logs
  useEffect(() => {
    if (autoScroll && logContainerRef.current) {
      logContainerRef.current.scrollTop = logContainerRef.current.scrollHeight
    }
  }, [logs, autoScroll])

  // Actions: Start, Stop, Restart, Delete
  const handleStart = async () => {
    setActionLoading('start')
    setActionError(null)
    try {
      const res = await fetch(`/api/deployments/${deployment.id}/start`, { method: 'POST' })
      if (!res.ok) {
        const err = await res.json().catch(() => ({}))
        throw new Error(err.error || `HTTP ${res.status}`)
      }
      const data = await res.json()
      onUpdate(data.deployment)
      fetchLogs()
      fetchHealth()
    } catch (err: any) {
      setActionError(err.message || 'Gagal memulai deployment')
    } finally {
      setActionLoading(null)
    }
  }

  const handleStop = async () => {
    setActionLoading('stop')
    setActionError(null)
    try {
      const res = await fetch(`/api/deployments/${deployment.id}/stop`, { method: 'POST' })
      if (!res.ok) {
        const err = await res.json().catch(() => ({}))
        throw new Error(err.error || `HTTP ${res.status}`)
      }
      const data = await res.json()
      onUpdate(data.deployment)
      fetchLogs()
      fetchHealth()
    } catch (err: any) {
      setActionError(err.message || 'Gagal menghentikan deployment')
    } finally {
      setActionLoading(null)
    }
  }

  const handleRestart = async () => {
    setActionLoading('restart')
    setActionError(null)
    try {
      const res = await fetch(`/api/deployments/${deployment.id}/restart`, { method: 'POST' })
      if (!res.ok) {
        const err = await res.json().catch(() => ({}))
        throw new Error(err.error || `HTTP ${res.status}`)
      }
      const data = await res.json()
      onUpdate(data.deployment)
      fetchLogs()
      fetchHealth()
    } catch (err: any) {
      setActionError(err.message || 'Gagal me-restart deployment')
    } finally {
      setActionLoading(null)
    }
  }

  const handleDelete = async () => {
    setActionLoading('delete')
    setActionError(null)
    try {
      const res = await fetch(`/api/deployments/${deployment.id}`, { method: 'DELETE' })
      if (!res.ok) {
        const err = await res.json().catch(() => ({}))
        throw new Error(err.error || `HTTP ${res.status}`)
      }
      onDelete(deployment.id)
      onClose()
    } catch (err: any) {
      setActionError(err.message || 'Gagal menghapus deployment')
    } finally {
      setActionLoading(null)
      setShowDeleteConfirm(false)
    }
  }

  const liveURL = deployment.domain
    ? `http://${deployment.domain}`
    : `http://${deployment.slug}.${baseDomain}`

  const copyLiveURL = () => {
    navigator.clipboard.writeText(liveURL)
    setCopiedUrl(true)
    setTimeout(() => setCopiedUrl(false), 2000)
  }

  const copyAllLogs = () => {
    const text = filteredLogs.map((l) => `[${l.timestamp}] [${l.stream}] ${l.message}`).join('\n')
    navigator.clipboard.writeText(text)
    setCopiedLogs(true)
    setTimeout(() => setCopiedLogs(false), 2000)
  }

  const filteredLogs = logs.filter((log) => {
    if (streamFilter === 'all') return true
    return log.stream === streamFilter
  })

  const getRuntimeBadge = (rt: string) => {
    switch (rt) {
      case 'php':
        return (
          <Badge variant="outline" className="border-blue-400 text-blue-600 dark:text-blue-400 gap-1 text-xs">
            <FileCode className="h-3 w-3" /> PHP / WP
          </Badge>
        )
      case 'nodejs':
        return (
          <Badge variant="outline" className="border-emerald-400 text-emerald-600 dark:text-emerald-400 gap-1 text-xs">
            <Cpu className="h-3 w-3" /> Node.js
          </Badge>
        )
      case 'static':
        return (
          <Badge variant="outline" className="border-amber-400 text-amber-600 dark:text-amber-400 gap-1 text-xs">
            <Globe className="h-3 w-3" /> Static HTML
          </Badge>
        )
      default:
        return (
          <Badge variant="outline" className="border-slate-400 text-slate-600 dark:text-slate-400 gap-1 text-xs">
            <Layers className="h-3 w-3" /> Custom
          </Badge>
        )
    }
  }

  const getStatusBadge = (status: string) => {
    switch (status) {
      case 'running':
        return (
          <Badge variant="success" className="gap-1.5 py-1 px-3">
            <span className="h-2 w-2 rounded-full bg-white animate-pulse" />
            Running
          </Badge>
        )
      case 'stopped':
        return (
          <Badge variant="secondary" className="gap-1.5 py-1 px-3">
            <span className="h-2 w-2 rounded-full bg-slate-400" />
            Stopped
          </Badge>
        )
      case 'building':
      case 'extracting':
      case 'pending':
        return (
          <Badge variant="secondary" className="bg-amber-100 text-amber-700 dark:bg-amber-950 dark:text-amber-300 gap-1.5 py-1 px-3">
            <RefreshCw className="h-3 w-3 animate-spin" />
            {status}
          </Badge>
        )
      case 'failed':
        return (
          <Badge variant="destructive" className="gap-1.5 py-1 px-3">
            <AlertCircle className="h-3 w-3" />
            Failed
          </Badge>
        )
      default:
        return <Badge variant="outline">{status}</Badge>
    }
  }

  return (
    <div className="fixed inset-0 z-50 bg-slate-900/50 backdrop-blur-xs flex justify-end animate-in fade-in duration-200">
      <div className="w-full max-w-2xl bg-white dark:bg-slate-900 h-full shadow-2xl border-l border-slate-200 dark:border-slate-800 flex flex-col overflow-hidden animate-in slide-in-from-right duration-300">
        {/* Drawer Header */}
        <div className="p-6 border-b border-slate-100 dark:border-slate-800 flex items-center justify-between bg-slate-50/50 dark:bg-slate-900/50">
          <div className="space-y-1">
            <div className="flex items-center gap-2.5">
              <h2 className="text-xl font-bold text-slate-900 dark:text-white tracking-tight">
                {deployment.project_name}
              </h2>
              {getStatusBadge(deployment.status)}
            </div>
            <div className="flex items-center gap-2 flex-wrap">
              <p className="text-xs font-mono text-slate-400">ID: {deployment.id}</p>
              {deployment.server_id ? (
                <Badge variant="outline" className="border-indigo-400/60 bg-indigo-500/10 text-indigo-600 dark:text-indigo-400 text-[10px] gap-1 font-mono">
                  <Server className="h-3 w-3" /> Remote: {deployment.server_id}
                </Badge>
              ) : (
                <Badge variant="outline" className="border-emerald-400/60 bg-emerald-500/10 text-emerald-600 dark:text-emerald-400 text-[10px] gap-1 font-mono">
                  <Cpu className="h-3 w-3" /> Local Sandbox
                </Badge>
              )}
            </div>
          </div>
          <button
            onClick={onClose}
            className="p-2 text-slate-400 hover:text-slate-600 dark:hover:text-slate-200 rounded-lg hover:bg-slate-100 dark:hover:bg-slate-800 transition-colors"
          >
            <X className="h-5 w-5" />
          </button>
        </div>

        {/* Action Error Alert */}
        {actionError && (
          <div className="p-4 bg-red-50 dark:bg-red-950/40 border-b border-red-200 dark:border-red-900 text-red-700 dark:text-red-300 text-xs flex items-center gap-2">
            <AlertCircle className="h-4 w-4 shrink-0" />
            <span>{actionError}</span>
          </div>
        )}

        <div className="flex-1 overflow-y-auto p-6 space-y-6">
          {/* 1. Live Subdomain Link Bar */}
          <div className="p-4 bg-emerald-50/60 dark:bg-emerald-950/20 rounded-2xl border border-emerald-200 dark:border-emerald-800/60 flex items-center justify-between gap-3">
            <div className="min-w-0">
              <span className="text-[11px] font-medium text-emerald-800 dark:text-emerald-300 uppercase tracking-wider block">
                Live Subdomain URL
              </span>
              <a
                href={liveURL}
                target="_blank"
                rel="noreferrer"
                className="font-mono text-sm font-semibold text-emerald-700 dark:text-emerald-400 hover:underline flex items-center gap-1.5 truncate mt-0.5"
              >
                {liveURL}
                <ExternalLink className="h-3.5 w-3.5 shrink-0" />
              </a>
            </div>
            <Button
              variant="outline"
              size="sm"
              onClick={copyLiveURL}
              className="border-emerald-300 dark:border-emerald-700 hover:bg-emerald-100 dark:hover:bg-emerald-900 text-emerald-800 dark:text-emerald-200 text-xs shrink-0"
            >
              {copiedUrl ? (
                <>
                  <Check className="h-3.5 w-3.5 text-emerald-600 mr-1" /> Tersalin
                </>
              ) : (
                <>
                  <Copy className="h-3.5 w-3.5 mr-1" /> Salin URL
                </>
              )}
            </Button>
          </div>

          {/* Custom Domains Bar (S05.4) */}
          <div className="p-4 bg-slate-50 dark:bg-slate-800/40 rounded-2xl border border-slate-200 dark:border-slate-800 space-y-3">
            <div className="flex items-center justify-between">
              <div className="flex items-center gap-2">
                <Globe className="h-4 w-4 text-emerald-500" />
                <span className="text-xs font-bold text-slate-800 dark:text-slate-200 uppercase tracking-wider">
                  Custom Domains ({customDomains.length})
                </span>
                {customDomains.length > 0 && (
                  <Badge variant="outline" className="border-emerald-500/40 text-emerald-600 dark:text-emerald-400 text-[10px] py-0">
                    {customDomains.filter((cd) => cd.dns_status === 'verified').length} Verified
                  </Badge>
                )}
              </div>
              <Button
                variant="outline"
                size="sm"
                onClick={() => setIsCustomDomainsModalOpen(true)}
                className="h-7 text-xs font-semibold px-2.5 flex items-center gap-1 border-emerald-300 dark:border-emerald-700 text-emerald-700 dark:text-emerald-300 hover:bg-emerald-50 dark:hover:bg-emerald-950"
              >
                <Globe className="h-3 w-3" />
                <span>{customDomains.length === 0 ? 'Hubungkan Domain' : 'Kelola Domains'}</span>
              </Button>
            </div>

            {customDomains.length === 0 ? (
              <p className="text-[11px] text-slate-400">
                Belum ada custom domain terhubung. Hubungkan domain pribadi Anda (misal <code className="text-emerald-600 dark:text-emerald-400">app.brand.com</code>) dengan On-Demand TLS otomatis.
              </p>
            ) : (
              <div className="space-y-1.5">
                {customDomains.map((cd) => (
                  <div
                    key={cd.id}
                    className="p-2.5 bg-white dark:bg-slate-900 rounded-xl border border-slate-200 dark:border-slate-800 flex items-center justify-between text-xs"
                  >
                    <div className="flex items-center gap-2 min-w-0 pr-2">
                      <a
                        href={`https://${cd.domain}`}
                        target="_blank"
                        rel="noreferrer"
                        className="font-mono font-semibold text-slate-800 dark:text-slate-200 hover:text-emerald-600 dark:hover:text-emerald-400 flex items-center gap-1 truncate"
                      >
                        <span className="truncate">{cd.domain}</span>
                        <ExternalLink className="h-3 w-3 text-slate-400 shrink-0" />
                      </a>
                    </div>
                    <div className="flex items-center gap-1.5 shrink-0">
                      {cd.dns_status === 'verified' ? (
                        <Badge className="bg-emerald-500/10 text-emerald-600 dark:text-emerald-400 border-emerald-500/30 gap-1 text-[10px] font-mono py-0 px-1.5">
                          <CheckCircle2 className="h-2.5 w-2.5" /> Verified
                        </Badge>
                      ) : (
                        <Badge variant="outline" className="border-amber-400 text-amber-600 dark:text-amber-400 gap-1 text-[10px] font-mono py-0 px-1.5">
                          <Clock className="h-2.5 w-2.5" /> Pending DNS
                        </Badge>
                      )}
                      <Button
                        size="sm"
                        variant="ghost"
                        onClick={() => setIsCustomDomainsModalOpen(true)}
                        className="h-6 px-1.5 text-[10px] text-slate-500 hover:text-slate-800 dark:hover:text-slate-200"
                      >
                        Kelola
                      </Button>
                    </div>
                  </div>
                ))}
              </div>
            )}
          </div>

          {/* 2. One-Click Interactive Control Action Bar */}
          <div className="space-y-2">
            <label className="text-xs font-semibold text-slate-500 uppercase tracking-wider block">
              1-Click Runtime Controls
            </label>
            <div className="grid grid-cols-2 sm:grid-cols-5 gap-2">
              {/* Start Button */}
              <Button
                onClick={handleStart}
                disabled={actionLoading !== null || deployment.status === 'running'}
                className="bg-emerald-600 hover:bg-emerald-500 text-white font-semibold text-xs flex items-center gap-1.5 disabled:opacity-50"
              >
                {actionLoading === 'start' ? (
                  <RefreshCw className="h-3.5 w-3.5 animate-spin" />
                ) : (
                  <Play className="h-3.5 w-3.5 fill-current" />
                )}
                Start
              </Button>

              {/* Stop Button */}
              <Button
                variant="outline"
                onClick={handleStop}
                disabled={actionLoading !== null || deployment.status === 'stopped'}
                className="border-amber-300 dark:border-amber-700 text-amber-700 dark:text-amber-300 hover:bg-amber-50 dark:hover:bg-amber-950 font-semibold text-xs flex items-center gap-1.5 disabled:opacity-50"
              >
                {actionLoading === 'stop' ? (
                  <RefreshCw className="h-3.5 w-3.5 animate-spin" />
                ) : (
                  <Square className="h-3.5 w-3.5 fill-current" />
                )}
                Stop
              </Button>

              {/* Restart Button */}
              <Button
                variant="outline"
                onClick={handleRestart}
                disabled={actionLoading !== null}
                className="border-blue-300 dark:border-blue-700 text-blue-700 dark:text-blue-300 hover:bg-blue-50 dark:hover:bg-blue-950 font-semibold text-xs flex items-center gap-1.5 disabled:opacity-50"
              >
                {actionLoading === 'restart' ? (
                  <RefreshCw className="h-3.5 w-3.5 animate-spin" />
                ) : (
                  <RotateCcw className="h-3.5 w-3.5" />
                )}
                Restart
              </Button>

              {/* Custom Domains Button (S05.4) */}
              <Button
                variant="outline"
                onClick={() => setIsCustomDomainsModalOpen(true)}
                className="border-emerald-300 dark:border-emerald-700 text-emerald-700 dark:text-emerald-300 hover:bg-emerald-50 dark:hover:bg-emerald-950 font-semibold text-xs flex items-center gap-1"
                title="Kelola Custom Domains"
              >
                <Globe className="h-3.5 w-3.5" />
                <span>Domains</span>
              </Button>

              {/* Delete Button */}
              <Button
                variant="outline"
                onClick={() => setShowDeleteConfirm(true)}
                disabled={actionLoading !== null}
                className="border-red-300 dark:border-red-800 text-red-600 dark:text-red-400 hover:bg-red-50 dark:hover:bg-red-950 font-semibold text-xs flex items-center gap-1.5 disabled:opacity-50"
              >
                <Trash2 className="h-3.5 w-3.5" />
                Delete
              </Button>
            </div>
          </div>

          {/* Delete Confirmation Modal / Inline */}
          {showDeleteConfirm && (
            <div className="p-4 bg-red-50 dark:bg-red-950/40 rounded-2xl border border-red-200 dark:border-red-900 space-y-3">
              <div className="flex items-center gap-2 text-red-800 dark:text-red-300 text-sm font-semibold">
                <AlertCircle className="h-4 w-4" />
                Konfirmasi Hapus Aplikasi
              </div>
              <p className="text-xs text-red-700 dark:text-red-400">
                Aplikasi <strong>{deployment.project_name}</strong> akan dihentikan dan seluruh berkas
                serta rute Caddy akan dihapus permanen.
              </p>
              <div className="flex items-center gap-2 pt-1">
                <Button
                  size="sm"
                  variant="outline"
                  onClick={() => setShowDeleteConfirm(false)}
                  disabled={actionLoading === 'delete'}
                  className="text-xs"
                >
                  Batal
                </Button>
                <Button
                  size="sm"
                  onClick={handleDelete}
                  disabled={actionLoading === 'delete'}
                  className="bg-red-600 hover:bg-red-500 text-white text-xs font-semibold"
                >
                  {actionLoading === 'delete' ? 'Menghapus...' : 'Ya, Hapus Permanen'}
                </Button>
              </div>
            </div>
          )}

          {/* 3. Specs & Process Details Card */}
          <div className="grid grid-cols-2 sm:grid-cols-5 gap-3 p-4 bg-slate-50 dark:bg-slate-800/40 rounded-2xl border border-slate-200 dark:border-slate-800">
            <div>
              <span className="text-[11px] text-slate-400 uppercase tracking-wider block">Stack Profile</span>
              <div className="mt-1">{getRuntimeBadge(deployment.runtime)}</div>
            </div>
            <div>
              <span className="text-[11px] text-slate-400 uppercase tracking-wider block">Target Node</span>
              <div className="mt-1">
                {deployment.server_id ? (
                  <Badge variant="outline" className="border-indigo-400 text-indigo-600 dark:text-indigo-400 gap-1 text-xs">
                    <Server className="h-3 w-3" /> Remote VPS
                  </Badge>
                ) : (
                  <Badge variant="outline" className="border-emerald-400 text-emerald-600 dark:text-emerald-400 gap-1 text-xs">
                    <Cpu className="h-3 w-3" /> Local Sandbox
                  </Badge>
                )}
              </div>
            </div>
            <div>
              <span className="text-[11px] text-slate-400 uppercase tracking-wider block">Internal Port</span>
              <span className="text-xs font-mono font-semibold text-slate-700 dark:text-slate-200 mt-1 block">
                {deployment.internal_port > 0 ? `:${deployment.internal_port}` : 'None'}
              </span>
            </div>
            <div>
              <span className="text-[11px] text-slate-400 uppercase tracking-wider block">Process PID</span>
              <span className="text-xs font-mono font-semibold text-slate-700 dark:text-slate-200 mt-1 block">
                {deployment.process_pid > 0
                  ? `${deployment.process_pid}${health?.alive ? ' (Alive)' : ''}`
                  : 'Stopped'}
              </span>
            </div>
            <div>
              <span className="text-[11px] text-slate-400 uppercase tracking-wider block">Source Type</span>
              <span className="text-xs font-semibold text-slate-700 dark:text-slate-200 flex items-center gap-1 mt-1">
                {deployment.source_type === 'git' ? (
                  <>
                    <FolderGit2 className="h-3 w-3 text-blue-500" /> Git Repo
                  </>
                ) : (
                  <>
                    <UploadCloud className="h-3 w-3 text-emerald-500" /> ZIP Archive
                  </>
                )}
              </span>
            </div>
          </div>

          {/* Environment Variables Viewer (if present) */}
          {deployment.env_vars && Object.keys(deployment.env_vars).length > 0 && (
            <div className="space-y-2">
              <label className="text-xs font-semibold text-slate-500 uppercase tracking-wider block">
                Environment Variables
              </label>
              <div className="p-3 bg-slate-100 dark:bg-slate-800 rounded-xl space-y-1 font-mono text-xs">
                {Object.entries(deployment.env_vars).map(([k, v]) => (
                  <div key={k} className="flex items-center gap-2">
                    <span className="text-slate-500">{k}:</span>
                    <span className="text-slate-900 dark:text-slate-100 font-semibold">{String(v)}</span>
                  </div>
                ))}
              </div>
            </div>
          )}

          {/* 4. Live Streaming Process Logs */}
          <div className="space-y-2">
            <div className="flex items-center justify-between">
              <div className="flex items-center gap-2">
                <Terminal className="h-4 w-4 text-emerald-500" />
                <h3 className="text-xs font-bold uppercase tracking-wider text-slate-700 dark:text-slate-200">
                  Live Streaming Logs
                </h3>
                {deployment.status === 'running' && (
                  <span className="flex items-center gap-1 text-[11px] font-mono text-emerald-600 dark:text-emerald-400">
                    <Radio className="h-3 w-3 animate-pulse" /> Live
                  </span>
                )}
              </div>

              <div className="flex items-center gap-2">
                <Button
                  variant="outline"
                  size="sm"
                  onClick={fetchLogs}
                  className="h-7 text-[11px] px-2 flex items-center gap-1"
                >
                  <RefreshCw className={`h-3 w-3 ${logsLoading ? 'animate-spin' : ''}`} /> Refresh
                </Button>
                <Button
                  variant="outline"
                  size="sm"
                  onClick={copyAllLogs}
                  className="h-7 text-[11px] px-2 flex items-center gap-1"
                >
                  {copiedLogs ? <Check className="h-3 w-3 text-emerald-500" /> : <Copy className="h-3 w-3" />}
                  Salin
                </Button>
              </div>
            </div>

            {/* Stream Filter Switcher */}
            <div className="flex items-center justify-between">
              <div className="flex rounded-lg bg-slate-100 dark:bg-slate-800 p-0.5 text-[11px]">
                {(['all', 'stdout', 'stderr', 'system'] as const).map((filter) => (
                  <button
                    key={filter}
                    onClick={() => setStreamFilter(filter)}
                    className={`px-2.5 py-1 rounded-md font-semibold transition-all ${
                      streamFilter === filter
                        ? 'bg-white dark:bg-slate-900 text-slate-900 dark:text-white shadow-xs'
                        : 'text-slate-500 hover:text-slate-900 dark:text-slate-400'
                    }`}
                  >
                    {filter.toUpperCase()}
                  </button>
                ))}
              </div>

              <label className="flex items-center gap-1.5 text-[11px] text-slate-500 cursor-pointer">
                <input
                  type="checkbox"
                  checked={autoScroll}
                  onChange={(e) => setAutoScroll(e.target.checked)}
                  className="rounded text-emerald-600 focus:ring-emerald-500"
                />
                Auto-scroll tail
              </label>
            </div>

            {/* Terminal Window */}
            <div
              ref={logContainerRef}
              className="h-72 bg-slate-950 text-slate-100 font-mono text-xs rounded-xl p-4 overflow-y-auto space-y-1.5 shadow-inner border border-slate-800"
            >
              {filteredLogs.length === 0 ? (
                <div className="h-full flex items-center justify-center text-slate-500 italic">
                  {logsLoading ? 'Memuat live logs...' : 'Belum ada log tercatat untuk deployment ini.'}
                </div>
              ) : (
                filteredLogs.map((log) => {
                  let badgeColor = 'text-blue-400 bg-blue-950/60'
                  if (log.stream === 'stdout') badgeColor = 'text-emerald-400 bg-emerald-950/60'
                  if (log.stream === 'stderr') badgeColor = 'text-red-400 bg-red-950/60'

                  return (
                    <div key={log.id} className="leading-relaxed flex items-start gap-2 hover:bg-slate-900/60 px-1 rounded">
                      <span className="text-[10px] text-slate-500 shrink-0 select-none">
                        {log.timestamp.split('T')[1]?.replace('Z', '') || log.timestamp}
                      </span>
                      <span className={`text-[10px] font-bold px-1.5 py-0.2 rounded shrink-0 ${badgeColor}`}>
                        {log.stream}
                      </span>
                      <span className="break-all whitespace-pre-wrap">{log.message}</span>
                    </div>
                  )
                })
              )}
            </div>
          </div>
        </div>
      </div>

      {/* Custom Domains Modal (S05.4) */}
      <CustomDomainsModal
        deployment={deployment}
        isOpen={isCustomDomainsModalOpen}
        onClose={() => {
          setIsCustomDomainsModalOpen(false)
          fetchCustomDomains()
        }}
        baseDomain={baseDomain}
        onDomainsUpdated={fetchCustomDomains}
      />
    </div>
  )
}

