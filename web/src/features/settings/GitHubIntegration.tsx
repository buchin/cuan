import { useEffect, useState } from 'react'
import { Link, useSearchParams } from 'react-router-dom'
import { apiFetch } from '../../api/client'
type Connection = { connected: boolean; account?: { login: string; name: string } }
export function GitHubIntegration() {
 const [params] = useSearchParams()
 const requestedReturn = params.get('return_to') || ''
 const returnTo = /^\/tasks\/[A-Za-z0-9_-]+(?:\/sessions\/[A-Za-z0-9_-]+)?$/.test(requestedReturn) ? requestedReturn : ''

 const [connection, setConnection] = useState<Connection>(), [token, setToken] = useState('')
 const [oauth, setOAuth] = useState<boolean | null>(null)
 const [oauthError, setOAuthError] = useState(false)
 const [busy, setBusy] = useState(false), [error, setError] = useState('')
 const load = () => { setError(''); return apiFetch<Connection>('/api/v1/integrations/github').then(setConnection).catch(() => setError('Could not load GitHub connection. Try again.')) }
 const checkOAuth = () => { setOAuthError(false); setOAuth(null); return apiFetch<{ configured: boolean }>('/api/v1/integrations/github/oauth').then(result => setOAuth(result.configured)).catch(() => setOAuthError(true)) }
 useEffect(() => { void load(); void checkOAuth() }, [])
 return <section className="mx-auto w-full max-w-2xl space-y-5" aria-label="GitHub integration">
  <Link to="/settings/integrations" className="action-secondary">← Integrations</Link>
  <p className="text-sm text-muted-foreground">Connect GitHub to choose repositories for your projects. Projects can also work without a repository.</p>
  {!connection && !error && <p role="status">Loading GitHub connection…</p>}
  {connection?.connected && <section className="space-y-3 rounded-lg border bg-card p-4">
   <p className="text-sm">Connected as <strong>{connection.account?.login}</strong></p>
   {returnTo && <Link className="action-primary" to={returnTo}>Back to task</Link>}
   <button className="action-secondary" disabled={busy} onClick={async () => {
    if (!confirm('Disconnect GitHub? Future repository access may require reconnecting. This does not revoke the token on GitHub.')) return
    setBusy(true); setError('')
    try { setConnection(await apiFetch<Connection>('/api/v1/integrations/github', { method: 'DELETE' })) }
    catch { setError('Could not disconnect GitHub. Try again.') } finally { setBusy(false) }
   }}>Disconnect</button>
  </section>}
  {connection && !connection.connected && <section className="space-y-3 rounded-lg border bg-card p-4">
   <p className="text-sm">Authorize Cuan on GitHub. Review access there, then return to your work.</p>
   <button type="button" className="action-primary" disabled={busy || oauth !== true} onClick={async () => {
    setBusy(true); setError('')
    try { const result = await apiFetch<{ authorize_url: string }>('/api/v1/integrations/github/oauth', { method: 'POST', body: JSON.stringify({ return_to: returnTo }) }); const url = new URL(result.authorize_url); if (url.origin !== 'https://github.com' || url.pathname !== '/login/oauth/authorize') throw new Error('Invalid authorization URL'); window.location.assign(url.href) }
    catch { setError('Could not start GitHub connection. Try again.'); setBusy(false) }
   }}>{busy ? 'Connecting…' : 'Connect GitHub'}</button>
   {oauth === false && <p className="text-sm text-muted-foreground">GitHub sign-in needs to be configured by this installation’s administrator.</p>}
   {oauth === null && !oauthError && <p role="status" className="text-sm text-muted-foreground">Checking GitHub sign-in availability…</p>}
   {oauthError && <div role="alert" className="space-y-2 text-sm"><p>Could not check GitHub sign-in availability.</p><button type="button" className="action-secondary" onClick={() => void checkOAuth()}>Check again</button></div>}
  </section>}
  {params.get('github_status') && params.get('github_status') !== 'connected' && <p role="status" className="text-sm text-muted-foreground">GitHub connection was not completed. Your work is saved. Try connecting again.</p>}
  {connection && !connection.connected && <details className="rounded-lg border bg-card p-4"><summary className="flex min-h-11 cursor-pointer items-center text-sm text-muted-foreground">Advanced</summary><form className="space-y-3 rounded-lg border bg-card p-4" onSubmit={async event => {
   event.preventDefault(); setBusy(true); setError('')
   try { setConnection(await apiFetch<Connection>('/api/v1/integrations/github', { method: 'PUT', body: JSON.stringify({ token }) })); setToken('') }
   catch { setError('Could not connect. Check your token and try again.') } finally { setBusy(false) }
  }}>
   <label className="block text-sm font-medium" htmlFor="github-token">GitHub access token</label>
   <input id="github-token" type="password" autoComplete="off" className="field-control" required maxLength={4096} value={token} disabled={busy} onChange={e => setToken(e.target.value)} />
   <p className="text-xs leading-5 text-muted-foreground">Create a fine-grained token for the repositories you want Cuan to access. Use Contents read access for cloning, or read and write for pushing. Your token is encrypted and is never displayed again.</p>
   <a className="inline-flex min-h-11 items-center text-sm underline" href="https://github.com/settings/personal-access-tokens/new" target="_blank" rel="noopener noreferrer">Create token on GitHub ↗</a>
   <div><button className="action-primary" disabled={busy || !token.trim()}>{busy ? 'Connecting…' : 'Connect with token'}</button></div>
  </form></details>}
  {error && <div role="alert" className="space-y-2 text-sm text-destructive"><p>{error}</p>{!connection && <button className="action-secondary" onClick={() => void load()}>Try again</button>}</div>}
 </section>
}
