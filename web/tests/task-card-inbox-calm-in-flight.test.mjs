import test from 'node:test'
import assert from 'node:assert/strict'
import fs from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'

const __filename = fileURLToPath(import.meta.url)
const __dirname = path.dirname(__filename)
const webDir = path.resolve(__dirname, '..')

const inboxViewPath = path.resolve(webDir, 'src/features/inbox/InboxView.tsx')
const taskCardPath = path.resolve(webDir, 'src/features/inbox/TaskCard.tsx')
const inboxViewSrc = fs.readFileSync(inboxViewPath, 'utf8')
const taskCardSrc = fs.readFileSync(taskCardPath, 'utf8')

test('1. InboxView: Elimination of redundant in-flight working bubble & sedang mengeksekusi di background', () => {
  // Must NOT contain the redundant 'sedang mengeksekusi di background' text
  assert.ok(
    !inboxViewSrc.includes('sedang mengeksekusi di background'),
    'InboxView.tsx must completely eliminate the deformed "sedang mengeksekusi di background" text'
  )

  // Must NOT trigger working bubble on activeConversationTasks.length > 0
  assert.ok(
    !inboxViewSrc.includes('activeConversationTasks.length > 0) && (\n                <div className="flex flex-col items-start animate-fade-in my-2" data-testid="inbox-working-indicator">') &&
    !inboxViewSrc.includes('activeConversationTasks.length > 0) && (\n                  <div className="bg-card'),
    'InboxView.tsx must not show duplicate working bubble when tasks are running'
  )

  // Typing indicator must strictly be bounded to sending state only
  assert.ok(
    inboxViewSrc.includes('{sending && activeContact.type === \'agent\' && ('),
    'Typing indicator must only be displayed when actively sending a message'
  )
})

// Task message binding and acknowledgement behavior is covered by
// task-message-bindings.test.mjs and task-launch-text.test.mjs.

test('3. TaskCard: Calm Design and Progressive Disclosure adherence', () => {
  // TaskCard must provide clean working states without visual clutter
  assert.ok(taskCardSrc.includes('data-card="task-card"'), 'TaskCard must have data-card attribute')
  assert.ok(taskCardSrc.includes('isWorking'), 'TaskCard must handle working state cleanly')
  assert.ok(taskCardSrc.includes('data-testid="task-card-live-timer"'), 'TaskCard must include live duration timer')
  assert.ok(taskCardSrc.includes('handleOpenTerminal'), 'TaskCard must provide terminal drawer integration')
})
