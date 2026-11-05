import type { Task } from '../../api/types'
import { workStatus } from './task-ui'

export function taskAttention(task: Task) {
  if (task.review_state || workStatus(task) !== 'Needs attention') return null
  const error = task.latest_attempt?.error_message || ''
  if (/timed out|execution deadline exceeded|hard execution timeout/i.test(error)) {
    return { reason: 'Work stopped before verification finished.' }
  }
  if (/project verification failed|project checks did not pass/i.test(error)) {
    return { reason: 'The required checks have not passed.' }
  }
  if (/sign-in|login required|invalid_grant|authentication expired/i.test(error)) {
    return { reason: 'The AI connection needs attention.' }
  }
  return { reason: 'A verified result is still missing.' }
}
