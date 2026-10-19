import type { Task, TaskAttempt, TaskStatus } from '../../api/types'
export const TASK_STATUSES: { id: TaskStatus; label: string }[] = [
  { id: 'backlog', label: 'Backlog' },
  { id: 'todo', label: 'Todo' },
  { id: 'doing', label: 'Doing' },
  { id: 'review', label: 'Review' },
  { id: 'done', label: 'Done' },
  { id: 'blocked', label: 'Blocked' },
]
export const taskStatus = (value: string) =>
  value === 'failed' ? 'blocked' : value
export const statusLabel = (value: string) =>
  TASK_STATUSES.find((s) => s.id === taskStatus(value))?.label || value
export const taskRef = (id: string) =>
  '#' +
  id
    .replace(/^(task_|msg_)/, '')
    .replace(/-/g, '')
    .slice(0, 6)
    .toUpperCase()
export const activeAttempt = (attempt?: TaskAttempt) =>
  !!attempt && ['queued', 'running'].includes(attempt.status)
export function executionLabel(task: Task) {
  const state = task.latest_attempt?.status || task.execution_status
  if (
    (state === 'succeeded' || state === 'completed') &&
    task.status === 'done'
  )
    return 'Finished · approved'
  return state
    ? (
        {
          queued: 'Queued',
          running: 'Running',
          succeeded: 'Finished · review required',
          completed: 'Finished · review required',
          failed: 'Failed',
          canceled: 'Cancelled',
          cancelled: 'Cancelled',
          interrupted: 'Interrupted',
        } as Record<string, string>
      )[state] || ''
    : ''
}
export function relativeTime(value?: string) {
  const n = Date.parse(value || '')
  if (!Number.isFinite(n)) return '—'
  const seconds = Math.max(0, Math.round((Date.now() - n) / 1000))
  if (seconds < 60) return 'Just now'
  if (seconds < 3600) return `${Math.floor(seconds / 60)}m ago`
  if (seconds < 86400) return `${Math.floor(seconds / 3600)}h ago`
  if (seconds < 604800) return `${Math.floor(seconds / 86400)}d ago`
  return new Date(n).toLocaleDateString('en-US', {
    month: 'short',
    day: 'numeric',
  })
}
export function attemptLabel(attempt: TaskAttempt) {
  return (
    (
      {
        succeeded: 'Finished · review required',
        canceled: 'Cancelled',
        running: 'Running',
        queued: 'Queued',
        failed: 'Failed',
        interrupted: 'Interrupted',
      } as Record<string, string>
    )[attempt.status] || attempt.status
  )
}
