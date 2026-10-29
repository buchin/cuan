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
}
