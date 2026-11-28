import assert from 'node:assert/strict'
import test from 'node:test'
import fs from 'node:fs'
import path from 'node:path'

const webRoot = path.resolve(import.meta.dirname, '..')

test('SecretInputCard implements masked password input, eye toggle, and submit button', () => {
  const src = fs.readFileSync(path.join(webRoot, 'src/features/inbox/SecretInputCard.tsx'), 'utf8')
  assert.match(src, /type=\{revealed \? 'text' : 'password'\}/)
  assert.match(src, /autoComplete="off"/)
  assert.match(src, /data-testid="secret-input-field"/)
  assert.match(src, /data-testid="secret-input-submit"/)
  assert.match(src, /setValue\(''\)/)
})

test('chat.ts and InboxView wire secret_input through dedicated /secret endpoint', () => {
  const chatTs = fs.readFileSync(path.join(webRoot, 'src/api/chat.ts'), 'utf8')
  assert.match(chatTs, /export interface SecretInput/)
  assert.match(chatTs, /secret_input\?: SecretInput/)
  assert.match(chatTs, /\/secret`/)

  const inboxView = fs.readFileSync(path.join(webRoot, 'src/features/inbox/InboxView.tsx'), 'utf8')
  assert.match(inboxView, /<SecretInputCard/)
  assert.match(inboxView, /chatApi\.submitSecret/)

  const reconciliation = fs.readFileSync(path.join(webRoot, 'src/features/inbox/message-reconciliation.ts'), 'utf8')
  assert.match(reconciliation, /a\.secret_input/)
})
