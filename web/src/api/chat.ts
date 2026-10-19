import { apiFetch, getAuthToken, ApiError } from './client'
import { DoDItem, Task } from './types'

export interface ChatContact {
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
}

export interface AgentActionCard {
  task_title: string
  dod_checklist: DoDItem[]
  worker_agent: string
  recommended: boolean
  approve_url?: string
  execution_state?: 'ready' | 'running' | 'completed'
}

export interface ChatMessage {
  id: number
  uuid?: string
  room_id?: string
  chat_session_uuid?: string
  sender_type: 'visitor' | 'operator'
  author_type: 'user' | 'agent' | 'system'
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
  created_at: string
}

export interface InboxRequest {
 agent_id?: string
 request_id: string
 message_id: string
 reply_id: string
 state: 'queued' | 'running' | 'cancelling' | 'cancelled' | 'completed' | 'failed' | 'interrupted'
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
}

export interface SendChatMessageRequest {
  message: string
  content?: string
  author_type?: 'user' | 'agent' | 'system'
  author_name?: string
}

export interface ApproveThreadRequest {
  title?: string
  assignee?: string
  dod_checklist?: DoDItem[]
}

export const chatApi = {
  listContacts: async (filter?: { type?: string; q?: string }): Promise<ChatContact[]> => {
    const params = new URLSearchParams()
    if (filter?.type && filter.type !== 'all') {
      params.set('type', filter.type)
    }
    if (filter?.q) {
      params.set('q', filter.q)
    }
    const query = params.toString() ? `?${params.toString()}` : ''
    const res = await apiFetch<{ contacts: ChatContact[]; total: number }>(`/api/v1/chat/contacts${query}`)
    return res.contacts || []
  },

  getConversation: async (id: string): Promise<ConversationDetail> => {
    return apiFetch<ConversationDetail>(`/api/v1/chat/conversations/${id}/messages`)
  },

  sendMessage: async (
    conversationId: string,
    message: string,
    authorName = 'Buchin',
    requestId?: string
  ): Promise<{ message: ChatMessage; agent_reply?: ChatMessage }> => {
    const controller = new AbortController()
    const timer = setTimeout(() => controller.abort(), 15000)
    try { return await apiFetch<{ message: ChatMessage; agent_reply?: ChatMessage }>(
      `/api/v1/chat/conversations/${conversationId}/messages`,
      {
        method: 'POST',
        signal: controller.signal,
        body: JSON.stringify({
          request_id: requestId,
          message,
          content: message,
          author_type: 'user',
          author_name: authorName,
        }),
      }
    )
    } finally { clearTimeout(timer) }
  },

  approve: async (
    threadId: string,
    data?: ApproveThreadRequest
  ): Promise<{ status: string; task: Task; message: ChatMessage }> => {
    return apiFetch<{ status: string; task: Task; message: ChatMessage }>(`/api/v1/inbox/${threadId}/approve`, {
      method: 'POST',
      body: JSON.stringify(data || {}),
    })
  },
}

export type InboxSnapshot = Pick<ConversationDetail, 'messages' | 'requests' | 'tasks'>

// Fetch streaming retains the app's Bearer authentication; no token in URLs.
export async function streamConversation(id: string, signal: AbortSignal, onSnapshot: (data: InboxSnapshot) => void): Promise<void> {
  const connection = new AbortController()
  const abort = () => connection.abort()
  signal.addEventListener('abort', abort, { once: true })
  if (signal.aborted) abort()
  // Heartbeats arrive every 10s. A silent half-open connection must reconnect
  // rather than leaving the UI permanently convinced that it is live.
  let idle = setTimeout(abort, 25000)
  let reader: ReadableStreamDefaultReader<Uint8Array> | undefined
  try {
    const response = await fetch(`/api/v1/chat/conversations/${encodeURIComponent(id)}/events`, {
      headers: { Authorization: `Bearer ${getAuthToken()}`, Accept: 'text/event-stream' }, signal: connection.signal,
    })
    if (!response.ok) throw new ApiError(`Koneksi chat gagal (${response.status})`, response.status)
    if (!response.body || !response.headers.get('content-type')?.includes('text/event-stream')) throw new Error('Streaming tidak tersedia')
    reader = response.body.getReader()
    const decoder = new TextDecoder()
    let buffer = ''
    while (true) {
      const { value, done } = await reader.read()
      if (done) break
      clearTimeout(idle); idle = setTimeout(abort, 25000)
      buffer = (buffer + decoder.decode(value, { stream: true })).replace(/\r\n/g, '\n')
      if (buffer.length > 8 * 1024 * 1024) throw new Error('Snapshot terlalu besar')
      let split: number
      while ((split = buffer.indexOf('\n\n')) >= 0) {
        const frame = buffer.slice(0, split); buffer = buffer.slice(split + 2)
        const lines = frame.split('\n')
        if (!lines.some(line => line === 'event: snapshot')) continue
        const payload = lines.filter(line => line.startsWith('data:')).map(line => line.slice(5).trimStart()).join('\n')
        const data = JSON.parse(payload) as InboxSnapshot
        if (Array.isArray(data.messages) && Array.isArray(data.requests)) onSnapshot(data)
      }
    }
  } finally {
    clearTimeout(idle); signal.removeEventListener('abort', abort)
    if (reader) { await reader.cancel().catch(() => {}); reader.releaseLock() }
  }
}

export async function cancelInboxRequest(conversationId: string, requestId: string): Promise<void> {
  await apiFetch(`/api/v1/chat/conversations/${encodeURIComponent(conversationId)}/requests/${encodeURIComponent(requestId)}/cancel`, { method: 'POST' })
}
