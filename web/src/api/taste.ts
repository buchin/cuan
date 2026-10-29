import { apiFetch } from './client'

export type TasteCategory = 'brand' | 'technical'

export interface GroundingTaste {
  id: string
  category: TasteCategory | string
  topic: string
  summary: string
  preview?: string
  content?: string
}

export interface SessionGroundingTastesResponse {
  session_id: string
  room_id?: string
  brand_tastes?: GroundingTaste[]
  technical_tastes?: GroundingTaste[]
  all_tastes?: GroundingTaste[]
  grounding_tastes?: GroundingTaste[]
  total: number
}

export interface TasteItem {
  id: string
  category: TasteCategory
  topic?: string
  content: string
  auto_approved?: boolean
  auto_approved_at?: string
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
  confidence?: number // 0.0 - 1.0
  created_at: string
  updated_at?: string
}

// Alias for CandidateTaste as specified in DoD
export type CandidateTaste = TasteCandidate

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
  auto_approved?: TasteCandidate[]
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

export interface UndoCandidateResponse {
  message: string
  candidate: TasteCandidate
  id: string
}

export interface TasteQuotaCategory {
  used: number
  max: number
  percentage: number
  can_auto_approve: boolean
}

export interface TasteQuotaResponse {
  brand: TasteQuotaCategory
  technical: TasteQuotaCategory
  categories: Record<string, TasteQuotaCategory>
  total_used: number
  total_max: number
  auto_approve_enabled: boolean
  auto_approve_threshold: number
}

export interface TasteConfig {
  auto_approve_enabled: boolean
  auto_approve_threshold: number
}

export interface UpdateTasteConfigRequest {
  auto_approve_enabled?: boolean
  auto_approve_threshold?: number
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
  getQuota: async (): Promise<TasteQuotaResponse> => {
    return apiFetch<TasteQuotaResponse>('/api/v1/taste/quota')
  },
  getConfig: async (): Promise<TasteConfig> => {
    return apiFetch<TasteConfig>('/api/v1/taste/config')
  },
  updateConfig: async (data: UpdateTasteConfigRequest): Promise<TasteConfig> => {
    return apiFetch<TasteConfig>('/api/v1/taste/config', {
      method: 'PUT',
      body: JSON.stringify(data),
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
  undoCandidate: async (id: string): Promise<UndoCandidateResponse> => {
    return apiFetch<UndoCandidateResponse>(`/api/v1/taste/candidates/${encodeURIComponent(id)}/undo`, {
      method: 'POST',
    })
  },
  getActiveGrounding: async (params?: {
    session_id?: string
    room_id?: string
    category?: string
  }): Promise<SessionGroundingTastesResponse> => {
    const search = new URLSearchParams()
    if (params?.session_id) search.set('session_id', params.session_id)
    if (params?.room_id) search.set('room_id', params.room_id)
    if (params?.category) search.set('category', params.category)
    const qs = search.toString()
    return apiFetch<SessionGroundingTastesResponse>(`/api/v1/taste/grounding${qs ? `?${qs}` : ''}`)
  },
  getConversationGrounding: async (conversationId: string): Promise<SessionGroundingTastesResponse> => {
    return apiFetch<SessionGroundingTastesResponse>(
      `/api/v1/chat/conversations/${encodeURIComponent(conversationId)}/grounding`
    )
  },
}
