import * as React from "react"
import { PanelLeft } from "lucide-react"
import { cn } from "../../lib/utils"

const SIDEBAR_WIDTH = "16rem"
const SIDEBAR_WIDTH_MOBILE = "18rem"
const SIDEBAR_WIDTH_ICON = "3.25rem"
const SIDEBAR_KEYBOARD_SHORTCUT = "b"

interface SidebarContextValue {
  state: "expanded" | "collapsed"
  open: boolean
  setOpen: (open: boolean | ((value: boolean) => boolean)) => void
  openMobile: boolean
  setOpenMobile: (open: boolean | ((value: boolean) => boolean)) => void
  isMobile: boolean
  toggleSidebar: () => void
}

const SidebarContext = React.createContext<SidebarContextValue | null>(null)

export function useSidebar() {
  const context = React.useContext(SidebarContext)
  if (!context) {
    throw new Error("useSidebar must be used within a SidebarProvider.")
  }
  return context
}

export interface SidebarProviderProps extends React.ComponentProps<"div"> {
  defaultOpen?: boolean
  open?: boolean
  onOpenChange?: (open: boolean) => void
}

export const SidebarProvider = React.forwardRef<HTMLDivElement, SidebarProviderProps>(
  (
    {
      defaultOpen = true,
      open: openProp,
      onOpenChange: setOpenProp,
      className,
      style,
      children,
      ...props
    },
    ref
  ) => {
    const [isMobile, setIsMobile] = React.useState(false)
    const [openMobile, setOpenMobile] = React.useState(false)

    // Check media query for mobile breakpoint (< 768px)
    React.useEffect(() => {
      const checkMobile = () => {
        setIsMobile(window.innerWidth < 768)
      }
      checkMobile()
      window.addEventListener("resize", checkMobile)
      return () => window.removeEventListener("resize", checkMobile)
    }, [])

    // Internal state when uncontrolled
    const [_open, _setOpen] = React.useState(() => {
      if (typeof window !== "undefined") {
        const saved = localStorage.getItem("sidebar:state")
        if (saved !== null) return saved === "true"
      }
      return defaultOpen
    })

    const open = openProp !== undefined ? openProp : _open
    const setOpen = React.useCallback(
      (value: boolean | ((value: boolean) => boolean)) => {
        const openState = typeof value === "function" ? value(open) : value
        if (setOpenProp) {
          setOpenProp(openState)
        } else {
          _setOpen(openState)
        }
        if (typeof window !== "undefined") {
          localStorage.setItem("sidebar:state", String(openState))
        }
      },
      [setOpenProp, open]
    )

    const toggleSidebar = React.useCallback(() => {
      if (isMobile) {
        setOpenMobile((prev) => !prev)
      } else {
        setOpen((prev) => !prev)
      }
    }, [isMobile, setOpen])

    // Keyboard shortcut (Cmd+B or Ctrl+B)
    React.useEffect(() => {
      const handleKeyDown = (event: KeyboardEvent) => {
        if (
          event.key === SIDEBAR_KEYBOARD_SHORTCUT &&
          (event.metaKey || event.ctrlKey)
        ) {
          event.preventDefault()
          toggleSidebar()
        }
      }

      window.addEventListener("keydown", handleKeyDown)
      return () => window.removeEventListener("keydown", handleKeyDown)
    }, [toggleSidebar])

    const state = open ? "expanded" : "collapsed"

    const contextValue = React.useMemo<SidebarContextValue>(
      () => ({
        state,
        open,
        setOpen,
        isMobile,
        openMobile,
        setOpenMobile,
        toggleSidebar,
      }),
      [state, open, setOpen, isMobile, openMobile, setOpenMobile, toggleSidebar]
    )

    return (
      <SidebarContext.Provider value={contextValue}>
        <div
          style={
            {
              "--sidebar-width": SIDEBAR_WIDTH,
              "--sidebar-width-icon": SIDEBAR_WIDTH_ICON,
              ...style,
            } as React.CSSProperties
          }
          className={cn(
            "group/sidebar-wrapper flex min-h-screen w-full has-[[data-variant=inset]]:bg-background text-foreground",
            className
          )}
          ref={ref}
          {...props}
        >
          {children}
        </div>
      </SidebarContext.Provider>
    )
  }
)
SidebarProvider.displayName = "SidebarProvider"

