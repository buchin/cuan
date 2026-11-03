import type { ReactNode } from 'react'
import { Link } from 'react-router-dom'
import { ChevronLeft } from 'lucide-react'
import { cn } from '../../lib/utils'

export function BackAction({ to, onClick, children, className, disabled }: { to?: string; onClick?: () => void; children: ReactNode; className?: string; disabled?: boolean }) {
  const classes = cn('inline-flex min-h-11 items-center gap-1 text-sm text-muted-foreground hover:text-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring', className)
  const content = <><ChevronLeft className="h-[18px] w-[18px]" aria-hidden="true" /><span>{children}</span></>
  return to ? <Link to={to} className={classes}>{content}</Link> : <button type="button" disabled={disabled} className={classes} onClick={onClick}>{content}</button>
}

// The app header owns the page title; the detail canvas supplies context once.
export function DetailLayout({ back, description, children }: { back: ReactNode; description?: ReactNode; children: ReactNode }) {
  return <div data-detail-layout className="mx-auto w-full max-w-3xl space-y-6 pb-8">
    <div className="space-y-2">{back}{description && <p className="max-w-[65ch] text-sm leading-6 text-muted-foreground">{description}</p>}</div>
    {children}
  </div>
}
