import { apiFetch } from './client'

export type TasteCategory = 'brand' | 'technical'

export interface TasteItem {
  id: string
  category: TasteCategory
  topic?: string
  content: string
  created_at: string
  updated_at: string
}

export interface TasteProfile {
  items: TasteItem[]
  brand_items: TasteItem[]
  technical_items: TasteItem[]
  total: number
  updated_at: string
}

export interface CreateTasteItemRequest {
  category: TasteCategory
  topic?: string
  content: string
}

export interface UpdateTasteItemRequest {
  category?: TasteCategory
  topic?: string
  content?: string
}

export type TasteCandidateStatus = 'suggested' | 'pending' | 'accepted' | 'dismissed'

export interface TasteCandidate {
  id: string
  category: TasteCategory
  topic?: string
  text: string
  content?: string
  source_snippet?: string
  reason?: string
  rationale?: string
  status: string
  created_at: string
}

export interface TasteCandidatesResponse {
  candidates: TasteCandidate[]
  items: TasteCandidate[]
  total: number
}

export interface ReflectTasteRequest {
  text?: string
  messages?: string[]
  limit?: number
}

export interface ReflectTasteResponse {
  message: string
  candidates: TasteCandidate[]
  extracted: TasteCandidate[]
  count: number
  total: number
}

export interface AcceptCandidateRequest {
  text?: string
  content?: string
  topic?: string
  category?: TasteCategory
}

export interface AcceptCandidateResponse {
  message: string
  candidate: TasteCandidate
  item: TasteItem
  id: string
}

export interface DismissCandidateResponse {
  message: string
  candidate: TasteCandidate
  id: string
}

export const tasteApi = {
  getProfile: async (): Promise<TasteProfile> => {
    return apiFetch<TasteProfile>('/api/v1/taste')
  },
  createItem: async (data: CreateTasteItemRequest): Promise<TasteItem> => {
    return apiFetch<TasteItem>('/api/v1/taste/items', {
      method: 'POST',
      body: JSON.stringify(data),
    })
  },
  updateItem: async (id: string, data: UpdateTasteItemRequest): Promise<TasteItem> => {
    return apiFetch<TasteItem>(`/api/v1/taste/items/${encodeURIComponent(id)}`, {
      method: 'PUT',
      body: JSON.stringify(data),
    })
  },
  deleteItem: async (id: string): Promise<{ message: string; id: string }> => {
    return apiFetch<{ message: string; id: string }>(`/api/v1/taste/items/${encodeURIComponent(id)}`, {
      method: 'DELETE',
    })
  },
  resetProfile: async (): Promise<TasteProfile> => {
    return apiFetch<TasteProfile>('/api/v1/taste/reset', {
      method: 'POST',
    })
  },
  getCandidates: async (params?: { status?: string; category?: string }): Promise<TasteCandidatesResponse> => {
    const search = new URLSearchParams()
    if (params?.status) search.set('status', params.status)
    if (params?.category) search.set('category', params.category)
    const qs = search.toString()
    return apiFetch<TasteCandidatesResponse>(`/api/v1/taste/candidates${qs ? `?${qs}` : ''}`)
  },
  reflectTaste: async (data?: ReflectTasteRequest): Promise<ReflectTasteResponse> => {
    return apiFetch<ReflectTasteResponse>('/api/v1/taste/reflect', {
      method: 'POST',
      body: JSON.stringify(data || {}),
    })
  },
  acceptCandidate: async (id: string, data?: AcceptCandidateRequest): Promise<AcceptCandidateResponse> => {
    return apiFetch<AcceptCandidateResponse>(`/api/v1/taste/candidates/${encodeURIComponent(id)}/accept`, {
      method: 'POST',
      body: JSON.stringify(data || {}),
    })
  },
  dismissCandidate: async (id: string): Promise<DismissCandidateResponse> => {
    return apiFetch<DismissCandidateResponse>(`/api/v1/taste/candidates/${encodeURIComponent(id)}/dismiss`, {
      method: 'POST',
    })
  },
}
