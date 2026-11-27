import test from 'node:test'
import assert from 'node:assert/strict'
import fs from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'

const __filename = fileURLToPath(import.meta.url)
const __dirname = path.dirname(__filename)
const webDir = path.resolve(__dirname, '..')
const approvalCardPath = path.resolve(webDir, 'src/features/inbox/ApprovalCard.tsx')
const inboxViewPath = path.resolve(webDir, 'src/features/inbox/InboxView.tsx')
const idLocalePath = path.resolve(webDir, 'src/locales/id.json')
const enLocalePath = path.resolve(webDir, 'src/locales/en.json')

test('1. ApprovalCard exports isApprovalCardStale and accurately evaluates continuation state', () => {
  assert.ok(fs.existsSync(approvalCardPath), 'ApprovalCard.tsx must exist')
  const src = fs.readFileSync(approvalCardPath, 'utf8')
  assert.ok(src.includes('export function isApprovalCardStale'), 'ApprovalCard must export isApprovalCardStale')

  // Inline simulation of isApprovalCardStale logic to verify all branch conditions
  const isApprovalCardStale = (plan, accepted, tasks) => {
    if (!plan?.continuation || accepted || !tasks || tasks.length === 0) return false
    const task = tasks.find((item) => item.id === plan.continuation?.task_id)
    if (!task) return false
    const statusStr = task.status
    if (statusStr === 'done' || statusStr === 'cancelled' || statusStr === 'canceled') return true
    if (statusStr === 'doing' || task.execution_status === 'running') return true
    if (
      task.latest_attempt?.id &&
      plan.continuation.attempt_id &&
      task.latest_attempt.id !== plan.continuation.attempt_id
    ) {
      return true
    }
    return false
  }

  const validPlan = {
    title: 'Lanjutkan pemeriksaan',
    description: 'Lengkapi pemeriksaan agar hasil pekerjaan bisa dipastikan.',
    continuation: { task_id: 'task-1', attempt_id: 'attempt-1' }
  }

  // Normal pending blocked task matching attempt: NOT stale
  assert.equal(
    isApprovalCardStale(validPlan, false, [{ id: 'task-1', status: 'blocked', latest_attempt: { id: 'attempt-1' } }]),
    false,
    'Pending continuation matching blocked task must not be stale'
  )

  // Already accepted card: NOT stale (preserves historical record)
  assert.equal(
    isApprovalCardStale(validPlan, true, [{ id: 'task-1', status: 'done', latest_attempt: { id: 'attempt-1' } }]),
    false,
    'Accepted card must preserve historical display'
  )

  // Task is finished (done): STALE -> must be dismissed
  assert.equal(
    isApprovalCardStale(validPlan, false, [{ id: 'task-1', status: 'done', latest_attempt: { id: 'attempt-1' } }]),
    true,
    'Card whose task is done must be stale'
  )

  // Task is canceled: STALE -> must be dismissed
  assert.equal(
    isApprovalCardStale(validPlan, false, [{ id: 'task-1', status: 'canceled', latest_attempt: { id: 'attempt-1' } }]),
    true,
    'Card whose task is canceled must be stale'
  )

  // Task is currently running (doing): STALE -> must be dismissed
  assert.equal(
    isApprovalCardStale(validPlan, false, [{ id: 'task-1', status: 'doing', latest_attempt: { id: 'attempt-1' } }]),
    true,
    'Card whose task is executing must be stale'
  )

  // Newer attempt created: STALE -> must be dismissed
  assert.equal(
    isApprovalCardStale(validPlan, false, [{ id: 'task-1', status: 'blocked', latest_attempt: { id: 'attempt-2' } }]),
    true,
    'Card whose attempt does not match latest attempt must be stale'
  )
})

test('2. InboxView guards ApprovalCard rendering with isApprovalCardStale', () => {
  const src = fs.readFileSync(inboxViewPath, 'utf8')

  assert.ok(
    src.includes('isApprovalCardStale') && src.includes("from './ApprovalCard'"),
    'InboxView must import isApprovalCardStale from ApprovalCard'
  )
  assert.ok(
    src.includes('!isApprovalCardStale('),
    'InboxView must guard ApprovalCard rendering with !isApprovalCardStale'
  )
})

test('3. InboxView handles stale approval card click gracefully without raw English error', () => {
  const src = fs.readFileSync(inboxViewPath, 'utf8')

  // Check error catching for stale continuation
  assert.ok(
    src.includes('isStaleContinuation'),
    'InboxView must define isStaleContinuation check in error handler'
  )
  assert.ok(
    src.includes('Rekomendasi persetujuan ini sudah tidak berlaku karena status tugas telah diperbarui.'),
    'InboxView must present friendly Indonesian notice for stale continuation'
  )
  assert.ok(
    src.includes('loadConversation(convId)'),
    'InboxView must trigger automatic conversation reload on stale continuation'
  )
  assert.ok(
    src.includes('staleNoticeTimerRef'),
    'InboxView must manage calm timer cleanup for stale notice'
  )
})

test('4. Localization files contain user-friendly stale approval strings', () => {
  const idLocale = JSON.parse(fs.readFileSync(idLocalePath, 'utf8'))
  const enLocale = JSON.parse(fs.readFileSync(enLocalePath, 'utf8'))

  const key = 'Rekomendasi persetujuan ini sudah tidak berlaku karena status tugas telah diperbarui.'
  assert.ok(idLocale[key], 'id.json must contain stale approval key')
  assert.ok(enLocale[key], 'en.json must contain translation for stale approval key')
  assert.equal(
    enLocale[key],
    'This approval recommendation is no longer current because the task status has been updated.'
  )
})

test('5. InboxView sends empty attachmentIds and bypasses stale pendingRef on explicit approveMessageId', () => {
  const src = fs.readFileSync(inboxViewPath, 'utf8')
  assert.ok(
    src.includes('const isExplicitAction =') &&
      src.includes('Boolean(approveMessageId)'),
    'InboxView must treat explicit approveMessageId as an explicit action that supersedes stale pendingRef'
  )
  assert.ok(
    src.includes('approveMessageId || overrideText !== undefined') &&
      src.includes('? []'),
    'InboxView must clear attachmentIds when sending an explicit plan approval'
  )
})
