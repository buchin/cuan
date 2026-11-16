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

test('2. S59-T6: InboxSubPanel eliminates noisy badge stack; Things 3 dynamic indicator in right slot', () => {
  // Elimination of noisy multi-badge stack per thread row
  assert.ok(
    !subPanelSource.includes('data-testid="task-badge-queued"'),
    'InboxSubPanel must eliminate queued task badge'
  )
  assert.ok(
    !subPanelSource.includes('data-testid="task-badge-done"'),
    'InboxSubPanel must eliminate done task badge'
  )
  assert.ok(
    !subPanelSource.includes('data-testid="task-badge-failed"'),
    'InboxSubPanel must eliminate failed task badge'
  )
  assert.ok(
    !subPanelSource.includes('data-testid="task-badge-active"'),
    'InboxSubPanel eliminates bottom task badge under message snippet for airy layout'
  )

  // Clean right slot: no TaskProgress or pulsating badge in right slot; unified in left indicator
  assert.ok(
    !subPanelSource.includes('absolute right-0 top-0'),
    'InboxSubPanel must not render task indicator in right slot'
  )
  assert.ok(
    subPanelSource.includes('data-testid="workflow-status-indicator"'),
    'InboxSubPanel integrates task progress and workflow indicator in single left indicator'
  )
  assert.ok(
    subPanelSource.includes('taskProgressModel(progress)'),
    'InboxSubPanel computes task progress model'
  )
})

test('3. S59-T6: Right slot task progress indicator retains accessible attributes and subtle pulse', () => {
  const taskProgressPath = path.join(__dirname, '../src/components/ui/TaskProgress.tsx')
  const taskProgressSource = fs.readFileSync(taskProgressPath, 'utf8')
  assert.ok(
    taskProgressSource.includes('aria-label={label}'),
    'TaskProgress indicator must include accessible aria-label'
  )
  assert.ok(
    taskProgressSource.includes('pulse'),
    'TaskProgress indicator must pulse while task is active'
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
