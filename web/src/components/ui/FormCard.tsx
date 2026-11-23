import * as React from 'react'
import { cn } from '../../lib/utils'

export interface FormCardProps {
  title?: React.ReactNode
  icon?: React.ReactNode
  accessory?: React.ReactNode
  children: React.ReactNode
  className?: string
}

export function FormCard({
  title,
  icon,
  accessory,
  children,
  className,
}: FormCardProps) {
  return (
    <div
      className={cn(
        'rounded-xl border border-border/40 bg-muted/20 p-3.5 sm:p-4 space-y-3.5',
        className,
      )}
    >
      {(title || accessory) && (
        <div className="flex items-center justify-between pb-0.5">
          <span className="text-xs font-semibold uppercase tracking-wider text-muted-foreground inline-flex items-center gap-1.5">
            {icon}
            {title}
          </span>
          {accessory && (
            <span className="text-[11px] text-muted-foreground/80 font-mono">
              {accessory}
            </span>
          )}
        </div>
      )}
      {children}
    </div>
  )
}
