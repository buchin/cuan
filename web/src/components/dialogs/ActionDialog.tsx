import { useEffect, useId, useRef, type ReactNode } from 'react'
import { X } from 'lucide-react'
import { trapDialogFocus } from '../../lib/dialog'

export function ActionDialog({
  open,
  title,
  onClose,
  busy = false,
  children,
}: {
  open: boolean
  title: string
  onClose: () => void
  busy?: boolean
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
        'input:not(:disabled),select:not(:disabled),textarea:not(:disabled)',
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
      onKeyDown={trapDialogFocus}
      onCancel={(e) => {
        e.preventDefault()
        if (!busy) onClose()
      }}
      className="m-auto max-h-[90dvh] w-[calc(100%_-_24px)] max-w-lg overflow-y-auto rounded-xl border border-border bg-card p-5 text-card-foreground shadow-2xl backdrop:bg-black/60"
    >
      <header className="mb-4 flex items-center justify-between gap-3">
        <h2 id={id} className="text-xl font-semibold">
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
