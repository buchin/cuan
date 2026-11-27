import type { ButtonHTMLAttributes } from 'react'
import { cn } from '../../lib/utils'

export function ActionMenuItem({ className, destructive = false, ...props }: ButtonHTMLAttributes<HTMLButtonElement> & { destructive?: boolean }) {
  return (
    <button
      type="button"
      role="menuitem"
      className={cn(
        'flex min-h-9 max-sm:min-h-10 w-full items-center gap-2 rounded-lg px-2.5 py-1.5 text-left text-sm transition-colors focus:outline-none disabled:opacity-50',
        destructive
          ? 'text-destructive hover:bg-destructive/10 focus:bg-destructive/10'
          : 'text-foreground hover:bg-muted/70 focus:bg-muted/70',
        className,
      )}
      {...props}
    />
  )
}
