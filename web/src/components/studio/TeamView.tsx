import { useState } from 'react'
import { Card, CardHeader, CardTitle, CardDescription, CardContent } from '../ui/card'
import { Button } from '../ui/button'
import { Input } from '../ui/input'
import { Badge } from '../ui/badge'
import { Alert, AlertDescription, AlertTitle } from '../ui/alert'
import {
  Users,
  UserPlus,
  ShieldCheck,
  Bot,
  Mail,
  CheckCircle2,
  Clock
} from 'lucide-react'
import { cn } from '../../lib/utils'

interface TeamMember {
  id: string
  name: string
  email: string
  role: 'owner' | 'admin' | 'developer' | 'agent'
  status: 'active' | 'invited' | 'offline'
  avatarText: string
  joinedDate: string
  description?: string
}

const DEFAULT_MEMBERS: TeamMember[] = [
  {
    id: 'user-buchin',
    name: 'Buchin (Owner)',
    email: 'buchin@dropsugar.com',
    role: 'owner',
    status: 'active',
    avatarText: 'BU',
    joinedDate: '2026-10-16',
    description: 'Pemilik instance tunggal platform Cuan & Product Owner Sprint.',
  },
  {
    id: 'agent-agy',
    name: 'agy (Dev Team)',
    email: 'agy@fural.space',
    role: 'agent',
    status: 'active',
    avatarText: 'AG',
    joinedDate: '2026-10-18',
    description: 'Autonomous Coding & Implementation Agent dengan terminal execution suite.',
  },
  {
    id: 'agent-jean',
    name: 'Jean (Scrum Master)',
    email: 'jean@fural.space',
    role: 'agent',
    status: 'active',
    avatarText: 'JN',
    joinedDate: '2026-10-18',
    description: 'Scrum Master & Backlog alignment lead untuk Sprint Planning & Review.',
  },
  {
    id: 'agent-webmaster',
    name: 'Webmaster AI Studio',
    email: 'webmaster@cuan.local',
    role: 'agent',
    status: 'active',
    avatarText: 'WM',
    joinedDate: '2026-10-17',
    description: 'Engine otomatis untuk injeksi skrip WordPress, asset ZIP, dan config tweaker.',
  },
]

