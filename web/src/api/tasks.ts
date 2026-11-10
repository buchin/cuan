import { apiFetch } from './client'
import type {
  Task,
  TaskDetail,
  TaskActivity,
  TaskAttempt,
  CreateTaskRequest,
  UpdateTaskRequest,
  TaskStatus,
} from './types'

function normalize(
  raw: Task & { instructions?: string; assigned_agent?: string },
): Task {
  const state = raw.latest_attempt?.status
  return {
    ...raw,
    execution_status:
      state === 'succeeded' ? 'completed' : state || raw.execution_status,
    description: (raw.description ?? raw.instructions ?? raw.body ?? '').split('\n\nProject guidance snapshot')[0],
    assignee_id: raw.assignee_id ?? raw.assigned_agent ?? raw.assignee ?? '',
    dod_checklist: (raw.dod_checklist || []).map((item) =>
      typeof item === 'string' ? { text: item, completed: false } : item,
    ),
  }
}
const path = (id: string) => `/api/v1/tasks/${encodeURIComponent(id)}`
export interface TaskLog {
  id: number
  stream: string
  line: string
  logged_at: string
}
export interface LiveTaskSession {
  session_id: string
  attempt_id: string
  agent_id: string
  started_at: string
}
export interface RunTaskResult {
  status: string
  session_id?: string
  replayed?: boolean
  attempt?: TaskAttempt
}
export interface RunTaskRequest {
  request_id: string
  agent_id: string
  revision?: string
  cols?: number
  rows?: number
  prompt?: string
  binary?: string
}

export function estimateViewportTerminalDimensions(): { cols: number; rows: number } {
  if (typeof window === 'undefined') return { cols: 80, rows: 24 }
  const isMobile = window.innerWidth < 480
  const charWidth = isMobile ? 7.2 : 7.8
  const lineHeight = isMobile ? 14.5 : 16.25
  const paddingX = isMobile ? 12 : 32
  const availableWidth = Math.max(200, window.innerWidth - paddingX)
  const availableHeight = Math.max(200, window.innerHeight - 180)
  const cols = Math.max(30, Math.min(200, Math.floor(availableWidth / charWidth)))
  const rows = Math.max(10, Math.min(80, Math.floor(availableHeight / lineHeight)))
  return { cols, rows }
}
export interface TaskPageOptions {
  exclude_terminals?: boolean; project_id?: string; search?: string; status?: string; priority?: string; sort?: string; page?: number; page_size?: number
}
export interface TaskPage {
  tasks: Task[]; total: number; scope_total: number; counts: Record<string, number>; page: number; page_size: number
}
export const tasksApi = {
  page: async (options: TaskPageOptions, signal?: AbortSignal): Promise<TaskPage> => {
    const query = new URLSearchParams({ view: 'board' })
    for (const [key,value] of Object.entries(options)) if (value !== undefined && value !== '' && value !== 'all') query.set(key,String(value))
    const res = await apiFetch<TaskPage>('/api/v1/tasks?' + query.toString(), { signal })
    if (!Number.isInteger(res.total) || res.total < 0 || !Number.isInteger(res.page) || res.page < 1 || !Number.isInteger(res.page_size) || res.page_size < 1 || res.page_size > 100 || !res.counts || !Array.isArray(res.tasks) || res.tasks.length > res.page_size) throw new Error('Invalid task page response')
    return { ...res, tasks: (res.tasks || []).map(normalize) }
  },
  list: async (projectId?: string): Promise<Task[]> => {
    const res = await apiFetch<{ tasks: Task[] }>(
      `/api/v1/tasks${projectId ? '?project_id=' + encodeURIComponent(projectId) : ''}`,
    )
    return (res.tasks || []).map(normalize)
  },
  get: async (id: string): Promise<TaskDetail> => {
    const raw = await apiFetch<TaskDetail & { task?: Task }>(path(id))
    return {
      ...normalize(raw.task || raw),
      attempts: raw.attempts || [],
      activities: raw.activities || [],
    }
  },
  create: async (data: CreateTaskRequest): Promise<Task> =>
    normalize(
      await apiFetch<Task>('/api/v1/tasks', {
        method: 'POST',
        body: JSON.stringify({
          ...data,
          instructions: data.description || '',
          assigned_agent: data.assignee_id ?? data.assignee ?? '',
        }),
      }),
    ),
  update: async (id: string, data: UpdateTaskRequest): Promise<Task> =>
    normalize(
      await apiFetch<Task>(path(id), {
        method: 'PUT',
        body: JSON.stringify({
          ...data,
          instructions: data.description,
          assigned_agent: data.assignee_id ?? data.assignee,
        }),
      }),
    ),
  updateStatus: async (id: string, status: TaskStatus): Promise<Task> =>
    tasksApi.update(id, { status }),
  delete: async (id: string): Promise<void> => {
    await apiFetch(path(id), { method: 'DELETE' })
  },
  runAgent: (id: string, payload: RunTaskRequest): Promise<RunTaskResult> => {
    const dims = estimateViewportTerminalDimensions()
    const enriched: RunTaskRequest = {
      ...payload,
      cols: payload.cols || dims.cols,
      rows: payload.rows || dims.rows,
    }
    return apiFetch(path(id) + '/run-agent', {
      method: 'POST',
      body: JSON.stringify(enriched),
    })
  },
  resumePlan: (id: string): Promise<{ available: boolean; attempt_id?: string }> => apiFetch(path(id) + '/resume'),
  resume: (id: string, attemptId: string): Promise<{ task_id: string; attempt_id: string; request_id: string; status: string; replayed?: boolean }> =>
    apiFetch(path(id) + '/resume', { method: 'POST', body: JSON.stringify({ attempt_id: attemptId }) }),
  cancel: (id: string): Promise<TaskAttempt> =>
    apiFetch(path(id) + '/cancel', { method: 'POST' }),
  steer: (
    id: string,
    prompt: string,
    author?: string,
  ): Promise<{ status: string; fallback?: boolean; message?: string }> =>
    apiFetch<{ status: string; fallback?: boolean; message?: string }>(
      `/api/v1/tasks/${encodeURIComponent(id)}/steer`,
      {
        method: 'POST',
        body: JSON.stringify({ prompt, author: author || 'user' }),
      },
    ).catch(() =>
      apiFetch<{ status: string; fallback?: boolean; message?: string }>(
        `/api/v1/runner/attempts/${encodeURIComponent(id)}/steer`,
        {
          method: 'POST',
          body: JSON.stringify({ prompt, author: author || 'user' }),
        },
      ),
    ),
  runtime: (
    id: string,
  ): Promise<{
    history: { session_id: string; attempt_id?: string; status: string }[]
    live_session: LiveTaskSession | null
  }> => apiFetch(path(id) + '/run-agent/history'),
  logs: (
    id: string,
    attemptId: string,
    after = 0,
  ): Promise<{ logs: TaskLog[] }> =>
    apiFetch(
      path(id) +
        `/attempts/${encodeURIComponent(attemptId)}/logs?after_id=${after}&limit=250`,
    ),
  comment: (id: string, message: string): Promise<TaskActivity> =>
    apiFetch(path(id) + '/comments', {
      method: 'POST',
      body: JSON.stringify({ message }),
    }),
  apply: (
    id: string,
  ): Promise<{ applied: boolean; sha?: string; message?: string; task?: Task }> =>
    apiFetch(path(id) + '/apply', {
      method: 'POST',
    }),
}
