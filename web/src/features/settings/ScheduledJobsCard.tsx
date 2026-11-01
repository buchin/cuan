import { useState, useEffect } from 'react'
import { Play, CheckCircle2, AlertCircle, Clock, CalendarClock } from 'lucide-react'
import { useLanguage } from '../../context/LanguageContext'
import { GroupedSection } from '../../components/layout/GroupedSection'

interface ScheduledJob {
  id: string
  name: string
  description?: string
  schedule_type: string
  time_of_day?: string
  day_of_week?: number
  enabled: boolean
  last_status: string
  last_run_at?: string
  next_run_at?: string
  last_error?: string
}

export function ScheduledJobsCard() {
  const { t } = useLanguage()
  const [jobs, setJobs] = useState<ScheduledJob[]>([])
  const [loading, setLoading] = useState(false)
  const [triggering, setTriggering] = useState<string | null>(null)
  const [feedback, setFeedback] = useState<string | null>(null)

  const fetchJobs = async () => {
    try {
      setLoading(true)
      const res = await fetch('/api/system/scheduler/jobs')
      if (res.ok) {
        const data = await res.json()
        setJobs(data.jobs || [])
      }
    } catch {
      // fallback
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    fetchJobs()
  }, [])

  const handleTrigger = async (id: string) => {
    try {
      setTriggering(id)
      const res = await fetch(`/api/system/scheduler/jobs/${encodeURIComponent(id)}/trigger`, {
        method: 'POST',
      })
      if (res.ok) {
        setFeedback(t('Job triggered in background'))
        setTimeout(() => setFeedback(null), 3000)
        setTimeout(fetchJobs, 1500)
      }
    } catch {
      // ignore
    } finally {
      setTriggering(null)
    }
  }

  return (
    <GroupedSection
      id="scheduled-jobs"
      aria-labelledby="scheduled-jobs-title"
      title={
        <span className="inline-flex items-center gap-2">
          <CalendarClock className="h-4 w-4 text-muted-foreground" />
          <span>{t('Scheduled Routines & Automations')}</span>
        </span>
      }
      description={t('Automated system routines running quietly in the background.')}
    >
      <div className="p-4 sm:p-5 space-y-3">
        {feedback && (
          <div className="text-xs text-emerald-500 flex items-center gap-1.5 bg-emerald-500/10 p-2.5 rounded-lg border border-emerald-500/20">
            <CheckCircle2 className="h-3.5 w-3.5" />
            <span>{feedback}</span>
          </div>
        )}

        {loading && jobs.length === 0 ? (
          <div className="text-sm text-muted-foreground py-2">{t('Loading schedules...')}</div>
        ) : jobs.length === 0 ? (
          <div className="text-sm text-muted-foreground py-2">{t('No active background routines.')}</div>
        ) : (
          <div className="divide-y divide-border/40">
          {jobs.map((job) => (
            <div key={job.id} className="py-3 first:pt-0 last:pb-0 flex flex-col sm:flex-row sm:items-center justify-between gap-3">
              <div className="space-y-1">
                <div className="flex items-center gap-2">
                  <span className="font-medium text-sm text-foreground">{job.name}</span>
                  <span className={`inline-flex items-center px-2 py-0.5 rounded text-[11px] font-medium ${
                    job.last_status === 'success'
                      ? 'bg-emerald-500/10 text-emerald-600 dark:text-emerald-400'
                      : job.last_status === 'failed'
                      ? 'bg-rose-500/10 text-rose-600 dark:text-rose-400'
                      : 'bg-muted text-muted-foreground'
                  }`}>
                    {job.last_status === 'success' ? t('Healthy') : job.last_status === 'failed' ? t('Failed') : t('Idle')}
                  </span>
                </div>
                {job.description && (
                  <p className="text-xs text-muted-foreground">{job.description}</p>
                )}
                <div className="flex items-center gap-3 text-xs text-muted-foreground pt-0.5">
                  <span className="flex items-center gap-1">
                    <Clock className="h-3 w-3" />
                    {job.schedule_type === 'daily'
                      ? `${t('Daily at')} ${job.time_of_day || '03:00'} WIB`
                      : t('Periodic schedule')}
                  </span>
                  {job.last_run_at && (
                    <span>
                      {t('Last run:')} {new Date(job.last_run_at).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })}
                    </span>
                  )}
                </div>
                {job.last_error && (
                  <p className="text-xs text-rose-500 flex items-center gap-1">
                    <AlertCircle className="h-3 w-3" />
                    {job.last_error}
                  </p>
                )}
              </div>

              <button
                type="button"
                onClick={() => handleTrigger(job.id)}
                disabled={triggering === job.id}
                className="action-secondary text-xs inline-flex items-center gap-1.5 py-1 px-3 self-start sm:self-center"
              >
                <Play className="h-3 w-3" />
                {triggering === job.id ? t('Running...') : t('Run now')}
              </button>
            </div>
          ))}
        </div>
      )}
      </div>
    </GroupedSection>
  )
}
