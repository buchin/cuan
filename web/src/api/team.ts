import { apiFetch } from './client'

export interface TeamMember {
  id: string
  name?: string
  username?: string
  email: string
  role: string
  is_active: boolean
  status: string
  created_at: string
}

export interface UserInvitation {
  id: string
  email: string
  role: string
  created_by?: string
  expires_at: string
  accepted_at?: string | null
  created_at: string
}

export interface TeamListResponse {
  status: string
  members: TeamMember[]
  invitations: UserInvitation[]
}

export interface CreateInviteRequest {
  email: string
  role?: string
}

export interface CreateInviteResponse {
  status: string
  invitation: UserInvitation
  token: string
  invite_url: string
}

export interface AcceptInviteRequest {
  token: string
  name: string
  password: string
}

export interface VerifyInviteResponse {
  status: string
  valid: boolean
  email: string
  role: string
}

export const teamApi = {
  listMembers: () => apiFetch<TeamListResponse>('/api/v1/team/members'),
  createInvite: (data: CreateInviteRequest) =>
    apiFetch<CreateInviteResponse>('/api/v1/team/invites', {
      method: 'POST',
      body: JSON.stringify(data),
    }),
  deleteInvite: (id: string) =>
    apiFetch<{ status: string; message: string }>(
      `/api/v1/team/invites/${encodeURIComponent(id)}`,
      { method: 'DELETE' }
    ),
  deleteMember: (id: string) =>
    apiFetch<{ status: string; message: string }>(
      `/api/v1/team/members/${encodeURIComponent(id)}`,
      { method: 'DELETE' }
    ),
  verifyInvite: (token: string) =>
    apiFetch<VerifyInviteResponse>(
      `/api/v1/auth/invites/verify?token=${encodeURIComponent(token)}`
    ),
  acceptInvite: (data: AcceptInviteRequest) =>
    apiFetch<{ token: string; user?: any; owner?: any }>(
      '/api/v1/auth/invites/accept',
      {
        method: 'POST',
        body: JSON.stringify(data),
      }
    ),
}
