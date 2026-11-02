import test from 'node:test'
import assert from 'node:assert/strict'
import fs from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'

const __filename = fileURLToPath(import.meta.url)
const __dirname = path.dirname(__filename)
const webDir = path.resolve(__dirname, '..')

const taskCardPath = path.resolve(webDir, 'src/features/inbox/TaskCard.tsx')
const chatMarkdownPath = path.resolve(webDir, 'src/features/inbox/ChatMarkdown.tsx')
const inboxViewPath = path.resolve(webDir, 'src/features/inbox/InboxView.tsx')
const roomCardPath = path.resolve(webDir, 'src/features/inbox/RoomCard.tsx')
const idLocalePath = path.resolve(webDir, 'src/locales/id.json')
const enLocalePath = path.resolve(webDir, 'src/locales/en.json')

const taskCardSrc = fs.readFileSync(taskCardPath, 'utf8')
const chatMarkdownSrc = fs.readFileSync(chatMarkdownPath, 'utf8')
const inboxViewSrc = fs.readFileSync(inboxViewPath, 'utf8')
const roomCardSrc = fs.readFileSync(roomCardPath, 'utf8')
const idLocale = JSON.parse(fs.readFileSync(idLocalePath, 'utf8'))
const enLocale = JSON.parse(fs.readFileSync(enLocalePath, 'utf8'))

test('1. TaskCard: in-memory task caching & elimination of skeleton flicker', () => {
  // Must export cacheTask, cacheTasks, getCachedTask
  assert.ok(
    taskCardSrc.includes('export function cacheTask'),
    'TaskCard must export cacheTask'
  )
  assert.ok(
    taskCardSrc.includes('export function cacheTasks'),
    'TaskCard must export cacheTasks'
  )
  assert.ok(
    taskCardSrc.includes('export function getCachedTask'),
    'TaskCard must export getCachedTask'
  )

  // Must initialize state using cached task
  assert.ok(
    taskCardSrc.includes('initialTask || getCachedTask(taskId)'),
    'TaskCard must initialize currentTask with initialTask or getCachedTask'
  )
  assert.ok(
    taskCardSrc.includes('!cached') || taskCardSrc.includes('!initialTask && !getCachedTask(taskId)'),
    'TaskCard must set loading to false when task is already cached'
  )

  // Polling and background fetch must save to cache
  assert.ok(
    taskCardSrc.includes('cacheTask(updated)'),
    'TaskCard polling must persist updated task to cache'
  )
})

test('2. ChatMarkdown: stable memoized components to prevent unmounting and jumping', () => {
  assert.ok(
    chatMarkdownSrc.includes('useMemo'),
    'ChatMarkdown must import useMemo'
  )
  assert.ok(
    chatMarkdownSrc.includes('const components = useMemo'),
    'ChatMarkdown components must be memoized with useMemo'
  )
  assert.ok(
    chatMarkdownSrc.includes('components={components}'),
    'ReactMarkdown must receive memoized components prop'
  )
  assert.ok(
    chatMarkdownSrc.includes('<TaskCard taskId='),
    'ChatMarkdown must render TaskCard for task links'
  )
})

test('3. TaskCard: Calm Things 3 loading placeholder (Zero Cognitive Fatigue)', () => {
  // Loading state must retain data-card="task-card"
  assert.ok(
    taskCardSrc.includes('data-card="task-card"'),
    'TaskCard must retain data-card="task-card"'
  )
  // Loading state must display taskRef and calm loading label
  assert.ok(
    taskCardSrc.includes('taskRef(taskId)'),
    'Loading placeholder must display taskRef'
  )
  assert.ok(
    taskCardSrc.includes("t('Memuat tugas…')") || taskCardSrc.includes("t('Loading…')"),
    'Loading placeholder must use calm localized loading text'
  )
})

test('4. InboxView: early task caching and message list thrashing elimination', () => {
  // Must import cacheTasks
  assert.ok(
    inboxViewSrc.includes('cacheTasks'),
    'InboxView must import cacheTasks'
  )
  // Must cache tasks on loadConversation and streamConversation
  assert.ok(
    inboxViewSrc.includes('if (data.tasks) cacheTasks(data.tasks)') ||
    inboxViewSrc.includes('cacheTasks(data.tasks)'),
    'InboxView must cache tasks into memory immediately on arrival'
  )
  // Stable task action callbacks
  assert.ok(
    inboxViewSrc.includes('handleTaskDetailsClick'),
    'InboxView must provide memoized handleTaskDetailsClick'
  )
  assert.ok(
    inboxViewSrc.includes('handleTaskTerminalClick'),
    'InboxView must provide memoized handleTaskTerminalClick'
  )
})

test('5. RoomCard: in-memory caching and calm loading state', () => {
  assert.ok(
    roomCardSrc.includes('export function cacheRoom'),
    'RoomCard must export cacheRoom'
  )
  assert.ok(
    roomCardSrc.includes('export function getCachedRoom'),
    'RoomCard must export getCachedRoom'
  )
  assert.ok(
    roomCardSrc.includes('initialContact || getCachedRoom(roomId)'),
    'RoomCard must initialize state with cached contact'
  )
})

test('6. Localization completeness for calm loading indicators', () => {
  assert.equal(idLocale['Memuat tugas…'], 'Memuat tugas…')
  assert.equal(enLocale['Memuat tugas…'], 'Loading task…')
  assert.equal(idLocale['Loading task…'], 'Memuat tugas…')
  assert.equal(enLocale['Loading task…'], 'Loading task…')
})
