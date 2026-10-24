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
  provider?: string
  auth_type?: string
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

export interface ConnectCLIAgentRequest {
  account_id?: string
  name: string
  handle?: string
  provider?: string
  model_id?: string
  role?: string
  system_prompt?: string
  auth_type?: string
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

export interface ModelInfo {
  id: string
  name: string
  display_name?: string
  supports_vision?: boolean
  multimodal?: boolean
}

export const agentsApi = {
  verifyConnection: (id: string) => apiFetch<{ connection_state: string; message: string; last_checked_at?: string; verified_at?: string }>(`/api/v1/agents/${encodeURIComponent(id)}/connection/verify`, { method: 'POST' }),
  authStatus: (harness: string, verify = false) => apiFetch<HarnessAuth>(`/api/v1/agents/auth-status?harness=${encodeURIComponent(harness)}&verify=${verify}`),
  authInit: (harness = 'agy') => apiFetch<HarnessAuth>('/api/v1/agents/auth-init', { method: 'POST', body: JSON.stringify({ harness }) }),
  authCode: (session_id: string, code: string) => apiFetch<HarnessAuth>('/api/v1/agents/auth-code', { method: 'POST', body: JSON.stringify({ session_id, code }) }),
  authCancel: (session_id: string) => apiFetch('/api/v1/agents/auth-code?cancel=true', { method: 'POST', body: JSON.stringify({ session_id, code: '' }) }),
  replaceCredentials: (id: string, apiKey: string) =>
    apiFetch<{ status: string }>(
      `/api/v1/agents/${encodeURIComponent(id)}/connection/credentials`,
      { method: 'POST', body: JSON.stringify({ api_key: apiKey }) },
    ),
  listModels: async (provider = 'commandcode'): Promise<ModelInfo[]> =>
    (
      await apiFetch<{ models: ModelInfo[] }>(
        `/api/v1/agents/models?provider=${encodeURIComponent(provider)}`,
      )
    ).models || [],
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
}
