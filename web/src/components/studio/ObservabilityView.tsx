import { useState, useEffect, useMemo } from 'react'
import { Card } from '../ui/card'
import { Button } from '../ui/button'
import { Input } from '../ui/input'
import { Badge } from '../ui/badge'
import {
  Activity,
  HeartPulse,
  AlertTriangle,
  CheckCircle2,
  XCircle,
  RefreshCw,
  Search,
  ExternalLink,
  Bell,
  Clock,
  Cpu,
  HardDrive,
  Server,
  Layers,
  Radio,
  Zap,
  TrendingUp,
  AlertCircle
} from 'lucide-react'
import {
  DeploymentItem,
  ServerItem,
  DeploymentHealthResponse,
  ServerMetricDTO,
  AlertIncidentDTO
} from './types'
import { AlertChannelsModal } from './AlertChannelsModal'

interface ObservabilityViewProps {
  onNavigate?: (path: string) => void
  baseDomain?: string
  deployments?: DeploymentItem[]
  onOpenDrawer?: (dep: DeploymentItem) => void
}

export function ObservabilityView({
  baseDomain = 'cuan.dojo.cc',
  deployments: initialDeployments = [],
  onOpenDrawer,
}: ObservabilityViewProps) {
  // Deployments state
  const [deployments, setDeployments] = useState<DeploymentItem[]>(initialDeployments)
  // Health map: deploymentId -> DeploymentHealthResponse
  const [healthMap, setHealthMap] = useState<Record<string, DeploymentHealthResponse>>({})
  const [healthLoading, setHealthLoading] = useState(false)

  // Servers & Server Metrics state
  const [servers, setServers] = useState<ServerItem[]>([])
  const [serverMetrics, setServerMetrics] = useState<Record<string, ServerMetricDTO>>({})

  // Incidents state
  const [incidents, setIncidents] = useState<AlertIncidentDTO[]>([])
  const [incidentsLoading, setIncidentsLoading] = useState(false)

  // Polling state (10s auto-refresh)
  const [autoRefresh, setAutoRefresh] = useState(true)
  const [countdown, setCountdown] = useState(10)

  // Filter & Search states
  const [searchQuery, setSearchQuery] = useState('')
  const [incidentStatusFilter, setIncidentStatusFilter] = useState<'all' | 'firing' | 'resolved'>('all')

  // Alert Channels Setup Modal
  const [isAlertModalOpen, setIsAlertModalOpen] = useState(false)

  // Probing trigger state: deploymentId -> boolean
  const [probingId, setProbingId] = useState<string | null>(null)
  const [toastMessage, setToastMessage] = useState<{ type: 'success' | 'error'; message: string } | null>(null)

  const showToast = (type: 'success' | 'error', message: string) => {
    setToastMessage({ type, message })
    setTimeout(() => setToastMessage(null), 4000)
  }

  // 1. Fetch Deployments & Health
  const fetchDeploymentsAndHealth = async () => {
    try {
      const res = await fetch('/api/deployments')
      if (res.ok) {
        const data = await res.json()
        const deps: DeploymentItem[] = data.deployments || []
        setDeployments(deps)

        // Fetch health probes for all deployments
        const healthResults: Record<string, DeploymentHealthResponse> = {}
        await Promise.all(
          deps.map(async (dep) => {
            try {
              const hRes = await fetch(`/api/deployments/${dep.id}/health`)
              if (hRes.ok) {
                const hData = await hRes.json()
                healthResults[dep.id] = hData
              }
            } catch {
              // ignore
            }
          })
        )
        setHealthMap(healthResults)
      }
    } catch {
      // ignore
    }
  }

  // 2. Fetch Servers & Server Resource Metrics
  const fetchServersAndMetrics = async () => {
    try {
      const res = await fetch('/api/servers')
      if (res.ok) {
        const data = await res.json()
        const srvs: ServerItem[] = data.servers || []
        setServers(srvs)

        // Fetch latest metric for each server
        const metricsMap: Record<string, ServerMetricDTO> = {}
        await Promise.all(
          srvs.map(async (srv) => {
            try {
              const mRes = await fetch(`/api/servers/${srv.id}/metrics?limit=1&collect=true`)
              if (mRes.ok) {
                const mData = await mRes.json()
                if (mData.latest) {
                  metricsMap[srv.id] = mData.latest
                }
              }
            } catch {
              // ignore
            }
          })
        )
        setServerMetrics(metricsMap)
      }
    } catch {
      // ignore
    }
  }

  // 3. Fetch Incidents History
  const fetchIncidents = async () => {
    setIncidentsLoading(true)
    try {
      const res = await fetch('/api/incidents?limit=50')
      if (res.ok) {
        const data = await res.json()
        setIncidents(data.incidents || [])
      }
    } catch {
      // ignore
    } finally {
      setIncidentsLoading(false)
    }
  }

  // Combined Refresh Function
  const refreshAllData = async () => {
    setHealthLoading(true)
    await Promise.all([
      fetchDeploymentsAndHealth(),
      fetchServersAndMetrics(),
      fetchIncidents(),
    ])
    setHealthLoading(false)
    setCountdown(10)
  }

  // Initial load
  useEffect(() => {
    refreshAllData()
  }, [])

  // Auto-refresh interval (10s)
  useEffect(() => {
    if (!autoRefresh) return

    const interval = setInterval(() => {
      setCountdown((prev) => {
        if (prev <= 1) {
          refreshAllData()
          return 10
        }
        return prev - 1
      })
    }, 1000)

    return () => clearInterval(interval)
  }, [autoRefresh])

  // Instant trigger health check now for a single deployment
  const handleTriggerHealthCheckNow = async (depId: string) => {
    setProbingId(depId)
    try {
      const res = await fetch(`/api/deployments/${depId}/health/check-now`, {
        method: 'POST',
      })
      if (!res.ok) {
        throw new Error(`HTTP ${res.status}`)
      }
      const data = await res.json()
      showToast('success', `Health check berhasil untuk ${depId}: Status ${data.status} (${data.response_time_ms}ms)`)
      // Refresh single health check in map
      setHealthMap((prev) => ({
        ...prev,
        [depId]: {
          ...prev[depId],
          status: data.status,
          last_status_code: data.last_status_code,
          response_time_ms: data.response_time_ms,
          consecutive_failures: data.consecutive_failures,
          last_checked_at: data.last_checked_at,
          uptime_percent: data.uptime_percent,
          avg_latency_ms: data.avg_latency_ms,
          total_probes: data.total_probes,
        },
      }))
      // Also refresh incidents
      fetchIncidents()
    } catch (err: any) {
      showToast('error', `Gagal menjalankan health check: ${err.message}`)
    } finally {
      setProbingId(null)
    }
  }

  // Computed summary metrics
  const summaryMetrics = useMemo(() => {
    const totalDeployments = deployments.length
    const healthValues = Object.values(healthMap)

    // Active outages count
    const activeOutages = healthValues.filter(
      (h) => h.status === 'unhealthy' || h.status === 'down'
    ).length

    // Degraded count
    const degradedCount = healthValues.filter((h) => h.status === 'degraded').length

    // Average latency ms
    const validLatencies = healthValues
      .map((h) => h.response_time_ms || h.avg_latency_ms || 0)
      .filter((lat) => lat > 0)

    const avgLatency =
      validLatencies.length > 0
        ? Math.round(validLatencies.reduce((a, b) => a + b, 0) / validLatencies.length)
        : 0

    // Average Uptime Score %
    const validUptimes = healthValues
      .map((h) => (h.total_probes > 0 ? h.uptime_percent : 100))
    const uptimeScore =
      validUptimes.length > 0
        ? Number((validUptimes.reduce((a, b) => a + b, 0) / validUptimes.length).toFixed(1))
        : 100.0

    return {
      totalDeployments,
      activeOutages,
      degradedCount,
      avgLatency,
      uptimeScore,
    }
  }, [deployments, healthMap])

  // Filtered incidents
  const filteredIncidents = useMemo(() => {
    return incidents.filter((inc) => {
      if (incidentStatusFilter !== 'all' && inc.status !== incidentStatusFilter) {
        return false
      }
      if (searchQuery.trim()) {
        const query = searchQuery.toLowerCase()
        return (
          inc.title.toLowerCase().includes(query) ||
          inc.message.toLowerCase().includes(query) ||
          inc.target_id.toLowerCase().includes(query) ||
          inc.target_type.toLowerCase().includes(query)
        )
      }
      return true
    })
  }, [incidents, incidentStatusFilter, searchQuery])

  // Duration formatter
  const formatDuration = (triggeredAt: string, resolvedAt?: string) => {
    const start = new Date(triggeredAt).getTime()
    const end = resolvedAt ? new Date(resolvedAt).getTime() : Date.now()
    const diffSec = Math.max(0, Math.floor((end - start) / 1000))

    if (diffSec < 60) return `${diffSec}d`
    const diffMin = Math.floor(diffSec / 60)
    if (diffMin < 60) return `${diffMin}m ${diffSec % 60}d`
    const diffHours = Math.floor(diffMin / 60)
    return `${diffHours}j ${diffMin % 60}m`
  }

  return (
    <div className="space-y-6 animate-in fade-in duration-200">
      {/* Toast Notification */}
      {toastMessage && (
        <div
          className={`p-3.5 rounded-xl border text-xs flex items-center justify-between shadow-lg animate-in slide-in-from-top-2 ${
            toastMessage.type === 'success'
              ? 'bg-emerald-50 dark:bg-emerald-950/80 border-emerald-300 dark:border-emerald-800 text-emerald-800 dark:text-emerald-200'
              : 'bg-red-50 dark:bg-red-950/80 border-red-300 dark:border-red-800 text-red-800 dark:text-red-200'
          }`}
        >
          <div className="flex items-center gap-2">
            {toastMessage.type === 'success' ? (
              <CheckCircle2 className="h-4 w-4 text-emerald-600" />
            ) : (
              <AlertCircle className="h-4 w-4 text-red-600" />
            )}
            <span>{toastMessage.message}</span>
          </div>
          <button
            onClick={() => setToastMessage(null)}
            className="text-slate-400 hover:text-slate-600 p-1"
          >
            <XCircle className="h-4 w-4" />
          </button>
        </div>
      )}

      {/* Header Actions Bar */}
      <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-3 pb-1 border-b border-slate-200 dark:border-slate-800">
        <div>
          <div className="flex items-center gap-2.5">
            <h1 className="text-xl font-bold tracking-tight text-slate-900 dark:text-white flex items-center gap-2">
              <Activity className="h-5 w-5 text-emerald-500" />
              <span>Observability & Incident Hub</span>
            </h1>
            <span className="flex items-center gap-1 text-[11px] font-mono text-emerald-600 dark:text-emerald-400 bg-emerald-50 dark:bg-emerald-950/60 px-2 py-0.5 rounded-full border border-emerald-200 dark:border-emerald-800">
              <Radio className="h-3 w-3 animate-pulse" /> Live Polling (10s)
            </span>
          </div>
          <p className="text-xs text-slate-500 dark:text-slate-400 mt-1">
            Pantau status ketersediaan, latensi respon HTTP, utilisasi resource CPU/RAM/Disk, dan riwayat alert outage aplikasi.
          </p>
        </div>

        <div className="flex items-center gap-2 flex-wrap">
          {/* Polling auto-refresh switch */}
          <button
            onClick={() => setAutoRefresh(!autoRefresh)}
            className={`flex items-center gap-1.5 px-2.5 py-1.5 rounded-lg border text-xs font-semibold transition-all ${
              autoRefresh
                ? 'bg-slate-100 dark:bg-slate-800 border-slate-300 dark:border-slate-700 text-slate-700 dark:text-slate-300'
                : 'bg-transparent border-dashed border-slate-300 text-slate-400'
            }`}
            title={autoRefresh ? 'Matikan auto-refresh' : 'Nyalakan auto-refresh (10s)'}
          >
            <Clock className="h-3.5 w-3.5 text-emerald-500" />
            <span>{autoRefresh ? `${countdown}s` : 'Paused'}</span>
          </button>

          {/* Refresh Now Button */}
          <Button
            size="sm"
            variant="outline"
            onClick={refreshAllData}
            disabled={healthLoading}
            className="h-8 text-xs font-semibold gap-1.5"
          >
            <RefreshCw className={`h-3.5 w-3.5 ${healthLoading ? 'animate-spin text-emerald-500' : ''}`} />
            <span>Refresh</span>
          </Button>

          {/* Alert Channels Setup Modal Button */}
          <Button
            size="sm"
            onClick={() => setIsAlertModalOpen(true)}
            className="h-8 text-xs font-semibold bg-emerald-600 hover:bg-emerald-500 text-white gap-1.5 shadow-xs"
          >
            <Bell className="h-3.5 w-3.5" />
            <span>Alert Channels</span>
          </Button>
        </div>
      </div>

      {/* ================= 1. Summary Cards ================= */}
      <div className="grid grid-cols-2 md:grid-cols-4 gap-3.5">
        {/* Card 1: Uptime Score */}
        <Card className="p-4 bg-white dark:bg-slate-900 border-slate-200 dark:border-slate-800 shadow-xs relative overflow-hidden group">
          <div className="flex items-center justify-between">
            <span className="text-xs font-medium text-slate-500 dark:text-slate-400">
              Uptime Score
            </span>
            <div className="p-2 rounded-xl bg-emerald-500/10 text-emerald-600 dark:text-emerald-400">
              <TrendingUp className="h-4 w-4" />
            </div>
          </div>
          <div className="mt-2.5 flex items-baseline gap-2">
            <span className="text-2xl font-bold font-mono tracking-tight text-slate-900 dark:text-white">
              {summaryMetrics.uptimeScore}%
            </span>
            <span className="text-[11px] font-medium text-emerald-600 dark:text-emerald-400">
              Rolling SLA
            </span>
          </div>
          <p className="text-[11px] text-slate-400 mt-1">
            Rata-rata ketersediaan probe deployment
          </p>
        </Card>

        {/* Card 2: Monitored Deployments */}
        <Card className="p-4 bg-white dark:bg-slate-900 border-slate-200 dark:border-slate-800 shadow-xs relative overflow-hidden group">
          <div className="flex items-center justify-between">
            <span className="text-xs font-medium text-slate-500 dark:text-slate-400">
              Monitored Deployments
            </span>
            <div className="p-2 rounded-xl bg-blue-500/10 text-blue-600 dark:text-blue-400">
              <Layers className="h-4 w-4" />
            </div>
          </div>
          <div className="mt-2.5 flex items-baseline gap-2">
            <span className="text-2xl font-bold font-mono tracking-tight text-slate-900 dark:text-white">
              {summaryMetrics.totalDeployments}
            </span>
            <span className="text-[11px] font-medium text-slate-500">
              Active Targets
            </span>
          </div>
          <p className="text-[11px] text-slate-400 mt-1">
            Local sandboxes & Remote BYOS nodes
          </p>
        </Card>

        {/* Card 3: Active Outages */}
        <Card className="p-4 bg-white dark:bg-slate-900 border-slate-200 dark:border-slate-800 shadow-xs relative overflow-hidden group">
          <div className="flex items-center justify-between">
            <span className="text-xs font-medium text-slate-500 dark:text-slate-400">
              Active Outages
            </span>
            <div
              className={`p-2 rounded-xl ${
                summaryMetrics.activeOutages > 0
                  ? 'bg-red-500/10 text-red-600 dark:text-red-400'
                  : 'bg-emerald-500/10 text-emerald-600 dark:text-emerald-400'
              }`}
            >
              {summaryMetrics.activeOutages > 0 ? (
                <AlertTriangle className="h-4 w-4" />
              ) : (
                <CheckCircle2 className="h-4 w-4" />
              )}
            </div>
          </div>
          <div className="mt-2.5 flex items-baseline gap-2">
            <span
              className={`text-2xl font-bold font-mono tracking-tight ${
                summaryMetrics.activeOutages > 0
                  ? 'text-red-600 dark:text-red-400'
                  : 'text-slate-900 dark:text-white'
              }`}
            >
              {summaryMetrics.activeOutages}
            </span>
            <span
              className={`text-[11px] font-medium ${
                summaryMetrics.activeOutages > 0
                  ? 'text-red-600 dark:text-red-400'
                  : 'text-emerald-600 dark:text-emerald-400'
              }`}
            >
              {summaryMetrics.activeOutages > 0 ? 'Outage Aktif!' : 'All Healthy'}
            </span>
          </div>
          <p className="text-[11px] text-slate-400 mt-1">
            {summaryMetrics.degradedCount > 0
              ? `${summaryMetrics.degradedCount} deployment degraded`
              : 'Tidak ada gangguan layanan'}
          </p>
        </Card>

        {/* Card 4: Average Response Latency */}
        <Card className="p-4 bg-white dark:bg-slate-900 border-slate-200 dark:border-slate-800 shadow-xs relative overflow-hidden group">
          <div className="flex items-center justify-between">
            <span className="text-xs font-medium text-slate-500 dark:text-slate-400">
              Avg Response Latency
            </span>
            <div className="p-2 rounded-xl bg-amber-500/10 text-amber-600 dark:text-amber-400">
              <Zap className="h-4 w-4" />
            </div>
          </div>
          <div className="mt-2.5 flex items-baseline gap-2">
            <span className="text-2xl font-bold font-mono tracking-tight text-slate-900 dark:text-white">
              {summaryMetrics.avgLatency}
              <span className="text-sm font-normal text-slate-400 ml-1">ms</span>
            </span>
            <span className="text-[11px] font-medium text-amber-600 dark:text-amber-400">
              HTTP Probe
            </span>
          </div>
          <p className="text-[11px] text-slate-400 mt-1">
            Waktu respon endpoint aplikasi
          </p>
        </Card>
      </div>

      {/* ================= 2. Resource Utilization Gauges (Live CPU, RAM, Disk) ================= */}
      <div className="space-y-3">
        <div className="flex items-center justify-between">
          <div className="flex items-center gap-2">
            <Server className="h-4 w-4 text-indigo-500" />
            <h2 className="text-sm font-bold tracking-tight text-slate-900 dark:text-white uppercase">
              Resource Telemetry & Gauges
            </h2>
          </div>
          <span className="text-xs text-slate-400 font-mono">
            {servers.length} Servers Monitored
          </span>
        </div>

        {servers.length === 0 ? (
          <Card className="p-6 text-center border-dashed border-slate-200 dark:border-slate-800">
            <p className="text-xs text-slate-500">
              Belum ada server BYOS terdaftar. Metrik local host runner dan container akan otomatis terkumpul saat terhubung.
            </p>
          </Card>
        ) : (
          <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-3.5">
            {servers.map((srv) => {
              const metric = serverMetrics[srv.id]
              const cpuPct = metric ? Math.min(100, Math.max(0, metric.cpu_percent)) : 0
              const memPct = metric ? Math.min(100, Math.max(0, metric.memory_percent)) : 0
              const diskPct = metric ? Math.min(100, Math.max(0, metric.disk_percent)) : 0

              const getBarColor = (pct: number) => {
                if (pct >= 85) return 'bg-red-500'
                if (pct >= 65) return 'bg-amber-500'
                return 'bg-emerald-500'
              }

              return (
                <Card
                  key={srv.id}
                  className="p-4 bg-white dark:bg-slate-900 border-slate-200 dark:border-slate-800 shadow-xs space-y-3"
                >
                  <div className="flex items-start justify-between">
                    <div>
                      <h3 className="font-bold text-sm text-slate-900 dark:text-white truncate">
                        {srv.name}
                      </h3>
                      <p className="text-[11px] font-mono text-slate-400">
                        {srv.host}:{srv.port}
                      </p>
                    </div>
                    <Badge variant="outline" className="border-indigo-400/40 text-indigo-600 dark:text-indigo-400 text-[10px] font-mono">
                      {srv.status}
                    </Badge>
                  </div>

                  {/* CPU Gauge */}
                  <div className="space-y-1">
                    <div className="flex items-center justify-between text-xs">
                      <span className="text-slate-500 flex items-center gap-1">
                        <Cpu className="h-3 w-3 text-slate-400" /> CPU Load
                      </span>
                      <span className="font-mono font-bold text-slate-800 dark:text-slate-200">
                        {metric ? `${cpuPct.toFixed(1)}%` : 'Collecting...'}
                      </span>
                    </div>
                    <div className="h-2 w-full rounded-full bg-slate-100 dark:bg-slate-800 overflow-hidden">
                      <div
                        className={`h-full transition-all duration-500 rounded-full ${getBarColor(cpuPct)}`}
                        style={{ width: `${cpuPct}%` }}
                      />
                    </div>
                  </div>

                  {/* RAM Gauge */}
                  <div className="space-y-1">
                    <div className="flex items-center justify-between text-xs">
                      <span className="text-slate-500 flex items-center gap-1">
                        <Activity className="h-3 w-3 text-slate-400" /> RAM Memory
                      </span>
                      <span className="font-mono font-bold text-slate-800 dark:text-slate-200">
                        {metric
                          ? `${metric.memory_used_mb} MB / ${metric.memory_total_mb} MB (${memPct.toFixed(0)}%)`
                          : 'Collecting...'}
                      </span>
                    </div>
                    <div className="h-2 w-full rounded-full bg-slate-100 dark:bg-slate-800 overflow-hidden">
                      <div
                        className={`h-full transition-all duration-500 rounded-full ${getBarColor(memPct)}`}
                        style={{ width: `${memPct}%` }}
                      />
                    </div>
                  </div>

                  {/* Disk Gauge */}
                  <div className="space-y-1">
                    <div className="flex items-center justify-between text-xs">
                      <span className="text-slate-500 flex items-center gap-1">
                        <HardDrive className="h-3 w-3 text-slate-400" /> Disk Storage
                      </span>
                      <span className="font-mono font-bold text-slate-800 dark:text-slate-200">
                        {metric
                          ? `${metric.disk_used_gb.toFixed(1)} GB / ${metric.disk_total_gb.toFixed(1)} GB (${diskPct.toFixed(0)}%)`
                          : 'Collecting...'}
                      </span>
                    </div>
                    <div className="h-2 w-full rounded-full bg-slate-100 dark:bg-slate-800 overflow-hidden">
                      <div
                        className={`h-full transition-all duration-500 rounded-full ${getBarColor(diskPct)}`}
                        style={{ width: `${diskPct}%` }}
                      />
                    </div>
                  </div>

                  <div className="pt-1 text-[10px] text-slate-400 flex items-center justify-between">
                    <span>Recorded: {metric ? new Date(metric.recorded_at).toLocaleTimeString() : '-'}</span>
                    <span className="text-emerald-500 font-medium">Non-invasive SSH Probe</span>
                  </div>
                </Card>
              )
            })}
          </div>
        )}
      </div>

      {/* ================= 3. Monitored Deployments Health Table ================= */}
      <div className="space-y-3">
        <div className="flex items-center justify-between">
          <div className="flex items-center gap-2">
            <HeartPulse className="h-4 w-4 text-emerald-500" />
            <h2 className="text-sm font-bold tracking-tight text-slate-900 dark:text-white uppercase">
              Deployments Health & Probes
            </h2>
          </div>
          <span className="text-xs text-slate-400">
            HTTP Check Every 30s
          </span>
        </div>

        <Card className="bg-white dark:bg-slate-900 border-slate-200 dark:border-slate-800 shadow-xs overflow-hidden">
          <div className="overflow-x-auto">
            <table className="w-full text-xs text-left">
              <thead className="bg-slate-50 dark:bg-slate-800/60 text-slate-500 border-b border-slate-200 dark:border-slate-800 text-[11px] uppercase tracking-wider font-semibold">
                <tr>
                  <th className="py-3 px-4">Deployment</th>
                  <th className="py-3 px-4">Probe URL</th>
                  <th className="py-3 px-4">Health Status</th>
                  <th className="py-3 px-4">Status Code</th>
                  <th className="py-3 px-4">Latency</th>
                  <th className="py-3 px-4">Uptime SLA</th>
                  <th className="py-3 px-4 text-right">Quick Action</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-slate-100 dark:divide-slate-800">
                {deployments.length === 0 ? (
                  <tr>
                    <td colSpan={7} className="py-8 text-center text-slate-400 italic">
                      Belum ada deployment yang dimonitor.
                    </td>
                  </tr>
                ) : (
                  deployments.map((dep) => {
                    const health = healthMap[dep.id]
                    const status = health ? health.status : (dep.status === 'running' ? 'healthy' : 'unhealthy')
                    const isHealthy = status === 'healthy'
                    const isDegraded = status === 'degraded'

                    const liveURL = dep.domain
                      ? `http://${dep.domain}`
                      : `http://${dep.slug}.${baseDomain}`

                    return (
                      <tr
                        key={dep.id}
                        className="hover:bg-slate-50/50 dark:hover:bg-slate-800/30 transition-colors"
                      >
                        <td className="py-3 px-4 font-semibold text-slate-900 dark:text-white">
                          <div
                            onClick={() => onOpenDrawer?.(dep)}
                            className="cursor-pointer hover:text-emerald-600 dark:hover:text-emerald-400 truncate max-w-[180px]"
                          >
                            {dep.project_name}
                          </div>
                          <span className="text-[10px] font-mono text-slate-400 block truncate">
                            /{dep.slug}
                          </span>
                        </td>

                        <td className="py-3 px-4 font-mono text-[11px] text-slate-600 dark:text-slate-400">
                          <a
                            href={liveURL}
                            target="_blank"
                            rel="noreferrer"
                            className="hover:underline flex items-center gap-1 text-emerald-600 dark:text-emerald-400 truncate max-w-[200px]"
                          >
                            {liveURL.replace('http://', '')}
                            <ExternalLink className="h-3 w-3 shrink-0" />
                          </a>
                        </td>

                        <td className="py-3 px-4">
                          {isHealthy ? (
                            <Badge className="bg-emerald-500/10 text-emerald-600 dark:text-emerald-400 border-emerald-500/30 gap-1 text-[11px] font-semibold py-0.5 px-2">
                              <span className="h-1.5 w-1.5 rounded-full bg-emerald-500 animate-pulse" />
                              Healthy
                            </Badge>
                          ) : isDegraded ? (
                            <Badge className="bg-amber-500/10 text-amber-600 dark:text-amber-400 border-amber-500/30 gap-1 text-[11px] font-semibold py-0.5 px-2">
                              <span className="h-1.5 w-1.5 rounded-full bg-amber-500" />
                              Degraded
                            </Badge>
                          ) : (
                            <Badge variant="destructive" className="gap-1 text-[11px] font-semibold py-0.5 px-2">
                              <span className="h-1.5 w-1.5 rounded-full bg-white" />
                              Down
                            </Badge>
                          )}
                        </td>

                        <td className="py-3 px-4 font-mono text-xs">
                          {health?.last_status_code ? (
                            <span
                              className={`px-1.5 py-0.5 rounded font-bold ${
                                health.last_status_code >= 200 && health.last_status_code < 400
                                  ? 'bg-emerald-100 text-emerald-700 dark:bg-emerald-950 dark:text-emerald-400'
                                  : 'bg-red-100 text-red-700 dark:bg-red-950 dark:text-red-400'
                              }`}
                            >
                              {health.last_status_code}
                            </span>
                          ) : (
                            <span className="text-slate-400">-</span>
                          )}
                        </td>

                        <td className="py-3 px-4 font-mono text-xs">
                          {health?.response_time_ms ? (
                            <span className="font-semibold text-slate-800 dark:text-slate-200">
                              {health.response_time_ms} ms
                            </span>
                          ) : (
                            <span className="text-slate-400">-</span>
                          )}
                        </td>

                        <td className="py-3 px-4 font-mono text-xs font-semibold text-emerald-600 dark:text-emerald-400">
                          {health ? `${health.uptime_percent.toFixed(1)}%` : '100%'}
                        </td>

                        <td className="py-3 px-4 text-right">
                          <Button
                            size="sm"
                            variant="outline"
                            onClick={() => handleTriggerHealthCheckNow(dep.id)}
                            disabled={probingId === dep.id}
                            className="h-7 text-[11px] px-2.5 font-semibold text-slate-700 dark:text-slate-300 hover:bg-slate-100 dark:hover:bg-slate-800"
                          >
                            <RefreshCw
                              className={`h-3 w-3 mr-1 ${probingId === dep.id ? 'animate-spin text-emerald-500' : ''}`}
                            />
                            <span>{probingId === dep.id ? 'Probing...' : 'Check Now'}</span>
                          </Button>
                        </td>
                      </tr>
                    )
                  })
                )}
              </tbody>
            </table>
          </div>
        </Card>
      </div>

      {/* ================= 4. Incident History Table ================= */}
      <div className="space-y-3">
        <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-2">
          <div className="flex items-center gap-2">
            <AlertTriangle className="h-4 w-4 text-amber-500" />
            <h2 className="text-sm font-bold tracking-tight text-slate-900 dark:text-white uppercase">
              Incident History & Audit Log
            </h2>
            <Badge variant="outline" className="text-[10px] font-mono">
              {filteredIncidents.length} Records
            </Badge>
          </div>

          <div className="flex items-center gap-2">
            {/* Incident Status Filter */}
            <div className="flex rounded-lg bg-slate-100 dark:bg-slate-800 p-0.5 text-xs">
              {(['all', 'firing', 'resolved'] as const).map((filter) => (
                <button
                  key={filter}
                  onClick={() => setIncidentStatusFilter(filter)}
                  className={`px-2.5 py-1 rounded-md font-semibold capitalize transition-all ${
                    incidentStatusFilter === filter
                      ? 'bg-white dark:bg-slate-900 text-slate-900 dark:text-white shadow-xs'
                      : 'text-slate-500 hover:text-slate-900 dark:text-slate-400'
                  }`}
                >
                  {filter}
                </button>
              ))}
            </div>

            {/* Incident Search */}
            <div className="relative">
              <Search className="h-3.5 w-3.5 absolute left-2.5 top-2.5 text-slate-400" />
              <Input
                value={searchQuery}
                onChange={(e) => setSearchQuery(e.target.value)}
                placeholder="Cari target / event..."
                className="h-8 pl-8 text-xs w-40 sm:w-56"
              />
            </div>
          </div>
        </div>

        <Card className="bg-white dark:bg-slate-900 border-slate-200 dark:border-slate-800 shadow-xs overflow-hidden">
          <div className="overflow-x-auto">
            <table className="w-full text-xs text-left">
              <thead className="bg-slate-50 dark:bg-slate-800/60 text-slate-500 border-b border-slate-200 dark:border-slate-800 text-[11px] uppercase tracking-wider font-semibold">
                <tr>
                  <th className="py-3 px-4">Status</th>
                  <th className="py-3 px-4">Target</th>
                  <th className="py-3 px-4">Event Title & Details</th>
                  <th className="py-3 px-4">Severity</th>
                  <th className="py-3 px-4">Duration</th>
                  <th className="py-3 px-4">Timestamp</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-slate-100 dark:divide-slate-800">
                {incidentsLoading && incidents.length === 0 ? (
                  <tr>
                    <td colSpan={6} className="py-8 text-center text-slate-400 italic">
                      <RefreshCw className="h-4 w-4 animate-spin mx-auto mb-1 text-emerald-500" />
                      Memuat riwayat insiden...
                    </td>
                  </tr>
                ) : filteredIncidents.length === 0 ? (
                  <tr>
                    <td colSpan={6} className="py-8 text-center text-slate-400 italic">
                      {searchQuery ? 'Tidak ada insiden yang cocok dengan pencarian.' : 'Belum ada catatan insiden tercatat (Sistem 100% stabil).'}
                    </td>
                  </tr>
                ) : (
                  filteredIncidents.map((inc) => {
                    const isFiring = inc.status === 'firing'

                    return (
                      <tr
                        key={inc.id}
                        className="hover:bg-slate-50/50 dark:hover:bg-slate-800/30 transition-colors"
                      >
                        {/* Status Badge */}
                        <td className="py-3 px-4">
                          {isFiring ? (
                            <Badge variant="destructive" className="gap-1 py-0.5 px-2 text-[10px] font-mono">
                              <span className="h-1.5 w-1.5 rounded-full bg-white animate-pulse" />
                              FIRING
                            </Badge>
                          ) : (
                            <Badge className="bg-emerald-500/10 text-emerald-600 dark:text-emerald-400 border-emerald-500/30 gap-1 py-0.5 px-2 text-[10px] font-mono">
                              <CheckCircle2 className="h-3 w-3" />
                              RESOLVED
                            </Badge>
                          )}
                        </td>

                        {/* Target */}
                        <td className="py-3 px-4">
                          <span className="font-semibold text-slate-900 dark:text-white capitalize block">
                            {inc.target_type}
                          </span>
                          <span className="font-mono text-[10px] text-slate-400 block truncate max-w-[120px]">
                            {inc.target_id}
                          </span>
                        </td>

                        {/* Title & Details */}
                        <td className="py-3 px-4">
                          <div className="font-bold text-slate-900 dark:text-white">
                            {inc.title}
                          </div>
                          <div className="text-[11px] text-slate-500 dark:text-slate-400 max-w-md line-clamp-2 mt-0.5">
                            {inc.message}
                          </div>
                        </td>

                        {/* Severity Badge */}
                        <td className="py-3 px-4">
                          {isFiring ? (
                            <Badge className="bg-red-100 text-red-700 dark:bg-red-950 dark:text-red-300 border-red-300 text-[10px]">
                              Critical Outage
                            </Badge>
                          ) : (
                            <Badge variant="outline" className="border-emerald-300 text-emerald-700 dark:text-emerald-400 text-[10px]">
                              Normal / Recovered
                            </Badge>
                          )}
                        </td>

                        {/* Duration */}
                        <td className="py-3 px-4 font-mono text-xs text-slate-600 dark:text-slate-400">
                          {isFiring ? (
                            <span className="text-red-600 dark:text-red-400 font-semibold flex items-center gap-1">
                              <Clock className="h-3 w-3" />
                              {formatDuration(inc.triggered_at)} (Aktif)
                            </span>
                          ) : (
                            <span>{formatDuration(inc.triggered_at, inc.resolved_at)}</span>
                          )}
                        </td>

                        {/* Timestamp */}
                        <td className="py-3 px-4 font-mono text-[11px] text-slate-400 whitespace-nowrap">
                          {new Date(inc.triggered_at).toLocaleString('id-ID', {
                            dateStyle: 'short',
                            timeStyle: 'medium',
                          })}
                        </td>
                      </tr>
                    )
                  })
                )}
              </tbody>
            </table>
          </div>
        </Card>
      </div>

      {/* Alert Channels Setup Modal */}
      <AlertChannelsModal
        isOpen={isAlertModalOpen}
        onClose={() => setIsAlertModalOpen(false)}
        projectId="default"
      />
    </div>
  )
}
