import { Bot, User } from 'lucide-react'
import { getContactInitials } from '../../features/inbox/contact-initials'
import { cn } from '../../lib/utils'

export interface AvatarProps {
  name?: string
  avatar?: string
  type?: 'agent' | 'customer' | string
  size?: 'sm' | 'md' | 'lg'
  className?: string
}

const SIZE_CLASSES = {
  sm: 'h-8 w-8 text-xs',
  md: 'h-9 w-9 text-xs',
  lg: 'h-12 w-12 text-base',
} as const

const ICON_SIZE_CLASSES = {
  sm: 'h-4 w-4',
  md: 'h-4 w-4',
  lg: 'h-6 w-6',
} as const

/**
 * Shared identity avatar for Inbox and Contacts lists.
 * Renders photo/emoji if available, Bot icon for agents, monogram initials
 * for named contacts, and a neutral User icon for anonymous visitors.
 */
export function Avatar({
  name,
  avatar,
  type,
  size = 'md',
  className,
}: AvatarProps) {
  const trimmedAvatar = avatar?.trim()
  const hasCustomAvatar = Boolean(trimmedAvatar && trimmedAvatar !== '👤')
  const isImageUrl =
    hasCustomAvatar &&
    (trimmedAvatar!.startsWith('http') || trimmedAvatar!.startsWith('/'))
  const initials = getContactInitials(name)
  const iconSize = ICON_SIZE_CLASSES[size]

  return (
    <span
      aria-hidden="true"
      data-avatar="true"
      className={cn(
        'flex shrink-0 items-center justify-center rounded-full bg-secondary/80 font-semibold border border-border/50 text-foreground overflow-hidden avatar select-none',
        SIZE_CLASSES[size],
        className,
      )}
    >
      {hasCustomAvatar ? (
        isImageUrl ? (
          <img
            src={trimmedAvatar}
            alt=""
            className="h-full w-full object-cover rounded-full"
          />
        ) : (
          <span>{trimmedAvatar}</span>
        )
      ) : type === 'agent' ? (
        <Bot className={cn(iconSize, 'text-primary')} />
      ) : initials ? (
        <span className="tracking-wide select-none">{initials}</span>
      ) : (
        <User className={cn(iconSize, 'text-muted-foreground')} />
      )}
    </span>
  )
}
