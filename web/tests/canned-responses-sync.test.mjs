import test from 'node:test'
import assert from 'node:assert/strict'
import fs from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'

const __dirname = path.dirname(fileURLToPath(import.meta.url))
const pickerPath = path.join(__dirname, '../src/features/inbox/CannedResponsesPicker.tsx')
const dialogPath = path.join(__dirname, '../src/features/inbox/CannedResponsesDialog.tsx')
const serverPath = path.join(__dirname, '../../internal/server/canned_responses.go')

const pickerSource = fs.readFileSync(pickerPath, 'utf8')
const dialogSource = fs.readFileSync(dialogPath, 'utf8')
const serverSource = fs.readFileSync(serverPath, 'utf8')

test('CannedResponsesPicker refetches on slash activation, tab focus/visibility, and local mutations', () => {
  assert.match(pickerSource, /export const CANNED_RESPONSES_CHANGED_EVENT = 'cuan-canned-responses-changed'/)
  assert.match(pickerSource, /window\.addEventListener\('focus', handleRefresh\)/)
  assert.match(pickerSource, /document\.addEventListener\('visibilitychange', handleRefresh\)/)
  assert.match(pickerSource, /window\.addEventListener\(CANNED_RESPONSES_CHANGED_EVENT, handleRefresh\)/)
  assert.match(pickerSource, /if \(isSlashActive\) \{\s*void fetchResponses\(\)\s*\}/)
  assert.match(pickerSource, /if \(isOpen\) \{\s*void fetchResponses\(\)\s*\}/)
})

test('CannedResponsesDialog notifies picker on save/delete and refreshes on focus while open', () => {
  assert.match(dialogSource, /CANNED_RESPONSES_CHANGED_EVENT/)
  const dispatchMatches = dialogSource.match(/window\.dispatchEvent\(new CustomEvent\(CANNED_RESPONSES_CHANGED_EVENT\)\)/g) || []
  assert.equal(dispatchMatches.length, 2, 'Should dispatch change event after save and delete')
  assert.match(dialogSource, /window\.addEventListener\('focus', handleRefresh\)/)
  assert.match(dialogSource, /document\.addEventListener\('visibilitychange', handleRefresh\)/)
})

test('Backend canned-responses list disables HTTP caching so cross-device updates are always fresh', () => {
  assert.match(serverSource, /w\.Header\(\)\.Set\("Cache-Control", "no-store"\)/)
})
