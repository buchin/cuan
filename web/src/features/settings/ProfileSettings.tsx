import { useState } from 'react'
import { useAuth } from '../../context/AuthContext'
import { useLanguage } from '../../context/LanguageContext'
import { apiFetch } from '../../api/client'
import { ChangePasswordModal } from '../../components/auth/ChangePasswordModal'
import { GroupedSection, GroupedRow } from '../../components/layout/GroupedSection'
import { User, KeyRound, Mail, Shield, LogOut } from 'lucide-react'

export function ProfileSettings() {
  const { user, refreshUser, isOwner, logout } = useAuth(),
    { t } = useLanguage()
  const [name, setName] = useState(user?.display_name || ''),
    [username, setUsername] = useState(user?.username || ''),
    [email, setEmail] = useState(user?.email || ''),
    [emailPassword, setEmailPassword] = useState(''),
    [password, setPassword] = useState(''),
    [busy, setBusy] = useState(false),
    [error, setError] = useState(''),
    [notice, setNotice] = useState(''),
    [passwordOpen, setPasswordOpen] = useState(false),
    [savedName, setSavedName] = useState(user?.display_name || ''),
    [savedEmail, setSavedEmail] = useState(user?.email || ''),
    [saving, setSaving] = useState<'name' | 'username' | 'email' | null>(null)
  const nameDirty = name.trim() !== savedName.trim()
  const usernameDirty = username.trim().toLowerCase() !== (user?.username || '').toLowerCase()
  const emailDirty = email.trim().toLowerCase() !== savedEmail.trim().toLowerCase()

  async function save(kind: 'name' | 'username' | 'email') {
    if (busy || (kind === 'name' ? !nameDirty : kind === 'username' ? !usernameDirty : !emailDirty)) return
    setSaving(kind)
    setBusy(true)
    setError('')
    setNotice('')
    try {
      if (kind === 'name')
        await apiFetch('/api/v1/profile', {
          method: 'PATCH',
          body: JSON.stringify({ display_name: name }),
        })
      else if (kind === 'username')
        await apiFetch('/api/v1/auth/username', {
          method: 'PATCH',
          body: JSON.stringify({ username, current_password: password }),
        })
      else
        await apiFetch('/api/v1/auth/email', {
          method: 'PATCH',
          body: JSON.stringify({ email, current_password: emailPassword }),
        })
      if (kind === 'name') setSavedName(name.trim())
      if (kind === 'email') {
        setSavedEmail(email.trim().toLowerCase())
        setEmailPassword('')
      }
      if (kind === 'username') setPassword('')
      await refreshUser()
      setNotice(
        t(
          kind === 'name'
            ? 'Display name saved.'
            : kind === 'username'
            ? 'Username saved. Use it the next time you sign in.'
            : 'Email address saved. Use it the next time you sign in.',
        ),
      )
    } catch (e) {
      setError(
        t(
          e instanceof Error
            ? e.message
            : 'Could not save your profile. Try again.',
        ),
      )
    } finally {
      setBusy(false)
      setSaving(null)
    }
  }

  return (
    <div className="max-w-2xl space-y-6">
      {/* Role Banner / Status Indicator */}
      <div
        data-testid="profile-role-card"
        className="flex items-center justify-between rounded-xl border border-border bg-card p-4 shadow-sm"
      >
        <div className="space-y-0.5">
          <div className="text-[11px] uppercase tracking-wider font-semibold text-muted-foreground">
            {t('Peran Akun')}
          </div>
          <div className="text-sm font-semibold text-foreground flex items-center gap-2">
            {!isOwner ? t('Member (Staf Tim)') : t('Owner (Pemilik Workspace)')}
          </div>
          <p className="text-xs text-muted-foreground">
            {!isOwner
              ? t('Akses operasional: mengelola task, berkolaborasi dengan asisten AI, dan menangani pesan Inbox.')
              : t('Akses penuh: mengelola tim staf, setelan model AI, koneksi aplikasi, dan operasional bisnis.')}
          </p>
        </div>
        <span
          className={`px-2.5 py-1 text-xs font-semibold rounded-full uppercase tracking-wider border shrink-0 ${
            !isOwner
              ? 'bg-blue-500/15 text-blue-600 dark:text-blue-400 border-blue-500/30'
              : 'bg-green-500/15 text-green-600 dark:text-green-400 border-green-500/30'
          }`}
        >
          {!isOwner ? t('Member') : t('Owner')}
        </span>
      </div>

      {error && (
        <p role="alert" className="text-sm text-destructive">
          {error}
        </p>
      )}
      {notice && (
        <p role="status" className="text-sm text-emerald-600 dark:text-emerald-400 font-medium">
          {notice}
        </p>
      )}

      {/* 1. Profile & Display Name */}
      <form
        onSubmit={(e) => {
          e.preventDefault()
          void save('name')
        }}
      >
        <GroupedSection
          title={t('Profile')}
          description={t('The name shown in your account menu. Your login stays the same.')}
        >
          <GroupedRow
            label={t('Display name')}
            icon={<User className="h-4 w-4 text-primary" />}
            control={
              <div className="flex flex-col sm:flex-row items-stretch sm:items-center gap-2 w-full max-w-sm">
                <input
                  id="profile-name"
                  className="field-control text-xs flex-1"
                  autoComplete="name"
                  maxLength={80}
                  value={name}
                  onChange={(e) => { setName(e.target.value); setNotice('') }}
                  disabled={busy}
                />
                <button
                  type="submit"
                  className="action-primary text-xs shrink-0"
                  disabled={busy || !nameDirty}
                >
                  {t(saving === 'name' ? 'Saving…' : 'Save display name')}
                </button>
              </div>
            }
          />
        </GroupedSection>
      </form>

      {/* 2. Sign-in Details (Username & Password) */}
      <form
        onSubmit={(e) => {
          e.preventDefault()
          void save('username')
        }}
      >
        <GroupedSection
          title={t('Sign-in details')}
          description={t('Use 3–32 letters, numbers, dots, underscores or hyphens. Your email also works for sign-in.')}
        >
          <GroupedRow
            label={t('Username')}
            icon={<Shield className="h-4 w-4 text-primary" />}
            control={
              <div className="w-full max-w-sm">
                <input
                  id="profile-username"
                  className="field-control text-xs w-full"
                  autoComplete="username"
                  value={username}
                  minLength={3}
                  maxLength={32}
                  pattern="[a-zA-Z0-9._\-]{3,32}"
                  required
                  disabled={busy}
                  onChange={(e) => { setUsername(e.target.value); setNotice('') }}
                />
              </div>
            }
          />
          {usernameDirty && (
            <GroupedRow
              label={t('Current password')}
              description={t('Required to authorize username changes.')}
              control={
                <div className="flex flex-col sm:flex-row items-stretch sm:items-center gap-2 w-full max-w-sm">
                  <input
                    id="profile-current-password"
                    className="field-control text-xs flex-1"
                    type="password"
                    autoComplete="current-password"
                    required
                    value={password}
                    disabled={busy}
                    onChange={(e) => setPassword(e.target.value)}
                  />
                  <button
                    type="submit"
                    className="action-primary text-xs shrink-0"
                    disabled={busy || !usernameDirty}
                  >
                    {t(saving === 'username' ? 'Saving…' : 'Change username')}
                  </button>
                </div>
              }
            />
          )}
          <GroupedRow
            label={t('Password')}
            description={t('Update your secret master sign-in password.')}
            icon={<KeyRound className="h-4 w-4 text-muted-foreground" />}
            control={
              <button
                type="button"
                className="action-secondary text-xs"
                onClick={() => setPasswordOpen(true)}
              >
                {t('Change password')}
              </button>
            }
          />
        </GroupedSection>
      </form>

      {/* 3. Email Address Form */}
      <form
        onSubmit={(e) => {
          e.preventDefault()
          void save('email')
        }}
      >
        <GroupedSection
          title={t('Email address')}
          description={t('Your sign-in and system notifications will be sent to this email.')}
        >
          <GroupedRow
            label={t('Email')}
            icon={<Mail className="h-4 w-4 text-primary" />}
            control={
              <div className="w-full max-w-sm">
                <input
                  id="profile-email"
                  type="email"
                  className="field-control text-xs w-full"
                  autoComplete="email"
                  value={email}
                  required
                  disabled={busy}
                  onChange={(e) => { setEmail(e.target.value); setNotice('') }}
                />
              </div>
            }
          />
          <GroupedRow
            label={t('Current password')}
            description={t('Required to authorize email address changes.')}
            control={
              <div className="flex flex-col sm:flex-row items-stretch sm:items-center gap-2 w-full max-w-sm">
                <input
                  id="profile-email-current-password"
                  className="field-control text-xs flex-1"
                  type="password"
                  autoComplete="current-password"
                  required={emailDirty}
                  value={emailPassword}
                  disabled={busy}
                  onChange={(e) => setEmailPassword(e.target.value)}
                />
                <button
                  type="submit"
                  className="action-primary text-xs shrink-0"
                  disabled={busy || !emailDirty}
                >
                  {t(saving === 'email' ? 'Saving…' : 'Change email')}
                </button>
              </div>
            }
          />
        </GroupedSection>
      </form>

      {/* 4. Session & Sign Out */}
      <GroupedSection
        title={t('Sesi & Akun')}
        description={t('Keluar dari sesi akun Anda di perangkat ini.')}
      >
        <GroupedRow
          label={t('Sign out')}
          description={t('Akhiri sesi aktif Anda di browser ini.')}
          icon={<LogOut className="h-4 w-4 text-destructive" />}
          control={
            <button
              type="button"
              onClick={() => logout()}
              className="action-secondary text-xs text-destructive hover:bg-destructive/10 inline-flex items-center gap-1.5"
            >
              <LogOut className="h-3.5 w-3.5" />
              <span>{t('Sign out')}</span>
            </button>
          }
        />
      </GroupedSection>

      <ChangePasswordModal
        isOpen={passwordOpen}
        onClose={() => setPasswordOpen(false)}
      />
    </div>
  )
}
