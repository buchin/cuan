import { BackAction, DetailLayout } from '../../components/layout/DetailLayout'
import { GeneralSettings } from './GeneralSettings'
import { IntegrationOverview } from './IntegrationOverview'
import { GitHubIntegration } from './GitHubIntegration'
import { IntegrationsView } from './IntegrationsView'
import { ResendSettingsCard } from './ResendSettingsCard'
import { ProjectSettings } from './ProjectSettings'
import { ProfileSettings } from './ProfileSettings'
import { NotificationSettings } from './NotificationSettings'
export { GeneralSettings } from './GeneralSettings'
import { SchedulesView } from '../schedules/SchedulesView'
import {
  useLanguage,
  type LanguagePreference,
} from '../../context/LanguageContext'
import { useEffect } from 'react'
import { Link, useLocation } from 'react-router-dom'
import type { Project } from '../../api/types'
import { useTheme } from '../../context/ThemeContext'
import { AppsSkillsView } from './AppsSkillsView'
import { AgentSettingsView } from '../agents/AgentSettingsView'
import { TeamMembersView } from '../team/TeamMembersView'
import { AgentMemoryView } from './AgentMemoryView'
import { SettingsMasterNav } from './SettingsMasterNav'
import { GroupedSection, GroupedRow } from '../../components/layout/GroupedSection'

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
  createAgentRequested: _createAgentRequested = false,
  onCreateAgentHandled: _onCreateAgentHandled,
  activeProject,
  embeddedProject = false,
  projects = [],
  onAgentAdded: _onAgentAdded,
  onDeleteProject,
  onProjectUpdated,
}: SettingsViewProps) {
  const { t, preference, setPreference } = useLanguage()
  const location = useLocation()
  const { theme, setTheme } = useTheme()

  const rawSubSection = location.pathname.split('/')[2]
  const hasExplicitSection = Boolean(rawSubSection)
  const isAccount = location.pathname.startsWith('/account')
  const section = rawSubSection || (isAccount ? 'profile' : 'general') // fallback: rawSubSection || 'general'

  useEffect(() => {
    if (location.hash === '#install-cuan' && section === 'general') {
      document
        .getElementById('install-cuan')
        ?.scrollIntoView({ block: 'start' })
    }
  }, [location.hash, section])

  const query = new URLSearchParams(location.search)
  const returnTo = query.get('returnTo') || query.get('return_to') || ''
  const safeReturn =
    returnTo === '/chat' || returnTo === '/studio' || /^\/inbox(?:\/[A-Za-z0-9_-]+)?(?:\?[^#]*)?$/.test(returnTo) || /^\/tasks\/[A-Za-z0-9_-]+(?:\/sessions\/[A-Za-z0-9_-]+)?$/.test(returnTo)
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
    <div className="mx-auto w-full max-w-4xl min-h-[calc(100vh-4rem)]">
      {/* Hidden accessible navigation tabs for screen readers and link compatibility */}
      <nav
        aria-label={t('Settings sections')}
        className="sr-only overflow-x-auto"
      >
        {[
          ['general', t('General')],
          ['team', t('Team')],
          ['agent', t('Assistant & AI Brain')],
          ['memory', t('Agent Memory')],
          ['schedules', t('Schedules')],
          ['profile', t('Profile & password')],
          ['contacts', t('Contacts')],
          ['apps-skills', t('Apps & Skills')],
          ['integrations', t('Integrations')],
          ['appearance', t('Appearance')],
          ['notifications', t('Notifications')],
        ]
          .map(([id, label]) => {
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

      {/* Settings index uses the same single-column canvas as other pages. */}
      <section
        className={hasExplicitSection ? 'hidden' : 'block p-4 md:p-6'}
      >
        <div className="space-y-6">
          <SettingsMasterNav
            currentSection={hasExplicitSection ? section : ''}
            isAccount={isAccount}
            safeReturn={safeReturn}
          />
        </div>
      </section>

      {/* Detail Content (Kanan) - flex-1 min-w-0 max-w-4xl */}
      <main
        className={`flex-1 min-w-0 overflow-y-auto p-4 md:p-6 ${
          !hasExplicitSection ? 'hidden' : 'block'
        }`}
      >
        <DetailLayout back={<BackAction to={safeReturn || ((section === 'integrations' && location.pathname.split('/')[3] ? '/settings/integrations' : isAccount ? '/account' : '/settings') + location.search)}>{t(safeReturn ? (safeReturn.startsWith('/tasks/') ? 'Back to task' : 'Back to conversation') : section === 'integrations' && location.pathname.split('/')[3] ? 'Back to integrations' : isAccount ? 'Back to Account' : 'Back to Settings')}</BackAction>}
          description={t(({ general: 'Application status, updates, and installation.', agent: 'One assistant, with AI accounts managed here.', agents: 'One assistant, with AI accounts managed here.', 'ai-providers': 'One assistant, with AI accounts managed here.', memory: 'Review learned brand guidelines, copywriting style, and workflow preferences.', schedules: 'Automated recurring background routines.', appearance: 'Theme modes, interface language, and display.', notifications: 'Choose which updates reach you and when.', profile: 'Manage your profile and sign-in details.', team: 'Manage human staff, members, and collaborators', integrations: 'Connect the services your projects need.', 'apps-skills': 'Apps and capabilities available to your assistant.' } as Record<string,string>)[section] || '')}>
          {section === 'profile' ? (
            <ProfileSettings />
          ) : section === 'team' ? (
            <TeamMembersView showHeading={false} />
          ) : section === 'agent' || section === 'agents' || section === 'ai-providers' ? (
            <AgentSettingsView showHeading={false} />
          ) : section === 'memory' ? (
            <AgentMemoryView showHeading={false} />
          ) : section === 'schedules' ? (
            <SchedulesView embedded />
          ) : section === 'notifications' ? (
            <NotificationSettings />
          ) : section === 'integrations' ? (
            location.pathname.split('/')[3] === 'github' ? <GitHubIntegration /> : location.pathname.split('/')[3] === 'live-chat' ? <IntegrationsView /> : location.pathname.split('/')[3] === 'resend' ? <ResendSettingsCard /> : <IntegrationOverview />
          ) : section === 'apps-skills' ? (
            <AppsSkillsView showHeading={false} />
          ) : section === 'appearance' ? (
            /* Apple / Things 3 Inset Grouped Rows for Appearance */
            <div className="space-y-6">
              <GroupedSection
                title={t('Interface & Language')}
                description={t('Preferences for language and visual display modes.')}
                footerNote={t('Changes apply immediately across all workspace views.')}
              >
                <GroupedRow
                  label={t('Language')}
                  description={t(
                    'Choose your interface language. Conversation content stays unchanged.',
                  )}
                  control={
                    <select
                      id="interface-language"
                      aria-label={t('Language')}
                      className="field-control text-xs w-44 sm:w-52"
                      value={preference}
                      onChange={(e) =>
                        setPreference(e.target.value as LanguagePreference)
                      }
                    >
                      <option value="auto">{t('Auto (browser language)')}</option>
                      <option value="en">{t('English')}</option>
                      <option value="id">Bahasa Indonesia</option>
                    </select>
                  }
                />
                <GroupedRow
                  label={t('Appearance')}
                  description={t(
                    'Cuan follows your preference. Your choice is saved in this browser.',
                  )}
                  control={
                    <div className="flex items-center gap-1.5">
                      {(['light', 'dark', 'system'] as const).map((id) => (
                        <button
                          key={id}
                          type="button"
                          aria-pressed={theme === id}
                          onClick={() => setTheme(id)}
                          className={`action-secondary text-xs px-2.5 py-1 ${
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
                  }
                />
              </GroupedSection>
            </div>
          ) : (
            <GeneralSettings projects={projects} />
          )}
        </DetailLayout>
      </main>
    </div>
  )
}
