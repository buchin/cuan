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
    description: raw.description ?? raw.instructions ?? raw.body ?? '',
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
  runAgent: (id: string, payload: RunTaskRequest): Promise<RunTaskResult> =>
    apiFetch(path(id) + '/run-agent', {
      method: 'POST',
      body: JSON.stringify(payload),
    }),
  cancel: (id: string): Promise<TaskAttempt> =>
    apiFetch(path(id) + '/cancel', { method: 'POST' }),
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
}
