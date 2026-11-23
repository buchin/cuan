import test from 'node:test'
import assert from 'node:assert/strict'
import fs from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'

const __dirname = path.dirname(fileURLToPath(import.meta.url))
const composerTasksPath = path.resolve(__dirname, '../src/features/inbox/ComposerTasks.tsx')
const chatCalmCssPath = path.resolve(__dirname, '../src/features/inbox/chat-calm.css')
const enJsonPath = path.resolve(__dirname, '../src/locales/en.json')
const idJsonPath = path.resolve(__dirname, '../src/locales/id.json')

const composerTasksSrc = fs.readFileSync(composerTasksPath, 'utf8')
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
})

test('3. Affordance pill styling on task actions (Rincian) with >= 44px touch target', () => {
  assert.ok(
    chatCalmCssSrc.includes('.composer-tasks [data-card-actions] > :is(button, a)'),
    'CSS must style card actions specifically for composer-tasks'
  )
  assert.ok(
    chatCalmCssSrc.includes('border-radius: 9999px;'),
    'Card action buttons must have pill border radius'
  )
  assert.ok(
    chatCalmCssSrc.includes('min-height: 44px;') && chatCalmCssSrc.includes('::after'),
    'Card action buttons must satisfy 44px touch target via expanded hit target'
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

test('5. Breathing room & scroll fighting isolation', () => {
  assert.ok(
    composerTasksSrc.includes('overscroll-contain overflow-y-auto scroll-pb-2'),
    'Panel must isolate scroll with overscroll-contain and overflow-y-auto'
  )
})

test('6. Localization completeness in id.json and en.json', () => {
  assert.equal(enJson['Terminal'], 'Terminal')
  assert.equal(idJson['Terminal'], 'Terminal')
  assert.equal(enJson['Conversation tasks'], 'Conversation tasks')
  assert.equal(idJson['Conversation tasks'], 'Task obrolan')
})
