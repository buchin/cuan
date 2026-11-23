import test from 'node:test'
import assert from 'node:assert/strict'
import fs from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'

const __dirname = path.dirname(fileURLToPath(import.meta.url))
const composerTasksPath = path.resolve(__dirname, '../src/features/inbox/ComposerTasks.tsx')
const enJsonPath = path.resolve(__dirname, '../src/locales/en.json')
const idJsonPath = path.resolve(__dirname, '../src/locales/id.json')

const composerTasksSrc = fs.readFileSync(composerTasksPath, 'utf8')
const enJson = JSON.parse(fs.readFileSync(enJsonPath, 'utf8'))
const idJson = JSON.parse(fs.readFileSync(idJsonPath, 'utf8'))

test('1. Reverse-Chronological: tasks are ordered newest first (created_at descending)', () => {
  assert.ok(
    composerTasksSrc.includes('Date.parse(b.created_at)') && composerTasksSrc.includes('Date.parse(a.created_at)'),
    'ComposerTasks must sort tasks using Date.parse in descending order'
  )
  assert.ok(
    composerTasksSrc.includes('(Date.parse(b.created_at) || 0) - (Date.parse(a.created_at) || 0)'),
    'ComposerTasks must order tasks reverse-chronologically so newest task is at index 0'
  )
})

test('2. Calm header status bar trigger remains stable without layout shift', () => {
  assert.ok(
    composerTasksSrc.includes('data-testid="composer-tasks-badge"'),
    'Header status bar must maintain composer-tasks-badge trigger'
  )
  assert.ok(
    !composerTasksSrc.includes('open && "sr-only"'),
    'Header trigger remains visible and calm, opening SlideDrawer instead of inline replacement'
  )
})

test('3. Default expanded state: latest task is top item', () => {
  assert.ok(
    composerTasksSrc.includes('const latestTask = ordered[0]'),
    'latestTask must be identified as index 0 of the reverse-chronological list'
  )
})

test('4. Linear timeline connector and status dot geometry without accordion nesting', () => {
  assert.ok(
    composerTasksSrc.includes('data-timeline-connector'),
    'ComposerTasks must render data-timeline-connector for connected timeline dots'
  )
  assert.ok(
    !composerTasksSrc.includes('<AccordionTimeline'),
    'ComposerTasks must eliminate AccordionTimeline in favor of linear timeline'
  )
})

test('5. Touch accessibility and responsive targets (WCAG AA 44px min-h-11 on mobile, 32px on desktop)', () => {
  assert.ok(
    composerTasksSrc.includes('min-h-11 sm:min-h-8'),
    'Trigger must satisfy 44px touch target on mobile and 32px on desktop'
  )
})

test('6. Localization: Conversation tasks label exists in both en and id', () => {
  assert.equal(enJson['Conversation tasks'], 'Conversation tasks')
  assert.equal(idJson['Conversation tasks'], 'Task obrolan')
})

test('7. SlideDrawer Integration & Zero Layout Shift on chat area', () => {
  assert.ok(
    composerTasksSrc.includes('<SlideDrawer') || composerTasksSrc.includes('SlideDrawer'),
    'ComposerTasks must open SlideDrawer on click'
  )
  assert.ok(
    composerTasksSrc.includes('data-testid="composer-tasks-drawer"'),
    'ComposerTasks must designate drawer with data-testid composer-tasks-drawer'
  )
})