export function TeamView() {
  const [members, setMembers] = useState<TeamMember[]>(DEFAULT_MEMBERS)
  const [isInviteModalOpen, setIsInviteModalOpen] = useState(false)
  const [inviteEmail, setInviteEmail] = useState('')
  const [inviteName, setInviteName] = useState('')
  const [inviteRole, setInviteRole] = useState<'admin' | 'developer'>('developer')
  const [toastMessage, setToastMessage] = useState<string | null>(null)

  const handleInviteSubmit = (e: React.FormEvent) => {
    e.preventDefault()
    if (!inviteEmail.trim()) return

    const newMember: TeamMember = {
      id: `member-${Date.now()}`,
      name: inviteName.trim() || inviteEmail.split('@')[0],
      email: inviteEmail.trim(),
      role: inviteRole,
      status: 'invited',
      avatarText: (inviteName.trim() || inviteEmail).slice(0, 2).toUpperCase(),
      joinedDate: new Date().toISOString().split('T')[0],
      description: `Undangan kolaborator via email sebagai ${inviteRole}.`,
    }

    setMembers((prev) => [newMember, ...prev])
    setIsInviteModalOpen(false)
    setInviteEmail('')
    setInviteName('')
    setToastMessage(`Undangan berhasil dikirim ke ${newMember.email}`)
    setTimeout(() => setToastMessage(null), 4000)
  }

  const getRoleBadge = (role: TeamMember['role']) => {
    switch (role) {
      case 'owner':
        return <Badge className="bg-amber-500/10 text-amber-400 border-amber-500/30 text-[10px]">Owner</Badge>
      case 'admin':
        return <Badge className="bg-primary/10 text-primary border-primary/30 text-[10px]">Admin</Badge>
      case 'developer':
        return <Badge className="bg-emerald-500/10 text-emerald-400 border-emerald-500/30 text-[10px]">Developer</Badge>
      case 'agent':
        return <Badge className="bg-purple-500/10 text-purple-400 border-purple-500/30 text-[10px] gap-1"><Bot className="h-2.5 w-2.5" /> Agent</Badge>
      default:
        return <Badge variant="outline" className="text-[10px]">Member</Badge>
    }
  }

  return (
    <div className="w-full max-w-6xl mx-auto space-y-6 animate-in fade-in-50">
      {/* Top Banner / Header */}
      <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-4 pb-4 border-b border-border">
        <div className="space-y-1">
          <div className="flex items-center gap-2">
            <div className="flex h-8 w-8 items-center justify-center bg-primary/10 text-primary border border-primary/20">
              <Users className="h-4 w-4" />
            </div>
            <h1 className="text-xl font-bold tracking-tight text-foreground">
              Team & Workspace Collaborators
            </h1>
          </div>
          <p className="text-xs text-muted-foreground">
            Kelola anggota tim pengembang, pemilik instance, dan autonomous AI agents yang berkolaborasi di Cuan.
          </p>
        </div>

        <Button
          onClick={() => setIsInviteModalOpen(true)}
          className="bg-primary text-primary-foreground hover:bg-primary/90 text-xs font-semibold gap-2 self-start sm:self-auto"
        >
          <UserPlus className="h-3.5 w-3.5" />
          Invite Team Member
        </Button>
      </div>

      {/* Notification Toast */}
      {toastMessage && (
        <Alert className="bg-emerald-500/10 border-emerald-500/30 text-emerald-400">
          <CheckCircle2 className="h-4 w-4 text-emerald-400" />
          <AlertTitle className="text-xs font-bold">Sukses</AlertTitle>
          <AlertDescription className="text-xs">{toastMessage}</AlertDescription>
        </Alert>
      )}

      {/* Stats Cards */}
      <div className="grid grid-cols-1 sm:grid-cols-3 gap-4">
        <Card className="bg-card text-card-foreground border-border">
          <CardContent className="p-4 flex items-center justify-between">
            <div className="space-y-1">
              <span className="text-[11px] text-muted-foreground uppercase font-semibold">Total Members</span>
              <div className="text-2xl font-bold text-foreground">
                {members.filter((m) => m.role !== 'agent').length}
              </div>
            </div>
            <div className="p-2.5 bg-primary/10 text-primary border border-primary/20">
              <Users className="h-5 w-5" />
            </div>
          </CardContent>
        </Card>

        <Card className="bg-card text-card-foreground border-border">
          <CardContent className="p-4 flex items-center justify-between">
            <div className="space-y-1">
              <span className="text-[11px] text-muted-foreground uppercase font-semibold">Autonomous Agents</span>
              <div className="text-2xl font-bold text-purple-400">
                {members.filter((m) => m.role === 'agent').length}
              </div>
            </div>
            <div className="p-2.5 bg-purple-500/10 text-purple-400 border border-purple-500/20">
              <Bot className="h-5 w-5" />
            </div>
          </CardContent>
        </Card>

        <Card className="bg-card text-card-foreground border-border">
          <CardContent className="p-4 flex items-center justify-between">
            <div className="space-y-1">
              <span className="text-[11px] text-muted-foreground uppercase font-semibold">Access Model</span>
              <div className="text-sm font-bold text-emerald-400 mt-1">
                Single-Owner + Dev Team
              </div>
            </div>
            <div className="p-2.5 bg-emerald-500/10 text-emerald-400 border border-emerald-500/20">
              <ShieldCheck className="h-5 w-5" />
            </div>
          </CardContent>
        </Card>
      </div>

      {/* Members List */}
      <Card className="bg-card text-card-foreground border-border">
        <CardHeader className="p-4 border-b border-border flex flex-row items-center justify-between">
          <div>
            <CardTitle className="text-sm font-semibold text-foreground">Anggota & AI Agents Aktif</CardTitle>
            <CardDescription className="text-xs text-muted-foreground">
              Daftar seluruh kolaborator manusia dan autonomous agents yang terhubung dengan instance ini.
            </CardDescription>
          </div>
          <Badge variant="outline" className="text-xs">
            {members.length} Total Kolaborator
          </Badge>
        </CardHeader>
        <CardContent className="p-0">
          <div className="divide-y divide-border">
            {members.map((member) => (
              <div
                key={member.id}
                className="p-4 flex flex-col sm:flex-row sm:items-center justify-between gap-3 hover:bg-muted/30 transition-colors"
              >
                <div className="flex items-start sm:items-center gap-3">
                  <div
                    className={cn(
                      'flex h-10 w-10 shrink-0 items-center justify-center font-bold text-xs border shadow-xs',
                      member.role === 'agent'
                        ? 'bg-purple-500/10 text-purple-400 border-purple-500/30'
                        : member.role === 'owner'
                        ? 'bg-amber-500/10 text-amber-400 border-amber-500/30'
                        : 'bg-primary/10 text-primary border-primary/30'
                    )}
                  >
                    {member.avatarText}
                  </div>
                  <div className="space-y-0.5">
                    <div className="flex items-center gap-2">
                      <span className="font-semibold text-xs text-foreground">{member.name}</span>
                      {getRoleBadge(member.role)}
                      {member.status === 'invited' && (
                        <Badge variant="outline" className="text-[9px] border-amber-500/40 text-amber-400">
                          Pending
                        </Badge>
                      )}
                    </div>
                    <div className="flex items-center gap-3 text-[11px] text-muted-foreground">
                      <span className="flex items-center gap-1 font-mono">
                        <Mail className="h-3 w-3" />
                        {member.email}
                      </span>
                      <span>•</span>
                      <span className="flex items-center gap-1">
                        <Clock className="h-3 w-3" />
                        Sejak {member.joinedDate}
                      </span>
                    </div>
                    {member.description && (
                      <p className="text-[11px] text-muted-foreground/80 pt-0.5 line-clamp-1">
                        {member.description}
                      </p>
                    )}
                  </div>
                </div>

                <div className="flex items-center gap-2 self-end sm:self-center">
                  <span
                    className={cn(
                      'inline-flex items-center gap-1 text-[10px] font-mono px-2 py-0.5 border',
                      member.status === 'active'
                        ? 'border-emerald-500/40 bg-emerald-500/10 text-emerald-400'
                        : 'border-amber-500/40 bg-amber-500/10 text-amber-400'
                    )}
                  >
                    <span
                      className={cn(
                        'h-1.5 w-1.5 rounded-full',
                        member.status === 'active' ? 'bg-emerald-400' : 'bg-amber-400'
                      )}
                    />
                    {member.status === 'active' ? 'ONLINE' : 'PENDING'}
                  </span>
                </div>
              </div>
            ))}
          </div>
        </CardContent>
      </Card>

      {/* Invite Member Modal */}
      {isInviteModalOpen && (
        <div className="fixed inset-0 z-50 flex items-center justify-center p-4 bg-black/75 backdrop-blur-sm animate-in fade-in-50">
          <Card className="w-full max-w-md bg-card border-border text-card-foreground shadow-2xl">
            <CardHeader className="p-4 border-b border-border">
              <CardTitle className="text-sm font-bold flex items-center gap-2">
                <UserPlus className="h-4 w-4 text-primary" />
                Invite Team Member
              </CardTitle>
              <CardDescription className="text-xs text-muted-foreground">
                Kirimkan tautan bergabung ke kolaborator baru untuk mengelola workspace.
              </CardDescription>
            </CardHeader>
            <form onSubmit={handleInviteSubmit}>
              <CardContent className="p-4 space-y-3.5">
                <div className="space-y-1">
                  <label className="text-xs font-semibold text-foreground">Nama Lengkap</label>
                  <Input
                    type="text"
                    placeholder="misal: Alex Developer"
                    value={inviteName}
                    onChange={(e) => setInviteName(e.target.value)}
                  />
                </div>

                <div className="space-y-1">
                  <label className="text-xs font-semibold text-foreground">Email Kolaborator *</label>
                  <Input
                    type="email"
                    placeholder="alex@domain.com"
                    value={inviteEmail}
                    onChange={(e) => setInviteEmail(e.target.value)}
                    required
                  />
                </div>

                <div className="space-y-1">
                  <label className="text-xs font-semibold text-foreground">Peran / Role</label>
                  <div className="grid grid-cols-2 gap-2">
                    <button
                      type="button"
                      onClick={() => setInviteRole('developer')}
                      className={cn(
                        'p-2.5 text-xs text-left border transition-all',
                        inviteRole === 'developer'
                          ? 'border-primary bg-primary/10 text-foreground font-semibold'
                          : 'border-border bg-card/40 text-muted-foreground'
                      )}
                    >
                      <div className="font-semibold text-xs">Developer</div>
                      <div className="text-[10px] text-muted-foreground">Akses task & deployment</div>
                    </button>
                    <button
                      type="button"
                      onClick={() => setInviteRole('admin')}
                      className={cn(
                        'p-2.5 text-xs text-left border transition-all',
                        inviteRole === 'admin'
                          ? 'border-primary bg-primary/10 text-foreground font-semibold'
                          : 'border-border bg-card/40 text-muted-foreground'
                      )}
                    >
                      <div className="font-semibold text-xs">Administrator</div>
                      <div className="text-[10px] text-muted-foreground">Akses penuh server & settings</div>
                    </button>
                  </div>
                </div>
              </CardContent>
              <div className="p-4 border-t border-border flex items-center justify-end gap-2 bg-muted/20">
                <Button
                  type="button"
                  variant="outline"
                  onClick={() => setIsInviteModalOpen(false)}
                  className="text-xs"
                >
                  Batal
                </Button>
                <Button
                  type="submit"
                  disabled={!inviteEmail.trim()}
                  className="bg-primary text-primary-foreground hover:bg-primary/90 text-xs font-semibold"
                >
                  Kirim Undangan
                </Button>
              </div>
            </form>
          </Card>
        </div>
      )}
    </div>
  )
}
