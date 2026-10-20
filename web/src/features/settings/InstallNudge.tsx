import { useState } from 'react'
import { Download, X } from 'lucide-react'
import { usePWA } from '../../context/PWAContext'
import { useLanguage } from '../../context/LanguageContext'
import { ActionDialog } from '../../components/dialogs/ActionDialog'

export function InstallNudge() {
  const { installed, canInstall, install, error } = usePWA(),
    { t } = useLanguage()
  const [dismissed, setDismissed] = useState(() => {
      try {
        return (
          Number(localStorage.getItem('cuan-install-dismissed-until') || 0) >
          Date.now()
        )
      } catch {
        return false
      }
    }),
    [instructions, setInstructions] = useState(false)
  const ios =
    /iPad|iPhone|iPod/.test(navigator.userAgent) ||
    (navigator.platform === 'MacIntel' && navigator.maxTouchPoints > 1)
  if (installed || dismissed || (!canInstall && !ios)) return null
  return (
    <aside
      aria-label={t('Install Cuan')}
      className="mt-6 rounded-xl border border-border p-3"
    >
      <div className="flex items-start gap-2">
        <Download className="mt-3 h-4 w-4 shrink-0 text-muted-foreground" />
        <div className="min-w-0 flex-1 pt-2">
          <p className="text-sm font-medium">{t('Keep Cuan one tap away.')}</p>
          <p className="mt-1 text-xs leading-relaxed text-muted-foreground">
            {t('Add Cuan to your home screen for quick access.')}
          </p>
        </div>
        <button
          type="button"
          className="icon-action shrink-0"
          aria-label={t('Dismiss install suggestion')}
          onClick={() => {
            try {
              localStorage.setItem(
                'cuan-install-dismissed-until',
                String(Date.now() + 7 * 24 * 60 * 60 * 1000),
              )
            } catch {
              /* Dismiss for this visit when storage is unavailable. */
            }
            setDismissed(true)
          }}
        >
          <X className="h-4 w-4" />
        </button>
      </div>
      <button
        type="button"
        className="action-secondary mt-3 w-full sm:w-auto"
        onClick={() => {
          if (canInstall) void install()
          else setInstructions(true)
        }}
      >
        {t('Install Cuan')}
        <Download className="h-4 w-4" />
      </button>
      {error && (
        <p role="alert" className="mt-2 text-xs text-destructive">
          {t(error)}
        </p>
      )}
      <ActionDialog
        open={instructions}
        title={t('Install Cuan')}
        onClose={() => setInstructions(false)}
      >
        <p className="text-sm leading-relaxed">
          {t('In Safari, open Share → Add to Home Screen → Add.')}
        </p>
        <button
          type="button"
          className="action-primary mt-5"
          onClick={() => setInstructions(false)}
        >
          {t('Got it')}
        </button>
      </ActionDialog>
    </aside>
  )
}
