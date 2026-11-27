import * as React from "react"
import { cn } from "../../lib/utils"

export interface BadgeProps extends React.HTMLAttributes<HTMLDivElement> {
  variant?: "default" | "secondary" | "destructive" | "outline" | "success" | "warning" | "info" | "neutral"
  size?: "default" | "sm"
}

function Badge({ className, variant = "default", size = "default", ...props }: BadgeProps) {
  const baseStyles = "inline-flex items-center font-medium transition-colors rounded-full border-0"
  const sizeStyles = {
    default: "px-2.5 py-0.5 text-xs gap-1",
    sm: "px-1.5 py-0.2 text-[10px] gap-1",
  }
  const variantStyles = {
    default: "bg-primary/10 text-primary",
    secondary: "bg-secondary text-secondary-foreground",
    destructive: "bg-destructive/10 text-destructive dark:text-red-400",
    outline: "bg-muted/60 text-muted-foreground",
    neutral: "bg-muted/80 text-muted-foreground",
    success: "bg-emerald-500/10 text-emerald-700 dark:text-emerald-400",
    warning: "bg-amber-500/10 text-amber-800 dark:text-amber-300",
    info: "bg-sky-500/10 text-sky-700 dark:text-sky-300",
  }

  return (
    <div data-badge="true" className={cn(baseStyles, sizeStyles[size], variantStyles[variant], className)} {...props} />
  )
}

export { Badge }

