import { ProjectSettings } from './ProjectSettings'
import { ProfileSettings } from './ProfileSettings'
import { NotificationSettings } from './NotificationSettings'
import { InstallAppCard } from './InstallAppCard'
import {
  useLanguage,
  type LanguagePreference,
} from '../../context/LanguageContext'
import { useEffect } from 'react'
import { Link, useLocation, useNavigate } from 'react-router-dom'
import type { Project } from '../../api/types'
import { useTheme } from '../../context/ThemeContext'
import { IntegrationsView } from './IntegrationsView'
import { TeamView } from '../team/TeamView'

interface SettingsViewProps {
  createAgentRequested?: boolean
  onCreateAgentHandled?: () => void
  embeddedProject?: boolean
  activeProject?: Project
  projects?: Project[]
  onAgentAdded?: () => void
  onProjectUpdated?: () => Promise<void>
  onDeleteProject?: (id: string) => Promise<void>
}
export function SettingsView({
  createAgentRequested = false,
  onCreateAgentHandled,
  activeProject,
  embeddedProject = false,
  projects = [],
  onAgentAdded,
  onDeleteProject,
  onProjectUpdated,
}: SettingsViewProps) {
  const { t, preference, setPreference } = useLanguage()
  const location = useLocation()
  const navigate = useNavigate()
  const { theme, setTheme } = useTheme()
  const section = location.pathname.split('/')[2] || 'general'
  useEffect(() => {
    if (location.hash === '#install-cuan' && section === 'general')
      document
        .getElementById('install-cuan')
        ?.scrollIntoView({ block: 'start' })
  }, [location.hash, section])
  const query = new URLSearchParams(location.search)
  const returnTo = query.get('returnTo') || ''
  const safeReturn =
    returnTo === '/studio' || /^\/(inbox)(\/[^?#]*)?$/.test(returnTo)
      ? returnTo : ''
  if (activeProject) return <ProjectSettings key={activeProject.id} project={activeProject} embedded={embeddedProject} onUpdated={onProjectUpdated} onArchive={onDeleteProject} />
  return (
    <section className="page-content space-y-5">
      {!(section === 'integrations' && location.pathname.split('/')[3]) && <header className="hidden sm:block">
        <p className="mt-1 text-sm text-muted-foreground">
          {t(section === 'integrations' ? 'Connect the services your projects need.' : 'Manage your workspace and agent connections.')}
        </p>
      </header>}
      <nav
        aria-label={t('Settings sections')}
        className="flex gap-1 overflow-x-auto border-b border-border [scrollbar-width:none] [-ms-overflow-style:none] [&::-webkit-scrollbar]:hidden"
      >
        {[
          ['contacts', t('Contacts')],
          ['general', t('General')],
          ['agents', t('Agents')],
          ['integrations', t('Integrations')],
          ['appearance', t('Appearance')],
        ].map(([id, label]) => {
          const isAccount = location.pathname.startsWith('/account')
          const to = isAccount
            ? `/account/${id}`
            : id === 'general' ? '/settings' : `/settings/${id}`
          return (
            <Link
              key={id}
              to={to}
              aria-current={section === id ? 'page' : undefined}
              className={`inline-flex min-h-11 shrink-0 items-center border-b-2 px-3 text-sm font-medium focus-visible:outline focus-visible:outline-2 focus-visible:outline-ring ${section === id ? 'border-primary text-primary' : 'border-transparent text-muted-foreground hover:text-foreground'}`}
            >
              {t(label)}
            </Link>
          )
        })}
      </nav>
      {section === 'profile' ? (
        <ProfileSettings />
      ) : section === 'notifications' ? (
        <NotificationSettings />
      ) : section === 'agents' ? (
        <>
          {safeReturn && (
            <Link to={safeReturn} className="action-secondary">
              {t(safeReturn === '/studio' ? '← Back to Studio' : '← Back to conversation')}
            </Link>
          )}
          <TeamView
            showHeading={false}
            showCreateAction={false}
            createRequested={createAgentRequested}
            onCreateHandled={onCreateAgentHandled}
            focusAgentId={query.get('agent') || undefined}
            onAgentAdded={onAgentAdded}
            onAgentConnected={() => {
              if (safeReturn === '/studio') navigate('/studio')
            }}
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
          <InstallAppCard />
          <section className="rounded-lg border border-border bg-card p-5">
            <h2 className="text-lg font-semibold">{t('Contacts')}</h2>
            <p className="my-3 text-sm text-muted-foreground">
              {t('Manage your contacts, tags, customer records, and notes.')}
            </p>
            <Link
              to={location.pathname.startsWith('/account') ? '/account/contacts' : '/contacts'}
              className="action-primary"
            >
              {t('View contacts')}
            </Link>
          </section>
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
