import { apiFetch } from './client'
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
 request_id: string
 message_id: string
 reply_id: string
 state: 'running' | 'completed' | 'failed' | 'interrupted'
 safe_error?: string
 created_at: string
}

export interface ConversationDetail {
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
    return apiFetch<{ message: ChatMessage; agent_reply?: ChatMessage }>(
      `/api/v1/chat/conversations/${conversationId}/messages`,
      {
        method: 'POST',
        body: JSON.stringify({
          request_id: requestId,
          message,
          content: message,
          author_type: 'user',
          author_name: authorName,
        }),
      }
    )
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
