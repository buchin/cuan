import { apiFetch } from './client'
import type { HarnessAuth } from './agents'

export interface ProviderAccount {
  agent_id?: string
  connection_id?: string
  allow_failover?: boolean
  allow_model_fallback?: boolean
  quota_retry_at?: string
  id: string
  label: string
  email?: string
  provider: string
  auth_type?: 'subscription' | 'byok' | 'oauth' | 'cli_login' | string
  model_id?: string
  legacy: boolean
  credentials_present: boolean
  state:
    | 'unknown'
    | 'verified'
    | 'login_required'
    | 'quota_limited'
    | 'unavailable'
  message?: string
  last_checked_at?: string
  verified_at?: string
}
export interface AccountUsage {
  available: boolean
  checked_at: string
  message?: string
  models: { model: string; remaining_percent: number; reset_at?: string }[]
}
const base = '/api/v1/provider-accounts'
export const accountsApi = {
  usage: (id: string) =>
    apiFetch<AccountUsage>(`${base}/${encodeURIComponent(id)}/usage`),
  modelFallback: async (id: string, enabled: boolean) =>
    (
      await apiFetch<{ account: ProviderAccount }>(
        `${base}/${encodeURIComponent(id)}/model-fallback`,
        { method: 'PATCH', body: JSON.stringify({ enabled }) },
      )
    ).account,
  failover: async (id: string, enabled: boolean) =>
    (
      await apiFetch<{ account: ProviderAccount }>(
        `${base}/${encodeURIComponent(id)}/failover`,
        { method: 'PATCH', body: JSON.stringify({ enabled }) },
      )
    ).account,
  list: async () =>
    (await apiFetch<{ accounts: ProviderAccount[] }>(base)).accounts,
  create: async (label: string) =>
    (
      await apiFetch<{ account: ProviderAccount }>(base, {
        method: 'POST',
        body: JSON.stringify({ label }),
      })
    ).account,
  login: (id: string) =>
    apiFetch<HarnessAuth>(`${base}/${encodeURIComponent(id)}/auth-init`, {
      method: 'POST',
    }),
  code: async (id: string, session_id: string, code: string, accept_provider_terms = false) =>
    (
      await apiFetch<{ account: ProviderAccount }>(
        `${base}/${encodeURIComponent(id)}/auth-code`,
        { method: 'POST', body: JSON.stringify({ session_id, code, accept_provider_terms }) },
      )
    ).account,
  cancel: (id: string, session_id: string) =>
    apiFetch(`${base}/${encodeURIComponent(id)}/auth-code?cancel=true`, {
      method: 'POST',
      body: JSON.stringify({ session_id, code: '' }),
    }),
  finishSetup: async (id: string) =>
    (await apiFetch<{ account: ProviderAccount }>(`${base}/${encodeURIComponent(id)}/verify`, {
      method: 'POST', body: JSON.stringify({ finish_setup: true, accept_provider_terms: true }),
    })).account,
  verify: async (id: string) =>
    (
      await apiFetch<{ account: ProviderAccount }>(
        `${base}/${encodeURIComponent(id)}/verify`,
        { method: 'POST' },
      )
    ).account,
}
