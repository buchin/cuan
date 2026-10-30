import { useState } from 'react'
import { useAuth } from '../../context/AuthContext'
import { useLanguage } from '../../context/LanguageContext'
import { apiFetch } from '../../api/client'
import { ChangePasswordModal } from '../../components/auth/ChangePasswordModal'
export function ProfileSettings() {
  const { user, refreshUser, isOwner } = useAuth(),
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
    <div className="max-w-2xl space-y-5">
      {/* Role Banner / Status Indicator */}
      <div
        data-testid="profile-role-card"
        className="flex items-center justify-between rounded-lg border border-border bg-card p-4 shadow-sm"
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
        <p role="status" className="text-sm">
          {notice}
        </p>
      )}
      <form
        className="space-y-4 rounded-lg border border-border bg-card p-5"
        onSubmit={(e) => {
          e.preventDefault()
          void save('name')
        }}
      >
        <h2 className="text-lg font-semibold">{t('Profile')}</h2>
        <label className="block text-sm font-medium" htmlFor="profile-name">
          {t('Display name')}
        </label>
        <input
          id="profile-name"
          className="field-control"
          autoComplete="name"
          maxLength={80}
          value={name}
          onChange={(e) => { setName(e.target.value); setNotice('') }}
          disabled={busy}
        />
        <p className="text-sm text-muted-foreground">
          {t('The name shown in your account menu. Your login stays the same.')}
        </p>
        {nameDirty && <p role="status" className="text-xs text-muted-foreground">{t(saving === 'name' ? 'Saving…' : 'Unsaved changes')}</p>}
        <button className="action-primary" disabled={busy || !nameDirty}>
          {t(saving === 'name' ? 'Saving…' : 'Save display name')}
        </button>
      </form>
      <form
        className="space-y-4 rounded-lg border border-border bg-card p-5"
        onSubmit={(e) => {
          e.preventDefault()
          void save('email')
        }}
      >
        <h2 className="text-lg font-semibold">{t('Email address')}</h2>
        <label className="block text-sm font-medium" htmlFor="profile-email">
          {t('Email')}
        </label>
        <input
          id="profile-email"
          type="email"
          className="field-control"
          autoComplete="email"
          value={email}
          required
          disabled={busy}
          onChange={(e) => { setEmail(e.target.value); setNotice('') }}
        />
        <p className="text-sm text-muted-foreground">
          {t('Your sign-in and system notifications will be sent to this email.')}
        </p>
        <label
          className="block text-sm font-medium"
          htmlFor="profile-email-current-password"
        >
          {t('Current password')}
        </label>
        <input
          id="profile-email-current-password"
          className="field-control"
          type="password"
          autoComplete="current-password"
          required
          value={emailPassword}
          disabled={busy}
          onChange={(e) => setEmailPassword(e.target.value)}
        />
        {emailDirty && (
          <p role="status" className="text-xs text-muted-foreground">
            {t(saving === 'email' ? 'Saving…' : 'Unsaved changes')}
          </p>
        )}
        <button
          className="action-primary"
          disabled={busy || !emailDirty}
        >
          {t(saving === 'email' ? 'Saving…' : 'Change email')}
        </button>
      </form>
      <form
        className="space-y-4 rounded-lg border border-border bg-card p-5"
        onSubmit={(e) => {
          e.preventDefault()
          void save('username')
        }}
      >
        <h2 className="text-lg font-semibold">{t('Sign-in details')}</h2>
        <label className="block text-sm font-medium" htmlFor="profile-username">
          {t('Username')}
        </label>
        <input
          id="profile-username"
          className="field-control"
          autoComplete="username"
          value={username}
          minLength={3}
          maxLength={32}
          pattern="[a-zA-Z0-9._\-]{3,32}"
          required
          disabled={busy}
          onChange={(e) => { setUsername(e.target.value); setNotice('') }}
        />
        <p className="text-sm text-muted-foreground">
          {t(
            'Use 3–32 letters, numbers, dots, underscores or hyphens. Your email also works for sign-in.',
          )}
        </p>
        <label
          className="block text-sm font-medium"
          htmlFor="profile-current-password"
        >
          {t('Current password')}
        </label>
        <input
          id="profile-current-password"
          className="field-control"
          type="password"
          autoComplete="current-password"
          required
          value={password}
          disabled={busy}
          onChange={(e) => setPassword(e.target.value)}
        />
        {usernameDirty && <p role="status" className="text-xs text-muted-foreground">{t(saving === 'username' ? 'Saving…' : 'Unsaved changes')}</p>}
        <button
          className="action-primary"
          disabled={busy || !usernameDirty}
        >
          {t(saving === 'username' ? 'Saving…' : 'Change username')}
        </button>
        <button
          type="button"
          className="action-secondary ml-2"
          onClick={() => setPasswordOpen(true)}
        >
          {t('Change password')}
        </button>
      </form>
      <ChangePasswordModal
        isOpen={passwordOpen}
        onClose={() => setPasswordOpen(false)}
      />
    </div>
  )
}
