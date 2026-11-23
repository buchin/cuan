import * as React from 'react'
import { cn } from '../../lib/utils'

export interface FormGridProps {
  cols?: 1 | 2 | 3 | 4
  gap?: 'sm' | 'default' | 'lg'
  className?: string
  children: React.ReactNode
}

export function FormGrid({
  cols = 2,
  gap = 'default',
  className,
  children,
}: FormGridProps) {
  const gapClasses = {
    sm: 'gap-2',
    default: 'gap-3',
    lg: 'gap-4',
  }

  const colClasses = {
    1: 'grid-cols-1',
    2: 'grid-cols-1 sm:grid-cols-2',
    3: 'grid-cols-1 sm:grid-cols-3',
    4: 'grid-cols-1 sm:grid-cols-2 lg:grid-cols-4',
  }

  return (
    <div className={cn('grid', colClasses[cols], gapClasses[gap], className)}>
      {children}
    </div>
  )
}
