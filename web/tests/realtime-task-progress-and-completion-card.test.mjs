import test from 'node:test'
import assert from 'node:assert/strict'
import fs from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'

const __filename = fileURLToPath(import.meta.url)
const __dirname = path.dirname(__filename)

const inboxViewPath = path.resolve(__dirname, '..', 'src', 'features', 'inbox', 'InboxView.tsx')
const taskCardPath = path.resolve(__dirname, '..', 'src', 'features', 'inbox', 'TaskCard.tsx')
const taskProgressPath = path.resolve(__dirname, '..', 'src', 'features', 'inbox', 'task-progress.ts')
const completionCardPath = path.resolve(__dirname, '..', 'src', 'features', 'inbox', 'TaskCompletionCard.tsx')
const inboxTaskActionPath = path.resolve(__dirname, '..', 'src', 'features', 'inbox', 'InboxTaskAction.tsx')
const idLocalePath = path.resolve(__dirname, '..', 'src', 'locales', 'id.json')
const enLocalePath = path.resolve(__dirname, '..', 'src', 'locales', 'en.json')

test('1. activeConversationTasks & isTaskActive selector logic', async () => {
  const taskProgressSource = fs.readFileSync(taskProgressPath, 'utf8')

  // Verify isTaskActive function exists and exports directly from task-progress
  assert.ok(
    taskProgressSource.includes('export function isTaskActive'),
    'isTaskActive must be exported by task-progress.ts'
  )

  // Import formatDuration and test formatting directly
  const { isTaskActive, formatDuration } = await import('../src/features/inbox/task-progress.ts')

  // Test isTaskActive cases
  assert.equal(isTaskActive(null), false, 'null task should be false')
  assert.equal(isTaskActive({ id: '1', status: 'queued' }), true, 'queued task should be active')
  assert.equal(isTaskActive({ id: '2', status: 'doing' }), true, 'doing task should be active')
  assert.equal(isTaskActive({ id: '3', status: 'todo', latest_attempt: { status: 'running' } }), true, 'running attempt should be active')
  assert.equal(isTaskActive({ id: '4', status: 'todo', latest_attempt: { status: 'queued' } }), true, 'queued attempt should be active')
  assert.equal(isTaskActive({ id: '5', status: 'todo', launch: { state: 'started' } }), true, 'started launch should be active')
  assert.equal(isTaskActive({ id: '6', status: 'todo', launch: { state: 'claimed' } }), true, 'claimed launch should be active')
  assert.equal(isTaskActive({ id: '7', status: 'done' }), false, 'done task should NOT be active')
  assert.equal(isTaskActive({ id: '8', status: 'review' }), false, 'review task should NOT be in-flight active')
  assert.equal(isTaskActive({ id: '9', status: 'blocked' }), false, 'blocked task should NOT be active')
  assert.equal(isTaskActive({ id: '10', status: 'failed' }), false, 'failed task should NOT be active')

  // Test formatDuration cases
  assert.equal(formatDuration(0), '00:00')
  assert.equal(formatDuration(45), '00:45')
  assert.equal(formatDuration(125), '02:05')
  assert.equal(formatDuration(3665), '1:01:05')
})

test('2. Self-Contained TaskCard: handles queued, running pulse & duration timer, and completion card transition', () => {
  const taskCardSource = fs.readFileSync(taskCardPath, 'utf8')

  // Verify TaskCard identifier and mobile-safe touch targets
  assert.ok(taskCardSource.includes('data-card="task-card"'), 'TaskCard must have data-card="task-card"')

  // Only active execution pulses; queued work must not imply activity.
  assert.ok(taskCardSource.includes('animate-ping'), 'TaskCard must render animate-ping indicator when working')
  assert.ok(!taskCardSource.includes('animate-pulse'), 'Queued TaskCard must remain static')

  // Verify live duration timer with Clock icon and testid
  assert.ok(taskCardSource.includes('data-testid="task-card-live-timer"'), 'TaskCard must render live timer with data-testid')
  assert.ok(taskCardSource.includes('formatDuration'), 'TaskCard must format duration using formatDuration')

  // Verify seamless transition to TaskCompletionCard
  assert.ok(taskCardSource.includes('<TaskCompletionCard'), 'TaskCard must render TaskCompletionCard when finished/review')
  assert.ok(taskCardSource.includes("import { TaskCompletionCard } from './TaskCompletionCard'"), 'TaskCard must import TaskCompletionCard')

  // Verify autonomous polling when active
  assert.ok(taskCardSource.includes('isTaskActive(currentTask)'), 'TaskCard must autonomously check active state')
})

