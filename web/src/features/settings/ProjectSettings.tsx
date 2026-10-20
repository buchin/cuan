import { useEffect, useState } from 'react'
import { GitHubRepositoryPicker } from './GitHubRepositoryPicker'
import { Link } from 'react-router-dom'
import type { Project } from '../../api/types'
import { ApiError } from '../../api/client'
import { projectsApi } from '../../api/projects'
import { useLanguage } from '../../context/LanguageContext'

export function ProjectSettings({ project, onUpdated, onArchive, embedded = false }: {
  embedded?: boolean
  project: Project
  onUpdated?: () => Promise<void>
  onArchive?: (id: string) => Promise<void>
}) {
  const { t } = useLanguage()
  const values = (p: Project) => ({
    name: p.name, goal: String(p.metadata?.cuan_settings?.goal || ''),
    instructions: String(p.metadata?.cuan_settings?.instructions || ''),
    repo_url: p.repo_url || '', default_branch: p.default_branch || 'main',
  })
  const [baseline, setBaseline] = useState(() => values(project))
  const [form, setForm] = useState(baseline)
  const [version, setVersion] = useState(project.updated_at)
  const [conflict, setConflict] = useState(false)
  const [busy, setBusy] = useState(false), [error, setError] = useState(''), [notice, setNotice] = useState('')
  const dirty = JSON.stringify(form) !== JSON.stringify(baseline)
  useEffect(() => {
    if (!dirty) return
    const warn = (event: BeforeUnloadEvent) => { event.preventDefault(); event.returnValue = '' }
    window.addEventListener('beforeunload', warn)
    return () => window.removeEventListener('beforeunload', warn)
  }, [dirty])
  const change = (key: keyof typeof form, value: string) => { setForm(old => ({ ...old, [key]: value })); setNotice(''); setError('') }
  return <section className={`mx-auto w-full max-w-2xl space-y-6 ${embedded ? 'px-4 py-5 sm:px-5' : 'page-content'}`}>
    {!embedded && <Link to={'/projects/' + encodeURIComponent(project.id) + '/tasks'} className="inline-flex min-h-11 items-center text-sm text-muted-foreground hover:text-foreground">{t('← Back to project')}</Link>}
    <form aria-label={t('Project settings')} className="space-y-5" onSubmit={async event => {
      event.preventDefault()
      if (busy || !dirty) return
      const connectionChanged = form.repo_url !== baseline.repo_url || form.default_branch !== baseline.default_branch
      if (connectionChanged && !confirm(t('Change the repository connection for new tasks? Existing tasks keep their original workspace.'))) return
      setBusy(true); setError(''); setNotice(''); setConflict(false)
      try {
        const saved = await projectsApi.updateSettings(project.id, { ...form, expected_updated_at: version })
        const next = values(saved); setBaseline(next); setForm(next); setVersion(saved.updated_at)
        setNotice('Changes saved.')
        try { await onUpdated?.() } catch { setNotice('Changes saved. Refresh the page to update navigation.') }
      } catch (cause) { setError(cause instanceof Error ? cause.message : 'Could not save changes. Try again.'); setConflict(cause instanceof ApiError && cause.status === 409) }
      finally { setBusy(false) }
    }}>
      <div className="space-y-2">
        <label htmlFor="project-name" className="text-sm font-medium">{t('Project name')}</label>
        <input id="project-name" className="field-control" value={form.name} required maxLength={120} disabled={busy} onChange={event => change('name', event.target.value)} />
      </div>
      <div className="space-y-2">
        <label htmlFor="project-goal" className="text-sm font-medium">{t('Goal')}</label>
        <textarea id="project-goal" className="field-control min-h-24 resize-y py-3" rows={3} maxLength={2000} disabled={busy} value={form.goal} onChange={event => change('goal', event.target.value)} placeholder={t('What should this project achieve?')} />
      </div>
      <div className="space-y-2">
        <label htmlFor="project-instructions" className="text-sm font-medium">{t('Instructions for Cuan')}</label>
        <textarea id="project-instructions" className="field-control min-h-28 resize-y py-3" rows={4} maxLength={4000} disabled={busy} value={form.instructions} onChange={event => change('instructions', event.target.value)} placeholder={t('Preferences, constraints, or context Cuan should remember.')} />
        <p className="text-xs text-muted-foreground">{t('Applies to new tasks. Existing tasks keep their original instructions.')}</p>
      </div>
      <details className="border-t pt-2">
        <summary className="flex min-h-11 cursor-pointer items-center text-sm font-medium">{t('Advanced')}</summary>
        <div className="space-y-4 pb-3 pt-2">
          <div className="space-y-2"><label htmlFor="project-repository" className="text-sm font-medium">{t('Repository URL')}</label>
            <GitHubRepositoryPicker disabled={busy} onSelect={url => { setForm(old => ({ ...old, repo_url: url, default_branch: url === baseline.repo_url ? baseline.default_branch : '' })); setNotice(''); setError('') }} />
            <input id="project-repository" className="field-control" disabled={busy} maxLength={500} value={form.repo_url} onChange={event => { setForm(old => ({ ...old, repo_url: event.target.value, default_branch: event.target.value === baseline.repo_url ? baseline.default_branch : '' })); setNotice(''); setError('') }} placeholder="https://github.com/owner/repository" /></div>
          <div className="space-y-2"><label htmlFor="project-branch" className="text-sm font-medium">{t('Default branch')}</label>
            <input id="project-branch" className="field-control" disabled={busy} maxLength={200} value={form.default_branch} placeholder={t('Use repository default')} onChange={event => change('default_branch', event.target.value)} /></div>
          <p className="text-xs leading-5 text-muted-foreground">{t('Leave the branch empty to use the repository default when connecting. A new connection is verified in a separate workspace. Existing tasks retain their original workspace, including when resumed.')}</p>
        </div>
      </details>
      {error && <div role="alert" className="text-sm text-destructive">{t(error)}
        {conflict && <button type="button" className="action-secondary mt-2" disabled={busy} onClick={async () => {
          if (!confirm(t('Discard your draft and reload saved settings?'))) return
          setBusy(true)
          try {
            const latest = await projectsApi.get(project.id), next = values(latest)
            setForm(next); setBaseline(next); setVersion(latest.updated_at); setError(''); setConflict(false)
          } catch { setError('Could not load settings. Try again.') } finally { setBusy(false) }
        }}>{t('Reload saved settings')}</button>}
      </div>}
      <div className="flex flex-wrap items-center gap-3">
        <button type="submit" className="action-primary" disabled={busy || !dirty || !form.name.trim()}>{t(busy ? 'Saving…' : 'Save changes')}</button>
        {dirty && <button type="button" className="action-secondary" disabled={busy} onClick={() => { setForm(baseline); setError(''); setNotice('') }}>{t('Discard changes')}</button>}
        <span role="status" className="text-xs text-muted-foreground">{t(notice || (dirty ? 'Unsaved changes' : ''))}</span>
      </div>
    </form>
    {onArchive && !project.is_archived && <section className="border-t pt-5">
      <h2 className="text-sm font-medium">{t('Archive project')}</h2>
      <p className="my-2 text-sm text-muted-foreground">{t('Hide this project from active projects. Your data stays available.')}</p>
      <button type="button" className="action-secondary" disabled={busy || dirty} onClick={async () => {
        if (!confirm(t('Archive project') + ' “' + project.name + '”?')) return
        setBusy(true); setError('')
        try { await onArchive(project.id); setNotice('Project archived.') } catch { setError('Could not archive the project. Try again.') } finally { setBusy(false) }
      }}>{t('Archive project')}</button>
    </section>}
  </section>
}
