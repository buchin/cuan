import { apiFetch } from './client'
import { Project, CreateProjectRequest } from './types'

export const projectsApi = {
  updateSettings: async (id: string, data: { name: string; goal: string; instructions: string; repo_url: string; default_branch: string; deploy_branch?: string; deploy_script?: string; expected_updated_at: string; auto_deploy?: boolean; environment?: string }): Promise<Project> =>
    apiFetch<Project>('/api/v1/projects/' + encodeURIComponent(id) + '/settings', { method: 'PATCH', body: JSON.stringify(data) }),

  list: async (): Promise<Project[]> => {
    const res = await apiFetch<{ projects: Project[] }>(
      '/api/v1/projects?include_archived=true',
    )
    return res.projects || []
  },

  get: async (id: string): Promise<Project> => {
    return apiFetch<Project>(`/api/v1/projects/${id}`)
  },

  create: async (data: CreateProjectRequest): Promise<Project> => {
    return apiFetch<Project>('/api/v1/projects', {
      method: 'POST',
      body: JSON.stringify(data),
    })
  },

  delete: async (id: string): Promise<void> => {
    await apiFetch(`/api/v1/projects/${id}`, {
      method: 'DELETE',
    })
  },

  archive: async (id: string, isArchived: boolean): Promise<Project> => {
    return apiFetch<Project>(
      `/api/v1/projects/${encodeURIComponent(id)}/${isArchived ? 'archive' : 'unarchive'}`,
      {
        method: 'POST',
      },
    )
  },

  reconcile: async (id: string): Promise<{ status: string; project_id: string; interrupted_count: number }> => {
    return apiFetch<{ status: string; project_id: string; interrupted_count: number }>(
      `/api/v1/projects/${encodeURIComponent(id)}/reconcile`,
      {
        method: 'POST',
      },
    )
  },

  getTerminalStatus: async (id: string, fetch?: boolean) => {
    return apiFetch<{
      project_id: string
      project_name: string
      work_dir: string
      session: {
        active: boolean
        session_id?: string
        subscribers?: number
        created_at?: string
        last_activity?: string
        cols?: number
        rows?: number
        pid?: number
        work_dir?: string
      }
      upstream: {
        branch: string
        head_sha: string
        short_head_sha: string
        status_summary: string
        upstream: string
        ahead_count: number
        behind_count: number
        sync_status: string
        warnings?: string[]
      }
    }>(`/api/v1/projects/${encodeURIComponent(id)}/terminal/status${fetch ? '?fetch=1' : ''}`)
  },

  syncTerminalUpstream: async (id: string) => {
    return apiFetch<{
      status: string
      upstream: {
        branch: string
        head_sha: string
        short_head_sha: string
        status_summary: string
        upstream: string
        ahead_count: number
        behind_count: number
        sync_status: string
        warnings?: string[]
      }
    }>(`/api/v1/projects/${encodeURIComponent(id)}/terminal/sync-upstream`, {
      method: 'POST',
    })
  },

  getGitLog: async (id: string, limit = 50, branch?: string): Promise<ProjectGitLogResponse> => {
    const params = new URLSearchParams()
    if (limit) params.set('limit', String(limit))
    if (branch) params.set('branch', branch)
    const qs = params.toString() ? `?${params.toString()}` : ''
    return apiFetch<ProjectGitLogResponse>(`/api/v1/projects/${encodeURIComponent(id)}/git-log${qs}`)
  },

  getCommitDetail: async (id: string, sha: string): Promise<ProjectCommitDetail> => {
    return apiFetch<ProjectCommitDetail>(`/api/v1/projects/${encodeURIComponent(id)}/commits/${encodeURIComponent(sha)}`)
  },

  getDeployments: async (id: string, limit = 50): Promise<ProjectDeploymentsResponse> => {
    const params = new URLSearchParams()
    if (limit) params.set('limit', String(limit))
    const qs = params.toString() ? `?${params.toString()}` : ''
    return apiFetch<ProjectDeploymentsResponse>(`/api/v1/projects/${encodeURIComponent(id)}/deployments${qs}`)
  },

  getDeploymentDetail: async (id: string, depId: string): Promise<{ deployment: ProjectDeploymentItem }> => {
    return apiFetch<{ deployment: ProjectDeploymentItem }>(`/api/v1/projects/${encodeURIComponent(id)}/deployments/${encodeURIComponent(depId)}`)
  },

  triggerDeploy: async (id: string, data?: { branch?: string; commit_sha?: string }): Promise<{ deployment: ProjectDeploymentItem; message: string }> => {
    return apiFetch<{ deployment: ProjectDeploymentItem; message: string }>(`/api/v1/projects/${encodeURIComponent(id)}/deployments`, {
      method: 'POST',
      body: JSON.stringify(data || {}),
    })
  },

  cancelDeploy: async (id: string, depId: string): Promise<{ deployment: ProjectDeploymentItem; message: string }> => {
    return apiFetch<{ deployment: ProjectDeploymentItem; message: string }>(`/api/v1/projects/${encodeURIComponent(id)}/deployments/${encodeURIComponent(depId)}/cancel`, {
      method: 'POST',
    })
  },
}

export interface ProjectDeploymentStep {
  id: string
  name: string
  status: 'pending' | 'building' | 'success' | 'failed' | 'skipped'
  duration?: string
  started_at?: string
  completed_at?: string
  summary?: string
  log?: string
}

export interface ProjectDeploymentItem {
  id: string
  project_id: string
  commit_sha: string
  short_sha: string
  commit_message: string
  branch: string
  status: 'building' | 'success' | 'failed' | 'canceled'
  trigger_source: 'manual' | 'auto_deploy' | 'git_push' | 'cli'
  trigger_author: string
  duration_seconds: number
  duration: string
  started_at: string
  completed_at?: string
  steps: ProjectDeploymentStep[]
  error_message?: string
  created_at: string
  relative_time: string
}

export interface ProjectDeploymentsResponse {
  project_id: string
  project_name: string
  branch: string
  domain: string
  deployments: ProjectDeploymentItem[]
  total: number
  live_sha?: string
}

export interface ProjectCommit {
  sha: string
  short_sha: string
  author_name: string
  author_email: string
  date: string
  relative_date: string
  message: string
  task_id?: string
  task_ref?: string
}

export interface ProjectGitLogResponse {
  project_id: string
  branch: string
  commits: ProjectCommit[]
  total: number
}

export interface CommitFileStat {
  path: string
  insertions: number
  deletions: number
}

export interface ProjectCommitDetail {
  sha: string
  short_sha: string
  author_name: string
  author_email: string
  date: string
  relative_date: string
  message: string
  body: string
  stats_summary: string
  files: CommitFileStat[]
  patch_preview?: string
  task_id?: string
  task_ref?: string
}
