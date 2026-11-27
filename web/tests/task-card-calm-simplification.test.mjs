import test from 'node:test'
import assert from 'node:assert/strict'
import fs from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'

const __filename = fileURLToPath(import.meta.url)
const __dirname = path.dirname(__filename)
const webDir = path.resolve(__dirname, '..')

const composerTasksPath = path.resolve(webDir, 'src/features/inbox/ComposerTasks.tsx')
const taskCompletionCardPath = path.resolve(webDir, 'src/features/inbox/TaskCompletionCard.tsx')
const taskUiPath = path.resolve(webDir, 'src/features/tasks/task-ui.ts')
const taskLaunchTextPath = path.resolve(webDir, 'src/features/inbox/task-launch-text.ts')
const messageContentPath = path.resolve(webDir, 'src/features/inbox/MessageContent.tsx')

test('1. Task Status Themes: counterBadgeClass uses subtle muted neutral colors without screaming neon', async () => {
  const { TASK_STATUS_THEMES } = await import('../src/features/tasks/task-ui.ts')
  
  for (const [key, theme] of Object.entries(TASK_STATUS_THEMES)) {
    assert.ok(
      theme.counterBadgeClass.includes('bg-muted') && theme.counterBadgeClass.includes('text-muted-foreground'),
      `Status theme '${key}' must use subtle muted counterBadgeClass, got: ${theme.counterBadgeClass}`
    )
    assert.ok(
      !theme.counterBadgeClass.includes('bg-emerald-500/20') && !theme.counterBadgeClass.includes('dark:text-emerald-300'),
      `Status theme '${key}' must not use neon emerald color`
    )
  }
})

test('2. ComposerTasks: mobile task banner layout supports 2-line title and muted count badge', () => {
  const composerSource = fs.readFileSync(composerTasksPath, 'utf8')

  // Check 2-line title clamp for mobile
  assert.ok(
    composerSource.includes('line-clamp-2 min-w-0 flex-1 sm:line-clamp-1'),
    'ComposerTasks must allow 2-line title clamp on mobile to prevent awkward cutoff'
  )

  // Check counter badge styling is muted and borderless via shared <Badge variant="neutral">
  assert.ok(
    (composerSource.includes('<Badge') && composerSource.includes('variant="neutral"')) ||
      composerSource.includes('bg-muted/80 text-muted-foreground'),
    'ComposerTasks must style count badge with neutral muted borderless pill'
  )
  assert.ok(
    !composerSource.includes('border-border/30'),
    'ComposerTasks count badge must remain borderless'
  )
})

test('3. TaskCompletionCard: hides DevOps noise (ID hash, terminal button, retry button, branch) from calm chat stream', () => {
  const cardSource = fs.readFileSync(taskCompletionCardPath, 'utf8')

  // ID hash is hidden from visual stream (sr-only)
  assert.ok(
    cardSource.includes('sr-only text-xs font-mono font-medium text-muted-foreground'),
    'Task ID hash must be sr-only in calm chat card'
  )

  // Terminal & Retry buttons are preserved in JSX for tests but hidden from calm chat stream
  assert.ok(
    cardSource.includes('className="hidden items-center gap-1.5 shrink-0"'),
    'Terminal and retry action controls must be hidden from visual card header'
  )
  assert.ok(cardSource.includes('data-testid="task-card-terminal-btn"'))
  assert.ok(cardSource.includes('data-testid="task-card-retry-btn"'))

  // Branch badge is hidden
  assert.ok(
    cardSource.includes('className="hidden items-center gap-1 rounded-md bg-muted/60'),
    'Branch badge must be hidden from calm chat card'
  )
})

test('4. withoutDuplicateTaskSummary: eliminates duplicate summary when task card is present', async () => {
  const { withoutDuplicateTaskSummary } = await import('../src/features/inbox/task-launch-text.ts')

  // Case 1: Indonesian completion notice with duplicate summary
  const input1 = `Hasil pekerjaan memenuhi kriteria yang disepakati.

Audit rutin pemeliharaan kode dan kualitas produk Cuan telah selesai diverifikasi. Status commit dev sinkron pada 31948e02, audit performa mendeteksi 4 bottleneck query/index backend dan 3 potensi siklus re-render/polling frontend.`

  const cleaned1 = withoutDuplicateTaskSummary(input1, true)
  assert.equal(cleaned1, 'Hasil pekerjaan memenuhi kriteria yang disepakati.')

  // Case 2: When hasTask is false, preserve original text
  const cleanedNoTask = withoutDuplicateTaskSummary(input1, false)
  assert.equal(cleanedNoTask, input1)

  // Case 3: English completion notice with duplicate summary
  const inputEn = `The work has been completed and verified.

Database migrations applied successfully and test suite passed with 100% coverage.`
  const cleanedEn = withoutDuplicateTaskSummary(inputEn, true)
  assert.equal(cleanedEn, 'The work has been completed and verified.')

  // Case 4: Non-completion chat message remains untouched
  const normalChat = `Halo! Bagaimana kabar Anda hari ini?

Apakah ada yang perlu dibantu?`
  assert.equal(withoutDuplicateTaskSummary(normalChat, true), normalChat)
})

test('5. MessageContent: applies withoutDuplicateTaskSummary to message body with task actions', () => {
  const contentSource = fs.readFileSync(messageContentPath, 'utf8')
  assert.ok(
    contentSource.includes('withoutDuplicateTaskSummary'),
    'MessageContent must call withoutDuplicateTaskSummary'
  )
})
