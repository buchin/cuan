import * as React from 'react'
import { cn } from '../../lib/utils'

export interface InputProps
  extends React.InputHTMLAttributes<HTMLInputElement> {
  mono?: boolean
}

export const formControlBaseClass =
  'h-11 min-h-11 w-full rounded-xl border border-border/60 bg-background/80 px-3.5 text-sm text-foreground placeholder:text-muted-foreground/60 focus:outline-none focus:ring-1 focus:ring-ring transition-colors disabled:cursor-not-allowed disabled:opacity-50'

const Input = React.forwardRef<HTMLInputElement, InputProps>(
  ({ className, type = 'text', mono, ...props }, ref) => {
    return (
      <input
        type={type}
        className={cn(
          formControlBaseClass,
          mono && 'font-mono',
          className,
        )}
        ref={ref}
        {...props}
      />
    )
  },
)
Input.displayName = 'Input'

export { Input }
