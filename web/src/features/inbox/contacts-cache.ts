import type { ChatContact } from '../../api/chat'

export const INBOX_CONTACTS_CACHE_KEY = 'cuan_inbox_contacts_cache'
const MAX_CACHED_CONTACTS = 200
const MAX_LAST_MESSAGE_LENGTH = 240

export interface CachedContactsSnapshot {
  userId: string
  contacts: ChatContact[]
  cachedAt: number
}

let memoryCache: CachedContactsSnapshot | null = null

export function trimContactSnapshot(contact: ChatContact): ChatContact {
  const lastMessage =
    typeof contact.last_message === 'string' && contact.last_message.length > MAX_LAST_MESSAGE_LENGTH
      ? contact.last_message.slice(0, MAX_LAST_MESSAGE_LENGTH)
      : contact.last_message

  const snapshot: ChatContact = {
    id: contact.id,
    type: contact.type,
    name: contact.name,
    avatar: contact.avatar,
    status: contact.status,
    conversation_id: contact.conversation_id,
    unread_count: contact.unread_count,
    updated_at: contact.updated_at,
  }

  if (contact.contact_id !== undefined) snapshot.contact_id = contact.contact_id
  if (contact.agent_id !== undefined) snapshot.agent_id = contact.agent_id
  if (contact.agent_name !== undefined) snapshot.agent_name = contact.agent_name
  if (contact.product !== undefined) snapshot.product = contact.product
  if (contact.page_title !== undefined) snapshot.page_title = contact.page_title
  if (contact.current_url !== undefined) snapshot.current_url = contact.current_url
  if (contact.is_online !== undefined) snapshot.is_online = contact.is_online
  if (contact.only_system_notices !== undefined) snapshot.only_system_notices = contact.only_system_notices
  if (lastMessage !== undefined) snapshot.last_message = lastMessage
  if (contact.last_message_at !== undefined) snapshot.last_message_at = contact.last_message_at
  if (contact.has_messages !== undefined) snapshot.has_messages = contact.has_messages
  if (contact.message_count !== undefined) snapshot.message_count = contact.message_count
  if (contact.resolved !== undefined) snapshot.resolved = contact.resolved
  if (contact.resolved_at !== undefined) snapshot.resolved_at = contact.resolved_at
  if (contact.resolution_observed_at !== undefined) snapshot.resolution_observed_at = contact.resolution_observed_at
  if (contact.task_count !== undefined) snapshot.task_count = contact.task_count
  if (contact.active_tasks !== undefined) snapshot.active_tasks = contact.active_tasks
  if (contact.review_tasks !== undefined) snapshot.review_tasks = contact.review_tasks
  if (contact.queued_tasks !== undefined) snapshot.queued_tasks = contact.queued_tasks
  if (contact.done_tasks !== undefined) snapshot.done_tasks = contact.done_tasks
  if (contact.canceled_tasks !== undefined) snapshot.canceled_tasks = contact.canceled_tasks
  if (contact.failed_tasks !== undefined) snapshot.failed_tasks = contact.failed_tasks
  if (contact.active_substage !== undefined) snapshot.active_substage = contact.active_substage

  return snapshot
}

export function readInboxContactsCache(userId?: string | null): ChatContact[] | undefined {
  if (!userId) return undefined

  if (memoryCache) {
    if (memoryCache.userId === userId) {
      return memoryCache.contacts
    }
    memoryCache = null
  }

  try {
    if (typeof localStorage === 'undefined') return undefined
    const raw = localStorage.getItem(INBOX_CONTACTS_CACHE_KEY)
    if (!raw) return undefined
    const parsed = JSON.parse(raw) as Partial<CachedContactsSnapshot> | null
    if (!parsed || typeof parsed.userId !== 'string' || !Array.isArray(parsed.contacts)) {
      localStorage.removeItem(INBOX_CONTACTS_CACHE_KEY)
      return undefined
    }
    if (parsed.userId !== userId) {
      // Cross-account isolation: discard snapshot belonging to another user
      localStorage.removeItem(INBOX_CONTACTS_CACHE_KEY)
      return undefined
    }
    const snapshot: CachedContactsSnapshot = {
      userId: parsed.userId,
      contacts: parsed.contacts,
      cachedAt: typeof parsed.cachedAt === 'number' ? parsed.cachedAt : Date.now(),
    }
    memoryCache = snapshot
    return snapshot.contacts
  } catch {
    return undefined
  }
}

export function writeInboxContactsCache(
  userId: string | null | undefined,
  contacts: ChatContact[],
): void {
  if (!userId || !Array.isArray(contacts)) return
  const snapshot: CachedContactsSnapshot = {
    userId,
    contacts: contacts.slice(0, MAX_CACHED_CONTACTS).map(trimContactSnapshot),
    cachedAt: Date.now(),
  }
  memoryCache = snapshot
  try {
    if (typeof localStorage !== 'undefined') {
      localStorage.setItem(INBOX_CONTACTS_CACHE_KEY, JSON.stringify(snapshot))
    }
  } catch {
    // Ignore storage quota or private-browsing write errors
  }
}

export function clearInboxContactsCache(): void {
  memoryCache = null
  try {
    if (typeof localStorage !== 'undefined') {
      localStorage.removeItem(INBOX_CONTACTS_CACHE_KEY)
    }
  } catch {
    // Ignore storage errors
  }
  try {
    if (typeof sessionStorage !== 'undefined') {
      for (let i = sessionStorage.length - 1; i >= 0; i--) {
        const key = sessionStorage.key(i)
        if (key && key.startsWith('cuan_chat_cache:')) {
          sessionStorage.removeItem(key)
        }
      }
    }
  } catch {
    // Ignore storage errors
  }
}
