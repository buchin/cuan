import type { Task } from '../../api/types'

export function isTaskActive(task?: Task | null): boolean {
  if (!task) return false
  const s = (task.status || '').toLowerCase().trim()
  const attemptStatus = (task.latest_attempt?.status || '').toLowerCase().trim()
  const launchState = (task.launch?.state || '').toLowerCase().trim()

  // Terminal or review states are not in-flight
  if (s === 'done' || s === 'blocked' || s === 'failed' || s === 'review') {
    return false
  }

  // Active if status is doing or queued
  if (s === 'queued' || s === 'doing') return true

  // Active if attempt is queued or running
  if (attemptStatus === 'queued' || attemptStatus === 'running') return true

  // Active if launch queue state is queued, claimed, or started
  if (launchState === 'queued' || launchState === 'claimed' || launchState === 'started') return true

  return false
}

export function formatDuration(totalSeconds: number): string {
  const safeSeconds = Math.max(0, Math.floor(totalSeconds))
  const minutes = Math.floor(safeSeconds / 60)
  const seconds = safeSeconds % 60
  if (minutes < 60) {
    return `${String(minutes).padStart(2, '0')}:${String(seconds).padStart(2, '0')}`
  }
  const hours = Math.floor(minutes / 60)
  const remainingMinutes = minutes % 60
  return `${hours}:${String(remainingMinutes).padStart(2, '0')}:${String(seconds).padStart(2, '0')}`
}
