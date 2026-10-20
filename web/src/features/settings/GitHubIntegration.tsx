import { useEffect, useState } from 'react'
import { Link } from 'react-router-dom'
import { apiFetch } from '../../api/client'
type Connection = { connected: boolean; account?: { login: string; name: string } }
export function GitHubIntegration() {
 const [connection, setConnection] = useState<Connection>(), [token, setToken] = useState('')
 const [busy, setBusy] = useState(false), [error, setError] = useState('')
 const load = () => { setError(''); return apiFetch<Connection>('/api/v1/integrations/github').then(setConnection).catch(() => setError('Could not load GitHub connection. Try again.')) }
 useEffect(() => { void load() }, [])
 return <section className="mx-auto w-full max-w-2xl space-y-5" aria-label="GitHub integration">
  <Link to="/settings/integrations" className="action-secondary">← Integrations</Link>
  <p className="text-sm text-muted-foreground">Connect GitHub to choose repositories for your projects. Projects can also work without a repository.</p>
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
  {connection && !connection.connected && <form className="space-y-3 rounded-lg border bg-card p-4" onSubmit={async event => {
   event.preventDefault(); setBusy(true); setError('')
   try { setConnection(await apiFetch<Connection>('/api/v1/integrations/github', { method: 'PUT', body: JSON.stringify({ token }) })); setToken('') }
   catch { setError('Could not connect. Check your token and try again.') } finally { setBusy(false) }
  }}>
   <label className="block text-sm font-medium" htmlFor="github-token">GitHub access token</label>
   <input id="github-token" type="password" autoComplete="off" className="field-control" required maxLength={4096} value={token} disabled={busy} onChange={e => setToken(e.target.value)} />
   <p className="text-xs leading-5 text-muted-foreground">Create a fine-grained token for the repositories you want Cuan to access. Use Contents read access for cloning, or read and write for pushing. Your token is encrypted and is never displayed again.</p>
   <a className="inline-flex min-h-11 items-center text-sm underline" href="https://github.com/settings/personal-access-tokens/new" target="_blank" rel="noopener noreferrer">Create token on GitHub ↗</a>
   <div><button className="action-primary" disabled={busy || !token.trim()}>{busy ? 'Connecting…' : 'Connect GitHub'}</button></div>
  </form>}
  {error && <div role="alert" className="space-y-2 text-sm text-destructive"><p>{error}</p>{!connection && <button className="action-secondary" onClick={() => void load()}>Try again</button>}</div>}
 </section>
}
