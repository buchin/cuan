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

test('1. Full-height sheet on mobile: panel uses 100dvh viewport height to eliminate clipping', () => {
  assert.ok(
    composerTasksSrc.includes('h-[calc(100dvh-120px)]'),
    'ComposerTasks panel must use 100dvh-based height on mobile for full-sheet presentation'
  )
  assert.ok(
    composerTasksSrc.includes('sm:max-h-[calc(100dvh-160px)]'),
    'ComposerTasks panel must have generous desktop max-height'
  )
})

test('2. Intuitive back to conversation & collapse navigation with 1 tap', () => {
  assert.ok(
    composerTasksSrc.includes('data-testid="composer-tasks-back"'),
    'Back to conversation button must have testid composer-tasks-back'
  )
  assert.ok(
    composerTasksSrc.includes('t(\'Back to conversation\')'),
    'Back to conversation button must use localized text'
  )
  assert.ok(
    composerTasksSrc.includes('data-testid="composer-tasks-collapse"'),
    'Top task header must have collapse toggle'
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
  assert.equal(enJson['Back to conversation'], 'Back to conversation')
  assert.equal(idJson['Back to conversation'], 'Kembali ke percakapan')
  assert.equal(enJson['Collapse tasks'], 'Collapse tasks')
  assert.equal(idJson['Collapse tasks'], 'Tutup tugas')
})
