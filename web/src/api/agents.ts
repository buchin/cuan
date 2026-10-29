import { apiFetch } from './client'

export interface Agent {
  id: string
  name: string
  harness?: string
  connection_id?: string
  enabled?: boolean
  avatar?: string
  role?: string
  system_prompt?: string
  model_id?: string
  fallback_model_id?: string
  temperature?: number
  token_budget?: number
  provider?: string
  auth_type?: 'subscription' | 'byok' | 'oauth' | 'cli_login' | string
  status?: string
  created_at?: string
  updated_at?: string
  api_key?: string
  endpoint?: string
  connection_state?: string
  last_checked_at?: string
  verified_at?: string
  host_session?: boolean
}

export interface AgentConnection {
  id: string
  user_id: string
  provider: string
  plan_hint?: string
  status: string
  has_credentials: boolean
  created_at: string
  updated_at: string
}

export interface AgentModelConfig {
  status?: string
  agent_id: string
  name: string
  role?: string
  harness?: string
  provider?: string
  model_id?: string
  fallback_model_id?: string
  temperature?: number
  token_budget?: number
  system_prompt?: string
  supported_models?: ModelInfo[]
}

export interface UpdateAgentModelConfigRequest {
  model_id?: string
  fallback_model_id?: string
  temperature?: number
  token_budget?: number
  system_prompt?: string
  role?: string
}

export interface ConnectCLIAgentRequest {
  account_id?: string
  name: string
  handle?: string
  provider?: string
  auth_type?: 'subscription' | 'byok' | 'oauth' | 'cli_login' | string
  model_id?: string
  role?: string
  system_prompt?: string
  session_token?: string
}

export interface ConnectCLIAgentResponse {
  status: string
  agent: Agent
  connection: AgentConnection
  handle: string
  provider: string
  room_id: string
  conversation_id: string
  auth_model: string
  message: string
}

export interface HarnessAuth {
  authenticated: boolean
  verified?: boolean
  credentials_present?: boolean
  message?: string
  auth_url?: string
  session_id?: string
  expires_at?: string
}

export interface ModelCapabilities {
  vision?: boolean
  supports_vision?: boolean
}

export interface ModelInfo {
  id: string
  name: string
  display_name?: string
  description?: string
  provider?: string
  input_token_limit?: number
  output_token_limit?: number
  is_default?: boolean
  supports_vision?: boolean
  capabilities?: ModelCapabilities
  preserved?: boolean
}

export function isModelVisionCapable(modelId?: string, provider?: string, modelInfo?: ModelInfo | null): boolean {
  if (modelInfo) {
    if (modelInfo.capabilities?.vision !== undefined) return modelInfo.capabilities.vision
    if (modelInfo.capabilities?.supports_vision !== undefined) return modelInfo.capabilities.supports_vision
    if (modelInfo.supports_vision !== undefined) return modelInfo.supports_vision
  }
  const p = (provider || '').toLowerCase()
  const m = (modelId || '').toLowerCase()
  if (p === 'agy' || p === 'antigravity' || p === 'google' || p === 'gemini') {
    if (m.includes('gpt-oss') || m.includes('text') || m.includes('embedding')) {
      return false
    }
    return true
  }
  if (!m || m === 'commandcode-default' || m === 'default') {
    return false
  }
  const visionKeywords = [
    'vision', 'multimodal', '-vl', 'super-vl', 'omni',
    'sonnet', 'opus', 'gemini', '4o', 'kimi-k2',
    'minimax-m3', 'inkling', 'mistral-large-4', 'muse-spark-1.3',
    'v4.1-flash',
  ]
  return visionKeywords.some((kw) => m.includes(kw))
}

export const agentsApi = {
  listModels: async (provider = 'commandcode', refresh = false): Promise<ModelInfo[]> => {
    const params = new URLSearchParams()
    if (provider) params.set('provider', provider)
    if (refresh) params.set('refresh', 'true')
    const query = params.toString() ? `?${params.toString()}` : ''
    const res = await apiFetch<{ status: string; provider: string; models: ModelInfo[]; refreshed?: boolean }>(
      `/api/v1/agents/models${query}`
    )
    return res.models || []
  },
  verifyConnection: (id: string) => apiFetch<{ connection_state: string; message: string; last_checked_at?: string; verified_at?: string }>(`/api/v1/agents/${encodeURIComponent(id)}/connection/verify`, { method: 'POST' }),
  replaceCredentials: (id: string, api_key: string) => apiFetch<{ status?: string }>(`/api/v1/agents/${encodeURIComponent(id)}/connection/credentials`, { method: 'POST', body: JSON.stringify({ api_key }) }),
  authStatus: (harness: string, verify = false) => apiFetch<HarnessAuth>(`/api/v1/agents/auth-status?harness=${encodeURIComponent(harness)}&verify=${verify}`),
  authInit: (harness = 'agy') => apiFetch<HarnessAuth>('/api/v1/agents/auth-init', { method: 'POST', body: JSON.stringify({ harness }) }),
  authCode: (session_id: string, code: string) => apiFetch<HarnessAuth>('/api/v1/agents/auth-code', { method: 'POST', body: JSON.stringify({ session_id, code }) }),
  authCancel: (session_id: string) => apiFetch('/api/v1/agents/auth-code?cancel=true', { method: 'POST', body: JSON.stringify({ session_id, code: '' }) }),
  list: async (): Promise<Agent[]> => {
    const res = await apiFetch<{ status: string; agents: Agent[]; count: number }>('/api/v1/agents')
    return res.agents || []
  },

  create: async (data: Partial<Agent>): Promise<Agent> => {
    const res = await apiFetch<{ status: string; agent: Agent }>('/api/v1/agents', {
      method: 'POST',
      body: JSON.stringify(data),
    })
    return res.agent
  },

  patch: async (id: string, data: Partial<Agent>): Promise<Agent> => {
    const res = await apiFetch<{ status: string; agent: Agent }>(`/api/v1/agents/${id}`, {
      method: 'PATCH',
      body: JSON.stringify(data),
    })
    return res.agent
  },

  delete: async (id: string): Promise<void> => {
    await apiFetch(`/api/v1/agents/${id}`, {
      method: 'DELETE',
    })
  },

  connectCLI: async (data: ConnectCLIAgentRequest): Promise<ConnectCLIAgentResponse> => {
    return apiFetch<ConnectCLIAgentResponse>('/api/v1/agents/connect-cli', {
      method: 'POST',
      body: JSON.stringify(data),
    })
  },

  listConnections: async (): Promise<AgentConnection[]> => {
    const res = await apiFetch<{ status: string; connections: AgentConnection[]; count: number }>(
      '/api/v1/agents/connections'
    )
    return res.connections || []
  },

  // Preserved endpoint signature: getModelConfig: async (id: string)
  getModelConfig: async (id: string, refresh = false): Promise<AgentModelConfig> => {
    const query = refresh ? '?refresh=true' : ''
    return apiFetch<AgentModelConfig>(`/api/v1/agents/${encodeURIComponent(id)}/model-config${query}`)
  },

  updateModelConfig: async (
    id: string,
    data: UpdateAgentModelConfigRequest,
  ): Promise<{ status: string; message: string; agent: Agent; config: AgentModelConfig }> => {
    return apiFetch<{ status: string; message: string; agent: Agent; config: AgentModelConfig }>(
      `/api/v1/agents/${encodeURIComponent(id)}/model-config`,
      {
        method: 'PATCH',
        body: JSON.stringify(data),
      },
    )
  },
}
