import test from 'node:test'
import assert from 'node:assert/strict'
import fs from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'

const __dirname = path.dirname(fileURLToPath(import.meta.url))
const inboxViewPath = path.resolve(__dirname, '../src/features/inbox/InboxView.tsx')
const chatApiPath = path.resolve(__dirname, '../src/api/chat.ts')
const widgetJsPath = path.resolve(__dirname, '../public/chat/widget.js')

test('1. ChatMessage interface declares author_id and sender_type', () => {
  const chatApiSrc = fs.readFileSync(chatApiPath, 'utf8')
  assert.ok(chatApiSrc.includes("sender_type: 'visitor' | 'operator'"), 'ChatMessage must declare sender_type')
  assert.ok(chatApiSrc.includes('author_id?: string'), 'ChatMessage must declare author_id')
})

test('2. InboxView calculates isOutgoing properly for operator in customer rooms', () => {
  const inboxViewSrc = fs.readFileSync(inboxViewPath, 'utf8')
  assert.ok(
    inboxViewSrc.includes('isOperatorInCustomerRoom'),
    'InboxView must declare isOperatorInCustomerRoom'
  )
  assert.ok(
    inboxViewSrc.includes("activeContact.type === 'customer'"),
    'InboxView must check activeContact.type === customer'
  )
  assert.ok(
    inboxViewSrc.includes("msg.sender_type === 'operator'") ||
    inboxViewSrc.includes("msg.author_id === 'operator_user'") ||
    inboxViewSrc.includes("msg.author_name === 'You'"),
    'InboxView must identify operator messages in customer room'
  )
  assert.ok(
    inboxViewSrc.includes("isOutgoing ? 'items-end' : 'items-start'"),
    'InboxView must align outgoing messages to end (right) and incoming to start (left)'
  )
  assert.ok(
    inboxViewSrc.includes("isOutgoing ? 'wa-outgoing' : 'wa-incoming'"),
    'InboxView must set wa-outgoing for outgoing and wa-incoming for incoming'
  )
})

test('3. Alignment simulation: operator vs visitor vs internal agent in customer chat', () => {
  // Simulate the exact logic from InboxView.tsx
  function computeIsOutgoing(msg, activeContact) {
    const isUserInAgentRoom =
      msg.author_type === 'user' && activeContact.type === 'agent'
    const isOperatorInCustomerRoom =
      activeContact.type === 'customer' &&
      (msg.sender_type === 'operator' ||
        msg.author_name === 'You' ||
        msg.author_name === 'Owner' ||
        msg.author_id === 'operator_user') &&
      !(msg.author_type === 'agent' && msg.is_internal) &&
      msg.author_name !== msg.mention_handle &&
      !msg.author_name?.startsWith('@')
    return Boolean(isUserInAgentRoom || isOperatorInCustomerRoom)
  }

  const customerContact = { type: 'customer', id: 'sess-123' }
  const agentContact = { type: 'agent', id: 'agent-sm' }

  // 1. Visitor message in customer chat
  const visitorMsg = {
    id: 1,
    uuid: 'msg-vis',
    author_type: 'user',
    author_id: 'guest-uuid-123',
    author_name: 'Visitor #cbaf02',
    sender_type: 'visitor',
    content: 'Testing',
    is_internal: false,
  }
  assert.equal(
    computeIsOutgoing(visitorMsg, customerContact),
    false,
    'Visitor message must be incoming (left-aligned / items-start)'
  )

  // 2. Operator reply ("Yo man") in customer chat
  const operatorMsg = {
    id: 2,
    uuid: 'msg-op',
    author_type: 'user',
    author_id: 'operator_user',
    author_name: 'You',
    sender_type: 'operator',
    content: 'Yo man',
    is_internal: false,
  }
  assert.equal(
    computeIsOutgoing(operatorMsg, customerContact),
    true,
    'Operator reply "Yo man" must be outgoing (right-aligned / items-end)'
  )

  // 3. Operator whisper mention in customer chat
  const operatorWhisperMsg = {
    id: 3,
    uuid: 'msg-whisper',
    author_type: 'user',
    author_id: 'operator_user',
    author_name: 'You',
    sender_type: 'operator',
    content: '@sm tolong bantu fix problem ini',
    is_internal: true,
    mention_handle: '@sm',
  }
  assert.equal(
    computeIsOutgoing(operatorWhisperMsg, customerContact),
    true,
    'Operator whisper must be outgoing (right-aligned / items-end)'
  )

  // 4. Agent internal reply in customer chat
  const agentInternalReply = {
    id: 4,
    uuid: 'msg-agent',
    author_type: 'agent',
    author_id: 'sm',
    author_name: '@sm',
    sender_type: 'operator',
    content: 'Saya sudah cek issue visitor.',
    is_internal: true,
  }
  assert.equal(
    computeIsOutgoing(agentInternalReply, customerContact),
    false,
    'Agent internal assistant reply must stay incoming (left-aligned / items-start)'
  )

  // 5. In agent chat: user is outgoing, agent is incoming
  assert.equal(
    computeIsOutgoing({ author_type: 'user', content: 'Halo SM' }, agentContact),
    true,
    'User message in agent chat must be outgoing'
  )
  assert.equal(
    computeIsOutgoing({ author_type: 'agent', content: 'Siap bos' }, agentContact),
    false,
    'Agent message in agent chat must be incoming'
  )
})

test('4. widget.js checks sender_type before author_type fallback', () => {
  const widgetJsSrc = fs.readFileSync(widgetJsPath, 'utf8')
  assert.ok(
    widgetJsSrc.includes("m.sender_type ? m.sender_type === 'visitor' : m.author_type === 'user'"),
    'widget.js must prioritize sender_type so operator messages are not mistaken for visitor'
  )
})
