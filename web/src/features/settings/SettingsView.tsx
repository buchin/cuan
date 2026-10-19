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
          ← Back to project
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
          <h2 className="text-lg font-semibold">Repository</h2>
          <dl className="space-y-3 text-sm">
            <div>
              <dt className="font-medium">Repository URL</dt>
              <dd className="mt-1 break-all text-muted-foreground">
                {activeProject.repo_url || 'Not connected'}
              </dd>
            </div>
            <div>
              <dt className="font-medium">Default branch</dt>
              <dd className="mt-1 text-muted-foreground">
                {activeProject.default_branch || 'Not available'}
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
            <h2 className="text-lg font-semibold">Archive project</h2>
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
              Archive project
            </button>
          </section>
        )}
      </section>
    )
  return (
    <section className="page-content space-y-5">
      <header>
        <p className="mt-1 text-sm text-muted-foreground">
          Manage your workspace and agent connections.
        </p>
      </header>
      <nav
        aria-label="Settings sections"
        className="flex gap-1 overflow-x-auto border-b border-border"
      >
        {[
          ['general', 'Umum'],
          ['agents', 'Agents'],
          ['integrations', 'Integrations'],
          ['appearance', 'Appearance'],
        ].map(([id, label]) => (
          <Link
            key={id}
            to={id === 'general' ? '/settings' : `/settings/${id}`}
            aria-current={section === id ? 'page' : undefined}
            className={`inline-flex min-h-11 shrink-0 items-center border-b-2 px-3 text-sm font-medium focus-visible:outline focus-visible:outline-2 focus-visible:outline-ring ${section === id ? 'border-primary text-primary' : 'border-transparent text-muted-foreground hover:text-foreground'}`}
          >
            {label}
          </Link>
        ))}
      </nav>
      {section === 'agents' ? (
        <>
          {safeReturn && (
            <Link to={safeReturn} className="action-secondary">
              ← Back to conversation
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
          <h2 className="text-lg font-semibold">Appearance</h2>
          <p className="mt-2 text-sm text-muted-foreground">
            Cuan follows your preference. Your choice is saved in this browser.
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
                {{ light: 'Light', dark: 'Dark', system: 'System' }[id]}
              </button>
            ))}
          </div>
        </section>
      ) : (
        <div className="space-y-4">
          <section className="rounded-lg border border-border bg-card p-5">
            <h2 className="text-lg font-semibold">Agents</h2>
            <p className="my-3 text-sm text-muted-foreground">
              Add agents or check existing account connections.
            </p>
            <Link to="/settings/agents" className="action-primary">
              Manage agents
            </Link>
          </section>
          <section className="rounded-lg border border-border bg-card p-5">
            <h2 className="text-lg font-semibold">Project settings</h2>
            <p className="my-3 text-sm text-muted-foreground">
              Select a project to view its repository configuration.
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
                Open Projects
              </Link>
            )}
          </section>
        </div>
      )}
    </section>
  )
}
