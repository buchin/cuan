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
}
