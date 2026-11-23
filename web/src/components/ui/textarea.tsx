import * as React from 'react'
import { cn } from '../../lib/utils'

export interface TextareaProps
  extends React.TextareaHTMLAttributes<HTMLTextAreaElement> {
  mono?: boolean
  error?: boolean
}

export const formTextareaBaseClass =
  'w-full rounded-xl border border-border/60 bg-background/80 p-3.5 text-sm leading-relaxed text-foreground placeholder:text-muted-foreground/60 focus:outline-none focus:ring-1 focus:ring-ring transition-colors resize-y disabled:cursor-not-allowed disabled:opacity-50'

const Textarea = React.forwardRef<HTMLTextAreaElement, TextareaProps>(
  ({ className, mono, error, ...props }, ref) => {
    const isError = Boolean(error || props['aria-invalid'] === true || props['aria-invalid'] === 'true')

    return (
      <textarea
        className={cn(
          formTextareaBaseClass,
          mono && 'font-mono text-xs',
          isError && 'border-destructive/80 focus:border-destructive focus:ring-destructive/30',
          className,
        )}
        ref={ref}
        aria-invalid={isError ? 'true' : undefined}
        {...props}
      />
    )
  },
)
Textarea.displayName = 'Textarea'

export { Textarea }
