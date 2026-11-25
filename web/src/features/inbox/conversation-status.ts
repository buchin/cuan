import type { ChatContact } from '../../api/chat'
import type { Task } from '../../api/types'
import { taskProgressModel } from '../../components/ui/TaskProgress'
import { workStatus } from '../tasks/task-ui'
import { isTaskActive } from './task-progress'
import { getRoomActiveSubstage } from './task-substage'

export type ConversationStatusKey =
  | 'done'
  | 'active'
  | 'queued'
  | 'review'
  | 'failed'
  | 'partial'
  | 'none'

const QUEUED_NOTICE_RE = /(?:Work queued|The work is (?:queued waiting|waiting for an available AI session|waiting to start)|Pekerjaan (?:masuk dalam antrean|sedang menunggu ketersediaan sesi AI|sedang menunggu untuk dimulai))/i
const COMPLETION_NOTICE_RE = /(?:Hasil pekerjaan (?:memenuhi kriteria|ini sudah diterima)|Pekerjaan (?:telah (?:berhasil )?diselesaikan|ditandai selesai)|The work has been completed and verified)/i

const QUEUED_SUBSTAGES = new Set([
  'queued',
  'menunggu antrean',
  'dalam antrean',
  'waiting to deploy',
  'waiting for checks',
  'menunggu pemeriksaan',
  'menunggu rilis',
])

const QUEUED_SPECIFIC_SUBSTAGES = new Set([
  'waiting to deploy',
  'waiting for checks',
  'menunggu pemeriksaan',
  'menunggu rilis',
])

const REVIEW_SUBSTAGES = new Set([
  'ready for review',
  'siap ditinjau',
  'checking result',
  'memeriksa hasil',
  'waiting for checks',
  'menunggu pemeriksaan',
  'waiting for review',
  'answered',
])

const TERMINAL_SUBSTAGES = new Set([
  'completed',
  'cancelled',
  'canceled',
  'done',
])

export function resolveScopedSubstage(
  status: ConversationStatusKey,
  substage?: string,
): string | undefined {
  if (!substage) return undefined
  const normalized = substage.trim().toLowerCase()
  if (!normalized) return undefined

  if (status === 'active') {
    if (
      QUEUED_SUBSTAGES.has(normalized) ||
      (REVIEW_SUBSTAGES.has(normalized) && normalized !== 'checking result' && normalized !== 'memeriksa hasil') ||
      TERMINAL_SUBSTAGES.has(normalized)
    ) {
      return undefined
    }
    return substage
  }
  if (status === 'queued') {
    return QUEUED_SPECIFIC_SUBSTAGES.has(normalized) ? substage : undefined
  }
  if (status === 'review') {
    return REVIEW_SUBSTAGES.has(normalized) ? substage : undefined
  }
  return undefined
}

export interface ResolvedConversationStatus {
  status: ConversationStatusKey
  activeTasks: number
  queuedTasks: number
  reviewTasks: number
  failedTasks: number
  doneTasks: number
  totalTasks: number
  hasTasks: boolean
  allTasksDone: boolean
  isResolved: boolean
  isComplete: boolean
  isWorking: boolean
  isQueued: boolean
  hasReview: boolean
  hasFailed: boolean
  substage?: string
  fraction: number
}

export function resolveConversationStatus(
  contact?: ChatContact,
  message?: string,
): ResolvedConversationStatus {
  const activeTasks = typeof contact?.active_tasks === 'number' ? contact.active_tasks : Number(contact?.active_tasks) || 0
  const queuedTasks = typeof contact?.queued_tasks === 'number' ? contact.queued_tasks : Number(contact?.queued_tasks) || 0
  const reviewTasks = typeof contact?.review_tasks === 'number' ? contact.review_tasks : Number(contact?.review_tasks) || 0
  const failedTasks = typeof contact?.failed_tasks === 'number' ? contact.failed_tasks : Number(contact?.failed_tasks) || 0
  const doneTasks = typeof contact?.done_tasks === 'number' ? contact.done_tasks : Number(contact?.done_tasks) || 0

  const progress = {
    total: contact?.task_count === undefined ? undefined : contact.task_count - (contact.canceled_tasks || 0),
    done: doneTasks,
    canceled: contact?.canceled_tasks,
    active: activeTasks,
    queued: queuedTasks,
    review: reviewTasks,
    blocked: failedTasks,
  }
  const progModel = taskProgressModel(progress)
  const totalTasks = contact?.task_count !== undefined
    ? Math.max(0, contact.task_count - (contact.canceled_tasks || 0))
    : (progModel?.total ?? 0)
  const hasTasks = totalTasks > 0 || !!progModel
  const allTasksDone = hasTasks && (progModel ? progModel.complete : doneTasks >= totalTasks)

  const roomId = contact?.conversation_id || contact?.id
  const cachedSubstage = roomId ? getRoomActiveSubstage(roomId) : undefined
  const substage = contact?.active_substage || cachedSubstage
  const rawMessage = message ?? contact?.last_message ?? ''
  const isResolved = Boolean(contact?.resolved)
  const isQueuedNotice = rawMessage ? QUEUED_NOTICE_RE.test(rawMessage) : false
  const isCompletionNotice = rawMessage ? COMPLETION_NOTICE_RE.test(rawMessage) : false

  const isWorking = activeTasks > 0
  const isQueued = !isWorking && !allTasksDone && !isResolved && (
    queuedTasks > 0 ||
    (!reviewTasks && !failedTasks && (
      isQueuedNotice ||
      substage === 'Menunggu antrean' ||
      substage === 'Queued'
    ))
  )
  const hasReview = reviewTasks > 0
  const hasFailed = failedTasks > 0
  const isComplete = isResolved || allTasksDone || (!isWorking && !isQueued && !hasReview && !hasFailed && isCompletionNotice)

  let status: ConversationStatusKey = 'none'
  if (isComplete) {
    status = 'done'
  } else if (isWorking) {
    status = 'active'
  } else if (isQueued) {
    status = 'queued'
  } else if (hasReview) {
    status = 'review'
  } else if (hasFailed) {
    status = 'failed'
  } else if (hasTasks) {
    status = 'partial'
  }

  const fraction = progModel ? progModel.fraction : (totalTasks > 0 ? doneTasks / totalTasks : 0)

  return {
    status,
    activeTasks,
    queuedTasks,
    reviewTasks,
    failedTasks,
    doneTasks,
    totalTasks,
    hasTasks,
    allTasksDone,
    isResolved,
    isComplete,
    isWorking,
    isQueued,
    hasReview,
    hasFailed,
    substage,
    fraction,
  }
}

/** Pick the highest-priority task matching the Inbox status order: active -> queued -> review -> failed -> first. */
export function pickPriorityTask(tasks: Task[]): Task | undefined {
  if (!tasks.length) return undefined
  return (
    tasks.find(t => workStatus(t) === 'Working') ||
    tasks.find(t => isTaskActive(t)) ||
    tasks.find(t => workStatus(t) === 'Ready for review') ||
    tasks.find(t => workStatus(t) === 'Needs attention') ||
    tasks[0]
  )
}
