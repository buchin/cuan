import { apiFetch } from './client'
import { DoDItem, Task } from './types'

export interface InboxMessage {
  id: string
  room_id: string
  author_type: 'user' | 'agent' | 'system'
  author_id: string
  author_name: string
  content: string
  reply_to_id?: string
  created_at: string
  updated_at: string
}

export interface InboxThreadItem {
  id: string
  project_id: string
  project_name: string
  title: string
  topic: string
  status: 'unread' | 'in_progress' | 'done'
  last_message?: InboxMessage
  message_count: number
  task_count: number
  active_tasks?: number
  created_at: string
  updated_at: string
}

export interface InboxThreadDetail {
  thread: InboxThreadItem
  messages: InboxMessage[]
  tasks: Task[]
}

export interface CreateThreadRequest {
  title: string
  message: string
  project_id?: string
}

export interface SendMessageRequest {
  content: string
  author_type?: 'user' | 'agent' | 'system'
  author_name?: string
}

export interface ApproveThreadRequest {
  title?: string
  assignee?: string
  dod_checklist?: DoDItem[]
}

export const inboxApi = {
  list: async (filter?: { status?: string; project_id?: string }): Promise<InboxThreadItem[]> => {
    const params = new URLSearchParams()
    if (filter?.status && filter.status !== 'all') {
      params.set('status', filter.status)
    }
    if (filter?.project_id) {
      params.set('project_id', filter.project_id)
    }
    const query = params.toString() ? `?${params.toString()}` : ''
    const res = await apiFetch<{ threads: InboxThreadItem[]; total: number }>(`/api/v1/inbox${query}`)
    return res.threads || []
  },

  get: async (id: string): Promise<InboxThreadDetail> => {
    return apiFetch<InboxThreadDetail>(`/api/v1/inbox/${id}`)
  },

  create: async (data: CreateThreadRequest): Promise<{ thread: InboxThreadItem; messages: InboxMessage[] }> => {
    return apiFetch<{ thread: InboxThreadItem; messages: InboxMessage[] }>('/api/v1/inbox', {
      method: 'POST',
      body: JSON.stringify(data),
    })
  },

  createMessage: async (id: string, data: SendMessageRequest): Promise<InboxMessage> => {
    const res = await apiFetch<{ message: InboxMessage }>(`/api/v1/inbox/${id}/messages`, {
      method: 'POST',
      body: JSON.stringify(data),
    })
    return res.message
  },

  approve: async (id: string, data?: ApproveThreadRequest): Promise<{ status: string; task: Task; message: InboxMessage }> => {
    return apiFetch<{ status: string; task: Task; message: InboxMessage }>(`/api/v1/inbox/${id}/approve`, {
      method: 'POST',
      body: JSON.stringify(data || {}),
    })
  },
}
