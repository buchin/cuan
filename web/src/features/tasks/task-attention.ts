import type { Task } from '../../api/types'
import { workStatus } from './task-ui'

export function taskAttention(task: Task) {
  if (task.review_state || workStatus(task) !== 'Needs attention') return null
  const error = task.latest_attempt?.error_message || ''
  if (/timed out|execution deadline exceeded|hard execution timeout/i.test(error)) {
    return { reason: 'Work stopped before verification finished.', recommendation: 'Review the saved findings and agree on a focused follow-up.' }
  }
  if (/project verification failed|project checks did not pass/i.test(error)) {
    return { reason: 'The required checks have not passed.', recommendation: 'Review the failing check before choosing a correction.' }
  }
  if (/sign-in|login required|invalid_grant|authentication expired/i.test(error)) {
    return { reason: 'The AI connection needs attention.', recommendation: 'Check the connection before continuing the saved work.' }
  }
  return { reason: 'A verified result is still missing.', recommendation: 'Review the saved result and choose the next step.' }
}

export function taskDiscussionDraft(task: Task, t: (key: string) => string) {
  return t('Review the saved findings for') + ` [${task.title}](/tasks/${encodeURIComponent(task.id)}). ` + t('Recommend one focused next step. Do not restart or change anything yet.')
}
