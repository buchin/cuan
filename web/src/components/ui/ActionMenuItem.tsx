import type { ButtonHTMLAttributes } from 'react'
import { cn } from '../../lib/utils'

export function ActionMenuItem({ className, ...props }: ButtonHTMLAttributes<HTMLButtonElement>) {
  return <button type="button" role="menuitem" className={cn('flex min-h-11 w-full items-center gap-2 rounded-md px-3 text-left text-sm text-foreground hover:bg-muted/50 focus:bg-muted/50 focus:outline-none disabled:opacity-50', className)} {...props} />
}
