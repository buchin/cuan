import React, { useState, useEffect } from 'react'
import { useNavigate, useLocation, Link } from 'react-router-dom'
import { useAuth } from '../../context/AuthContext'
import { useLanguage } from '../../context/LanguageContext'
import { teamApi } from '../../api/team'
import { CuanMark } from '../brand/CuanMark'
import { Badge } from '../ui/badge'
import { CheckCircle, AlertCircle, Loader2, ArrowRight, Shield, User } from 'lucide-react'

export function AcceptInvitePage() {
  const { t } = useLanguage()
  const { acceptInvite, setSession } = useAuth()
  const navigate = useNavigate()
  const location = useLocation()

  const query = new URLSearchParams(location.search)
  const token = query.get('token') || ''

  const [verifying, setVerifying] = useState(true)
  const [verifyError, setVerifyError] = useState('')
  const [inviteData, setInviteData] = useState<{
    email: string
    role: string
  } | null>(null)

  const [name, setName] = useState('')
  const [password, setPassword] = useState('')
  const [confirmPassword, setConfirmPassword] = useState('')
  const [submitting, setSubmitting] = useState(false)
  const [formError, setFormError] = useState('')
  const [success, setSuccess] = useState(false)

  useEffect(() => {
    async function checkToken() {
      if (!token.trim()) {
        setVerifying(false)
        setVerifyError(t('Tautan undangan tidak lengkap. Parameter token tidak ditemukan.'))
        return
      }

      try {
        const resp = await teamApi.verifyInvite(token)
        if (resp.valid) {
          setInviteData({
            email: resp.email,
            role: resp.role || 'member',
          })
        } else {
          setVerifyError(t('Undangan tidak valid atau sudah kedaluwarsa.'))
        }
      } catch (err) {
        const msg = err instanceof Error ? err.message : t('Gagal memverifikasi token undangan.')
        setVerifyError(msg)
      } finally {
        setVerifying(false)
      }
    }

    checkToken()
  }, [token, t])

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    setFormError('')

    if (!name.trim()) {
      setFormError(t('Silakan masukkan nama lengkap Anda.'))
      return
    }

    if (password.length < 8) {
      setFormError(t('Kata sandi minimal 8 karakter.'))
      return
    }

    if (password !== confirmPassword) {
      setFormError(t('Konfirmasi kata sandi tidak cocok.'))
      return
    }

    setSubmitting(true)
    try {
      if (acceptInvite) {
        await acceptInvite(token, name.trim(), password)
      } else {
        const resp = await teamApi.acceptInvite({
          token: token.trim(),
          name: name.trim(),
          password: password,
        })
        const receivedUser = resp.user || resp.owner
        if (resp.token && receivedUser) {
          setSession(resp.token, receivedUser)
        }
      }

      setSuccess(true)
      setTimeout(() => {
        navigate('/chat', { replace: true })
      }, 1200)
    } catch (err) {
      setFormError(
        err instanceof Error ? err.message : t('Gagal mengaktifkan akun. Silakan coba lagi.')
      )
      setSubmitting(false)
    }
  }

  return (
    <div className="min-h-screen bg-background text-foreground flex flex-col justify-center py-12 sm:px-6 lg:px-8">
      <div className="sm:mx-auto sm:w-full sm:max-w-md text-center">
        <div className="flex justify-center mb-3">
          <CuanMark decorative className="h-12 w-12 text-primary" />
        </div>
        <h1 className="text-2xl font-bold tracking-tight text-foreground">
          {t('Aktivasi Akun Tim Cuan')}
        </h1>
        <p className="mt-1 text-sm text-muted-foreground">
          {t('Lengkapi nama dan kata sandi Anda untuk mulai berkolaborasi.')}
        </p>
      </div>

      <div className="mt-6 sm:mx-auto sm:w-full sm:max-w-md px-4 sm:px-0">
        <div className="bg-card border border-border py-8 px-6 shadow-xl rounded-xl sm:px-10">
          {verifying ? (
            <div className="py-12 flex flex-col items-center justify-center gap-3 text-center">
              <Loader2 className="h-8 w-8 animate-spin text-primary" />
              <p className="text-sm text-muted-foreground">
                {t('Memverifikasi tautan undangan…')}
              </p>
            </div>
          ) : verifyError ? (
            <div className="space-y-4 py-4 text-center">
              <div className="mx-auto flex h-12 w-12 items-center justify-center rounded-full bg-destructive/10 text-destructive border border-destructive/20">
                <AlertCircle className="h-6 w-6" />
              </div>
              <div>
                <h3 className="text-base font-semibold text-foreground">
                  {t('Undangan Tidak Valid atau Kedaluwarsa')}
                </h3>
                <p className="mt-2 text-xs text-muted-foreground leading-relaxed">
                  {t(verifyError)}
                </p>
              </div>
              <div className="pt-4">
                <Link
                  to="/"
                  className="action-primary w-full inline-flex items-center justify-center min-h-11 text-xs"
                >
                  {t('Kembali ke Halaman Masuk')}
                </Link>
              </div>
            </div>
          ) : success ? (
            <div className="py-8 text-center space-y-3">
              <div className="mx-auto flex h-12 w-12 items-center justify-center rounded-full bg-green-500/10 text-green-600 dark:text-green-400 border border-green-500/20">
                <CheckCircle className="h-6 w-6" />
              </div>
              <h3 className="text-base font-semibold text-foreground">
                {t('Akun Berhasil Diaktifkan!')}
              </h3>
              <p className="text-xs text-muted-foreground">
                {t('Mengalihkan ke dashboard ruang kerja Cuan…')}
              </p>
            </div>
          ) : (
            <form onSubmit={handleSubmit} className="space-y-4">
              {formError && (
                <div role="alert" className="rounded-lg border border-destructive/30 bg-destructive/10 p-3 text-xs text-destructive">
                  {t(formError)}
                </div>
              )}

              {inviteData && (
                <div className="p-3.5 rounded-lg border border-border bg-muted/30 space-y-1.5">
                  <div className="text-[11px] font-medium text-muted-foreground uppercase tracking-wider">
                    {t('Undangan Untuk')}
                  </div>
                  <div className="flex items-center justify-between gap-2">
                    <span className="font-semibold text-sm text-foreground truncate">
                      {inviteData.email}
                    </span>
                    <Badge
                      variant={inviteData.role === 'owner' ? 'success' : 'info'}
                      className="shrink-0 font-semibold"
                    >
                      {inviteData.role === 'owner' ? <Shield className="h-3 w-3" /> : <User className="h-3 w-3" />}
                      <span className="capitalize">{inviteData.role === 'owner' ? t('Owner') : t('Member')}</span>
                    </Badge>
                  </div>
                </div>
              )}

              <div>
                <label htmlFor="accept-name" className="block text-xs font-medium text-foreground mb-1">
                  {t('Nama Lengkap')} <span className="text-destructive">*</span>
                </label>
                <input
                  id="accept-name"
                  type="text"
                  required
                  autoFocus
                  placeholder={t('Nama lengkap Anda')}
                  value={name}
                  onChange={(e) => setName(e.target.value)}
                  className="field-control w-full text-sm"
                  disabled={submitting}
                />
              </div>

              <div>
                <label htmlFor="accept-password" className="block text-xs font-medium text-foreground mb-1">
                  {t('Kata Sandi Baru')} <span className="text-destructive">*</span>
                </label>
                <input
                  id="accept-password"
                  type="password"
                  required
                  minLength={8}
                  placeholder={t('Minimal 8 karakter')}
                  value={password}
                  onChange={(e) => setPassword(e.target.value)}
                  className="field-control w-full text-sm"
                  disabled={submitting}
                />
              </div>

              <div>
                <label htmlFor="accept-confirm-password" className="block text-xs font-medium text-foreground mb-1">
                  {t('Konfirmasi Kata Sandi')} <span className="text-destructive">*</span>
                </label>
                <input
                  id="accept-confirm-password"
                  type="password"
                  required
                  minLength={8}
                  placeholder={t('Ulangi kata sandi baru')}
                  value={confirmPassword}
                  onChange={(e) => setConfirmPassword(e.target.value)}
                  className="field-control w-full text-sm"
                  disabled={submitting}
                />
              </div>

              <div className="pt-2">
                <button
                  type="submit"
                  disabled={submitting || !name.trim() || !password || !confirmPassword}
                  className="action-primary w-full min-h-11 flex items-center justify-center gap-2 text-xs font-semibold"
                >
                  {submitting ? (
                    <>
                      <Loader2 className="h-4 w-4 animate-spin" />
                      <span>{t('Mengaktifkan Akun…')}</span>
                    </>
                  ) : (
                    <>
                      <span>{t('Aktifkan Akun & Masuk')}</span>
                      <ArrowRight className="h-4 w-4" />
                    </>
                  )}
                </button>
              </div>

              <div className="pt-2 text-center text-xs text-muted-foreground">
                <p>
                  {t('Sudah punya akun sebelumnya?')}{' '}
                  <Link to="/" className="text-primary hover:underline font-medium">
                    {t('Masuk di sini')}
                  </Link>
                </p>
              </div>
            </form>
          )}
        </div>
      </div>
    </div>
  )
}
