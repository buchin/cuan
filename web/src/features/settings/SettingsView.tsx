import {
  useLanguage,
  type LanguagePreference,
} from '../../context/LanguageContext'
import { useState } from 'react'
import { Link, useLocation } from 'react-router-dom'
import type { Project } from '../../api/types'
import { useTheme } from '../../context/ThemeContext'
import { IntegrationsView } from './IntegrationsView'
import { TeamView } from '../team/TeamView'

interface SettingsViewProps {
  createAgentRequested?: boolean
  onCreateAgentHandled?: () => void
  activeProject?: Project
  projects?: Project[]
  onAgentAdded?: () => void
  onDeleteProject?: (id: string) => Promise<void>
}
export function SettingsView({
  createAgentRequested = false,
  onCreateAgentHandled,
  activeProject,
  projects = [],
  onAgentAdded,
  onDeleteProject,
}: SettingsViewProps) {
  const { t, preference, setPreference } = useLanguage()
  const location = useLocation()
  const { theme, setTheme } = useTheme()
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')
  const [notice, setNotice] = useState('')
  const section = location.pathname.split('/')[2] || 'general'
  const query = new URLSearchParams(location.search)
  const returnTo = query.get('returnTo') || ''
  const safeReturn = /^\/(inbox)(\/[^?#]*)?$/.test(returnTo) ? returnTo : ''
  if (activeProject)
    return (
      <section className="page-content max-w-3xl space-y-6">
        <Link
          to={`/projects/${encodeURIComponent(activeProject.id)}`}
          className="action-secondary md:hidden"
        >
          {t('← Back to project')}
        </Link>
        <header>
          <p className="mt-2 text-sm text-muted-foreground">
            Settings for project {activeProject.name}.
          </p>
        </header>
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
        <section className="space-y-4 rounded-lg border border-border bg-card p-5">
          <h2 className="text-lg font-semibold">{t('Repository')}</h2>
          <dl className="space-y-3 text-sm">
            <div>
              <dt className="font-medium">{t('Repository URL')}</dt>
              <dd className="mt-1 break-all text-muted-foreground">
                {activeProject.repo_url || t('Not connected')}
              </dd>
            </div>
            <div>
              <dt className="font-medium">{t('Default branch')}</dt>
              <dd className="mt-1 text-muted-foreground">
                {activeProject.default_branch || t('Not available')}
              </dd>
            </div>
          </dl>
          <p className="text-sm text-muted-foreground">
            These details reflect the saved project configuration. Editing
            repository connections is not available here yet.
          </p>
        </section>
        {onDeleteProject && !activeProject.is_archived && (
          <section className="rounded-lg border border-border bg-card p-5">
            <h2 className="text-lg font-semibold">{t('Archive project')}</h2>
            <p className="my-3 text-sm text-muted-foreground">
              Move this project to the archive. You can restore it from
              Projects. Your repository and data are preserved.
            </p>
            <button
              disabled={busy}
              className="action-secondary"
              onClick={async () => {
                if (!confirm(`Archive project "${activeProject.name}"?`)) return
                setBusy(true)
                setError('')
                try {
                  await onDeleteProject(activeProject.id)
                  setNotice('Project archived.')
                } catch {
                  setError('Could not archive the project. Try again.')
                } finally {
                  setBusy(false)
                }
              }}
            >
              {t('Archive project')}
            </button>
          </section>
        )}
      </section>
    )
  return (
    <section className="page-content space-y-5">
      <header>
        <p className="mt-1 text-sm text-muted-foreground">
          {t('Manage your workspace and agent connections.')}
        </p>
      </header>
      <nav
        aria-label={t('Settings sections')}
        className="flex gap-1 overflow-x-auto border-b border-border"
      >
        {[
          ['general', t('General')],
          ['agents', t('Agents')],
          ['integrations', t('Integrations')],
          ['appearance', t('Appearance')],
        ].map(([id, label]) => (
          <Link
            key={id}
            to={id === 'general' ? '/settings' : `/settings/${id}`}
            aria-current={section === id ? 'page' : undefined}
            className={`inline-flex min-h-11 shrink-0 items-center border-b-2 px-3 text-sm font-medium focus-visible:outline focus-visible:outline-2 focus-visible:outline-ring ${section === id ? 'border-primary text-primary' : 'border-transparent text-muted-foreground hover:text-foreground'}`}
          >
            {t(label)}
          </Link>
        ))}
      </nav>
      {section === 'agents' ? (
        <>
          {safeReturn && (
            <Link to={safeReturn} className="action-secondary">
              {t('← Back to conversation')}
            </Link>
          )}
          <TeamView
            showHeading={false}
            showCreateAction={false}
            createRequested={createAgentRequested}
            onCreateHandled={onCreateAgentHandled}
            focusAgentId={query.get('agent') || undefined}
            onAgentAdded={onAgentAdded}
          />
        </>
      ) : section === 'integrations' ? (
        <IntegrationsView />
      ) : section === 'appearance' ? (
        <section className="rounded-lg border border-border bg-card p-5">
          <label
            htmlFor="interface-language"
            className="block text-sm font-medium"
          >
            {t('Language')}
          </label>
          <p className="mt-2 text-sm text-muted-foreground">
            {t(
              'Choose your interface language. Conversation content stays unchanged.',
            )}
          </p>
          <select
            id="interface-language"
            className="field-control mt-3 mb-6 max-w-sm"
            value={preference}
            onChange={(e) =>
              setPreference(e.target.value as LanguagePreference)
            }
          >
            <option value="auto">{t('Auto (browser language)')}</option>
            <option value="en">{t('English')}</option>
            <option value="id">Bahasa Indonesia</option>
          </select>
          <h2 className="text-lg font-semibold">{t('Appearance')}</h2>
          <p className="mt-2 text-sm text-muted-foreground">
            {t(
              'Cuan follows your preference. Your choice is saved in this browser.',
            )}
          </p>
          <div className="mt-4 flex flex-wrap gap-3">
            {(['light', 'dark', 'system'] as const).map((id) => (
              <button
                key={id}
                type="button"
                aria-pressed={theme === id}
                onClick={() => setTheme(id)}
                className={`action-secondary ${theme === id ? 'bg-accent font-semibold ring-2 ring-ring' : ''}`}
              >
                {
                  { light: t('Light'), dark: t('Dark'), system: t('System') }[
                    id
                  ]
                }
              </button>
            ))}
          </div>
        </section>
      ) : (
        <div className="space-y-4">
          <section className="rounded-lg border border-border bg-card p-5">
            <h2 className="text-lg font-semibold">{t('Agents')}</h2>
            <p className="my-3 text-sm text-muted-foreground">
              {t('Add agents or check existing account connections.')}
            </p>
            <Link to="/settings/agents" className="action-primary">
              {t('Manage agents')}
            </Link>
          </section>
          <section className="rounded-lg border border-border bg-card p-5">
            <h2 className="text-lg font-semibold">{t('Project settings')}</h2>
            <p className="my-3 text-sm text-muted-foreground">
              {t('Select a project to view its repository configuration.')}
            </p>
            <ul className="space-y-2">
              {projects
                .filter((p) => !p.is_archived)
                .map((p) => (
                  <li key={p.id}>
                    <Link
                      to={`/projects/${encodeURIComponent(p.id)}/settings`}
                      className="action-secondary"
                    >
                      {p.name}
                    </Link>
                  </li>
                ))}
            </ul>
            {!projects.some((p) => !p.is_archived) && (
              <Link to="/projects" className="action-secondary">
                {t('Open Projects')}
              </Link>
            )}
          </section>
        </div>
      )}
    </section>
  )
}
