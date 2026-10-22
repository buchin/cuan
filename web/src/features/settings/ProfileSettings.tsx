import { useState } from 'react'
import { useAuth } from '../../context/AuthContext'
import { useLanguage } from '../../context/LanguageContext'
import { apiFetch } from '../../api/client'
import { ChangePasswordModal } from '../../components/auth/ChangePasswordModal'
export function ProfileSettings() {
  const { user, refreshUser } = useAuth(),
    { t } = useLanguage()
  const [name, setName] = useState(user?.display_name || ''),
    [username, setUsername] = useState(user?.username || ''),
    [password, setPassword] = useState(''),
    [busy, setBusy] = useState(false),
    [error, setError] = useState(''),
    [notice, setNotice] = useState(''),
    [passwordOpen, setPasswordOpen] = useState(false),
    [savedName, setSavedName] = useState(user?.display_name || ''),
    [saving, setSaving] = useState<'name' | 'username' | null>(null)
  const nameDirty = name.trim() !== savedName.trim()
  const usernameDirty = username.trim().toLowerCase() !== (user?.username || '').toLowerCase()
  async function save(kind: 'name' | 'username') {
    if (busy || (kind === 'name' ? !nameDirty : !usernameDirty)) return
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
      else
        await apiFetch('/api/v1/auth/username', {
          method: 'PATCH',
          body: JSON.stringify({ username, current_password: password }),
        })
      if (kind === 'name') setSavedName(name.trim())
      setPassword('')
      await refreshUser()
      setNotice(
        t(
          kind === 'name'
            ? 'Display name saved.'
            : 'Username saved. Use it the next time you sign in.',
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
