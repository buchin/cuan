import { useCallback, useEffect, useState } from 'react'
import {
  Users,
  UserPlus,
  Shield,
  User,
  Clock,
  Trash2,
  Search,
  AlertCircle,
  RefreshCw,
} from 'lucide-react'
import { useLanguage } from '../../context/LanguageContext'
import { useAuth } from '../../context/AuthContext'
import { teamApi, type TeamMember, type UserInvitation } from '../../api/team'
import { Badge } from '../../components/ui/badge'
import { InviteMemberModal } from './InviteMemberModal'
import { HairlineList } from '../../components/layout/HairlineList'

function formatJoinedDate(isoString?: string, language: string = 'id'): string {
  if (!isoString) return '-'
  const date = new Date(isoString)
  if (isNaN(date.getTime())) return isoString
  return date.toLocaleDateString(language === 'en' ? 'en-US' : 'id-ID', {
    day: 'numeric',
    month: 'short',
    year: 'numeric',
  })
}

export function TeamMembersView({ showHeading = true }: { showHeading?: boolean }) {
  const { t, language } = useLanguage()
  const { user, isOwner } = useAuth()

  const [members, setMembers] = useState<TeamMember[]>([])
  const [invitations, setInvitations] = useState<UserInvitation[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')
  const [notice, setNotice] = useState('')
  const [isInviteModalOpen, setIsInviteModalOpen] = useState(false)
  const [busyId, setBusyId] = useState('')
  const [search, setSearch] = useState('')

  const loadTeamMembers = useCallback(async () => {
    setLoading(true)
    setError('')
    try {
      const data = await teamApi.listMembers()
      setMembers(data.members || [])
      setInvitations(data.invitations || [])
    } catch (err) {
      setError(
        err instanceof Error ? err.message : t('Gagal memuat daftar anggota tim.')
      )
    } finally {
      setLoading(false)
    }
  }, [t])

  useEffect(() => {
    loadTeamMembers()
  }, [loadTeamMembers])

  const handleRemoveMember = async (memberId: string, memberName: string) => {
    if (!isOwner) return
    if (
      !window.confirm(
        t('Hapus akses untuk {name}? Akun staf ini tidak akan dapat masuk lagi.').replace('{name}', memberName)
      )
    )
      return
    setBusyId(memberId)
    try {
      await teamApi.deleteMember(memberId)
      setNotice(t('Akses untuk {name} berhasil dihapus.').replace('{name}', memberName))
      await loadTeamMembers()
    } catch (err) {
      setError(
        err instanceof Error
          ? err.message
          : t('Gagal menghapus anggota tim.')
      )
    } finally {
      setBusyId('')
    }
  }

  const handleRevokeInvite = async (inviteId: string, inviteEmail: string) => {
    if (!isOwner) return
    if (
      !window.confirm(
        t('Cabut undangan untuk {email}? Tautan undangan ini akan kedaluwarsa seketika.').replace('{email}', inviteEmail)
      )
    )
      return
    setBusyId(inviteId)
    try {
      await teamApi.deleteInvite(inviteId)
      setNotice(t('Undangan untuk {email} berhasil dicabut.').replace('{email}', inviteEmail))
      await loadTeamMembers()
    } catch (err) {
      setError(
        err instanceof Error ? err.message : t('Gagal mencabut undangan.')
      )
    } finally {
      setBusyId('')
    }
  }

  const filteredMembers = members.filter((m) => {
    if (!search.trim()) return true
    const q = search.toLowerCase()
    return (
      (m.name || '').toLowerCase().includes(q) ||
      (m.email || '').toLowerCase().includes(q) ||
      (m.role || '').toLowerCase().includes(q)
    )
  })

  const filteredInvites = invitations.filter((inv) => {
    if (!search.trim()) return true
    const q = search.toLowerCase()
    return (
      (inv.email || '').toLowerCase().includes(q) ||
      (inv.role || '').toLowerCase().includes(q)
    )
  })

  return (
    <div className="w-full space-y-6 pb-8">
      {/* Header */}
      {(showHeading || isOwner) && (
        <div className={showHeading ? "flex flex-col sm:flex-row sm:items-center justify-between gap-4 pb-2 border-b border-border/40" : "flex justify-end"}>
          {showHeading && <div>
            <h2 className="text-xl font-bold tracking-tight text-foreground flex items-center gap-2">
              <Users className="h-5 w-5 text-primary" />
              <span>{t('Anggota Tim')}</span>
            </h2>
            <p className="mt-1 text-xs sm:text-sm text-muted-foreground">
              {t('Kelola rekan kerja dan staf yang memiliki akses ke akun ruang kerja Cuan.')}
            </p>
          </div>}
          {isOwner && (
            <button
              type="button"
              onClick={() => setIsInviteModalOpen(true)}
              className="inline-flex min-h-11 items-center justify-center gap-2 rounded-xl bg-primary px-4 py-2.5 text-xs font-semibold text-primary-foreground shadow-sm hover:bg-primary/90 transition-colors shrink-0"
            >
              <UserPlus className="h-4 w-4" />
              <span>{t('Undang Anggota')}</span>
            </button>
          )}
        </div>
      )}

      {/* Notifications / Alerts */}
      {notice && (
        <div role="status" className="rounded-xl border border-primary/20 bg-primary/10 p-3.5 text-xs text-primary flex items-center justify-between">
          <span>{notice}</span>
          <button type="button" onClick={() => setNotice('')} className="underline text-xs ml-2">
            {t('Tutup')}
          </button>
        </div>
      )}

      {error && (
        <div role="alert" className="rounded-xl border border-destructive/30 bg-destructive/10 p-3.5 text-xs text-destructive flex items-center justify-between">
          <div className="flex items-center gap-2">
            <AlertCircle className="h-4 w-4 shrink-0" />
            <span>{error}</span>
          </div>
          <button type="button" onClick={loadTeamMembers} className="underline font-medium hover:opacity-80">
            {t('Coba lagi')}
          </button>
        </div>
      )}

      {/* Search Bar */}
      {(members.length > 1 || invitations.length > 0) && (
        <div className="relative">
          <Search className="pointer-events-none absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" />
          <input
            className="field-control pl-9 w-full text-xs sm:text-sm rounded-xl"
            placeholder={t('Cari nama atau email staf…')}
            value={search}
            onChange={(e) => setSearch(e.target.value)}
          />
          {search && (
            <button
              type="button"
              onClick={() => setSearch('')}
              className="absolute right-3 top-1/2 -translate-y-1/2 text-xs text-muted-foreground hover:text-foreground"
            >
              {t('Clear')}
            </button>
          )}
        </div>
      )}

      {/* Loading state */}
      {loading ? (
        <div className="flex items-center justify-center p-12 text-muted-foreground gap-2">
          <RefreshCw className="h-4 w-4 animate-spin" />
          <span className="text-xs">{t('Memuat daftar anggota tim…')}</span>
        </div>
      ) : members.length === 0 && invitations.length === 0 ? (
        /* Empty State */
        <div className="rounded-2xl border border-dashed border-border/60 p-8 sm:p-12 text-center bg-card/40 space-y-3">
          <div className="mx-auto flex h-12 w-12 items-center justify-center rounded-2xl bg-muted text-muted-foreground">
            <Users className="h-6 w-6" />
          </div>
          <h3 className="font-semibold text-foreground text-base">{t('Belum Ada Anggota Tim')}</h3>
          <p className="text-xs sm:text-sm text-muted-foreground max-w-sm mx-auto">
            {t('Undang staf atau rekan bisnis Anda untuk mulai berkolaborasi di Cuan.')}
          </p>
          {isOwner && (
            <div className="pt-2">
              <button
                type="button"
                onClick={() => setIsInviteModalOpen(true)}
                className="inline-flex min-h-11 items-center justify-center gap-2 rounded-xl bg-primary px-4 py-2.5 text-xs font-semibold text-primary-foreground shadow-sm hover:bg-primary/90 transition-colors"
              >
                <UserPlus className="h-4 w-4" />
                <span>{t('Undang Anggota Pertama')}</span>
              </button>
            </div>
          )}
        </div>
      ) : (
        <div className="space-y-6">
          {/* Active Members Section */}
          <div className="space-y-3">
            <div className="flex items-center justify-between">
              <h3 className="text-xs font-semibold uppercase tracking-wider text-muted-foreground flex items-center gap-1.5">
                <User className="h-3.5 w-3.5" />
                <span>{t('Staf Aktif')} ({filteredMembers.length})</span>
              </h3>
            </div>

            <div className="rounded-2xl border border-border/40 bg-card/40 overflow-hidden shadow-xs">
              <HairlineList role="list" aria-label={t('Daftar Anggota Tim')}>
                {filteredMembers.map((member) => {
                  const displayName = member.name || member.username || member.email
                  const initials = displayName.slice(0, 2).toUpperCase()
                  const isSelf = member.id === user?.id
                  const isMemberOwner = member.role === 'owner'

                  return (
                    <div
                      role="listitem"
                      key={member.id}
                      className="p-4 sm:p-5 flex flex-col sm:flex-row sm:items-center justify-between gap-3"
                    >
                      <div className="flex items-start gap-3.5 min-w-0">
                        <div
                          className={`flex h-10 w-10 shrink-0 items-center justify-center rounded-full font-bold text-xs ${
                            isMemberOwner
                              ? 'bg-emerald-500/15 text-emerald-700 dark:text-emerald-400 border border-emerald-500/30'
                              : 'bg-primary/15 text-primary border border-primary/30'
                          }`}
                        >
                          {initials}
                        </div>

                        <div className="min-w-0 space-y-1">
                          <div className="flex flex-wrap items-center gap-2">
                            <span className="font-semibold text-sm text-foreground truncate">
                              {displayName}
                            </span>
                            {isSelf && (
                              <Badge variant="neutral" size="sm" className="px-2 py-0.5 font-mono">
                                {t('Anda')}
                              </Badge>
                            )}
                            <Badge
                              variant={isMemberOwner ? 'success' : 'default'}
                              className="text-[11px] font-semibold"
                            >
                              {isMemberOwner ? <Shield className="h-3 w-3" /> : <User className="h-3 w-3" />}
                              <span className="capitalize">{isMemberOwner ? t('Owner') : t('Member')}</span>
                            </Badge>
                            <Badge variant="success" size="sm" className="px-2 py-0.5">
                              <span className="h-1.5 w-1.5 rounded-full bg-emerald-500" />
                              <span>{t('Aktif')}</span>
                            </Badge>
                          </div>

                          <div className="flex flex-wrap items-center gap-x-3 gap-y-1 text-xs text-muted-foreground">
                            <span>{member.email}</span>
                            <span aria-hidden="true">·</span>
                            <span>{t('Bergabung')} {formatJoinedDate(member.created_at, language)}</span>
                          </div>
                        </div>
                      </div>

                      {/* Member Actions */}
                      {isOwner && !isSelf && (
                        <div className="flex items-center gap-2 self-end sm:self-center shrink-0">
                          <button
                            type="button"
                            disabled={busyId === member.id}
                            onClick={() => handleRemoveMember(member.id, displayName)}
                            className="inline-flex min-h-11 items-center gap-1.5 px-3 py-1.5 rounded-lg text-xs font-medium text-destructive hover:bg-destructive/10 transition-colors disabled:opacity-50"
                            title={t('Hapus akses staf')}
                          >
                            <Trash2 className="h-3.5 w-3.5" />
                            <span>{t('Hapus Akses')}</span>
                          </button>
                        </div>
                      )}
                    </div>
                  )
                })}
              </HairlineList>
            </div>
          </div>

          {/* Pending Invitations Section */}
          {filteredInvites.length > 0 && (
            <div className="space-y-3 pt-4">
              <div className="flex items-center justify-between">
                <h3 className="text-xs font-semibold uppercase tracking-wider text-muted-foreground flex items-center gap-1.5">
                  <Clock className="h-3.5 w-3.5" />
                  <span>{t('Undangan Menunggu')} ({filteredInvites.length})</span>
                </h3>
              </div>

              <div className="rounded-2xl border border-border/40 bg-card/40 overflow-hidden shadow-xs">
                <HairlineList role="list" aria-label={t('Daftar Undangan Menunggu')}>
                  {filteredInvites.map((invite) => (
                    <div
                      role="listitem"
                      key={invite.id}
                      className="p-4 sm:p-5 flex flex-col sm:flex-row sm:items-center justify-between gap-3"
                    >
                      <div className="flex items-start gap-3 min-w-0">
                        <div className="flex h-9 w-9 shrink-0 items-center justify-center rounded-full bg-amber-500/10 text-amber-600 dark:text-amber-400 border border-amber-500/30">
                          <Clock className="h-4 w-4" />
                        </div>
                        <div className="min-w-0 space-y-1">
                          <div className="flex items-center gap-2">
                            <span className="font-medium text-sm text-foreground truncate">{invite.email}</span>
                            <Badge variant="warning" size="sm" className="px-2 py-0.5 font-semibold">
                              {t('Menunggu Konfirmasi')}
                            </Badge>
                            <Badge variant="neutral" size="sm" className="px-2 py-0.5 capitalize">
                              {invite.role}
                            </Badge>
                          </div>
                          <p className="text-xs text-muted-foreground">
                            {t('Dikirim')} {formatJoinedDate(invite.created_at, language)}
                          </p>
                        </div>
                      </div>

                      {isOwner && (
                        <div className="flex items-center gap-2 self-end sm:self-center shrink-0">
                          <button
                            type="button"
                            disabled={busyId === invite.id}
                            onClick={() => handleRevokeInvite(invite.id, invite.email)}
                            className="inline-flex min-h-11 items-center gap-1.5 px-3 py-1.5 rounded-lg text-xs font-medium text-destructive hover:bg-destructive/10 transition-colors disabled:opacity-50"
                          >
                            <Trash2 className="h-3.5 w-3.5" />
                            <span>{t('Cabut')}</span>
                          </button>
                        </div>
                      )}
                    </div>
                  ))}
                </HairlineList>
              </div>
            </div>
          )}
        </div>
      )}

      {/* Invite Modal */}
      <InviteMemberModal
        open={isInviteModalOpen}
        onClose={() => setIsInviteModalOpen(false)}
        onInviteCreated={() => {
          loadTeamMembers()
          setNotice(t('Undangan berhasil dibuat. Tautan dapat dibagikan langsung ke rekan kerja.'))
        }}
      />
    </div>
  )
}
