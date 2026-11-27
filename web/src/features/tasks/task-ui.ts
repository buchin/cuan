import type { Task, TaskAttempt, TaskDetail, TaskStatus } from '../../api/types'
import type { StatusDotVariant } from '../../components/ui/StatusDot'

export type TaskStateKey = 'review' | 'working' | 'done' | 'queued' | 'attention' | 'neutral'

export interface TaskStatusTheme {
  key: TaskStateKey
  dotVariant: StatusDotVariant
  textColor: string
  badgeClass: string
  counterBadgeClass: string
}

export const TASK_STATUS_THEMES: Record<TaskStateKey, TaskStatusTheme> = {
  review: {
    key: 'review',
    dotVariant: 'review',
    textColor: 'text-purple-600 dark:text-purple-400',
    badgeClass: 'bg-purple-500/15 text-purple-700 dark:text-purple-300',
    counterBadgeClass: 'bg-muted/80 text-muted-foreground',
  },
  working: {
    key: 'working',
    dotVariant: 'working',
    textColor: 'text-[color:var(--status-working)]',
    badgeClass: 'bg-blue-500/15 text-blue-700 dark:text-blue-300',
    counterBadgeClass: 'bg-muted/80 text-muted-foreground',
  },
  done: {
    key: 'done',
    dotVariant: 'success',
    textColor: 'text-[color:var(--status-success)]',
    badgeClass: 'bg-emerald-500/15 text-emerald-800 dark:text-emerald-300',
    counterBadgeClass: 'bg-muted/80 text-muted-foreground',
  },
  queued: {
    key: 'queued',
    dotVariant: 'queued',
    textColor: 'text-muted-foreground',
    badgeClass: 'bg-muted/80 text-muted-foreground',
    counterBadgeClass: 'bg-muted/80 text-muted-foreground',
  },
  attention: {
    key: 'attention',
    dotVariant: 'warning',
    textColor: 'text-destructive',
    badgeClass: 'bg-destructive/15 text-destructive dark:text-red-400',
    counterBadgeClass: 'bg-muted/80 text-muted-foreground',
  },
  neutral: {
    key: 'neutral',
    dotVariant: 'neutral',
    textColor: 'text-muted-foreground',
    badgeClass: 'bg-muted/80 text-muted-foreground',
    counterBadgeClass: 'bg-muted/80 text-muted-foreground',
  },
}

export function getTaskStateKey(statusOrTask: string | Task): TaskStateKey {
  if (typeof statusOrTask === 'object' && statusOrTask !== null) {
    const task = statusOrTask
    if (task.status === 'done') return 'done'
    if (task.review_state || task.status === 'review') return 'review'
    const ws = workStatus(task)
    if (ws === 'Completed' || ws === 'Answered') return 'done'
    if (ws === 'Ready for review') return 'review'
    if (ws === 'Working') return 'working'
    if (ws === 'Queued') return 'queued'
    if (ws === 'Needs attention') return 'attention'
    return 'neutral'
  }
  const s = String(statusOrTask).toLowerCase()
  if (['review', 'ready for review', 'waiting for review', 'waiting for checks', 'checking result', 'memeriksa hasil', 'menunggu pemeriksaan'].includes(s)) return 'review'
  if (['working', 'doing', 'running', 'in progress', 'deploying'].includes(s)) return 'working'
  if (['done', 'completed', 'answered', 'success'].includes(s)) return 'done'
  if (['queued', 'waiting to deploy', 'waiting to start', 'pending'].includes(s)) return 'queued'
  if (['needs attention', 'attention', 'blocked', 'failed', 'warning'].includes(s)) return 'attention'
  return 'neutral'
}

