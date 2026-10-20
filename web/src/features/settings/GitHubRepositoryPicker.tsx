import { useState } from 'react'
import { Link } from 'react-router-dom'
import { apiFetch } from '../../api/client'
type Repository = { full_name: string; default_branch: string; permissions: { push: boolean } }
export function GitHubRepositoryPicker({ disabled, onSelect }: { disabled: boolean; onSelect: (url: string) => void }) {
 const [repos, setRepos] = useState<Repository[]>([]), [page, setPage] = useState(0)
 const [more, setMore] = useState(false), [busy, setBusy] = useState(false), [error, setError] = useState('')
 async function load(next: number) {
  setBusy(true); setError('')
  try {
   const result = await apiFetch<{ repositories: Repository[]; has_more: boolean }>('/api/v1/integrations/github/repositories?page=' + next)
   setRepos(old => next === 1 ? result.repositories : [...old, ...result.repositories.filter(repo => !old.some(item => item.full_name === repo.full_name))]); setPage(next); setMore(result.has_more)
  } catch { setError('Could not load repositories. Connect GitHub or try again.') } finally { setBusy(false) }
 }
 return <div className="space-y-2">
  {!page && <button type="button" className="action-secondary" disabled={disabled || busy} onClick={() => void load(1)}>{busy ? 'Loading…' : 'Choose from GitHub'}</button>}
  {page > 0 && <>
   <label htmlFor="github-repository-picker" className="block text-xs text-muted-foreground">Accessible repositories</label>
   <select id="github-repository-picker" className="field-control" value="" disabled={disabled || busy} onChange={event => { if (event.target.value) onSelect('https://github.com/' + event.target.value) }}>
    <option value="">Choose repository…</option>
    {repos.map(repo => <option key={repo.full_name} value={repo.full_name}>{repo.full_name}{repo.permissions?.push ? '' : ' · read only'}</option>)}
   </select>
   {!repos.length && <p className="text-xs text-muted-foreground">No repositories found. Check repository access in GitHub.</p>}
   {more && <button type="button" className="action-secondary" disabled={disabled || busy} onClick={() => void load(page + 1)}>{busy ? 'Loading…' : 'Load more'}</button>}
  </>}
  {error && <p role="alert" className="text-xs text-destructive">{error} <button type="button" disabled={disabled || busy} className="underline" onClick={() => void load(page ? page + 1 : 1)}>Try again</button></p>}
  <Link className="inline-flex min-h-11 items-center text-xs underline" to="/settings/integrations/github">Manage GitHub connection</Link>
 </div>
}
