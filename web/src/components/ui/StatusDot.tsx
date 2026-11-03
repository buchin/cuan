import * as React from "react"
import { cn } from "../../lib/utils"

export type StatusDotVariant =
  | "healthy"
  | "success"
  | "neutral"
  | "idle"
  | "warning"
  | "exception"
  | "error"
  | "danger"
  | "running"
  | "pulse"
  | "working"

export interface StatusDotProps extends React.HTMLAttributes<HTMLSpanElement> {
  status?: StatusDotVariant
  pulse?: boolean
  size?: "xs" | "sm" | "md" | "lg"
  label?: React.ReactNode
  showLabel?: boolean
}

export const StatusDot = React.forwardRef<HTMLSpanElement, StatusDotProps>(
  (
    {
      className,
      status = "neutral",
      pulse = false,
      size = "md",
      label,
      showLabel = false,
      title,
      ...props
    },
    ref
  ) => {
    // Resolve variant styling following the "Status-by-Exception" principle:
    // Restrict high-contrast/screaming colors ONLY for exceptions/errors.
    // Normal states are calm, understated, and non-distracting.
    const isPulse = pulse || status === "running" || status === "pulse" || status === "working"

    const sizeClasses = {
      xs: "h-1.5 w-1.5", // 6px
      sm: "h-1.5 w-1.5", // 6px
      md: "h-2 w-2",     // 8px
      lg: "h-2.5 w-2.5", // 10px
    }[size]

    const colorClasses = {
      healthy: "bg-emerald-500/90 dark:bg-emerald-400",
      success: "bg-emerald-500/90 dark:bg-emerald-400",
      neutral: "bg-muted-foreground/40 dark:bg-muted-foreground/50",
      idle: "bg-muted-foreground/30 dark:bg-muted-foreground/40",
      warning: "bg-amber-500/90 dark:bg-amber-400",
      exception: "bg-rose-500 dark:bg-rose-400",
      error: "bg-rose-500 dark:bg-rose-400",
      danger: "bg-rose-500 dark:bg-rose-400",
      running: "bg-sky-500 dark:bg-sky-400",
      pulse: "bg-sky-500 dark:bg-sky-400",
      working: "bg-blue-500/90 dark:bg-blue-400",
    }[status] || "bg-muted-foreground/40"

    const defaultTitle = typeof label === "string" ? label : status

    const dot = (
      <span
        ref={ref}
        data-status-dot={status}
        title={title || defaultTitle}
        aria-label={props["aria-label"] || (typeof label === "string" ? label : status)}
        className={cn(
          "relative inline-flex shrink-0 rounded-full transition-colors",
          sizeClasses,
          colorClasses,
          isPulse && "animate-pulse",
          className
        )}
        {...props}
      >
        {isPulse && (
          <span
            className={cn(
              "absolute -inset-0.5 rounded-full opacity-60 animate-ping",
              colorClasses
            )}
            aria-hidden="true"
          />
        )}
      </span>
    )

    if (showLabel && label) {
      return (
        <span className="inline-flex items-center gap-1.5 text-xs text-muted-foreground">
          {dot}
          <span className="truncate">{label}</span>
        </span>
      )
    }

    return dot
  }
)

StatusDot.displayName = "StatusDot"
