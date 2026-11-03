import test from 'node:test'
import assert from 'node:assert/strict'
import fs from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'

const __filename = fileURLToPath(import.meta.url)
const __dirname = path.dirname(__filename)
const webDir = path.resolve(__dirname, '..')

const taskCardPath = path.resolve(webDir, 'src/features/inbox/TaskCard.tsx')
const approvalCardPath = path.resolve(webDir, 'src/features/inbox/ApprovalCard.tsx')
const collapsibleTextPath = path.resolve(webDir, 'src/features/inbox/CollapsibleChatText.tsx')
const messageContentPath = path.resolve(webDir, 'src/features/inbox/MessageContent.tsx')
const idLocalePath = path.resolve(webDir, 'src/locales/id.json')
const enLocalePath = path.resolve(webDir, 'src/locales/en.json')

const taskCardSrc = fs.readFileSync(taskCardPath, 'utf8')
const approvalCardSrc = fs.readFileSync(approvalCardPath, 'utf8')
const collapsibleTextSrc = fs.readFileSync(collapsibleTextPath, 'utf8')
const messageContentSrc = fs.readFileSync(messageContentPath, 'utf8')
const idLocale = JSON.parse(fs.readFileSync(idLocalePath, 'utf8'))
const enLocale = JSON.parse(fs.readFileSync(enLocalePath, 'utf8'))

test('1. TaskCard: Elimination of redundant header buttons and preservation of footer actions', () => {
  // Must NOT have action buttons in the header row
  assert.ok(
    !taskCardSrc.includes('Live Terminal & Retry Actionable Controls in Header'),
    'TaskCard must eliminate redundant action buttons from header row'
  )

  // Must preserve 44px touch targets in the footer
  assert.ok(
    taskCardSrc.includes('data-testid="task-card-terminal-btn"'),
    'TaskCard footer must provide Live Terminal button'
  )
  assert.ok(
    taskCardSrc.includes('data-testid="task-card-retry-btn"'),
    'TaskCard footer must provide Retry button for failed states'
  )
})

// Replaced by rendered interaction checks in docs/evidence/accounts/model-picker-check.cjs.


test('3. ApprovalCard: Collapsible plan description', () => {
  assert.ok(
    approvalCardSrc.includes('data-testid="approval-card-toggle-plan"'),
    'ApprovalCard must provide toggle button for long plan descriptions'
  )
  assert.ok(
    approvalCardSrc.includes('max-h-[140px] overflow-hidden'),
    'ApprovalCard must clamp long plans to prevent pushing action buttons off-screen'
  )
  assert.ok(
    approvalCardSrc.includes('<ChatMarkdown content={plan.description}'),
    'ApprovalCard must render markdown using ChatMarkdown'
  )
})

test('4. MessageContent: Bubble preservation and integration with CollapsibleChatText', () => {
  assert.ok(
    messageContentSrc.includes('CollapsibleChatText'),
    'MessageContent must render text using CollapsibleChatText'
  )
  assert.ok(
    messageContentSrc.includes('calm-message-bubble wa-incoming rounded-2xl'),
    'MessageContent must preserve speech bubble container when sibling action cards exist'
  )
})

test('5. Localization completeness for Read more and Plan details', () => {
  assert.equal(idLocale['Read more'], 'Baca selengkapnya')
  assert.equal(idLocale['Show less'], 'Tampilkan lebih sedikit')
  assert.equal(idLocale['Show full plan'], 'Lihat seluruh rencana')
  assert.equal(idLocale['Hide plan details'], 'Sembunyikan detail')

  assert.equal(enLocale['Read more'], 'Read more')
  assert.equal(enLocale['Show less'], 'Show less')
  assert.equal(enLocale['Show full plan'], 'Show full plan')
  assert.equal(enLocale['Hide plan details'], 'Hide details')
})
