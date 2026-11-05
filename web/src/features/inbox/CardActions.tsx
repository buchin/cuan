import type { ComponentProps } from 'react'
import { cn } from '../../lib/utils'

export function CardActions({ className, ...props }: ComponentProps<'footer'>) {
  return <footer {...props} data-card-actions className={cn('flex flex-wrap items-center justify-end gap-2 py-1 border-t border-border/50 [&>button]:rounded-md [&>button]:bg-muted/50 [&>button]:px-3 [&>button]:hover:bg-muted [&>a]:rounded-md [&>a]:bg-muted/50 [&>a]:px-3 [&>a]:hover:bg-muted', className)} />
}
