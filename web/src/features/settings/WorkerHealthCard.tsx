import { useState, useEffect, useCallback } from 'react'
import { RefreshCw } from 'lucide-react'
import { useLanguage } from '../../context/LanguageContext'
import { GroupedSection } from '../../components/layout/GroupedSection'
import { StatusDot } from '../../components/layout/StatusDot'

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
    <GroupedSection
      data-testid="worker-health-card"
      title={t('Kesehatan Worker & Antrean')}
      description={t('Status daemon cuan-runner, kedalaman antrean tugas ruang kerja, dan detak jantung terakhir.')}
      className="mb-4"
    >
      {error && (
        <div className="px-3.5 py-2 text-xs text-destructive bg-destructive/10 border-b border-border/30">
          {error}
        </div>
      )}

      {/* Single calm status row */}
      <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-3 px-3.5 py-3 sm:px-4 sm:py-3.5 min-h-[44px] text-sm">
        {/* Left: StatusDot + Details in one calm line */}
        <div className="flex items-center gap-3 min-w-0 flex-1">
          <StatusDot
            status={isReady ? 'healthy' : isBusy ? 'warning' : 'exception'}
            size="md"
            pulse={isBusy}
            title={isReady ? t('Aktif / Siap') : isBusy ? t('Sibuk / Memproses') : t('Perhatian / Tidak Aktif')}
          />
          <div className="min-w-0 flex-1">
            <div className="flex flex-wrap items-center gap-2">
              <span className="text-sm font-semibold text-foreground">
                cuan-runner
              </span>
              <span
                data-testid="worker-health-status"
                className={`inline-flex items-center text-xs font-medium px-2 py-0.5 rounded-full border ${
                  isReady
                    ? 'border-emerald-500/30 text-emerald-600 dark:text-emerald-400'
                    : isBusy
                    ? 'border-amber-500/30 text-amber-600 dark:text-amber-400'
                    : 'border-destructive/30 text-destructive'
                }`}
              >
                {isReady ? t('Aktif / Siap') : isBusy ? t('Sibuk / Memproses') : t('Perhatian / Tidak Aktif')}
              </span>
            </div>
            <div className="mt-1 flex flex-wrap items-center gap-x-3 gap-y-0.5 text-xs text-muted-foreground">
              <span data-testid="worker-queue-depth">
                {t('Antrean')}: <strong className="font-semibold text-foreground">{health?.queue_depth ?? 0}</strong>{' '}
                ({t('Menunggu')}: {health?.queued_count ?? 0}, {t('Berjalan')}: {health?.running_count ?? 0})
              </span>
              <span>•</span>
              <span data-testid="worker-heartbeat" className="truncate">
                {t('Detak Jantung')}: {formattedHeartbeat()}
              </span>
            </div>
          </div>
        </div>

        {/* Right: Refresh button */}
        <div className="flex shrink-0 items-center self-end sm:self-center">
          <button
            type="button"
            data-testid="worker-refresh-btn"
            onClick={() => void fetchWorkerHealth(true)}
            disabled={loading}
            className="action-secondary min-h-11 sm:min-h-8 inline-flex items-center gap-1.5 px-3 py-1 text-xs font-medium"
            title={t('Periksa Status')}
            aria-label={t('Periksa Status')}
          >
            <RefreshCw className={`h-3.5 w-3.5 ${loading ? 'animate-spin' : ''}`} />
            <span>{loading ? t('Memeriksa...') : t('Periksa Status')}</span>
          </button>
        </div>
      </div>
    </GroupedSection>
  )
}
