import type { ComponentProps } from 'react'
import { cn } from '../../lib/utils'

export function CardActions({ className, ...props }: ComponentProps<'footer'>) {
  return <footer {...props} data-card-actions className={cn('flex flex-wrap items-center justify-end gap-4 border-t border-border/30', className)} />
}