export function getTaskStatusTheme(statusOrTask: string | Task): TaskStatusTheme {
  const key = getTaskStateKey(statusOrTask)
  return TASK_STATUS_THEMES[key] || TASK_STATUS_THEMES.neutral
}
export const TASK_STATUSES: { id: TaskStatus; label: string }[] = [
  { id: 'backlog', label: 'Backlog' },
  { id: 'todo', label: 'Todo' },
  { id: 'queued', label: 'Queued' },
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
  if (task.status === 'done') return 'Completed'
  if (['cancelled', 'canceled'].includes(task.status as string)) return 'Cancelled'
  if (task.launch?.state === 'queued' || task.launch?.state === 'claimed') return 'Queued'
  const attempt = task.latest_attempt?.status || task.execution_status
  if (attempt === 'running') return 'Working'
  if (attempt === 'queued') return 'Queued'
  if (attempt === 'canceled' || attempt === 'cancelled') return 'Cancelled'
  if (task.status === 'review') return task.result_kind === 'answer' && ['succeeded', 'completed'].includes(attempt || '') ? 'Answered' : 'Ready for review'
  if (task.launch?.state === 'stopped') return 'Needs attention'
  if (task.launch?.state === 'cancelled') return 'Cancelled'
  if (task.status === 'blocked' || ['failed', 'interrupted'].includes(attempt || '')) return 'Needs attention'
  if ((task.status as string) === 'queued' || (task.status as string) === 'pending') return 'Queued'
  if (task.status === 'doing') return 'Not running'
  return 'Not started'
}

// Display the actual automated checking phase without changing execution,
// queue, terminal availability or completion semantics.
export function workDisplayStatus(task: Task) {
  const status = workStatus(task)
  if (status === 'Completed' || status === 'Cancelled') return status
  if (status === 'Queued') {
    if (task.launch?.kind === 'native_release') return 'Waiting to deploy'
    if (task.launch?.kind === 'verification' || task.launch?.kind === 'inspection') return 'Waiting for checks'
    if (task.review_state !== 'queued') return 'Queued'
  }
  if (status === 'Working' && task.launch?.kind === 'native_release') return 'Deploying'
  if (task.review_state === 'running') return 'Checking result'
  if (task.review_state === 'queued') return 'Waiting for checks'
  if (status === 'Needs attention') {
    if (task.review_action === 'needs_owner') return 'Needs your input'
    if (task.review_action === 'wait') return 'Waiting'
    return 'Unfinished'
  }
  return status
}

// Active execution means a runner or review worker is actively executing right now.
export function taskExecutionActive(task: Task): boolean {
  if (['done', 'cancelled', 'canceled'].includes(task.status as string)) return false
  const attempt = task.latest_attempt
  if (attempt?.cancel_requested || attempt?.canceled_at || ['cancelled', 'canceled'].includes(attempt?.status || '')) return false
  if (task.review_state === 'running') return true
  const attemptStatus = attempt?.status || task.execution_status
  return attemptStatus === 'running'
}

export function taskAgentQueued(task: Task): boolean {
  if (['done', 'cancelled', 'canceled'].includes(task.status as string)) return false
  if (taskExecutionActive(task)) return false
  const status = workStatus(task)
  return status === 'Queued' || (task.status as string) === 'queued' || (task.status as string) === 'pending' || task.latest_attempt?.status === 'queued' || task.launch?.state === 'queued' || task.review_state === 'queued'
}

// Pulse expresses ownership of the unfinished work, including recovery and
// external waits. Native execution/terminal state remains separate.
export function taskAgentPending(task: Task) {
  if (['done', 'cancelled', 'canceled'].includes(task.status as string)) return false
  if (task.launch?.state === 'queued' || task.launch?.state === 'claimed') return true
  const attempt = task.latest_attempt
  if (attempt?.cancel_requested || attempt?.canceled_at || ['cancelled', 'canceled'].includes(attempt?.status || '')) return false
  if (task.review_state === 'queued' || task.review_state === 'running') return true
  if (task.source_room_id && attempt && task.review_action !== 'needs_owner') return true
  return ['queued', 'running'].includes(attempt?.status || task.execution_status || '')
}

