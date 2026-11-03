import { forwardRef, type InputHTMLAttributes, type HTMLAttributes, type ButtonHTMLAttributes, type ReactNode } from 'react'
import { Search } from 'lucide-react'
import { cn } from '../../lib/utils'

export const ListSearch = forwardRef<HTMLInputElement, InputHTMLAttributes<HTMLInputElement> & { containerClassName?: string }>(
  ({ containerClassName, className, type = 'search', ...props }, ref) => (
    <label data-list-search className={cn('flex h-11 min-h-11 min-w-0 flex-1 items-center gap-2 rounded-md border border-border/40 bg-muted/20 px-3 focus-within:ring-2 focus-within:ring-ring', containerClassName)}>
      <Search aria-hidden="true" className="h-[18px] w-[18px] shrink-0 text-muted-foreground" />
      <input ref={ref} type={type} maxLength={200} className={cn('h-full w-full min-w-0 bg-transparent text-base text-foreground outline-none placeholder:text-muted-foreground md:text-sm', className)} {...props} />
    </label>
  ),
)
ListSearch.displayName = 'ListSearch'

export function ListTabs({ className, children, onKeyDown, ...props }: HTMLAttributes<HTMLDivElement>) {
  return <div data-list-tabs role="tablist" className={cn('flex min-w-0 shrink-0 items-center gap-4 overflow-x-auto border-b border-border/40 [scrollbar-width:none] [&::-webkit-scrollbar]:hidden', className)}
    onKeyDown={event => {
      onKeyDown?.(event)
      if (event.defaultPrevented || !['ArrowLeft', 'ArrowRight', 'Home', 'End'].includes(event.key)) return
      const tabs = Array.from(event.currentTarget.querySelectorAll<HTMLButtonElement>('button[role="tab"]:not(:disabled)'))
      const index = tabs.indexOf(document.activeElement as HTMLButtonElement)
      if (index < 0 || !tabs.length) return
      event.preventDefault()
      const next = event.key === 'Home' ? 0 : event.key === 'End' ? tabs.length - 1 : (index + (event.key === 'ArrowRight' ? 1 : -1) + tabs.length) % tabs.length
      tabs[next].focus(); tabs[next].click(); tabs[next].scrollIntoView({ block: 'nearest', inline: 'nearest' })
    }} {...props}>{children}</div>
}

export function ListTab({ selected, count, children, className, ...props }: ButtonHTMLAttributes<HTMLButtonElement> & { selected: boolean; count?: number }) {
  return <button type="button" role="tab" aria-selected={selected} aria-pressed={selected} tabIndex={selected ? 0 : -1}
    className={cn('inline-flex min-h-11 shrink-0 items-center justify-center gap-1.5 border-b-2 px-1 text-sm transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-inset focus-visible:ring-ring', selected ? 'border-foreground font-medium text-foreground' : 'border-transparent text-muted-foreground hover:text-foreground', className)} {...props}>
    {children}{count !== undefined && <span className="text-xs font-normal tabular-nums text-muted-foreground">{count}</span>}
  </button>
}

export function ListRowContent({ leading, title, description, trailing, children }: { leading?: ReactNode; title: ReactNode; description?: ReactNode; trailing?: ReactNode; children?: ReactNode }) {
  return <span data-list-row-content className="flex w-full min-w-0 items-center gap-3">
    {leading && <span className="flex shrink-0 items-center justify-center">{leading}</span>}
    <span className="min-w-0 flex-1"><span className="block truncate text-sm font-medium leading-snug text-foreground">{title}</span>
      {description && <span className="mt-1 block truncate text-xs leading-normal text-muted-foreground">{description}</span>}{children}
    </span>
    {trailing && <span className="shrink-0 text-xs tabular-nums text-muted-foreground">{trailing}</span>}
  </span>
}
