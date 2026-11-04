import test from 'node:test'
import assert from 'node:assert'
import fs from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'

const __filename = fileURLToPath(import.meta.url)
const __dirname = path.dirname(__filename)

const subPanelPath = path.join(__dirname, '../src/features/inbox/InboxSubPanel.tsx')
const chatApiPath = path.join(__dirname, '../src/api/chat.ts')
const inboxApiPath = path.join(__dirname, '../src/api/inbox.ts')
const inboxViewPath = path.join(__dirname, '../src/features/inbox/InboxView.tsx')

const subPanelSource = fs.readFileSync(subPanelPath, 'utf8')
const chatApiSource = fs.readFileSync(chatApiPath, 'utf8')
const inboxApiSource = fs.readFileSync(inboxApiPath, 'utf8')
const inboxViewSource = fs.readFileSync(inboxViewPath, 'utf8')

test('1. ChatContact & InboxThreadItem interfaces define task counters including queued, done and failed', () => {
  assert.ok(
    chatApiSource.includes('task_count?: number'),
    'ChatContact must define optional task_count'
  )
  assert.ok(
    chatApiSource.includes('active_tasks?: number'),
    'ChatContact must define optional active_tasks'
  )
  assert.ok(
    chatApiSource.includes('queued_tasks?: number'),
    'ChatContact must define optional queued_tasks'
  )
  assert.ok(
    chatApiSource.includes('done_tasks?: number'),
    'ChatContact must define optional done_tasks'
  )
  assert.ok(
    chatApiSource.includes('failed_tasks?: number'),
    'ChatContact must define optional failed_tasks'
  )
  assert.ok(
    inboxApiSource.includes('task_count: number'),
    'InboxThreadItem must define task_count'
  )
  assert.ok(
    inboxApiSource.includes('active_tasks?: number'),
    'InboxThreadItem must define optional active_tasks'
  )
  assert.ok(
    inboxApiSource.includes('review_tasks?: number'),
    'InboxThreadItem must define optional review_tasks'
  )
  assert.ok(
    inboxApiSource.includes('queued_tasks?: number'),
    'InboxThreadItem must define optional queued_tasks'
  )
  assert.ok(
    inboxApiSource.includes('done_tasks?: number'),
    'InboxThreadItem must define optional done_tasks'
  )
  assert.ok(
    inboxApiSource.includes('failed_tasks?: number'),
    'InboxThreadItem must define optional failed_tasks'
  )
})

test('2. S59-T6: InboxSubPanel eliminates 4-badge stack in favor of subtle StatusDot (Status-by-Exception)', () => {
  // Elimination of noisy multi-badge stack per thread row
  assert.ok(
    !subPanelSource.includes('data-testid="task-badge-queued"'),
    'InboxSubPanel must eliminate queued task badge in favor of single StatusDot'
  )
  assert.ok(
    !subPanelSource.includes('data-testid="task-badge-done"'),
    'InboxSubPanel must eliminate done task badge in favor of single StatusDot'
  )
  assert.ok(
    !subPanelSource.includes('data-testid="task-badge-failed"'),
    'InboxSubPanel must eliminate failed task badge in favor of single StatusDot'
  )

  // Subtle StatusDot micro-indicator for active work
  assert.ok(
    subPanelSource.includes('data-testid="task-count-badge"'),
    'InboxSubPanel must maintain task-count-badge container'
  )
  assert.ok(
    subPanelSource.includes('data-testid="task-badge-active"'),
    'InboxSubPanel must render active task indicator with testid task-badge-active'
  )
  assert.ok(
    subPanelSource.includes('<StatusDot status="working"'),
    'InboxSubPanel must render StatusDot with status="working" for active agent work'
  )
})

test('3. S59-T6: Active work micro-indicator retains accessible attributes', () => {
  assert.ok(
    subPanelSource.includes('aria-label={`${activeTasks} active tasks`}'),
    'Active task micro-indicator must include accessible aria-label'
  )
  assert.ok(
    subPanelSource.includes('pulse'),
    'Active StatusDot micro-indicator must pulse while agent is working'
  )

})

test('4. InboxView synchronizes activeContact task counts and triggers refresh on task creation', () => {
  assert.ok(
    inboxViewSource.includes('activeContact.task_count = total') && inboxViewSource.includes("count('task_count', data.tasks.length)") && inboxViewSource.includes('const summary = data.contact'),
    'InboxView must prefer server task counts, retaining a fallback for older snapshots'
  )
  assert.ok(
    inboxViewSource.includes('activeContact.review_tasks = rt'),
    'InboxView must sync review_tasks onto activeContact'
  )
  assert.ok(
    inboxViewSource.includes('activeContact.queued_tasks = qt'),
    'InboxView must sync queued_tasks onto activeContact'
  )
  assert.ok(
    inboxViewSource.includes('activeContact.done_tasks = dt'),
    'InboxView must sync done_tasks onto activeContact'
  )
  assert.ok(
    inboxViewSource.includes('activeContact.failed_tasks = ft'),
    'InboxView must sync failed_tasks onto activeContact'
  )
  assert.ok(
    inboxViewSource.includes('onContactUpdated?.()'),
    'InboxView must trigger onContactUpdated when tasks change'
  )
})
