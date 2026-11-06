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

const FORBIDDEN_JARGON_PATTERNS = [
  /\bdefinition[\s-]+of[\s-]+done\b/i,
  /\bdod\b/i,
  /\bheartbeat\s+lost\b/i,
  /\blease\s+expired\b/i,
  /\battempt\s*(?:#?\s*\d+)?\b/i,
]

function assertNoJargon(contextName, text) {
  for (const pattern of FORBIDDEN_JARGON_PATTERNS) {
    const match = text.match(pattern)
    assert.equal(
      match,
      null,
      `[${contextName}] Found forbidden technical jargon '${match?.[0]}' in text:\n"${text}"`
    )
  }
}

test('1. S65-T4 Quality Gate: Anti-Jargon Guardrail Test on task-envelope parser and visitor draft', async () => {
  const { parseTaskReviewEnvelope, sanitizeJargon } = await import('../src/features/inbox/task-envelope.ts')

  // Direct test of sanitizeJargon
  const dirtyText = 'Evaluasi DoD dan Definition of Done gagal karena runner heartbeat lost and lease expired pada attempt #2.'
  const cleanText = sanitizeJargon(dirtyText)
  assertNoJargon('sanitizeJargon direct output', cleanText)

  // Test parseTaskReviewEnvelope with dirty technical payload
  const dirtyEnvelope = JSON.stringify({
    summary: 'Evaluasi DoD menemukan masalah Definition of Done saat attempt #1.',
    draft: 'Halo! Terkait kebutuhan Anda, kami menangani issue heartbeat lost dan lease expired pada attempt #2.',
    evidence: 'Log server teknis',
    requires_deploy: false,
  })

  const task = {
    id: 'task_qg_dirty_envelope',
    title: 'Perbaikan bug Definition of Done dan heartbeat lost',
    status: 'review',
    summary: dirtyEnvelope,
  }

  const parsed = parseTaskReviewEnvelope(task)
  assertNoJargon('Parsed envelope summary', parsed.summary)
  assertNoJargon('Parsed envelope draft', parsed.draft)
})

test('2. S65-T4 Quality Gate: Dual-Path Test in Frontend Envelope Parser', async () => {
  const { parseTaskReviewEnvelope } = await import('../src/features/inbox/task-envelope.ts')

  // Path (a): Code-fix task requires deployment
  const codeFixTask = {
    id: 'task_qg_codefix',
    title: 'Perbaikan bug keranjang belanja',
    status: 'done',
    summary: JSON.stringify({
      summary: 'Perbaikan keranjang belanja telah dikompilasi.',
      draft: 'Halo! Keranjang belanja Anda telah kami perbarui dan berfungsi normal.',
      evidence: 'Commit abcdef123456 modified internal/cart/checkout.go',
      requires_deploy: true,
    }),
  }
  const resCodeFix = parseTaskReviewEnvelope(codeFixTask)
  assert.equal(resCodeFix.requiresDeploy, true, 'Code-fix task must have requiresDeploy: true')
  assert.equal(resCodeFix.isFromEnvelope, true)

  // Path (b): Investigation task does NOT require deployment
  const investigationTask = {
    id: 'task_qg_investigation',
    title: 'Investigasi error 500 payment gateway',
    status: 'done',
    summary: JSON.stringify({
      summary: 'Investigasi log payment gateway selesai.',
      draft: 'Halo! Kami telah mengaudit log payment gateway dan mengidentifikasi penyebabnya.',
      evidence: 'RCA: Gangguan pihak ketiga eksternal, tidak ada perubahan kode',
      requires_deploy: false,
    }),
  }
  const resInv = parseTaskReviewEnvelope(investigationTask)
  assert.equal(resInv.requiresDeploy, false, 'Investigation task must have requiresDeploy: false')
  assert.equal(resInv.isFromEnvelope, true)
})

test('3. S65-T4 Quality Gate: Visual Ergonomics, SlideDrawer segregation & 1-Click Draft Action', () => {
  const taskCompletionCardSrc = fs.readFileSync(taskCompletionCardPath, 'utf8')
  const taskCardSrc = fs.readFileSync(taskCardPath, 'utf8')
  const inboxViewSrc = fs.readFileSync(inboxViewPath, 'utf8')

  // Ensure 1-click draft action exists and is wired to composer
  assert.ok(
    taskCompletionCardSrc.includes('onUseDraft') || taskCompletionCardSrc.includes('useDraftAction'),
    'TaskCompletionCard must support 1-click draft insertion into composer'
  )

  // Ensure live terminal button is present
  assert.ok(
    taskCompletionCardSrc.includes('onOpenTerminal') || taskCompletionCardSrc.includes('Live Terminal'),
    'TaskCompletionCard must offer live terminal bridge'
  )

  // Ensure SlideDrawer integration for secondary technical details
  assert.ok(
    inboxViewSrc.includes('InboxWorkPanel') && (inboxViewSrc.includes('peekTask') || inboxViewSrc.includes('inbox-task-drawer')),
    'InboxView must isolate diagnostic details and terminal into SlideDrawer'
  )

  // Ensure card diet: raw DoD checklist is not dumped directly into main timeline
  assert.ok(
    !taskCardSrc.includes('table-fixed') && !taskCompletionCardSrc.includes('table-fixed'),
    'Chat timeline must avoid heavy wireframe tables in favor of calm cards'
  )
})
