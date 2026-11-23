import { useEffect, useState } from 'react'
import { useSearchParams } from 'react-router-dom'
import { ApiError, apiFetch } from '../../api/client'
import { FormField, Input } from '../../components/ui/forms'
type DeviceLogin = { id: string; state: 'preparing' | 'starting' | 'awaiting_authorization' | 'connected' | 'failed' | 'expired' | 'cancelled'; user_code?: string; expires_at: string; error?: string }
const pendingLogin = (job?: DeviceLogin) => !!job && ['preparing', 'starting', 'awaiting_authorization'].includes(job.state)
type Connection = { connected: boolean; account?: { login: string; name: string } }
export function GitHubIntegration() {
 const [params] = useSearchParams()
 const requestedReturn = params.get('return_to') || ''
 const returnTo = /^\/tasks\/[A-Za-z0-9_-]+(?:\/sessions\/[A-Za-z0-9_-]+)?$/.test(requestedReturn) ? requestedReturn : ''

 const [connection, setConnection] = useState<Connection>(), [token, setToken] = useState('')
 const [oauth, setOAuth] = useState<boolean | null>(null)
 const [oauthError, setOAuthError] = useState(false)
 const [device, setDevice] = useState<DeviceLogin>()
 const [copied, setCopied] = useState(false)
 const [busy, setBusy] = useState(false), [error, setError] = useState('')
 const load = () => { setError(''); return apiFetch<Connection>('/api/v1/integrations/github').then(setConnection).catch(() => setError('Could not load GitHub connection. Try again.')) }
 const checkOAuth = () => { setOAuthError(false); setOAuth(null); return apiFetch<{ configured: boolean; device_login?: DeviceLogin }>('/api/v1/integrations/github/oauth').then(result => { setOAuth(result.configured); if (result.device_login) setDevice(result.device_login) }).catch(() => setOAuthError(true)) }
 useEffect(() => { void load(); void checkOAuth() }, [])
 useEffect(() => {
  if (!device || !pendingLogin(device)) return
  let stopped = false
  let timer: ReturnType<typeof setTimeout>
  const controller = new AbortController()
  const poll = async () => {
   try {
    const result = await apiFetch<{ device_login: DeviceLogin }>('/api/v1/integrations/github/device/' + device.id, { signal: controller.signal })
    if (stopped) return
    setError(''); setDevice(result.device_login)
    if (result.device_login.state === 'connected') { await load(); return }
    if (!pendingLogin(result.device_login)) { setError(result.device_login.error || 'GitHub sign-in was cancelled. You can start again.'); return }
   } catch (cause) {
    if (stopped) return
    if (cause instanceof ApiError && [401, 404].includes(cause.status)) { setDevice({ ...device, state: 'expired' }); setError('This sign-in has expired. Start again.'); return }
    setError('Could not check GitHub sign-in. Retrying automatically…')
   }
   if (!stopped) timer = setTimeout(() => void poll(), 2000)
  }
  void poll()
  return () => { stopped = true; clearTimeout(timer); controller.abort() }
 }, [device?.id, pendingLogin(device)])
 return <section className="mx-auto w-full max-w-2xl space-y-5" aria-label="GitHub integration">
  <p className="text-sm text-muted-foreground">Connect repositories so Cuan can work on your project files. Projects can also work without GitHub.</p>
  {!connection && !error && <p role="status">Loading GitHub connection…</p>}
  {connection?.connected && <section className="space-y-3 rounded-lg border bg-card p-4">
   <p className="text-sm">Connected as <strong>{connection.account?.login}</strong></p>
   <button className="action-secondary" disabled={busy} onClick={async () => {
    if (!confirm('Disconnect GitHub? Future repository access may require reconnecting. This does not revoke the token on GitHub.')) return
    setBusy(true); setError('')
    try { setConnection(await apiFetch<Connection>('/api/v1/integrations/github', { method: 'DELETE' })) }
    catch { setError('Could not disconnect GitHub. Try again.') } finally { setBusy(false) }
   }}>Disconnect</button>
  </section>}
  {connection && !connection.connected && <section className="space-y-3 rounded-lg border bg-card p-4">
   <p className="text-sm">Sign in to GitHub to connect your repositories. You will review access on GitHub before returning to Cuan.</p>
   {!pendingLogin(device) && <button type="button" className="action-primary" disabled={busy || (oauth === null && !oauthError)} onClick={async () => {
    setError('')
    setBusy(true); setCopied(false)
    try {
     const result = await apiFetch<{ authorize_url?: string; device_login?: DeviceLogin }>('/api/v1/integrations/github/oauth', { method: 'POST', body: JSON.stringify({ return_to: returnTo }) })
     if (result.device_login) { setDevice(result.device_login); return }
     if (!result.authorize_url) throw new Error('Missing sign-in')
     const url = new URL(result.authorize_url)
     if (url.origin !== 'https://github.com' || url.pathname !== '/login/oauth/authorize') throw new Error('Invalid authorization URL')
     window.location.assign(url.href)
    } catch { setError('Could not start GitHub sign-in. Try again.') }
    finally { setBusy(false) }
   }}>{busy ? 'Connecting…' : 'Sign in with GitHub'}</button>}
   {pendingLogin(device) && <div className="space-y-3" aria-live="polite">
    {device?.user_code ? <>
     <p className="text-sm text-muted-foreground">Copy this one-time code and enter it on GitHub. This page will update when you finish.</p>
     <button type="button" className="action-secondary font-mono text-lg tracking-widest" aria-label="Copy GitHub verification code" onClick={async () => { try { await navigator.clipboard.writeText(device.user_code!); setCopied(true) } catch { setError('Copy the code shown here and enter it on GitHub.') } }}>{device.user_code}</button>
     {copied && <p role="status" className="text-sm text-muted-foreground">Code copied.</p>}
     <div><a className="action-primary" href="https://github.com/login/device" target="_blank" rel="noopener noreferrer">Continue on GitHub ↗</a></div>
    </> : <p role="status" className="text-sm text-muted-foreground">Preparing GitHub sign-in…</p>}
    <button type="button" className="action-secondary" disabled={busy} onClick={async () => {
     setBusy(true)
     try { const result = await apiFetch<{ device_login: DeviceLogin }>('/api/v1/integrations/github/device/' + device!.id + '/cancel', { method: 'POST' }); setDevice(result.device_login); if (result.device_login.state === 'connected') await load(); else setError('') }
     catch { setError('Could not cancel sign-in. Try again.') } finally { setBusy(false) }
    }}>Cancel</button>
   </div>}
   {oauth === null && !oauthError && <p role="status" className="text-sm text-muted-foreground">Checking GitHub sign-in…</p>}
   {oauthError && <p role="alert" className="text-sm text-muted-foreground">Could not check sign-in availability. You can still try signing in.</p>}
  </section>}
  {params.get('github_status') && params.get('github_status') !== 'connected' && <p role="status" className="text-sm text-muted-foreground">GitHub connection was not completed. Your work is saved. Try connecting again.</p>}
  {connection && !connection.connected && <details className="border-t pt-2"><summary className="flex min-h-11 cursor-pointer items-center text-sm text-muted-foreground">Advanced</summary>
   <form className="space-y-4 py-3" onSubmit={async event => {
   event.preventDefault(); setBusy(true); setError('')
   try { setConnection(await apiFetch<Connection>('/api/v1/integrations/github', { method: 'PUT', body: JSON.stringify({ token }) })); setToken('') }
   catch { setError('Could not connect. Check your token and try again.') } finally { setBusy(false) }
  }}>
   <FormField
     label="GitHub access token"
     htmlFor="github-token"
     required
     helperText="Create a fine-grained token for the repositories you want Cuan to access. Use Contents read access for cloning, or read and write for pushing. Your token is encrypted and is never displayed again."
   >
     <Input
       id="github-token"
       type="password"
       autoComplete="off"
       required
       maxLength={4096}
       value={token}
       disabled={busy}
       onChange={e => setToken(e.target.value)}
       mono
     />
   </FormField>
   <a className="inline-flex min-h-11 items-center text-sm underline" href="https://github.com/settings/personal-access-tokens/new" target="_blank" rel="noopener noreferrer">Create token on GitHub ↗</a>
   <div><button className="action-primary" disabled={busy || !token.trim()}>{busy ? 'Connecting…' : 'Connect with token'}</button></div>
  </form></details>}
  {error && <div role="alert" className="space-y-2 text-sm text-destructive"><p>{error}</p>{!connection && <button className="action-secondary" onClick={() => void load()}>Try again</button>}</div>}
 </section>
}
