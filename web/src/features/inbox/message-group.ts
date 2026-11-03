import type { ChatMessage } from '../../api/chat'

type GroupMessage = Pick<ChatMessage, 'author_type' | 'author_id' | 'author_name' | 'sender_type' | 'is_internal' | 'created_at'>

/** A new speaker, visibility, day or five-minute gap starts a speech group. */
export function startsMessageGroup(current: GroupMessage, previous?: GroupMessage): boolean {
  if (!previous) return true
  const sender = (m: GroupMessage) => [m.author_type, m.author_id || m.author_name || m.sender_type, !!m.is_internal].join(':')
  if (sender(current) !== sender(previous)) return true
  const now = new Date(current.created_at), before = new Date(previous.created_at)
  const elapsed = now.getTime() - before.getTime()
  return !Number.isFinite(elapsed) || elapsed < 0 || elapsed >= 5 * 60_000 || now.toDateString() !== before.toDateString()
}
