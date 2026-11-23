import * as React from 'react'
import { cn } from '../../lib/utils'
import { formControlBaseClass, formControlSmClass } from './input'

export interface SelectProps
  extends React.SelectHTMLAttributes<HTMLSelectElement> {
  error?: boolean
  sizeVariant?: 'default' | 'sm'
}

const Select = React.forwardRef<HTMLSelectElement, SelectProps>(
  ({ className, error, sizeVariant = 'default', children, ...props }, ref) => {
    const isError = Boolean(error || props['aria-invalid'] === true || props['aria-invalid'] === 'true')
    const base = sizeVariant === 'sm' ? formControlSmClass : formControlBaseClass

    return (
      <select
        className={cn(
          base,
          'cursor-pointer',
          isError && 'border-destructive/80 focus:border-destructive focus:ring-destructive/30',
          className,
        )}
        ref={ref}
        aria-invalid={isError ? 'true' : undefined}
        {...props}
      >
        {children}
      </select>
    )
  },
)
Select.displayName = 'Select'

export { Select }
