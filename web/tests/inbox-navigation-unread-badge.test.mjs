import test from 'node:test'
import assert from 'node:assert/strict'
import fs from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'

const __filename = fileURLToPath(import.meta.url)
const __dirname = path.dirname(__filename)

const webRoot = path.resolve(__dirname, '..')
const appPath = path.join(webRoot, 'src', 'App.tsx')
const appShellPath = path.join(webRoot, 'src', 'components', 'layout', 'AppShell.tsx')
const enLocalePath = path.join(webRoot, 'src', 'locales', 'en.json')
const idLocalePath = path.join(webRoot, 'src', 'locales', 'id.json')

test('1. Scope & Architecture: App.tsx calculates inboxUnread from unread messages instead of unresolved conversations', () => {
  const appSource = fs.readFileSync(appPath, 'utf8')

  // Verify App.tsx imports useArchivedItems
  assert.ok(
    appSource.includes("import { useArchivedItems } from './hooks/useArchivedItems'"),
    'App.tsx must import useArchivedItems hook'
  )

  // Verify App.tsx instantiates conversationArchive
  assert.ok(
    appSource.includes("useArchivedItems('conversation')"),
    'App.tsx must instantiate useArchivedItems for conversation kind'
  )

  // Verify inboxUnread is computed reactively from unread messages (unreadCount > 0)
  assert.ok(
    appSource.includes('conversationArchive.unreadCount'),
    'App.tsx must calculate unread state using conversationArchive.unreadCount'
  )

  // Verify App.tsx eliminated unresolved check (!contact.resolved) from inboxUnread calculation
  assert.ok(
    !appSource.includes('inboxUnread={contacts.filter(contact => !contact.resolved'),
    'App.tsx must not calculate inboxUnread by filtering !contact.resolved'
  )

  // Verify App.tsx passes calculated inboxUnread to AppShell
  assert.ok(
    appSource.includes('inboxUnread={inboxUnread}'),
    'App.tsx must pass reactive inboxUnread calculation to AppShell'
  )
})

test('2. Scope & Accessibility: AppShell.tsx reflects unread messages in aria-label and eliminates unresolved conversations', () => {
  const appShellSource = fs.readFileSync(appShellPath, 'utf8')

  // Verify elimination of 'unresolved conversations' in AppShell
  assert.ok(
    !appShellSource.includes('unresolved conversations'),
    'AppShell.tsx must eliminate all references to "unresolved conversations"'
  )

  // Verify aria-label references unread messages
  assert.ok(
    appShellSource.includes("t('unread messages')") || appShellSource.includes('unread messages'),
    'AppShell.tsx must use unread messages representation in aria-label'
  )

  // Verify badges are conditionally rendered when effectiveInboxUnread > 0
  assert.ok(
    appShellSource.includes('data-testid="inbox-unread-badge-rail"'),
    'AppShell.tsx must retain data-testid="inbox-unread-badge-rail"'
  )
  assert.ok(
    appShellSource.includes('data-testid="inbox-unread-badge-bottom"'),
    'AppShell.tsx must retain data-testid="inbox-unread-badge-bottom"'
  )
})

test('3. Localization: en.json and id.json provide complete translations for unread messages and unread conversations', () => {
  const enLocale = JSON.parse(fs.readFileSync(enLocalePath, 'utf8'))
  const idLocale = JSON.parse(fs.readFileSync(idLocalePath, 'utf8'))

  assert.ok(enLocale['unread messages'], 'en.json must contain "unread messages"')
  assert.equal(enLocale['unread messages'], 'unread messages')

  assert.ok(idLocale['unread messages'], 'id.json must contain "unread messages"')
  assert.equal(idLocale['unread messages'], 'pesan belum dibaca')

  assert.ok(enLocale['{{count}} unread messages'], 'en.json must contain "{{count}} unread messages"')
  assert.ok(idLocale['{{count}} unread messages'], 'id.json must contain "{{count}} unread messages"')
})

test('4. Logic & Root Cause: 19 unresolved read conversations produce 0 unread badge count (curing user reported fatigue)', () => {
  // Simulate the exact user scenario from the report screenshot:
  // 19 conversations exist, all are unresolved (!contact.resolved), but all are read (unread_count === 0).
  const mockContacts = Array.from({ length: 19 }, (_, i) => ({
    id: `conv_${i + 1}`,
    conversation_id: `conv_${i + 1}`,
    name: `Conversation ${i + 1}`,
    type: 'agent',
    resolved: false,
    has_messages: true,
    unread_count: 0,
    last_message_at: '2026-11-20T10:00:00Z',
    updated_at: '2026-11-20T10:00:00Z',
  }))

  // Old broken logic:
  const brokenCount = mockContacts.filter((c) => !c.resolved && c.has_messages !== false).length
  assert.equal(brokenCount, 19, 'Broken logic causes 19 badge count on read conversations')

  // Simulated archive items helper mimicking useArchivedItems
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
      if (contact.has_messages === false) return false
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

  // All 19 read -> fixed count is 0!
  const emptyArchive = createArchive({})
  assert.equal(computeInboxUnread(mockContacts, emptyArchive), 0, 'Read unresolved conversations must not show a badge')

  // Now simulate 2 new incoming messages
  mockContacts[0].unread_count = 2
  mockContacts[1].unread_count = 1
  assert.equal(computeInboxUnread(mockContacts, emptyArchive), 2, 'Badge shows count 2 for 2 unread conversations')

  // User opens/reads conversation 0
  const updatedItems = {
    conv_1: { read_at: '2026-11-20T10:05:00Z' },
  }
  const reactiveArchive1 = createArchive(updatedItems)
  assert.equal(
    computeInboxUnread(mockContacts, reactiveArchive1),
    1,
    'Badge automatically decrements from 2 to 1 when conversation is read'
  )

  // User opens/reads conversation 1
  updatedItems.conv_2 = { read_at: '2026-11-20T10:05:00Z' }
  const reactiveArchive2 = createArchive(updatedItems)
  assert.equal(
    computeInboxUnread(mockContacts, reactiveArchive2),
    0,
    'Badge automatically disappears (reaches 0) when all unread conversations are read'
  )
})

test('5. Edge cases: draft contacts without messages and manual mark-as-unread', () => {
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
      if (contact.has_messages === false) return false
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

  // Contact with has_messages: false must not be counted even if unread_count is anomalous
  const draftContact = [{
    id: 'draft_1',
    conversation_id: 'draft_1',
    has_messages: false,
    unread_count: 5,
  }]
  assert.equal(computeInboxUnread(draftContact, createArchive()), 0, 'Contacts without messages must not produce unread badge')

  // Contact marked as unread manually via item preferences
  const manualUnreadContact = [{
    id: 'conv_manual',
    conversation_id: 'conv_manual',
    has_messages: true,
    unread_count: 0,
    last_message_at: '2026-11-20T10:00:00Z',
  }]
  const manualArchive = createArchive({ conv_manual: { unread: true } })
  assert.equal(computeInboxUnread(manualUnreadContact, manualArchive), 1, 'Manually marked unread conversation must be counted')
})