export function getQueuedTasksPositions(tasks: Task[]): Map<string, number> {
  const positions = new Map<string, number>()
  const queued = tasks
    .filter((t) => {
      const status = workStatus(t)
      return (
        status === 'Queued' ||
        (t.status as string) === 'queued' ||
        (t.status as string) === 'pending' ||
        t.latest_attempt?.status === 'queued' ||
        t.launch?.state === 'queued'
      )
    })
    .sort((a, b) => new Date(a.created_at || a.updated_at).getTime() - new Date(b.created_at || b.updated_at).getTime())

  queued.forEach((t, idx) => {
    positions.set(t.id, idx + 1)
  })
  return positions
}
export function tasksForProject(tasks: Task[], projectId: string) {
  return tasks.filter((task) => task.project_id === projectId || task.project_ids?.includes(projectId) ||
    (projectId === 'default' && !task.project_id && !task.project_ids?.length))
}

export const agentDisplayName = (value?: string) => value?.startsWith('agent_') ? '@' + value.slice(6) : value || 'Agent'

export function isTaskRunning(task: Task) {
  if (task.status === 'done') return false
  const attempt = task.latest_attempt?.status || task.execution_status
  return (
    attempt === 'running' ||
    attempt === 'queued' ||
    task.launch?.state === 'queued' ||
    task.launch?.state === 'claimed' ||
    (task.status as string) === 'queued' ||
    (task.status as string) === 'pending'
  )
}

export function workStatusBadgeClass(status: string) {
  switch (status) {
    case 'Working':
      return TASK_STATUS_THEMES.working.badgeClass
    case 'Waiting to start':
    case 'Queued':
      return TASK_STATUS_THEMES.queued.badgeClass
    case 'Completed':
    case 'Answered':
      return TASK_STATUS_THEMES.done.badgeClass
    case 'Ready for review':
      return TASK_STATUS_THEMES.review.badgeClass
    case 'Needs attention':
      return TASK_STATUS_THEMES.attention.badgeClass
    case 'Cancelled':
      return 'bg-zinc-500/15 text-zinc-700 dark:text-zinc-300'
    case 'Not running':
    case 'Not started':
    default:
      return TASK_STATUS_THEMES.neutral.badgeClass
  }
}

export function taskStatusBadgeClass(status: TaskStatus | string) {
  switch (status) {
    case 'doing':
      return TASK_STATUS_THEMES.working.badgeClass
    case 'queued':
      return TASK_STATUS_THEMES.queued.badgeClass
    case 'review':
      return TASK_STATUS_THEMES.review.badgeClass
    case 'done':
      return TASK_STATUS_THEMES.done.badgeClass
    case 'blocked':
    case 'failed':
      return TASK_STATUS_THEMES.attention.badgeClass
    case 'todo':
    case 'backlog':
    default:
      return TASK_STATUS_THEMES.neutral.badgeClass
  }
}

export function taskPriorityBadgeClass(priority?: string) {
  switch (priority) {
    case 'urgent':
      return 'bg-red-500/15 text-red-700 dark:text-red-300 border-red-500/30'
    case 'high':
      return 'bg-amber-500/15 text-amber-800 dark:text-amber-200 border-amber-500/35'
    case 'normal':
      return 'bg-blue-500/10 text-blue-700 dark:text-blue-300 border-blue-500/25'
    case 'low':
      return 'bg-zinc-500/10 text-zinc-700 dark:text-zinc-300 border-zinc-500/20'
    default:
      return 'bg-muted text-muted-foreground border-border/60'
  }
}

export interface ParsedTaskInstructions {
  cleanText: string
  isDelegation: boolean
  metadata: {
    protocol?: string
    scopeDigest?: string
    approvalMessage?: string
    userAuth?: string
    conversationId?: string
  } | null
}

