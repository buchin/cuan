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
const chatCalmCssPath = path.resolve(webDir, 'src/features/inbox/chat-calm.css')

test('Approval Card Status Cleanup: Header status has no double checkmark character', () => {
  assert.ok(fs.existsSync(approvalCardPath), 'ApprovalCard.tsx must exist')
  const src = fs.readFileSync(approvalCardPath, 'utf8')

  // Header must use CheckCircle2 icon and clean text label without checkmark prefix
  assert.ok(src.includes('CheckCircle2'), 'Header must render CheckCircle2 icon')
  assert.ok(!src.includes("{accepted ? '✓ ' : ''}"), 'Header must not prepend checkmark character')
  assert.ok(
    src.includes("accepted") && src.includes("? 'Approved'") && src.includes("? 'Saved for later'"),
    'Header must render clean status text'
  )
})

test('Approval Card Status Cleanup: Footer eliminates duplicate ✓✓ Approved status', () => {
  const src = fs.readFileSync(approvalCardPath, 'utf8')

  // Footer action buttons must only render when NOT accepted
  assert.ok(src.includes('{!accepted &&'), 'Action buttons must only render when not accepted')
  assert.ok(!src.includes('CheckCheck'), 'CheckCheck (double check) icon must be completely removed')
  assert.ok(!src.includes("{accepted ? '✓ ' : ''}{t(accepted ? 'Approved' : 'Approve and start')}"), 'Redundant accepted footer button must be removed')
})

test('Approval Card Status Cleanup: Chat bubble footer and metadata slot precision', () => {
  const inboxSrc = fs.readFileSync(inboxViewPath, 'utf8')
  const cssSrc = fs.readFileSync(chatCalmCssPath, 'utf8')

  // Message metadata and timestamp slot in chat bubble
  assert.ok(inboxSrc.includes('<div className="wa-markdown-footer">{metadata}</div>'), 'InboxView must render wa-markdown-footer with metadata slot')
  assert.ok(cssSrc.includes('.wa-markdown-footer'), 'chat-calm.css must style wa-markdown-footer')
  assert.ok(cssSrc.includes('.wa-message-meta'), 'chat-calm.css must style wa-message-meta timestamp')
})
