import { apiFetch, getAuthToken, ApiError } from './client'
import { DoDItem, Task } from './types'
import type { GroundingTaste } from './taste'
import type { ReceiptExtraction } from './appsSkills'

export type { GroundingTaste }
export type { ReceiptExtraction }

export interface ChatContact {
  resolution_observed_at?: string
  resolved?: boolean
  resolved_at?: string
  contact_id?: string
  agent_id?: string
  agent_name?: string
  id: string
  type: 'agent' | 'customer'
  name: string
  email?: string
  avatar: string
  status: 'online' | 'idle' | 'offline'
  product?: string
  guest_uuid?: string
  conversation_id: string
  unread_count: number
  last_message?: string
  last_message_at?: string
  updated_at: string
  has_messages?: boolean
  message_count?: number
  is_online?: boolean
  current_url?: string
  page_title?: string
  task_count?: number
  active_tasks?: number
  review_tasks?: number
  queued_tasks?: number
  done_tasks?: number
  canceled_tasks?: number
  failed_tasks?: number
}

export interface AgentActionCard {
  task_title: string
  dod_checklist: DoDItem[]
  worker_agent: string
  recommended: boolean
  approve_url?: string
  execution_state?: 'ready' | 'running' | 'completed'
}

export interface InboxTaskDraft {
  title: string
  description: string
  project_id?: string
}

export interface ApprovalPlan {
  title: string
  description: string
  project_id?: string
  project_name?: string
  continuation?: { task_id: string; attempt_id: string }
}

export interface PromptOption {
  id: string
  label: string
  variant?: 'primary' | 'outline' | 'ghost'
}

export interface PromptChoices {
  question?: string
  options: PromptOption[]
  allow_other?: boolean
}

export interface ChatMessage {
  work_update?: { task_id: string; attempt_id?: string; kind: 'progress' | 'files' | 'result' }
  regenerate_reply_id?: string
  delivered_at?: string
  read_confirmed_at?: string
  email_delivery?: { status: string; recipient_type: 'operator' | 'visitor' }
  plan_deferred?: boolean
  plan_defer_request_id?: string
  approval_accepted?: boolean
  approval_request_id?: string
  approval_title?: string
  approval_plan?: ApprovalPlan
  prompt_choices?: PromptChoices
  attachments?: import('../features/inbox/Attachments').Attachment[]
  task_draft?: InboxTaskDraft
  id: number
  uuid?: string
  room_id?: string
  chat_session_uuid?: string
  sender_type: 'visitor' | 'operator'
  author_type: 'user' | 'agent' | 'system'
  author_id?: string
  author_name: string
  message: string
  content?: string
  product?: string
  attachment_url?: string
  attachment_type?: string
  attachment_name?: string
  attachment_size?: number
  has_attachment?: boolean
  action_card?: AgentActionCard
  is_internal?: boolean
  mention_handle?: string
  reply_draft?: string
  created_at: string
  grounding_tastes?: GroundingTaste[]
  receipt_extraction?: ReceiptExtraction
}

export type Message = ChatMessage

export interface InboxRequest {
  recovery_action?: 'try_again' | 'continue_analysis' | 'check_status' | 'check_usage' | 'check_agent' | 'recovering'

  agent_id?: string
  request_id: string
  message_id: string
  reply_id: string
  state:
    | 'queued'
    | 'running'
    | 'cancelling'
    | 'cancelled'
    | 'completed'
    | 'failed'
    | 'interrupted'
  started_at?: string
  steps?: { seq: number; kind: string; text: string; at: string }[]
  safe_error?: string
  created_at: string
}

export interface ConversationDetail {
  tasks?: Task[]
  requests?: InboxRequest[]
  conversation_id: string
  contact: ChatContact
  messages: ChatMessage[]
  has_more?: boolean
}

export interface SendChatMessageRequest {
  message: string
  content?: string
  author_type?: 'user' | 'agent' | 'system'
  author_name?: string
}

export interface CannedResponse {
  id: string
  shortcut: string
  title: string
  content: string
  category: string
  created_at: string
  updated_at: string
}

export interface CreateCannedResponseRequest {
  shortcut: string
  title: string
  content: string
  category?: string
}

export interface UpdateCannedResponseRequest {
  shortcut?: string
  title?: string
  content?: string
  category?: string
}

export interface ApproveThreadRequest {
  title?: string
  assignee?: string
  dod_checklist?: DoDItem[]
}

