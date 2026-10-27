import { useEffect, useId, useRef, type ReactNode } from 'react'
import { X } from 'lucide-react'
import { trapDialogFocus } from '../../lib/dialog'

export function ActionDialog({
  open,
  title,
  onClose,
  busy = false,
  drawer = false,
  compact = false,
  children,
}: {
  open: boolean
  title: string
  onClose: () => void
  busy?: boolean
  drawer?: boolean
  compact?: boolean
  children: ReactNode
}) {
  const ref = useRef<HTMLDialogElement>(null),
    id = useId()
  useEffect(() => {
    if (!open) return
    const previous = document.activeElement as HTMLElement | null
    const dialog = ref.current
    dialog?.showModal()
    ;(
      dialog?.querySelector<HTMLElement>(
        'input:not(:disabled):not([tabindex="-1"]),select:not(:disabled),textarea:not(:disabled)',
      ) ||
      dialog?.querySelector<HTMLElement>(
        '[data-action-dialog-body] button:not(:disabled)',
      ) ||
      dialog?.querySelector<HTMLElement>('button')
    )?.focus()
    return () => {
      dialog?.close()
      if (previous?.isConnected) previous.focus()
    }
  }, [open])
  if (!open) return null
  return (
    <dialog
      ref={ref}
      aria-labelledby={id}
      onKeyDown={(e) => {
        e.stopPropagation()
        trapDialogFocus(e)
      }}
      onCancel={(e) => {
        e.preventDefault()
        e.stopPropagation()
        if (!busy) onClose()
      }}
      className={
        drawer
          ? 'session-detail-panel m-0 ml-auto h-dvh max-h-dvh w-full max-w-md overflow-y-auto border-l border-border bg-card p-4 text-card-foreground shadow-2xl backdrop:bg-black/60'
          : compact
            ? 'm-auto max-h-[85dvh] w-[calc(100%_-_24px)] max-w-xl overflow-hidden rounded-xl border border-border bg-card p-3 text-card-foreground shadow-2xl backdrop:bg-black/60'
            : 'm-auto max-h-[90dvh] w-[calc(100%_-_24px)] max-w-lg overflow-y-auto rounded-xl border border-border bg-card p-4 sm:p-5 text-card-foreground shadow-2xl backdrop:bg-black/60'
      }
    >
      <header
        className={
          compact
            ? 'absolute right-3 top-3 z-10'
            : 'mb-4 flex items-center justify-between gap-3'
        }
      >
        <h2 id={id} className={compact ? 'sr-only' : 'text-xl font-semibold'}>
          {title}
        </h2>
        <button
          type="button"
          className="icon-action"
          aria-label={`Close ${title}`}
          disabled={busy}
          onClick={onClose}
        >
          <X className="h-5 w-5" />
        </button>
      </header>
      <div data-action-dialog-body>{children}</div>
    </dialog>
  )
}
