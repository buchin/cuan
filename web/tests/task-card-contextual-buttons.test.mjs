import test from 'node:test'
import assert from 'node:assert/strict'
import fs from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'

const __filename = fileURLToPath(import.meta.url)
const __dirname = path.dirname(__filename)
const webDir = path.resolve(__dirname, '..')

const taskItemCardPath = path.resolve(webDir, 'src/features/tasks/TaskItemCard.tsx')
const taskCardPath = path.resolve(webDir, 'src/features/inbox/TaskCard.tsx')
const landingPagePath = path.resolve(webDir, 'src/features/landing/LandingPage.tsx')
const designGuidelinesPath = path.resolve(webDir, '../docs/design/design_guidelines.md')
const idLocalePath = path.resolve(webDir, 'src/locales/id.json')
const enLocalePath = path.resolve(webDir, 'src/locales/en.json')

test('1. TaskItemCard: Contextual separation of Detail vs Retry and elimination of redundant buttons', () => {
  const src = fs.readFileSync(taskItemCardPath, 'utf8')

  // Secondary Detail button is conditionally scoped (not unconditionally rendered on all states)
  assert.ok(
    src.includes("status === 'Needs attention' ||") &&
    src.includes("(currentTask.pr_url && !applied && status === 'Ready for review')"),
    'TaskItemCard must conditionally render Detail button only on Needs attention or unapplied PR review'
  )

  // Secondary Detail CTA must satisfy touch target standards
  assert.ok(
    src.includes('action-secondary min-h-11 min-w-[44px]'),
    'TaskItemCard secondary Detail CTA must have min-h-11 min-w-[44px] touch target'
  )

  // Primary contextual Retry CTA with spinner and loading indicator
  assert.ok(
    src.includes("status === 'Needs attention' ? (") &&
    src.includes("disabled={retrying}") &&
    src.includes("onClick={handleRetry}") &&
    src.includes("RefreshCw") &&
    src.includes("retrying ? t('Retrying...') : t('Retry')"),
    'TaskItemCard must provide 1-click contextual Retry button with loading spinner on Needs attention'
  )

  // Single-CTA clarity on normal/completed states (View result, Live Terminal, Start session)
  assert.ok(
    src.includes("t('View result')"),
    'TaskItemCard must provide View result CTA'
  )
  assert.ok(
    src.includes("t('Live Terminal')"),
    'TaskItemCard must provide Live Terminal CTA'
  )

  // Retry error display
  assert.ok(
    src.includes('{retryError && ('),
    'TaskItemCard must render retryError feedback'
  )
})

test('2. TaskCard (Inbox Stream): Contextual Retry, elimination of duplicate Open task, and error feedback', () => {
  const src = fs.readFileSync(taskCardPath, 'utf8')

  // When isNeedsAttention is true, primary button is Retry; when false, terminal/open button is rendered
  assert.ok(
    src.includes('isNeedsAttention ? (') &&
    src.includes('data-testid="task-card-retry-btn"') &&
    src.includes('data-testid="task-card-terminal-btn"'),
    'TaskCard must conditionally switch between primary Retry and Terminal/Open button'
  )

  // Primary Retry button features loading spinner and disabled state
  assert.ok(
    src.includes("retrying && 'animate-spin'") &&
    src.includes("retrying ? t('Retrying...') : t('Retry')"),
    'TaskCard must show animated spinner and Retrying... text when retrying'
  )

  // Diagnostic link preserved on left
  assert.ok(
    src.includes('data-testid="task-card-diagnostic-link"'),
    'TaskCard must preserve diagnostic details link on left'
  )

  // Error feedback state and render
  assert.ok(
    src.includes("const [retryError, setRetryError] = useState('')") &&
    src.includes('{retryError && ('),
    'TaskCard must provide retryError state and error notification'
  )
})

test('3. Locales: Completeness of Retry, Retrying..., and Failed to retry session', () => {
  const idLocale = JSON.parse(fs.readFileSync(idLocalePath, 'utf8'))
  const enLocale = JSON.parse(fs.readFileSync(enLocalePath, 'utf8'))

  assert.equal(idLocale['Retry'], 'Coba lagi')
  assert.equal(idLocale['Retrying...'], 'Mencoba lagi...')
  assert.equal(idLocale['Failed to retry session'], 'Gagal mengulang sesi')

  assert.equal(enLocale['Retry'], 'Retry')
  assert.equal(enLocale['Retrying...'], 'Retrying...')
  assert.equal(enLocale['Failed to retry session'], 'Failed to retry session')
})

test('4. Documentation & Landing Page: Synchronous reflection of contextual buttons', () => {
  const landingSrc = fs.readFileSync(landingPagePath, 'utf8')
  const designSrc = fs.readFileSync(designGuidelinesPath, 'utf8')

  assert.ok(
    landingSrc.includes('contextual action buttons (1-click retry on failure, live terminal when running, zero redundant double actions)'),
    'LandingPage must reflect contextual action buttons and zero redundant double actions'
  )

  assert.ok(
    designSrc.includes('Pemisahan Peran Tombol Kartu Tugas: Navigasi vs Aksi Kontekstual (Zero Redundant CTA)') &&
    designSrc.includes('single-CTA clarity') &&
    designSrc.includes('Needs attention'),
    'design_guidelines.md must document the contextual button separation and single-CTA clarity standard'
  )
})
