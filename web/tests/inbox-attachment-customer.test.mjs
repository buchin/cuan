import test from 'node:test'
import assert from 'node:assert/strict'
import fs from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'

const __dirname = path.dirname(fileURLToPath(import.meta.url))
const inboxViewPath = path.resolve(__dirname, '../src/features/inbox/InboxView.tsx')

test('1. InboxView renders AttachmentPicker for customer contacts', () => {
  const inboxViewSrc = fs.readFileSync(inboxViewPath, 'utf8')
  // Should NOT be guarded by activeContact.type === 'agent'
  assert.ok(
    !inboxViewSrc.includes("{activeContact.type === 'agent' && (\n                    <AttachmentPicker"),
    'AttachmentPicker must not be restricted to agent contacts only'
  )
  assert.ok(
    inboxViewSrc.includes('<AttachmentPicker'),
    'InboxView must render AttachmentPicker in the composer capsule'
  )
})

test('2. InboxView renders AttachmentPreviews for customer contacts', () => {
  const inboxViewSrc = fs.readFileSync(inboxViewPath, 'utf8')
  // Should NOT be guarded by activeContact.type === 'agent'
  assert.ok(
    !inboxViewSrc.includes("{activeContact.type === 'agent' && (\n                  <AttachmentPreviews"),
    'AttachmentPreviews must not be restricted to agent contacts only'
  )
  assert.ok(
    inboxViewSrc.includes('<AttachmentPreviews uploads={uploads} disabled={sending} />'),
    'InboxView must render AttachmentPreviews above the input capsule'
  )
})

test('3. InboxView enables image attachments for customer contacts', () => {
  const inboxViewSrc = fs.readFileSync(inboxViewPath, 'utf8')
  assert.ok(
    inboxViewSrc.includes("disableImages={activeContact.type === 'agent' && !contactSupportsVision}"),
    'disableImages must only be true for agent contacts whose model lacks vision'
  )
})

test('4. InboxView preserves attachmentIds when sending in customer rooms', () => {
  const inboxViewSrc = fs.readFileSync(inboxViewPath, 'utf8')
  assert.ok(
    !inboxViewSrc.includes("activeContact.type !== 'agent'"),
    'InboxView must not discard attachmentIds for customer contacts'
  )
})
