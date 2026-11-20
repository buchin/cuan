import test from 'node:test'
import assert from 'node:assert/strict'
import fs from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'

const __filename = fileURLToPath(import.meta.url)
const __dirname = path.dirname(__filename)

const inboxViewPath = path.join(__dirname, '../src/features/inbox/InboxView.tsx')
const studioViewPath = path.join(__dirname, '../src/features/inbox/StudioView.tsx')
const composerInputPath = path.join(__dirname, '../src/components/ui/ComposerInput.tsx')
const calmCssPath = path.join(__dirname, '../src/features/inbox/chat-calm.css')

const inboxViewSrc = fs.readFileSync(inboxViewPath, 'utf8')
const studioViewSrc = fs.readFileSync(studioViewPath, 'utf8')
const composerInputSrc = fs.readFileSync(composerInputPath, 'utf8')
const calmCssSrc = fs.readFileSync(calmCssPath, 'utf8')

test('1. Composer container alignment with chat stream (DoD 1)', () => {
  // InboxView composer wrapper must have calm-stream-constraint max-w-[75ch] w-full
  assert.ok(
    inboxViewSrc.includes('calm-stream-constraint max-w-[75ch] mx-auto w-full bg-transparent'),
    'Composer wrapper in InboxView must use calm-stream-constraint max-w-[75ch] mx-auto w-full'
  )

  // Chat message list inner constraint must also use calm-stream-constraint max-w-[75ch] mx-auto w-full
  assert.ok(
    inboxViewSrc.includes('calm-stream-constraint max-w-[75ch] mx-auto w-full space-y-2 pb-4'),
    'Chat stream in InboxView must use calm-stream-constraint max-w-[75ch] mx-auto w-full'
  )

  // Studio canvas inside InboxView must also use calm-stream-constraint max-w-[75ch] w-full instead of max-w-2xl
  assert.ok(
    inboxViewSrc.includes('calm-stream-constraint max-w-[75ch] mx-auto w-full'),
    'Studio canvas in InboxView must align with calm-stream-constraint max-w-[75ch] w-full'
  )

  // chat-calm.css @media (min-width: 640px) must use 24px padding matching sm:px-6 (24px)
  assert.ok(
    calmCssSrc.includes('padding: 12px 24px max(12px, env(safe-area-inset-bottom, 0px));'),
    'chat-calm.css must specify 24px horizontal padding at >=640px to match sm:px-6 of chat stream'
  )
})

test('2. Textarea auto-expand and max-height cap (DoD 2)', () => {
  // ComposerInput must allow expansion up to 200px
  assert.ok(
    composerInputSrc.includes('max-h-[200px]'),
    'ComposerInput must define max-h-[200px] to allow comfortable multi-line typing'
  )

  assert.ok(
    composerInputSrc.includes('Math.min(200,'),
    'resizeComposerInput cap must allow expansion up to 200px'
  )

  // StudioView must define autoResizeGoal auto-expansion up to 320px
  assert.ok(
    studioViewSrc.includes('const autoResizeGoal'),
    'StudioView must implement autoResizeGoal for smooth multi-line expansion'
  )
  assert.ok(
    studioViewSrc.includes('Math.min(Math.max(el.scrollHeight, minimum), cap)'),
    'autoResizeGoal must calculate dynamic height between minimum and cap'
  )
})

test('3. Mobile 390px touch targets and padding (DoD 3)', () => {
  // Studio dynamic canvas mobile padding must use px-3 (12px) matching chat stream
  assert.ok(
    inboxViewSrc.includes('px-3 py-6 sm:px-6 md:px-8'),
    'Studio dynamic canvas must use px-3 for mobile to maximize typing width'
  )

  // Mobile composer padding must be 12px
  assert.ok(
    calmCssSrc.includes('padding: 12px 12px max(12px, calc(env(safe-area-inset-bottom, 0px) + 8px));'),
    'Mobile composer padding must be 12px matching px-3 of message stream'
  )
})
