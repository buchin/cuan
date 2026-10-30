import { apiFetch } from './client'

export interface App {
  id: string
  name: string
  description: string
  icon: string
  category: string // "productivity", "finance", "messaging", etc.
  status: 'connected' | 'disconnected' | 'error'
  account?: string
  connected_at?: string
  capabilities: string[]
  error?: string
}

export interface Skill {
  id: string
  app_id?: string
  name: string
  description: string
  icon: string
  category: string // "vision", "finance", "automation", "productivity"
  enabled: boolean
  tools: string[]
  bound_agents: string[]
  created_at?: string
  updated_at?: string
}

export interface AgentSkillView {
  skill_id: string
  name: string
  description: string
  icon: string
  category: string
  app_id?: string
  enabled: boolean
  tools?: string[]
  updated_at?: string
}

export interface AgentSkillInput {
  skill_id: string
  enabled: boolean
}

export interface ReceiptItem {
  name: string
  quantity: number
  price: number
  total: number
}

export interface ReceiptExtraction {
  merchant: string
  date: string
  total_amount: number
  currency: string
  items: ReceiptItem[]
  category: string
  confidence: number
  raw_text?: string
  confirmed?: boolean
  recorded_at?: string
}

export const appsSkillsApi = {
  /**
   * List all third-party external apps in the catalog.
   */
  async listApps(category?: string): Promise<{ status: string; apps: App[]; total: number }> {
    const query = category ? `?category=${encodeURIComponent(category)}` : ''
    return apiFetch<{ status: string; apps: App[]; total: number }>(`/api/v1/apps${query}`)
  },

  /**
   * 1-Click connect an external app.
   */
  async connectApp(id: string, account?: string): Promise<{ status: string; app: App }> {
    return apiFetch<{ status: string; app: App }>(`/api/v1/apps/${encodeURIComponent(id)}/connect`, {
      method: 'POST',
      body: JSON.stringify({ account: account || 'bisnis@cuan.id' }),
    })
  },

  /**
   * 1-Click disconnect an external app.
   */
  async disconnectApp(id: string): Promise<{ status: string; app: App }> {
    return apiFetch<{ status: string; app: App }>(`/api/v1/apps/${encodeURIComponent(id)}/disconnect`, {
      method: 'POST',
    })
  },

  /**
   * List all modular intelligence skills in the catalog.
   */
  async listSkills(category?: string): Promise<{ status: string; skills: Skill[]; total: number }> {
    const query = category ? `?category=${encodeURIComponent(category)}` : ''
    return apiFetch<{ status: string; skills: Skill[]; total: number }>(`/api/v1/skills${query}`)
  },

  /**
   * Get skill enablement configuration for a specific agent.
   */
  async getAgentSkills(agentId: string): Promise<{
    status: string
    agent_id: string
    skills: AgentSkillView[]
    total: number
  }> {
    return apiFetch<{
      status: string
      agent_id: string
      skills: AgentSkillView[]
      total: number
    }>(`/api/v1/agents/${encodeURIComponent(agentId)}/skills`)
  },

  /**
   * Update skill enablement configuration for a specific agent.
   */
  async updateAgentSkills(
    agentId: string,
    skills: AgentSkillInput[]
  ): Promise<{
    status: string
    agent_id: string
    skills: AgentSkillView[]
    total: number
  }> {
    return apiFetch<{
      status: string
      agent_id: string
      skills: AgentSkillView[]
      total: number
    }>(`/api/v1/agents/${encodeURIComponent(agentId)}/skills`, {
      method: 'PUT',
      body: JSON.stringify({ skills }),
    })
  },
}
