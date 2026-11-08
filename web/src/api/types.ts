// Core API Models and DTOs for Cuan

export interface User {
  id: string
  username: string
  email: string
}

export interface AuthStatusResponse {
  setup_required: boolean
  has_owner: boolean
}

export interface LoginResponse {
  token: string
  user: User
}

export type ProjectType = 'custom' | 'blog' | 'youtube' | 'olshop' | 'affiliate' | 'saas' | 'blank'

export interface Project {
  task_progress?: { total: number; done: number; active?: number; queued?: number; review?: number; blocked?: number; canceled?: number }
  id: string
  name: string
  repo_url: string
  repo_owner?: string
  repo_name?: string
  default_branch?: string
  local_path?: string
  has_token?: boolean
  token_preview?: string
  clone_status?: 'pending' | 'connected' | 'synced' | 'failed'
  project_type: ProjectType
  metadata?: Record<string, any>
  enabled_modules?: string[]
  source_type?: string
  source_metadata?: Record<string, any>
  is_archived: boolean
  last_fetched_at?: string
  last_error?: string
  created_at: string
  updated_at: string
}

export interface CreateProjectRequest {
  name: string
  repo_url?: string
  project_type?: ProjectType
  github_token?: string
}

export interface UpdateProjectRequest {
  name?: string
  repo_url?: string
  project_type?: ProjectType
  is_archived?: boolean
}

export type TaskStatus = 'backlog' | 'todo' | 'queued' | 'doing' | 'review' | 'done' | 'blocked'
export type TaskPriority = 'low' | 'normal' | 'high' | 'urgent'

export interface DoDItem {
  evidence?: string
  id?: string
  text: string
  completed?: boolean
}

export interface TaskAttempt {
  cancel_requested?: boolean
  canceled_at?: string
  execution_deadline?: string
  id: string
  task_id: string
  runner_id: string
  agent_name: string
  status: 'queued' | 'running' | 'succeeded' | 'failed' | 'canceled' | 'interrupted'
  attempt_number: number
  exit_code?: number
  error_message?: string
  created_at: string
  started_at?: string
  finished_at?: string
  lease_expires_at?: string
}
export interface TaskActivity {
  id: string
  task_id: string
  attempt_id?: string
  activity_type: string
  author: string
  message: string
  old_status?: string
  new_status?: string
  created_at: string
}
export interface Attachment {
  id: string
  name: string
  type: string
  size: number
  url: string
}
export interface TaskDetail extends Task {
  attempts: TaskAttempt[]
  activities: TaskActivity[]
  attachments?: Attachment[]
}
export interface Task {
  review_state?: 'queued' | 'running'
  review_finding?: string
  review_action?: 'continue' | 'replan' | 'wait' | 'needs_owner'
  repo_url?: string
  result_kind?: 'answer' | 'implementation'
  launch?: { state: 'queued' | 'claimed' | 'started' | 'stopped' | 'cancelled'; safe_error?: string; available_at: string }
  latest_attempt?: TaskAttempt
  source_room_id?: string
  source_message_id?: string
  id: string
  project_id?: string
  project_ids?: string[]
  title: string
  description?: string
  summary?: string
  complete_note?: string
  project_name?: string
  project_names?: string[]
  branch_name?: string
  pr_url?: string
  commit_sha?: string
  diff_summary?: string
  test_summary?: string
  body?: string
  prompt?: string
  status: TaskStatus
  priority: TaskPriority
  assignee?: string
  assignee_id?: string
  assignee_type?: 'human' | 'agent'
  dod_checklist?: (DoDItem | string)[]
  tags?: string[]
  runner_state?: string
  latest_progress?: string
  execution_status?: 'idle' | 'running' | 'completed' | 'failed' | string
  created_at: string
  updated_at: string
}

export interface CreateTaskRequest {
  request_id?: string
  title: string
  description?: string
  project_ids?: string[]
  priority?: TaskPriority
  status?: TaskStatus
  assignee?: string
  assignee_id?: string
  execution_status?: string
  dod_checklist?: (DoDItem | string)[]
  tags?: string[]
}

export interface UpdateTaskRequest {
  expected_updated_at?: string
  summary?: string
  complete_note?: string
  title?: string
  description?: string
  project_ids?: string[]
  priority?: TaskPriority
  status?: TaskStatus
  assignee?: string
  assignee_id?: string
  execution_status?: string
  dod_checklist?: (DoDItem | string)[]
  tags?: string[]
}

export interface AgentInfo {
  id: string
  name: string
  status: 'idle' | 'running' | 'offline'
  current_task_id?: string
  model?: string
  last_active?: string
}
