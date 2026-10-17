import { useState, useEffect, useMemo, MouseEvent } from 'react'
import { Card } from '../ui/card'
import { Button } from '../ui/button'
import { Input } from '../ui/input'
import { Badge } from '../ui/badge'
import { Alert, AlertTitle, AlertDescription } from '../ui/alert'
import {
  Rocket,
  Plus,
  RefreshCw,
  Search,
  Play,
  CheckCircle2,
  AlertCircle,
  Clock,
  Layers,
  ChevronRight,
  ExternalLink,
  Activity,
  XCircle
} from 'lucide-react'
import { BatchCampaignDTO, RollupMetrics } from './types'

interface CampaignDashboardViewProps {
  onSelectCampaign: (campaignId: string) => void
  onOpenCreateModal: () => void
  onNavigate?: (path: string) => void
}

export function CampaignDashboardView({
  onSelectCampaign,
  onOpenCreateModal,
  onNavigate: _onNavigate,
}: CampaignDashboardViewProps) {
  const [campaigns, setCampaigns] = useState<BatchCampaignDTO[]>([])
  const [rollups, setRollups] = useState<Record<string, RollupMetrics>>({})
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [notification, setNotification] = useState<{ type: 'success' | 'error'; message: string } | null>(null)

  // Filters & Search
  const [searchQuery, setSearchQuery] = useState('')
  const [statusFilter, setStatusFilter] = useState<'all' | 'draft' | 'pending' | 'running' | 'completed' | 'failed' | 'review'>('all')

  // Executing action state
  const [executingId, setExecutingId] = useState<string | null>(null)

  const showToast = (type: 'success' | 'error', message: string) => {
    setNotification({ type, message })
    setTimeout(() => setNotification(null), 4000)
  }

  const fetchCampaigns = async () => {
    setError(null)
    try {
      const res = await fetch('/api/campaigns')
      if (!res.ok) {
        throw new Error(`HTTP ${res.status}: Gagal memuat daftar kampanye batch`)
      }
      const data = await res.json()
      const list: BatchCampaignDTO[] = data.campaigns || []
      setCampaigns(list)

      // Fetch rollup for each campaign asynchronously
      fetchRollupsForList(list)
    } catch (err: any) {
      setError(err.message || 'Terjadi kesalahan jaringan saat mengambil data kampanye.')
    } finally {
      setLoading(false)
    }
  }

  const fetchRollupsForList = async (list: BatchCampaignDTO[]) => {
    const promises = list.map(async (c) => {
      try {
        const res = await fetch(`/api/campaigns/${c.id}/rollup`)
        if (res.ok) {
          const rData = await res.json()
          if (rData.rollup) {
            return { id: c.id, rollup: rData.rollup as RollupMetrics }
          }
        }
      } catch {}
      return null
    })

    const results = await Promise.all(promises)
    const newRollups: Record<string, RollupMetrics> = {}
    results.forEach((item) => {
      if (item) {
        newRollups[item.id] = item.rollup
      }
    })
    setRollups((prev) => ({ ...prev, ...newRollups }))
  }

  useEffect(() => {
    fetchCampaigns()
    // Poll campaigns every 6 seconds to update progress
    const interval = setInterval(fetchCampaigns, 6000)
    return () => clearInterval(interval)
  }, [])

  // Execute campaign handler
  const handleExecute = async (c: BatchCampaignDTO, e: MouseEvent) => {
    e.stopPropagation()
    setExecutingId(c.id)
    try {
      const res = await fetch(`/api/campaigns/${c.id}/execute`, {
        method: 'POST',
      })
      const data = await res.json()
      if (!res.ok) {
        throw new Error(data.error || `HTTP ${res.status}: Gagal menjalankan kampanye`)
      }

      showToast('success', `Kampanye "${c.title}" berhasil dijalankan (${data.completed_tasks}/${data.total_tasks} sukses).`)
      await fetchCampaigns()
    } catch (err: any) {
      showToast('error', err.message || 'Gagal mengeksekusi kampanye batch.')
    } finally {
      setExecutingId(null)
    }
  }

  // Filtered campaigns
  const filteredCampaigns = useMemo(() => {
    return campaigns.filter((c) => {
      if (statusFilter !== 'all') {
        const currentStatus = rollups[c.id]?.status || c.status
        if (currentStatus.toLowerCase() !== statusFilter.toLowerCase()) {
          return false
        }
      }

      if (searchQuery.trim()) {
        const q = searchQuery.toLowerCase()
        const matchTitle = c.title.toLowerCase().includes(q)
        const matchDesc = (c.description || '').toLowerCase().includes(q)
        const matchId = c.id.toLowerCase().includes(q)
        if (!matchTitle && !matchDesc && !matchId) return false
      }

      return true
    })
  }, [campaigns, statusFilter, searchQuery, rollups])

  // Aggregate stats across all campaigns
  const stats = useMemo(() => {
    let total = campaigns.length
    let running = 0
    let completed = 0
    let draft = 0
    let failed = 0

    campaigns.forEach((c) => {
      const st = (rollups[c.id]?.status || c.status).toLowerCase()
      if (st === 'running') running++
      else if (st === 'completed') completed++
      else if (st === 'failed') failed++
      else draft++
    })

    return { total, running, completed, draft, failed }
  }, [campaigns, rollups])

  const renderStatusBadge = (statusStr: string) => {
    const s = statusStr.toLowerCase()
    switch (s) {
      case 'completed':
      case 'done':
        return (
          <Badge className="bg-emerald-500/10 text-emerald-600 dark:text-emerald-400 border-emerald-500/30 text-[10px] py-0 px-2 flex items-center gap-1 font-mono">
            <CheckCircle2 className="h-3 w-3" />
            completed
          </Badge>
        )
      case 'running':
      case 'doing':
        return (
          <Badge className="bg-blue-500/10 text-blue-600 dark:text-blue-400 border-blue-500/30 text-[10px] py-0 px-2 flex items-center gap-1 font-mono">
            <span className="h-1.5 w-1.5 rounded-full bg-blue-500 animate-pulse" />
            running
          </Badge>
        )
      case 'failed':
        return (
          <Badge className="bg-red-500/10 text-red-600 dark:text-red-400 border-red-500/30 text-[10px] py-0 px-2 flex items-center gap-1 font-mono">
            <XCircle className="h-3 w-3" />
            failed
          </Badge>
        )
      case 'review':
        return (
          <Badge className="bg-purple-500/10 text-purple-600 dark:text-purple-400 border-purple-500/30 text-[10px] py-0 px-2 flex items-center gap-1 font-mono">
            <Activity className="h-3 w-3" />
            review
          </Badge>
        )
      case 'pending':
        return (
          <Badge className="bg-amber-500/10 text-amber-600 dark:text-amber-400 border-amber-500/30 text-[10px] py-0 px-2 flex items-center gap-1 font-mono">
            <Clock className="h-3 w-3" />
            pending
          </Badge>
        )
      default:
        return (
          <Badge variant="outline" className="text-slate-500 dark:text-slate-400 text-[10px] py-0 px-2 font-mono">
            draft
          </Badge>
        )
    }
  }

  return (
    <div className="space-y-6 animate-in fade-in duration-200">
      {/* Toast Notification */}
      {notification && (
        <div
          className={`p-3.5 rounded-xl flex items-center justify-between text-xs font-semibold shadow-md ${
            notification.type === 'success'
              ? 'bg-emerald-600 text-white'
              : 'bg-red-600 text-white'
          }`}
        >
          <span>{notification.message}</span>
          <button onClick={() => setNotification(null)} className="opacity-80 hover:opacity-100">
            <ExternalLink className="h-3.5 w-3.5" />
          </button>
        </div>
      )}

      {/* Header Banner & Actions */}
      <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-4 p-5 rounded-2xl bg-gradient-to-r from-emerald-950/40 via-slate-900 to-slate-900 border border-emerald-500/20 text-white shadow-lg">
        <div className="flex items-start gap-3.5">
          <div className="h-11 w-11 rounded-xl bg-emerald-500/20 border border-emerald-500/40 text-emerald-400 flex items-center justify-center shrink-0 mt-0.5">
            <Rocket className="h-6 w-6" />
          </div>
          <div>
            <div className="flex items-center gap-2 flex-wrap">
              <h2 className="text-lg font-bold text-white tracking-tight">
                Batch Campaign Studio
              </h2>
              <Badge className="bg-emerald-500/20 text-emerald-300 border-emerald-500/40 text-[10px] font-mono">
                Sprint 8 Multi-Project Fan-Out
              </Badge>
            </div>
            <p className="text-xs text-slate-300 mt-0.5">
              Kelola, orkestrasi, dan eksekusi otomatisasi massal (WP-CLI, AdSense, patch konfigurasi) lintas portofolio multi-situs.
            </p>
          </div>
        </div>

        <div className="flex items-center gap-2.5 self-end sm:self-center shrink-0">
          <Button
            variant="outline"
            size="sm"
            onClick={fetchCampaigns}
            className="h-9 text-xs border-slate-700 bg-slate-800/80 hover:bg-slate-700 text-slate-200"
          >
            <RefreshCw className={`h-3.5 w-3.5 mr-1.5 ${loading ? 'animate-spin' : ''}`} />
            Refresh
          </Button>

          <Button
            onClick={onOpenCreateModal}
            className="h-9 bg-emerald-600 hover:bg-emerald-500 text-white font-semibold text-xs shadow-md shadow-emerald-600/20 flex items-center gap-1.5"
          >
            <Plus className="h-4 w-4" />
            <span>New Campaign</span>
          </Button>
        </div>
      </div>

      {/* Stats Summary Cards */}
      <div className="grid grid-cols-2 sm:grid-cols-4 gap-3 sm:gap-4">
        <Card className="p-4 bg-white dark:bg-slate-900 border-slate-200 dark:border-slate-800">
          <div className="flex items-center justify-between">
            <div>
              <p className="text-xs font-medium text-slate-500 dark:text-slate-400">Total Kampanye</p>
              <p className="text-2xl font-bold tracking-tight text-slate-900 dark:text-white mt-0.5">
                {stats.total}
              </p>
            </div>
            <div className="h-9 w-9 rounded-xl bg-slate-100 dark:bg-slate-800 text-slate-600 dark:text-slate-300 flex items-center justify-center">
              <Layers className="h-4 w-4" />
            </div>
          </div>
        </Card>

        <Card className="p-4 bg-white dark:bg-slate-900 border-slate-200 dark:border-slate-800">
          <div className="flex items-center justify-between">
            <div>
              <p className="text-xs font-medium text-slate-500 dark:text-slate-400">Aktif Running</p>
              <div className="flex items-center gap-2 mt-0.5">
                <p className="text-2xl font-bold tracking-tight text-blue-600 dark:text-blue-400">
                  {stats.running}
                </p>
                {stats.running > 0 && <span className="h-2 w-2 rounded-full bg-blue-500 animate-pulse" />}
              </div>
            </div>
            <div className="h-9 w-9 rounded-xl bg-blue-100 dark:bg-blue-950/60 text-blue-600 dark:text-blue-400 flex items-center justify-center">
              <Activity className="h-4 w-4" />
            </div>
          </div>
        </Card>

        <Card className="p-4 bg-white dark:bg-slate-900 border-slate-200 dark:border-slate-800">
          <div className="flex items-center justify-between">
            <div>
              <p className="text-xs font-medium text-slate-500 dark:text-slate-400">Completed (Selesai)</p>
              <p className="text-2xl font-bold tracking-tight text-emerald-600 dark:text-emerald-400 mt-0.5">
                {stats.completed}
              </p>
            </div>
            <div className="h-9 w-9 rounded-xl bg-emerald-100 dark:bg-emerald-950/60 text-emerald-600 dark:text-emerald-400 flex items-center justify-center">
              <CheckCircle2 className="h-4 w-4" />
            </div>
          </div>
        </Card>

        <Card className="p-4 bg-white dark:bg-slate-900 border-slate-200 dark:border-slate-800">
          <div className="flex items-center justify-between">
            <div>
              <p className="text-xs font-medium text-slate-500 dark:text-slate-400">Draft / Pending</p>
              <p className="text-2xl font-bold tracking-tight text-slate-600 dark:text-slate-400 mt-0.5">
                {stats.draft}
              </p>
            </div>
            <div className="h-9 w-9 rounded-xl bg-slate-100 dark:bg-slate-800 text-slate-400 flex items-center justify-center">
              <Clock className="h-4 w-4" />
            </div>
          </div>
        </Card>
      </div>

      {/* Search and Filters Bar */}
      <div className="flex flex-col sm:flex-row gap-3 items-center justify-between bg-white dark:bg-slate-900 p-3 rounded-2xl border border-slate-200 dark:border-slate-800 shadow-xs">
        <div className="relative w-full sm:w-80">
          <Search className="absolute left-3 top-1/2 -translate-y-1/2 h-3.5 w-3.5 text-slate-400" />
          <Input
            type="text"
            placeholder="Cari judul kampanye, ID, deskripsi..."
            value={searchQuery}
            onChange={(e) => setSearchQuery(e.target.value)}
            className="pl-9 h-9 text-xs bg-slate-50 dark:bg-slate-800/50"
          />
        </div>

        {/* Status Filters */}
        <div className="flex items-center rounded-lg bg-slate-100 dark:bg-slate-800 p-1 text-xs overflow-x-auto w-full sm:w-auto">
          {(['all', 'draft', 'pending', 'running', 'completed', 'failed'] as const).map((st) => (
            <button
              key={st}
              onClick={() => setStatusFilter(st)}
              className={`px-3 py-1 rounded-md font-semibold capitalize transition-all whitespace-nowrap ${
                statusFilter === st
                  ? 'bg-white dark:bg-slate-900 text-slate-900 dark:text-white shadow-xs'
                  : 'text-slate-500 hover:text-slate-900 dark:text-slate-400'
              }`}
            >
              {st}
            </button>
          ))}
        </div>
      </div>

      {/* Error alert */}
      {error && (
        <Alert variant="destructive">
          <AlertCircle className="h-4 w-4" />
          <AlertTitle>Kesalahan Jaringan</AlertTitle>
          <AlertDescription>{error}</AlertDescription>
        </Alert>
      )}

      {/* Campaigns List / Empty State */}
      {filteredCampaigns.length === 0 ? (
        <div className="py-16 px-4 text-center rounded-2xl border-2 border-dashed border-slate-200 dark:border-slate-800 bg-white/50 dark:bg-slate-900/50 flex flex-col items-center justify-center space-y-4">
          <div className="h-14 w-14 rounded-2xl bg-emerald-100 dark:bg-emerald-950/60 text-emerald-600 dark:text-emerald-400 flex items-center justify-center shadow-inner">
            <Rocket className="h-7 w-7" />
          </div>
          <div className="max-w-md space-y-1">
            <h3 className="font-bold text-base text-slate-900 dark:text-white">
              {campaigns.length === 0 ? 'Belum Ada Kampanye Batch' : 'Tidak Ada Kampanye yang Cocok'}
            </h3>
            <p className="text-xs text-slate-500 dark:text-slate-400">
              {campaigns.length === 0
                ? 'Buat kampanye batch baru untuk menyebarkan perubahan skrip atau perintah shell ke puluhan proyek WordPress / AdSense secara serempak.'
                : 'Ubah kata kunci pencarian atau bersihkan filter status untuk melihat kampanye lain.'}
            </p>
          </div>
          {campaigns.length === 0 && (
            <Button
              onClick={onOpenCreateModal}
              className="mt-2 bg-emerald-600 hover:bg-emerald-500 text-white text-xs font-semibold px-4 h-9 shadow-md shadow-emerald-600/20"
            >
              <Plus className="h-4 w-4 mr-1.5" />
              Buat Kampanye Pertama
            </Button>
          )}
        </div>
      ) : (
        <div className="grid grid-cols-1 gap-4">
          {filteredCampaigns.map((c) => {
            const rollup = rollups[c.id]
            const totalTasks = rollup?.total_target ?? (c.target_criteria?.project_ids?.length || 0)
            const completedTasks = rollup?.completed_tasks ?? 0
            const runningTasks = rollup?.running_tasks ?? 0
            const failedTasks = rollup?.failed_tasks ?? 0
            const successPct = rollup?.success_percentage ?? 0
            const currentStatus = rollup?.status || c.status
            const isExecuting = executingId === c.id

            // Extract target info
            const targetTags = c.target_criteria?.tags || []
            const targetCount = c.target_criteria?.project_ids?.length || 0
            const isAll = c.target_criteria?.all

            return (
              <Card
                key={c.id}
                onClick={() => onSelectCampaign(c.id)}
                className="p-5 bg-white dark:bg-slate-900 border-slate-200 dark:border-slate-800 hover:border-emerald-500/50 hover:shadow-md transition-all cursor-pointer group"
              >
                <div className="flex flex-col lg:flex-row lg:items-center justify-between gap-4">
                  {/* Left Column: Title & Metadata */}
                  <div className="space-y-2 flex-1 min-w-0">
                    <div className="flex items-center gap-2.5 flex-wrap">
                      {renderStatusBadge(currentStatus)}
                      <h3 className="font-bold text-sm sm:text-base text-slate-900 dark:text-white group-hover:text-emerald-600 dark:group-hover:text-emerald-400 transition-colors truncate">
                        {c.title}
                      </h3>
                      <span className="text-[10px] text-slate-400 font-mono">
                        {c.id.slice(0, 8)}...
                      </span>
                    </div>

                    {c.description && (
                      <p className="text-xs text-slate-500 dark:text-slate-400 line-clamp-2">
                        {c.description}
                      </p>
                    )}

                    <div className="flex items-center gap-3 text-[11px] text-slate-400 flex-wrap pt-1">
                      <span className="flex items-center gap-1">
                        <Layers className="h-3 w-3 text-slate-400" />
                        Target: {isAll ? 'Semua Proyek' : `${targetCount} Proyek`}
                      </span>

                      {targetTags.length > 0 && (
                        <div className="flex items-center gap-1">
                          {targetTags.map((tg: string) => (
                            <span
                              key={tg}
                              className="text-[10px] px-1.5 py-0.2 rounded bg-slate-100 dark:bg-slate-800 text-slate-600 dark:text-slate-300 font-mono"
                            >
                              tag:{tg}
                            </span>
                          ))}
                        </div>
                      )}

                      <span className="flex items-center gap-1">
                        <Clock className="h-3 w-3 text-slate-400" />
                        {new Date(c.created_at).toLocaleString('id-ID', {
                          dateStyle: 'medium',
                          timeStyle: 'short',
                        })}
                      </span>
                    </div>
                  </div>

                  {/* Right Column: Progress Bar & Actions */}
                  <div className="flex flex-col sm:flex-row sm:items-center gap-4 lg:w-96 shrink-0 pt-2 lg:pt-0 border-t lg:border-t-0 border-slate-100 dark:border-slate-800">
                    {/* Aggregate Progress Bar */}
                    <div className="flex-1 space-y-1.5">
                      <div className="flex items-center justify-between text-xs">
                        <span className="font-medium text-slate-700 dark:text-slate-300">
                          Progress Rollup
                        </span>
                        <span className="font-mono font-bold text-emerald-600 dark:text-emerald-400">
                          {completedTasks}/{totalTasks} ({successPct}%)
                        </span>
                      </div>

                      <div className="h-2.5 w-full bg-slate-100 dark:bg-slate-800 rounded-full overflow-hidden flex">
                        {/* Completed (green) */}
                        <div
                          style={{ width: `${totalTasks > 0 ? (completedTasks / totalTasks) * 100 : 0}%` }}
                          className="bg-emerald-500 h-full transition-all duration-500"
                        />
                        {/* Running (blue) */}
                        <div
                          style={{ width: `${totalTasks > 0 ? (runningTasks / totalTasks) * 100 : 0}%` }}
                          className="bg-blue-500 h-full animate-pulse transition-all duration-500"
                        />
                        {/* Failed (red) */}
                        <div
                          style={{ width: `${totalTasks > 0 ? (failedTasks / totalTasks) * 100 : 0}%` }}
                          className="bg-red-500 h-full transition-all duration-500"
                        />
                      </div>

                      {/* Mini counters */}
                      <div className="flex items-center gap-3 text-[10px] text-slate-400 font-mono">
                        <span className="text-emerald-600 dark:text-emerald-400">{completedTasks} done</span>
                        {runningTasks > 0 && <span className="text-blue-500">{runningTasks} doing</span>}
                        {failedTasks > 0 && <span className="text-red-500">{failedTasks} fail</span>}
                        <span>{totalTasks - completedTasks - runningTasks - failedTasks} pending</span>
                      </div>
                    </div>

                    {/* Quick Action Buttons */}
                    <div className="flex items-center gap-2 shrink-0">
                      {currentStatus !== 'running' && currentStatus !== 'completed' && (
                        <Button
                          size="sm"
                          onClick={(e) => handleExecute(c, e)}
                          disabled={isExecuting}
                          className="h-8 text-xs bg-emerald-600 hover:bg-emerald-500 text-white font-semibold px-3 shadow-xs"
                          title="Jalankan otomatisasi worker pool fan-out"
                        >
                          <Play className={`h-3 w-3 mr-1 fill-current ${isExecuting ? 'animate-spin' : ''}`} />
                          {isExecuting ? 'Running...' : 'Execute'}
                        </Button>
                      )}

                      <Button
                        variant="outline"
                        size="sm"
                        onClick={(e) => {
                          e.stopPropagation()
                          onSelectCampaign(c.id)
                        }}
                        className="h-8 text-xs px-2.5"
                      >
                        <span>Details</span>
                        <ChevronRight className="h-3.5 w-3.5 ml-1 text-slate-400" />
                      </Button>
                    </div>
                  </div>
                </div>
              </Card>
            )
          })}
        </div>
      )}
    </div>
  )
}
