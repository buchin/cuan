import test from 'node:test'
import assert from 'node:assert/strict'
import fs from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'

const __filename = fileURLToPath(import.meta.url)
const __dirname = path.dirname(__filename)
const webDir = path.resolve(__dirname, '..')

const approvalCardPath = path.resolve(webDir, 'src/features/inbox/ApprovalCard.tsx')
const inboxViewPath = path.resolve(webDir, 'src/features/inbox/InboxView.tsx')
const messageContentPath = path.resolve(webDir, 'src/features/inbox/MessageContent.tsx')

const approvalCardSrc = fs.readFileSync(approvalCardPath, 'utf8')
const inboxViewSrc = fs.readFileSync(inboxViewPath, 'utf8')
const messageContentSrc = fs.readFileSync(messageContentPath, 'utf8')

test('1. ApprovalCard eliminates nested box cards (border ganda & background bertingkat)', () => {
  assert.ok(fs.existsSync(approvalCardPath), 'ApprovalCard.tsx must exist')

  // Must eliminate outer 4-sided border and stacked card background
  assert.ok(
    !approvalCardSrc.includes("rounded-xl border p-3.5"),
    'ApprovalCard must eliminate rounded-xl border outer box'
  )
  assert.ok(
    !approvalCardSrc.includes("bg-card/60") && !approvalCardSrc.includes("bg-card/20"),
    'ApprovalCard must eliminate stacked bg-card backgrounds'
  )
  assert.ok(
    !approvalCardSrc.includes("shadow-2xs"),
    'ApprovalCard must eliminate card drop shadow'
  )

  // Must use calm transparent surface with clean left accent indicator
  assert.ok(
    approvalCardSrc.includes("border-0 border-l-2"),
    'ApprovalCard must use border-0 border-l-2 hairline accent'
  )
  assert.ok(
    approvalCardSrc.includes("bg-transparent"),
    'ApprovalCard must blend seamlessly using bg-transparent'
  )
  assert.ok(
    approvalCardSrc.includes("border-l-primary"),
    'ApprovalCard pending state must use border-l-primary'
  )
  assert.ok(
    approvalCardSrc.includes("border-l-2 border-l-emerald-500"),
    'ApprovalCard accepted state must use border-l-2 border-l-emerald-500'
  )
})

test('2. ApprovalCard plan description clamp eliminates hardcoded from-card gradient', () => {
  // Clamped long plan gradient must NOT fade from --card which creates a fake card-colored patch
  assert.ok(
    !approvalCardSrc.includes("from-card/95"),
    'ApprovalCard must eliminate hardcoded from-card/95 gradient'
  )
})

test('3. InboxView eliminates nested speech bubble wrapper for approval messages', () => {
  // Must NOT pass hasSiblingCard={true} for approval_plan
  assert.ok(
    !inboxViewSrc.includes("hasSiblingCard={!!(isAgentMessage && msg.approval_plan)"),
    'InboxView must not pass hasSiblingCard for approval proposals'
  )
})

test('4. ApprovalCard maintains 44px mobile touch targets and accessibility', () => {
  assert.ok(
    approvalCardSrc.includes("min-h-11"),
    'ApprovalCard action buttons must maintain min-h-11 44px touch targets'
  )
  assert.ok(
    approvalCardSrc.includes("data-status={accepted ? 'accepted' : deferred ? 'deferred' : 'pending'}"),
    'ApprovalCard must maintain data-status attribute'
  )
  assert.ok(
    approvalCardSrc.includes("data-testid=\"approval-card\""),
    'ApprovalCard must maintain data-testid="approval-card"'
  )
})
