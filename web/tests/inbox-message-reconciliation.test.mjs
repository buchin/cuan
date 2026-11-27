import test from 'node:test'
import assert from 'node:assert/strict'
import {
  areMessagesEqual,
  compareMessages,
  mergeConversationMessages,
} from '../src/features/inbox/message-reconciliation.ts'

test('1. areMessagesEqual detects identical and modified messages accurately', () => {
  const base = {
    id: 1,
    uuid: 'msg-1',
    sender_type: 'operator',
    author_type: 'agent',
    author_name: 'Cuan',
    message: 'Hello world',
    content: 'Hello world',
    created_at: '2026-11-15T10:00:00Z',
  }

  // Same reference
  assert.equal(areMessagesEqual(base, base), true)

  // Cloned identical
  assert.equal(areMessagesEqual(base, { ...base }), true)

  // Changed text
  assert.equal(areMessagesEqual(base, { ...base, message: 'Hello updated' }), false)

  // Work update change (e.g. runner progress to result)
  const withProgress = {
    ...base,
    work_update: { task_id: 'task-1', attempt_id: 'att-1', kind: 'progress' },
  }
  const withResult = {
    ...base,
    work_update: { task_id: 'task-1', attempt_id: 'att-1', kind: 'result' },
  }
  assert.equal(areMessagesEqual(withProgress, withResult), false)

  // Approval accepted change
  assert.equal(
    areMessagesEqual(base, { ...base, approval_accepted: true }),
    false,
  )

  // Read confirmed at timestamp
  assert.equal(
    areMessagesEqual(base, { ...base, read_confirmed_at: '2026-11-15T10:01:00Z' }),
    false,
  )
})

test('2. Preserves older history: 50 poller messages do not overwrite older loaded messages', () => {
  // Simulate 100 messages in conversation:
  // Older messages 1..50 (loaded earlier via loadOlderMessages)
  const olderMessages = Array.from({ length: 50 }, (_, i) => ({
    id: i + 1,
    uuid: `msg-${i + 1}`,
    sender_type: 'operator',
    author_type: 'agent',
    author_name: 'Cuan',
    message: `Older message ${i + 1}`,
    created_at: new Date(1700000000000 + (i + 1) * 1000).toISOString(),
  }))

  // Newer messages 51..100
  const newerMessages = Array.from({ length: 50 }, (_, i) => ({
    id: i + 51,
    uuid: `msg-${i + 51}`,
    sender_type: 'operator',
    author_type: 'agent',
    author_name: 'Cuan',
    message: `Newer message ${i + 51}`,
    created_at: new Date(1700000000000 + (i + 51) * 1000).toISOString(),
  }))

  const prev = [...olderMessages, ...newerMessages]
  assert.equal(prev.length, 100)

  // Adaptive poller requests latest 50 messages (which returns newerMessages 51..100)
  const incomingFromPoller = newerMessages.map((m) => ({ ...m }))
  assert.equal(incomingFromPoller.length, 50)

  const merged = mergeConversationMessages(prev, incomingFromPoller)

  // Must retain ALL 100 messages - older messages are never lost!
  assert.equal(merged.length, 100)
  assert.equal(merged[0].id, 1)
  assert.equal(merged[49].id, 50)
  assert.equal(merged[50].id, 51)
  assert.equal(merged[99].id, 100)

  // Reference stability: when incoming messages are identical, prev instance is returned!
  assert.equal(merged, prev)
})