export interface SidebarProps extends React.ComponentProps<"div"> {
  side?: "left" | "right"
  variant?: "sidebar" | "floating" | "inset"
  collapsible?: "offcanvas" | "icon" | "none"
}

export const Sidebar = React.forwardRef<HTMLDivElement, SidebarProps>(
  (
    {
      side = "left",
      variant = "sidebar",
      collapsible = "icon",
      className,
      children,
      ...props
    },
    ref
  ) => {
    const { isMobile, state, openMobile, setOpenMobile } = useSidebar()

    if (collapsible === "none") {
      return (
        <div
          className={cn(
            "flex h-full w-[--sidebar-width] flex-col bg-sidebar text-sidebar-foreground border-r border-sidebar-border",
            className
          )}
          ref={ref}
          {...props}
        >
          {children}
        </div>
      )
    }

    if (isMobile) {
      return (
        <>
          {openMobile && (
            <div
              className="fixed inset-0 z-50 bg-black/60 backdrop-blur-xs transition-opacity duration-200"
              onClick={() => setOpenMobile(false)}
            />
          )}
          <div
            data-mobile="true"
            data-state={openMobile ? "open" : "closed"}
            className={cn(
              "fixed inset-y-0 z-50 flex h-full w-[--sidebar-width-mobile] max-w-[85vw] flex-col bg-sidebar text-sidebar-foreground shadow-2xl transition-transform duration-300 ease-in-out border-r border-sidebar-border",
              side === "left"
                ? openMobile
                  ? "translate-x-0 left-0"
                  : "-translate-x-full left-0"
                : openMobile
                ? "translate-x-0 right-0"
                : "translate-x-full right-0",
              className
            )}
            style={{ "--sidebar-width-mobile": SIDEBAR_WIDTH_MOBILE } as React.CSSProperties}
            ref={ref}
            {...props}
          >
            {children}
          </div>
        </>
      )
    }

    return (
      <div
        ref={ref}
        data-state={state}
        data-collapsible={state === "collapsed" ? collapsible : ""}
        data-variant={variant}
        data-side={side}
        className={cn(
          "group peer hidden md:block text-sidebar-foreground",
          className
        )}
      >
        {/* Invisible spacer for relative layout flow */}
        <div
          className={cn(
            "duration-200 relative h-screen bg-transparent transition-[width] ease-linear",
            state === "collapsed"
              ? collapsible === "icon"
                ? "w-[--sidebar-width-icon]"
                : "w-0"
              : "w-[--sidebar-width]"
          )}
        />
        {/* Fixed sidebar element */}
        <div
          className={cn(
            "duration-200 fixed inset-y-0 z-20 flex h-screen flex-col bg-sidebar border-r border-sidebar-border transition-[width,left,right] ease-linear",
            side === "left" ? "left-0" : "right-0",
            state === "collapsed"
              ? collapsible === "icon"
                ? "w-[--sidebar-width-icon]"
                : "w-0 overflow-hidden"
              : "w-[--sidebar-width]"
          )}
          {...props}
        >
          <div
            data-sidebar="sidebar"
            className="flex h-full w-full flex-col overflow-hidden"
          >
            {children}
          </div>
        </div>
      </div>
    )
  }
)
Sidebar.displayName = "Sidebar"

export const SidebarTrigger = React.forwardRef<
  HTMLButtonElement,
  React.ComponentProps<"button">
