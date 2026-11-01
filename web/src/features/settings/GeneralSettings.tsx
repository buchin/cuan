import type { Project } from '../../api/types'
import { GroupedSection, GroupedRow } from '../../components/layout/GroupedSection'
import { Brain, Command, FolderGit2 } from 'lucide-react'
import { Link } from 'react-router-dom'
import { useLanguage } from '../../context/LanguageContext'
import { WorkerHealthCard } from './WorkerHealthCard'
import { AppUpdateCard } from './AppUpdateCard'
import { ScheduledJobsCard } from './ScheduledJobsCard'
import { InstallAppCard } from './InstallAppCard'

export interface GeneralSettingsProps {
  projects?: Project[]
}

export function GeneralSettings({ projects = [] }: GeneralSettingsProps) {
  const { t } = useLanguage()

  return (
    <div className="space-y-6" data-testid="general-settings">
      {/* AI Intelligence */}
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

      {/* Productivity & Shortcuts */}
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

      {/* Project settings */}
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
  )
}
