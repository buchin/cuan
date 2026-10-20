import { useEffect, useRef, useState, type ReactNode } from 'react'
import { createPortal } from 'react-dom'
import { MoreHorizontal } from 'lucide-react'
import { useLanguage } from '../../context/LanguageContext'

export type ItemAction = { label: string; run: () => void | Promise<void>; disabled?: boolean; previewLabel?: string }
export function ItemActions({ label, actions, swipeAction, children, subtle = false }: {
  label: string; actions: ItemAction[]; swipeAction?: ItemAction; children: ReactNode; subtle?: boolean
}) {
  const { t } = useLanguage()
  const [menu, setMenu] = useState<{ x: number; y: number } | null>(null)
  const [offset, setOffset] = useState(0)
  const [error, setError] = useState('')
  const [feedback, setFeedback] = useState('')
  const [busy, setBusy] = useState(false)
  const trigger = useRef<HTMLButtonElement>(null), panel = useRef<HTMLDivElement>(null)
  const timer = useRef<ReturnType<typeof setTimeout>>()
  const pointer = useRef<{ x: number; y: number; dx: number; horizontal: boolean }>()
  const suppress = useRef(false)
  const clear = () => { clearTimeout(timer.current) }
  const close = () => { suppress.current = false; setMenu(null); trigger.current?.focus({ preventScroll: true }) }
  useEffect(() => () => clearTimeout(timer.current), [])
  useEffect(() => { if (!feedback) return; const timeout = setTimeout(() => setFeedback(''), 2000); return () => clearTimeout(timeout) }, [feedback])
  useEffect(() => {
    if (!menu) return
    panel.current?.querySelector<HTMLButtonElement>('button:not(:disabled)')?.focus()
    const outside = (event: PointerEvent) => {
      if (!panel.current?.contains(event.target as Node) && !trigger.current?.contains(event.target as Node)) setMenu(null)
    }
    const dismiss = () => setMenu(null)
    const escape = (event: KeyboardEvent) => { if (event.key === 'Escape' && !event.defaultPrevented) { event.preventDefault(); close() } }
    document.addEventListener('pointerdown', outside)
    document.addEventListener('keydown', escape)
    window.addEventListener('resize', dismiss)
    window.addEventListener('scroll', dismiss, true)
    return () => {
      document.removeEventListener('pointerdown', outside)
      document.removeEventListener('keydown', escape)
      window.removeEventListener('resize', dismiss)
      window.removeEventListener('scroll', dismiss, true)
    }
  }, [menu])
  function open(x: number, y: number) {
    if (!actions.length || busy || window.getSelection()?.toString()) return
    clear()
    setMenu({ x: Math.max(8, Math.min(x, innerWidth - 240)), y: Math.max(8, Math.min(y, innerHeight - Math.min(actions.length * 44 + 12, innerHeight - 16))) })
  }
  async function perform(action: ItemAction) {
    if (busy || action.disabled) return
    setMenu(null); setOffset(0); setBusy(true); setError('')
    try { await action.run(); if (action.label.startsWith('Copy')) setFeedback(t('Copied')) } catch { setError(t('Could not complete this action. Try again.')) }
    finally { setBusy(false) }
  }
  return (
    <div className={`relative min-w-0 ${subtle ? "message-actions max-w-[92%]" : "overflow-hidden"}`} style={swipeAction ? { touchAction: 'pan-y' } : undefined}
      onContextMenu={event => {
        if (window.getSelection()?.toString() || (event.target as HTMLElement).closest('input,textarea,[contenteditable=true]')) return
        event.preventDefault(); open(event.clientX, event.clientY)
      }}
      onKeyDown={event => {
        if ((event.shiftKey && event.key === 'F10') || event.key === 'ContextMenu') {
          event.preventDefault(); const r = event.currentTarget.getBoundingClientRect(); open(r.right - 232, r.top)
        }
      }}
      onPointerDown={event => {
        if (event.pointerType !== 'touch' || busy || (event.target as HTMLElement).closest('[data-item-menu],input,textarea,[contenteditable=true]')) return
        suppress.current = false
        pointer.current = { x: event.clientX, y: event.clientY, dx: 0, horizontal: false }
        timer.current = setTimeout(() => { suppress.current = true; open(event.clientX, event.clientY) }, 550)
      }}
      onPointerMove={event => {
        const start = pointer.current
        if (!start) return
        const dx = event.clientX - start.x, dy = event.clientY - start.y
        if (Math.abs(dx) > 8 || Math.abs(dy) > 8) clear()
        if (!start.horizontal && Math.abs(dy) > 12 && Math.abs(dy) >= Math.abs(dx)) { pointer.current = undefined; return }
        if (swipeAction && !swipeAction.disabled && dx < -12 && Math.abs(dx) > Math.abs(dy) * 1.5) {
          start.horizontal = true
          if (!event.currentTarget.hasPointerCapture(event.pointerId)) event.currentTarget.setPointerCapture(event.pointerId)
        }
        if (start.horizontal) { start.dx = Math.max(-100, Math.min(0, dx)); setOffset(start.dx); suppress.current = true }
      }}
      onPointerUp={() => {
        clear()
        const start = pointer.current; pointer.current = undefined
        if (start?.horizontal && start.dx <= -72 && swipeAction) void perform(swipeAction)
        setOffset(0)
      }}
      onPointerCancel={() => { clear(); pointer.current = undefined; setOffset(0); suppress.current = false }}
      onClickCapture={event => {
        if (suppress.current && !(event.target as HTMLElement).closest('[data-item-menu]')) {
          event.preventDefault(); event.stopPropagation(); suppress.current = false
        }
      }}
    >
      {!!offset && swipeAction && <div aria-hidden="true" className="absolute inset-0 flex items-center justify-end rounded-md bg-muted pr-3 text-xs font-medium">{t(swipeAction.previewLabel || swipeAction.label)}</div>}
      <div className={`relative flex min-w-0 ${subtle ? "items-start" : "items-center bg-background"}`} style={{ transform: offset ? `translateX(${offset}px)` : undefined }}>
        <div className="min-w-0 flex-1">{children}</div>
        <button ref={trigger} data-item-menu type="button" disabled={busy} className="icon-action text-muted-foreground/60 hover:text-foreground focus-visible:text-foreground" aria-label={t('Actions for') + ' ' + label} aria-haspopup="menu" aria-expanded={!!menu}
          onClick={event => { event.stopPropagation(); if (menu) close(); else { const r = event.currentTarget.getBoundingClientRect(); open(r.right - 232, r.bottom) } }}>
          <MoreHorizontal className="h-3.5 w-3.5" aria-hidden="true" />
        </button>
      </div>
      {feedback && <p role="status" className="px-3 pb-2 text-xs text-muted-foreground">{feedback}</p>}
      {error && <p role="alert" className="px-3 pb-2 text-xs text-destructive">{error}</p>}
      {menu && createPortal(
        <div ref={panel} role="menu" aria-label={t('Actions for') + ' ' + label} className="fixed z-[100] max-h-[calc(100dvh-16px)] w-56 overflow-y-auto rounded-lg border bg-popover p-1 text-popover-foreground shadow-lg"
          style={{ left: menu.x, top: menu.y }}
          onKeyDown={event => {
            const buttons = Array.from(event.currentTarget.querySelectorAll<HTMLButtonElement>('button:not(:disabled)'))
            const index = buttons.indexOf(document.activeElement as HTMLButtonElement)
            if (event.key === 'Escape' || event.key === 'Tab') { if (event.key === 'Escape') event.preventDefault(); close(); return }
            if (['ArrowDown','ArrowUp','Home','End'].includes(event.key)) {
              event.preventDefault()
              const next = event.key === 'Home' ? 0 : event.key === 'End' ? buttons.length - 1 : (index + (event.key === 'ArrowDown' ? 1 : -1) + buttons.length) % buttons.length
              buttons[next]?.focus()
            }
          }}>
          {actions.map(action => <button key={action.label} role="menuitem" type="button" disabled={action.disabled} className="flex min-h-11 w-full items-center rounded-md px-3 text-left text-sm hover:bg-accent focus:bg-accent focus:outline-none disabled:opacity-50" onClick={() => void perform(action)}>{t(action.label)}</button>)}
        </div>, document.body)}
    </div>
  )
}

export async function copyItemLink(path: string) {
  await navigator.clipboard.writeText(new URL(path, location.origin).href)
}
