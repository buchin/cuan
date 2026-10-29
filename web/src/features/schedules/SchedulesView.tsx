import { useState, useEffect } from 'react'
import {
  CalendarClock,
  Plus,
  Play,
  Trash2,
  Clock,
  CheckCircle2,
  AlertCircle,
  Pause,
  RotateCw,
  Sparkles,
  X,
} from 'lucide-react'
import { useLanguage } from '../../context/LanguageContext'

export interface ScheduledTask {
  id: string
  title: string
  prompt: string
  project_id?: string
  schedule_type: 'daily' | 'weekly' | 'interval'
  time_of_day?: string
  day_of_week?: number
  interval_mins?: number
  enabled: boolean
  created_at: string
  next_run_at?: string
  last_run_at?: string
  last_status: 'idle' | 'running' | 'success' | 'failed'
  last_error?: string
}

interface SchedulesViewProps {
  embedded?: boolean
}

export function SchedulesView({ embedded = false }: SchedulesViewProps) {
  const { t } = useLanguage()
  const [tasks, setTasks] = useState<ScheduledTask[]>([])
  const [loading, setLoading] = useState(true)
  const [isModalOpen, setIsModalOpen] = useState(false)
  const [triggeringId, setTriggeringId] = useState<string | null>(null)
  const [actionFeedback, setActionFeedback] = useState<string | null>(null)

  // Form states
  const [title, setTitle] = useState('')
  const [prompt, setPrompt] = useState('')
  const [scheduleType, setScheduleType] = useState<'daily' | 'weekly' | 'interval'>('daily')
  const [timeOfDay, setTimeOfDay] = useState('09:00')
  const [dayOfWeek, setDayOfWeek] = useState(1) // Monday
  const [intervalMins, setIntervalMins] = useState(60)
  const [submitting, setSubmitting] = useState(false)

  const fetchTasks = async () => {
    try {
      setLoading(true)
      const res = await fetch('/api/system/scheduler/tasks')
      if (res.ok) {
        const data = await res.json()
        setTasks(data.tasks || [])
      }
    } catch {
      // ignore
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    fetchTasks()
  }, [])

  const handleCreate = async (e: React.FormEvent) => {
    e.preventDefault()
    if (!title.trim() || !prompt.trim()) return

    try {
      setSubmitting(true)
      const res = await fetch('/api/system/scheduler/tasks', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          title,
          prompt,
          schedule_type: scheduleType,
          time_of_day: timeOfDay,
          day_of_week: dayOfWeek,
          interval_mins: intervalMins,
        }),
      })

      if (res.ok) {
        setIsModalOpen(false)
        setTitle('')
        setPrompt('')
        setActionFeedback(t('Schedule created successfully'))
        setTimeout(() => setActionFeedback(null), 3000)
        fetchTasks()
      }
    } catch {
      // ignore
    } finally {
      setSubmitting(false)
    }
  }

  const handleToggle = async (id: string, currentEnabled: boolean) => {
    try {
      const res = await fetch(`/api/system/scheduler/tasks/${encodeURIComponent(id)}/toggle`, {
        method: 'PATCH',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ enabled: !currentEnabled }),
      })
      if (res.ok) {
        fetchTasks()
      }
    } catch {
      // ignore
    }
  }

  const handleDelete = async (id: string) => {
    if (!window.confirm(t('Are you sure you want to delete this schedule?'))) return
    try {
      const res = await fetch(`/api/system/scheduler/tasks/${encodeURIComponent(id)}`, {
        method: 'DELETE',
      })
      if (res.ok) {
        fetchTasks()
      }
    } catch {
      // ignore
    }
  }

  const handleTrigger = async (id: string) => {
    try {
      setTriggeringId(id)
      const res = await fetch(`/api/system/scheduler/tasks/${encodeURIComponent(id)}/trigger`, {
        method: 'POST',
      })
      if (res.ok) {
        setActionFeedback(t('Task triggered in background'))
        setTimeout(() => setActionFeedback(null), 3000)
        setTimeout(fetchTasks, 1500)
      }
    } catch {
      // ignore
    } finally {
      setTriggeringId(null)
    }
  }

  const formatScheduleBadge = (task: ScheduledTask) => {
    if (task.schedule_type === 'daily') {
      return `${t('Daily at')} ${task.time_of_day || '08:00'} WIB`
    }
    if (task.schedule_type === 'weekly') {
      const days = [
        t('Sunday'),
        t('Monday'),
        t('Tuesday'),
        t('Wednesday'),
        t('Thursday'),
        t('Friday'),
        t('Saturday'),
      ]
      return `${t('Every')} ${days[task.day_of_week ?? 1]} · ${task.time_of_day || '09:00'} WIB`
    }
    return `${t('Every')} ${task.interval_mins || 60} ${t('minutes')}`
  }

  return (
    <div className={`space-y-5 ${embedded ? '' : 'p-4 sm:p-6 max-w-5xl mx-auto'}`}>
      {/* Header */}
      <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-4">
        <div>
          <div className="flex items-center gap-2.5">
            <CalendarClock className="h-6 w-6 text-primary" />
            <h1 className="text-xl sm:text-2xl font-bold tracking-tight text-foreground">
              {t('Scheduled Routines')}
            </h1>
          </div>
          <p className="mt-1 text-sm text-muted-foreground">
            {t('Automated recurring routines that agents execute without requiring manual prompts.')}
          </p>
        </div>

        <button
          type="button"
          onClick={() => setIsModalOpen(true)}
          className="action-primary inline-flex items-center gap-2 shrink-0 self-start sm:self-center"
        >
          <Plus className="h-4 w-4" />
          <span>{t('New Schedule')}</span>
        </button>
      </div>

      {actionFeedback && (
        <div className="text-sm text-emerald-600 dark:text-emerald-400 flex items-center gap-2 bg-emerald-500/10 p-3 rounded-lg border border-emerald-500/20">
          <CheckCircle2 className="h-4 w-4 shrink-0" />
          <span>{actionFeedback}</span>
        </div>
      )}

      {/* Content List */}
      {loading && tasks.length === 0 ? (
        <div className="flex items-center justify-center py-12 text-sm text-muted-foreground">
          <RotateCw className="h-5 w-5 animate-spin mr-2" />
          <span>{t('Loading schedules...')}</span>
        </div>
      ) : tasks.length === 0 ? (
        <div className="rounded-xl border border-dashed border-border p-8 sm:p-12 text-center bg-card/40">
          <div className="inline-flex h-12 w-12 items-center justify-center rounded-full bg-primary/10 text-primary mb-3">
            <Sparkles className="h-6 w-6" />
          </div>
          <h3 className="text-base font-semibold text-foreground">{t('No scheduled routines yet')}</h3>
          <p className="mt-1.5 text-sm text-muted-foreground max-w-md mx-auto">
            {t('Create your first routine or ask Cuan in Chat: "Tolong tiap Senin jam 9 pagi buatkan rekap penjualan."')}
          </p>
          <button
            type="button"
            onClick={() => setIsModalOpen(true)}
            className="action-primary mt-4 inline-flex items-center gap-1.5"
          >
            <Plus className="h-4 w-4" />
            <span>{t('Create Schedule')}</span>
          </button>
        </div>
      ) : (
        <div className="grid gap-4 sm:grid-cols-2">
          {tasks.map((task) => (
            <div
              key={task.id}
              className={`rounded-xl border bg-card p-5 space-y-3 transition-all ${
                task.enabled ? 'border-border shadow-sm' : 'border-border/60 opacity-75 bg-muted/20'
              }`}
            >
              <div className="flex items-start justify-between gap-3">
                <div className="space-y-1">
                  <div className="flex items-center gap-2 flex-wrap">
                    <h3 className="font-semibold text-base text-foreground leading-snug">
                      {task.title}
                    </h3>
                    <span
                      className={`inline-flex items-center px-2 py-0.5 rounded text-[11px] font-medium ${
                        task.enabled
                          ? 'bg-emerald-500/10 text-emerald-600 dark:text-emerald-400 border border-emerald-500/20'
                          : 'bg-muted text-muted-foreground border border-border'
                      }`}
                    >
                      {task.enabled ? t('Active') : t('Paused')}
                    </span>
                  </div>
                  <div className="flex items-center gap-1.5 text-xs text-primary font-medium">
                    <Clock className="h-3.5 w-3.5 shrink-0" />
                    <span>{formatScheduleBadge(task)}</span>
                  </div>
                </div>

                <div className="flex items-center gap-1">
                  <button
                    type="button"
                    onClick={() => handleToggle(task.id, task.enabled)}
                    title={task.enabled ? t('Pause schedule') : t('Resume schedule')}
                    className="action-secondary p-1.5 h-8 w-8 inline-flex items-center justify-center rounded-md text-muted-foreground hover:text-foreground"
                  >
                    {task.enabled ? <Pause className="h-4 w-4" /> : <Play className="h-4 w-4" />}
                  </button>
                  <button
                    type="button"
                    onClick={() => handleDelete(task.id)}
                    title={t('Delete schedule')}
                    className="action-secondary p-1.5 h-8 w-8 inline-flex items-center justify-center rounded-md text-muted-foreground hover:text-destructive"
                  >
                    <Trash2 className="h-4 w-4" />
                  </button>
                </div>
              </div>

              {/* Prompt Instruction */}
              <div className="bg-muted/40 rounded-lg p-3 text-xs text-muted-foreground font-mono leading-relaxed line-clamp-3">
                {task.prompt}
              </div>

              {/* Footer info & trigger */}
              <div className="flex items-center justify-between pt-2 border-t border-border text-xs text-muted-foreground">
                <div className="truncate">
                  {task.next_run_at && task.enabled ? (
                    <span>
                      {t('Next run:')}{' '}
                      {new Date(task.next_run_at).toLocaleTimeString([], {
                        hour: '2-digit',
                        minute: '2-digit',
                      })}
                    </span>
                  ) : (
                    <span>{t('Schedule paused')}</span>
                  )}
                </div>

                <button
                  type="button"
                  onClick={() => handleTrigger(task.id)}
                  disabled={triggeringId === task.id || !task.enabled}
                  className="action-secondary text-xs inline-flex items-center gap-1.5 py-1 px-2.5 h-7"
                >
                  <Play className="h-3 w-3" />
                  <span>{triggeringId === task.id ? t('Running...') : t('Run now')}</span>
                </button>
              </div>

              {task.last_error && (
                <div className="flex items-center gap-1.5 text-xs text-rose-500 pt-1">
                  <AlertCircle className="h-3.5 w-3.5 shrink-0" />
                  <span className="truncate">{task.last_error}</span>
                </div>
              )}
            </div>
          ))}
        </div>
      )}

      {/* Create Modal Dialog */}
      {isModalOpen && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/60 p-4 backdrop-blur-sm animate-fade-in">
          <div className="w-full max-w-lg rounded-xl border border-border bg-card p-6 shadow-xl space-y-4">
            <div className="flex items-center justify-between border-b border-border pb-3">
              <h2 className="text-lg font-bold text-foreground">{t('New Scheduled Routine')}</h2>
              <button
                type="button"
                onClick={() => setIsModalOpen(false)}
                className="text-muted-foreground hover:text-foreground"
              >
                <X className="h-5 w-5" />
              </button>
            </div>

            <form onSubmit={handleCreate} className="space-y-4">
              <div className="space-y-1.5">
                <label className="text-xs font-semibold uppercase tracking-wider text-muted-foreground">
                  {t('Routine Title')}
                </label>
                <input
                  type="text"
                  required
                  placeholder={t('e.g. Follow-up Pelanggan WhatsApp')}
                  value={title}
                  onChange={(e) => setTitle(e.target.value)}
                  className="field-control w-full rounded-md border-border text-sm"
                />
              </div>

              <div className="space-y-1.5">
                <label className="text-xs font-semibold uppercase tracking-wider text-muted-foreground">
                  {t('Agent Instruction (Prompt)')}
                </label>
                <textarea
                  required
                  rows={3}
                  placeholder={t('e.g. Cek orderan yang belum dibayar hari ini dan kirimkan reminder sopan via tebar.app')}
                  value={prompt}
                  onChange={(e) => setPrompt(e.target.value)}
                  className="field-control w-full rounded-md border-border text-sm resize-none"
                />
              </div>

              <div className="grid grid-cols-2 gap-3">
                <div className="space-y-1.5">
                  <label className="text-xs font-semibold uppercase tracking-wider text-muted-foreground">
                    {t('Frequency')}
                  </label>
                  <select
                    value={scheduleType}
                    onChange={(e) => setScheduleType(e.target.value as any)}
                    className="field-control w-full rounded-md border-border text-sm"
                  >
                    <option value="daily">{t('Daily')}</option>
                    <option value="weekly">{t('Weekly')}</option>
                    <option value="interval">{t('Interval (Minutes)')}</option>
                  </select>
                </div>

                {scheduleType === 'interval' ? (
                  <div className="space-y-1.5">
                    <label className="text-xs font-semibold uppercase tracking-wider text-muted-foreground">
                      {t('Every (Minutes)')}
                    </label>
                    <input
                      type="number"
                      min={15}
                      step={15}
                      value={intervalMins}
                      onChange={(e) => setIntervalMins(Number(e.target.value))}
                      className="field-control w-full rounded-md border-border text-sm"
                    />
                  </div>
                ) : (
                  <div className="space-y-1.5">
                    <label className="text-xs font-semibold uppercase tracking-wider text-muted-foreground">
                      {t('Time (WIB)')}
                    </label>
                    <select
                      value={timeOfDay}
                      onChange={(e) => setTimeOfDay(e.target.value)}
                      className="field-control w-full rounded-md border-border text-sm"
                    >
                      <option value="08:00">08:00 WIB (Pagi)</option>
                      <option value="09:00">09:00 WIB</option>
                      <option value="10:00">10:00 WIB</option>
                      <option value="12:00">12:00 WIB (Siang)</option>
                      <option value="14:00">14:00 WIB</option>
                      <option value="17:00">17:00 WIB (Sore)</option>
                      <option value="20:00">20:00 WIB (Malam)</option>
                    </select>
                  </div>
                )}
              </div>

              {scheduleType === 'weekly' && (
                <div className="space-y-1.5">
                  <label className="text-xs font-semibold uppercase tracking-wider text-muted-foreground">
                    {t('Day of Week')}
                  </label>
                  <select
                    value={dayOfWeek}
                    onChange={(e) => setDayOfWeek(Number(e.target.value))}
                    className="field-control w-full rounded-md border-border text-sm"
                  >
                    <option value={1}>{t('Monday')}</option>
                    <option value={2}>{t('Tuesday')}</option>
                    <option value={3}>{t('Wednesday')}</option>
                    <option value={4}>{t('Thursday')}</option>
                    <option value={5}>{t('Friday')}</option>
                    <option value={6}>{t('Saturday')}</option>
                    <option value={0}>{t('Sunday')}</option>
                  </select>
                </div>
              )}

              <div className="flex items-center justify-end gap-2 pt-3 border-t border-border">
                <button
                  type="button"
                  onClick={() => setIsModalOpen(false)}
                  className="action-secondary"
                >
                  {t('Cancel')}
                </button>
                <button
                  type="submit"
                  disabled={submitting}
                  className="action-primary"
                >
                  {submitting ? t('Saving...') : t('Save Schedule')}
                </button>
              </div>
            </form>
          </div>
        </div>
      )}
    </div>
  )
}
