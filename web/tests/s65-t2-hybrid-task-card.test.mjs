import test from 'node:test'
import assert from 'node:assert/strict'
import fs from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'

const __filename = fileURLToPath(import.meta.url)
const __dirname = path.dirname(__filename)
const webDir = path.resolve(__dirname, '..')

const taskEnvelopePath = path.resolve(webDir, 'src/features/inbox/task-envelope.ts')
const taskCompletionCardPath = path.resolve(webDir, 'src/features/inbox/TaskCompletionCard.tsx')
const taskCardPath = path.resolve(webDir, 'src/features/inbox/TaskCard.tsx')
const inboxViewPath = path.resolve(webDir, 'src/features/inbox/InboxView.tsx')
const idLocalePath = path.resolve(webDir, 'src/locales/id.json')
const enLocalePath = path.resolve(webDir, 'src/locales/en.json')

test('1. S65-T2: task-envelope parser extracts dual-channel envelope JSON and provides safe fallbacks', async () => {
  const envelopeSource = fs.readFileSync(taskEnvelopePath, 'utf8')
  assert.ok(envelopeSource.includes('export function parseTaskReviewEnvelope'), 'Must export parseTaskReviewEnvelope')
  assert.ok(envelopeSource.includes('requires_deploy'), 'Must check requires_deploy')

  const { parseTaskReviewEnvelope } = await import('../src/features/inbox/task-envelope.ts')

  // Case A: Full dual-channel JSON envelope in task.summary
  const jsonEnvelope = JSON.stringify({
    summary: 'Investigasi log server telah tuntas.',
    draft: 'Halo! Permintaan Anda terkait audit koneksi telah kami selesaikan dengan baik.',
    evidence: 'RCA menunjukkan timeout 5m pada pihak ketiga; tidak ada perubahan kode repositori.',
    requires_deploy: false,
  })

  const taskA = {
    id: 'task_envelope_1',
    title: 'Audit koneksi pihak ketiga',
    status: 'done',
    summary: jsonEnvelope,
  }

  const resA = parseTaskReviewEnvelope(taskA)
  assert.equal(resA.isFromEnvelope, true, 'Must identify envelope JSON')
  assert.equal(resA.summary, 'Investigasi log server telah tuntas.')
  assert.equal(resA.draft, 'Halo! Permintaan Anda terkait audit koneksi telah kami selesaikan dengan baik.')
  assert.equal(resA.requiresDeploy, false)
  assert.ok(resA.evidence.includes('RCA menunjukkan timeout'))

  // Case B: Non-JSON plain string task with fallback draft
  const taskB = {
    id: 'task_plain_2',
    title: 'Perbaikan bug keranjang belanja',
    status: 'done',
    summary: 'Kode checkout telah diperbaiki dan diuji.',
  }

  const resB = parseTaskReviewEnvelope(taskB)
  assert.equal(resB.isFromEnvelope, false)
  assert.equal(resB.summary, 'Kode checkout telah diperbaiki dan diuji.')
  assert.ok(resB.draft.includes('Perbaikan bug keranjang belanja'))
  assert.equal(resB.requiresDeploy, false)

  // Case C: Review status task
  const taskC = {
    id: 'task_review_3',
    title: 'Review proposal desain',
    status: 'review',
    summary: 'Desain telah siap untuk ditinjau.',
  }
  const resC = parseTaskReviewEnvelope(taskC)
  assert.ok(resC.draft.includes('Review proposal desain'))
})

test('2. S65-T2: TaskCompletionCard integrates Live Terminal, Retry, and 1-Click Actionable Visitor Reply Draft', () => {
  const cardSource = fs.readFileSync(taskCompletionCardPath, 'utf8')

  // Header Calm Controls
  assert.ok(
    cardSource.includes('data-testid="task-card-terminal-btn"'),
    'TaskCompletionCard must include Live Terminal button with data-testid="task-card-terminal-btn"'
  )
  assert.ok(
    cardSource.includes('data-testid="task-card-retry-btn"'),
    'TaskCompletionCard must include Retry button with data-testid="task-card-retry-btn"'
  )
  assert.ok(
    cardSource.includes('handleOpenTerminal'),
    'TaskCompletionCard must handle opening terminal'
  )
  assert.ok(
    cardSource.includes('handleRetry'),
    'TaskCompletionCard must handle task retry'
  )

  // Visitor Reply Draft Section & 1-Click Actionable button
  assert.ok(
    cardSource.includes('data-testid="task-card-draft-section"'),
    'TaskCompletionCard must render visitor reply draft container'
  )
  assert.ok(
    cardSource.includes('data-testid="task-card-use-draft-btn"'),
    'TaskCompletionCard must render [Gunakan Draf di Input] button'
  )
  assert.ok(
    cardSource.includes('handleUseDraft'),
    'TaskCompletionCard must define handleUseDraft 1-click action'
  )
  assert.ok(
    cardSource.includes('parseTaskReviewEnvelope'),
    'TaskCompletionCard must use parseTaskReviewEnvelope'
  )

  // Secondary Technical Details Link to SlideDrawer
  assert.ok(
    cardSource.includes('data-testid="task-card-diagnostic-link"'),
    'TaskCompletionCard must render [Detail Diagnostik & Status Deploy →] link'
  )
  assert.ok(
    cardSource.includes("t('Detail Diagnostik & Status Deploy →')"),
    'Must display localized diagnostic link text'
  )

  // Clean chat timeline: zero raw checklist <ul> on the chat card
  assert.ok(
    !cardSource.includes('<ul className="space-y-1.5 pt-0.5">'),
    'Raw DoD checklist items must be purged from chat timeline card'
  )
})

