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
          succeeded: 'Finished',
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
        succeeded: 'Finished',
        canceled: 'Cancelled',
        running: 'Running',
        queued: 'Queued',
        failed: 'Failed',
        interrupted: 'Interrupted',
      } as Record<string, string>
    )[attempt.status] || attempt.status
  )
}

// One user-facing status across project lists and session headers. A finished
// process is not proof that a task was accepted or completed.
export function workStatus(task: Task) {
  if (task.launch?.state === 'queued' || task.launch?.state === 'claimed') return 'Waiting to start'
  const attempt = task.latest_attempt?.status || task.execution_status
  if (attempt === 'running' || attempt === 'queued') return attempt === 'running' ? 'Working' : 'Waiting to start'
  if (task.status === 'done') return 'Completed'
  if (task.launch?.state === 'stopped') return 'Needs attention'
  if (task.launch?.state === 'cancelled') return 'Cancelled'
  if (task.status === 'blocked' || ['failed', 'interrupted'].includes(attempt || '')) return 'Needs attention'
  if (attempt === 'canceled' || attempt === 'cancelled') return 'Cancelled'
  if (task.status === 'review') return task.result_kind === 'answer' && ['succeeded', 'completed'].includes(attempt || '') ? 'Answered' : 'Ready for review'
  if (task.status === 'doing') return 'Not running'
  return 'Not started'
}
export function tasksForProject(tasks: Task[], projectId: string) {
  return tasks.filter((task) => task.project_id === projectId || task.project_ids?.includes(projectId) ||
    (projectId === 'default' && !task.project_id && !task.project_ids?.length))
}

export const agentDisplayName = (value?: string) => value?.startsWith('agent_') ? '@' + value.slice(6) : value || 'Agent'

export function isTaskRunning(task: Task) {
  const attempt = task.latest_attempt?.status || task.execution_status
  return (
    attempt === 'running' ||
    attempt === 'queued' ||
    task.launch?.state === 'queued' ||
    task.launch?.state === 'claimed'
  )
}

export function workStatusBadgeClass(status: string) {
  switch (status) {
    case 'Working':
      return 'bg-blue-500/10 text-blue-600 dark:text-blue-400 border-blue-500/20'
    case 'Waiting to start':
      return 'bg-amber-500/10 text-amber-600 dark:text-amber-400 border-amber-500/20'
    case 'Completed':
    case 'Answered':
      return 'bg-emerald-500/10 text-emerald-600 dark:text-emerald-400 border-emerald-500/20'
    case 'Ready for review':
      return 'bg-purple-500/10 text-purple-600 dark:text-purple-400 border-purple-500/20'
    case 'Needs attention':
      return 'bg-destructive/10 text-destructive border-destructive/20'
    case 'Cancelled':
    case 'Not running':
    case 'Not started':
    default:
      return 'bg-muted text-muted-foreground border-border'
  }
}

export function taskTargetUrl(task: { id: string; project_id?: string | null }) {
  return task.project_id
    ? `/projects/${encodeURIComponent(task.project_id)}/tasks/${encodeURIComponent(task.id)}`
    : `/tasks/${encodeURIComponent(task.id)}`
}

export function taskSummaryText(task: Task) {
  const status = workStatus(task)
  const text =
    (status === 'Completed' || status === 'Answered'
      ? task.complete_note || task.summary
      : task.launch?.safe_error || task.summary || task.description || task.body) || ''
  return text.replace(/\s+/g, ' ').trim()
}