test('3. Reference stability: identical background poll returns same array instance', () => {
  const prev = [
    { id: 1, uuid: 'msg-1', sender_type: 'user', author_type: 'user', author_name: 'Owner', message: 'Task 1', created_at: '2026-11-15T09:00:00Z' },
    { id: 2, uuid: 'msg-2', sender_type: 'operator', author_type: 'agent', author_name: 'Cuan', message: 'Working on it', created_at: '2026-11-15T09:01:00Z' },
  ]

  const incoming = [
    { id: 1, uuid: 'msg-1', sender_type: 'user', author_type: 'user', author_name: 'Owner', message: 'Task 1', created_at: '2026-11-15T09:00:00Z' },
    { id: 2, uuid: 'msg-2', sender_type: 'operator', author_type: 'agent', author_name: 'Cuan', message: 'Working on it', created_at: '2026-11-15T09:01:00Z' },
  ]

  const result = mergeConversationMessages(prev, incoming)
  // Must return EXACT same reference to prevent re-render thrashing and scroll jumping
  assert.equal(result, prev)
})

test('4. In-place updates: updates existing message without dropping older messages', () => {
  const prev = [
    { id: 1, uuid: 'msg-1', sender_type: 'user', author_type: 'user', author_name: 'Owner', message: 'Start task', created_at: '2026-11-15T09:00:00Z' },
    { id: 2, uuid: 'msg-2', sender_type: 'operator', author_type: 'agent', author_name: 'Cuan', message: 'Progress update', work_update: { task_id: 't1', kind: 'progress' }, created_at: '2026-11-15T09:01:00Z' },
  ]

  // Runner completes task, changing work_update.kind to 'result'
  const incoming = [
    { id: 2, uuid: 'msg-2', sender_type: 'operator', author_type: 'agent', author_name: 'Cuan', message: 'Task completed', work_update: { task_id: 't1', kind: 'result' }, created_at: '2026-11-15T09:01:00Z' },
  ]

  const result = mergeConversationMessages(prev, incoming)
  assert.equal(result.length, 2)
  assert.equal(result[0].id, 1) // Older message 1 preserved
  assert.equal(result[1].id, 2)
  assert.equal(result[1].work_update?.kind, 'result')
  assert.equal(result[1].message, 'Task completed')
})

test('5. New message arrival: appends new runner message while preserving older history', () => {
  const prev = [
    { id: 1, uuid: 'msg-1', sender_type: 'user', author_type: 'user', author_name: 'Owner', message: 'Start task', created_at: '2026-11-15T09:00:00Z' },
    { id: 2, uuid: 'msg-2', sender_type: 'operator', author_type: 'agent', author_name: 'Cuan', message: 'In progress', created_at: '2026-11-15T09:01:00Z' },
  ]

  const incoming = [
    { id: 2, uuid: 'msg-2', sender_type: 'operator', author_type: 'agent', author_name: 'Cuan', message: 'In progress', created_at: '2026-11-15T09:01:00Z' },
    { id: 3, uuid: 'msg-3', sender_type: 'operator', author_type: 'agent', author_name: 'Cuan', message: 'Final report ready', created_at: '2026-11-15T09:05:00Z' },
  ]

  const result = mergeConversationMessages(prev, incoming)
  assert.equal(result.length, 3)
  assert.equal(result[0].id, 1)
  assert.equal(result[1].id, 2)
  assert.equal(result[2].id, 3)
  assert.equal(result[2].message, 'Final report ready')
})

test('6. SSE and Poller deduplication: no duplicate messages when both streams deliver', () => {
  const prev = [
    { id: 1, uuid: 'msg-1', sender_type: 'user', author_type: 'user', author_name: 'Owner', message: 'Hi', created_at: '2026-11-15T09:00:00Z' },
  ]

  // SSE event arrives with message 2
  const sseIncoming = [
    { id: 2, uuid: 'msg-2', sender_type: 'operator', author_type: 'agent', author_name: 'Cuan', message: 'Hello from SSE', created_at: '2026-11-15T09:01:00Z' },
  ]
  const afterSSE = mergeConversationMessages(prev, sseIncoming)
  assert.equal(afterSSE.length, 2)

  // Poller loop arrives 2s later with message 2 as well
  const pollerIncoming = [
    { id: 1, uuid: 'msg-1', sender_type: 'user', author_type: 'user', author_name: 'Owner', message: 'Hi', created_at: '2026-11-15T09:00:00Z' },
    { id: 2, uuid: 'msg-2', sender_type: 'operator', author_type: 'agent', author_name: 'Cuan', message: 'Hello from SSE', created_at: '2026-11-15T09:01:00Z' },
  ]
  const afterPoller = mergeConversationMessages(afterSSE, pollerIncoming)

  // No duplicate message 2!
  assert.equal(afterPoller.length, 2)
  assert.equal(afterPoller, afterSSE) // Same reference!
})

