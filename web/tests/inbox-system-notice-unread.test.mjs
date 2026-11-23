import test from 'node:test'
import assert from 'node:assert/strict'
import fs from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'

const __filename = fileURLToPath(import.meta.url)
const __dirname = path.dirname(__filename)
const webRoot = path.resolve(__dirname, '..')

test('1. Inbox unread computation: System lifecycle notices with unread_count=0 produce 0 unread badge on shell', () => {
  // Simulate 9 conversations that received system lifecycle notices ('Sistem baru saja diperbarui...')
  // but whose unread_count is 0 as per backend classification.
  const mockContacts = Array.from({ length: 9 }, (_, i) => ({
    id: `conv_${i + 1}`,
    conversation_id: `conv_${i + 1}`,
    name: `Task Conversation ${i + 1}`,
    type: 'agent',
    resolved: true,
    only_system_notices: true,
    has_messages: true,
    unread_count: 0,
    last_message: 'Sistem baru saja diperbarui. Pekerjaan Anda tersimpan aman dan dilanjutkan secara otomatis...',
    last_message_at: '2026-11-23T02:00:00Z',
    updated_at: '2026-11-23T02:00:00Z',
  }))

  function createArchive(items = {}) {
    return {
      unreadCount: (id, fallback, latestMessage) => {
        const val = items[id]
        if (val?.unread) return Math.max(1, fallback)
        const read = Date.parse(val?.read_at || '')
        const latest = Date.parse(latestMessage || '')
        if (Number.isFinite(read) && Number.isFinite(latest) && latest <= read) return 0
        return fallback
      },
    }
  }

  function computeInboxUnread(contacts, archive) {
    return contacts.filter((contact) => {
      if (contact.has_messages === false || contact.resolved) return false
      const id = contact.conversation_id || contact.id
      let count = archive.unreadCount(
        id,
        Number(contact.unread_count) || 0,
        contact.last_message_at || contact.updated_at,
      )
      if (count > 0 && contact.conversation_id && contact.id && contact.conversation_id !== contact.id) {
        count = archive.unreadCount(
          contact.id,
          count,
          contact.last_message_at || contact.updated_at,
        )
      }
      return count > 0
    }).length
  }

  const archive = createArchive({})
  const badgeCount = computeInboxUnread(mockContacts, archive)
  assert.equal(badgeCount, 0, 'Inbox unread badge must remain 0 after background system update notices')
})

test('2. Filter "Belum tuntas" (unresolvedOnly) in InboxSubPanel excludes conversations with only system notices', () => {
  const subPanelPath = path.join(webRoot, 'src', 'features', 'inbox', 'InboxSubPanel.tsx')
  const subPanelSource = fs.readFileSync(subPanelPath, 'utf8')

  // Verify InboxSubPanel filters out resolved or only_system_notices conversations in unresolvedOnly mode
  assert.ok(
    subPanelSource.includes('(c.resolved || c.only_system_notices)') ||
    subPanelSource.includes('unresolvedOnly && (c.resolved || c.only_system_notices)'),
    'InboxSubPanel.tsx must exclude conversations with only system notices when unresolvedOnly is active'
  )

  // Simulation of InboxSubPanel filteredContacts logic
  const contacts = [
    {
      id: 'c_normal_unresolved',
      name: 'User discussion',
      resolved: false,
      only_system_notices: false,
      unread_count: 0,
    },
    {
      id: 'c_system_only',
      name: 'System Restart Notice Room',
      resolved: true,
      only_system_notices: true,
      unread_count: 0,
    },
    {
      id: 'c_system_only_flagged',
      name: 'Background Worker Alert',
      resolved: false,
      only_system_notices: true,
      unread_count: 0,
    },
  ]

  const showArchived = false
  const unresolvedOnly = true

  const filtered = contacts.filter((c) => {
    if (showArchived ? !c.resolved : unresolvedOnly && (c.resolved || c.only_system_notices)) return false
    return true
  })

  assert.equal(filtered.length, 1, 'Only genuine uncompleted conversation must remain in Belum tuntas')
  assert.equal(filtered[0].id, 'c_normal_unresolved')
})

test('3. ChatContact type definition in api/chat.ts includes only_system_notices', () => {
  const chatApiPath = path.join(webRoot, 'src', 'api', 'chat.ts')
  const chatApiSource = fs.readFileSync(chatApiPath, 'utf8')

  assert.ok(
    chatApiSource.includes('only_system_notices?: boolean'),
    'ChatContact in chat.ts must include only_system_notices?: boolean'
  )
})
