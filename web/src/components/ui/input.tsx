import * as React from 'react'
import { cn } from '../../lib/utils'

export interface InputProps
  extends React.InputHTMLAttributes<HTMLInputElement> {
  mono?: boolean
  error?: boolean
  sizeVariant?: 'default' | 'sm'
  prefixIcon?: React.ReactNode
  suffixIcon?: React.ReactNode
}

export const formControlBaseClass =
  'h-11 min-h-11 w-full rounded-xl border border-border/60 bg-background/80 px-3.5 text-sm text-foreground placeholder:text-muted-foreground/60 focus:outline-none focus:ring-1 focus:ring-ring transition-colors disabled:cursor-not-allowed disabled:opacity-50'

export const formControlSmClass =
  'h-9 min-h-9 w-full rounded-lg border border-border/60 bg-background/80 px-3 text-xs text-foreground placeholder:text-muted-foreground/60 focus:outline-none focus:ring-1 focus:ring-ring transition-colors disabled:cursor-not-allowed disabled:opacity-50'

const Input = React.forwardRef<HTMLInputElement, InputProps>(
  (
    {
      className,
      type = 'text',
      mono,
      error,
      sizeVariant = 'default',
      prefixIcon,
      suffixIcon,
      ...props
    },
    ref,
  ) => {
    const isError = Boolean(error || props['aria-invalid'] === true || props['aria-invalid'] === 'true')
    const base = sizeVariant === 'sm' ? formControlSmClass : formControlBaseClass

    const inputElement = (
      <input
        type={type}
        className={cn(
          base,
          mono && 'font-mono',
          isError && 'border-destructive/80 focus:border-destructive focus:ring-destructive/30',
          prefixIcon && (sizeVariant === 'sm' ? 'pl-8' : 'pl-10'),
          suffixIcon && (sizeVariant === 'sm' ? 'pr-8' : 'pr-10'),
          className,
        )}
        ref={ref}
        aria-invalid={isError ? 'true' : undefined}
        {...props}
      />
    )

    if (!prefixIcon && !suffixIcon) {
      return inputElement
    }

    return (
      <div className="relative flex items-center w-full">
        {prefixIcon && (
          <div className="absolute left-3 flex items-center pointer-events-none text-muted-foreground/70">
            {prefixIcon}
          </div>
        )}
        {inputElement}
        {suffixIcon && (
          <div className="absolute right-3 flex items-center text-muted-foreground/70">
            {suffixIcon}
          </div>
        )}
      </div>
    )
  },
)
Input.displayName = 'Input'

export { Input }