export const chatApi = {
  listCannedResponses: async (filter?: {
    q?: string
    category?: string
  }): Promise<CannedResponse[]> => {
    const params = new URLSearchParams()
    if (filter?.q) params.set('q', filter.q)
    if (filter?.category && filter.category !== 'all')
      params.set('category', filter.category)
    const query = params.toString() ? `?${params.toString()}` : ''
    const res = await apiFetch<{
      canned_responses: CannedResponse[]
      total: number
    }>(`/api/v1/chat/canned-responses${query}`)
    return res.canned_responses || []
  },

  getCannedResponse: async (id: string): Promise<CannedResponse> => {
    return apiFetch<CannedResponse>(`/api/v1/chat/canned-responses/${id}`)
  },

  createCannedResponse: async (
    data: CreateCannedResponseRequest,
  ): Promise<CannedResponse> => {
    return apiFetch<CannedResponse>('/api/v1/chat/canned-responses', {
      method: 'POST',
      body: JSON.stringify(data),
    })
  },

  updateCannedResponse: async (
    id: string,
    data: UpdateCannedResponseRequest,
  ): Promise<CannedResponse> => {
    return apiFetch<CannedResponse>(`/api/v1/chat/canned-responses/${id}`, {
      method: 'PUT',
      body: JSON.stringify(data),
    })
  },

  deleteCannedResponse: async (id: string): Promise<{ ok: boolean }> => {
    return apiFetch<{ ok: boolean }>(`/api/v1/chat/canned-responses/${id}`, {
      method: 'DELETE',
    })
  },
  listContacts: async (filter?: {
    type?: string
    q?: string
  }): Promise<ChatContact[]> => {
    const params = new URLSearchParams({ view: 'list' })
    if (filter?.type && filter.type !== 'all') {
      params.set('type', filter.type)
    }
    if (filter?.q) {
      params.set('q', filter.q)
    }
    const query = params.toString() ? `?${params.toString()}` : ''
    const res = await apiFetch<{
      contacts?: ChatContact[]
      conversations?: ChatContact[]
      threads?: ChatContact[]
      total?: number
    }>(`/api/v1/chat/contacts${query}`)
    const list = res.contacts || res.conversations || res.threads || []
    return list.map((c) => ({
      ...c,
      task_count: typeof c.task_count === 'number' ? c.task_count : Number(c.task_count) || 0,
      active_tasks: typeof c.active_tasks === 'number' ? c.active_tasks : Number(c.active_tasks) || 0,
      review_tasks: Number(c.review_tasks) || 0,
      queued_tasks: typeof c.queued_tasks === 'number' ? c.queued_tasks : Number(c.queued_tasks) || 0,
      done_tasks: typeof c.done_tasks === 'number' ? c.done_tasks : Number(c.done_tasks) || 0,
      canceled_tasks: Number(c.canceled_tasks) || 0,
      failed_tasks: typeof c.failed_tasks === 'number' ? c.failed_tasks : Number(c.failed_tasks) || 0,
    }))
  },

  getConversation: async (
    id: string,
    params?: { limit?: number; before_id?: string; all?: boolean },
  ): Promise<ConversationDetail> => {
    const q = new URLSearchParams()
    if (params?.limit) q.set('limit', String(params.limit))
    if (params?.before_id) q.set('before_id', params.before_id)
    if (params?.all) q.set('all', '1')
    const query = q.toString() ? `?${q.toString()}` : ''
    return apiFetch<ConversationDetail>(
      `/api/v1/chat/conversations/${id}/messages${query}`,
    )
  },

  sendMessage: async (
    conversationId: string,
    message: string,
    authorName = 'You',
    requestId?: string,
    attachmentIds?: string[],
    approveMessageId?: string,
    planAction?: 'approve' | 'defer',
    regenerateReplyId?: string,
  ): Promise<{ request?: InboxRequest; user_message_saved?: boolean; message?: ChatMessage; agent_reply?: ChatMessage }> => {
    const controller = new AbortController()
    const timer = setTimeout(() => controller.abort(), 15000)
    try {
      return await apiFetch<{
        request?: InboxRequest
        user_message_saved?: boolean
        message?: ChatMessage
        agent_reply?: ChatMessage
      }>(`/api/v1/chat/conversations/${conversationId}/messages`, {
        method: 'POST',
        signal: controller.signal,
        body: JSON.stringify({
          request_id: requestId,
          attachment_ids: attachmentIds,
          approve_message_id: approveMessageId,
          plan_action: planAction,
          regenerate_reply_id: regenerateReplyId,
          message,
          content: message,
          author_type: 'user',
          author_name: authorName,
        }),
      })
    } finally {
      clearTimeout(timer)
    }
  },

  approve: async (
    threadId: string,
    data?: ApproveThreadRequest,
  ): Promise<{ status: string; task: Task; message: ChatMessage }> => {
    return apiFetch<{ status: string; task: Task; message: ChatMessage }>(
      `/api/v1/inbox/${threadId}/approve`,
      {
        method: 'POST',
        body: JSON.stringify(data || {}),
      },
    )
  },

  updateTitle: async (
    conversationId: string,
    title: string,
  ): Promise<{ id: string; title: string }> => {
    return apiFetch<{ id: string; title: string }>(
      `/api/v1/chat/conversations/${encodeURIComponent(conversationId)}`,
      {
        method: 'PATCH',
        body: JSON.stringify({ title }),
      },
    )
  },
}

