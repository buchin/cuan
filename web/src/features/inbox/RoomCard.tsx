import { useState, useEffect, memo } from 'react'
import { useNavigate } from 'react-router-dom'
import {
  MessageSquare,
  Bot,
  Copy,
  Check,
  Users,
} from 'lucide-react'
import type { ChatContact } from '../../api/chat'
import { chatApi } from '../../api/chat'
import { useLanguage } from '../../context/LanguageContext'
import { cn } from '../../lib/utils'
import { relativeTime, agentDisplayName } from '../tasks/task-ui'
import {
  extractConversationRoomId,
  extractTaskId,
  isCuanHost,
  isConversationRoute,
  isTaskRoute,
} from './room-link'

export {
  extractConversationRoomId,
  extractTaskId,
  isCuanHost,
  isConversationRoute,
  isTaskRoute,
}

export interface RoomCardProps {
  roomId: string
  initialContact?: ChatContact
  onNavigate?: () => void
  onClick?: (contact: ChatContact) => void
  className?: string
}

// Global in-memory cache for room contacts to eliminate layout shifts
const roomMemoryCache = new Map<string, ChatContact>()

export function cacheRoom(roomId: string, contact: ChatContact) {
  if (roomId && contact) {
    roomMemoryCache.set(roomId, contact)
    if (contact.id) roomMemoryCache.set(contact.id, contact)
    if (contact.conversation_id) roomMemoryCache.set(contact.conversation_id, contact)
  }
}

export function getCachedRoom(roomId: string): ChatContact | undefined {
  return roomMemoryCache.get(roomId)
}