test('3. Actionable Completion Card: business-friendly summary, DoD, and quick actions', () => {
  const cardSource = fs.readFileSync(completionCardPath, 'utf8')

  // Verify completion card testids
  assert.ok(cardSource.includes('data-testid="task-completion-card"'), 'Must have data-testid="task-completion-card"')
  assert.ok(cardSource.includes('data-testid="completion-card-status"'), 'Must have completion-card-status testid')
  assert.ok(cardSource.includes('data-testid="completion-card-title"'), 'Must have completion-card-title testid')
  assert.ok(cardSource.includes('data-testid="completion-card-summary"'), 'Must have completion-card-summary testid')
  assert.ok(cardSource.includes('data-testid="completion-card-chat-btn"'), 'Must have continue chat button testid')
  assert.ok(cardSource.includes('data-testid="completion-card-action-btn"'), 'Must have task action button testid')

  // Verify support for both done (emerald) and review (purple)
  assert.ok(cardSource.includes('border-border/40'), 'Status must use calm shared borders')
  assert.ok(cardSource.includes('stopped ? t(stateLabel)'), 'Stopped work must not claim review-ready success')

  // Verify DoD checklist support
  assert.ok(cardSource.includes('dod_checklist'), 'Must inspect dod_checklist')
  assert.ok(cardSource.includes("t('Kriteria Selesai (DoD)')"), 'Must display localized DoD section title')
})

test('4. InboxView Integration: clean chat layout (no redundant TaskProgressBanner, no pt-14 shift), adaptive poller & stream placement', () => {
  const inboxViewSource = fs.readFileSync(inboxViewPath, 'utf8')

  // Verify TaskProgressBanner is NOT imported and NOT rendered
  assert.ok(
    !inboxViewSource.includes('TaskProgressBanner'),
    'InboxView must NOT import or render TaskProgressBanner'
  )

  // Verify artificial pt-14 padding compensation is eliminated
  assert.ok(
    !inboxViewSource.includes('pt-14'),
    'InboxView must NOT contain artificial pt-14 padding compensation'
  )

  // Verify isTaskActive import directly from task-progress
  assert.ok(
    inboxViewSource.includes("import { isTaskActive } from './task-progress'"),
    'InboxView must import isTaskActive from task-progress'
  )

  // Verify TaskCompletionCard import
  assert.ok(
    inboxViewSource.includes("import { TaskCompletionCard } from './TaskCompletionCard'"),
    'InboxView must import TaskCompletionCard'
  )

  // Verify activeConversationTasks selector memoization
  assert.ok(
    inboxViewSource.includes('const activeConversationTasks = useMemo(() => {'),
    'InboxView must define activeConversationTasks via useMemo'
  )

  // Verify Adaptive Poller: 3-5s (4000ms) when active vs 20000ms when idle
  assert.ok(
    inboxViewSource.includes('const intervalMs = hasActiveTasks ? 4000 : 20000'),
    'Adaptive poller must use 4000ms for in-flight tasks and 20000ms for idle backoff'
  )

  // Verify standalone completed tasks in stream
  assert.ok(
    inboxViewSource.includes('unattachedCompletedTasks.map'),
    'InboxView must render unattachedCompletedTasks in the message stream'
  )
})

test('5. Inbox task references show current status and open detail without nested completion cards', () => {
  const source=fs.readFileSync(inboxTaskActionPath,'utf8')
  assert.ok(source.includes('workStatus(existing)'), 'Reference must show current task state')
  assert.ok(source.includes('onClick={() => onViewTask(existing)}'), 'Reference opens the existing task detail')
  assert.ok(!source.includes('<TaskCompletionCard') && !source.includes('<TaskCard'), 'Speech references must not embed full work-management cards')
})

test('6. Localization: id.json and en.json contain all Task Progress and Completion Card strings', () => {
  const idLocale = JSON.parse(fs.readFileSync(idLocalePath, 'utf8'))
  const enLocale = JSON.parse(fs.readFileSync(enLocalePath, 'utf8'))

  const requiredKeys = [
    'Dalam Antrean',
    'Sedang Dikerjakan',
    'Lihat Detail',
    'Lihat Detail Tugas',
    'tugas lainnya',
    'Tugas Selesai',
    'Siap Direviu',
    'Ringkasan Hasil',
    'Kriteria Selesai (DoD)',
    'Lanjutkan Diskusi',
    'Buka Detail Tugas',
  ]

  for (const key of requiredKeys) {
    assert.ok(idLocale[key], `id.json must contain translation key "${key}"`)
    assert.ok(enLocale[key], `en.json must contain translation key "${key}"`)
  }
})