>(({ className, onClick, ...props }, ref) => {
  const { toggleSidebar } = useSidebar()

  return (
    <button
      ref={ref}
      data-sidebar="trigger"
      type="button"
      className={cn(
        "inline-flex h-8 w-8 items-center justify-center rounded-md border border-input bg-background text-foreground shadow-xs hover:bg-accent hover:text-accent-foreground focus-visible:outline-hidden focus-visible:ring-1 focus-visible:ring-ring disabled:pointer-events-none disabled:opacity-50 transition-colors",
        className
      )}
      onClick={(event) => {
        onClick?.(event)
        toggleSidebar()
      }}
      aria-label="Toggle Sidebar"
      {...props}
    >
      <PanelLeft className="h-4 w-4" />
      <span className="sr-only">Toggle Sidebar</span>
    </button>
  )
})
SidebarTrigger.displayName = "SidebarTrigger"

export const SidebarRail = React.forwardRef<
  HTMLButtonElement,
  React.ComponentProps<"button">
>(({ className, ...props }, ref) => {
  const { toggleSidebar } = useSidebar()

  return (
    <button
      ref={ref}
      data-sidebar="rail"
      aria-label="Toggle Sidebar"
      tabIndex={-1}
      onClick={toggleSidebar}
      title="Toggle Sidebar"
      className={cn(
        "absolute inset-y-0 z-20 hidden w-3 -translate-x-1/2 transition-all ease-linear after:absolute after:inset-y-0 after:left-1/2 after:w-[2px] hover:after:bg-sidebar-border group-data-[side=left]:-right-3 group-data-[side=right]:left-0 sm:flex",
        className
      )}
      {...props}
    />
  )
})
SidebarRail.displayName = "SidebarRail"

export const SidebarInset = React.forwardRef<
  HTMLDivElement,
  React.ComponentProps<"main">
>(({ className, ...props }, ref) => {
  return (
    <main
      ref={ref}
      className={cn(
        "relative flex min-h-screen flex-1 flex-col bg-background text-foreground overflow-y-auto transition-all duration-200",
        className
      )}
      {...props}
    />
  )
})
SidebarInset.displayName = "SidebarInset"

export const SidebarHeader = React.forwardRef<
  HTMLDivElement,
  React.ComponentProps<"div">
>(({ className, ...props }, ref) => {
  return (
    <div
      ref={ref}
      data-sidebar="header"
      className={cn("flex flex-col gap-2 p-2.5 border-b border-sidebar-border shrink-0", className)}
      {...props}
    />
  )
})
SidebarHeader.displayName = "SidebarHeader"

export const SidebarFooter = React.forwardRef<
  HTMLDivElement,
  React.ComponentProps<"div">
>(({ className, ...props }, ref) => {
  return (
    <div
      ref={ref}
      data-sidebar="footer"
      className={cn("flex flex-col gap-2 p-2 border-t border-sidebar-border shrink-0 mt-auto", className)}
      {...props}
    />
  )
})
SidebarFooter.displayName = "SidebarFooter"

export const SidebarSeparator = React.forwardRef<
  HTMLDivElement,
  React.ComponentProps<"div">
>(({ className, ...props }, ref) => {
  return (
    <div
      ref={ref}
      data-sidebar="separator"
      className={cn("mx-2 h-[1px] bg-sidebar-border", className)}
      {...props}
    />
  )
})
SidebarSeparator.displayName = "SidebarSeparator"

export const SidebarContent = React.forwardRef<
  HTMLDivElement,
  React.ComponentProps<"div">
>(({ className, ...props }, ref) => {
  return (
    <div
      ref={ref}
      data-sidebar="content"
      className={cn(
        "flex min-h-0 flex-1 flex-col gap-1 overflow-y-auto overflow-x-hidden p-2 scrollbar-none",
        className
      )}
      {...props}
    />
  )
})
SidebarContent.displayName = "SidebarContent"

export const SidebarGroup = React.forwardRef<
  HTMLDivElement,
  React.ComponentProps<"div">
>(({ className, ...props }, ref) => {
  return (
    <div
      ref={ref}
      data-sidebar="group"
      className={cn("relative flex w-full min-w-0 flex-col py-2", className)}
      {...props}
    />
  )
})
SidebarGroup.displayName = "SidebarGroup"

