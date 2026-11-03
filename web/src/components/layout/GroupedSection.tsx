import * as React from "react"
import { cn } from "../../lib/utils"

export interface GroupedSectionProps
  extends Omit<React.HTMLAttributes<HTMLDivElement>, "title"> {
  title?: React.ReactNode
  description?: React.ReactNode
  footerNote?: React.ReactNode
  inset?: boolean
  containerClassName?: string
}

export const GroupedSection = React.forwardRef<HTMLDivElement, GroupedSectionProps>(
  (
    {
      className,
      title,
      description,
      footerNote,
      inset = true,
      containerClassName,
      children,
      ...props
    },
    ref
  ) => {
    return (
      <section
        ref={ref}
        className={cn("w-full mb-6 last:mb-0", className)}
        {...props}
      >
        {(title || description) && (
          <div className="px-3 mb-2">
            {title && (
              <h3 className="text-xs font-semibold text-muted-foreground uppercase tracking-wider">
                {title}
              </h3>
            )}
            {description && (
              <p className="text-xs text-muted-foreground mt-0.5">
                {description}
              </p>
            )}
          </div>
        )}

        <div
          className={cn(
            "w-full divide-y divide-border/30 bg-card/50 transition-colors",
            inset && "rounded-xl border border-border/40 overflow-hidden shadow-2xs",
            containerClassName
          )}
        >
          {children}
        </div>

        {footerNote && (
          <div className="px-3 mt-1.5 text-xs text-muted-foreground">
            {footerNote}
          </div>
        )}
      </section>
    )
  }
)

GroupedSection.displayName = "GroupedSection"

export const InsetGroup = GroupedSection

export interface GroupedRowProps extends React.HTMLAttributes<HTMLDivElement> {
  label: React.ReactNode
  description?: React.ReactNode
  icon?: React.ReactNode
  control?: React.ReactNode
  onClick?: React.MouseEventHandler<HTMLDivElement>
  destructive?: boolean
}

export const GroupedRow = React.forwardRef<HTMLDivElement, GroupedRowProps>(
  (
    {
      className,
      label,
      description,
      icon,
      control,
      onClick,
      destructive = false,
      children,
      ...props
    },
    ref
  ) => {
    const isInteractive = Boolean(onClick)

    return (
      <div
        ref={ref}
        onClick={onClick}
        role={isInteractive ? "button" : undefined}
        tabIndex={isInteractive ? 0 : undefined}
        onKeyDown={
          isInteractive
            ? (e) => {
                if (e.key === "Enter" || e.key === " ") {
                  e.preventDefault()
                  onClick?.(e as any)
                }
              }
            : undefined
        }
        className={cn(
          "flex flex-col items-stretch justify-between gap-3 px-3.5 sm:flex-row sm:items-center py-3 sm:px-4 sm:py-3.5 min-h-[44px] text-sm transition-colors",
          isInteractive &&
            "cursor-pointer hover:bg-muted/40 active:bg-muted/60 focus-visible:outline-none focus-visible:bg-muted/40",
          destructive && "text-destructive",
          className
        )}
        {...props}
      >
        {children ? (
          children
        ) : (
          <>
            <div className="flex items-center gap-3 min-w-0 flex-1">
              {icon && (
                <div
                  className={cn(
                    "flex shrink-0 items-center justify-center text-muted-foreground",
                    destructive && "text-destructive"
                  )}
                >
                  {icon}
                </div>
              )}
              <div className="min-w-0 flex-1">
                <div
                  className={cn(
                    "text-sm font-medium leading-snug text-foreground",
                    destructive && "text-destructive"
                  )}
                >
                  {label}
                </div>
                {description && (
                  <p className="text-xs leading-5 text-muted-foreground mt-1">
                    {description}
                  </p>
                )}
              </div>
            </div>

            {control && (
              <div className="flex w-full max-w-full flex-wrap shrink-0 items-center gap-2 sm:w-auto">
                {control}
              </div>
            )}
          </>
        )}
      </div>
    )
  }
)

GroupedRow.displayName = "GroupedRow"

export const InsetRow = GroupedRow
