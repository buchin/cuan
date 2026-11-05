import type { ComponentProps } from 'react'
import { cn } from '../../lib/utils'
import './card-actions.css'

export function CardActions({ className, ...props }: ComponentProps<'footer'>) {
  return <footer {...props} data-card-actions className={cn('flex flex-wrap items-center justify-end gap-2 border-t border-border/50', className)} />
}