export const SidebarGroupLabel = React.forwardRef<
  HTMLDivElement,
  React.ComponentProps<"div">
>(({ className, ...props }, ref) => {
  const { state } = useSidebar()
  if (state === "collapsed") return null

  return (
    <div
      ref={ref}
      data-sidebar="group-label"
      className={cn(
        "flex h-8 shrink-0 items-center px-2 text-[11px] font-semibold uppercase tracking-wider text-sidebar-foreground/60 transition-opacity",
        className
      )}
      {...props}
    />
  )
})
SidebarGroupLabel.displayName = "SidebarGroupLabel"

export const SidebarGroupContent = React.forwardRef<
  HTMLDivElement,
  React.ComponentProps<"div">
>(({ className, ...props }, ref) => {
  return (
    <div
      ref={ref}
      data-sidebar="group-content"
      className={cn("w-full text-sm", className)}
      {...props}
    />
  )
})
SidebarGroupContent.displayName = "SidebarGroupContent"

export const SidebarMenu = React.forwardRef<
  HTMLUListElement,
  React.ComponentProps<"ul">
>(({ className, ...props }, ref) => {
  return (
    <ul
      ref={ref}
      data-sidebar="menu"
      className={cn("flex w-full min-w-0 flex-col gap-1 list-none p-0 m-0", className)}
      {...props}
    />
  )
})
SidebarMenu.displayName = "SidebarMenu"

export const SidebarMenuItem = React.forwardRef<
  HTMLLIElement,
  React.ComponentProps<"li">
>(({ className, ...props }, ref) => {
  return (
    <li
      ref={ref}
      data-sidebar="menu-item"
      className={cn("group/menu-item relative list-none", className)}
      {...props}
    />
  )
})
SidebarMenuItem.displayName = "SidebarMenuItem"

export interface SidebarMenuButtonProps extends React.ComponentProps<"button"> {
  isActive?: boolean
  tooltip?: string
  size?: "default" | "sm" | "lg"
}

export const SidebarMenuButton = React.forwardRef<
  HTMLButtonElement,
  SidebarMenuButtonProps
>(
  (
    {
      className,
      isActive = false,
      tooltip,
      size = "default",
      children,
      ...props
    },
    ref
  ) => {
    const { state } = useSidebar()

    const sizeClasses = {
      default: "h-9 px-2.5 text-xs",
      sm: "h-8 px-2 text-xs",
      lg: "h-11 px-3 text-sm",
    }

    return (
      <button
        ref={ref}
        data-sidebar="menu-button"
        data-active={isActive}
        title={state === "collapsed" ? tooltip : undefined}
        className={cn(
          "peer/menu-button flex w-full items-center gap-2.5 rounded-none font-medium transition-colors outline-hidden select-none",
          "hover:bg-sidebar-accent hover:text-sidebar-accent-foreground focus-visible:ring-2 focus-visible:ring-sidebar-ring",
          isActive
            ? "bg-sidebar-accent text-sidebar-primary font-semibold shadow-xs"
            : "text-sidebar-foreground/80 hover:text-sidebar-foreground",
          sizeClasses[size],
          state === "collapsed" && "justify-center px-0 w-9 h-9 mx-auto",
          className
        )}
        {...props}
      >
        {children}
      </button>
    )
  }
)
SidebarMenuButton.displayName = "SidebarMenuButton"

export const SidebarMenuBadge = React.forwardRef<
  HTMLDivElement,
  React.ComponentProps<"div">
>(({ className, ...props }, ref) => {
  const { state } = useSidebar()
  if (state === "collapsed") return null

  return (
    <div
      ref={ref}
      data-sidebar="menu-badge"
      className={cn(
        "ml-auto flex h-5 min-w-5 items-center justify-center rounded-none px-1.5 text-[10px] font-mono font-medium text-sidebar-foreground/70 bg-sidebar-accent/80",
        className
      )}
      {...props}
    />
  )
})
SidebarMenuBadge.displayName = "SidebarMenuBadge"
