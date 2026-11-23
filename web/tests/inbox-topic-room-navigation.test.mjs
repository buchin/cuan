import test from 'node:test'
import assert from 'node:assert/strict'
import fs from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'

const __dirname = path.dirname(fileURLToPath(import.meta.url))
const inboxViewPath = path.join(__dirname, '../src/features/inbox/InboxView.tsx')

test('InboxView: distinguishes topic rooms from direct agent chat and preserves separate convKeys', () => {
  const content = fs.readFileSync(inboxViewPath, 'utf8')

  // Must check isDirectAgentChat before collapsing to agent_id
  assert.ok(
    content.includes('const isDirectAgentChat = Boolean('),
    'InboxView must explicitly detect whether contact is direct agent chat vs a topic room',
  )

  // Must use conversation_id or id for topic rooms
  assert.ok(
    content.includes('convKey = activeContact\n    ? isDirectAgentChat\n      ? activeContact.agent_id || activeContact.id.replace(/^contact_/, \'\')\n      : activeContact.conversation_id || activeContact.id'),
    'InboxView convKey must use conversation_id for topic rooms so navigating between tasks triggers reload',
  )

  // isSameConversation must check isDirect before matching agent_id
  assert.ok(
    content.includes('isDirect ? contact.agent_id : \'\''),
    'isSameConversation must only match agent_id for direct agent chat, never for topic rooms',
  )
})
