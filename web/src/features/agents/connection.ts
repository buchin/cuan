import type { Agent } from '../../api/agents'
import type { ChatContact } from '../../api/chat'

export function agentForContact(contact: ChatContact, agents: Agent[]) {
  return contact.type === 'agent'
    ? agents.find(
        (a) =>
          a.id === (contact.agent_id || contact.id.replace(/^contact_/, '')),
      )
    : undefined
}

export function connectionStatus(agent?: Agent, _now = Date.now()) {
  if (agent && (agent.enabled === false || agent.status === 'inactive'))
    return { state: 'disabled', label: 'Disabled', verified: false }
  const state = agent?.connection_state || 'unknown'
  const labels: Record<string, string> = {
    verified: 'Connected',
    login_required: 'Sign-in required',
    quota_limited: 'Quota limited',
    unavailable: 'Connection error',
    unknown: 'Not checked',
    disabled: 'Disabled',
  }
  return {
    state,
    label: labels[state] || labels.unknown,
    verified: state === 'verified',
  }
}
