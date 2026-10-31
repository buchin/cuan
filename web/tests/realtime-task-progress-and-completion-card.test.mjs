import test from 'node:test'
import assert from 'node:assert/strict'
import fs from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'

const __filename = fileURLToPath(import.meta.url)
const __dirname = path.dirname(__filename)

const inboxViewPath = path.resolve(__dirname, '..', 'src', 'features', 'inbox', 'InboxView.tsx')
const bannerPath = path.resolve(__dirname, '..', 'src', 'features', 'inbox', 'TaskProgressBanner.tsx')
const completionCardPath = path.resolve(__dirname, '..', 'src', 'features', 'inbox', 'TaskCompletionCard.tsx')
const inboxTaskActionPath = path.resolve(__dirname, '..', 'src', 'features', 'inbox', 'InboxTaskAction.tsx')
const idLocalePath = path.resolve(__dirname, '..', 'src', 'locales', 'id.json')
const enLocalePath = path.resolve(__dirname, '..', 'src', 'locales', 'en.json')

test('1. activeConversationTasks & isTaskActive selector logic', async () => {
  const bannerSource = fs.readFileSync(bannerPath, 'utf8')

  // Verify isTaskActive function exists and exports
  assert.ok(
    bannerSource.includes('isTaskActive') && bannerSource.includes('export { isTaskActive'),
    'isTaskActive must be exported by TaskProgressBanner'
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

test('2. Floating Live Progress Banner: UI components & accessibility standards', () => {
  const bannerSource = fs.readFileSync(bannerPath, 'utf8')

  // Verify floating container testid and accessibility attributes
  assert.ok(bannerSource.includes('data-testid="floating-task-progress-banner"'), 'Banner must have data-testid')
  assert.ok(bannerSource.includes('role="status"'), 'Banner status card must have role="status"')
  assert.ok(bannerSource.includes('aria-live="polite"'), 'Banner status card must have aria-live="polite"')

  // Verify non-intrusive floating classes: pointer-events-none outer, pointer-events-auto inner
  assert.ok(bannerSource.includes('pointer-events-none absolute inset-x-0 top-3 z-30'), 'Outer banner must be non-intrusive pointer-events-none at top-3')
  assert.ok(bannerSource.includes('pointer-events-auto'), 'Inner banner pill must be interactive pointer-events-auto')

  // Verify status badge, timer, agent, and action button testids
  assert.ok(bannerSource.includes('data-testid="live-progress-status-badge"'), 'Must have status badge testid')
  assert.ok(bannerSource.includes('data-testid="live-progress-title"'), 'Must have title testid')
  assert.ok(bannerSource.includes('data-testid="live-progress-agent"'), 'Must have agent testid')
  assert.ok(bannerSource.includes('data-testid="live-progress-timer"'), 'Must have live timer testid')
  assert.ok(bannerSource.includes('data-testid="view-task-details-btn"'), 'Must have view task details button testid')

  // Verify subtle pulse animation
  assert.ok(bannerSource.includes('animate-ping'), 'Must render pulse ping indicator')
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
  assert.ok(cardSource.includes('border-emerald-500'), 'Must style done state with emerald')
  assert.ok(cardSource.includes('border-purple-500'), 'Must style review state with purple')

  // Verify DoD checklist support
  assert.ok(cardSource.includes('dod_checklist'), 'Must inspect dod_checklist')
  assert.ok(cardSource.includes("t('Kriteria Selesai (DoD)')"), 'Must display localized DoD section title')
})

test('4. InboxView Integration: reactive activeConversationTasks, adaptive poller & stream placement', () => {
  const inboxViewSource = fs.readFileSync(inboxViewPath, 'utf8')

  // Verify TaskProgressBanner import
  assert.ok(
    inboxViewSource.includes("import { TaskProgressBanner, isTaskActive } from './TaskProgressBanner'"),
    'InboxView must import TaskProgressBanner and isTaskActive'
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

  // Verify TaskProgressBanner rendering
  assert.ok(
    inboxViewSource.includes('<TaskProgressBanner'),
    'InboxView must render TaskProgressBanner'
  )
  assert.ok(
    inboxViewSource.includes('tasks={activeConversationTasks}'),
    'TaskProgressBanner must receive activeConversationTasks'
  )

  // Verify standalone completed tasks in stream
  assert.ok(
    inboxViewSource.includes('unattachedCompletedTasks.map'),
    'InboxView must render unattachedCompletedTasks in the message stream'
  )
})

test('5. InboxTaskAction Integration: renders TaskCompletionCard when done or review', () => {
  const actionSource = fs.readFileSync(inboxTaskActionPath, 'utf8')

  // Verify TaskCompletionCard import
  assert.ok(
    actionSource.includes("import { TaskCompletionCard } from './TaskCompletionCard'"),
    'InboxTaskAction must import TaskCompletionCard'
  )

  // Verify completion card rendering condition
  assert.ok(
    actionSource.includes("existing.status || '').toLowerCase().trim() === 'done' ||"),
    'InboxTaskAction must check for done status'
  )
  assert.ok(
    actionSource.includes("existing.status || '').toLowerCase().trim() === 'review'"),
    'InboxTaskAction must check for review status'
  )
  assert.ok(
    actionSource.includes('<TaskCompletionCard'),
    'InboxTaskAction must render TaskCompletionCard when task is finished/review'
  )
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
