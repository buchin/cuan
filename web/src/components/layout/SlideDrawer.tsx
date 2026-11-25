import * as React from "react"
import { X } from "lucide-react"
import { cn } from "../../lib/utils"
import { trapDialogFocus } from "../../lib/dialog"
import { SurfaceHeader } from './SurfaceHeader'

export interface SlideDrawerProps {
  isOpen: boolean
  onClose: () => void
  title?: React.ReactNode
  description?: React.ReactNode
  badge?: React.ReactNode
  size?: "sm" | "md" | "lg" | "xl" | "full"
  children: React.ReactNode
  footer?: React.ReactNode
  headerActions?: React.ReactNode
  className?: string
  contentClassName?: string
  bodyRef?: React.Ref<HTMLDivElement>
  onBodyScroll?: React.UIEventHandler<HTMLDivElement>
  headerSize?: 'compact' | 'normal'
  closeOnBackdropClick?: boolean
  closeOnEscape?: boolean
  "aria-label"?: string
  "data-testid"?: string
}

export function SlideDrawer({
  isOpen,
  onClose,
  title,
  description,
  badge,
  size = "md",
  children,
  footer,
  headerActions,
  className,
  contentClassName,
  bodyRef,
  onBodyScroll,
  headerSize,
  closeOnBackdropClick = true,
  closeOnEscape = true,
  "aria-label": ariaLabel,
  "data-testid": dataTestId,
}: SlideDrawerProps) {
  const dialogRef = React.useRef<HTMLDialogElement>(null)

  // Manage native dialog open state
  React.useEffect(() => {
    const dialog = dialogRef.current
    if (!dialog) return

    if (isOpen) {
      if (!dialog.open) {
        try {
          dialog.showModal()
        } catch {
          dialog.setAttribute("open", "")
        }
      }
    } else {
      if (dialog.open) {
        dialog.close()
      }
    }
  }, [isOpen])

  if (!isOpen) return null

  const sizeClasses = {
    sm: "w-full sm:w-[380px]",
    md: "w-full sm:w-[480px]",
    lg: "w-full sm:w-[620px]",
    xl: "w-full sm:w-[780px]",
    full: "w-full sm:w-[94vw]",
  }[size]

  return (
    <dialog
      ref={dialogRef}
      aria-modal="true"
      data-slide-drawer
      data-testid={dataTestId}
      aria-label={ariaLabel || (typeof title === "string" ? title : "Slide Drawer")}
      onKeyDown={(e) => {
        trapDialogFocus(e)
        if (closeOnEscape && e.key === "Escape") {
          e.preventDefault()
          onClose()
        }
      }}
      onCancel={(e) => {
        e.preventDefault()
        if (closeOnEscape) onClose()
      }}
      onClick={(e) => {
        if (closeOnBackdropClick && e.target === dialogRef.current) {
          onClose()
        }
      }}
      className={cn(
        "fixed inset-y-0 right-0 left-auto m-0 flex h-dvh max-h-none max-w-none flex-col border-l border-border/40 bg-card p-0 text-card-foreground shadow-2xl backdrop:bg-black/40 backdrop:backdrop-blur-sm z-50 animate-in slide-in-from-right duration-200 outline-none",
        sizeClasses,
        className
      )}
    >
      {/* Drawer Header */}
      <SurfaceHeader title={title} description={description} badge={badge} size={headerSize} actions={<>
          {headerActions}
          <button
            type="button"
            onClick={onClose}
            aria-label="Close drawer"
            data-testid={dataTestId === "composer-tasks-drawer" ? "composer-tasks-collapse" : dataTestId ? `${dataTestId}-close` : "slide-drawer-close"}
            data-drawer-close
            className="rounded-md p-2 text-muted-foreground hover:bg-muted/60 hover:text-foreground transition-colors focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-ring min-h-11 min-w-11 flex items-center justify-center"
          >
            <X className="h-4 w-4" />
          </button>
      </>} />

      {/* Drawer Body (Scroller) */}
      <div
        ref={bodyRef}
        onScroll={onBodyScroll}
        data-drawer-scroll
        className={cn(
          "flex-1 min-h-0 overflow-y-auto overscroll-contain scroll-pb-2 px-4 py-4 sm:px-6 sm:py-5 min-w-0 focus:outline-none",
          contentClassName
        )}
      >
        {children}
      </div>

      {/* Optional Drawer Footer */}
      {footer && (
        <div className="shrink-0 border-t border-border/40 bg-card/60 px-4 py-3 sm:px-6 flex items-center justify-end gap-2">
          {footer}
        </div>
      )}
    </dialog>
  )
}

export const SideDrawer = SlideDrawer
