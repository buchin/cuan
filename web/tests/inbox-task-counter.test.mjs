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

test('1. ChatContact & InboxThreadItem interfaces define task_count and active_tasks', () => {
  assert.ok(
    chatApiSource.includes('task_count?: number'),
    'ChatContact must define optional task_count'
  )
  assert.ok(
    chatApiSource.includes('active_tasks?: number'),
    'ChatContact must define optional active_tasks'
  )
  assert.ok(
    inboxApiSource.includes('task_count: number'),
    'InboxThreadItem must define task_count'
  )
  assert.ok(
    inboxApiSource.includes('active_tasks?: number'),
    'InboxThreadItem must define optional active_tasks'
  )
})

test('2. InboxSubPanel: CheckSquare icon imported and task counter badge rendered', () => {
  assert.ok(
    subPanelSource.includes('CheckSquare'),
    'InboxSubPanel must import CheckSquare icon from lucide-react'
  )
  assert.ok(
    subPanelSource.includes('data-testid="task-count-badge"'),
    'InboxSubPanel must render task count badge with testid task-count-badge'
  )
  assert.ok(
    subPanelSource.includes('taskBadgeCount'),
    'InboxSubPanel must compute taskBadgeCount based on active_tasks or task_count'
  )
  assert.ok(
    subPanelSource.includes('CheckSquare className="h-2.5 w-2.5"'),
    'InboxSubPanel must render CheckSquare icon inside badge'
  )
})

test('3. InboxSubPanel: Accessible attributes and badge hierarchy', () => {
  assert.ok(
    subPanelSource.includes('aria-label={`${taskBadgeCount} active tasks`}'),
    'Task badge must include accessible aria-label with count'
  )
  assert.ok(
    subPanelSource.includes('tabular-nums'),
    'Task badge must use tabular-nums for neat numeric alignment'
  )
  assert.ok(
    subPanelSource.includes('bg-blue-500/15'),
    'Task badge must use consistent active task accent color theme'
  )
})

test('4. InboxView synchronizes activeContact task counts and triggers refresh on task creation', () => {
  assert.ok(
    inboxViewSource.includes('activeContact.task_count = data.tasks.length'),
    'InboxView must sync task_count onto activeContact when loading conversation'
  )
  assert.ok(
    inboxViewSource.includes('onContactUpdated?.()'),
    'InboxView must trigger onContactUpdated when a new task is created'
  )
})
