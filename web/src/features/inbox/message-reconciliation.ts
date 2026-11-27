import type { ChatMessage } from '../../api/chat'

/**
 * Compares two ChatMessage objects for logical equivalence.
 * Returns true if all display and state properties are identical.
 */
export function areMessagesEqual(a: ChatMessage, b: ChatMessage): boolean {
  if (a === b) return true
  if (a.id !== b.id) return false
  if ((a.uuid || undefined) !== (b.uuid || undefined)) return false
  if (a.content !== b.content) return false
  if (a.message !== b.message) return false
  if (a.created_at !== b.created_at) return false
  if (a.updated_at !== b.updated_at) return false
  if (Boolean(a.edited) !== Boolean(b.edited)) return false
  if (a.delivered_at !== b.delivered_at) return false
  if (a.read_confirmed_at !== b.read_confirmed_at) return false
  if (a.sender_type !== b.sender_type) return false
  if (a.author_type !== b.author_type) return false
  if (a.author_id !== b.author_id) return false
  if (a.author_name !== b.author_name) return false
  if (a.product !== b.product) return false
  if (a.attachment_url !== b.attachment_url) return false
  if (a.attachment_type !== b.attachment_type) return false
  if (a.attachment_name !== b.attachment_name) return false
  if (a.attachment_size !== b.attachment_size) return false
  if (a.has_attachment !== b.has_attachment) return false
  if (a.is_internal !== b.is_internal) return false
  if (a.mention_handle !== b.mention_handle) return false
  if (a.reply_draft !== b.reply_draft) return false
  if (a.regenerate_reply_id !== b.regenerate_reply_id) return false
  if (a.plan_deferred !== b.plan_deferred) return false
  if (a.plan_defer_request_id !== b.plan_defer_request_id) return false
  if (a.approval_accepted !== b.approval_accepted) return false
  if (a.approval_request_id !== b.approval_request_id) return false
  if (a.approval_title !== b.approval_title) return false
  if (a.email_delivery?.status !== b.email_delivery?.status) return false
  if (a.email_delivery?.recipient_type !== b.email_delivery?.recipient_type) return false
  if (a.work_update?.kind !== b.work_update?.kind) return false
  if (a.work_update?.task_id !== b.work_update?.task_id) return false
  if (a.work_update?.attempt_id !== b.work_update?.attempt_id) return false
  if (fastJson(a.prompt_choices) !== fastJson(b.prompt_choices)) return false
  if (fastJson(a.approval_plan) !== fastJson(b.approval_plan)) return false
  if (fastJson(a.action_card) !== fastJson(b.action_card)) return false
  if (fastJson(a.receipt_extraction) !== fastJson(b.receipt_extraction)) return false
  if (fastJson(a.attachments) !== fastJson(b.attachments)) return false
  if (fastJson(a.task_draft) !== fastJson(b.task_draft)) return false
  return true
}

function fastJson(val: unknown): string {
  if (val === undefined || val === null) return ''
  try {
    return JSON.stringify(val)
  } catch {
    return String(val)
  }
}

/**
 * Compares two messages for chronological sorting.
 */
export function compareMessages(a: ChatMessage, b: ChatMessage): number {
  if (a.id && b.id && a.id !== b.id) {
    return a.id - b.id
  }
  const timeA = a.created_at ? Date.parse(a.created_at) : 0
  const timeB = b.created_at ? Date.parse(b.created_at) : 0
  if (timeA !== timeB && !Number.isNaN(timeA) && !Number.isNaN(timeB)) {
    return timeA - timeB
  }
  return (a.id || 0) - (b.id || 0)
}

export interface MergeConversationOptions {
  /**
   * When true, `incoming` is treated as an authoritative latest-window snapshot
   * from the server (e.g., GET /conversations/:id or SSE `snapshot` event).
   * Messages in `prev` within the snapshot window (or all of `prev` when
   * `incoming.length < 50`) that are absent from `incoming` are removed.
   */
  authoritativeSnapshot?: boolean
}

