import { apiFetch } from './client'
import type { HarnessAuth } from './agents'

export interface ProviderAccount {
  id: string
  label: string
  provider: string
  legacy: boolean
  credentials_present: boolean
  state: 'unknown' | 'verified' | 'login_required' | 'quota_limited' | 'unavailable'
  message?: string
  last_checked_at?: string
  verified_at?: string
}
const base = '/api/v1/provider-accounts'
export const accountsApi = {
  list: async () => (await apiFetch<{ accounts: ProviderAccount[] }>(base)).accounts,
  create: async (label: string) => (await apiFetch<{ account: ProviderAccount }>(base, { method: 'POST', body: JSON.stringify({ label }) })).account,
  login: (id: string) => apiFetch<HarnessAuth>(`${base}/${encodeURIComponent(id)}/auth-init`, { method: 'POST' }),
  code: async (id: string, session_id: string, code: string) => (await apiFetch<{ account: ProviderAccount }>(`${base}/${encodeURIComponent(id)}/auth-code`, { method: 'POST', body: JSON.stringify({ session_id, code }) })).account,
  cancel: (id: string, session_id: string) => apiFetch(`${base}/${encodeURIComponent(id)}/auth-code?cancel=true`, { method: 'POST', body: JSON.stringify({ session_id, code: '' }) }),
  verify: async (id: string) => (await apiFetch<{ account: ProviderAccount }>(`${base}/${encodeURIComponent(id)}/verify`, { method: 'POST' })).account,
}
