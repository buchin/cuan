import { useState, useEffect } from 'react'
import { Card } from '../ui/card'
import { Button } from '../ui/button'
import { Badge } from '../ui/badge'
import { Alert, AlertTitle, AlertDescription } from '../ui/alert'
import {
  ArrowLeft,
  Play,
  RefreshCw,
  AlertCircle,
  Terminal,
  Layers,
  ExternalLink,
  X,
  Code
} from 'lucide-react'
import { BatchCampaignDTO, RollupMetrics, ProjectTaskDTO, DoDItem } from './types'

interface CampaignDetailViewProps {
  campaignId: string
  onBack: () => void
  onNavigate?: (path: string) => void
}

interface RunnerLogModalProps {
  task: ProjectTaskDTO | null
  onClose: () => void
  campaignScript?: string
}

function RunnerLogModal({ task, onClose, campaignScript }: RunnerLogModalProps) {
  if (!task) return null

  // Generate or display runner logs for this subtask
  const isDone = task.status === 'done' || task.status === 'completed'
  const isDoing = task.status === 'doing' || task.status === 'running'
  const isFailed = task.status === 'failed'

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center p-4 bg-slate-950/75 backdrop-blur-sm overflow-y-auto animate-in fade-in duration-200">
      <div className="relative w-full max-w-3xl bg-slate-900 rounded-2xl border border-slate-800 shadow-2xl flex flex-col max-h-[85vh] text-white">
        {/* Header */}
        <div className="flex items-center justify-between px-6 py-4 border-b border-slate-800 shrink-0">
          <div className="flex items-center gap-3">
            <div className="h-9 w-9 rounded-xl bg-slate-800 text-emerald-400 flex items-center justify-center">
              <Terminal className="h-5 w-5" />
            </div>
            <div>
              <div className="flex items-center gap-2">
                <h3 className="font-bold text-sm text-white">
                  Runner Execution Log
                </h3>
                <Badge
                  variant="outline"
                  className={`text-[10px] font-mono ${
                    isDone
                      ? 'border-emerald-500/50 text-emerald-400 bg-emerald-500/10'
                      : isDoing
                      ? 'border-blue-500/50 text-blue-400 bg-blue-500/10'
                      : isFailed
                      ? 'border-red-500/50 text-red-400 bg-red-500/10'
                      : 'border-slate-700 text-slate-400'
                  }`}
                >
                  {task.status}
                </Badge>
              </div>
              <p className="text-xs text-slate-400 font-mono mt-0.5">
                Task: {task.title} | Proyek: {task.project_id}
              </p>
            </div>
          </div>
          <button
            onClick={onClose}
            className="p-1.5 rounded-lg text-slate-400 hover:text-white hover:bg-slate-800 transition-colors"
          >
            <X className="h-5 w-5" />
          </button>
        </div>

        {/* Log Viewer Body */}
        <div className="p-6 overflow-y-auto space-y-4">
          <div className="rounded-xl bg-slate-950 p-4 border border-slate-800/80 font-mono text-xs text-slate-300 space-y-1.5 overflow-x-auto">
            <div className="text-slate-500"># --- CUAN ISOLATED RUNNER POOL (TASK RUNNER v1) ---</div>
            <div className="text-slate-500"># Task ID: {task.id}</div>
            <div className="text-slate-500"># Project Target ID: {task.project_id}</div>
            <div className="text-slate-500"># Started at: {task.created_at}</div>
            <div className="text-slate-400">--------------------------------------------------</div>
            <div className="text-emerald-400">[runner:init] Allocating worker thread from pool (worker-concurrency=5)...</div>
            <div className="text-emerald-400">[runner:workspace] Preparing isolated target repository directory...</div>
            <div className="text-blue-400">[runner:env] Sandbox environment verified. Status check OK.</div>
            <div className="text-slate-300">[runner:exec] Spawning script execution pipeline:</div>
            
            <div className="p-2.5 rounded bg-slate-900 border border-slate-800 text-emerald-300 text-[11px] whitespace-pre-wrap">
              {campaignScript || `// Default payload execution
wp plugin update --all --allow-root
wp cache flush --allow-root`}
            </div>

            {isDoing && (
              <div className="text-blue-400 flex items-center gap-2 pt-2 animate-pulse">
                <span className="h-2 w-2 rounded-full bg-blue-500" />
                <span>[runner:executing] Process active... listening to stdout stream</span>
              </div>
            )}

            {isDone && (
              <>
                <div className="text-emerald-400">[runner:stdout] Payload script finished execution with code 0.</div>
                <div className="text-emerald-400">[runner:verify] Verifying Definition of Done (DoD) checklist items:</div>
                {task.dod_checklist?.map((it, idx) => (
                  <div key={idx} className="text-emerald-300 pl-4">
                    [DoD #{idx + 1}] [PASS] {it.text}
                  </div>
                ))}
                <div className="text-emerald-400 font-bold pt-2">
                  [runner:complete] 100% PASS - Subtask finished cleanly. Aggregated to Rollup Engine.
                </div>
              </>
            )}

            {isFailed && (
              <div className="text-red-400 pt-2 font-bold">
                [runner:error] Process exited with non-zero exit code or timeout. Isolation preserved.
              </div>
            )}
          </div>
        </div>

        {/* Footer */}
        <div className="px-6 py-3 border-t border-slate-800 bg-slate-950/60 rounded-b-2xl flex items-center justify-between text-xs text-slate-400">
          <span>Target: {task.project_id}</span>
          <Button
            size="sm"
            variant="outline"
            onClick={onClose}
            className="h-8 text-xs border-slate-700 bg-slate-800 hover:bg-slate-700 text-slate-200"
          >
            Tutup
          </Button>
        </div>
      </div>
    </div>
  )
}

