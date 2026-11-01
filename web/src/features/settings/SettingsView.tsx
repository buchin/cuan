import { ProjectSettings } from './ProjectSettings'
import { ProfileSettings } from './ProfileSettings'
import { NotificationSettings } from './NotificationSettings'
import { InstallAppCard } from './InstallAppCard'
import { AppUpdateCard } from './AppUpdateCard'
import { ScheduledJobsCard } from './ScheduledJobsCard'
import { WorkerHealthCard } from './WorkerHealthCard'
export { GeneralSettings } from './GeneralSettings'
import { SchedulesView } from '../schedules/SchedulesView'
import {
  useLanguage,
  type LanguagePreference,
} from '../../context/LanguageContext'
import { useEffect } from 'react'
import { Link, useLocation, useNavigate } from 'react-router-dom'
import type { Project } from '../../api/types'
import { useTheme } from '../../context/ThemeContext'
import { useAuth } from '../../context/AuthContext'
import { AppsSkillsView } from './AppsSkillsView'
import { AIProvidersView } from './AIProvidersView'
import { TeamView } from '../team/TeamView'
import { AgentMemoryView } from './AgentMemoryView'
import { SettingsMasterNav } from './SettingsMasterNav'
import { GroupedSection, GroupedRow } from '../../components/layout/GroupedSection'
import { Brain, ChevronLeft, Command, FolderGit2 } from 'lucide-react'

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
  const { isOwner } = useAuth()
  const location = useLocation()
  const navigate = useNavigate()
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
          ['ai-providers', t('AI Providers')],
          ['memory', t('Agent Memory')],
          ['schedules', t('Schedules')],
          ['profile', t('Profile & password')],
          ['contacts', t('Contacts')],
          ['agents', t('Agents')],
          ['apps-skills', t('Apps & Skills')],
          ['integrations', t('Integrations')],
          ['appearance', t('Appearance')],
          ['notifications', t('Notifications')],
        ]
          .filter(([id]) => (id === 'ai-providers' ? isOwner : true))
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

      {/* Master Sidebar (Kiri) - w-72 / w-80 di desktop */}
      <aside
        className={`w-full md:w-72 lg:w-80 shrink-0 border-b md:border-b-0 md:border-r border-border/40 bg-card/25 ${
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
        className={`flex-1 min-w-0 overflow-y-auto p-4 md:p-6 ${
          !hasExplicitSection ? 'hidden md:block' : 'block'
        }`}
      >
        <div className="max-w-4xl mx-auto space-y-6">
          {/* Mobile Back Button to return to categorized master menu */}
          <div className="md:hidden">
            <Link
              to={
                location.search
                  ? `${isAccount ? '/account' : '/settings'}${location.search}`
                  : isAccount
                    ? '/account'
                    : '/settings'
              }
              className="inline-flex min-h-11 items-center gap-1.5 rounded-md text-sm font-medium text-muted-foreground hover:text-foreground transition-colors focus-visible:outline focus-visible:outline-2 focus-visible:outline-ring"
            >
              <ChevronLeft className="h-4 w-4 shrink-0" />
              <span>
                {isAccount ? t('← Back to Account') : t('← Back to Settings')}
              </span>
            </Link>
          </div>

          {!(section === 'integrations' && location.pathname.split('/')[3]) && (
            <header className="hidden sm:block pb-2 border-b border-border/40">
              <h2 className="text-lg font-semibold capitalize text-foreground">
                {t(
                  section === 'general'
                    ? 'General'
                    : section === 'ai-providers'
                      ? 'AI Providers'
                      : section === 'memory'
                        ? 'Agent Memory'
                      : section === 'schedules'
                        ? 'Schedules'
                        : section === 'profile'
                          ? 'Profile & password'
                          : section === 'agents'
                            ? 'Agents'
                            : section === 'apps-skills'
                              ? 'Apps & Skills'
                              : section === 'integrations'
                                ? 'Integrations'
                                : section === 'appearance'
                                ? 'Appearance'
                                : section === 'notifications'
                                  ? 'Notifications'
                                  : section,
                )}
              </h2>
              <p className="mt-1 text-xs sm:text-sm text-muted-foreground">
                {t(
                  section === 'ai-providers'
                    ? 'Pilih dan atur penyedia AI ruang kerja Anda, langganan resmi, dan kunci API hemat.'
                    : section === 'apps-skills'
                    ? 'Connect 1-click external business apps and discover modular AI intelligence capabilities.'
                    : section === 'integrations'
                      ? 'Connect the services your projects need.'
                      : section === 'memory'
                      ? 'Review learned brand guidelines, copywriting style, and workflow preferences.'
                      : section === 'schedules'
                        ? 'Automated recurring background routines.'
                        : section === 'appearance'
                          ? 'Theme modes, interface language, and display.'
                          : section === 'notifications'
                            ? 'Notification preferences and alerts'
                            : 'Manage your workspace and agent connections.',
                )}
              </p>
            </header>
          )}

          {section === 'profile' ? (
            <ProfileSettings />
          ) : section === 'ai-providers' ? (
            !isOwner ? (
              <div role="alert" className="rounded-xl border border-destructive/30 bg-destructive/10 p-6 text-center space-y-3">
                <h3 className="font-semibold text-destructive">{t('Akses Dibatasi')}</h3>
                <p className="text-xs sm:text-sm text-muted-foreground">
                  {t('Pengaturan penyedia AI hanya dapat diakses oleh Pemilik (Owner) ruang kerja.')}
                </p>
                <div>
                  <Link to={isAccount ? '/account' : '/settings'} className="action-secondary text-xs inline-flex items-center">
                    {t('Kembali ke Pengaturan')}
                  </Link>
                </div>
              </div>
            ) : (
              <AIProvidersView />
            )
          ) : section === 'memory' ? (
            <AgentMemoryView />
          ) : section === 'schedules' ? (
            <SchedulesView embedded />
          ) : section === 'notifications' ? (
            <NotificationSettings />
          ) : section === 'agents' ? (
            <div className="space-y-4">
              {safeReturn && (
                <Link to={safeReturn} className="action-secondary text-xs inline-flex items-center">
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
          ) : section === 'apps-skills' || section === 'integrations' ? (
            <AppsSkillsView />
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
            /* General Settings with Things / Apple Inset Grouped Rows */
            <div className="space-y-6">
              <GroupedSection
                title={t('AI Intelligence')}
                description={t('Review learned brand guidelines, copywriting style, and workflow preferences.')}
              >
                <GroupedRow
                  label={t('Agent Memory / Taste')}
                  description={t(
                    'Review learned brand guidelines, copywriting style, and workflow preferences.',
                  )}
                  icon={<Brain className="h-4 w-4 text-primary" />}
                  control={
                    <Link
                      to="/settings/memory"
                      className="action-secondary text-xs inline-flex items-center gap-1.5"
                    >
                      <span>{t('Manage memory')}</span>
                    </Link>
                  }
                />
              </GroupedSection>

              {/* System Observability & Automation Widgets */}
              <div className="space-y-4">
                <WorkerHealthCard />
                <AppUpdateCard />
                <ScheduledJobsCard />
                <InstallAppCard />
              </div>

              <GroupedSection
                title={t('Productivity & Shortcuts')}
                description={t('Quick navigation and action hotkeys for desktop workflow.')}
              >
                <GroupedRow
                  label={t('Keyboard shortcuts')}
                  description={t(
                    'Quick navigation and action hotkeys for desktop workflow.',
                  )}
                  icon={<Command className="h-4 w-4 text-muted-foreground" />}
                  control={
                    <button
                      type="button"
                      onClick={() =>
                        window.dispatchEvent(new Event('cuan:keyboard-shortcuts'))
                      }
                      className="action-secondary text-xs"
                    >
                      {t('View shortcuts')}
                    </button>
                  }
                />
              </GroupedSection>

              <GroupedSection
                title={t('Project settings')}
                description={t('Select a project to view its repository configuration.')}
              >
                {projects.filter((p) => !p.is_archived).length > 0 ? (
                  projects
                    .filter((p) => !p.is_archived)
                    .map((p) => (
                      <GroupedRow
                        key={p.id}
                        label={p.name}
                        description={p.repo_url || t('Custom project')}
                        icon={<FolderGit2 className="h-4 w-4 text-primary" />}
                        control={
                          <Link
                            to={`/projects/${encodeURIComponent(p.id)}/settings`}
                            className="action-secondary text-xs"
                          >
                            {t('Configure')}
                          </Link>
                        }
                      />
                    ))
                ) : (
                  <GroupedRow
                    label={t('No active projects')}
                    description={t('Open Projects to create or configure a project.')}
                    control={
                      <Link to="/projects" className="action-secondary text-xs">
                        {t('Open Projects')}
                      </Link>
                    }
                  />
                )}
              </GroupedSection>
            </div>
          )}
        </div>
      </main>
    </div>
  )
}
