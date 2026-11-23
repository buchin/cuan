import type { Task } from '../../api/types'

export interface CachedConversation {
  messages: any[]
  requests: any[]
  tasks: Task[]
  hasMore?: boolean
  cachedAt: number
}

// SWR conversation cache: eliminates blank screen and loading delays when switching conversations
export const conversationCache = new Map<string, CachedConversation>()

export function getCachedConversation(convId: string): CachedConversation | undefined {
  if (!convId) return undefined
  const memory = conversationCache.get(convId)
  if (memory && memory.messages?.length > 0) return memory
  try {
    const raw = sessionStorage.getItem('cuan_chat_cache:' + convId)
    if (raw) {
      const parsed = JSON.parse(raw) as CachedConversation
      if (parsed && Array.isArray(parsed.messages) && parsed.messages.length > 0) {
        conversationCache.set(convId, parsed)
        return parsed
      }
    }
  } catch {}
  return undefined
}

export function setCachedConversation(convId: string, data: CachedConversation) {
  if (!convId) return
  conversationCache.set(convId, data)
  try {
    const trimmed: CachedConversation = {
      ...data,
      messages: (data.messages || []).slice(-30),
      requests: (data.requests || []).slice(-5),
    }
    sessionStorage.setItem('cuan_chat_cache:' + convId, JSON.stringify(trimmed))
  } catch {}
}
