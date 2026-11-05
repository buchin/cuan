import type { ReactNode } from 'react'
import { cn } from '../../lib/utils'

export interface SurfaceHeaderProps {
  title?: ReactNode
  description?: ReactNode
  badge?: ReactNode
  leading?: ReactNode
  actions?: ReactNode
  variant?: 'panel' | 'card'
  size?: 'compact' | 'normal'
  className?: string
}

export function SurfaceHeader({ title, description, badge, leading, actions, variant = 'panel', size = variant === 'card' ? 'compact' : 'normal', className }: SurfaceHeaderProps) {
  const Title = variant === 'panel' ? 'h2' : 'div'
  return <header className={cn('flex shrink-0 items-center justify-between gap-3', size === 'compact' ? 'px-4 py-2.5' : 'px-4 py-3 sm:px-6', variant === 'panel' ? 'border-b border-border/40' : 'text-xs text-muted-foreground', className)}>
    <div className="flex min-w-0 flex-1 items-center gap-2">
      {leading && <div className="shrink-0">{leading}</div>}
      <div className="min-w-0 flex-1">
        <div className="flex min-w-0 items-center gap-2">
          {title && <Title className={cn('min-w-0 truncate', variant === 'panel' ? 'text-base font-semibold leading-none tracking-tight text-foreground' : 'text-xs font-normal')}>{title}</Title>}
          {badge && <div className="shrink-0">{badge}</div>}
        </div>
        {description && <p className="mt-1 truncate text-xs text-muted-foreground">{description}</p>}
      </div>
    </div>
    {actions && <div className="flex shrink-0 items-center gap-1">{actions}</div>}
  </header>
}
