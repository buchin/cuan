import * as React from "react"
import { cn } from "../../lib/utils"

export interface BadgeProps extends React.HTMLAttributes<HTMLDivElement> {
  variant?: "default" | "secondary" | "destructive" | "outline" | "success" | "warning" | "info" | "neutral"
  size?: "default" | "sm"
}

function Badge({ className, variant = "default", size = "default", ...props }: BadgeProps) {
  const baseStyles = "inline-flex items-center font-medium transition-colors rounded-md border"
  const sizeStyles = {
    default: "px-2.5 py-0.5 text-xs gap-1",
    sm: "px-1.5 py-0.2 text-[10px] gap-1",
  }
  const variantStyles = {
    default: "border-primary/20 bg-primary/10 text-primary",
    secondary: "border-transparent bg-secondary text-secondary-foreground",
    destructive: "border-destructive/25 bg-destructive/10 text-destructive dark:text-red-400",
    outline: "border-border/60 text-muted-foreground bg-transparent",
    neutral: "border-border/40 bg-muted/80 text-muted-foreground",
    success: "border-emerald-500/25 bg-emerald-500/10 text-emerald-700 dark:text-emerald-400",
    warning: "border-amber-500/25 bg-amber-500/10 text-amber-800 dark:text-amber-300",
    info: "border-sky-500/25 bg-sky-500/10 text-sky-700 dark:text-sky-300",
  }

  return (
    <div className={cn(baseStyles, sizeStyles[size], variantStyles[variant], className)} {...props} />
  )
}

export { Badge }

