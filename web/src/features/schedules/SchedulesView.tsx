import { useState, useEffect, useCallback } from 'react'
import { useSearchParams } from 'react-router-dom'
import {
  CalendarClock,
  Plus,
  Play,
  Trash2,
  Pencil,
  Clock,
  CheckCircle2,
  AlertCircle,
  RotateCw,
  ChevronDown,
  Repeat,
  FolderGit2,
} from 'lucide-react'
import { useLanguage } from '../../context/LanguageContext'
import { HairlineList } from '../../components/layout/HairlineList'
import { StatusDot } from '../../components/ui/StatusDot'
import { SlideDrawer } from '../../components/layout/SlideDrawer'
import { projectsApi } from '../../api/projects'
import { FormField, Input, Select, FormCard } from '../../components/ui/forms'
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
  timezone?: string
  enabled: boolean
  created_at: string
  next_run_at?: string
  last_run_at?: string
  last_status: 'idle' | 'running' | 'success' | 'failed'
  last_error?: string
}

interface ProjectOption {
  id: string
  name: string
}

interface SchedulesViewProps {
  embedded?: boolean
}

export const fieldControlClass =
  'h-11 min-h-11 w-full rounded-xl border border-border/60 bg-background/80 px-3.5 text-sm text-foreground focus:outline-none focus:ring-1 focus:ring-ring transition-colors'

