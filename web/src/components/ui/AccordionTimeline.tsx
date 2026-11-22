import { useId, useRef, useState, type ReactNode } from 'react'
import { ChevronDown } from 'lucide-react'
import { cn } from '../../lib/utils'

export interface AccordionTimelineItem {
  id: string
  title: ReactNode | ((expanded: boolean) => ReactNode)
  meta?: ReactNode
  indicator: ReactNode
  content?: ReactNode
  pinnedAction?: ReactNode
}

/** Caller supplies chronological facts and phase indicators; disclosure never
 * changes work state. Explicit choices survive polling and new default items. */
export function AccordionTimeline({ items, label, defaultExpandedIds, followLatest = true, className, showTriggerCaret = true }: {
  items: AccordionTimelineItem[]
  label: string
  defaultExpandedIds?: string[]
  followLatest?: boolean
  className?: string
  showTriggerCaret?: boolean
}) {
  const prefix = useId()
  const [choices, setChoices] = useState<Record<string, boolean>>({})
  const defaults = useRef<string[]>([])
  if (followLatest) defaults.current = defaultExpandedIds ?? items.filter(item => item.content != null && item.content !== false).slice(-1).map(item => item.id)
  return <ol aria-label={label} className={cn('min-w-0', className)} data-accordion-timeline onKeyDown={event => {
    if (!(event.target instanceof HTMLElement) || !event.target.matches('[data-timeline-trigger]') || !['ArrowUp', 'ArrowDown', 'Home', 'End'].includes(event.key)) return
    const buttons = Array.from(event.currentTarget.querySelectorAll<HTMLButtonElement>('[data-timeline-trigger]'))
    const current = buttons.indexOf(event.target as HTMLButtonElement)
    const next = event.key === 'Home' ? 0 : event.key === 'End' ? buttons.length - 1 : (current + (event.key === 'ArrowDown' ? 1 : -1) + buttons.length) % buttons.length
    event.preventDefault()
    buttons[next]?.focus()
  }}>
    {items.map((item, index) => {
      const expandable = item.content != null && item.content !== false
      const expanded = expandable && (choices[item.id] ?? defaults.current.includes(item.id))
      const bodyID = `${prefix}-${index}`
      const heading = (
        <>
          <span className={cn('min-w-0 flex-1 break-words text-sm font-medium leading-5', item.pinnedAction && 'pr-20 sm:pr-16')}>
            {typeof item.title === 'function' ? item.title(expanded) : item.title}
          </span>
          {item.meta && <span className="shrink-0 text-[11px] font-normal text-muted-foreground">{item.meta}</span>}
          {expandable && showTriggerCaret && (
            <ChevronDown className={cn('h-3.5 w-3.5 shrink-0 text-muted-foreground transition-transform motion-reduce:transition-none', expanded && 'rotate-180')} aria-hidden />
          )}
        </>
      )
      return <li key={item.id} data-timeline-item={item.id} data-work-event={item.id} className="relative min-w-0 pb-2 pl-6 last:pb-0">
        {index < items.length - 1 && <span data-timeline-connector aria-hidden className="absolute left-[7.5px] top-[30px] -bottom-[14px] w-px bg-border/60" />}
        <span className="absolute left-0 top-0 flex h-11 w-4 items-center justify-center">{item.indicator}</span>
        {item.pinnedAction && (
          <div className="absolute right-0 top-0 flex h-11 sm:h-8 items-center z-10">
            {item.pinnedAction}
          </div>
        )}
        {expandable ? <button data-timeline-trigger type="button" aria-expanded={expanded} aria-controls={expanded ? bodyID : undefined} onClick={() => setChoices(value => ({...value, [item.id]: !expanded}))} className="flex min-h-11 w-full items-center gap-3 py-2 text-left">{heading}</button> : <div className="flex min-h-11 items-center gap-3 py-2">{heading}</div>}
        {expanded && <div id={bodyID} className="min-w-0 pb-2 text-sm leading-relaxed text-muted-foreground">{item.content}</div>}
      </li>
    })}
  </ol>
}
