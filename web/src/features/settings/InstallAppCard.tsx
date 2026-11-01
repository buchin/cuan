import { useState } from 'react'
import { Download, Check } from 'lucide-react'
import { usePWA } from '../../context/PWAContext'
import { useLanguage } from '../../context/LanguageContext'
import { GroupedSection } from '../../components/layout/GroupedSection'

export function InstallAppCard() {
  const { installed, canInstall, install, error } = usePWA(),
    { t } = useLanguage()
  const [busy, setBusy] = useState(false)
  const ios =
    /iPad|iPhone|iPod/.test(navigator.userAgent) ||
    (navigator.platform === 'MacIntel' && navigator.maxTouchPoints > 1)
  const android = /Android/.test(navigator.userAgent)
  const safari =
    /Safari/.test(navigator.userAgent) &&
    !/Chrome|Chromium|Edg/.test(navigator.userAgent)
  const instructions = ios
    ? [
        'Open Cuan in Safari.',
        'Tap Share, then Add to Home Screen.',
        'Tap Add to finish.',
      ]
    : android
      ? [
          'Open your browser menu.',
          'Choose Install app or Add to Home Screen, when available.',
          'Confirm to add Cuan.',
        ]
      : safari
        ? [
            'Open the File menu in Safari.',
            'Choose Add to Dock, when available.',
            'Confirm to add Cuan.',
          ]
        : [
            'Open your browser menu.',
            'Choose Install Cuan or Install app, when available.',
            'Confirm to add Cuan.',
          ]
  return (
    <GroupedSection
      id="install-cuan"
      aria-labelledby="install-cuan-title"
      title={t('Install Cuan')}
      description={t('Open Cuan from your home screen in its own window. An internet connection is required.')}
      className="scroll-mt-5"
    >
      <div className="p-4 sm:p-5 space-y-3">
        <h2 id="install-cuan-title" className="sr-only">
          {t('Install Cuan')}
        </h2>
        {installed ? (
          <p role="status" className="flex items-center gap-2 text-sm text-emerald-500 font-medium">
            <Check className="h-4 w-4" />
            {t('Cuan is installed.')}
          </p>
        ) : canInstall ? (
          <button
            type="button"
            className="action-primary"
            disabled={busy}
            onClick={async () => {
              setBusy(true)
              try {
                await install()
              } finally {
                setBusy(false)
              }
            }}
          >
            <Download className="h-4 w-4" />
            {t('Install Cuan')}
          </button>
        ) : (
          <div className="space-y-2 text-sm">
            <p className="font-medium text-foreground">{t('Add Cuan from your browser')}</p>
            <ol className="list-decimal space-y-1 pl-5 text-muted-foreground text-xs sm:text-sm">
              {instructions.map((step) => (
                <li key={step}>{t(step)}</li>
              ))}
            </ol>
            <p className="text-xs text-muted-foreground">
              {t(
                'If the install option is missing, try a supported browser or check whether Cuan is already installed.',
              )}
            </p>
          </div>
        )}
        {error && (
          <p role="alert" className="text-sm text-destructive">
            {t(error)}
          </p>
        )}
      </div>
    </GroupedSection>
  )
}
