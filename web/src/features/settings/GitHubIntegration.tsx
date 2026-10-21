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
 const [setupOpen, setSetupOpen] = useState(false)
 const [busy, setBusy] = useState(false), [error, setError] = useState('')
 const load = () => { setError(''); return apiFetch<Connection>('/api/v1/integrations/github').then(setConnection).catch(() => setError('Could not load GitHub connection. Try again.')) }
 const checkOAuth = () => { setOAuthError(false); setOAuth(null); return apiFetch<{ configured: boolean }>('/api/v1/integrations/github/oauth').then(result => setOAuth(result.configured)).catch(() => setOAuthError(true)) }
 useEffect(() => { void load(); void checkOAuth() }, [])
 return <section className="mx-auto w-full max-w-2xl space-y-5" aria-label="GitHub integration">
  <Link to="/settings/integrations" className="action-secondary">← Integrations</Link>
  <p className="text-sm text-muted-foreground">Connect repositories so Cuan can work on your project files. Projects can also work without GitHub.</p>
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
   <p className="text-sm">Sign in to GitHub to connect your repositories. You will review access on GitHub before returning to Cuan.</p>
   <button type="button" className="action-primary" disabled={busy || (oauth === null && !oauthError)} onClick={async () => {
    setError('')
    if (oauth !== true) {
     if (oauthError) { void checkOAuth(); return }
     setError('GitHub sign-in is not ready on this installation yet. The installation owner needs to enable it. Your work is saved.')
     return
    }
    setBusy(true)
    try { const result = await apiFetch<{ authorize_url: string }>('/api/v1/integrations/github/oauth', { method: 'POST', body: JSON.stringify({ return_to: returnTo }) }); const url = new URL(result.authorize_url); if (url.origin !== 'https://github.com' || url.pathname !== '/login/oauth/authorize') throw new Error('Invalid authorization URL'); window.location.assign(url.href) }
    catch { setError('Could not start GitHub sign-in. Try again.'); setBusy(false) }
   }}>{busy ? 'Connecting…' : 'Sign in with GitHub'}</button>
   {oauth === null && !oauthError && <p role="status" className="text-sm text-muted-foreground">Checking GitHub sign-in…</p>}
   {oauthError && <p role="alert" className="text-sm text-muted-foreground">Could not check GitHub sign-in. Select Sign in with GitHub to check again.</p>}
  </section>}
  {params.get('github_status') && params.get('github_status') !== 'connected' && <p role="status" className="text-sm text-muted-foreground">GitHub connection was not completed. Your work is saved. Try connecting again.</p>}
  {connection && !connection.connected && <details className="border-t pt-2"><summary className="flex min-h-11 cursor-pointer items-center text-sm text-muted-foreground">Advanced</summary>
   {oauth === false && <div className="space-y-3 py-3 text-sm text-muted-foreground">
    <p>For installation owners: enable GitHub sign-in once so users can connect without copying tokens.</p>
    <button type="button" className="action-secondary" aria-expanded={setupOpen} aria-controls="github-signin-setup" onClick={() => setSetupOpen(open => !open)}>Installation setup <span aria-hidden="true">{setupOpen ? '▴' : '▾'}</span></button>
    {setupOpen && <div id="github-signin-setup" className="space-y-3 rounded-md border p-3">
     <ol className="list-decimal space-y-2 pl-5">
      <li><a href="https://github.com/settings/applications/new" target="_blank" rel="noopener noreferrer" className="underline">Register a GitHub OAuth App ↗</a> for this installation.</li>
      <li>Homepage: <code className="break-all">{window.location.origin}</code>. Callback: <code className="break-all">{window.location.origin}/api/v1/integrations/github/oauth/callback</code>.</li>
      <li>Set <code>CUAN_GITHUB_CLIENT_ID</code>, <code>CUAN_GITHUB_CLIENT_SECRET</code>, and <code>CUAN_GITHUB_REDIRECT_URL</code> in the Cuan service environment, then restart the service. Keep the client secret private.</li>
     </ol>
     <button type="button" className="action-secondary" onClick={() => void checkOAuth()}>Check setup again</button>
    </div>}
   </div>}
   <form className="space-y-3 py-3" onSubmit={async event => {
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