test('3. S65-T2: TaskCard provides unified actionable controls and diagnostic drawer handoff', () => {
  const taskCardSource = fs.readFileSync(taskCardPath, 'utf8')

  assert.ok(
    taskCardSource.includes('data-testid="task-card-terminal-btn"'),
    'TaskCard must render Live Terminal button'
  )
  assert.ok(
    taskCardSource.includes('data-testid="task-card-retry-btn"'),
    'TaskCard must render Retry button'
  )
  assert.ok(
    taskCardSource.includes('data-testid="task-card-draft-section"'),
    'TaskCard must support visitor reply draft section'
  )
  assert.ok(
    taskCardSource.includes('data-testid="task-card-use-draft-btn"'),
    'TaskCard must render 1-click use draft button'
  )
  assert.ok(
    taskCardSource.includes('data-testid="task-card-diagnostic-link"'),
    'TaskCard must render diagnostic details link'
  )
})

test('4. S65-T2: InboxView SlideDrawer parks DoD checklist, file diff/status deploy, and Live Terminal xterm stream', () => {
  const inboxViewSource = fs.readFileSync(inboxViewPath, 'utf8')

  // SlideDrawer tabs: Details vs Live Terminal
  assert.ok(
    inboxViewSource.includes('peekTab'),
    'InboxView must maintain peekTab state'
  )
  assert.ok(
    inboxViewSource.includes('data-testid="drawer-live-terminal-tab"'),
    'SlideDrawer must render Live Terminal tab toggle'
  )
  assert.ok(
    inboxViewSource.includes('TaskTerminal'),
    'SlideDrawer must embed TaskTerminal component for xterm stream'
  )

  // Deploy Status in SlideDrawer
  assert.ok(
    inboxViewSource.includes("t('Status Deploy')"),
    'SlideDrawer must display Status Deploy section'
  )
  assert.ok(
    inboxViewSource.includes('envelopeResult.requiresDeploy'),
    'SlideDrawer must check requiresDeploy from dual-channel envelope'
  )

  // 1-Click draft injection into composer
  assert.ok(
    inboxViewSource.includes('handleUseDraft'),
    'InboxView must provide handleUseDraft callback'
  )
  assert.ok(
    inboxViewSource.includes('setInputMessage(draftText)'),
    'handleUseDraft must set composer input text'
  )
  assert.ok(
    inboxViewSource.includes('textareaRef.current.focus()'),
    'handleUseDraft must auto-focus textarea'
  )

  // Clean timeline: action_card definition of done raw list eliminated
  assert.ok(
    !inboxViewSource.includes('Definition of Done (DoD):\n'),
    'InboxView timeline must be clean of raw Definition of Done (DoD) header'
  )
})

test('5. S65-T2: Localization dictionaries completeness in id.json and en.json', () => {
  const idLocale = JSON.parse(fs.readFileSync(idLocalePath, 'utf8'))
  const enLocale = JSON.parse(fs.readFileSync(enLocalePath, 'utf8'))

  const requiredKeys = [
    'Gunakan Draf di Input',
    'Draf Balasan Pengunjung',
    'Detail Diagnostik & Status Deploy →',
    'Detail Diagnostik & Status Deploy',
    'Status Deploy',
    'Coba Lagi',
    '1-klik mengisi composer obrolan dan fokus kursor otomatis.',
  ]

  for (const key of requiredKeys) {
    assert.ok(idLocale[key], `id.json must include translation for: ${key}`)
    assert.ok(enLocale[key], `en.json must include translation for: ${key}`)
  }
})
