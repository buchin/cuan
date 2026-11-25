import { RefreshCw, WifiOff } from 'lucide-react'

interface TerminalPopoutBannerProps {
  onReturnToMain: () => void
  t: (key: string) => string
}

/**
 * Calm status banner shown in the main window while a terminal session is popped out.
 */
export function TerminalPopoutBanner({ onReturnToMain, t }: TerminalPopoutBannerProps) {
  return (
    <div className="flex shrink-0 items-center justify-between px-3 py-1.5 bg-muted/40 border-b border-border/50 text-xs text-muted-foreground select-none">
      <div className="flex items-center gap-2 min-w-0">
        <span className="h-2 w-2 rounded-full bg-emerald-500 animate-pulse task-pulse-subtle shrink-0" />
        <span className="truncate">{t('Terminal aktif di jendela terpisah')}</span>
      </div>
      <button
        type="button"
        className="text-primary hover:underline text-xs shrink-0 ml-2 font-medium cursor-pointer"
        onClick={onReturnToMain}
      >
        {t('Tampilkan di sini')}
      </button>
    </div>
  )
}

interface TerminalDisconnectBannerProps {
  connection: string
  language: string
  onReconnect: () => void
  t: (key: string) => string
}

/**
 * Reconnect banner shown when an inactive/saved terminal loses its socket connection.
 */
export function TerminalDisconnectBanner({
  connection,
  language,
  onReconnect,
  t,
}: TerminalDisconnectBannerProps) {
  const isConnecting = connection === 'connecting'
  const reconnectLabel = language === 'id' ? 'Sambungkan Ulang' : t('Reconnect')

  return (
    <div
      role="status"
      className="flex shrink-0 flex-wrap items-center justify-between gap-2 border-b border-amber-500/30 bg-amber-500/10 px-3 py-2 text-xs text-amber-200"
    >
      <div className="flex min-w-0 items-center gap-2">
        <WifiOff className="h-4 w-4 shrink-0 text-amber-400" />
        <div className="min-w-0 flex-1">
          <p className="font-medium text-amber-200">
            {isConnecting
              ? t('Reconnecting to terminal…')
              : t('Terminal disconnected. The task is still running in the background.')}
          </p>
          <p className="text-[11px] text-amber-300/80">
            {isConnecting
              ? t('Connecting to active session…')
              : t('Auto-retry when tab becomes active')}
          </p>
        </div>
      </div>
      <button
        type="button"
        disabled={isConnecting}
        onClick={onReconnect}
        className="inline-flex shrink-0 items-center gap-1.5 rounded-md bg-amber-500 px-3 py-1.5 text-xs font-semibold text-black transition-colors hover:bg-amber-400 disabled:opacity-50"
        aria-label={reconnectLabel}
      >
        <RefreshCw className={`h-3.5 w-3.5 ${isConnecting ? 'animate-spin' : ''}`} />
        {isConnecting ? t('Connecting…') : reconnectLabel}
      </button>
    </div>
  )
}
