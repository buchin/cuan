import { apiFetch, getAuthToken, ApiError } from './client'
import { DoDItem, Task } from './types'

export interface ChatContact {
  agent_id?: string
  agent_name?: string
  id: string
  type: 'agent' | 'customer'
  name: string
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
}

export interface ChatMessage {
  plan_deferred?: boolean
  plan_defer_request_id?: string
  approval_accepted?: boolean
  approval_request_id?: string
  approval_title?: string
  approval_plan?: ApprovalPlan
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
}

export interface InboxRequest {
  recovery_action?: 'try_again' | 'check_status' | 'check_usage' | 'check_agent' | 'recovering'

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
    const params = new URLSearchParams()
    if (filter?.type && filter.type !== 'all') {
      params.set('type', filter.type)
    }
    if (filter?.q) {
      params.set('q', filter.q)
    }
    const query = params.toString() ? `?${params.toString()}` : ''
    const res = await apiFetch<{ contacts: ChatContact[]; total: number }>(
      `/api/v1/chat/contacts${query}`,
    )
    return res.contacts || []
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
  ): Promise<{ message: ChatMessage; agent_reply?: ChatMessage }> => {
    const controller = new AbortController()
    const timer = setTimeout(() => controller.abort(), 15000)
    try {
      return await apiFetch<{
        message: ChatMessage
        agent_reply?: ChatMessage
      }>(`/api/v1/chat/conversations/${conversationId}/messages`, {
        method: 'POST',
        signal: controller.signal,
        body: JSON.stringify({
          request_id: requestId,
          attachment_ids: attachmentIds,
          approve_message_id: approveMessageId,
          plan_action: planAction,
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
}

export type InboxSnapshot = Pick<
  ConversationDetail,
  'messages' | 'requests' | 'tasks'
>

// Fetch streaming retains the app's Bearer authentication; no token in URLs.
export async function streamConversation(
  id: string,
  signal: AbortSignal,
  onSnapshot: (data: InboxSnapshot) => void,
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
        if (!lines.some((line) => line === 'event: snapshot')) continue
        const payload = lines
          .filter((line) => line.startsWith('data:'))
          .map((line) => line.slice(5).trimStart())
          .join('\n')
        const data = JSON.parse(payload) as InboxSnapshot
        if (Array.isArray(data.messages) && Array.isArray(data.requests))
          onSnapshot(data)
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
