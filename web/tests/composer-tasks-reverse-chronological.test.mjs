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
  // Must sort b.created_at - a.created_at
  assert.ok(
    composerTasksSrc.includes('Date.parse(b.created_at)') && composerTasksSrc.includes('Date.parse(a.created_at)'),
    'ComposerTasks must sort tasks using Date.parse in descending order'
  )
  assert.ok(
    composerTasksSrc.includes('(Date.parse(b.created_at) || 0) - (Date.parse(a.created_at) || 0)'),
    'ComposerTasks must order tasks reverse-chronologically so newest task is at index 0'
  )
})

test('2. Header Redundancy Elimination: trigger is hidden when open, latest task is top timeline item', () => {
  assert.ok(
    composerTasksSrc.includes('open && "sr-only"'),
    'Collapsed trigger button must be hidden with sr-only when open to eliminate duplicate header'
  )
  assert.ok(
    composerTasksSrc.includes('data-testid="composer-tasks-collapse"'),
    'Top task in expanded timeline must provide collapse toggle control'
  )
})

test('3. Default expanded state: latest task is expanded, older tasks collapsed', () => {
  assert.ok(
    composerTasksSrc.includes('const latestTask = ordered[0]'),
    'latestTask must be identified as index 0 of the reverse-chronological list'
  )
  assert.ok(
    composerTasksSrc.includes('const defaultExpandedIds = latestTask ? [latestTask.id] : []'),
    'defaultExpandedIds must expand only the latest task by default'
  )
})

test('4. Timeline connector and status dot geometry', () => {
  assert.ok(
    composerTasksSrc.includes('<AccordionTimeline'),
    'ComposerTasks must render AccordionTimeline for connected timeline dots'
  )
  assert.ok(
    composerTasksSrc.includes('defaultExpandedIds={defaultExpandedIds}'),
    'ComposerTasks must pass defaultExpandedIds to AccordionTimeline'
  )
})

test('5. Touch accessibility and responsive targets (WCAG AA 44px min-h-11 on mobile, 32px on desktop)', () => {
  assert.ok(
    composerTasksSrc.includes('min-h-11 sm:min-h-8'),
    'Trigger must satisfy 44px touch target on mobile and 32px on desktop'
  )
  assert.ok(
    composerTasksSrc.includes('min-h-11 min-w-11 sm:min-h-8 sm:min-w-8'),
    'Collapse button must satisfy 44px touch target on mobile and 32px on desktop'
  )
})

test('6. Localization: Collapse tasks label exists in both en and id', () => {
  assert.equal(enJson['Collapse tasks'], 'Collapse tasks')
  assert.equal(idJson['Collapse tasks'], 'Tutup tugas')
})

test('7. Pinned toggle & Zero Layout Shift: toggle is pinned to top-right of top task without shifting', () => {
  const accordionTimelinePath = path.resolve(__dirname, '../src/components/ui/AccordionTimeline.tsx')
  const accordionTimelineSrc = fs.readFileSync(accordionTimelinePath, 'utf8')
  assert.ok(
    composerTasksSrc.includes('pinnedAction: isLatest ?'),
    'ComposerTasks must pass pinnedAction for the latest top task'
  )
  assert.ok(
    accordionTimelineSrc.includes('item.pinnedAction'),
    'AccordionTimeline must support item.pinnedAction'
  )
  assert.ok(
    accordionTimelineSrc.includes('absolute right-0 top-0 flex h-11 sm:h-8 items-center z-10'),
    'AccordionTimeline must pin pinnedAction to top-right aligned with first line of task'
  )
  assert.ok(
    accordionTimelineSrc.includes("item.pinnedAction && 'pr-20 sm:pr-16'") ||
    accordionTimelineSrc.includes('item.pinnedAction && "pr-20 sm:pr-16"'),
    'AccordionTimeline must provide right padding for wrapping text so it does not collide with pinned toggle'
  )
})

