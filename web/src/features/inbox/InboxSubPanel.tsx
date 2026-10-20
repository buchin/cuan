import { ItemActions, copyItemLink } from '../../components/ui/ItemActions'
import { useArchivedItems, ArchiveNotice } from '../../hooks/useArchivedItems'
import { useConnectionClock } from '../agents/useConnectionClock'
import { useLanguage } from '../../context/LanguageContext'
import type { Agent } from '../../api/agents'
import { agentForContact, connectionStatus } from '../agents/connection'
import { useState } from 'react'
import { ChatContact } from '../../api/chat'
import { Search, Bot, User } from 'lucide-react'
import { cn } from '../../lib/utils'

export function conversationPreview(message?: string) {
  if (!message?.trim()) return 'No messages yet.'
  return (
    message
      .replace(/^\s*\[@[^\]\r\n]{1,100}\]\s*/, '')
      .replace(/```cuan-task[\s\S]*?(?:```|$)/g, ' Draft task ')
      .replace(/!\[([^\]]*)\]\([^)]*\)/g, '$1')
      .replace(/\[([^\]]+)\]\([^)]*\)/g, '$1')
      .replace(/```[^\n]*\n?/g, ' ')
      .replace(/`([^`]+)`/g, '$1')
      .replace(/\*\*([^*]+)\*\*/g, '$1')
      .replace(/__([^_]+)__/g, '$1')
      .replace(/\*([^*\n]+)\*/g, '$1')
      .replace(/_([^_\n]+)_/g, '$1')
      .replace(/~~([^~]+)~~/g, '$1')
      .replace(/(^|\n)\s{0,3}(?:#{1,6} |[>]|[-*] )/g, ' ')
      .replace(/\s+/g, ' ')
      .trim() || 'No messages yet.'
  )
}

export type ContactFilterTab = 'all' | 'agents' | 'customers'

interface InboxSubPanelProps {
  contacts: ChatContact[]
  agents?: Agent[]
  now?: number
  loading?: boolean
  error?: string
  onRetry?: () => void
  activeContactId?: string
  onSelectContact: (contact: ChatContact) => void
  selectedTab?: ContactFilterTab
  onTabChange?: (tab: ContactFilterTab) => void
  searchQuery?: string
  onSearchChange?: (q: string) => void
}

export function InboxSubPanel({
  contacts,
  agents = [],
  now: suppliedNow,
  loading = false,
  error = '',
  onRetry,
  activeContactId,
  onSelectContact,
  selectedTab = 'all',
  onTabChange,
  searchQuery = '',
  onSearchChange,
}: InboxSubPanelProps) {
  const archive = useArchivedItems('conversation')
  const [showArchived, setShowArchived] = useState(false)
  const clock = useConnectionClock(agents)
  const now = suppliedNow ?? clock
  const { t, language } = useLanguage()

  const [internalTab, setInternalTab] = useState<ContactFilterTab>(selectedTab)
  const [internalQuery, setInternalQuery] = useState(searchQuery)

  const activeTab = onTabChange ? selectedTab : internalTab
  const handleTabClick = (tab: ContactFilterTab) => {
    if (onTabChange) onTabChange(tab)
    else setInternalTab(tab)
  }

  const query = onSearchChange ? searchQuery : internalQuery
  const handleQueryChange = (val: string) => {
    if (onSearchChange) onSearchChange(val)
    else setInternalQuery(val)
  }

  const agentCount = contacts.filter((c) => c.type === 'agent').length
  const customerCount = contacts.filter((c) => c.type === 'customer').length

  const filteredContacts = contacts.filter((c) => {
    if (archive.isArchived(c.conversation_id || c.id, c.last_message_at || c.updated_at) !== showArchived) return false
    if (activeTab === 'agents' && c.type !== 'agent') return false
    if (activeTab === 'customers' && c.type !== 'customer') return false
    if (query.trim()) {
      const q = query.toLowerCase()
      const matchName = [c.name, c.agent_name]
        .join(' ')
        .toLowerCase()
        .includes(q)
      const matchMsg = (c.last_message || '').toLowerCase().includes(q)
      const matchProduct = (c.product || '').toLowerCase().includes(q)
      return matchName || matchMsg || matchProduct
    }
    return true
  })

  const formatTime = (iso?: string) => {
    if (!iso) return ''
    const date = new Date(iso)
    if (!Number.isFinite(date.getTime())) return ''
    const today = new Date()
    const yesterday = new Date()
    yesterday.setDate(today.getDate() - 1)
    if (date.toDateString() === today.toDateString())
      return date.toLocaleTimeString(language, {
        hour: '2-digit',
        minute: '2-digit',
        hourCycle: 'h23',
      })
    if (date.toDateString() === yesterday.toDateString()) return 'Yesterday'
    return date.toLocaleDateString(language, { day: 'numeric', month: 'short' })
  }

  return (
    <div className="flex flex-col h-full inbox-list text-foreground select-none">
      <div className="px-3 pt-2 pb-0 border-b border-border/40 space-y-1">
        {/* Search Bar */}
        <div className="relative">
          <Search className="absolute left-2.5 top-3.5 h-3.5 w-3.5 text-muted-foreground" />
          <input
            type="text"
            value={query}
            onChange={(e) => handleQueryChange(e.target.value)}
            aria-label={t('Search conversations')}
            placeholder="Search conversations…"
            className="w-full min-h-11 pl-8 pr-3 py-2 text-base rounded-full inbox-search border border-transparent text-foreground placeholder:text-muted-foreground focus:outline-none focus:ring-1 focus:ring-ring"
          />
        </div>

        <button type="button" className="min-h-11 px-2 text-xs text-muted-foreground" aria-pressed={showArchived} onClick={() => setShowArchived((value) => !value)}>{t(showArchived ? 'Back to Inbox' : 'Archived conversations')}</button>
        {/* Filter Tabs (All, Agents, Customers) */}
        <div className="flex gap-4 text-sm">
          <button
            aria-pressed={activeTab === 'all'}
            onClick={() => handleTabClick('all')}
            className={cn(
              'flex-1 min-h-11 border-b-2 py-2 text-center transition-colors font-medium text-sm',
              activeTab === 'all'
                ? 'border-primary text-primary'
                : 'border-transparent text-muted-foreground hover:text-foreground',
            )}
          >
            {t('All')}
          </button>
          <button
            aria-pressed={activeTab === 'agents'}
            onClick={() => handleTabClick('agents')}
            className={cn(
              'flex-1 min-h-11 border-b-2 py-2 text-center transition-colors font-medium text-sm flex items-center justify-center gap-1',
              activeTab === 'agents'
                ? 'border-primary text-primary'
                : 'border-transparent text-muted-foreground hover:text-foreground',
            )}
          >
            <span>{t('Agents')}</span>
            <span className="sr-only">({agentCount})</span>
          </button>
          <button
            aria-pressed={activeTab === 'customers'}
            onClick={() => handleTabClick('customers')}
            className={cn(
              'flex-1 min-h-11 border-b-2 py-2 text-center transition-colors font-medium text-sm flex items-center justify-center gap-1',
              activeTab === 'customers'
                ? 'border-primary text-primary'
                : 'border-transparent text-muted-foreground hover:text-foreground',
            )}
          >
            <span>{t('Customers')}</span>
            <span className="sr-only">({customerCount})</span>
          </button>
        </div>
      </div>

      {archive.error && <div role="alert" className="px-3 text-xs">{t(archive.error)} <button className="min-h-11 underline" onClick={() => void archive.refresh()}>{t('Try again')}</button></div>}
      {/* Contacts List */}
      <div className="flex-1 overflow-y-auto divide-y divide-border/20">
        {loading ? (
          <p role="status" className="p-4 text-sm">
            Loading conversations…
          </p>
        ) : error ? (
          <div role="alert" className="p-4 text-sm">
            <p>{error}</p>
            <button className="action-secondary mt-3" onClick={onRetry}>
              Try again
            </button>
          </div>
        ) : filteredContacts.length === 0 ? (
          <div className="p-6 text-center text-xs text-muted-foreground">
            No matching conversations.
          </div>
        ) : (
          filteredContacts.map((contact) => {
            const isSelected =
              activeContactId === contact.id ||
              activeContactId === contact.conversation_id
            const status = connectionStatus(
              agentForContact(contact, agents),
              now,
            )
            const warning =
              contact.type === 'agent' &&
              [
                'disabled',
                'login_required',
                'quota_limited',
                'unavailable',
              ].includes(status.state)
            const id = contact.conversation_id || contact.id
            const unread = archive.unreadCount(id, contact.unread_count, contact.last_message_at || contact.updated_at)
            const openContact = () => {
              onSelectContact(contact)
              void archive.markRead(id, true, contact.last_message_at).catch(() => {})
            }
            const archiveAction = { label: showArchived ? 'Restore conversation' : 'Archive conversation', disabled: !archive.loaded, run: () => archive.archive(id, !showArchived) }
            return (
              <ItemActions key={contact.id} label={contact.name} swipeAction={archiveAction} actions={[
                { label: 'Open conversation', run: openContact },
                { label: unread ? 'Mark as read' : 'Mark as unread', disabled: !archive.loaded, run: () => archive.markRead(id, unread > 0) },
                { label: 'Copy link', run: () => copyItemLink('/inbox/' + encodeURIComponent(id)) },
                archiveAction,
              ]}>
              <button
                key={contact.id}
                onClick={openContact}
                className={cn(
                  'inbox-contact-row w-full min-h-20 text-left px-3 py-3 transition-colors flex items-center gap-3 hover:bg-accent/40 focus-visible:outline focus-visible:outline-2 focus-visible:outline-ring focus-visible:-outline-offset-2',
                  isSelected && 'bg-accent text-accent-foreground font-medium',
                )}
              >
                {/* Avatar with Online Dot */}
                <div className="relative shrink-0 mt-0.5">
                  <div className="h-11 w-11 rounded-full bg-secondary flex items-center justify-center text-sm font-semibold border border-border shadow-xs">
                    {contact.avatar ? (
                      <span>{contact.avatar}</span>
                    ) : contact.type === 'agent' ? (
                      <Bot className="h-4 w-4 text-primary" />
                    ) : (
                      <User className="h-4 w-4 text-muted-foreground" />
                    )}
                  </div>
                </div>

                {/* Details */}
                <div className="flex-1 min-w-0">
                  <div className="flex items-center justify-between gap-1 mb-0.5">
                    <span className={cn("text-sm text-foreground truncate", unread ? "font-semibold" : "font-medium")}>
                      {contact.name}
                    </span>
                    <span className={cn("text-xs shrink-0 tabular-nums", unread ? "text-primary font-medium" : "text-muted-foreground")}>
                      {formatTime(
                        contact.last_message_at || contact.updated_at,
                      )}
                    </span>
                  </div>

                  {contact.agent_name &&
                    contact.name !== contact.agent_name && (
                      <p className="truncate text-xs text-muted-foreground">
                        {contact.agent_name}
                      </p>
                    )}
                  <div className="flex items-center justify-between gap-2">
                    <p className="text-sm text-muted-foreground truncate leading-relaxed">
                      {conversationPreview(contact.last_message)}
                    </p>
                    {unread > 0 && (
                      <span
                        aria-label={`${unread} unread messages`}
                        className="shrink-0 h-4 min-w-4 px-1 rounded-full bg-primary text-primary-foreground text-xs font-bold flex items-center justify-center"
                      >
                        {unread}
                      </span>
                    )}
                  </div>
                  {warning && (
                    <p className="mt-1 text-xs text-amber-700 dark:text-amber-400">
                      {status.label}
                    </p>
                  )}
                </div>
              </button>
              </ItemActions>
            )
          })
        )}
      </div>
      <ArchiveNotice state={archive} />
    </div>
  )
}
