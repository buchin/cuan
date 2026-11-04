import { useEffect, useId, useLayoutEffect, useRef, useState } from 'react'
import { createPortal } from 'react-dom'
import { EyeOff } from 'lucide-react'
import { useLanguage } from '../../context/LanguageContext'
import { cn } from '../../lib/utils'

/** Display only when the backing message/surface is actually internal. */
export function InternalVisibility({ className }: { className?: string }) {
  const { t } = useLanguage()
  const [open, setOpen] = useState(false), [pinned, setPinned] = useState(false)
  const [position, setPosition] = useState({ left: 12, top: 12 })
  const trigger = useRef<HTMLButtonElement>(null), panel = useRef<HTMLDivElement>(null)
  const id = useId()
  const close = (restore = false) => { setOpen(false); setPinned(false); if (restore) trigger.current?.focus() }
  useLayoutEffect(() => {
    if (!open || !trigger.current || !panel.current) return
    const rect = trigger.current.getBoundingClientRect(), height = panel.current.getBoundingClientRect().height
    const width = Math.min(264, innerWidth - 24)
    setPosition({ left: Math.max(12, Math.min(rect.left, innerWidth - width - 12)), top: Math.max(12, Math.min(rect.bottom + 6, innerHeight - height - 12)) })
    if (pinned) panel.current.focus({ preventScroll: true })
  }, [open, pinned])
  useEffect(() => {
    if (!open) return
    const outside = (event: PointerEvent) => { if (!trigger.current?.contains(event.target as Node) && !panel.current?.contains(event.target as Node)) close() }
    const escape = (event: KeyboardEvent) => { if (event.key === 'Escape') { event.preventDefault(); event.stopPropagation(); close(true) } }
    const scroll = () => close()
    document.addEventListener('pointerdown', outside); document.addEventListener('keydown', escape)
    window.addEventListener('scroll', scroll, true); window.addEventListener('resize', scroll)
    return () => { document.removeEventListener('pointerdown', outside); document.removeEventListener('keydown', escape); window.removeEventListener('scroll', scroll, true); window.removeEventListener('resize', scroll) }
  }, [open])
  return <>
    <button ref={trigger} type="button" data-internal-visibility aria-label={t('Internal · Invisible to visitor')} aria-haspopup="dialog" aria-expanded={open} aria-controls={open ? id : undefined}
      className={cn('inline-flex h-11 w-11 shrink-0 items-center justify-center rounded-md text-muted-foreground/75 hover:text-foreground focus-visible:outline focus-visible:outline-2 focus-visible:outline-ring', className)}
      onPointerDown={event => event.stopPropagation()} onKeyDown={event => event.stopPropagation()}
      onMouseEnter={() => { if (matchMedia('(hover: hover) and (pointer: fine)').matches) setOpen(true) }} onMouseLeave={() => { if (!pinned) setOpen(false) }}
      onClick={event => { event.preventDefault(); event.stopPropagation(); if (open && pinned) close(); else { setPinned(true); setOpen(true) } }}>
      <EyeOff aria-hidden="true" className="h-3.5 w-3.5" />
    </button>
    {open && createPortal(<div ref={panel} id={id} role="dialog" aria-label={t('Internal visibility')} tabIndex={-1}
      className="fixed z-[140] max-w-[calc(100vw-24px)] rounded-lg border border-border/40 bg-popover p-3 text-xs leading-relaxed text-popover-foreground shadow-md outline-none"
      style={{ ...position, width: 264 }} onKeyDown={event => { if (event.key === 'Escape') { event.preventDefault(); event.stopPropagation(); close(true) } }} onPointerDown={event => event.stopPropagation()} onClick={event => event.stopPropagation()}>
      <p className="font-medium">{t('Internal')}</p>
      <p className="mt-1 text-muted-foreground">{t('Only you and your team can see this. The customer cannot see this message or attachment.')}</p>
    </div>, trigger.current?.closest('dialog') || document.body)}
  </>
}
