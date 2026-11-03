import test from 'node:test'
import assert from 'node:assert/strict'
import fs from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'

const __filename = fileURLToPath(import.meta.url)
const __dirname = path.dirname(__filename)
const webDir = path.resolve(__dirname, '..')

const taskEnvelopePath = path.resolve(webDir, 'src/features/inbox/task-envelope.ts')
const taskCardPath = path.resolve(webDir, 'src/features/inbox/TaskCard.tsx')
const taskCompletionCardPath = path.resolve(webDir, 'src/features/inbox/TaskCompletionCard.tsx')
const inboxViewPath = path.resolve(webDir, 'src/features/inbox/InboxView.tsx')
const inboxTaskActionPath = path.resolve(webDir, 'src/features/inbox/InboxTaskAction.tsx')
const messageContentPath = path.resolve(webDir, 'src/features/inbox/MessageContent.tsx')
const chatMarkdownPath = path.resolve(webDir, 'src/features/inbox/ChatMarkdown.tsx')

test('1. task-envelope: in-flight (queued / doing) tasks MUST NOT produce fallback visitor drafts', async () => {
  const { parseTaskReviewEnvelope } = await import('../src/features/inbox/task-envelope.ts')

  // Queued task without explicit envelope draft
  const queuedTask = {
    id: 'task_queued_1',
    title: 'Ekstraksi Library Publik: Agent Process & Workspace Runner',
    status: 'queued',
    summary: 'Waiting in worker queue...',
  }
  const resQueued = parseTaskReviewEnvelope(queuedTask)
  assert.equal(resQueued.draft, '', 'Queued task must have empty draft')

  // Doing / running task without explicit envelope draft
  const doingTask = {
    id: 'task_doing_2',
    title: 'Refactor internal runner execution engine',
    status: 'doing',
    summary: 'Sedang mengeksekusi di background...',
  }
  const resDoing = parseTaskReviewEnvelope(doingTask)
  assert.equal(resDoing.draft, '', 'Doing task must have empty draft')

  // Completed task still gets friendly visitor draft
  const doneTask = {
    id: 'task_done_3',
    title: 'Perbaikan pemutaran video',
    status: 'done',
    summary: 'Masalah pemutaran video telah diatasi.',
  }
  const resDone = parseTaskReviewEnvelope(doneTask)
  assert.ok(resDone.draft.length > 0, 'Done task should have fallback draft')
  assert.ok(resDone.draft.includes('Perbaikan pemutaran video'))
})

test('2. TaskCard & TaskCompletionCard: isCustomerRoom guard ensures drafts only appear in customer conversations', () => {
  const taskCardSrc = fs.readFileSync(taskCardPath, 'utf8')
  const taskCompletionCardSrc = fs.readFileSync(taskCompletionCardPath, 'utf8')

  // Both components must accept isCustomerRoom prop
  assert.ok(taskCardSrc.includes('isCustomerRoom'), 'TaskCard must accept isCustomerRoom prop')
  assert.ok(taskCompletionCardSrc.includes('isCustomerRoom'), 'TaskCompletionCard must accept isCustomerRoom prop')

  // Draft section MUST be conditioned on isCustomerRoom
  assert.ok(
    taskCardSrc.includes('isCustomerRoom && envelopeResult.draft'),
    'TaskCard must guard draft section with isCustomerRoom'
  )
  assert.ok(
    taskCompletionCardSrc.includes('isCustomerRoom && envelopeResult.draft'),
    'TaskCompletionCard must guard draft section with isCustomerRoom'
  )
})

test('3. Card Diet & Low-Noise: elimination of 3-level nested cards and noisy microcopy', () => {
  const taskCardSrc = fs.readFileSync(taskCardPath, 'utf8')
  const taskCompletionCardSrc = fs.readFileSync(taskCompletionCardPath, 'utf8')

  // Must not have nested card border box inside draft section
  assert.ok(
    !taskCardSrc.includes('className="rounded-xl border border-border/40 bg-muted/30'),
    'TaskCard must eliminate nested box wrapper inside card'
  )
  assert.ok(
    !taskCompletionCardSrc.includes('className="rounded-xl border border-border/40 bg-muted/30'),
    'TaskCompletionCard must eliminate nested box wrapper inside card'
  )

  // Must eliminate nested background quote box
  assert.ok(
    !taskCardSrc.includes('className="text-xs text-foreground/90 italic bg-background/50'),
    'TaskCard must eliminate nested background quote box'
  )
  assert.ok(
    !taskCompletionCardSrc.includes('className="text-xs text-foreground/90 italic bg-background/50'),
    'TaskCompletionCard must eliminate nested background quote box'
  )

  // Must eliminate SaaS explanation microcopy noise
  assert.ok(
    !taskCardSrc.includes("1-klik mengisi composer obrolan dan fokus kursor otomatis."),
    'TaskCard must eliminate 1-click focus microcopy noise'
  )
  assert.ok(
    !taskCompletionCardSrc.includes("1-klik mengisi composer obrolan dan fokus kursor otomatis."),
    'TaskCompletionCard must eliminate 1-click focus microcopy noise'
  )

  // Must use Bear Notes clean accent styling
  assert.ok(
    taskCardSrc.includes('border-l-2 border-primary/40 pl-3'),
    'TaskCard must use clean Bear Notes left border accent'
  )
  assert.ok(
    taskCompletionCardSrc.includes('border-l-2 border-primary/40 pl-3'),
    'TaskCompletionCard must use clean Bear Notes left border accent'
  )
})

test('4. Flow propagation: InboxView, InboxTaskAction, MessageContent, ChatMarkdown propagate isCustomerRoom', () => {
  const inboxViewSrc = fs.readFileSync(inboxViewPath, 'utf8')
  const inboxTaskActionSrc = fs.readFileSync(inboxTaskActionPath, 'utf8')
  const messageContentSrc = fs.readFileSync(messageContentPath, 'utf8')
  const chatMarkdownSrc = fs.readFileSync(chatMarkdownPath, 'utf8')

  assert.ok(
    inboxViewSrc.includes("isCustomerRoom={activeContact.type === 'customer'}"),
    'InboxView must pass isCustomerRoom based on activeContact.type'
  )
  assert.ok(
    !inboxTaskActionSrc.includes('<TaskCard') && !inboxTaskActionSrc.includes('<TaskCompletionCard') && inboxTaskActionSrc.includes('onViewTask(existing)'),
    'Compact task references must expose navigation without direct execution controls'
  )
  assert.ok(
    messageContentSrc.includes('isCustomerRoom={isCustomerRoom}'),
    'MessageContent must forward isCustomerRoom to TaskCard'
  )
  assert.ok(
    chatMarkdownSrc.includes('isCustomerRoom={isCustomerRoom}'),
    'ChatMarkdown must forward isCustomerRoom to TaskCard'
  )
})
