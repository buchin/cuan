import { useEffect, useState, type ReactNode } from 'react'
import type { Task } from '../../api/types'
import { useLanguage } from '../../context/LanguageContext'
import { taskExecutionActive } from './task-ui'

export function TaskDuration({ task, compact = false, fallback = null, className = '' }: {
  task: Task; compact?: boolean; fallback?: ReactNode; className?: string
}) {
  const { language } = useLanguage()
  const [now, setNow] = useState(Date.now)
  const done = task.status === 'done'
  const active = !done && taskExecutionActive(task)
  useEffect(() => {
    if (!active) return
    setNow(Date.now())
    const timer = setInterval(() => setNow(Date.now()), 60_000)
    return () => clearInterval(timer)
  }, [active, task.id])
  const start = Date.parse(task.created_at)
  const end = done ? Date.parse(task.completed_at || '') : now
  if ((!done && !active) || !Number.isFinite(start) || !Number.isFinite(end) || end < start) return <>{fallback}</>
  const minutes = Math.floor((end - start) / 60_000)
  const id = language === 'id'
  const duration = minutes < 1 ? (id ? 'kurang dari 1 menit' : 'less than 1 minute')
    : minutes < 60 ? `${minutes} ${id ? 'menit' : minutes === 1 ? 'minute' : 'minutes'}`
    : minutes < 1440 ? `${Math.floor(minutes / 60)} ${id ? 'jam' : Math.floor(minutes / 60) === 1 ? 'hour' : 'hours'}${minutes % 60 ? ` ${minutes % 60} ${id ? 'menit' : minutes % 60 === 1 ? 'minute' : 'minutes'}` : ''}`
    : `${Math.floor(minutes / 1440)} ${id ? 'hari' : Math.floor(minutes / 1440) === 1 ? 'day' : 'days'}${Math.floor(minutes % 1440 / 60) ? ` ${Math.floor(minutes % 1440 / 60)} ${id ? 'jam' : Math.floor(minutes % 1440 / 60) === 1 ? 'hour' : 'hours'}` : ''}`
  const label = done ? `${id ? 'Selesai dalam' : 'Completed in'} ${duration}` : `${id ? 'Durasi' : 'Elapsed'} ${duration}`
  const explanation = id ? 'Sejak task dibuat, termasuk antrean, pengerjaan, jeda, dan verifikasi.' : 'Since task creation, including queueing, work, pauses and verification.'
  return <time data-task-duration dateTime={`PT${Math.floor((end - start) / 1000)}S`} aria-label={label} title={`${label}. ${explanation}`} className={`text-xs font-normal tabular-nums text-muted-foreground ${className}`}>
    {compact ? duration : label}
  </time>
}
