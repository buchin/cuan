import { useState, useEffect } from 'react'
import {
  CalendarClock,
  Plus,
  Play,
  Trash2,
  Clock,
  CheckCircle2,
  AlertCircle,
  RotateCw,
  X,
  ChevronDown,
  Repeat,
} from 'lucide-react'
import { useLanguage } from '../../context/LanguageContext'
import { HairlineList } from '../../components/layout/HairlineList'
import { StatusDot } from '../../components/ui/StatusDot'
import { cn } from '../../lib/utils'

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
  const [expandedId, setExpandedId] = useState<string | null>(null)

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
        if (expandedId === id) setExpandedId(null)
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

  const formatScheduleTime = (task: ScheduledTask) => {
    if (task.schedule_type === 'daily') {
      return `${task.time_of_day || '08:00'} WIB`
    }
    if (task.schedule_type === 'weekly') {
      const days = [
        t('Sun'),
        t('Mon'),
        t('Tue'),
        t('Wed'),
        t('Thu'),
        t('Fri'),
        t('Sat'),
      ]
      return `${days[task.day_of_week ?? 1]} · ${task.time_of_day || '09:00'} WIB`
    }
    return `${task.interval_mins || 60} ${t('min')}`
  }

  const formatScheduleTypeLabel = (task: ScheduledTask) => {
    if (task.schedule_type === 'daily') return t('Daily')
    if (task.schedule_type === 'weekly') return t('Weekly')
    return t('Interval')
  }

  return (
    <div className={`space-y-5 ${embedded ? '' : 'p-4 sm:p-6 max-w-4xl mx-auto'}`}>
      {/* Header */}
      <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-4">
        <div>
          <div className="flex items-center gap-2.5">
            <CalendarClock className="h-5 w-5 text-primary" />
            <h1 className="text-xl sm:text-2xl font-bold tracking-tight text-foreground">
              {t('Scheduled Routines')}
            </h1>
          </div>
          <p className="mt-1 text-xs sm:text-sm text-muted-foreground">
            {t('Automated recurring routines that agents execute without requiring manual prompts.')}
          </p>
        </div>

        <button
          type="button"
          onClick={() => setIsModalOpen(true)}
          className="action-primary inline-flex items-center gap-2 shrink-0 self-start sm:self-center text-xs sm:text-sm min-h-[40px]"
        >
          <Plus className="h-4 w-4" />
          <span>{t('New Schedule')}</span>
        </button>
      </div>

      {actionFeedback && (
        <div className="text-sm text-emerald-600 dark:text-emerald-400 flex items-center gap-2 bg-emerald-500/10 p-3 rounded-xl border border-emerald-500/20">
          <CheckCircle2 className="h-4 w-4 shrink-0" />
          <span>{actionFeedback}</span>
        </div>
      )}

      {/* Repeating Routines (Things 3 Inspired: Time on left, Routine in center, Toggle on right) */}
      {loading && tasks.length === 0 ? (
        <div className="flex items-center justify-center py-12 text-sm text-muted-foreground">
          <RotateCw className="h-5 w-5 animate-spin mr-2" />
          <span>{t('Loading schedules...')}</span>
        </div>
      ) : tasks.length === 0 ? (
        <div className="rounded-xl border border-dashed border-border/60 p-8 sm:p-12 text-center bg-muted/10">
          <div className="inline-flex h-11 w-11 items-center justify-center rounded-full bg-primary/10 text-primary mb-3">
            <Repeat className="h-5 w-5" />
          </div>
          <h3 className="text-sm font-semibold text-foreground">{t('No scheduled routines yet')}</h3>
          <p className="mt-1 text-xs text-muted-foreground max-w-md mx-auto">
            {t('Create your first routine or ask Cuan in Chat: "Tolong tiap Senin jam 9 pagi buatkan rekap penjualan."')}
          </p>
          <button
            type="button"
            onClick={() => setIsModalOpen(true)}
            className="action-primary mt-4 inline-flex items-center gap-1.5 text-xs"
          >
            <Plus className="h-3.5 w-3.5" />
            <span>{t('Create Schedule')}</span>
          </button>
        </div>
      ) : (
        <div className="rounded-xl border border-border/40 bg-card/40 overflow-hidden transition-colors">
          <HairlineList>
            {tasks.map((task) => {
              const isExpanded = expandedId === task.id
              const isTriggering = triggeringId === task.id

              return (
                <div key={task.id} className="transition-colors">
                  {/* Routine Surface Row (Things 3 repeating format) */}
                  <div
                    role="button"
                    tabIndex={0}
                    onClick={() => setExpandedId(isExpanded ? null : task.id)}
                    onKeyDown={(e) => {
                      if (e.key === 'Enter' || e.key === ' ') {
                        e.preventDefault()
                        setExpandedId(isExpanded ? null : task.id)
                      }
                    }}
                    className={cn(
                      'group flex items-center justify-between gap-3 px-3.5 py-3.5 sm:px-4 min-h-[52px] cursor-pointer hover:bg-muted/30 transition-colors select-none outline-none focus-visible:bg-muted/40',
                      !task.enabled && 'opacity-65',
                      isExpanded && 'bg-muted/20',
                    )}
                  >
                    {/* Left: Time & Cadence (Things 3 repeating routine style) */}
                    <div className="w-28 sm:w-36 shrink-0 flex items-center gap-2 text-xs font-mono font-medium text-muted-foreground">
                      <Clock className="h-3.5 w-3.5 text-muted-foreground/70 shrink-0" />
                      <span className="truncate">{formatScheduleTime(task)}</span>
                    </div>

                    {/* Center: Routine Title & Status Indicator */}
                    <div className="min-w-0 flex-1 flex items-center gap-2.5">
                      <StatusDot
                        status={
                          isTriggering
                            ? 'running'
                            : task.enabled
                              ? 'healthy'
                              : 'neutral'
                        }
                        pulse={isTriggering}
                        title={task.enabled ? t('Active') : t('Paused')}
                      />
                      <div className="min-w-0 flex-1">
                        <span className="font-medium text-sm text-foreground truncate block group-hover:text-primary transition-colors">
                          {task.title}
                        </span>
                        <div className="flex items-center gap-2 text-[11px] text-muted-foreground mt-0.5">
                          <span>{formatScheduleTypeLabel(task)}</span>
                          {task.next_run_at && task.enabled && (
                            <>
                              <span>·</span>
                              <span className="truncate">
                                {t('Next run:')}{' '}
                                {new Date(task.next_run_at).toLocaleTimeString([], {
                                  hour: '2-digit',
                                  minute: '2-digit',
                                })}
                              </span>
                            </>
                          )}
                          {!task.enabled && (
                            <>
                              <span>·</span>
                              <span className="text-muted-foreground/70">
                                {t('Paused')}
                              </span>
                            </>
                          )}
                        </div>
                      </div>
                    </div>

                    {/* Right: Switch Toggle & Expand Chevron */}
                    <div className="flex shrink-0 items-center gap-2 sm:gap-3 pl-2">
                      {/* Apple/Things 3 style Switch Toggle */}
                      <button
                        type="button"
                        role="switch"
                        aria-checked={task.enabled}
                        title={task.enabled ? t('Pause schedule') : t('Resume schedule')}
                        onClick={(e) => {
                          e.stopPropagation()
                          void handleToggle(task.id, task.enabled)
                        }}
                        className={cn(
                          'relative inline-flex h-6 w-11 shrink-0 cursor-pointer rounded-full border-2 border-transparent transition-colors duration-200 ease-in-out focus:outline-none focus-visible:ring-1 focus-visible:ring-ring',
                          task.enabled
                            ? 'bg-emerald-500/90 dark:bg-emerald-400'
                            : 'bg-muted-foreground/30',
                        )}
                      >
                        <span
                          className={cn(
                            'pointer-events-none inline-block h-5 w-5 transform rounded-full bg-white shadow-sm ring-0 transition duration-200 ease-in-out',
                            task.enabled ? 'translate-x-5' : 'translate-x-0',
                          )}
                        />
                      </button>

                      <ChevronDown
                        className={cn(
                          'h-4 w-4 text-muted-foreground/70 transition-transform duration-200',
                          isExpanded && 'rotate-180 text-foreground',
                        )}
                      />
                    </div>
                  </div>

                  {/* Deep Layer: Progressive Disclosure Expandable Section */}
                  {isExpanded && (
                    <div className="border-t border-border/30 bg-muted/15 px-4 py-4 sm:px-6 space-y-3.5 animate-in fade-in-50 duration-150">
                      <div className="space-y-1.5">
                        <span className="text-xs font-semibold uppercase tracking-wider text-muted-foreground">
                          {t('Agent Instruction (Prompt)')}
                        </span>
                        <div className="rounded-lg border border-border/40 bg-card p-3 font-mono text-xs text-muted-foreground leading-relaxed whitespace-pre-wrap break-words">
                          {task.prompt}
                        </div>
                      </div>

                      {task.last_error && (
                        <div className="flex items-center gap-2 text-xs text-rose-500 bg-rose-500/10 border border-rose-500/20 rounded-lg p-2.5">
                          <AlertCircle className="h-4 w-4 shrink-0" />
                          <span className="truncate">{task.last_error}</span>
                        </div>
                      )}

                      {/* Detail actions & trigger bar */}
                      <div className="flex items-center justify-between pt-1 text-xs">
                        <div className="text-muted-foreground">
                          {task.last_run_at ? (
                            <span>
                              {t('Last run:')}{' '}
                              {new Date(task.last_run_at).toLocaleString([], {
                                dateStyle: 'short',
                                timeStyle: 'short',
                              })}
                            </span>
                          ) : (
                            <span>{t('Never executed yet')}</span>
                          )}
                        </div>

                        <div className="flex items-center gap-2">
                          <button
                            type="button"
                            onClick={(e) => {
                              e.stopPropagation()
                              void handleDelete(task.id)
                            }}
                            title={t('Delete schedule')}
                            className="action-secondary text-xs p-1.5 text-muted-foreground hover:text-destructive min-h-[32px] inline-flex items-center gap-1.5"
                          >
                            <Trash2 className="h-3.5 w-3.5" />
                            <span className="hidden sm:inline">{t('Delete')}</span>
                          </button>

                          <button
                            type="button"
                            onClick={(e) => {
                              e.stopPropagation()
                              void handleTrigger(task.id)
                            }}
                            disabled={isTriggering || !task.enabled}
                            className="action-secondary text-xs inline-flex items-center gap-1.5 py-1 px-3 min-h-[32px]"
                          >
                            <Play className="h-3 w-3" />
                            <span>{isTriggering ? t('Running...') : t('Run now')}</span>
                          </button>
                        </div>
                      </div>
                    </div>
                  )}
                </div>
              )
            })}
          </HairlineList>
        </div>
      )}

      {/* Create Modal Dialog */}
      {isModalOpen && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/60 p-4 backdrop-blur-sm animate-fade-in">
          <div className="w-full max-w-lg rounded-xl border border-border bg-card p-6 shadow-xl space-y-4">
            <div className="flex items-center justify-between border-b border-border/50 pb-3">
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

              <div className="flex items-center justify-end gap-2 pt-3 border-t border-border/50">
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
