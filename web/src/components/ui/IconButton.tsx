import { forwardRef, type ButtonHTMLAttributes } from 'react'
import { cn } from '../../lib/utils'

export interface IconButtonProps extends ButtonHTMLAttributes<HTMLButtonElement> {
  label: string
  active?: boolean
}

// Every icon action has the same visual size and a 44px touch target.
export const IconButton = forwardRef<HTMLButtonElement, IconButtonProps>(
  ({ label, active, className, children, type = 'button', ...props }, ref) => (
    <button ref={ref} type={type} aria-label={label} title={label}
      className={cn('inline-flex h-11 w-11 min-h-11 min-w-11 shrink-0 items-center justify-center rounded-md text-muted-foreground transition-colors hover:bg-muted/50 hover:text-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring disabled:pointer-events-none disabled:opacity-50 [&_svg]:h-[18px] [&_svg]:w-[18px]', active && 'bg-muted/50 text-foreground', className)}
      {...props}>{children}</button>
  ),
)
IconButton.displayName = 'IconButton'
