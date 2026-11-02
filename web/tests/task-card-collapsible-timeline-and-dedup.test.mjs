import test from 'node:test'
import assert from 'node:assert/strict'
import fs from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'

const __filename = fileURLToPath(import.meta.url)
const __dirname = path.dirname(__filename)
const webDir = path.resolve(__dirname, '..')

const taskCardPath = path.resolve(webDir, 'src/features/inbox/TaskCard.tsx')
const inboxViewPath = path.resolve(webDir, 'src/features/inbox/InboxView.tsx')
const idLocalePath = path.resolve(webDir, 'src/locales/id.json')
const enLocalePath = path.resolve(webDir, 'src/locales/en.json')

const taskCardSrc = fs.readFileSync(taskCardPath, 'utf8')
const inboxViewSrc = fs.readFileSync(inboxViewPath, 'utf8')
const idLocale = JSON.parse(fs.readFileSync(idLocalePath, 'utf8'))
const enLocale = JSON.parse(fs.readFileSync(enLocalePath, 'utf8'))

test('1. TaskCard: Collapsible session history timeline state & toggle button', () => {
  assert.ok(
    taskCardSrc.includes('const [showTimeline, setShowTimeline] = useState(false)'),
    'TaskCard must have showTimeline state initialized to false'
  )
  assert.ok(
    taskCardSrc.includes('const attempts = useMemo'),
    'TaskCard must memoize attempts from currentTask.attempts and latest_attempt'
  )
  assert.ok(
    taskCardSrc.includes('data-testid="task-card-timeline-toggle"'),
    'TaskCard must include timeline toggle button'
  )
  assert.ok(
    taskCardSrc.includes('data-testid="task-card-timeline-content"'),
    'TaskCard must conditionally render timeline content'
  )
})

test('2. TaskCard: Attempt timeline row formatting, status, and error display', () => {
  assert.ok(
    taskCardSrc.includes('data-testid={`task-attempt-row-${attemptNum}`}'),
    'TaskCard must render each attempt with a testid'
  )
  assert.ok(
    taskCardSrc.includes('att.error_message'),
    'TaskCard must display attempt error message when present'
  )
  assert.ok(
    taskCardSrc.includes('formatDuration'),
    'TaskCard must format attempt elapsed duration'
  )
})

test('3. InboxView: Deduplicates stacked boilerplate launch messages for the same task', () => {
  assert.ok(
    inboxViewSrc.includes('isBoilerplateLaunchMessage'),
    'InboxView must identify boilerplate launch messages'
  )
  assert.ok(
    inboxViewSrc.includes("Work started. I'll report progress and the result here.") ||
    inboxViewSrc.includes('Work started. I’ll report progress and the result here.'),
    'InboxView must check for Work started launch message'
  )
  assert.ok(
    inboxViewSrc.includes('messages.slice(index + 1).some'),
    'InboxView must inspect subsequent messages for the same taskId to suppress duplicates'
  )
})

test('4. Localization: Dictionaries include Riwayat Sesi, Sesi, and toggle labels', () => {
  assert.equal(idLocale['Riwayat Sesi'], 'Riwayat Sesi')
  assert.equal(enLocale['Riwayat Sesi'], 'Session History')
  assert.equal(idLocale['Sesi'], 'Sesi')
  assert.equal(enLocale['Sesi'], 'Session')
  assert.equal(idLocale['Lihat'], 'Lihat')
  assert.equal(enLocale['Lihat'], 'View')
  assert.equal(idLocale['Sembunyikan'], 'Sembunyikan')
  assert.equal(enLocale['Sembunyikan'], 'Hide')
})
