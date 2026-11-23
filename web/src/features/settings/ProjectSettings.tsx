import { useEffect, useState } from 'react'
import { Eye, EyeOff } from 'lucide-react'
import { GitHubRepositoryPicker } from './GitHubRepositoryPicker'
import { Link, useLocation } from 'react-router-dom'
import type { Project } from '../../api/types'
import { ApiError } from '../../api/client'
import { projectsApi } from '../../api/projects'
import { useLanguage } from '../../context/LanguageContext'
import { FormField, Input, Textarea } from '../../components/ui/forms'

export function ProjectSettings({ project, onUpdated, onArchive, embedded = false }: {
  embedded?: boolean
  project: Project
  onUpdated?: () => Promise<void>
  onArchive?: (id: string) => Promise<void>
}) {
  const { t } = useLanguage()
  const isDefault = project.id === 'default'
  const pathSegments = useLocation().pathname.split('/').filter(Boolean)
  const repositoryView = pathSegments[2] === 'repository'
  const environmentView = pathSegments[2] === 'environment'
  const values = (p: Project) => ({
    name: p.name, goal: String(p.metadata?.cuan_settings?.goal || ''),
    instructions: String(p.metadata?.cuan_settings?.instructions || ''),
    repo_url: p.repo_url || '', default_branch: p.default_branch || '',
    deploy_branch: String(p.metadata?.cuan_settings?.deploy_branch || ''),
    deploy_script: String(p.metadata?.cuan_settings?.deploy_script || ''),
    auto_deploy: p.metadata?.cuan_settings?.auto_deploy !== false,
    environment: String(p.metadata?.cuan_settings?.environment || ''),
  })
  const [baseline, setBaseline] = useState(() => values(project))
  const [form, setForm] = useState(baseline)
  const [version, setVersion] = useState(project.updated_at)
  const [conflict, setConflict] = useState(false)
  const [busy, setBusy] = useState(false), [error, setError] = useState(''), [notice, setNotice] = useState('')
  const [showSecrets, setShowSecrets] = useState(false)
  const dirty = JSON.stringify(form) !== JSON.stringify(baseline)
  useEffect(() => {
    let active = true
    projectsApi.get(project.id).then(latest => {
      if (!active || !latest) return
      const next = values(latest)
      setBaseline(next)
      setForm(current => {
        if (JSON.stringify(current) === JSON.stringify(baseline)) {
          return next
        }
        return current
      })
      setVersion(latest.updated_at)
    }).catch(() => {})
    return () => { active = false }
  }, [project.id])
  useEffect(() => {
    if (project.metadata?.cuan_settings) {
      const next = values(project)
      setBaseline(next)
      setForm(current => {
        if (JSON.stringify(current) === JSON.stringify(baseline)) {
          return next
        }
        return current
      })
      setVersion(project.updated_at)
    }
  }, [project])
  useEffect(() => {
    if (!dirty) return
    const warn = (event: BeforeUnloadEvent) => { event.preventDefault(); event.returnValue = '' }
    window.addEventListener('beforeunload', warn)
    return () => window.removeEventListener('beforeunload', warn)
  }, [dirty])
  const change = <K extends keyof typeof form>(key: K, value: (typeof form)[K]) => {
    setForm(old => ({ ...old, [key]: value }))
    setNotice('')
    setError('')
  }
  return <section className={`mx-auto w-full max-w-2xl space-y-6 ${embedded ? 'px-4 py-5 sm:px-5' : 'page-content'}`}>
    {!embedded && <Link to={'/projects/' + encodeURIComponent(project.id) + '/tasks'} className="inline-flex min-h-11 items-center text-sm text-muted-foreground hover:text-foreground">{t('← Back to project')}</Link>}
    <form aria-label={t(environmentView ? 'Project environment' : repositoryView ? 'Project repository' : 'Project settings')} className="space-y-5" onSubmit={async event => {
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
      {!repositoryView && !environmentView && <>
      <FormField label={t('Project name')} htmlFor="project-name" required>
        <Input
          id="project-name"
          value={form.name}
          required
          maxLength={120}
          disabled={busy || isDefault}
          onChange={event => change('name', event.target.value)}
        />
      </FormField>
      <FormField label={t('Goal')} htmlFor="project-goal">
        <Textarea
          id="project-goal"
          className="min-h-24"
          rows={3}
          maxLength={2000}
          disabled={busy}
          value={form.goal}
          onChange={event => change('goal', event.target.value)}
          placeholder={t('What should this project achieve?')}
        />
      </FormField>
      <FormField
        label={t('Instructions for Cuan')}
        htmlFor="project-instructions"
        helperText={t('Applies to new tasks. Existing tasks keep their original instructions.')}
      >
        <Textarea
          id="project-instructions"
          className="min-h-28"
          rows={4}
          maxLength={4000}
          disabled={busy}
          value={form.instructions}
          onChange={event => change('instructions', event.target.value)}
          placeholder={t('Preferences, constraints, or context Cuan should remember.')}
        />
      </FormField>
      <div className="flex items-center justify-between rounded-xl border border-border/40 bg-muted/20 p-4">
        <div className="space-y-0.5">
          <label htmlFor="project-auto-deploy" className="text-sm font-medium">{t('Auto-deploy completed tasks to live server')}</label>
          <p className="text-xs text-muted-foreground">{t('When enabled (recommended), changes verified by Scrum Master are automatically deployed live. You can override this per task.')}</p>
        </div>
        <input
          id="project-auto-deploy"
          type="checkbox"
          className="h-4 w-4 rounded border-border text-primary focus:ring-primary cursor-pointer"
          checked={form.auto_deploy}
          disabled={busy}
          onChange={event => change('auto_deploy', event.target.checked)}
        />
      </div>
      {form.auto_deploy && !isDefault && (
        <FormField
          label={t('Deploy Script (Optional)')}
          htmlFor="project-deploy-script"
          helperText={t('Shell command or script executed to publish this project. If empty, Cuan uses the default publication verifier.')}
        >
          <Textarea
            id="project-deploy-script"
            mono
            className="min-h-20 leading-relaxed"
            rows={2}
            maxLength={10000}
            disabled={busy}
            value={form.deploy_script}
            onChange={event => change('deploy_script', event.target.value)}
            placeholder={t('e.g. bash scripts/release.sh or npm run deploy')}
          />
        </FormField>
      )}
      {isDefault && <p className="text-xs leading-5 text-muted-foreground">{t('Default is for general work and has no repository. Cuan can create a separate project when your work needs one.')}</p>}
      </>}
      {environmentView && <section className="space-y-5">
        <div className="space-y-2">
          <h2 className="text-sm font-semibold">{t('Environment Variables')}</h2>
          <p className="text-sm leading-6 text-muted-foreground">{t('Variabel lingkungan ini otomatis di-inject ke file .env di direktori kerjaan dan di-export ke sesi agent terminal saat task dijalankan.')}</p>
        </div>
        <div className="space-y-2">
          <div className="flex items-center justify-between">
            <label htmlFor="project-environment" className="text-sm font-medium">{t('.env Configuration')}</label>
            <button
              type="button"
              onClick={() => setShowSecrets(prev => !prev)}
              className="text-xs font-medium text-muted-foreground hover:text-foreground inline-flex items-center gap-1.5 transition-colors cursor-pointer"
            >
              {showSecrets ? <EyeOff className="h-3.5 w-3.5" /> : <Eye className="h-3.5 w-3.5" />}
              <span>{t(showSecrets ? 'Mask secrets' : 'Reveal secrets')}</span>
            </button>
          </div>
          <textarea
            id="project-environment"
            rows={12}
            maxLength={32000}
            disabled={busy}
            value={form.environment}
            onChange={event => change('environment', event.target.value)}
            placeholder={'# Project environment variables\nAPP_ENV=production\nDOJO_VPS_PUBLIC_IP=43.157.241.102\nDOJO_VPS_PASSWORD=...'}
            className={`field-control font-mono text-xs leading-relaxed resize-y py-3 min-h-[240px] ${showSecrets ? '' : 'tracking-wider'}`}
            style={showSecrets ? {} : { WebkitTextSecurity: 'disc' } as any}
          />
          <p className="text-xs text-muted-foreground leading-relaxed">
            {t('Format standar 12-factor: KEY=VALUE per baris. Komentar yang diawali # dan baris kosong diabaikan. Variabel ini akan otomatis di-overlay ke template .env.example di repositori.')}
          </p>
        </div>
      </section>}
      {repositoryView && (isDefault ? <p className="text-sm text-muted-foreground">{t('Default is for general work and has no repository. Cuan can create a separate project when your work needs one.')}</p> : <section className="space-y-5">
        <div className="space-y-2">
          <h2 className="text-sm font-semibold">{t('Repository')}</h2>
          <p className="text-sm leading-6 text-muted-foreground">{t('Connect a repository to keep this project’s files and version history together.')}</p>
        </div>
        {form.repo_url ? <div className="rounded-xl border border-border/40 bg-muted/20 p-4">
          <p className="text-xs text-muted-foreground">{t(form.repo_url === baseline.repo_url ? 'Connected repository' : 'Selected repository')}</p>
          <p className="mt-1 break-all text-sm font-medium">{form.repo_url.replace(/^https?:\/\/(?:www\.)?github\.com\//i, '').replace(/\.git$/, '')}</p>
        </div> : <p className="text-sm text-muted-foreground">{t('No repository connected. This project can still work without one.')}</p>}
        <GitHubRepositoryPicker disabled={busy} onSelect={url => { setForm(old => ({ ...old, repo_url: url, default_branch: url === baseline.repo_url ? baseline.default_branch : '' })); setNotice(''); setError('') }} />
        <details className="border-t border-border/40 pt-2">
        <summary className="flex min-h-11 cursor-pointer items-center text-sm font-medium">{t('Advanced')}</summary>
        <div className="space-y-4 pb-3 pt-2">
          <FormField label={t('Repository URL')} htmlFor="project-repository">
            <Input
              id="project-repository"
              disabled={busy}
              maxLength={500}
              value={form.repo_url}
              onChange={event => { setForm(old => ({ ...old, repo_url: event.target.value, default_branch: event.target.value === baseline.repo_url ? baseline.default_branch : '' })); setNotice(''); setError('') }}
              placeholder="https://github.com/owner/repository"
            />
          </FormField>
          <FormField
            label={t('Default branch')}
            htmlFor="project-branch"
            helperText={t('Leave the branch empty to use the repository default. Changes apply to new work; existing work keeps its connection.')}
          >
            <Input
              id="project-branch"
              disabled={busy}
              maxLength={200}
              value={form.default_branch}
              placeholder={t('Use repository default')}
              onChange={event => change('default_branch', event.target.value)}
            />
          </FormField>
          <FormField
            label={t('Deployment branch')}
            htmlFor="project-deploy-branch"
            helperText={t('Leave empty to deploy from default branch. Used when production release branch differs from development.')}
          >
            <Input
              id="project-deploy-branch"
              disabled={busy}
              maxLength={200}
              value={form.deploy_branch}
              placeholder={t('Same as default branch')}
              onChange={event => change('deploy_branch', event.target.value)}
            />
          </FormField>
        </div>
      </details>
      </section>)}
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
      {(!repositoryView || !isDefault) && <div className="flex flex-wrap items-center gap-3">
        <button type="submit" className="action-primary" disabled={busy || !dirty || !form.name.trim()}>{t(busy ? 'Saving…' : 'Save changes')}</button>
        {dirty && <button type="button" className="action-secondary" disabled={busy} onClick={() => { setForm(baseline); setError(''); setNotice('') }}>{t('Discard changes')}</button>}
        <span role="status" className="text-xs text-muted-foreground">{t(notice || (dirty ? 'Unsaved changes' : ''))}</span>
      </div>}
    </form>
    {!repositoryView && !environmentView && onArchive && !isDefault && !project.is_archived && <section className="border-t pt-5">
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
