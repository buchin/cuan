import type { Project } from '../../api/types'
import { GroupedSection, GroupedRow } from '../../components/layout/GroupedSection'
import { Command } from 'lucide-react'
import { useLanguage } from '../../context/LanguageContext'
import { WorkerHealthCard } from './WorkerHealthCard'
import { AppUpdateCard } from './AppUpdateCard'
import { InstallAppCard } from './InstallAppCard'

export interface GeneralSettingsProps {
  projects?: Project[]
}

export function GeneralSettings({ projects: _projects = [] }: GeneralSettingsProps) {
  const { t } = useLanguage()

  return (
    <div className="space-y-6" data-testid="general-settings">
      {/* System Observability & Automation Widgets */}
      <div className="space-y-4">
        <WorkerHealthCard />
        <AppUpdateCard />
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


    </div>
  )
}
