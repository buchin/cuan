import type { HTMLAttributes } from 'react'
import { Link, useLocation } from 'react-router-dom'
import { Inbox, CheckSquare, FolderGit2, Users, Settings, UserRound, Bell } from 'lucide-react'
import { CuanMark } from '../brand/CuanMark'
import { useLanguage } from '../../context/LanguageContext'
import { cn } from '../../lib/utils'

export function PageHeader({ className, ...props }: HTMLAttributes<HTMLElement>) {
  return <header data-page-header className={cn('app-page-header flex h-14 shrink-0 items-center justify-between gap-2 border-b border-border/40 bg-background px-3 md:h-13 md:px-5 [&_.lucide]:h-[18px] [&_.lucide]:w-[18px]', className)} {...props} />
}
// Shared mobile page identity; desktop navigation already supplies the icon.
export function PageHeaderBrand() {
  const { t } = useLanguage()
  const { pathname } = useLocation()
  const section = pathname.split('/')[1]
  const Icon = ({ inbox: Inbox, chat: Inbox, tasks: CheckSquare, projects: FolderGit2, contacts: Users, settings: Settings, account: UserRound, notifications: Bell, schedules: Settings } as Record<string, typeof Inbox>)[section]
  if (Icon) return <span aria-hidden="true" className="inline-flex h-11 w-8 shrink-0 items-center justify-center text-muted-foreground md:hidden"><Icon className="h-[18px] w-[18px]" /></span>
  return <Link to="/studio" aria-label={t('Go to Studio')} className="inline-flex h-11 w-11 shrink-0 items-center justify-center rounded-md text-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring md:hidden">
    <CuanMark decorative className="h-7 w-7" />
  </Link>
}
export function PageTitle({ className, ...props }: HTMLAttributes<HTMLHeadingElement>) {
  return <h1 className={cn('page-title min-w-0 truncate text-base font-semibold tracking-tight text-foreground', className)} {...props} />
}
