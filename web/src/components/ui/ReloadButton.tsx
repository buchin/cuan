import { useState, useCallback } from 'react'
import { RotateCw } from 'lucide-react'
import { reloadApp } from '../../lib/reload'
import { useLanguage } from '../../context/LanguageContext'
import { cn } from '../../lib/utils'

export interface ReloadButtonProps {
  className?: string
  iconClassName?: string
  variant?: 'icon' | 'menu-item'
  label?: string
  title?: string
  onClick?: () => void
}

export function ReloadButton({
  className,
  iconClassName,
  variant = 'icon',
  label,
  title,
  onClick,
}: ReloadButtonProps) {
  const { t } = useLanguage()
  const [reloading, setReloading] = useState(false)

  const handleReload = useCallback(async () => {
    if (reloading) return
    setReloading(true)
    onClick?.()
    try {
      await reloadApp()
    } catch {
      setReloading(false)
    }
  }, [reloading, onClick])

  const buttonText = label || t('Reload app')
  const tooltipText = title || label || t('Reload app')

  if (variant === 'menu-item') {
    return (
      <button
        type="button"
        onClick={handleReload}
        disabled={reloading}
        title={tooltipText}
        className={cn(
          'flex min-h-11 w-full items-center gap-2 rounded-md px-2.5 py-2 text-left text-sm text-foreground/90 hover:bg-accent hover:text-accent-foreground disabled:opacity-50 transition-colors',
          className,
        )}
      >
        <RotateCw
          className={cn(
            'h-4 w-4 shrink-0 text-muted-foreground',
            reloading && 'animate-spin text-primary',
            iconClassName,
          )}
        />
        <span>{buttonText}</span>
      </button>
    )
  }

  return (
    <button
      type="button"
      onClick={handleReload}
      disabled={reloading}
      aria-label={tooltipText}
      title={tooltipText}
      className={cn(
        'icon-action disabled:opacity-50 transition-colors',
        className,
      )}
    >
      <RotateCw
        className={cn(
          'h-4 w-4',
          reloading && 'animate-spin text-primary',
          iconClassName,
        )}
      />
    </button>
  )
}
