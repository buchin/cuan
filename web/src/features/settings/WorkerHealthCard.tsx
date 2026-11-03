import { SlideDrawer } from '../../components/layout/SlideDrawer'
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
  const [showDetails, setShowDetails] = useState(false)
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

  return <>
    <GroupedSection data-testid="worker-health-card" title={t('Application status')}>
      <div className="flex min-h-11 items-center gap-3 px-4 py-4">
        <StatusDot status={!health ? 'warning' : isReady ? 'healthy' : isBusy ? 'healthy' : 'exception'} size="md" pulse={isBusy} />
        <div className="min-w-0 flex-1"><p data-testid="worker-health-status" className="text-sm font-medium">{!health ? t(loading ? 'Loading…' : 'Status unavailable') : t(isReady ? 'Ready for work' : isBusy ? 'Working on your tasks' : 'Connection needs attention')}</p>
          <p data-testid="worker-queue-depth" className="mt-1 text-xs text-muted-foreground">{health ? health.running_count + ' ' + t('running') + ' · ' + health.queued_count + ' ' + t('waiting') : ''}</p>
        </div>
        <button type="button" className="min-h-11 text-sm text-muted-foreground hover:text-foreground" onClick={() => setShowDetails(true)}>{t('Details')}</button>
      </div>
      {error && <p role="alert" className="px-4 pb-3 text-sm text-destructive">{t(error)}</p>}
    </GroupedSection>
    <SlideDrawer isOpen={showDetails} onClose={() => setShowDetails(false)} title={t('Application status')} size="md">
      <div className="space-y-5 text-sm"><p className="text-muted-foreground">cuan-runner · {health?.runner_name || health?.runner_id || t('Loading…')}</p><p data-testid="worker-heartbeat">{t('Last checked')}: {formattedHeartbeat()}</p>
        <button type="button" data-testid="worker-refresh-btn" onClick={() => void fetchWorkerHealth(true)} disabled={loading} className="action-secondary min-h-11 inline-flex items-center gap-2" aria-label={t('Periksa Status')}><RefreshCw className={loading ? 'h-4 w-4 animate-spin' : 'h-4 w-4'} />{t(loading ? 'Memeriksa...' : 'Periksa Status')}</button>
      </div>
    </SlideDrawer>
  </>
}
