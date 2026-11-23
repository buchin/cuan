import React, { useState, useEffect, useRef } from 'react'
import { useLanguage } from '../../context/LanguageContext'
import { teamApi } from '../../api/team'
import { X, Copy, Check, MessageSquare, UserPlus, Shield, User, Loader2 } from 'lucide-react'
import { FormField, Input } from '../../components/ui/forms'

interface InviteMemberModalProps {
  open: boolean
  onClose: () => void
  onInviteCreated?: () => void
}

export function InviteMemberModal({
  open,
  onClose,
  onInviteCreated,
}: InviteMemberModalProps) {
  const { t } = useLanguage()
  const [email, setEmail] = useState('')
  const [name, setName] = useState('')
  const [role, setRole] = useState<'member' | 'owner'>('member')
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')
  const [copied, setCopied] = useState(false)
  const [inviteResult, setInviteResult] = useState<{
    token: string
    inviteUrl: string
    email: string
    role: string
    name: string
  } | null>(null)

  const dialogRef = useRef<HTMLDivElement>(null)
  const emailInputRef = useRef<HTMLInputElement>(null)

  useEffect(() => {
    if (open) {
      setEmail('')
      setName('')
      setRole('member')
      setError('')
      setCopied(false)
      setInviteResult(null)
      setTimeout(() => emailInputRef.current?.focus(), 50)
    }
  }, [open])

  useEffect(() => {
    function handleKeyDown(e: KeyboardEvent) {
      if (!open) return
      if (e.key === 'Escape') {
        onClose()
      }
    }
    window.addEventListener('keydown', handleKeyDown)
    return () => window.removeEventListener('keydown', handleKeyDown)
  }, [open, onClose])

  if (!open) return null

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    if (busy) return
    setError('')
    setBusy(true)

    try {
      const resp = await teamApi.createInvite({
        email: email.trim().toLowerCase(),
        role,
      })

      const rawUrl = resp.invite_url || `/invite/accept?token=${encodeURIComponent(resp.token)}`
      const fullUrl = rawUrl.startsWith('http')
        ? rawUrl
        : `${window.location.origin}${rawUrl}`

      setInviteResult({
        token: resp.token,
        inviteUrl: fullUrl,
        email: email.trim(),
        role,
        name: name.trim(),
      })
      onInviteCreated?.()
    } catch (err) {
      setError(
        err instanceof Error ? err.message : t('Gagal membuat undangan staf.')
      )
    } finally {
      setBusy(false)
    }
  }

  const handleCopyLink = async () => {
    if (!inviteResult) return
    try {
      await navigator.clipboard.writeText(inviteResult.inviteUrl)
      setCopied(true)
      setTimeout(() => setCopied(false), 2500)
    } catch {
      // Fallback
      const ta = document.createElement('textarea')
      ta.value = inviteResult.inviteUrl
      document.body.appendChild(ta)
      ta.select()
      document.execCommand('copy')
      document.body.removeChild(ta)
      setCopied(true)
      setTimeout(() => setCopied(false), 2500)
    }
  }

  const handleSendWhatsApp = () => {
    if (!inviteResult) return
    const recipientName = inviteResult.name ? ` ${inviteResult.name}` : ''
    const roleTitle = inviteResult.role === 'owner' ? 'Owner' : 'Member'
    const message = `Halo${recipientName}! Anda diundang untuk bergabung ke tim Cuan sebagai ${roleTitle}. Silakan klik tautan berikut untuk mengaktifkan akun Anda: ${inviteResult.inviteUrl}`
    const waUrl = `https://api.whatsapp.com/send?text=${encodeURIComponent(message)}`
    window.open(waUrl, '_blank', 'noopener,noreferrer')
  }

  return (
    <div
      role="presentation"
      className="fixed inset-0 z-50 flex items-center justify-center bg-black/60 backdrop-blur-sm p-4 overflow-y-auto"
      onClick={(e) => {
        if (e.target === e.currentTarget) onClose()
      }}
    >
      <div
        ref={dialogRef}
        role="dialog"
        aria-modal="true"
        aria-labelledby="invite-modal-title"
        className="relative w-full max-w-lg rounded-xl border border-border bg-card p-6 text-card-foreground shadow-2xl transition-all"
      >
        <button
          type="button"
          onClick={onClose}
          className="absolute right-4 top-4 rounded-lg p-2 text-muted-foreground hover:bg-muted hover:text-foreground transition-colors"
          aria-label={t('Tutup')}
        >
          <X className="h-4 w-4" />
        </button>

        {!inviteResult ? (
          <div>
            <div className="flex items-center gap-3 mb-4">
              <div className="flex h-10 w-10 shrink-0 items-center justify-center rounded-xl bg-primary/10 text-primary border border-primary/20">
                <UserPlus className="h-5 w-5" />
              </div>
              <div>
                <h2 id="invite-modal-title" className="pr-8 text-lg font-semibold tracking-tight text-foreground">
                  {t('Undang Anggota Tim')}
                </h2>
              </div>
            </div>

            {error && (
              <div role="alert" className="mb-4 rounded-lg border border-destructive/30 bg-destructive/10 p-3 text-xs text-destructive">
                {t(error)}
              </div>
            )}

            <form onSubmit={handleSubmit} className="space-y-4">
              <FormField label={t('Alamat Email Staf')} required htmlFor="invite-email">
                <Input
                  ref={emailInputRef}
                  id="invite-email"
                  type="email"
                  required
                  placeholder={t('misal: staf@workspace.com')}
                  value={email}
                  onChange={(e) => setEmail(e.target.value)}
                  disabled={busy}
                />
              </FormField>

              <FormField label={t('Nama Staf / Panggilan (Opsional)')} htmlFor="invite-name">
                <Input
                  id="invite-name"
                  type="text"
                  placeholder={t('misal: Budi Santoso / Tim Ruang Kerja')}
                  value={name}
                  onChange={(e) => setName(e.target.value)}
                  disabled={busy}
                />
              </FormField>

              <div>
                <label className="block text-xs font-medium text-foreground mb-2">
                  {t('Peran Pengguna')}
                </label>
                <div className="grid grid-cols-1 sm:grid-cols-2 gap-2.5">
                  <label
                    className={`flex items-start gap-3 p-3 rounded-lg border cursor-pointer transition-colors ${
                      role === 'member'
                        ? 'border-primary bg-primary/5 text-foreground ring-1 ring-primary'
                        : 'border-border bg-card hover:bg-accent/40 text-muted-foreground'
                    }`}
                  >
                    <input
                      type="radio"
                      name="invite-role"
                      value="member"
                      checked={role === 'member'}
                      onChange={() => setRole('member')}
                      className="mt-0.5 text-primary focus:ring-primary"
                    />
                    <div className="text-left leading-tight">
                      <div className="font-semibold text-xs text-foreground flex items-center gap-1.5">
                        <User className="h-3.5 w-3.5 text-blue-500" />
                        <span>{t('Member')}</span>
                        <span className="text-[10px] bg-blue-500/15 text-blue-600 dark:text-blue-400 px-1.5 py-0.2 rounded font-mono font-normal">
                          {t('Default')}
                        </span>
                      </div>
                      <p className="mt-1 text-[11px] text-muted-foreground leading-snug">
                        {t('Akses operasional task, inbox, & asisten AI. Tidak bisa ubah setelan AI.')}
                      </p>
                    </div>
                  </label>

                  <label
                    className={`flex items-start gap-3 p-3 rounded-lg border cursor-pointer transition-colors ${
                      role === 'owner'
                        ? 'border-primary bg-primary/5 text-foreground ring-1 ring-primary'
                        : 'border-border bg-card hover:bg-accent/40 text-muted-foreground'
                    }`}
                  >
                    <input
                      type="radio"
                      name="invite-role"
                      value="owner"
                      checked={role === 'owner'}
                      onChange={() => setRole('owner')}
                      className="mt-0.5 text-primary focus:ring-primary"
                    />
                    <div className="text-left leading-tight">
                      <div className="font-semibold text-xs text-foreground flex items-center gap-1.5">
                        <Shield className="h-3.5 w-3.5 text-green-500" />
                        <span>{t('Owner')}</span>
                      </div>
                      <p className="mt-1 text-[11px] text-muted-foreground leading-snug">
                        {t('Kendali penuh atas tim staf, konfigurasi penyedia AI, agen, dan sistem.')}
                      </p>
                    </div>
                  </label>
                </div>
              </div>

              <div className="flex items-center justify-end gap-2.5 pt-3 border-t border-border">
                <button
                  type="button"
                  onClick={onClose}
                  disabled={busy}
                  className="action-secondary min-h-11 px-4 text-xs"
                >
                  {t('Batal')}
                </button>
                <button
                  type="submit"
                  disabled={busy || !email.trim()}
                  className="action-primary min-h-11 px-4 text-xs flex items-center gap-2"
                >
                  {busy ? (
                    <>
                      <Loader2 className="h-4 w-4 animate-spin" />
                      <span>{t('Membuat Undangan…')}</span>
                    </>
                  ) : (
                    <>
                      <UserPlus className="h-4 w-4" />
                      <span>{t('Buat Tautan Undangan')}</span>
                    </>
                  )}
                </button>
              </div>
            </form>
          </div>
        ) : (
          <div className="space-y-4">
            <div className="flex items-center gap-3">
              <div className="flex h-10 w-10 shrink-0 items-center justify-center rounded-xl bg-green-500/10 text-green-600 dark:text-green-400 border border-green-500/20">
                <Check className="h-5 w-5" />
              </div>
              <div>
                <h2 id="invite-modal-title" className="pr-8 text-lg font-semibold tracking-tight text-foreground">
                  {t('Tautan Undangan Siap')}
                </h2>
                <p className="text-xs text-muted-foreground">
                  {t('Undangan untuk')} <strong className="text-foreground">{inviteResult.email}</strong> ({inviteResult.role === 'owner' ? t('Owner') : t('Member')}). {t('Berlaku selama 7 hari.')}
                </p>
              </div>
            </div>

            <div className="rounded-xl border border-border/60 bg-muted/30 p-3.5 space-y-2">
              <FormField label={t('Tautan Aktivasi Akun')} htmlFor="generated-invite-url">
                <Input
                  id="generated-invite-url"
                  readOnly
                  value={inviteResult.inviteUrl}
                  onClick={(e) => (e.target as HTMLInputElement).select()}
                  mono
                  className="text-xs"
                />
              </FormField>
            </div>

            <div className="flex flex-col sm:flex-row gap-2.5 pt-2">
              <button
                type="button"
                onClick={handleCopyLink}
                className="action-primary min-h-11 flex-1 flex items-center justify-center gap-2 text-xs font-semibold"
              >
                {copied ? (
                  <>
                    <Check className="h-4 w-4 text-white" />
                    <span>{t('Tautan Tersalin!')}</span>
                  </>
                ) : (
                  <>
                    <Copy className="h-4 w-4" />
                    <span>{t('Salin Tautan Undangan')}</span>
                  </>
                )}
              </button>

              <button
                type="button"
                onClick={handleSendWhatsApp}
                className="action-secondary inline-flex min-h-11 items-center justify-center gap-2 px-4 text-xs font-semibold"
              >
                <MessageSquare className="h-4 w-4" />
                <span>{t('Kirim via WhatsApp')}</span>
              </button>
            </div>

            <div className="pt-2 border-t border-border flex justify-end">
              <button
                type="button"
                onClick={onClose}
                className="action-secondary min-h-11 px-5 text-xs"
              >
                {t('Selesai')}
              </button>
            </div>
          </div>
        )}
      </div>
    </div>
  )
}
