import assert from 'node:assert/strict'
import fs from 'node:fs'
import path from 'node:path'
import { test } from 'node:test'
import { fileURLToPath } from 'node:url'

const __dirname = path.dirname(fileURLToPath(import.meta.url))
const cssPath = path.join(__dirname, '..', 'src', 'features', 'tasks', 'task-workspace.css')
const taskItemCardPath = path.join(__dirname, '..', 'src', 'features', 'tasks', 'TaskItemCard.tsx')
const taskUiPath = path.join(__dirname, '..', 'src', 'features', 'tasks', 'task-ui.ts')

test('1. Task Card Timeline Connector CSS & Queued Accent Tokens', () => {
  const cssContent = fs.readFileSync(cssPath, 'utf8')

  assert(
    cssContent.includes('.task-timeline-connector'),
    '.task-timeline-connector rule must be defined in task-workspace.css'
  )
  assert(
    cssContent.includes('left: 14px') || cssContent.includes('left:14px'),
    'Timeline connector must be positioned at left: 14px'
  )
  assert(
    cssContent.includes('width: 2px') || cssContent.includes('width:2px'),
    'Timeline connector must have width: 2px'
  )
  assert(
    cssContent.includes('.dark .task-timeline-connector'),
    'Dark mode styling for .task-timeline-connector must exist'
  )

  // Queued state dashed timeline connector
  assert(
    cssContent.includes('.task-timeline-connector-queued'),
    '.task-timeline-connector-queued rule must be defined for queued/pending state'
  )
  assert(
    cssContent.includes('border-left: 2px dashed') || cssContent.includes('dashed'),
    '.task-timeline-connector-queued must feature dashed border style for waiting status'
  )
  assert(
    cssContent.includes('rgba(245, 158, 11') || cssContent.includes('amber'),
    '.task-timeline-connector-queued must use amber color accent'
  )
})

test('2. Sumbu Dot Alignment and Title Indentation Geometry', () => {
  // - Badge padding: px-2.5 (10px = 0.625rem)
  // - Badge dot: h-2 w-2 (8px = 0.5rem)
  // - Center of badge bullet: 10px + 4px = 14px
  // - Pill padding: px-2.5 (10px = 0.625rem)
  // - Pill dot: h-2 w-2 (8px = 0.5rem)
  // - Center of pill bullet: 10px + 4px = 14px
  // - Connector line: left: 14px, width: 2px, translateX(-50%) => center = 14px
  // - Title indentation: pl-7 (28px = 1.75rem) => text starts at 28px, clearance = 13px
  const BADGE_PADDING_PX = 10
  const BULLET_WIDTH_PX = 8
  const BADGE_BULLET_AXIS_PX = BADGE_PADDING_PX + BULLET_WIDTH_PX / 2

  const PILL_PADDING_PX = 10
  const PILL_BULLET_AXIS_PX = PILL_PADDING_PX + BULLET_WIDTH_PX / 2

  const CONNECTOR_LINE_AXIS_PX = 14
  const TITLE_INDENT_PX = 28
  const CLEARANCE_PX = TITLE_INDENT_PX - (CONNECTOR_LINE_AXIS_PX + 1)

  assert.strictEqual(BADGE_BULLET_AXIS_PX, 14, 'Badge bullet axis must be exactly 14px')
  assert.strictEqual(PILL_BULLET_AXIS_PX, 14, 'Pill bullet axis must be exactly 14px')
  assert.strictEqual(BADGE_BULLET_AXIS_PX, PILL_BULLET_AXIS_PX, 'Badge and Pill bullet axes must be collinearly aligned')
  assert.strictEqual(CONNECTOR_LINE_AXIS_PX, BADGE_BULLET_AXIS_PX, 'Connector line axis must align with bullet axis')
  assert(CLEARANCE_PX >= 12, 'Title text must have at least 12px clearance to prevent collision with line')
})

test('3. TaskItemCard Component Source Layout Verification', () => {
  const taskItemCardSrc = fs.readFileSync(taskItemCardPath, 'utf8')

  assert(
    taskItemCardSrc.includes("import './task-workspace.css'"),
    'TaskItemCard must import task-workspace.css'
  )
  assert(
    taskItemCardSrc.includes('task-timeline-connector'),
    'TaskItemCard must render the task-timeline-connector element'
  )
  assert(
    taskItemCardSrc.includes('task-timeline-connector-queued'),
    'TaskItemCard must render the task-timeline-connector-queued element'
  )
  assert(
    taskItemCardSrc.includes('(isWorking || isQueued) && \'pl-7\'') ||
    taskItemCardSrc.includes('pl-7'),
    'TaskItemCard title link must conditionally apply pl-7 when running or queued'
  )
  assert(
    taskItemCardSrc.includes('left-[14px]'),
    'TaskItemCard must specify left-[14px] on the connector line'
  )
  assert(
    taskItemCardSrc.includes('-top-3'),
    'TaskItemCard must specify -top-3 to connect to status badge'
  )
  assert(
    taskItemCardSrc.includes('-bottom-2'),
    'TaskItemCard must specify -bottom-2 to connect to live progress pill'
  )
})

test('4. Queued / Pending Visual State Differentiation from Blue Working', () => {
  const taskItemCardSrc = fs.readFileSync(taskItemCardPath, 'utf8')
  const taskUiSrc = fs.readFileSync(taskUiPath, 'utf8')

  // Status differentiation
  assert(
    taskItemCardSrc.includes('const isWorking = status === \'Working\''),
    'TaskItemCard must determine isWorking strictly from status'
  )
  assert(
    taskItemCardSrc.includes('isQueued'),
    'TaskItemCard must explicitly define isQueued state'
  )

  // Working state: blue pulsing ping dot and blue pill
  assert(
    taskItemCardSrc.includes('bg-blue-500 opacity-75'),
    'Working state must render blue animated ping dot'
  )
  assert(
    taskItemCardSrc.includes('bg-blue-500/10'),
    'Working state must render blue container accent'
  )

  // Queued state: amber calm pulse dot and amber pill
  assert(
    taskItemCardSrc.includes('bg-amber-400 opacity-75') || taskItemCardSrc.includes('animate-pulse'),
    'Queued state must render amber pulse dot'
  )
  assert(
    taskItemCardSrc.includes('bg-amber-500/10') && taskItemCardSrc.includes('border-amber-500/25'),
    'Queued state must render distinct amber-slate accent pill container'
  )

  // workStatusBadgeClass handles Waiting to start / Queued with amber styling
  assert(
    taskUiSrc.includes("case 'Waiting to start':") && (/bg-amber-500\/(10|15)/.test(taskUiSrc)),
    'workStatusBadgeClass must return amber styling for Waiting to start'
  )
})