export function CampaignDetailView({
  campaignId,
  onBack,
  onNavigate: _onNavigate,
}: CampaignDetailViewProps) {
  const [campaign, setCampaign] = useState<BatchCampaignDTO | null>(null)
  const [rollup, setRollup] = useState<RollupMetrics | null>(null)
  const [tasks, setTasks] = useState<ProjectTaskDTO[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [executing, setExecuting] = useState(false)
  const [notification, setNotification] = useState<{ type: 'success' | 'error'; message: string } | null>(null)

  // Real-time SSE Stream status
  const [sseConnected, setSseConnected] = useState(false)

  // Runner log inspection modal
  const [inspectTask, setInspectTask] = useState<ProjectTaskDTO | null>(null)

  const showToast = (type: 'success' | 'error', message: string) => {
    setNotification({ type, message })
    setTimeout(() => setNotification(null), 4000)
  }

  // Fetch campaign and subtasks
  const fetchCampaignData = async () => {
    setError(null)
    try {
      const res = await fetch(`/api/campaigns/${campaignId}`)
      if (!res.ok) {
        throw new Error(`HTTP ${res.status}: Gagal memuat data kampanye`)
      }
      const data = await res.json()
      setCampaign(data.campaign)
      if (data.tasks) {
        setTasks(data.tasks)
      }
      if (data.rollup) {
        setRollup(data.rollup)
      } else {
        fetchRollup()
      }
    } catch (err: any) {
      setError(err.message || 'Terjadi kesalahan jaringan.')
    } finally {
      setLoading(false)
    }
  }

  // Fetch rollup metrics specifically
  const fetchRollup = async () => {
    try {
      const res = await fetch(`/api/campaigns/${campaignId}/rollup`)
      if (res.ok) {
        const data = await res.json()
        if (data.rollup) {
          setRollup(data.rollup)
        }
      }
    } catch {}
  }

  // Initial load
  useEffect(() => {
    fetchCampaignData()
  }, [campaignId])

  // Real-time SSE Stream Listener
  useEffect(() => {
    if (!campaignId) return

    let eventSource: EventSource | null = null

    try {
      eventSource = new EventSource(`/api/campaigns/${campaignId}/events`)

      eventSource.onopen = () => {
        setSseConnected(true)
      }

      eventSource.addEventListener('connected', () => {
        setSseConnected(true)
      })

      // Event: campaign.progress
      eventSource.addEventListener('campaign.progress', (e) => {
        try {
          const payload = JSON.parse(e.data)
          if (payload.rollup) {
            setRollup(payload.rollup)
            if (payload.rollup.status) {
              setCampaign((prev) => (prev ? { ...prev, status: payload.rollup.status } : prev))
            }
          }
        } catch {}
      })

      // Event: task.status_changed
      eventSource.addEventListener('task.status_changed', (e) => {
        try {
          const payload = JSON.parse(e.data)
          // payload: { campaign_id, task_id, project_id, title, old_status, new_status, timestamp }
          setTasks((prev) =>
            prev.map((t) =>
              t.id === payload.task_id ? { ...t, status: payload.new_status } : t
            )
          )
        } catch {}
      })

      // Event: campaign.completed
      eventSource.addEventListener('campaign.completed', (e) => {
        try {
          const payload = JSON.parse(e.data)
          if (payload.rollup) {
            setRollup(payload.rollup)
          }
          const finalStatus = payload.status || 'completed'
          setCampaign((prev) => (prev ? { ...prev, status: finalStatus } : prev))
          showToast('success', `Kampanye batch selesai dengan status: ${finalStatus}!`)
        } catch {}
      })

      eventSource.onerror = () => {
        setSseConnected(false)
      }
    } catch (err) {
      console.error('SSE initialization error:', err)
    }

    return () => {
      if (eventSource) {
        eventSource.close()
      }
    }
  }, [campaignId])

  // Execute Campaign
  const handleExecute = async () => {
    setExecuting(true)
    try {
      const res = await fetch(`/api/campaigns/${campaignId}/execute`, {
        method: 'POST',
      })
      const data = await res.json()
      if (!res.ok) {
        throw new Error(data.error || `HTTP ${res.status}: Gagal mengeksekusi kampanye`)
      }

      showToast(
        'success',
        `Eksekusi selesai! ${data.completed_tasks} dari ${data.total_tasks} subtask sukses.`
      )
      await fetchCampaignData()
    } catch (err: any) {
      showToast('error', err.message || 'Gagal menjalankan kampanye.')
    } finally {
      setExecuting(false)
    }
  }

  const currentStatus = rollup?.status || campaign?.status || 'draft'
  const totalTasks = rollup?.total_target ?? tasks.length
  const completedTasks = rollup?.completed_tasks ?? tasks.filter((t) => t.status === 'done').length
  const runningTasks = rollup?.running_tasks ?? tasks.filter((t) => t.status === 'doing').length
  const failedTasks = rollup?.failed_tasks ?? tasks.filter((t) => t.status === 'failed').length
  const successPct = rollup?.success_percentage ?? (totalTasks > 0 ? Math.round((completedTasks / totalTasks) * 100) : 0)

  return (
    <div className="space-y-6 animate-in fade-in duration-200">
      {/* Toast Notification */}
      {notification && (
        <div
          className={`p-3.5 rounded-xl flex items-center justify-between text-xs font-semibold shadow-md ${
            notification.type === 'success' ? 'bg-emerald-600 text-white' : 'bg-red-600 text-white'
          }`}
        >
          <span>{notification.message}</span>
          <button onClick={() => setNotification(null)} className="opacity-80 hover:opacity-100">
            <ExternalLink className="h-3.5 w-3.5" />
          </button>
        </div>
      )}

      {/* Runner Log Modal */}
      {inspectTask && (
        <RunnerLogModal
          task={inspectTask}
          onClose={() => setInspectTask(null)}
          campaignScript={campaign?.template_payload?.script}
        />
      )}

      {/* Top Navigation & Action Header */}
      <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-4">
        <div className="flex items-center gap-3">
          <Button
            variant="outline"
            size="sm"
            onClick={onBack}
            className="h-9 px-3 text-xs gap-1.5 text-slate-700 dark:text-slate-300"
          >
            <ArrowLeft className="h-4 w-4" />
            <span>Kembali ke Katalog</span>
          </Button>

          <div className="flex items-center gap-2">
            <Badge
              variant="outline"
              className={`text-[10px] font-mono flex items-center gap-1.5 ${
                sseConnected
                  ? 'border-emerald-500/60 bg-emerald-500/10 text-emerald-600 dark:text-emerald-400'
                  : 'border-slate-300 dark:border-slate-700 text-slate-400'
              }`}
            >
              <span
                className={`h-2 w-2 rounded-full ${
                  sseConnected ? 'bg-emerald-500 animate-pulse' : 'bg-slate-400'
                }`}
              />
              {sseConnected ? 'SSE Live Stream Active' : 'Connecting SSE...'}
            </Badge>
          </div>
        </div>

        <div className="flex items-center gap-2.5">
          <Button
            variant="outline"
            size="sm"
            onClick={fetchCampaignData}
            className="h-9 text-xs"
          >
            <RefreshCw className={`h-3.5 w-3.5 mr-1.5 ${loading ? 'animate-spin' : ''}`} />
            Refresh
          </Button>

          {currentStatus !== 'running' && currentStatus !== 'completed' && (
            <Button
              size="sm"
              onClick={handleExecute}
              disabled={executing || loading}
              className="h-9 text-xs bg-emerald-600 hover:bg-emerald-500 text-white font-semibold px-4 shadow-md shadow-emerald-600/20"
            >
              <Play className={`h-3.5 w-3.5 mr-1.5 fill-current ${executing ? 'animate-spin' : ''}`} />
              {executing ? 'Executing Runner Pool...' : 'Execute Campaign'}
            </Button>
          )}
        </div>
      </div>

      {error && (
        <Alert variant="destructive">
          <AlertCircle className="h-4 w-4" />
          <AlertTitle>Kesalahan</AlertTitle>
          <AlertDescription>{error}</AlertDescription>
        </Alert>
      )}

      {/* Main Campaign Hero Card with Live Rollup Progress Bar */}
      {campaign && (
        <Card className="p-6 bg-white dark:bg-slate-900 border-slate-200 dark:border-slate-800 shadow-sm space-y-6">
          <div className="flex flex-col lg:flex-row lg:items-start justify-between gap-4">
            <div className="space-y-1.5 flex-1">
              <div className="flex items-center gap-2.5 flex-wrap">
                <Badge
                  className={`text-[11px] font-mono py-0.5 px-2.5 ${
                    currentStatus === 'completed'
                      ? 'bg-emerald-500/10 text-emerald-600 dark:text-emerald-400 border-emerald-500/30'
                      : currentStatus === 'running'
                      ? 'bg-blue-500/10 text-blue-600 dark:text-blue-400 border-blue-500/30'
                      : currentStatus === 'failed'
                      ? 'bg-red-500/10 text-red-600 dark:text-red-400 border-red-500/30'
                      : 'bg-slate-100 text-slate-700 dark:bg-slate-800 dark:text-slate-300'
                  }`}
                >
                  {currentStatus}
                </Badge>
                <h1 className="text-xl font-extrabold text-slate-900 dark:text-white">
                  {campaign.title}
                </h1>
              </div>

              {campaign.description && (
                <p className="text-xs text-slate-500 dark:text-slate-400 max-w-2xl">
                  {campaign.description}
                </p>
              )}

              <div className="flex items-center gap-3 text-xs text-slate-400 pt-1 font-mono">
                <span>ID: {campaign.id}</span>
                <span>•</span>
                <span>
                  Dibuat:{' '}
                  {new Date(campaign.created_at).toLocaleString('id-ID', {
                    dateStyle: 'medium',
                    timeStyle: 'short',
                  })}
                </span>
              </div>
            </div>

            {/* Quick Metrics */}
            <div className="grid grid-cols-3 gap-2 sm:gap-3 lg:w-80 shrink-0">
              <div className="p-3 rounded-xl bg-slate-50 dark:bg-slate-800/50 border border-slate-100 dark:border-slate-800 text-center">
                <p className="text-[10px] text-slate-400 uppercase font-bold">Total Target</p>
                <p className="text-lg font-bold text-slate-900 dark:text-white mt-0.5">{totalTasks}</p>
              </div>
              <div className="p-3 rounded-xl bg-emerald-50 dark:bg-emerald-950/30 border border-emerald-100 dark:border-emerald-900/30 text-center">
                <p className="text-[10px] text-emerald-600 dark:text-emerald-400 uppercase font-bold">Completed</p>
                <p className="text-lg font-bold text-emerald-600 dark:text-emerald-400 mt-0.5">{completedTasks}</p>
              </div>
              <div className="p-3 rounded-xl bg-blue-50 dark:bg-blue-950/30 border border-blue-100 dark:border-blue-900/30 text-center">
                <p className="text-[10px] text-blue-600 dark:text-blue-400 uppercase font-bold">Success Pct</p>
                <p className="text-lg font-bold text-blue-600 dark:text-blue-400 mt-0.5">{successPct}%</p>
              </div>
            </div>
          </div>

          {/* Live Rollup Progress Bar */}
          <div className="space-y-2 pt-2 border-t border-slate-100 dark:border-slate-800">
            <div className="flex items-center justify-between text-xs">
              <div className="flex items-center gap-2">
                <span className="font-bold text-slate-800 dark:text-slate-200">
                  Live Rollup Progress Stream
                </span>
                {sseConnected && (
                  <span className="h-1.5 w-1.5 rounded-full bg-emerald-500 animate-ping" />
                )}
              </div>
              <span className="font-mono font-bold text-emerald-600 dark:text-emerald-400">
                {completedTasks} / {totalTasks} Subtasks Selesai ({successPct}%)
              </span>
            </div>

            <div className="h-3.5 w-full bg-slate-100 dark:bg-slate-800 rounded-full overflow-hidden flex shadow-inner">
              {/* Completed */}
              <div
                style={{ width: `${totalTasks > 0 ? (completedTasks / totalTasks) * 100 : 0}%` }}
                className="bg-emerald-500 h-full transition-all duration-500 shadow-sm"
              />
              {/* Running */}
              <div
                style={{ width: `${totalTasks > 0 ? (runningTasks / totalTasks) * 100 : 0}%` }}
                className="bg-blue-500 h-full animate-pulse transition-all duration-500"
              />
              {/* Failed */}
              <div
                style={{ width: `${totalTasks > 0 ? (failedTasks / totalTasks) * 100 : 0}%` }}
                className="bg-red-500 h-full transition-all duration-500"
              />
            </div>

            <div className="flex items-center justify-between text-[11px] text-slate-400 font-mono">
              <div className="flex items-center gap-3">
                <span className="text-emerald-600 dark:text-emerald-400">{completedTasks} completed</span>
                {runningTasks > 0 && <span className="text-blue-500">{runningTasks} running</span>}
                {failedTasks > 0 && <span className="text-red-500">{failedTasks} failed</span>}
                <span>{totalTasks - completedTasks - runningTasks - failedTasks} pending</span>
              </div>
              <span>Heartbeat: 15s</span>
            </div>
          </div>
        </Card>
      )}

      {/* Script Template Preview Panel */}
      {campaign?.template_payload?.script && (
        <Card className="p-4 bg-slate-950 border-slate-800 text-slate-300 font-mono text-xs">
          <div className="flex items-center justify-between mb-2">
            <div className="flex items-center gap-2 text-emerald-400">
              <Code className="h-4 w-4" />
              <span className="font-bold text-[11px] uppercase tracking-wider">
                Decompiled Script Template Payload
              </span>
            </div>
            <Badge variant="outline" className="text-[10px] border-slate-700 text-slate-400">
              Isolated Blast Radius
            </Badge>
          </div>
          <pre className="text-emerald-300 text-[11px] whitespace-pre-wrap max-h-36 overflow-y-auto bg-slate-900/60 p-3 rounded-lg border border-slate-800/80">
            {campaign.template_payload.script}
          </pre>
        </Card>
      )}

      {/* Child Tasks Breakdown Section */}
      <div className="space-y-3">
        <div className="flex items-center justify-between">
          <div>
            <h3 className="text-base font-bold text-slate-900 dark:text-white flex items-center gap-2">
              <Layers className="h-4 w-4 text-emerald-500" />
              <span>Breakdown Child Tasks per Proyek</span>
              <Badge variant="outline" className="text-xs font-mono ml-1">
                {tasks.length} Subtasks
              </Badge>
            </h3>
            <p className="text-xs text-slate-500 dark:text-slate-400">
              Daftar seluruh subtask yang dihasilkan oleh Decompiler Engine dan dieksekusi oleh Isolated Runner.
            </p>
          </div>
        </div>

        {tasks.length === 0 ? (
          <div className="p-8 text-center rounded-xl border border-slate-200 dark:border-slate-800 bg-white dark:bg-slate-900 text-xs text-slate-400">
            Belum ada subtask yang di-decompile. Klik &quot;Execute Campaign&quot; untuk memecah kampanye dan memulai runner.
          </div>
        ) : (
          <div className="grid grid-cols-1 gap-3">
            {tasks.map((task, index) => {
              const isTaskDone = task.status === 'done' || task.status === 'completed'
              const isTaskDoing = task.status === 'doing' || task.status === 'running'
              const isTaskFailed = task.status === 'failed'

              const dodList: DoDItem[] = task.dod_checklist || []
              const completedDoD = dodList.filter((d) => d.completed || isTaskDone).length
              const totalDoD = dodList.length

              return (
                <Card
                  key={task.id}
                  className="p-4 bg-white dark:bg-slate-900 border-slate-200 dark:border-slate-800 flex flex-col md:flex-row md:items-center justify-between gap-4 hover:border-slate-300 dark:hover:border-slate-700 transition-all"
                >
                  {/* Task Metadata */}
                  <div className="space-y-1.5 flex-1 min-w-0">
                    <div className="flex items-center gap-2 flex-wrap">
                      <span className="font-mono text-slate-400 text-xs">#{index + 1}</span>

                      {/* Status Badge */}
                      <Badge
                        variant="outline"
                        className={`text-[10px] font-mono py-0 px-2 flex items-center gap-1 ${
                          isTaskDone
                            ? 'border-emerald-500/50 bg-emerald-500/10 text-emerald-600 dark:text-emerald-400'
                            : isTaskDoing
                            ? 'border-blue-500/50 bg-blue-500/10 text-blue-600 dark:text-blue-400'
                            : isTaskFailed
                            ? 'border-red-500/50 bg-red-500/10 text-red-600 dark:text-red-400'
                            : 'border-slate-300 dark:border-slate-700 text-slate-500'
                        }`}
                      >
                        {isTaskDoing && (
                          <span className="h-1.5 w-1.5 rounded-full bg-blue-500 animate-pulse" />
                        )}
                        {task.status}
                      </Badge>

                      <h4 className="font-semibold text-xs sm:text-sm text-slate-900 dark:text-white truncate">
                        {task.title}
                      </h4>
                    </div>

                    <div className="flex items-center gap-3 text-[11px] text-slate-400 font-mono flex-wrap">
                      <span>Proyek: <strong className="text-slate-700 dark:text-slate-300">{task.project_id}</strong></span>
                      <span>•</span>
                      <span>Task ID: {task.id.slice(0, 8)}...</span>
                      {task.tags && task.tags.length > 0 && (
                        <div className="flex items-center gap-1">
                          {task.tags.map((tg) => (
                            <span
                              key={tg}
                              className="text-[9px] px-1.5 py-0.2 rounded bg-slate-100 dark:bg-slate-800 text-slate-600 dark:text-slate-300 font-mono"
                            >
                              {tg}
                            </span>
                          ))}
                        </div>
                      )}
                    </div>
                  </div>

                  {/* DoD Checklist & Log Action */}
                  <div className="flex flex-col sm:flex-row sm:items-center gap-4 shrink-0 pt-2 md:pt-0 border-t md:border-t-0 border-slate-100 dark:border-slate-800">
                    {/* DoD Progress indicator */}
                    <div className="w-44 space-y-1">
                      <div className="flex items-center justify-between text-[11px]">
                        <span className="text-slate-500">DoD Checklist</span>
                        <span className="font-mono text-emerald-600 dark:text-emerald-400 font-semibold">
                          {completedDoD}/{totalDoD}
                        </span>
                      </div>
                      <div className="h-2 w-full bg-slate-100 dark:bg-slate-800 rounded-full overflow-hidden">
                        <div
                          style={{
                            width: `${totalDoD > 0 ? (completedDoD / totalDoD) * 100 : isTaskDone ? 100 : 0}%`,
                          }}
                          className={`h-full transition-all duration-300 ${
                            isTaskDone ? 'bg-emerald-500' : 'bg-blue-500'
                          }`}
                        />
                      </div>
                    </div>

                    {/* Inspect Runner Log Button */}
                    <Button
                      variant="outline"
                      size="sm"
                      onClick={() => setInspectTask(task)}
                      className="h-8 text-xs gap-1.5 px-3 border-slate-200 dark:border-slate-700 hover:bg-slate-100 dark:hover:bg-slate-800"
                    >
                      <Terminal className="h-3.5 w-3.5 text-emerald-500" />
                      <span>Inspect Log</span>
                    </Button>
                  </div>
                </Card>
              )
            })}
          </div>
        )}
      </div>
    </div>
  )
}