export type InboxSnapshot = Pick<
  ConversationDetail,
  'messages' | 'requests' | 'tasks'
> & {
  room_id?: string
  room_title?: string
}

export interface ChatDeltaPayload {
  request_id: string
  delta: string
}

// Fetch streaming retains the app's Bearer authentication; no token in URLs.
export async function streamConversation(
  id: string,
  signal: AbortSignal,
  onSnapshot: (data: InboxSnapshot) => void,
  onDelta?: (data: ChatDeltaPayload) => void,
): Promise<void> {
  const connection = new AbortController()
  const abort = () => connection.abort()
  signal.addEventListener('abort', abort, { once: true })
  if (signal.aborted) abort()
  // Heartbeats arrive every 10s. A silent half-open connection must reconnect
  // rather than leaving the UI permanently convinced that it is live.
  let idle = setTimeout(abort, 25000)
  let reader: ReadableStreamDefaultReader<Uint8Array> | undefined
  try {
    const response = await fetch(
      `/api/v1/chat/conversations/${encodeURIComponent(id)}/events`,
      {
        headers: {
          Authorization: `Bearer ${getAuthToken()}`,
          Accept: 'text/event-stream',
        },
        signal: connection.signal,
      },
    )
    if (!response.ok)
      throw new ApiError(
        `Chat connection failed (${response.status})`,
        response.status,
      )
    if (
      !response.body ||
      !response.headers.get('content-type')?.includes('text/event-stream')
    )
      throw new Error('Streaming is unavailable')
    reader = response.body.getReader()
    const decoder = new TextDecoder()
    let buffer = ''
    while (true) {
      const { value, done } = await reader.read()
      if (done) break
      clearTimeout(idle)
      idle = setTimeout(abort, 25000)
      buffer = (buffer + decoder.decode(value, { stream: true })).replace(
        /\r\n/g,
        '\n',
      )
      if (buffer.length > 8 * 1024 * 1024)
        throw new Error('Conversation snapshot is too large')
      let split: number
      while ((split = buffer.indexOf('\n\n')) >= 0) {
        const frame = buffer.slice(0, split)
        buffer = buffer.slice(split + 2)
        const lines = frame.split('\n')
        if (lines.some((line) => line === 'event: chat.delta')) {
          const payload = lines
            .filter((line) => line.startsWith('data:'))
            .map((line) => line.slice(5).trimStart())
            .join('\n')
          try {
            const data = JSON.parse(payload) as ChatDeltaPayload
            if (data?.request_id && typeof data.delta === 'string') {
              onDelta?.(data)
            }
          } catch {}
          continue
        }
        if (!lines.some((line) => line === 'event: snapshot')) continue
        const payload = lines
          .filter((line) => line.startsWith('data:'))
          .map((line) => line.slice(5).trimStart())
          .join('\n')
        try {
          const data = JSON.parse(payload) as InboxSnapshot
          if (Array.isArray(data.messages) && Array.isArray(data.requests))
            onSnapshot(data)
        } catch {}
      }
    }
  } finally {
    clearTimeout(idle)
    signal.removeEventListener('abort', abort)
    if (reader) {
      await reader.cancel().catch(() => {})
      reader.releaseLock()
    }
  }
}

export async function cancelInboxRequest(
  conversationId: string,
  requestId: string,
): Promise<void> {
  await apiFetch(
    `/api/v1/chat/conversations/${encodeURIComponent(conversationId)}/requests/${encodeURIComponent(requestId)}/cancel`,
    { method: 'POST' },
  )
}

export async function retryInboxRequest(conversationId: string, requestId: string) {
  return apiFetch<{ request: InboxRequest }>(
    `/api/v1/chat/conversations/${encodeURIComponent(conversationId)}/requests/${encodeURIComponent(requestId)}/retry`,
    { method: 'POST' },
  )
}

export interface SendCustomerEmailPayload {
  recipient?: string
  subject?: string
  draft?: string
  content?: string
  idempotency_key?: string
}

export interface SendCustomerEmailResponse {
  status: string
  email_id: string
  delivery_status: string
  recipient?: string
  message?: string
  already_sent?: boolean
}

export async function sendCustomerReplyEmail(
  conversationId: string,
  messageId: string,
  payload: SendCustomerEmailPayload = {}
): Promise<SendCustomerEmailResponse> {
  return apiFetch<SendCustomerEmailResponse>(
    `/api/v1/chat/conversations/${encodeURIComponent(conversationId)}/messages/${encodeURIComponent(messageId)}/email`,
    {
      method: 'POST',
      body: JSON.stringify(payload),
    }
  )
}

export async function sendConversationEmail(
  conversationId: string,
  payload: SendCustomerEmailPayload & { message_id?: string }
): Promise<SendCustomerEmailResponse> {
  return apiFetch<SendCustomerEmailResponse>(
    `/api/v1/chat/conversations/${encodeURIComponent(conversationId)}/send-email`,
    {
      method: 'POST',
      body: JSON.stringify(payload),
    }
  )
}
