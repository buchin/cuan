import { useState, useEffect, useCallback } from 'react'
import { Activity, RefreshCw, CheckCircle2, Clock, AlertTriangle, Layers, Server } from 'lucide-react'
import { useLanguage } from '../../context/LanguageContext'

export interface WorkerHealthInfo {
  status: 'ready' | 'busy' | 'attention' | string
  runner_id: string
  runner_name?: string
  hostname?: string
  is_alive: boolean
  last_heartbeat_at: string
  last_seen_seconds_ago: number
  queued_count: number
  running_count: number
  queue_depth: number
  timestamp: string
}

export function WorkerHealthCard() {
  const { t } = useLanguage()
  const [health, setHealth] = useState<WorkerHealthInfo | null>(null)
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)

  const fetchWorkerHealth = useCallback(async (isManual = false) => {
    try {
      if (isManual) setLoading(true)
      const res = await fetch('/api/system/worker-health')
      if (res.ok) {
        const data = await res.json()
        setHealth(data)
        setError(null)
      } else {
        setError('Gagal memuat status worker')
      }
    } catch {
      setError('Koneksi ke endpoint worker terputus')
    } finally {
      if (isManual) setLoading(false)
    }
  }, [])

  useEffect(() => {
    void fetchWorkerHealth(true)

    // Periodic refresh every 10 seconds while settings view is mounted
    const interval = setInterval(() => {
      if (typeof document !== 'undefined' && document.visibilityState === 'visible') {
        void fetchWorkerHealth(false)
      }
    }, 10000)

    return () => clearInterval(interval)
  }, [fetchWorkerHealth])

  const status = health?.status || (health?.is_alive ? 'ready' : 'attention')
  const isReady = status === 'ready'
  const isBusy = status === 'busy'
  const isAttention = status === 'attention' || (!isReady && !isBusy)

  const formattedHeartbeat = () => {
    if (!health || !health.last_heartbeat_at || health.last_seen_seconds_ago < 0) {
      return t('Belum ada detak jantung')
    }
    const secs = health.last_seen_seconds_ago
    if (secs < 10) return t('Baru saja (beberapa detik lalu)')
    if (secs < 60) return t(`${secs} detik yang lalu`)
    const mins = Math.floor(secs / 60)
    return t(`${mins} menit yang lalu`)
  }

  return (
    <section
      data-testid="worker-health-card"
      className="rounded-lg border border-border bg-card p-5 space-y-4"
    >
      {/* Header & Refresh */}
      <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-3">
        <div className="flex items-center gap-3">
          <div className="h-10 w-10 rounded-lg bg-primary/10 text-primary flex items-center justify-center shrink-0">
            <Activity className="h-5 w-5" />
          </div>
          <div>
            <h2 className="text-lg font-semibold text-foreground flex items-center gap-2">
              <span>{t('Kesehatan Worker & Antrean')}</span>
              <span className="text-xs px-2 py-0.5 rounded-full border border-border bg-muted/50 font-normal text-muted-foreground">
                cuan-runner
              </span>
            </h2>
            <p className="mt-0.5 text-sm text-muted-foreground">
              {t('Status daemon cuan-runner, kedalaman antrean tugas toko, dan detak jantung terakhir.')}
            </p>
          </div>
        </div>
        <button
          type="button"
          data-testid="worker-refresh-btn"
          onClick={() => void fetchWorkerHealth(true)}
          disabled={loading}
          className="action-secondary shrink-0 inline-flex items-center gap-1.5 self-start sm:self-auto text-xs"
        >
          <RefreshCw className={`h-3.5 w-3.5 ${loading ? 'animate-spin' : ''}`} />
          <span>{loading ? t('Memeriksa...') : t('Periksa Status')}</span>
        </button>
      </div>

      {error && (
        <div className="rounded-md border border-destructive/30 bg-destructive/10 px-3 py-2 text-xs text-destructive">
          {error}
        </div>
      )}

      {/* Main Grid: Status Indicator & Queue Depth */}
      <div className="grid grid-cols-1 sm:grid-cols-3 gap-3 pt-1">
        {/* Status Indicator Pill */}
        <div
          data-testid="worker-health-status"
          className={`rounded-lg border p-4 flex flex-col justify-between space-y-2 transition-colors ${
            isReady
              ? 'border-emerald-500/30 bg-emerald-500/5'
              : isBusy
              ? 'border-amber-500/30 bg-amber-500/5'
              : 'border-destructive/30 bg-destructive/5'
          }`}
        >
          <div className="flex items-center justify-between">
            <span className="text-xs font-medium uppercase tracking-wider text-muted-foreground">
              {t('Status Daemon')}
            </span>
            <span className="relative flex h-3 w-3">
              {(isReady || isBusy) && (
                <span
                  className={`animate-ping absolute inline-flex h-full w-full rounded-full opacity-75 ${
                    isReady ? 'bg-emerald-400' : 'bg-amber-400'
                  }`}
                />
              )}
              <span
                className={`relative inline-flex rounded-full h-3 w-3 ${
                  isReady ? 'bg-emerald-500' : isBusy ? 'bg-amber-500' : 'bg-destructive'
                }`}
              />
            </span>
          </div>
          <div>
            <div className="text-base font-semibold text-foreground flex items-center gap-1.5">
              {isReady && (
                <>
                  <CheckCircle2 className="h-4 w-4 text-emerald-500" />
                  <span>{t('Aktif / Siap')}</span>
                </>
              )}
              {isBusy && (
                <>
                  <Clock className="h-4 w-4 text-amber-500 animate-spin" />
                  <span>{t('Sibuk / Memproses')}</span>
                </>
              )}
              {isAttention && (
                <>
                  <AlertTriangle className="h-4 w-4 text-destructive" />
                  <span>{t('Perhatian / Tidak Aktif')}</span>
                </>
              )}
            </div>
            <p className="mt-1 text-xs text-muted-foreground">
              {isReady
                ? t('Worker siap menerima dan mengeksekusi tugas baru.')
                : isBusy
                ? t('Worker sedang memproses tugas antrean secara aktif.')
                : t('Tidak ada heartbeat terbaru dari daemon cuan-runner.')}
            </p>
          </div>
        </div>

        {/* Queue Depth Metric */}
        <div
          data-testid="worker-queue-depth"
          className="rounded-lg border border-border bg-card/60 p-4 flex flex-col justify-between space-y-2"
        >
          <div className="flex items-center justify-between text-muted-foreground">
            <span className="text-xs font-medium uppercase tracking-wider">
              {t('Kedalaman Antrean')}
            </span>
            <Layers className="h-4 w-4" />
          </div>
          <div>
            <div className="flex items-baseline gap-2">
              <span className="text-2xl font-bold tracking-tight text-foreground">
                {health?.queue_depth ?? 0}
              </span>
              <span className="text-xs text-muted-foreground">
                {t('tugas total')}
              </span>
            </div>
            <div className="mt-1 flex items-center gap-3 text-xs text-muted-foreground">
              <span>
                {t('Menunggu')}: <strong className="text-foreground">{health?.queued_count ?? 0}</strong>
              </span>
              <span>•</span>
              <span>
                {t('Berjalan')}: <strong className="text-foreground">{health?.running_count ?? 0}</strong>
              </span>
            </div>
          </div>
        </div>

        {/* Heartbeat & Runner Info */}
        <div
          data-testid="worker-heartbeat"
          className="rounded-lg border border-border bg-card/60 p-4 flex flex-col justify-between space-y-2"
        >
          <div className="flex items-center justify-between text-muted-foreground">
            <span className="text-xs font-medium uppercase tracking-wider">
              {t('Detak Jantung (Heartbeat)')}
            </span>
            <Server className="h-4 w-4" />
          </div>
          <div>
            <div className="text-sm font-semibold text-foreground truncate">
              {formattedHeartbeat()}
            </div>
            <p className="mt-1 text-xs text-muted-foreground truncate">
              {health?.hostname
                ? `${t('Host')}: ${health.hostname}`
                : `${t('Runner')}: ${health?.runner_id || 'cuan-runner'}`}
            </p>
          </div>
        </div>
      </div>
    </section>
  )
}
