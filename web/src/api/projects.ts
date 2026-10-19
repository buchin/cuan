import { apiFetch } from './client'
import { Project, CreateProjectRequest } from './types'

export const projectsApi = {
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
}