export function SchedulesView({ embedded = false }: SchedulesViewProps) {
  const { t } = useLanguage()
  const [searchParams, setSearchParams] = useSearchParams()
  const [tasks, setTasks] = useState<ScheduledTask[]>([])
  const [projects, setProjects] = useState<ProjectOption[]>([])
  const [loading, setLoading] = useState(true)
  const [editingTask, setEditingTask] = useState<ScheduledTask | null>(null)
  const [triggeringId, setTriggeringId] = useState<string | null>(null)
  const [actionFeedback, setActionFeedback] = useState<string | null>(null)
  const [errorFeedback, setErrorFeedback] = useState<string | null>(null)
  const [expandedId, setExpandedId] = useState<string | null>(null)

  // URL deep-linking: ?action=new opens creation drawer, ?edit=<id> opens edit drawer
  const editParam = searchParams.get('edit') || ''
  const isCreating = searchParams.get('action') === 'new'
  const isDrawerOpen = Boolean(editParam || isCreating)

  // Form states
  const [title, setTitle] = useState('')
  const [prompt, setPrompt] = useState('')
  const [projectId, setProjectId] = useState('')
  const [scheduleType, setScheduleType] = useState<'daily' | 'weekly' | 'interval'>('daily')
  const [timeOfDay, setTimeOfDay] = useState('09:00')
  const [dayOfWeek, setDayOfWeek] = useState(1) // Monday
  const [intervalMins, setIntervalMins] = useState(60)
  const [timezone, setTimezone] = useState(() => {
    try {
      return Intl.DateTimeFormat().resolvedOptions().timeZone || 'Asia/Jakarta'
    } catch {
      return 'Asia/Jakarta'
    }
  })
  const [submitting, setSubmitting] = useState(false)

  const resetForm = useCallback(() => {
    setEditingTask(null)
    setTitle('')
    setPrompt('')
    setProjectId('')
    setScheduleType('daily')
    setTimeOfDay('09:00')
    setDayOfWeek(1)
    setIntervalMins(60)
    setTimezone(Intl.DateTimeFormat().resolvedOptions().timeZone || 'Asia/Jakarta')
  }, [])

  const populateFormWithTask = useCallback((task: ScheduledTask) => {
    setEditingTask(task)
    setTitle(task.title || '')
    setPrompt(task.prompt || '')
    setProjectId(task.project_id || '')
    setScheduleType(task.schedule_type || 'daily')
    setTimeOfDay(task.time_of_day || '09:00')
    setDayOfWeek(task.day_of_week ?? 1)
    setIntervalMins(task.interval_mins || 60)
    setTimezone(task.timezone || Intl.DateTimeFormat().resolvedOptions().timeZone || 'Asia/Jakarta')
  }, [])

  const fetchTasks = async () => {
    try {
      setLoading(true)
      const res = await fetch('/api/system/scheduler/tasks')
      if (!res.ok) {
        setErrorFeedback(t('Failed to load scheduled routines.'))
        return
      }
      const data = await res.json()
      setTasks(data.tasks || [])
      setErrorFeedback(null)
    } catch {
      setErrorFeedback(t('Failed to load scheduled routines.'))
    } finally {
      setLoading(false)
    }
  }

  const fetchProjects = async () => {
    try {
      const list = await projectsApi.list()
      setProjects(list.map((p) => ({ id: p.id, name: p.name })))
    } catch {
      // ignore
    }
  }

  useEffect(() => {
    fetchTasks()
    fetchProjects()
  }, [])

  // Deep linking sync: react to URL query changes or direct page refresh with ?edit=<id> / ?action=new
  useEffect(() => {
    if (editParam) {
      const existing = tasks.find((item) => item.id === editParam)
      if (existing) {
        populateFormWithTask(existing)
      } else {
        // Direct fetch when page reloads or user bookmarks direct edit URL
        fetch(`/api/system/scheduler/tasks/${encodeURIComponent(editParam)}`)
          .then((res) => (res.ok ? res.json() : null))
          .then((task) => {
            if (task && task.id) {
              populateFormWithTask(task)
            }
          })
          .catch(() => {
            // ignore
          })
      }
    } else if (isCreating) {
      resetForm()
    } else {
      setEditingTask(null)
    }
  }, [editParam, isCreating, tasks, populateFormWithTask, resetForm])

  const handleOpenCreate = () => {
    resetForm()
    setSearchParams((prev) => {
      const next = new URLSearchParams(prev)
      next.set('action', 'new')
      next.delete('edit')
      return next
    })
  }

  const handleOpenEdit = (task: ScheduledTask) => {
    populateFormWithTask(task)
    setSearchParams((prev) => {
      const next = new URLSearchParams(prev)
      next.set('edit', task.id)
      next.delete('action')
      return next
    })
  }

  const handleCloseDrawer = () => {
    setSearchParams((prev) => {
      const next = new URLSearchParams(prev)
      next.delete('action')
      next.delete('edit')
      return next
    })
    setEditingTask(null)
  }

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    if (!title.trim() || !prompt.trim()) return

    try {
      setSubmitting(true)
      const userTz = timezone || Intl.DateTimeFormat().resolvedOptions().timeZone || 'Asia/Jakarta'
      const payload = {
        title: title.trim(),
        prompt: prompt.trim(),
        project_id: projectId.trim() || undefined,
        schedule_type: scheduleType,
        time_of_day: timeOfDay,
        day_of_week: dayOfWeek,
        interval_mins: intervalMins,
        timezone: userTz,
      }

      const url = editingTask
        ? `/api/system/scheduler/tasks/${encodeURIComponent(editingTask.id)}`
        : '/api/system/scheduler/tasks'
      const method = editingTask ? 'PUT' : 'POST'

      const res = await fetch(url, {
        method,
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(payload),
      })

      if (res.ok) {
        handleCloseDrawer()
        setActionFeedback(
          editingTask
            ? t('Schedule updated successfully')
            : t('Schedule created successfully'),
        )
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
      setErrorFeedback(null)
      const res = await fetch(`/api/system/scheduler/tasks/${encodeURIComponent(id)}/toggle`, {
        method: 'PATCH',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ enabled: !currentEnabled }),
      })
      if (!res.ok) {
        setErrorFeedback(t('Failed to update schedule status.'))
        return
      }
      fetchTasks()
    } catch {
      setErrorFeedback(t('Failed to update schedule status.'))
    }
  }

  const handleDelete = async (id: string) => {
    if (!window.confirm(t('Are you sure you want to delete this schedule?'))) return
    try {
      setErrorFeedback(null)
      const res = await fetch(`/api/system/scheduler/tasks/${encodeURIComponent(id)}`, {
        method: 'DELETE',
      })
      if (!res.ok) {
        setErrorFeedback(t('Failed to delete schedule.'))
        return
      }
      if (expandedId === id) setExpandedId(null)
      fetchTasks()
    } catch {
      setErrorFeedback(t('Failed to delete schedule.'))
    }
  }

  const handleTrigger = async (id: string) => {
    try {
      setTriggeringId(id)
      setErrorFeedback(null)
      const res = await fetch(`/api/system/scheduler/tasks/${encodeURIComponent(id)}/trigger`, {
        method: 'POST',
      })
      if (!res.ok) {
        setErrorFeedback(t('Failed to trigger schedule.'))
        return
      }
      setActionFeedback(t('Task triggered in background'))
      setTimeout(() => setActionFeedback(null), 3000)
      setTimeout(fetchTasks, 1500)
    } catch {
      setErrorFeedback(t('Failed to trigger schedule.'))
    } finally {
      setTriggeringId(null)
    }
  }

  const formatScheduleTime = (task: ScheduledTask) => {
    if (task.schedule_type === 'daily') {
      return `${task.time_of_day || '08:00'}`
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
      return `${days[task.day_of_week ?? 1]} · ${task.time_of_day || '09:00'}`
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
      <div
        className={cn(
          'flex items-center justify-between gap-3',
          embedded ? 'pb-1' : 'flex-col sm:flex-row sm:items-center',
        )}
      >
        {!embedded ? (
          <div>
            <div className="flex items-center gap-2.5">
              <CalendarClock className="h-5 w-5 text-primary" />
              <h1 className="text-xl sm:text-2xl font-bold tracking-tight text-foreground">
                {t('Scheduled Routines')}
              </h1>
            </div>
            <p className="mt-1 text-xs sm:text-sm text-muted-foreground">
              {t(
                'Automated recurring routines that agents execute without requiring manual prompts.',
              )}
            </p>
          </div>
        ) : (
          <div className="text-xs font-medium text-muted-foreground">
            {tasks.length > 0 && (
              <span>{tasks.length} {t('routine(s) configured')}</span>
            )}
          </div>
        )}

        {tasks.length > 0 && (
          <button
            type="button"
            onClick={handleOpenCreate}
            className="action-secondary inline-flex items-center gap-1.5 text-xs font-medium px-3 min-h-9 rounded-lg shrink-0 hover:text-foreground hover:bg-muted/60 transition-colors"
          >
            <Plus className="h-3.5 w-3.5" />
            <span>{t('New Schedule')}</span>
          </button>
        )}
      </div>

      {actionFeedback && (
        <div className="text-sm text-emerald-600 dark:text-emerald-400 flex items-center gap-2 bg-emerald-500/10 p-3 rounded-xl border border-emerald-500/20">
          <CheckCircle2 className="h-4 w-4 shrink-0" />
          <span>{actionFeedback}</span>
        </div>
      )}

      {errorFeedback && (
        <div role="alert" className="text-sm text-destructive flex items-center justify-between gap-2 bg-destructive/10 p-3 rounded-xl border border-destructive/20">
          <span>{errorFeedback}</span>
          <button
            type="button"
            onClick={() => setErrorFeedback(null)}
            className="text-xs underline underline-offset-2 shrink-0"
          >
            {t('Dismiss')}
          </button>
        </div>
      )}

      {/* Repeating Routines (Things 3 Inspired: Time on left, Routine in center, Toggle on right) */}
      {loading && tasks.length === 0 ? (
        <div className="flex items-center justify-center py-12 text-sm text-muted-foreground">
          <RotateCw className="h-5 w-5 animate-spin mr-2" />
          <span>{t('Loading schedules...')}</span>
        </div>
      ) : tasks.length === 0 ? (
        <div className="py-10 sm:py-14 px-4 text-center">
          <div className="inline-flex h-11 w-11 items-center justify-center rounded-full bg-primary/10 text-primary mb-3">
            <Repeat className="h-5 w-5" />
          </div>
          <h3 className="text-sm font-semibold text-foreground">{t('No scheduled routines yet')}</h3>
          <p className="mt-1 text-xs text-muted-foreground max-w-md mx-auto">
            {t('Create your first routine or ask Cuan in Chat: "Tolong tiap Senin jam 9 pagi buatkan rekap penjualan."')}
          </p>
          <button
            type="button"
            onClick={handleOpenCreate}
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
              const projectName = task.project_id
                ? projects.find((p) => p.id === task.project_id)?.name || null
                : null

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
                      'group flex items-center justify-between gap-3 px-3.5 py-3 sm:px-4 min-h-[52px] cursor-pointer hover:bg-muted/30 transition-colors select-none outline-none focus-visible:bg-muted/40',
                      !task.enabled && 'opacity-65',
                      isExpanded && 'bg-muted/20',
                    )}
                  >
                    {/* Left: Time & Cadence (Desktop / Tablet view: Things 3 column) */}
                    <div className="hidden sm:flex w-28 shrink-0 items-center gap-1.5 text-xs font-mono font-medium text-muted-foreground">
                      <Clock className="h-3.5 w-3.5 text-muted-foreground/70 shrink-0" />
                      <span className="truncate">{formatScheduleTime(task)}</span>
                    </div>

                    {/* Center: Routine Title & Status Indicator */}
                    <div className="min-w-0 flex-1 flex items-start gap-2.5">
                      <div className="pt-0.5 shrink-0">
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
                      </div>
                      <div className="min-w-0 flex-1">
                        <div className="min-w-0">
                          <span className="font-medium text-sm text-foreground group-hover:text-primary transition-colors leading-snug line-clamp-2 block">
                            {task.title}
                          </span>
                        </div>
                        <div className="flex items-center gap-1.5 text-[11px] text-muted-foreground mt-1 flex-wrap">
                          <span className="sm:hidden font-mono font-medium text-foreground/85 inline-flex items-center gap-1">
                            <Clock className="h-3 w-3 text-muted-foreground/70" />
                            {formatScheduleTime(task)}
                          </span>
                          <span className="sm:hidden text-muted-foreground/40">·</span>
                          <span>{formatScheduleTypeLabel(task)}</span>
                          {projectName && (
                            <>
                              <span className="text-muted-foreground/40">·</span>
                              <span className="inline-flex items-center gap-1 text-muted-foreground/90 font-medium">
                                <FolderGit2 className="h-3 w-3 opacity-70" />
                                <span className="max-w-[120px] truncate">{projectName}</span>
                              </span>
                            </>
                          )}
                          {task.next_run_at && task.enabled && (
                            <>
                              <span className="text-muted-foreground/40">·</span>
                              <span className="text-muted-foreground/80">
                                {t('Next run:')}{' '}
                                <span className="font-mono">
                                  {new Date(task.next_run_at).toLocaleTimeString([], {
                                    hour: '2-digit',
                                    minute: '2-digit',
                                  })}
                                </span>
                              </span>
                            </>
                          )}
                          {!task.enabled && (
                            <>
                              <span className="text-muted-foreground/40">·</span>
                              <span className="text-muted-foreground/60 italic">
                                {t('Paused')}
                              </span>
                            </>
                          )}
                        </div>
                      </div>
                    </div>

                    {/* Right: Switch Toggle & Expand Chevron */}
                    <div className="flex shrink-0 items-center gap-2 sm:gap-3 pl-1 self-center">
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
                            ? 'bg-emerald-600 dark:bg-emerald-500'
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

                        <div className="flex flex-wrap items-center gap-2">
                          <button
                            type="button"
                            onClick={(e) => {
                              e.stopPropagation()
                              handleOpenEdit(task)
                            }}
                            title={t('Edit schedule')}
                            className="action-secondary text-xs px-2.5 min-h-11 sm:min-h-9 text-muted-foreground hover:text-foreground inline-flex items-center gap-1.5"
                          >
                            <Pencil className="h-3.5 w-3.5" />
                            <span>{t('Edit')}</span>
                          </button>

                          <button
                            type="button"
                            onClick={(e) => {
                              e.stopPropagation()
                              void handleDelete(task.id)
                            }}
                            title={t('Delete schedule')}
                            className="action-secondary text-xs px-2.5 min-h-11 sm:min-h-9 text-muted-foreground hover:text-destructive inline-flex items-center gap-1.5"
                          >
                            <Trash2 className="h-3.5 w-3.5" />
                            <span>{t('Delete')}</span>
                          </button>

                          <button
                            type="button"
                            onClick={(e) => {
                              e.stopPropagation()
                              void handleTrigger(task.id)
                            }}
                            disabled={isTriggering || !task.enabled}
                            className="action-secondary text-xs inline-flex items-center gap-1.5 px-3 min-h-11 sm:min-h-9"
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

      {/* Create / Edit SlideDrawer (Spacious Side Sheet & Mobile Full-Screen with URL Deep Linking) */}
      <SlideDrawer
        isOpen={isDrawerOpen}
        onClose={handleCloseDrawer}
        title={editingTask ? t('Edit Scheduled Routine') : t('New Scheduled Routine')}
        description={
          editingTask
            ? t('Update routine parameters and agent execution instructions.')
            : t('Configure routine parameters and agent execution instructions.')
        }
        badge={
          editingTask ? (
            <span className="font-mono text-[11px] text-muted-foreground">
              {editingTask.id}
            </span>
          ) : undefined
        }
        size="lg"
        footer={
          <div className="flex items-center justify-end gap-2 w-full">
            <button
              type="button"
              onClick={handleCloseDrawer}
              className="action-secondary min-h-11 sm:min-h-9 px-4 text-xs"
            >
              {t('Cancel')}
            </button>
            <button
              type="submit"
              form="schedule-form"
              disabled={submitting}
              className="action-primary min-h-11 sm:min-h-9 px-4 text-xs font-medium"
            >
              {submitting
                ? t('Saving...')
                : editingTask
                  ? t('Save Changes')
                  : t('Save Schedule')}
            </button>
          </div>
        }
      >
        <form id="schedule-form" onSubmit={handleSubmit} className="space-y-5">
          {/* 1. Judul Rutinitas */}
          <FormField label={t('Routine Title')}>
            <Input
              type="text"
              required
              placeholder={t('e.g. Follow-up Pelanggan WhatsApp')}
              value={title}
              onChange={(e) => setTitle(e.target.value)}
              className="font-medium"
            />
          </FormField>

          {/* 2. Proyek Target (Dedicated full-width row with contextual help text) */}
          <FormField
            label={t('Target Project')}
            helperText={t('Binds this routine to a specific project repository and isolated worktree.')}
          >
            <Select
              value={projectId}
              onChange={(e) => setProjectId(e.target.value)}
            >
              <option value="">{t('None / System-wide')}</option>
              {projects.map((p) => (
                <option key={p.id} value={p.id}>
                  {p.name}
                </option>
              ))}
            </Select>
          </FormField>

          {/* 3. Kadens & Waktu Eksekusi (Clean symmetrical card with zero asymmetric gaps) */}
          <FormCard
            title={t('Execution Cadence')}
            icon={<Clock className="h-3.5 w-3.5 text-muted-foreground/70" />}
            accessory={timezone}
          >
            {/* Daily: 2 equal 50% columns */}
            {scheduleType === 'daily' && (
              <div className="grid grid-cols-1 sm:grid-cols-2 gap-3">
                <FormField label={t('Frequency')}>
                  <Select
                    value={scheduleType}
                    onChange={(e) => setScheduleType(e.target.value as any)}
                  >
                    <option value="daily">{t('Daily')}</option>
                    <option value="weekly">{t('Weekly')}</option>
                    <option value="interval">{t('Interval (Minutes)')}</option>
                  </Select>
                </FormField>

                <FormField label={t('Execution Time')}>
                  <Input
                    type="time"
                    required
                    mono
                    value={timeOfDay}
                    onChange={(e) => setTimeOfDay(e.target.value)}
                  />
                </FormField>
              </div>
            )}

            {/* Weekly: 3 equal 33% columns */}
            {scheduleType === 'weekly' && (
              <div className="grid grid-cols-1 sm:grid-cols-3 gap-3">
                <FormField label={t('Frequency')}>
                  <Select
                    value={scheduleType}
                    onChange={(e) => setScheduleType(e.target.value as any)}
                  >
                    <option value="daily">{t('Daily')}</option>
                    <option value="weekly">{t('Weekly')}</option>
                    <option value="interval">{t('Interval (Minutes)')}</option>
                  </Select>
                </FormField>

                <FormField label={t('Day of Week')}>
                  <Select
                    value={dayOfWeek}
                    onChange={(e) => setDayOfWeek(Number(e.target.value))}
                  >
                    <option value={1}>{t('Monday')}</option>
                    <option value={2}>{t('Tuesday')}</option>
                    <option value={3}>{t('Wednesday')}</option>
                    <option value={4}>{t('Thursday')}</option>
                    <option value={5}>{t('Friday')}</option>
                    <option value={6}>{t('Saturday')}</option>
                    <option value={0}>{t('Sunday')}</option>
                  </Select>
                </FormField>

                <FormField label={t('Execution Time')}>
                  <Input
                    type="time"
                    required
                    mono
                    value={timeOfDay}
                    onChange={(e) => setTimeOfDay(e.target.value)}
                  />
                </FormField>
              </div>
            )}

            {/* Interval: 2 equal 50% columns */}
            {scheduleType === 'interval' && (
              <div className="grid grid-cols-1 sm:grid-cols-2 gap-3">
                <FormField label={t('Frequency')}>
                  <Select
                    value={scheduleType}
                    onChange={(e) => setScheduleType(e.target.value as any)}
                  >
                    <option value="daily">{t('Daily')}</option>
                    <option value="weekly">{t('Weekly')}</option>
                    <option value="interval">{t('Interval (Minutes)')}</option>
                  </Select>
                </FormField>

                <FormField label={t('Every (Minutes)')}>
                  <Input
                    type="number"
                    min={15}
                    step={15}
                    mono
                    value={intervalMins}
                    onChange={(e) => setIntervalMins(Number(e.target.value))}
                  />
                </FormField>
              </div>
            )}
          </FormCard>

          {/* 4. Instruksi Agen (Prompt) */}
          <FormField
            label={t('Agent Instruction (Prompt)')}
            accessory={t('Markdown and multi-step workflows supported')}
            helperText={t('This instruction will be executed automatically by the AI assistant on schedule.')}
          >
            <div className="rounded-xl border border-border/60 bg-muted/10 overflow-hidden focus-within:border-primary/60 focus-within:ring-1 focus-within:ring-ring/20 transition-all">
              <div className="flex items-center justify-between px-3.5 py-1.5 border-b border-border/30 bg-muted/30 text-[11px] text-muted-foreground font-mono">
                <span>prompt.md</span>
                <span>{prompt.length} {t('characters')}</span>
              </div>
              <textarea
                required
                rows={12}
                placeholder={t('e.g. Cek orderan yang belum dibayar hari ini dan kirimkan reminder sopan via tebar.app')}
                value={prompt}
                onChange={(e) => setPrompt(e.target.value)}
                className="w-full bg-transparent p-3.5 font-mono text-xs leading-relaxed text-foreground placeholder:text-muted-foreground/60 focus:outline-none resize-y min-h-[260px]"
              />
            </div>
          </FormField>
        </form>
      </SlideDrawer>
    </div>
  )
}
