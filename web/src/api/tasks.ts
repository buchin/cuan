import { apiFetch } from './client'
import { Task, CreateTaskRequest, UpdateTaskRequest, TaskStatus } from './types'

export const tasksApi = {
  list: async (projectId?: string): Promise<Task[]> => {
    const url = projectId ? `/api/v1/tasks?project_id=${projectId}` : '/api/v1/tasks'
    const res = await apiFetch<{ tasks: Task[] }>(url)
    return res.tasks || []
  },

  get: async (id: string): Promise<Task> => {
    return apiFetch<Task>(`/api/v1/tasks/${id}`)
  },

  create: async (data: CreateTaskRequest): Promise<Task> => {
    return apiFetch<Task>('/api/v1/tasks', {
      method: 'POST',
      body: JSON.stringify(data),
    })
  },

  update: async (id: string, data: UpdateTaskRequest): Promise<Task> => {
    return apiFetch<Task>(`/api/v1/tasks/${id}`, {
      method: 'PUT',
      body: JSON.stringify(data),
    })
  },

  updateStatus: async (id: string, status: TaskStatus): Promise<Task> => {
    return apiFetch<Task>(`/api/v1/tasks/${id}/status`, {
      method: 'PATCH',
      body: JSON.stringify({ status }),
    })
  },

  delete: async (id: string): Promise<void> => {
    await apiFetch(`/api/v1/tasks/${id}`, {
      method: 'DELETE',
    })
  },

  runAgent: async (taskId: string, payload?: { agent_id?: string; prompt?: string; binary?: string }): Promise<{ session_id?: string; status: string; terminal_url?: string; pid?: number }> => {
    return apiFetch(`/api/v1/tasks/${taskId}/run-agent`, {
      method: 'POST',
      body: JSON.stringify(payload || {}),
    })
  },
}
