import type { Agent } from '../../api/agents'
import type { ChatContact } from '../../api/chat'

export function agentForContact(contact: ChatContact, agents: Agent[]) {
  return contact.type === 'agent'
    ? agents.find((a) => a.id === contact.id.replace(/^contact_/, ''))
    : undefined
}

export function connectionStatus(agent?: Agent, now = Date.now()) {
  if (agent && (agent.enabled === false || agent.status === 'inactive'))
    return { state: 'disabled', label: 'Dinonaktifkan', verified: false }
  let state = agent?.connection_state || 'unknown'
  const verified = Date.parse(agent?.verified_at || '')
  if (
    state === 'verified' &&
    (!Number.isFinite(verified) ||
      verified > now + 60_000 ||
      verified + 5 * 60_000 <= now)
  )
    state = 'unknown'
  const labels: Record<string, string> = {
    verified: 'Terhubung',
    login_required: 'Perlu login',
    quota_limited: 'Kuota terbatas',
    unavailable: 'Koneksi terganggu',
    unknown: 'Belum diperiksa',
  }
  return {
    state,
    label: labels[state] || labels.unknown,
    verified: state === 'verified',
  }
}
