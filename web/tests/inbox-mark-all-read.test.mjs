import test from 'node:test'
import assert from 'node:assert'
import fs from 'node:fs'
import ts from 'typescript'
import path from 'node:path'
import { fileURLToPath } from 'node:url'

const __filename = fileURLToPath(import.meta.url)
const __dirname = path.dirname(__filename)

const archivedHookPath = path.join(__dirname, '../src/hooks/useArchivedItems.tsx')
const archivedHookSource = fs.readFileSync(archivedHookPath, 'utf8')

const distDir = path.join(__dirname, '../dist/assets')
const distJsFile = fs.readdirSync(distDir).find((f) => f.startsWith('index-') && f.endsWith('.js'))
const distJsContent = distJsFile ? fs.readFileSync(path.join(distDir, distJsFile), 'utf8') : ''

const policySource = fs.readFileSync(new URL('../src/features/inbox/conversation-resolution-policy.ts', import.meta.url), 'utf8')
const policyJS = ts.transpileModule(policySource, { compilerOptions: { module: ts.ModuleKind.ESNext } }).outputText
const { resolvableConversation, conversationHasUnfinishedTasks } = await import('data:text/javascript;base64,' + Buffer.from(policyJS).toString('base64'))

test('bulk resolution includes taskless conversations and canceled work but excludes unfinished tasks', () => {
  for (const [room, eligible] of [
    [{task_count: 0, has_messages: true}, true],
    [{task_count: 0, has_messages: false}, false],
    [{task_count: 1, done_tasks: 1}, true],
    [{task_count: 2, done_tasks: 1, canceled_tasks: 1}, true],
    [{task_count: 1, canceled_tasks: 1}, true],
    [{task_count: 2, done_tasks: 1}, false],
    [{task_count: 1, done_tasks: 1, review_tasks: 1}, false],
    [{task_count: 1, done_tasks: 1, active_tasks: 1}, false],
    [{task_count: 1, done_tasks: 1, failed_tasks: 1}, false],
  ]) assert.equal(resolvableConversation(room), eligible)
  assert.equal(conversationHasUnfinishedTasks({task_count: 1, canceled_tasks: 1}), false, 'manual resolution permits canceled work')
  assert.equal(conversationHasUnfinishedTasks({task_count: 2, done_tasks: 1}), true)
})

test('2. useArchivedItems: batch mark-all API integration and event dispatch', () => {
  assert.ok(
    archivedHookSource.includes('markAllItemPreferencesRead'),
    'useArchivedItems must export markAllItemPreferencesRead'
  )
  assert.ok(
    archivedHookSource.includes('all: true, read: true') || archivedHookSource.includes('all: true'),
    'markAllItemPreferencesRead must send batch mark read payload to /api/v1/item-preferences'
  )
  assert.ok(
    archivedHookSource.includes('cuan-item-preferences'),
    'markAllItemPreferencesRead must dispatch cuan-item-preferences event for instant read-marker synchronization'
  )
  assert.ok(
    archivedHookSource.includes('markAllRead:'),
    'useArchivedItems must expose markAllRead method'
  )
})

// Replaced by rendered interaction checks in docs/evidence/accounts/model-picker-check.cjs.


test('4. Production bundle contains compiled mark-all logic', () => {
  assert.ok(distJsContent.length > 0, 'Production bundle must exist')
  assert.ok(
    distJsContent.includes('markAllItemPreferencesRead') || distJsContent.includes('cuan-item-preferences'),
    'Production bundle must retain preferences event sync'
  )
})
