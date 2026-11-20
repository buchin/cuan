import test from 'node:test'
import assert from 'node:assert/strict'
import fs from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'

const __dirname = path.dirname(fileURLToPath(import.meta.url))
const inboxViewPath = path.resolve(__dirname, '../src/features/inbox/InboxView.tsx')

test('1. InboxView computes isInternalNote to protect incoming visitor replies from whisper styling', () => {
  const inboxViewSrc = fs.readFileSync(inboxViewPath, 'utf8')
  assert.ok(
    inboxViewSrc.includes('const isInternalNote = Boolean('),
    'InboxView must define isInternalNote guard'
  )
  assert.ok(
    inboxViewSrc.includes('isInternalNote && \'wa-whisper\''),
    'Bubble styling wa-whisper must be guarded by isInternalNote'
  )
  assert.ok(
    inboxViewSrc.includes('isInternalNote && !imageBubble'),
    'InternalVisibility badge must be guarded by isInternalNote'
  )
  assert.ok(
    inboxViewSrc.includes('internal={isInternalNote}'),
    'AttachmentFile internal prop must use isInternalNote'
  )
})

test('2. Behavior simulation: visitor GitHub confirmation vs operator whisper vs agent note', () => {
  // InboxView logic simulation
  function computeMessageState(msg, activeContact) {
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

    const isOutgoing = Boolean(isUserInAgentRoom || isOperatorInCustomerRoom)
    const isInternalNote = Boolean(
      msg.is_internal && (activeContact.type !== 'customer' || isOutgoing || msg.author_type === 'agent')
    )

    const bubbleClasses = [
      'wa-message-bubble',
      isInternalNote && 'wa-whisper',
      isOutgoing ? 'wa-outgoing' : 'wa-incoming',
    ].filter(Boolean).join(' ')

    const showsInternalBadge = isInternalNote

    return {
      isOutgoing,
      isInternalNote,
      bubbleClasses,
      showsInternalBadge,
    }
  }

  const customerContact = { type: 'customer', id: 'session-e4497725-59e7-4200-b1ec-d2524c44f61f' }

  // Case A: Visitor reply confirming GitHub username ("username github : gvinum")
  const visitorGithubReply = {
    id: 101,
    uuid: 'msg-vis-gh',
    room_id: customerContact.id,
    author_type: 'user',
    author_id: 'guest-visitor-1',
    author_name: 'Visitor #e44977',
    sender_type: 'visitor',
    content: 'username github : gvinum',
    is_internal: false,
  }
  const stateA = computeMessageState(visitorGithubReply, customerContact)
  assert.equal(stateA.isOutgoing, false, 'Visitor message must be incoming')
  assert.equal(stateA.isInternalNote, false, 'Visitor reply must not be an internal note')
  assert.ok(stateA.bubbleClasses.includes('wa-incoming'), 'Bubble must be wa-incoming')
  assert.ok(!stateA.bubbleClasses.includes('wa-whisper'), 'Visitor reply must NEVER have wa-whisper')
  assert.equal(stateA.showsInternalBadge, false, 'InternalVisibility badge must NOT show for visitor')

  // Case B: Defensive check - even if raw payload had is_internal: true (backend legacy bug scenario)
  const visitorCorruptedPayload = {
    ...visitorGithubReply,
    is_internal: true,
  }
  const stateB = computeMessageState(visitorCorruptedPayload, customerContact)
  assert.equal(stateB.isInternalNote, false, 'Incoming visitor message must reject isInternalNote even if raw flag was true')
  assert.ok(!stateB.bubbleClasses.includes('wa-whisper'), 'Must not render wa-whisper')
  assert.equal(stateB.showsInternalBadge, false, 'Must not render InternalVisibility badge')

  // Case C: Operator internal whisper with @cuan mention
  const operatorWhisper = {
    id: 102,
    uuid: 'msg-op-whisper',
    room_id: customerContact.id,
    author_type: 'user',
    author_id: 'operator_user',
    author_name: 'Owner',
    sender_type: 'operator',
    content: '@cuan tolong siapkan akses repo untuk user ini',
    is_internal: true,
    mention_handle: '@cuan',
  }
  const stateC = computeMessageState(operatorWhisper, customerContact)
  assert.equal(stateC.isOutgoing, true, 'Operator message is outgoing')
  assert.equal(stateC.isInternalNote, true, 'Operator whisper must be recognized as internal note')
  assert.ok(stateC.bubbleClasses.includes('wa-whisper'), 'Operator whisper must have wa-whisper')
  assert.equal(stateC.showsInternalBadge, true, 'Operator whisper must show InternalVisibility badge')

  // Case D: Agent internal note / task cue
  const agentNote = {
    id: 103,
    uuid: 'msg-agent-note',
    room_id: customerContact.id,
    author_type: 'agent',
    author_id: 'agent_cuan',
    author_name: 'Cuan',
    content: 'Repository access verification in progress...',
    is_internal: true,
  }
  const stateD = computeMessageState(agentNote, customerContact)
  assert.equal(stateD.isInternalNote, true, 'Agent internal note must be internal note')
  assert.ok(stateD.bubbleClasses.includes('wa-whisper'), 'Agent internal note must have wa-whisper')
  assert.equal(stateD.showsInternalBadge, true, 'Agent internal note must show InternalVisibility badge')
})
