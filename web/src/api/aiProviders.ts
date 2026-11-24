import { apiFetch } from './client'

export interface HarnessBinaryStatus {
  name: string
  installed: boolean
  path?: string
  version?: string
  source?: string
  last_checked_at?: string
}

export interface AIProviderModelInfo {
  id: string
  name: string
  display_name?: string
  description?: string
  provider?: string
  supports_vision?: boolean
  input_token_limit?: number
  output_token_limit?: number
  is_default?: boolean
}

export interface AIProviderDTO {
  id: string
  name: string
  mode: 'subscription' | 'byok' | string
  supported_modes: string[]
  has_api_key: boolean
  api_key_masked?: string
  base_url?: string
  default_model?: string
  models: AIProviderModelInfo[]
  binary_status?: HarnessBinaryStatus
  status: 'ready' | 'configured' | 'unconfigured' | string
  enabled: boolean
}

export interface SaveAIProviderRequest {
  clear_api_key?: boolean
  provider: string
  id?: string
  mode?: 'subscription' | 'byok' | string
  api_key?: string
  base_url?: string
  default_model?: string
  enabled?: boolean
}

export interface TestAIProviderRequest {
  provider: string
  id?: string
  api_key?: string
  base_url?: string
  model?: string
}

export interface TestAIProviderResponse {
  status: 'success' | 'error' | string
  valid: boolean
  provider: string
  latency_ms: number
  plan_hint?: string
  models?: AIProviderModelInfo[]
  error_message?: string
  tested_at?: string
}

export interface InstallBinaryRequest {
  binary: string
  name?: string
  harness?: string
}

export interface InstallBinaryResponse {
  status: 'success' | 'error' | string
  binary: string
  installed: boolean
  path?: string
  version?: string
  source?: string
  message: string
}

export interface AgentProviderBinding {
  status?: string
  agent_id: string
  provider: string
  harness?: string
  model_id?: string
  fallback_model_id?: string
}

export interface UpdateAgentProviderRequest {
  provider?: string
  harness?: string
  model_id?: string
  fallback_model_id?: string
}

const DEFAULT_PROVIDER_KEY = 'cuan_default_store_provider'
const DEFAULT_MODEL_KEY = 'cuan_default_store_model'

export function getDefaultStoreProvider(): string {
  try {
    return localStorage.getItem(DEFAULT_PROVIDER_KEY) || 'google'
  } catch {
    return 'google'
  }
}

export function setDefaultStoreProvider(providerId: string, modelId?: string): void {
  try {
    localStorage.setItem(DEFAULT_PROVIDER_KEY, providerId)
    if (modelId) {
      localStorage.setItem(DEFAULT_MODEL_KEY, modelId)
    }
    window.dispatchEvent(
      new CustomEvent('cuan:default-provider-changed', {
        detail: { provider: providerId, model: modelId },
      })
    )
  } catch {
    // ignore local storage errors in sandboxed environments
  }
}

export function getDefaultStoreModel(): string {
  try {
    return localStorage.getItem(DEFAULT_MODEL_KEY) || ''
  } catch {
    return ''
  }
}

export const aiProvidersApi = {
  list: async (): Promise<AIProviderDTO[]> => {
    const res = await apiFetch<{ status: string; providers: AIProviderDTO[] }>('/api/v1/ai/providers')
    return res.providers || []
  },

  save: async (req: SaveAIProviderRequest): Promise<{ status: string; provider?: AIProviderDTO }> => {
    return apiFetch<{ status: string; provider?: AIProviderDTO }>('/api/v1/ai/providers', {
      method: 'POST',
      body: JSON.stringify(req),
    })
  },

  test: async (req: TestAIProviderRequest): Promise<TestAIProviderResponse> => {
    return apiFetch<TestAIProviderResponse>('/api/v1/ai/providers/test', {
      method: 'POST',
      body: JSON.stringify(req),
    })
  },

  fetchModels: async (req: {
    provider: string
    id?: string
    api_key?: string
    base_url?: string
    refresh?: boolean
  }): Promise<{ status: string; models: AIProviderModelInfo[] }> => {
    return apiFetch<{ status: string; models: AIProviderModelInfo[] }>('/api/v1/ai/providers/models', {
      method: 'POST',
      body: JSON.stringify(req),
    })
  },

  installBinary: async (binary: string): Promise<InstallBinaryResponse> => {
    return apiFetch<InstallBinaryResponse>('/api/v1/ai/providers/install-binary', {
      method: 'POST',
      body: JSON.stringify({ binary }),
    })
  },

  getAgentProvider: async (agentId: string): Promise<AgentProviderBinding> => {
    return apiFetch<AgentProviderBinding>(`/api/v1/agents/${encodeURIComponent(agentId)}/provider`)
  },

  updateAgentProvider: async (
    agentId: string,
    req: UpdateAgentProviderRequest
  ): Promise<{ status: string; agent?: any }> => {
    return apiFetch<{ status: string; agent?: any }>(`/api/v1/agents/${encodeURIComponent(agentId)}/provider`, {
      method: 'PUT',
      body: JSON.stringify(req),
    })
  },
}
