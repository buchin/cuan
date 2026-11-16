import { Link } from 'react-router-dom'
import { CuanMark } from '../brand/CuanMark'
import { useState, type FormEvent } from 'react'
import { Button } from '../ui/button'
import { Input } from '../ui/input'
import { useAuth } from '../../context/AuthContext'
import { useLanguage } from '../../context/LanguageContext'
import { Eye, EyeOff, LoaderCircle } from 'lucide-react'

export function LoginScreen() {
  const { login, setup, setupRequired, setupError, refreshUser } = useAuth()
  const { t } = useLanguage()
  const [identifier, setIdentifier] = useState('')
  const [password, setPassword] = useState('')
  const [showPassword, setShowPassword] = useState(false)
  const [error, setError] = useState('')
  const [submitting, setSubmitting] = useState(false)
  const creating = setupRequired === true
  const submit = async (event: FormEvent) => {
    event.preventDefault()
    if (submitting || setupRequired === null) return
    setError('')
    setSubmitting(true)
    try {
      if (creating) await setup(identifier, password)
      else await login(identifier, password)
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Could not continue. Try again.')
    } finally {
      setSubmitting(false)
    }
  }
  return (
    <div className="flex min-h-[100dvh] flex-col bg-background bg-[#fafafa] dark:bg-[#121214] p-4 text-foreground sm:p-6">
      <header className="mx-auto w-full max-w-5xl">
        <Link to="/" className="inline-flex min-h-11 items-center gap-2.5" aria-label={t('Cuan home')}>
          <CuanMark className="h-8 w-8 text-foreground" />
          <span className="font-display text-lg font-bold tracking-tight text-foreground">
            cuan
          </span>
        </Link>
      </header>
      <main className="mx-auto my-auto w-full max-w-sm py-8">
        <h1 className="text-2xl font-semibold tracking-tight">{t(creating ? 'Set up your workspace' : 'Sign in to Cuan')}</h1>
        <p className="mt-2 text-sm leading-relaxed text-muted-foreground">{t(creating ? 'Create your owner account to get started.' : 'Welcome back. Your workspace is ready for you.')}</p>
        {setupRequired === null ? (
          <div role="alert" className="mt-6 space-y-3 text-sm">
            <p>{t('Could not check workspace setup. Check your connection and try again.')}</p>
            {setupError && <p className="text-xs text-muted-foreground">{setupError}</p>}
            <Button onClick={() => void refreshUser()}>{t('Try again')}</Button>
          </div>
        ) : (
          <form onSubmit={submit} className="mt-6 space-y-4">
            {error && <p role="alert" className="rounded-lg border border-destructive/30 p-3 text-sm text-destructive">{t(error)}</p>}
            <div className="space-y-2">
              <label htmlFor="owner-identifier" className="block text-sm font-medium">{t(creating ? 'Email' : 'Username or email')}</label>
              <Input id="owner-identifier" type={creating ? 'email' : 'text'} autoComplete={creating ? 'email' : 'username'} autoCapitalize="none" spellCheck={false} placeholder={creating ? 'you@example.com' : 'Username or email'} value={identifier} onChange={e => setIdentifier(e.target.value)} disabled={submitting} required autoFocus />
            </div>
            <div className="space-y-2">
              <label htmlFor="owner-password" className="block text-sm font-medium">{t('Password')}</label>
              <div className="relative">
                <Input id="owner-password" type={showPassword ? 'text' : 'password'} autoComplete={creating ? 'new-password' : 'current-password'} minLength={creating ? 8 : undefined} aria-describedby={creating ? 'owner-password-help' : undefined} value={password} onChange={e => setPassword(e.target.value)} disabled={submitting} required className="pr-12" />
                <button type="button" aria-label={t(showPassword ? 'Hide password' : 'Show password')} aria-pressed={showPassword} onClick={() => setShowPassword(!showPassword)} className="absolute inset-y-0 right-0 flex min-w-11 items-center justify-center text-muted-foreground hover:text-foreground">
                  {showPassword ? <EyeOff className="h-4 w-4" /> : <Eye className="h-4 w-4" />}
                </button>
              </div>
              {creating && <p id="owner-password-help" className="text-xs text-muted-foreground">{t('Use at least 8 characters.')}</p>}
            </div>
            <Button
              type="submit"
              disabled={submitting}
              className="w-full bg-foreground text-background hover:bg-foreground/90 font-medium tracking-tight shadow-sm transition-colors"
            >
              {submitting && <LoaderCircle className="mr-2 h-4 w-4 animate-spin text-background" aria-hidden="true" />}
              {t(submitting ? (creating ? 'Creating account…' : 'Signing in…') : (creating ? 'Create account' : 'Sign in'))}
            </Button>
            {creating && <p className="text-xs leading-relaxed text-muted-foreground">{t('Next, connect your AI and tell Cuan what you want to get done.')}</p>}
          </form>
        )}
      </main>
    </div>
  )
}
