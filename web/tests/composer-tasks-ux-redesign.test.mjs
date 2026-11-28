import test from 'node:test'
import assert from 'node:assert/strict'
import fs from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'

const __dirname = path.dirname(fileURLToPath(import.meta.url))
const composerTasksPath = path.resolve(__dirname, '../src/features/inbox/ComposerTasks.tsx')
const slideDrawerPath = path.resolve(__dirname, '../src/components/layout/SlideDrawer.tsx')
const chatCalmCssPath = path.resolve(__dirname, '../src/features/inbox/chat-calm.css')
const enJsonPath = path.resolve(__dirname, '../src/locales/en.json')
const idJsonPath = path.resolve(__dirname, '../src/locales/id.json')

const composerTasksSrc = fs.readFileSync(composerTasksPath, 'utf8')
const slideDrawerSrc = fs.readFileSync(slideDrawerPath, 'utf8')
const chatCalmCssSrc = fs.readFileSync(chatCalmCssPath, 'utf8')
const enJson = JSON.parse(fs.readFileSync(enJsonPath, 'utf8'))
const idJson = JSON.parse(fs.readFileSync(idJsonPath, 'utf8'))

test('1. SlideDrawer integration: opens SlideDrawer for desktop right panel and mobile full-sheet', () => {
  assert.ok(
    composerTasksSrc.includes('SlideDrawer'),
    'ComposerTasks must render SlideDrawer for task details'
  )
  assert.ok(
    composerTasksSrc.includes('data-testid="composer-tasks-drawer"'),
    'ComposerTasks must mount SlideDrawer with data-testid composer-tasks-drawer'
  )
})

test('2. Linear timeline without accordion nesting & total elimination of inline back button', () => {
  assert.ok(
    !composerTasksSrc.includes('data-testid="composer-tasks-back"'),
    'Inline Back to conversation button must be completely eliminated'
  )
  assert.ok(
    !composerTasksSrc.includes('<AccordionTimeline'),
    'ComposerTasks must not use nested AccordionTimeline'
  )
  assert.ok(
    composerTasksSrc.includes('data-timeline="linear-progress"'),
    'ComposerTasks must present tasks as a clean linear timeline'
  )
  assert.ok(
    composerTasksSrc.includes('left-[7.5px]') && composerTasksSrc.includes('w-px') && !composerTasksSrc.includes('w-0.5'),
    'ComposerTasks timeline connector must be a 1px hairline aligned to the center of the 16px status icon'
  )
  assert.ok(
    !chatCalmCssSrc.includes('width: 2px !important'),
    'chat-calm.css must not force 2px width on timeline connectors'
  )
})

test('3. Task actions (Rincian & Terminal) use standard Button component with >= 44px mobile touch target and no custom pill CSS override', () => {
  assert.ok(
    composerTasksSrc.includes("import { Button } from '../../components/ui/button'"),
    'ComposerTasks must import standard Button component'
  )
  assert.ok(
    composerTasksSrc.includes('<Button') && composerTasksSrc.includes("{t('Details')}") && composerTasksSrc.includes("{t('Terminal')}"),
    'Details and Terminal actions in ComposerTasks must use Button component'
  )
  assert.ok(
    composerTasksSrc.includes('min-h-11'),
    'ComposerTasks action buttons must satisfy >= 44px mobile touch target (min-h-11)'
  )
  assert.ok(
    !chatCalmCssSrc.includes('.composer-tasks [data-card-actions] > :is(button, a)') &&
      !chatCalmCssSrc.includes('[data-slide-drawer] [data-card-actions] > :is(button, a)'),
    'chat-calm.css must not override CardActions Button children with custom pill styling'
  )
  const cardActionsCssSrc = fs.readFileSync(
    path.resolve(__dirname, '../src/features/inbox/card-actions.css'),
    'utf8'
  )
  assert.ok(
    cardActionsCssSrc.includes(':not([data-slot="button"])'),
    'card-actions.css must not apply custom ::before pill surface to standard Button components'
  )
})

test('4. Terminal action button uses concise "Terminal" label', () => {
  assert.ok(
    composerTasksSrc.includes('{t(\'Terminal\')}'),
    'TaskTerminalAction must render concise label Terminal'
  )
  assert.ok(
    !composerTasksSrc.includes('{t(\'Live terminal\')}'),
    'TaskTerminalAction must not use verbose Live terminal label'
  )
})

test('5. Breathing room & single scroll container isolation', () => {
  assert.ok(
    slideDrawerSrc.includes('min-h-0 overflow-y-auto overscroll-contain'),
    'SlideDrawer body must own the single min-h-0 overflow-y-auto overscroll-contain scroll container'
  )
  assert.ok(
    composerTasksSrc.includes('bodyRef={panelRef}') && !composerTasksSrc.includes('composer-tasks-panel overscroll-contain overflow-y-auto'),
    'ComposerTasks must delegate scrolling to SlideDrawer bodyRef without nested overflow-y-auto'
  )
})

test('6. Localization completeness in id.json and en.json', () => {
  assert.equal(enJson['Terminal'], 'Terminal')
  assert.equal(idJson['Terminal'], 'Terminal')
  assert.equal(enJson['Conversation tasks'], 'Conversation tasks')
  assert.equal(idJson['Conversation tasks'], 'Task obrolan')
})

test('7. Header bar icon and badgeStatus sync with latestTask via shared TaskStatusIcon', () => {
  assert.ok(
    composerTasksSrc.includes('<TaskStatusIcon task={latestTask}'),
    'Header bar must render TaskStatusIcon for latestTask so icon matches its title'
  )
  assert.ok(
    composerTasksSrc.includes('<TaskStatusIcon task={task}'),
    'Drawer timeline item must use the same TaskStatusIcon helper'
  )
  assert.ok(
    composerTasksSrc.includes('getComposerTaskBadgeStatus(latestTask)'),
    'badgeStatus must derive from latestTask rather than aggregating unrelated older tasks'
  )
  assert.ok(
    !composerTasksSrc.includes('ordered.some(t => workStatus(t) === \'Needs attention\')'),
    'Header bar must not override latestTask icon with older Needs attention tasks'
  )
})