export function parseTaskInstructions(text?: string): ParsedTaskInstructions {
  if (!text) return { cleanText: '', isDelegation: false, metadata: null }
  const isDelegation = /^Cuan managed delegation/im.test(text.trim())
  if (!isDelegation) return { cleanText: text, isDelegation: false, metadata: null }

  let scopeDigest: string | undefined
  const digestMatch = text.match(/^Approved scope digest:\s*([a-f0-9]+)/im)
  if (digestMatch) scopeDigest = digestMatch[1]

  let approvalMessage: string | undefined
  const appMsgMatch = text.match(/^Approval message:\s*([^\n]+)/im)
  if (appMsgMatch) approvalMessage = appMsgMatch[1].trim()

  let userAuth: string | undefined
  const userAuthMatch = text.match(/^User authorization:\s*([^\n]+)/im)
  if (userAuthMatch) userAuth = userAuthMatch[1].trim()

  let conversationId: string | undefined
  const convMatch = text.match(/^Conversation:\s*([a-f0-9-]+)/im)
  if (convMatch) conversationId = convMatch[1].trim()

  // Clean the human instructions body
  let clean = text
    .replace(/^Cuan managed delegation[^\n]*\n?/im, '')
    .replace(/^Approved scope digest:[^\n]*\n?/im, '')
    .replace(/^Approval message:[^\n]*(\n|$)/im, '')
    .replace(/^User authorization:[^\n]*(\n|$)/im, '')
    .replace(/^Conversation:[^\n]*(\n|$)/im, '')
    .replace(/^Report actual progress[^\n]*(\n|$)/im, '')
    .replace(/^Project guidance snapshot[\s\S]*?```\s*$/im, '')
    .replace(/^## User attachments[\s\S]*?(?=(?:\n[A-Z]|\n\n|$))/im, '')
    .trim()

  return {
    cleanText: clean,
    isDelegation: true,
    metadata: {
      protocol: 'Cuan managed delegation v1',
      scopeDigest,
      approvalMessage,
      userAuth,
      conversationId,
    },
  }
}

export function taskTargetUrl(task: { id: string; project_id?: string | null }) {
  return `/tasks/${encodeURIComponent(task.id)}`
}

export function taskDetailUrl(task: { id: string; project_id?: string | null }) {
  return `/tasks/${encodeURIComponent(task.id)}?view=detail`
}

export function taskTerminalUrl(task: { id: string; project_id?: string | null }) {
  return `/tasks/${encodeURIComponent(task.id)}/sessions/latest`
}

// Regex to clean ANSI escape codes (CSI, OSC, etc.)
const ANSI_REGEX = /\x1b(?:[@-Z\\-_]|\[[0-?]*[ -/]*[@-~]|\].*?(?:\x07|\x1b\\)|[PX^_].*?\x1b\\)/g
// Regex to clean terminal spinners (Unicode braille patterns ⠋..⣯), bullets, prompts, and leading punctuation
const SPINNER_BULLET_REGEX = /^[\s\x00-\x1f\r\b\x7f\u2800-\u28ff\u2022\u25cb\u25cf\u25aa\u25ab\u25e6\*\-\+\>\$\:\;\.\|·⋮…]+/
const NOISE_PAREN_REGEX = /\(ctrl\+[a-z]\s+to\s+expand\)/gi

export function cleanTerminalText(text: string): string {
  if (!text) return ''
  let cleaned = text
    .replace(ANSI_REGEX, '')
    .replace(NOISE_PAREN_REGEX, '')
    .replace(/[\r\b]/g, '')
    .replace(SPINNER_BULLET_REGEX, '')
    .replace(/\s+/g, ' ')
    .trim()
  return cleaned
}

export function isTerminalNoiseLine(line: string): boolean {
  if (!line || line.length < 3) return true
  const lower = line.toLowerCase()
  if (lower.startsWith('esc to cancel') || lower.startsWith('ctrl+')) return true
  if (lower.includes('gemini 3.') || lower.includes('claude 3.') || lower.includes('gpt-4')) return true
  if (/^[\s─\-_=\*~]+$/.test(line)) return true
  if (line === '>' || line === '$') return true
  return false
}

export function cleanTaskDescription(desc: string): string {
  if (!desc) return ''
  let cleaned = desc
    .replace(/^Cuan managed delegation[^\n]*\n?/i, '')
    .replace(/^Approved scope digest:[^\n]*\n?/i, '')
    .replace(/^Approval message:[^\n]*\n?/im, '')
    .replace(/^User authorization:[^\n]*\n?/im, '')
    .replace(/^## User attachments[\s\S]*$/im, '')
    .replace(/^Conversation:[^\n]*\n?/im, '')
    .replace(/^Report actual progress[\s\S]*$/im, '')
    .trim()
  return cleaned.replace(/\s+/g, ' ').trim()
}

export function taskLiveActivityText(task: Task | TaskDetail, fallbackText: string = 'Runner aktif'): string {
  if (task.latest_progress) {
    const cleaned = cleanTerminalText(task.latest_progress)
    if (cleaned && !isTerminalNoiseLine(cleaned)) {
      return cleaned
    }
  }
  // Check if activities contain an agent progress report
  const activities = 'activities' in task ? task.activities : undefined
  if (activities && activities.length > 0) {
    for (let i = activities.length - 1; i >= 0; i--) {
      const act = activities[i]
      if (task.latest_attempt?.id && act.attempt_id === task.latest_attempt.id && act.activity_type === 'agent_report' && act.new_status === 'progress' && act.message) {
        const cleaned = cleanTerminalText(act.message.split('\n')[0])
        if (cleaned && !isTerminalNoiseLine(cleaned)) return cleaned
      }
    }
  }
  return fallbackText
}

const PRE_PUBLICATION_GAP_RE = /\s*Remaining gap:\s*[^\n]*(?:not yet pushed|not yet merged|not yet deployed|belum di-push|belum di-merge|belum dirilis)[^\n]*\.?/gim
const UNCONFIRMED_DEPLOY_RE = /\s*Verifikasi deployment production belum dikonfirmasi\.?/gim

/** Removes stale pre-publication gap notes once a task is done/live. */
export function sanitizeCompletedTaskNote(text: string, isDone: boolean): string {
  if (!text) return ''
  if (!isDone) return text.trim()
  return text
    .replace(PRE_PUBLICATION_GAP_RE, '')
    .replace(UNCONFIRMED_DEPLOY_RE, '')
    .replace(/\n{3,}/g, '\n\n')
    .trim()
}

function conciseLiveMarkerLine(note: string): string {
  const match = note.match(/🚀\s*Perubahan telah (?:diverifikasi aktif pada rilis live|berhasil diterapkan)[^\n]*/)
  return match ? match[0].trim() : ''
}

function stripTechnicalVerificationNoise(text: string): string {
  return text
    .replace(/\s*\((?:saved in|audit in)[^)]*session:[^)]*\)/gi, '')
    .replace(/\b([0-9a-f]{8})[0-9a-f]{32}\b/gi, '$1')
    .replace(/\n{3,}/g, '\n\n')
    .trim()
}

/** Concise, non-contradictory task summary for the Conversation tasks drawer. */
export function taskConversationSummary(task: Task): string {
  const work = workStatus(task)
  const isDone = task.status === 'done'
  const cleanNote = sanitizeCompletedTaskNote(task.complete_note || '', isDone)
  const cleanSummary = sanitizeCompletedTaskNote(task.summary || '', isDone)
  if (work === 'Needs attention') {
    return task.review_finding || cleanNote || cleanSummary
  }
  if (work === 'Completed' || work === 'Answered') {
    if (cleanSummary && !cleanSummary.startsWith('{')) {
      const liveMarker = conciseLiveMarkerLine(cleanNote)
      if (liveMarker && !cleanSummary.includes(liveMarker) && !cleanSummary.includes('rilis live')) {
        return `${cleanSummary}\n\n${liveMarker}`
      }
      return cleanSummary
    }
    return stripTechnicalVerificationNoise(cleanNote)
  }
  return taskLiveActivityText(task, '') || cleanNote || cleanSummary
}

export function taskSummaryText(task: Task) {
  const status = workStatus(task)
  if (status === 'Completed' || status === 'Answered') {
    const text = sanitizeCompletedTaskNote(task.complete_note || task.summary || '', task.status === 'done')
    return text.replace(/\s+/g, ' ').trim()
  }
  if (status === 'Needs attention') {
    const text =
      task.launch?.safe_error ||
      task.latest_attempt?.error_message ||
      task.summary ||
      ''
    return text.replace(/\s+/g, ' ').trim()
  }
  const text = task.summary || cleanTaskDescription(task.description || task.body || '')
  return text.replace(/\s+/g, ' ').trim()
}