/**
 * Reconciles and merges incoming messages into previous message state.
 *
 * 1. Upserts messages matching by ID or UUID.
 * 2. Preserves all older history and existing messages in `prev` (and removes
 *    deleted messages only when `options.authoritativeSnapshot` is true).
 * 3. Appends new messages chronologically.
 * 4. Crucially: returns `prev` unmodified (preserving reference equality)
 *    if no message changed and no new message was added or removed.
 */
export function mergeConversationMessages(
  prev: ChatMessage[],
  incoming: ChatMessage[],
  options?: MergeConversationOptions,
): ChatMessage[] {
  const isSnapshot = Boolean(options?.authoritativeSnapshot)
  if (!incoming || incoming.length === 0) {
    if (isSnapshot) {
      return prev.length === 0 ? prev : []
    }
    return prev
  }
  if (!prev || prev.length === 0) return incoming

  let hasChanges = false
  let updated: ChatMessage[]

  if (isSnapshot) {
    const incomingUuids = new Set<string>()
    const incomingIds = new Set<number>()
    for (const inc of incoming) {
      if (inc.uuid) incomingUuids.add(inc.uuid)
      if (inc.id != null && inc.id !== 0) incomingIds.add(inc.id)
    }

    const windowStart = incoming[0]?.created_at ? Date.parse(incoming[0].created_at) : Number.NaN
    const hasOlderWindow = incoming.length >= 50 && !Number.isNaN(windowStart)

    updated = []
    for (const m of prev) {
      const inIncoming =
        (m.uuid ? incomingUuids.has(m.uuid) : false) ||
        (m.id != null && m.id !== 0 ? incomingIds.has(m.id) : false)
      if (inIncoming) {
        updated.push(m)
        continue
      }
      const mTime = m.created_at ? Date.parse(m.created_at) : Number.NaN
      if (hasOlderWindow && !Number.isNaN(mTime) && mTime < windowStart) {
        updated.push(m)
      } else {
        hasChanges = true
      }
    }
  } else {
    updated = [...prev]
  }

  const byUuid = new Map<string, number>()
  const byId = new Map<number, number>()

  for (let i = 0; i < updated.length; i++) {
    const m = updated[i]
    if (m.uuid) byUuid.set(m.uuid, i)
    if (m.id != null && m.id !== 0) byId.set(m.id, i)
  }

  const toAppend: ChatMessage[] = []

  for (const inc of incoming) {
    let existingIndex: number | undefined
    if (inc.uuid && byUuid.has(inc.uuid)) {
      existingIndex = byUuid.get(inc.uuid)
    } else if (inc.id != null && inc.id !== 0 && byId.has(inc.id)) {
      existingIndex = byId.get(inc.id)
    }

    if (existingIndex !== undefined) {
      const existing = updated[existingIndex]
      if (!areMessagesEqual(existing, inc)) {
        const merged: ChatMessage = { ...existing, ...inc }
        updated[existingIndex] = merged
        hasChanges = true
        if (merged.uuid) byUuid.set(merged.uuid, existingIndex)
        if (merged.id != null && merged.id !== 0) byId.set(merged.id, existingIndex)
      }
    } else {
      // New message
      const targetIndex = updated.length + toAppend.length
      if (inc.uuid) byUuid.set(inc.uuid, targetIndex)
      if (inc.id != null && inc.id !== 0) byId.set(inc.id, targetIndex)
      toAppend.push(inc)
      hasChanges = true
    }
  }

  if (toAppend.length > 0) {
    const combined = [...updated, ...toAppend]
    let isSorted = true
    for (let i = 1; i < combined.length; i++) {
      if (compareMessages(combined[i - 1], combined[i]) > 0) {
        isSorted = false
        break
      }
    }
    if (!isSorted) {
      combined.sort(compareMessages)
    }
    return combined
  }

  return hasChanges ? updated : prev
}
