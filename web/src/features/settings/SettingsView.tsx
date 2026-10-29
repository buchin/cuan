import { ProjectSettings } from './ProjectSettings'
import { ProfileSettings } from './ProfileSettings'
import { NotificationSettings } from './NotificationSettings'
import { InstallAppCard } from './InstallAppCard'
import { AppUpdateCard } from './AppUpdateCard'
import { ScheduledJobsCard } from './ScheduledJobsCard'
import { SchedulesView } from '../schedules/SchedulesView'
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
import { AgentMemoryView } from './AgentMemoryView'
import { SettingsMasterNav } from './SettingsMasterNav'
import { Brain, ChevronLeft } from 'lucide-react'

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

  const rawSubSection = location.pathname.split('/')[2]
  const hasExplicitSection = Boolean(rawSubSection)
  const section = rawSubSection || 'general'
  const isAccount = location.pathname.startsWith('/account')

  useEffect(() => {
    if (location.hash === '#install-cuan' && section === 'general') {
      document
        .getElementById('install-cuan')
        ?.scrollIntoView({ block: 'start' })
    }
  }, [location.hash, section])

  const query = new URLSearchParams(location.search)
  const returnTo = query.get('returnTo') || ''
  const safeReturn =
    returnTo === '/chat' || returnTo === '/studio' || /^\/(inbox)(\/[^?#]*)?$/.test(returnTo)
      ? (returnTo === '/studio' ? '/chat' : returnTo) : ''

  if (activeProject) {
    return (
      <ProjectSettings
        key={activeProject.id}
        project={activeProject}
        embedded={embeddedProject}
        onUpdated={onProjectUpdated}
        onArchive={onDeleteProject}
      />
    )
  }

  return (
    <div className="mx-auto w-full max-w-7xl flex flex-col md:flex-row min-h-[calc(100vh-4rem)]">
      {/* Hidden accessible navigation tabs for screen readers and link compatibility */}
      <nav
        aria-label={t('Settings sections')}
        className="sr-only overflow-x-auto"
      >
        {[
          ['general', t('General')],
          ['memory', t('Agent Memory')],
          ['schedules', t('Schedules')],
          ['profile', t('Profile & password')],
          ['contacts', t('Contacts')],
          ['agents', t('Agents')],
          ['integrations', t('Integrations')],
          ['appearance', t('Appearance')],
        ].map(([id, label]) => {
          const isAccount = location.pathname.startsWith('/account')
          const to = isAccount
            ? `/account/${id}`
            : id === 'contacts' ? '/account/contacts' : id === 'general' ? '/settings' : `/settings/${id}`
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

      {/* Master Sidebar (Kiri) - w-72 / w-80 di desktop */}
      <aside
        className={`w-full md:w-72 lg:w-80 shrink-0 border-b md:border-b-0 md:border-r border-border bg-card/25 ${
          hasExplicitSection ? 'hidden md:block' : 'block'
        }`}
      >
        <div className="sticky top-0 p-4 md:p-5 overflow-y-auto max-h-[calc(100vh-4rem)] space-y-4">
          <header className="hidden md:block pb-1">
            <h1 className="text-xl font-bold tracking-tight text-foreground">
              {t(isAccount ? 'Account' : 'Settings')}
            </h1>
            <p className="mt-1 text-xs text-muted-foreground">
              {t(
                isAccount
                  ? 'Personal profile, preferences, and relations.'
                  : 'Manage your workspace, AI intelligence, and personal account.',
              )}
            </p>
          </header>

          <SettingsMasterNav
            currentSection={section}
            isAccount={isAccount}
            safeReturn={safeReturn}
          />
        </div>
      </aside>

      {/* Detail Content (Kanan) - flex-1 min-w-0 max-w-4xl */}
      <main
        className={`flex-1 min-w-0 overflow-y-auto p-4 md:p-6 lg:p-8 ${
          !hasExplicitSection ? 'hidden md:block' : 'block'
        }`}
      >
        <div className="max-w-4xl mx-auto space-y-6">
          {/* Mobile Back Button to return to categorized master menu */}
          <div className="md:hidden">
            <Link
              to={isAccount ? '/account' : '/settings'}
              className="inline-flex min-h-11 items-center gap-1.5 rounded-md text-sm font-medium text-muted-foreground hover:text-foreground transition-colors"
            >
              <ChevronLeft className="h-4 w-4 shrink-0" />
              <span>{t('← Back to Settings')}</span>
            </Link>
          </div>

          {!(section === 'integrations' && location.pathname.split('/')[3]) && (
            <header className="hidden sm:block pb-2 border-b border-border/50">
              <h2 className="text-lg font-semibold capitalize text-foreground">
                {t(
                  section === 'general'
                    ? 'General'
                    : section === 'memory'
                      ? 'Agent Memory'
                      : section === 'schedules'
                        ? 'Schedules'
                        : section === 'profile'
                          ? 'Profile & password'
                          : section === 'agents'
                            ? 'Agents'
                            : section === 'integrations'
                              ? 'Integrations'
                              : section === 'appearance'
                                ? 'Appearance'
                                : section === 'notifications'
                                  ? 'Notifications'
                                  : section,
                )}
              </h2>
              <p className="mt-1 text-sm text-muted-foreground">
                {t(
                  section === 'integrations'
                    ? 'Connect the services your projects need.'
                    : section === 'memory'
                      ? 'Review learned brand guidelines, copywriting style, and workflow preferences.'
                      : section === 'schedules'
                        ? 'Automated recurring background routines.'
                        : section === 'appearance'
                          ? 'Theme modes, interface language, and display.'
                          : 'Manage your workspace and agent connections.',
                )}
              </p>
            </header>
          )}

          {section === 'profile' ? (
            <ProfileSettings />
          ) : section === 'memory' ? (
            <AgentMemoryView />
          ) : section === 'schedules' ? (
            <SchedulesView embedded />
          ) : section === 'notifications' ? (
            <NotificationSettings />
          ) : section === 'agents' ? (
            <div className="space-y-4">
              {safeReturn && (
                <Link to={safeReturn} className="action-secondary">
                  {t(
                    safeReturn === '/chat' || safeReturn === '/studio'
                      ? '← Back to Chat'
                      : '← Back to conversation',
                  )}
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
                  if (safeReturn === '/chat' || safeReturn === '/studio') {
                    navigate('/chat')
                  }
                }}
              />
            </div>
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
                    className={`action-secondary ${
                      theme === id ? 'bg-accent font-semibold ring-2 ring-ring' : ''
                    }`}
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
                <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-3">
                  <div>
                    <h2 className="text-lg font-semibold">
                      {t('Agent Memory / Taste')}
                    </h2>
                    <p className="mt-1 text-sm text-muted-foreground">
                      {t(
                        'Review learned brand guidelines, copywriting style, and workflow preferences.',
                      )}
                    </p>
                  </div>
                  <Link
                    to="/settings/memory"
                    className="action-secondary shrink-0 inline-flex items-center gap-1.5"
                  >
                    <Brain className="h-4 w-4 text-primary" />
                    <span>{t('Manage memory')}</span>
                  </Link>
                </div>
              </section>
              <AppUpdateCard />
              <ScheduledJobsCard />
              <InstallAppCard />
              <section className="rounded-lg border border-border bg-card p-5">
                <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-3">
                  <div>
                    <h2 className="text-lg font-semibold">
                      {t('Keyboard shortcuts')}
                    </h2>
                    <p className="mt-1 text-sm text-muted-foreground">
                      {t(
                        'Quick navigation and action hotkeys for desktop workflow.',
                      )}
                    </p>
                  </div>
                  <button
                    type="button"
                    onClick={() =>
                      window.dispatchEvent(new Event('cuan:keyboard-shortcuts'))
                    }
                    className="action-secondary shrink-0"
                  >
                    {t('View shortcuts')}
                  </button>
                </div>
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
        </div>
      </main>
    </div>
  )
}
