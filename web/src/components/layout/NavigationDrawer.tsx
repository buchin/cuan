import { useEffect, useRef, type ReactNode } from 'react'
import { trapDialogFocus } from '../../lib/dialog'
import { X } from 'lucide-react'

// Native modal dialog makes background inert and traps focus, including Shift+Tab.
// Closed drawers are unmounted, so hidden controls cannot receive keyboard focus.
export function NavigationDrawer({
  onClose,
  children,
}: {
  onClose: () => void
  children: ReactNode
}) {
  const ref = useRef<HTMLDialogElement>(null)
  const close = useRef(onClose)
  close.current = onClose
  useEffect(() => {
    const previous = document.activeElement as HTMLElement | null
    const dialog = ref.current
    dialog?.showModal()
    const media = window.matchMedia('(min-width: 768px)')
    const resize = () => {
      if (media.matches) close.current()
    }
    media.addEventListener('change', resize)
    return () => {
      media.removeEventListener('change', resize)
      dialog?.close()
      if (previous?.isConnected) previous.focus()
    }
  }, [])
  return (
    <dialog
      ref={ref}
      aria-labelledby="navigation-title"
      aria-modal="true"
      className="navigation-drawer"
      onKeyDown={trapDialogFocus}
      onCancel={(e) => {
        e.preventDefault()
        onClose()
      }}
      onClick={(e) => {
        const r = e.currentTarget.getBoundingClientRect()
        if (
          e.clientX > r.right ||
          e.clientY > r.bottom ||
          e.clientX < r.left ||
          e.clientY < r.top
        )
          onClose()
      }}
    >
      <header className="flex h-16 shrink-0 items-center justify-between border-b border-border px-4">
        <h2 id="navigation-title" className="text-lg font-semibold">
          Navigasi
        </h2>
        <button
          autoFocus
          onClick={onClose}
          aria-label="Tutup menu"
          className="icon-action"
        >
          <X className="h-5 w-5" />
        </button>
      </header>
      <div className="min-h-0 flex-1 overflow-y-auto p-3">{children}</div>
    </dialog>
  )
}