export const RoomCard = memo(function RoomCard({
  roomId,
  initialContact,
  onNavigate,
  onClick,
  className,
}: RoomCardProps) {
  const { t } = useLanguage()
  const navigate = useNavigate()

  const cached = initialContact || getCachedRoom(roomId)
  const [contact, setContact] = useState<ChatContact | null>(cached || null)
  const [loading, setLoading] = useState(!cached)
  const [error, setError] = useState(false)
  const [copied, setCopied] = useState(false)

  useEffect(() => {
    if (initialContact) {
      cacheRoom(roomId, initialContact)
      setContact(initialContact)
      setLoading(false)
      return
    }
    const memoryContact = getCachedRoom(roomId)
    if (memoryContact) {
      setContact(memoryContact)
      setLoading(false)
    }
    let active = true
    chatApi
      .getConversation(roomId)
      .then((detail) => {
        if (active && detail?.contact) {
          cacheRoom(roomId, detail.contact)
          setContact(detail.contact)
          setError(false)
        }
      })
      .catch(() => {
        if (active) {
          // Fallback to checking contact list
          chatApi
            .listContacts()
            .then((list) => {
              if (active) {
                const found = list.find(
                  (c) => c.id === roomId || c.conversation_id === roomId,
                )
                if (found) {
                  cacheRoom(roomId, found)
                  setContact(found)
                  setError(false)
                } else if (!memoryContact) {
                  setError(true)
                }
              }
            })
            .catch(() => {
              if (active && !memoryContact) setError(true)
            })
        }
      })
      .finally(() => {
        if (active) setLoading(false)
      })
    return () => {
      active = false
    }
  }, [roomId, initialContact])

  const handleCardClick = (_e: React.MouseEvent) => {
    if (onClick && contact) {
      onClick(contact)
    } else {
      navigate(`/inbox/${encodeURIComponent(roomId)}`)
    }
    onNavigate?.()
  }

  const handleKeyDown = (e: React.KeyboardEvent) => {
    if (e.target !== e.currentTarget) return
    if (e.key === 'Enter' || e.key === ' ') {
      e.preventDefault()
      if (onClick && contact) {
        onClick(contact)
      } else {
        navigate(`/inbox/${encodeURIComponent(roomId)}`)
      }
      onNavigate?.()
    }
  }

  const handleCopyLink = async (e: React.MouseEvent) => {
    e.preventDefault()
    e.stopPropagation()
    try {
      const origin = typeof window !== 'undefined' ? window.location.origin : ''
      await navigator.clipboard.writeText(`${origin}/inbox/${roomId}`)
    } catch {
      // ignore
    }
    setCopied(true)
    setTimeout(() => setCopied(false), 2000)
  }

  const handleOpenRoom = (e: React.MouseEvent) => {
    e.preventDefault()
    e.stopPropagation()
    navigate(`/inbox/${encodeURIComponent(roomId)}`)
    onNavigate?.()
  }

  if (loading) {
    return (
      <div
        data-card="room-card"
        className={cn(
          'w-full max-w-full rounded-xl border border-border/40 bg-card/40 p-3.5 sm:p-4 text-card-foreground select-none space-y-2.5 min-h-[44px]',
          className
        )}
      >
        <div className="flex items-center justify-between gap-2 border-b border-border/30 pb-2">
          <div className="flex items-center gap-2">
            <span className="relative flex h-2 w-2 shrink-0 items-center justify-center">
              <span className="relative inline-flex rounded-full h-1.5 w-1.5 bg-muted-foreground/60" />
            </span>
            <span className="font-mono text-xs text-muted-foreground">
              {roomId.slice(0, 8)}
            </span>
          </div>
          <span className="text-[11px] text-muted-foreground/70 font-sans">
            {t('Loading…')}
          </span>
        </div>
        <div className="space-y-1.5 py-0.5">
          <div className="h-4 w-2/3 bg-muted/40 rounded-sm" />
          <div className="h-3 w-1/3 bg-muted/30 rounded-sm" />
        </div>
      </div>
    )
  }

  if (error || !contact) {
    return (
      <div
        data-card="room-card"
        onClick={handleCardClick}
        onKeyDown={handleKeyDown}
        role="button"
        tabIndex={0}
        className={cn(
          'w-full max-w-full rounded-xl border border-border/80 bg-card p-3.5 sm:p-4 text-card-foreground shadow-xs transition-all hover:border-primary/50 cursor-pointer min-h-[44px]',
          className
        )}
      >
        <div className="flex items-center justify-between gap-2">
          <div className="flex items-center gap-1.5 text-xs text-muted-foreground font-mono">
            <MessageSquare className="h-3.5 w-3.5 shrink-0 text-muted-foreground" />
            <span>#{roomId.slice(0, 8)}</span>
          </div>
          <button
            type="button"
            onClick={handleCopyLink}
            onKeyDown={(e) => e.stopPropagation()}
            aria-label={t('Copy link')}
            className="action-secondary min-h-[44px] min-w-[44px] px-3 text-xs font-medium rounded-lg inline-flex items-center justify-center gap-1.5 cursor-pointer"
          >
            {copied ? <Check className="h-3.5 w-3.5 text-emerald-600" /> : <Copy className="h-3.5 w-3.5" />}
            <span>{copied ? t('Copied') : t('Copy link')}</span>
          </button>
        </div>
        <p className="mt-1 text-sm font-medium text-foreground">{t('Room')} #{roomId.slice(0, 8)}</p>
      </div>
    )
  }

  const isCustomer = contact.type === 'customer'
  const isOnline = contact.is_online || contact.status === 'online'
  const displayName = contact.name || (isCustomer ? t('Customer Room') : t('Agent Room'))

  return (
    <div
      data-card="room-card"
      onClick={handleCardClick}
      onKeyDown={handleKeyDown}
      role="button"
      tabIndex={0}
      className={cn(
        'group relative w-full max-w-full min-w-0 rounded-xl border border-border/80 bg-card p-3.5 sm:p-4 text-card-foreground shadow-xs transition-all hover:border-primary/50 hover:shadow-md cursor-pointer select-none space-y-2.5 min-h-[44px]',
        className
      )}
    >
      {/* Header: Room Type Badge, Online Indicator, Ref, and Timestamp */}
      <div className="flex flex-wrap items-center justify-between gap-2">
        <div className="flex flex-wrap items-center gap-2 min-w-0">
          <span
            className={cn(
              'inline-flex shrink-0 items-center gap-1.5 rounded-full border px-2.5 py-0.5 text-[11px] font-semibold transition-colors',
              isCustomer
                ? 'bg-blue-500/10 text-blue-600 dark:text-blue-400 border-blue-500/20'
                : 'bg-purple-500/10 text-purple-600 dark:text-purple-400 border-purple-500/20'
            )}
          >
            {isCustomer ? (
              <MessageSquare className="h-3 w-3 shrink-0" />
            ) : (
              <Bot className="h-3 w-3 shrink-0" />
            )}
            <span>{isCustomer ? t('Customer Room') : t('Agent Room')}</span>
          </span>

          {isOnline && (
            <span className="inline-flex items-center gap-1 text-[11px] font-medium text-emerald-600 dark:text-emerald-400">
              <span className="relative flex h-2 w-2 shrink-0 items-center justify-center">
                <span className="animate-ping absolute inline-flex h-full w-full rounded-full bg-emerald-400 opacity-75" />
                <span className="relative inline-flex rounded-full h-1.5 w-1.5 bg-emerald-500" />
              </span>
              <span>Online</span>
            </span>
          )}

          <span className="font-mono text-xs text-muted-foreground">
            #{roomId.slice(0, 8)}
          </span>
        </div>

        {contact.updated_at && (
          <time
            dateTime={contact.updated_at}
            title={new Date(contact.updated_at).toLocaleString()}
            className="text-xs text-muted-foreground whitespace-nowrap shrink-0"
          >
            {t(relativeTime(contact.updated_at))}
          </time>
        )}
      </div>

      {/* Body: Room / Contact Name & Snippet */}
      <div className="space-y-1">
        <h4 className="text-sm font-semibold text-foreground group-hover:text-primary transition-colors break-words leading-snug line-clamp-2">
          {displayName}
        </h4>
        {contact.product && (
          <p className="text-xs text-muted-foreground/90 font-medium line-clamp-1 break-words">
            {contact.product}
          </p>
        )}
        {contact.last_message ? (
          <p className="text-xs text-muted-foreground line-clamp-2 leading-relaxed break-words">
            {contact.last_message}
          </p>
        ) : null}
      </div>

      {/* Participants snippet */}
      {(contact.agent_name || contact.agent_id) && (
        <div className="flex items-center gap-1.5 text-xs text-muted-foreground font-mono">
          <Users className="h-3 w-3 shrink-0 text-muted-foreground/70" />
          <span className="text-muted-foreground/70">{t('Participants')}:</span>
          <span className="font-medium text-foreground">
            {agentDisplayName(contact.agent_name || contact.agent_id)}
          </span>
        </div>
      )}

      {/* Footer: Quick Action Buttons (Tombol Aksi Cepat) */}
      <div className="flex flex-wrap items-center justify-between gap-2 pt-2 border-t border-border/50">
        <button
          type="button"
          onClick={handleCopyLink}
          onKeyDown={(e) => e.stopPropagation()}
          aria-label={t('Copy link')}
          className="action-secondary min-h-[44px] min-w-[44px] px-3.5 text-xs font-medium rounded-lg inline-flex items-center justify-center gap-1.5 transition-colors cursor-pointer"
        >
          {copied ? (
            <Check className="h-3.5 w-3.5 text-emerald-600 dark:text-emerald-400 shrink-0" />
          ) : (
            <Copy className="h-3.5 w-3.5 shrink-0" />
          )}
          <span>{copied ? t('Copied') : t('Copy link')}</span>
        </button>

        <button
          type="button"
          onClick={handleOpenRoom}
          onKeyDown={(e) => e.stopPropagation()}
          aria-label={`${t('Open room')}: ${displayName}`}
          className="action-primary min-h-[44px] min-w-[44px] px-3.5 text-xs font-medium rounded-lg inline-flex items-center justify-center gap-1.5 shadow-xs cursor-pointer"
        >
          <MessageSquare className="h-3.5 w-3.5 shrink-0" />
          <span>{t('Open room')}</span>
        </button>
      </div>
    </div>
  )
})

export const ChatPreviewCard = RoomCard
export type ChatPreviewCardProps = RoomCardProps