test('7. Empty edge cases', () => {
  const prev = [
    { id: 1, uuid: 'msg-1', sender_type: 'user', author_type: 'user', author_name: 'Owner', message: 'Hi', created_at: '2026-11-15T09:00:00Z' },
  ]
  assert.equal(mergeConversationMessages(prev, []), prev)
  assert.equal(mergeConversationMessages([], prev), prev)
})

test('8. Authoritative snapshot removes deleted messages while preserving older history', () => {
  const prev = [
    { id: 1, uuid: 'msg-1', sender_type: 'user', author_type: 'user', author_name: 'Owner', message: 'First', created_at: '2026-11-15T09:00:00Z' },
    { id: 2, uuid: 'msg-2', sender_type: 'user', author_type: 'user', author_name: 'Owner', message: 'To delete', created_at: '2026-11-15T09:01:00Z' },
  ]

  // Snapshot with msg-2 removed
  const afterDelete = mergeConversationMessages(prev, [prev[0]], { authoritativeSnapshot: true })
  assert.equal(afterDelete.length, 1)
  assert.equal(afterDelete[0].uuid, 'msg-1')

  // Snapshot with all messages deleted
  const afterClear = mergeConversationMessages(afterDelete, [], { authoritativeSnapshot: true })
  assert.deepEqual(afterClear, [])

  // 50-message snapshot preserves older loaded messages outside the window
  const older = Array.from({ length: 10 }, (_, i) => ({
    id: i + 1,
    uuid: `old-${i + 1}`,
    sender_type: 'operator',
    author_type: 'agent',
    author_name: 'Cuan',
    message: `Old ${i + 1}`,
    created_at: new Date(1700000000000 + (i + 1) * 1000).toISOString(),
  }))
  const window50 = Array.from({ length: 50 }, (_, i) => ({
    id: i + 11,
    uuid: `win-${i + 11}`,
    sender_type: 'user',
    author_type: 'user',
    author_name: 'Owner',
    message: `Win ${i + 11}`,
    created_at: new Date(1700000000000 + (i + 11) * 1000).toISOString(),
  }))
  const fullPrev = [...older, ...window50]
  // Remove one message inside the window, keep 50 in incoming by shifting window or 49 in incoming
  const windowWithoutOne = window50.filter((m) => m.uuid !== 'win-20')
  const mergedWindow = mergeConversationMessages(fullPrev, windowWithoutOne, { authoritativeSnapshot: true })
  // When < 50 in incoming, snapshot is complete; when >= 50, older messages before windowStart are preserved
  assert.equal(mergedWindow.some((m) => m.uuid === 'win-20'), false)
  // Test >= 50 snapshot preserving older messages
  const window50Plus1 = [
    ...window50.filter((m) => m.uuid !== 'win-20'),
    {
      id: 61,
      uuid: 'win-61',
      sender_type: 'user',
      author_type: 'user',
      author_name: 'Owner',
      message: 'Win 61',
      created_at: new Date(1700000000000 + 61 * 1000).toISOString(),
    },
  ]
  const merged50 = mergeConversationMessages(fullPrev, window50Plus1, { authoritativeSnapshot: true })
  assert.equal(merged50.length, 60) // 10 older + 49 remaining + 1 new
  assert.equal(merged50[0].uuid, 'old-1')
  assert.equal(merged50.some((m) => m.uuid === 'win-20'), false)
  assert.equal(merged50[merged50.length - 1].uuid, 'win-61')
})
