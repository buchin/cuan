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

export type TaskStatus = 'backlog' | 'todo' | 'doing' | 'review' | 'done' | 'blocked'
export type TaskPriority = 'low' | 'normal' | 'high' | 'urgent'

export interface DoDItem {
  id?: string
  text: string
  completed?: boolean
}

export interface Task {
  latest_attempt?: { id: string; status: string; attempt_number?: number; error_message?: string }
  source_room_id?: string
  source_message_id?: string
  id: string
  project_id?: string
  project_ids?: string[]
  title: string
  description?: string
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
  execution_status?: 'idle' | 'running' | 'completed' | 'failed' | string
  created_at: string
  updated_at: string
}

export interface CreateTaskRequest {
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
