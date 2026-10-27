import { useEffect, useState } from 'react'
import { Link } from 'react-router-dom'
import { Github, MessageCircle, ArrowRight } from 'lucide-react'
import { apiFetch } from '../../api/client'
type Connection = { connected: boolean; account?: { login: string } }
export function IntegrationOverview() {
 const [github, setGitHub] = useState<Connection>(), [available, setAvailable] = useState<boolean>()
 const [failed, setFailed] = useState(false), [revision, setRevision] = useState(0)
 useEffect(() => {
  let alive = true; setFailed(false); setGitHub(undefined); setAvailable(undefined)
  void Promise.all([
   apiFetch<Connection>('/api/v1/integrations/github'),
   apiFetch<{ configured: boolean }>('/api/v1/integrations/github/oauth'),
  ]).then(([connection, oauth]) => { if (alive) { setGitHub(connection); setAvailable(oauth.configured) } }).catch(() => { if (alive) setFailed(true) })
  return () => { alive = false }
 }, [revision])
 const status = failed ? 'Status unavailable' : !github ? 'Checking connection…' : github.connected ? 'Connected' : available ? 'Not connected' : 'Setup needed'
 const action = failed || !github ? 'View details' : github?.connected ? 'Manage' : available ? 'Connect GitHub' : 'Set up GitHub'
 return <div className="grid min-w-0 max-w-4xl gap-4 md:grid-cols-2" aria-label="Available integrations">
  <section className="flex min-w-0 flex-col gap-4 rounded-lg border bg-card p-4 sm:p-5" aria-label="GitHub integration card">
   <header className="flex items-center gap-3"><span className="flex h-9 w-9 shrink-0 items-center justify-center rounded-lg bg-muted"><Github className="h-5 w-5" aria-hidden /></span><h2 className="text-sm font-semibold">GitHub</h2></header>
   <p className="text-sm leading-6 text-muted-foreground">Connect repositories so Cuan can work on your project files and save changes.</p>
   <div className="min-h-10 text-xs text-muted-foreground" role="status"><span className={github?.connected ? 'font-medium text-foreground' : ''}>{status}</span>{github?.connected && github.account?.login && <span className="mt-1 block truncate">{github.account.login}</span>}</div>
   <div className="mt-auto flex flex-wrap items-center gap-2"><Link className={github?.connected ? 'action-secondary' : 'action-primary'} to="/settings/integrations/github">{action}<ArrowRight className="ml-1.5 h-3.5 w-3.5" aria-hidden /></Link>{failed && <button type="button" className="action-secondary" onClick={() => setRevision(value => value + 1)}>Try again</button>}</div>
  </section>
  <section className="flex min-w-0 flex-col gap-4 rounded-lg border bg-card p-4 sm:p-5" aria-label="Live Chat integration card">
   <header className="flex items-center gap-3"><span className="flex h-9 w-9 shrink-0 items-center justify-center rounded-lg bg-muted"><MessageCircle className="h-5 w-5" aria-hidden /></span><h2 className="text-sm font-semibold">Live Chat</h2></header>
   <p className="text-sm leading-6 text-muted-foreground">Chat with visitors on your website. Their conversations and contact details appear in Cuan.</p>
   <p className="min-h-10 text-xs text-muted-foreground">Website widget · Installation guide</p>
   <div className="mt-auto"><Link className="action-secondary" to="/settings/integrations/live-chat">Set up Live Chat<ArrowRight className="ml-1.5 h-3.5 w-3.5" aria-hidden /></Link></div>
  </section>
 </div>
}
