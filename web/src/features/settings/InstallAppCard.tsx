import { Download, Check } from 'lucide-react'
import { usePWA } from '../../context/PWAContext'
import { useLanguage } from '../../context/LanguageContext'
export function InstallAppCard() {
  const { installed, canInstall, install, error } = usePWA(),
    { t } = useLanguage()
  const ios =
    /iPad|iPhone|iPod/.test(navigator.userAgent) ||
    (navigator.platform === 'MacIntel' && navigator.maxTouchPoints > 1)
  return (
    <section className="space-y-3 rounded-lg border border-border bg-card p-5">
      <h2 className="text-lg font-semibold">{t('Install Cuan')}</h2>
      <p className="text-sm text-muted-foreground">
        {t(
          'Open Cuan from your home screen in its own window. An internet connection is required.',
        )}
      </p>
      {installed ? (
        <p role="status" className="flex items-center gap-2 text-sm">
          <Check className="h-4 w-4" />
          {t('Cuan is installed.')}
        </p>
      ) : canInstall ? (
        <button className="action-primary" onClick={() => void install()}>
          <Download className="h-4 w-4" />
          {t('Install app')}
        </button>
      ) : (
        <p className="text-sm">
          {t(
            ios
              ? 'In Safari, open Share → Add to Home Screen → Add.'
              : 'Open your browser menu and choose Install app or Add to Home Screen, when available.',
          )}
        </p>
      )}
      {error && (
        <p role="alert" className="text-sm text-destructive">
          {t(error)}
        </p>
      )}
    </section>
  )
}
