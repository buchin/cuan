import test from 'node:test'
import assert from 'node:assert/strict'
import fs from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'
import ts from 'typescript'

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
    /<AttachmentPreviews\b[^>]*uploads=\{uploads\}[^>]*disabled=\{sending\}/.test(inboxViewSrc),
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
  const source = ts.createSourceFile('InboxView.tsx', inboxViewSrc, ts.ScriptTarget.Latest, true, ts.ScriptKind.TSX)
  const initializers = []
  const visit = node => {
    if (ts.isVariableDeclaration(node) && node.name.getText(source) === 'request' && node.initializer) {
      const findAttachments = child => {
        if (ts.isPropertyAssignment(child) && child.name.getText(source) === 'attachmentIds') initializers.push(child.initializer.getText(source))
        ts.forEachChild(child, findAttachments)
      }
      findAttachments(node.initializer)
    }
    ts.forEachChild(node, visit)
  }
  visit(source)
  assert.equal(initializers.length, 1, 'Locate the actual outgoing request attachment expression')
  const collect = new Function('activeContact', 'approveMessageId', 'overrideText', 'uploads', `return (${initializers[0]})`)
  for (const type of ['customer', 'agent']) {
    assert.deepEqual(collect({type}, undefined, undefined, {items:[{id:'image-a'},{id:'file-b'}]}), ['image-a','file-b'])
    assert.deepEqual(collect({type}, 'approval', undefined, {items:[{id:'unsent'}]}), [])
    assert.deepEqual(collect({type}, undefined, 'override', {items:[{id:'unsent'}]}), [])
  }
})
