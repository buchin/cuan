import { useState, useEffect, useMemo } from 'react'
import { Card, CardHeader } from '../ui/card'
import { Button } from '../ui/button'
import { Input } from '../ui/input'
import { Badge } from '../ui/badge'
import { Alert, AlertTitle, AlertDescription } from '../ui/alert'
import {
  Server,
  Plus,
  RefreshCw,
  Search,
  AlertCircle,
  Activity,
  Trash2,
  Key,
  Cpu,
  HardDrive,
  Copy,
  Check,
  ExternalLink,
  Clock,
  Terminal,
  Zap
} from 'lucide-react'
import { ServerItem, TestServerResult } from './types'
import { ConnectServerModal } from './ConnectServerModal'

interface ServersViewProps {
  onNavigate?: (path: string) => void
  baseDomain?: string
}

export function ServersView({ onNavigate: _onNavigate, baseDomain: _baseDomain }: ServersViewProps) {
  const [servers, setServers] = useState<ServerItem[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [notification, setNotification] = useState<{
    type: 'success' | 'error'
    message: string
  } | null>(null)

  // Filters & Search
  const [searchQuery, setSearchQuery] = useState('')
  const [statusFilter, setStatusFilter] = useState<'all' | 'online' | 'offline' | 'error'>('all')

  // Modals
  const [isConnectModalOpen, setIsConnectModalOpen] = useState(false)
  const [deleteTarget, setDeleteTarget] = useState<ServerItem | null>(null)

  // Per-server Action states
  const [actionLoadingId, setActionLoadingId] = useState<string | null>(null)
  const [actionType, setActionType] = useState<'test' | 'resync' | 'delete' | null>(null)
  const [copiedId, setCopiedId] = useState<string | null>(null)

  const showToast = (type: 'success' | 'error', message: string) => {
    setNotification({ type, message })
    setTimeout(() => setNotification(null), 4000)
  }

  const fetchServers = async () => {
    setError(null)
    try {
      const res = await fetch('/api/servers')
      if (!res.ok) {
        throw new Error(`Gagal memuat remote server (HTTP ${res.status})`)
      }
      const data = await res.json()
      setServers(data.servers || [])
    } catch (err: any) {
      setError(err.message || 'Terjadi kesalahan jaringan saat memuat daftar server.')
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    fetchServers()
    const interval = setInterval(fetchServers, 8000)
    return () => clearInterval(interval)
  }, [])

  // Format relative time
  const timeAgo = (dateStr?: string): string => {
    if (!dateStr) return 'Belum pernah'
    try {
      const diffSec = Math.floor((new Date().getTime() - new Date(dateStr).getTime()) / 1000)
      if (diffSec < 60) return `${diffSec} detik lalu`
      if (diffSec < 3600) return `${Math.floor(diffSec / 60)} menit lalu`
      if (diffSec < 86400) return `${Math.floor(diffSec / 3600)} jam lalu`
      return `${Math.floor(diffSec / 86400)} hari lalu`
    } catch {
      return dateStr
    }
  }

  // Copy helper
  const copyFingerprint = (id: string, fp: string) => {
    navigator.clipboard.writeText(fp)
    setCopiedId(id)
    setTimeout(() => setCopiedId(null), 2000)
  }

  // Action: Test Connection (POST /api/servers/{id}/test)
  const handleTestConnection = async (srv: ServerItem) => {
    setActionLoadingId(srv.id)
    setActionType('test')
    try {
      const res = await fetch(`/api/servers/${srv.id}/test`, { method: 'POST' })
      const data: TestServerResult = await res.json()

      if (data.success) {
        showToast(
          'success',
          `Server "${srv.name}" online! Ping: ${data.specs?.ping_ms || 1}ms (${data.specs?.cores || 1} Cores, ${data.specs?.ram_mb || 0} MB RAM)`
        )
      } else {
        showToast('error', `Tes gagal untuk "${srv.name}": ${data.error || 'Host tidak merespon'}`)
      }

      // Update local state with latest server data
      setServers((prev) =>
        prev.map((item) => (item.id === srv.id ? { ...item, ...data.server } : item))
      )
    } catch (err: any) {
      showToast('error', `Koneksi gagal: ${err.message}`)
    } finally {
      setActionLoadingId(null)
      setActionType(null)
    }
  }

  // Action: Re-sync Fingerprint (POST /api/servers/{id}/test)
  const handleResyncFingerprint = async (srv: ServerItem) => {
    setActionLoadingId(srv.id)
    setActionType('resync')
    try {
      const res = await fetch(`/api/servers/${srv.id}/test`, { method: 'POST' })
      const data: TestServerResult = await res.json()

      if (data.success) {
        showToast(
          'success',
          `Fingerprint & spesifikasi hardware "${srv.name}" berhasil di-resinkronisasi.`
        )
      } else {
        showToast('error', `Resync gagal: ${data.error || 'Tidak dapat menghubungi host'}`)
      }

      setServers((prev) =>
        prev.map((item) => (item.id === srv.id ? { ...item, ...data.server } : item))
      )
    } catch (err: any) {
      showToast('error', `Gagal resync: ${err.message}`)
    } finally {
      setActionLoadingId(null)
      setActionType(null)
    }
  }

  // Action: Delete Server (DELETE /api/servers/{id})
  const handleDeleteServer = async () => {
    if (!deleteTarget) return
    setActionLoadingId(deleteTarget.id)
    setActionType('delete')

    try {
      const res = await fetch(`/api/servers/${deleteTarget.id}`, { method: 'DELETE' })
      if (!res.ok) {
        const err = await res.json().catch(() => ({}))
        throw new Error(err.error || `HTTP ${res.status}`)
      }

      setServers((prev) => prev.filter((item) => item.id !== deleteTarget.id))
      showToast('success', `Server "${deleteTarget.name}" berhasil dihapus.`)
      setDeleteTarget(null)
    } catch (err: any) {
      showToast('error', `Gagal menghapus server: ${err.message}`)
    } finally {
      setActionLoadingId(null)
      setActionType(null)
    }
  }

  // Filtered servers
  const filteredServers = useMemo(() => {
    return servers.filter((srv) => {
      // Status filter
      if (statusFilter === 'online' && srv.status !== 'online') return false
      if (statusFilter === 'offline' && srv.status !== 'offline') return false
      if (statusFilter === 'error' && srv.status !== 'error') return false

      // Search query
      if (searchQuery.trim()) {
        const q = searchQuery.toLowerCase()
        const matchName = srv.name.toLowerCase().includes(q)
        const matchHost = srv.host.toLowerCase().includes(q)
        const matchUser = srv.username.toLowerCase().includes(q)
        const matchFp = (srv.key_fingerprint || '').toLowerCase().includes(q)
        if (!matchName && !matchHost && !matchUser && !matchFp) return false
      }
      return true
    })
  }, [servers, statusFilter, searchQuery])

  // Statistics
  const stats = useMemo(() => {
    const total = servers.length
    const online = servers.filter((s) => s.status === 'online').length
    const offline = servers.filter((s) => s.status === 'offline').length
    const errors = servers.filter((s) => s.status === 'error').length

    let totalCores = 0
    let totalRAMMB = 0

    servers.forEach((s) => {
      if (s.status === 'online' && s.os_info) {
        totalCores += s.os_info.cores || 0
        totalRAMMB += s.os_info.ram_mb || 0
      }
    })

    return {
      total,
      online,
      offline,
      errors,
      totalCores,
      totalRAMGB: (totalRAMMB / 1024).toFixed(1)
    }
  }, [servers])

  return (
    <div className="space-y-6">
      {/* Toast Notification */}
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

      {/* View Header Banner */}
      <div className="p-5 rounded-2xl bg-gradient-to-r from-slate-900 via-indigo-950/70 to-slate-900 border border-indigo-500/30 text-white shadow-lg flex flex-col sm:flex-row items-start sm:items-center justify-between gap-4">
        <div className="flex items-start gap-3.5">
          <div className="h-10 w-10 rounded-xl bg-indigo-500/20 border border-indigo-500/40 text-indigo-400 flex items-center justify-center shrink-0 mt-0.5">
            <Server className="h-5 w-5" />
          </div>
          <div className="space-y-1">
            <div className="flex items-center gap-2 flex-wrap">
              <h2 className="text-lg font-bold tracking-tight">Remote Servers Hub (BYOS)</h2>
              <Badge variant="outline" className="border-indigo-400/50 bg-indigo-500/10 text-indigo-300 text-[10px] font-mono">
                Sprint 4 Orchestrator
              </Badge>
            </div>
            <p className="text-xs text-slate-300 max-w-2xl leading-relaxed">
              Kelola node server remote (VPS) Anda sendiri. Hubungkan instance Ubuntu/Debian melalui SSH terenkripsi untuk target deployment terisolasi dengan driver eksekusi remote.
            </p>
          </div>
        </div>

        <div className="flex items-center gap-2 w-full sm:w-auto shrink-0">
          <Button
            variant="outline"
            size="sm"
            onClick={fetchServers}
            className="h-9 text-xs flex items-center gap-1.5 border-slate-700 bg-slate-800/40 text-slate-200 hover:bg-slate-800"
          >
            <RefreshCw className={`h-3.5 w-3.5 ${loading ? 'animate-spin' : ''}`} />
            <span className="hidden sm:inline">Refresh</span>
          </Button>

          <Button
            onClick={() => setIsConnectModalOpen(true)}
            className="h-9 bg-indigo-600 hover:bg-indigo-500 text-white font-semibold text-xs shadow-md shadow-indigo-600/20 flex items-center gap-1.5"
          >
            <Plus className="h-4 w-4" />
            <span>Connect Server</span>
          </Button>
        </div>
      </div>

      {/* Error Alert */}
      {error && (
        <Alert variant="destructive">
          <AlertCircle className="h-4 w-4" />
          <AlertTitle>Kesalahan</AlertTitle>
          <AlertDescription>{error}</AlertDescription>
        </Alert>
      )}

      {/* 1. Statistics Cards */}
      <div className="grid grid-cols-2 sm:grid-cols-4 gap-4">
        <Card className="p-4 bg-white dark:bg-slate-900 border-slate-200 dark:border-slate-800 shadow-xs">
          <div className="flex items-center justify-between">
            <div>
              <p className="text-xs font-medium text-slate-500 dark:text-slate-400">Total Remote Servers</p>
              <p className="text-2xl font-bold tracking-tight text-slate-900 dark:text-white mt-0.5">
                {stats.total}
              </p>
            </div>
            <div className="h-10 w-10 rounded-xl bg-slate-100 dark:bg-slate-800 text-slate-600 dark:text-slate-300 flex items-center justify-center">
              <Server className="h-5 w-5" />
            </div>
          </div>
        </Card>

        <Card className="p-4 bg-white dark:bg-slate-900 border-slate-200 dark:border-slate-800 shadow-xs">
          <div className="flex items-center justify-between">
            <div>
              <p className="text-xs font-medium text-slate-500 dark:text-slate-400">Online & Siap Pakai</p>
              <div className="flex items-center gap-2 mt-0.5">
                <p className="text-2xl font-bold tracking-tight text-emerald-600 dark:text-emerald-400">
                  {stats.online}
                </p>
                {stats.online > 0 && <span className="h-2.5 w-2.5 rounded-full bg-emerald-500 animate-pulse" />}
              </div>
            </div>
            <div className="h-10 w-10 rounded-xl bg-emerald-100 dark:bg-emerald-950/60 text-emerald-600 dark:text-emerald-400 flex items-center justify-center">
              <Activity className="h-5 w-5" />
            </div>
          </div>
        </Card>

        <Card className="p-4 bg-white dark:bg-slate-900 border-slate-200 dark:border-slate-800 shadow-xs">
          <div className="flex items-center justify-between">
            <div>
              <p className="text-xs font-medium text-slate-500 dark:text-slate-400">Kapasitas CPU Aktif</p>
              <p className="text-2xl font-bold tracking-tight text-indigo-600 dark:text-indigo-400 mt-0.5">
                {stats.totalCores} <span className="text-xs font-normal text-slate-400">vCPU</span>
              </p>
            </div>
            <div className="h-10 w-10 rounded-xl bg-indigo-100 dark:bg-indigo-950/60 text-indigo-600 dark:text-indigo-400 flex items-center justify-center">
              <Cpu className="h-5 w-5" />
            </div>
          </div>
        </Card>

        <Card className="p-4 bg-white dark:bg-slate-900 border-slate-200 dark:border-slate-800 shadow-xs">
          <div className="flex items-center justify-between">
            <div>
              <p className="text-xs font-medium text-slate-500 dark:text-slate-400">Kapasitas RAM Online</p>
              <p className="text-2xl font-bold tracking-tight text-blue-600 dark:text-blue-400 mt-0.5">
                {stats.totalRAMGB} <span className="text-xs font-normal text-slate-400">GB</span>
              </p>
            </div>
            <div className="h-10 w-10 rounded-xl bg-blue-100 dark:bg-blue-950/60 text-blue-600 dark:text-blue-400 flex items-center justify-center">
              <HardDrive className="h-5 w-5" />
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
            placeholder="Cari nama, host, atau IP..."
            value={searchQuery}
            onChange={(e) => setSearchQuery(e.target.value)}
            className="pl-9 h-9 text-xs bg-slate-50 dark:bg-slate-800/50"
          />
        </div>

        <div className="flex items-center gap-2 w-full sm:w-auto">
          <div className="flex rounded-lg bg-slate-100 dark:bg-slate-800 p-0.5 text-xs">
            {(['all', 'online', 'offline', 'error'] as const).map((st) => (
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

      {/* 3. Server List / Empty State */}
      {loading && servers.length === 0 ? (
        <div className="h-64 flex flex-col items-center justify-center gap-3 bg-white dark:bg-slate-900 rounded-2xl border border-slate-200 dark:border-slate-800">
          <RefreshCw className="h-8 w-8 text-indigo-500 animate-spin" />
          <p className="text-sm text-slate-500">Memuat daftar remote server...</p>
        </div>
      ) : servers.length === 0 ? (
        /* Empty State */
        <div className="rounded-3xl border border-dashed border-indigo-500/40 bg-gradient-to-b from-indigo-950/20 via-slate-900/60 to-slate-900/80 p-8 sm:p-12 text-center space-y-6 shadow-xl">
          <div className="h-16 w-16 rounded-2xl bg-indigo-500/20 text-indigo-400 border border-indigo-500/30 flex items-center justify-center mx-auto shadow-md">
            <Server className="h-8 w-8" />
          </div>

          <div className="max-w-xl mx-auto space-y-2">
            <Badge className="bg-indigo-500/10 text-indigo-400 border-indigo-500/30 text-xs py-0.5 px-2.5 font-semibold">
              Bring Your Own Server (BYOS)
            </Badge>
            <h3 className="text-xl sm:text-2xl font-bold text-white tracking-tight">
              Belum ada Remote Server yang terhubung
            </h3>
            <p className="text-xs sm:text-sm text-slate-400 leading-relaxed">
              Hubungkan VPS Ubuntu atau Debian Anda untuk mengeksekusi aplikasi Cuan langsung di infrastruktur sendiri. Deployer akan melakukan SSH connection pooling, live log streaming, dan process monitoring otomatis.
            </p>
          </div>

          <div>
            <Button
              onClick={() => setIsConnectModalOpen(true)}
              className="h-11 px-6 bg-indigo-600 hover:bg-indigo-500 text-white font-semibold text-xs shadow-lg shadow-indigo-600/25 gap-2"
            >
              <Plus className="h-4 w-4" />
              <span>Hubungkan Server Pertama Anda</span>
            </Button>
          </div>
        </div>
      ) : filteredServers.length === 0 ? (
        <div className="h-64 flex flex-col items-center justify-center text-center p-8 bg-white dark:bg-slate-900 rounded-2xl border border-slate-200 dark:border-slate-800 space-y-3">
          <p className="text-sm font-semibold text-slate-700 dark:text-slate-200">
            Tidak ada server yang sesuai dengan filter.
          </p>
          <Button
            variant="outline"
            size="sm"
            onClick={() => {
              setSearchQuery('')
              setStatusFilter('all')
            }}
            className="text-xs"
          >
            Reset Filter
          </Button>
        </div>
      ) : (
        /* Server Cards Grid */
        <div className="grid grid-cols-1 lg:grid-cols-2 gap-5">
          {filteredServers.map((srv) => {
            const isActionLoading = actionLoadingId === srv.id
            const isOnline = srv.status === 'online'
            const isError = srv.status === 'error'

            return (
              <Card
                key={srv.id}
                className="bg-white dark:bg-slate-900 border-slate-200 dark:border-slate-800 shadow-xs hover:shadow-md transition-all flex flex-col justify-between overflow-hidden"
              >
                <CardHeader className="p-5 pb-3 space-y-3">
                  {/* Top Bar: Name, Host, and Status */}
                  <div className="flex items-start justify-between gap-2">
                    <div className="space-y-1 min-w-0">
                      <div className="flex items-center gap-2">
                        <h3 className="font-bold text-slate-900 dark:text-white text-base truncate">
                          {srv.name}
                        </h3>
                      </div>
                      <p className="text-xs font-mono text-slate-500 dark:text-slate-400 flex items-center gap-1.5">
                        <Terminal className="h-3 w-3 text-indigo-500 shrink-0" />
                        <span>{srv.username}@{srv.host}:{srv.port}</span>
                      </p>
                    </div>

                    {/* Status Badge */}
                    {isOnline ? (
                      <Badge variant="success" className="gap-1 py-0.5 px-2 text-[11px] shrink-0">
                        <span className="h-1.5 w-1.5 rounded-full bg-white animate-pulse" />
                        Online
                      </Badge>
                    ) : isError ? (
                      <Badge variant="destructive" className="gap-1 py-0.5 px-2 text-[11px] shrink-0">
                        <AlertCircle className="h-3 w-3" />
                        Error
                      </Badge>
                    ) : (
                      <Badge variant="secondary" className="gap-1 py-0.5 px-2 text-[11px] shrink-0">
                        <span className="h-1.5 w-1.5 rounded-full bg-slate-400" />
                        {srv.status}
                      </Badge>
                    )}
                  </div>

                  {/* Hardware & OS Specs Grid */}
                  <div className="grid grid-cols-2 sm:grid-cols-4 gap-2 p-3 bg-slate-50 dark:bg-slate-800/50 rounded-xl border border-slate-100 dark:border-slate-800 text-xs">
                    <div>
                      <span className="text-[10px] text-slate-400 uppercase tracking-wider block">Distro / OS</span>
                      <span className="font-semibold text-slate-700 dark:text-slate-200 truncate block mt-0.5">
                        {srv.os_info?.distro || srv.os_info?.os || 'Linux'}
                      </span>
                    </div>

                    <div>
                      <span className="text-[10px] text-slate-400 uppercase tracking-wider block">CPU Cores</span>
                      <span className="font-semibold text-slate-700 dark:text-slate-200 block mt-0.5">
                        {srv.os_info?.cores ? `${srv.os_info.cores} vCPU` : '-'}
                      </span>
                    </div>

                    <div>
                      <span className="text-[10px] text-slate-400 uppercase tracking-wider block">RAM Total</span>
                      <span className="font-semibold text-slate-700 dark:text-slate-200 block mt-0.5">
                        {srv.os_info?.ram_mb ? `${(srv.os_info.ram_mb / 1024).toFixed(1)} GB` : '-'}
                      </span>
                    </div>

                    <div>
                      <span className="text-[10px] text-slate-400 uppercase tracking-wider block">Latency Ping</span>
                      <span className={`font-semibold block mt-0.5 ${isOnline ? 'text-emerald-600 dark:text-emerald-400' : 'text-slate-400'}`}>
                        {srv.os_info?.ping_ms ? `${srv.os_info.ping_ms} ms` : '-'}
                      </span>
                    </div>
                  </div>

                  {/* Fingerprint & Last Ping row */}
                  <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-2 pt-1 text-[11px] text-slate-400 border-t border-slate-100 dark:border-slate-800/80">
                    <div className="flex items-center gap-1.5 truncate">
                      <Key className="h-3.5 w-3.5 text-slate-400 shrink-0" />
                      <span className="font-mono truncate" title={srv.key_fingerprint || ''}>
                        {srv.key_fingerprint ? srv.key_fingerprint.slice(0, 24) + '...' : 'No fingerprint'}
                      </span>
                      {srv.key_fingerprint && (
                        <button
                          onClick={() => copyFingerprint(srv.id, srv.key_fingerprint!)}
                          className="text-slate-400 hover:text-slate-600 dark:hover:text-slate-200 p-0.5"
                          title="Salin Key Fingerprint"
                        >
                          {copiedId === srv.id ? (
                            <Check className="h-3 w-3 text-emerald-500" />
                          ) : (
                            <Copy className="h-3 w-3" />
                          )}
                        </button>
                      )}
                    </div>

                    <div className="flex items-center gap-1 shrink-0 text-slate-400">
                      <Clock className="h-3 w-3" />
                      <span>Last ping: {timeAgo(srv.last_ping_at)}</span>
                    </div>
                  </div>

                  {/* Error detail if present */}
                  {srv.os_info?.error && (
                    <div className="p-2.5 bg-red-50 dark:bg-red-950/30 rounded-lg border border-red-200 dark:border-red-900 text-xs text-red-700 dark:text-red-300 font-mono">
                      {srv.os_info.error}
                    </div>
                  )}
                </CardHeader>

                {/* Footer Actions */}
                <div className="p-4 pt-3 border-t border-slate-100 dark:border-slate-800/80 bg-slate-50/50 dark:bg-slate-900/50 flex items-center justify-between gap-2">
                  <div className="flex items-center gap-2">
                    {/* Test Connection Button */}
                    <Button
                      size="sm"
                      variant="outline"
                      onClick={() => handleTestConnection(srv)}
                      disabled={isActionLoading}
                      className="h-8 text-xs font-semibold border-indigo-200 dark:border-indigo-800 text-indigo-700 dark:text-indigo-300 hover:bg-indigo-50 dark:hover:bg-indigo-950 flex items-center gap-1.5"
                    >
                      {isActionLoading && actionType === 'test' ? (
                        <RefreshCw className="h-3 w-3 animate-spin" />
                      ) : (
                        <Activity className="h-3 w-3" />
                      )}
                      <span>{isActionLoading && actionType === 'test' ? 'Testing...' : 'Test Connection'}</span>
                    </Button>

                    {/* Re-sync Fingerprint Button */}
                    <Button
                      size="sm"
                      variant="outline"
                      onClick={() => handleResyncFingerprint(srv)}
                      disabled={isActionLoading}
                      className="h-8 text-xs font-semibold text-slate-600 dark:text-slate-300 hover:bg-slate-100 dark:hover:bg-slate-800 flex items-center gap-1.5"
                      title="Re-probe host and refresh SSH fingerprint & system specs"
                    >
                      {isActionLoading && actionType === 'resync' ? (
                        <RefreshCw className="h-3 w-3 animate-spin" />
                      ) : (
                        <Zap className="h-3 w-3 text-amber-500" />
                      )}
                      <span className="hidden sm:inline">Re-sync</span>
                    </Button>
                  </div>

                  {/* Delete Button */}
                  <Button
                    size="sm"
                    variant="outline"
                    onClick={() => setDeleteTarget(srv)}
                    disabled={isActionLoading}
                    className="h-8 text-xs border-red-200 dark:border-red-900 text-red-600 dark:text-red-400 hover:bg-red-50 dark:hover:bg-red-950 flex items-center gap-1"
                    title="Hapus server ini"
                  >
                    <Trash2 className="h-3 w-3" />
                    <span className="hidden sm:inline">Delete</span>
                  </Button>
                </div>
              </Card>
            )
          })}
        </div>
      )}

      {/* Delete Confirmation Modal */}
      {deleteTarget && (
        <div className="fixed inset-0 z-50 bg-slate-900/60 backdrop-blur-xs flex items-center justify-center p-4 animate-in fade-in duration-200">
          <div className="w-full max-w-md bg-white dark:bg-slate-900 rounded-2xl border border-slate-200 dark:border-slate-800 p-6 space-y-4 shadow-2xl">
            <div className="flex items-center gap-3 text-red-600 dark:text-red-400">
              <div className="h-10 w-10 rounded-xl bg-red-100 dark:bg-red-950/60 flex items-center justify-center">
                <Trash2 className="h-5 w-5" />
              </div>
              <div>
                <h3 className="font-bold text-base text-slate-900 dark:text-white">
                  Hapus Remote Server?
                </h3>
                <p className="text-xs text-slate-500">Tindakan ini tidak dapat dibatalkan.</p>
              </div>
            </div>

            <p className="text-xs text-slate-600 dark:text-slate-300 leading-relaxed">
              Server <strong>{deleteTarget.name}</strong> ({deleteTarget.host}) akan dihapus dari Cuan. Seluruh deployment yang terikat ke server ini akan diputus dan connection pool akan ditutup.
            </p>

            <div className="flex items-center justify-end gap-2 pt-2 border-t border-slate-100 dark:border-slate-800">
              <Button
                variant="outline"
                size="sm"
                onClick={() => setDeleteTarget(null)}
                disabled={actionLoadingId !== null}
                className="text-xs"
              >
                Batal
              </Button>
              <Button
                size="sm"
                onClick={handleDeleteServer}
                disabled={actionLoadingId !== null}
                className="bg-red-600 hover:bg-red-500 text-white font-semibold text-xs"
              >
                {actionLoadingId === deleteTarget.id ? 'Menghapus...' : 'Ya, Hapus Server'}
              </Button>
            </div>
          </div>
        </div>
      )}

      {/* Connect Server Modal */}
      <ConnectServerModal
        isOpen={isConnectModalOpen}
        onClose={() => setIsConnectModalOpen(false)}
        onSuccess={(newServer) => {
          setIsConnectModalOpen(false)
          setServers((prev) => [newServer, ...prev.filter((s) => s.id !== newServer.id)])
          showToast('success', `Server "${newServer.name}" berhasil terhubung!`)
        }}
      />
    </div>
  )
}
