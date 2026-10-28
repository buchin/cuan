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

test('2. InboxSubPanel: Lucide icons imported for smart badges (Zap, Clock, XCircle, CheckSquare)', () => {
  assert.ok(
    subPanelSource.includes('CheckSquare'),
    'InboxSubPanel must import CheckSquare icon from lucide-react'
  )
  assert.ok(
    subPanelSource.includes('Zap'),
    'InboxSubPanel must import Zap icon from lucide-react'
  )
  assert.ok(
    subPanelSource.includes('Clock'),
    'InboxSubPanel must import Clock icon from lucide-react'
  )
  assert.ok(
    subPanelSource.includes('XCircle'),
    'InboxSubPanel must import XCircle icon from lucide-react'
  )
  assert.ok(
    subPanelSource.includes('data-testid="task-count-badge"'),
    'InboxSubPanel must maintain task-count-badge testid for backward compatibility'
  )
  assert.ok(
    subPanelSource.includes('data-testid="task-badge-active"'),
    'InboxSubPanel must render active task badge with testid task-badge-active'
  )
  assert.ok(
    subPanelSource.includes('data-testid="task-badge-queued"'),
    'InboxSubPanel must render queued task badge with testid task-badge-queued'
  )
  assert.ok(
    subPanelSource.includes('data-testid="task-badge-done"'),
    'InboxSubPanel must render done task badge with testid task-badge-done'
  )
  assert.ok(
    subPanelSource.includes('data-testid="task-badge-failed"'),
    'InboxSubPanel must render failed/blocked task badge with testid task-badge-failed'
  )
})

test('3. InboxSubPanel: Smart dynamic badges placed beside agent handle with accessible attributes', () => {
  assert.ok(
    subPanelSource.includes('aria-label={`${activeTasks} active tasks`}'),
    'Active task badge must include accessible aria-label with active task count'
  )
  assert.ok(
    subPanelSource.includes('aria-label={`${queuedTasks} queued tasks`}'),
    'Queued task badge must include accessible aria-label with queued count'
  )
  assert.ok(
    subPanelSource.includes('aria-label={`${doneTasks} completed tasks`}'),
    'Done task badge must include accessible aria-label with completed count'
  )
  assert.ok(
    subPanelSource.includes('tabular-nums'),
    'Task badge must use tabular-nums for neat numeric alignment'
  )
  assert.ok(
    subPanelSource.includes('bg-blue-500/15'),
    'Active badge must use blue accent color theme'
  )
  assert.ok(
    subPanelSource.includes('bg-amber-500/15'),
    'Queued badge must use amber accent color theme'
  )
  assert.ok(
    subPanelSource.includes('bg-emerald-500/15'),
    'Done badge must use emerald/green accent color theme'
  )
  assert.ok(
    subPanelSource.includes('bg-rose-500/15'),
    'Failed/blocked badge must use rose/red accent color theme'
  )
  assert.ok(
    subPanelSource.includes('showAgentName'),
    'Smart badges must render on the second line alongside agent name (@sm)'
  )
})

test('4. InboxView synchronizes activeContact task counts and triggers refresh on task creation', () => {
  assert.ok(
    inboxViewSource.includes('activeContact.task_count = data.tasks.length'),
    'InboxView must sync task_count onto activeContact when loading conversation'
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
