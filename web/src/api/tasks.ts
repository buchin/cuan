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
    description: (raw.description ?? raw.instructions ?? raw.body ?? '').split('\n\nProject guidance snapshot (context only; does not expand task authorization):\n```cuan-project-context\n')[0],
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
export const tasksApi = {
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
  cancel: (id: string): Promise<TaskAttempt> =>
    apiFetch(path(id) + '/cancel', { method: 'POST' }),
  steer: (
    attemptId: string,
    prompt: string,
    author?: string,
  ): Promise<{ status: string; fallback?: boolean; message?: string }> =>
    apiFetch(`/api/v1/runner/attempts/${encodeURIComponent(attemptId)}/steer`, {
      method: 'POST',
      body: JSON.stringify({ prompt, author: author || 'user' }),
    }),
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
