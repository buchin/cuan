import * as React from "react"
import { cn } from "../../lib/utils"

export interface HairlineListProps
  extends React.HTMLAttributes<HTMLElement> {
  as?: "div" | "ul" | "ol"
  inset?: boolean
  dividerClassName?: string
}

export const HairlineList = React.forwardRef<HTMLElement, HairlineListProps>(
  (
    {
      as: Comp = "div",
      className,
      inset = false,
      dividerClassName,
      children,
      ...props
    },
    ref
  ) => {
    return React.createElement(
      Comp,
      {
        ref,
        role: Comp === "ul" || Comp === "ol" ? undefined : "list",
        className: cn(
          "w-full divide-y divide-border/40",
          inset && "px-1 sm:px-2",
          dividerClassName,
          className
        ),
        ...props,
      },
      children
    )
  }
)

HairlineList.displayName = "HairlineList"

export interface HairlineListItemProps
  extends Omit<React.HTMLAttributes<HTMLElement>, "title"> {
  as?: "div" | "li"
  leading?: React.ReactNode
  title?: React.ReactNode
  description?: React.ReactNode
  trailing?: React.ReactNode
  interactive?: boolean
  selected?: boolean
  onClick?: React.MouseEventHandler<HTMLElement>
}

export const HairlineListItem = React.forwardRef<
  HTMLElement,
  HairlineListItemProps
>(
  (
    {
      as: Comp = "div",
      className,
      leading,
      title,
      description,
      trailing,
      interactive = false,
      selected = false,
      onClick,
      children,
      ...props
    },
    ref
  ) => {
    const isClickable = interactive || Boolean(onClick)

    const handleKeyDown = isClickable
      ? (e: React.KeyboardEvent<HTMLElement>) => {
          if (e.key === "Enter" || e.key === " ") {
            e.preventDefault()
            onClick?.(e as any)
          }
        }
      : undefined

    const content = children ? (
      children
    ) : (
      <>
        {leading && (
          <div className="flex shrink-0 items-center text-muted-foreground group-hover:text-foreground transition-colors">
            {leading}
          </div>
        )}
        <div className="min-w-0 flex-1">
          {title && (
            <div className="truncate font-medium text-foreground tracking-tight">
              {title}
            </div>
          )}
          {description && (
            <div className="truncate text-xs text-muted-foreground mt-0.5">
              {description}
            </div>
          )}
        </div>
        {trailing && (
          <div className="flex shrink-0 items-center text-xs text-muted-foreground gap-2">
            {trailing}
          </div>
        )}
      </>
    )

    return React.createElement(
      Comp,
      {
        ref,
        role: Comp === "li" ? undefined : "listitem",
        tabIndex: isClickable ? 0 : undefined,
        onClick,
        onKeyDown: handleKeyDown,
        className: cn(
          "group flex items-center gap-3 py-3 px-3 sm:px-4 min-h-[44px] text-sm transition-colors outline-none",
          isClickable &&
            "cursor-pointer hover:bg-muted/30 focus-visible:bg-muted/40 focus-visible:ring-1 focus-visible:ring-ring active:bg-muted/50",
          selected && "bg-muted/40 font-medium",
          className
        ),
        ...props,
      },
      content
    )
  }
)

HairlineListItem.displayName = "HairlineListItem"
