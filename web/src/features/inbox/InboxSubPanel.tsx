import { ListCreateAction } from '../../components/layout/ListCreateAction'
import { ItemActions, copyItemLink } from '../../components/ui/ItemActions'
import { useArchivedItems, ArchiveNotice } from '../../hooks/useArchivedItems'
import { useConnectionClock } from '../agents/useConnectionClock'
import { useLanguage } from '../../context/LanguageContext'
import type { Agent } from '../../api/agents'
import { agentForContact, connectionStatus } from '../agents/connection'
import { useState } from 'react'
import { Link, useNavigate } from 'react-router-dom'
import { ChatContact } from '../../api/chat'
import { Search, Bot, User, Archive, CheckCheck, CheckSquare, Zap, XCircle } from 'lucide-react'
import { cn } from '../../lib/utils'
import './chat-whatsapp.css'

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
  onNewMessage?: () => void
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
  onNewMessage,
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
  const navigate = useNavigate()
  const archive = useArchivedItems('conversation')
  const [showArchived, setShowArchived] = useState(false)
  const clock = useConnectionClock(agents)
  const now = suppliedNow ?? clock
  const { t, language } = useLanguage()

  const [internalTab, setInternalTab] = useState<ContactFilterTab>(selectedTab)
  const [internalQuery, setInternalQuery] = useState(searchQuery)
  const [customerSubFilter, setCustomerSubFilter] = useState<'chats' | 'online'>('chats')

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
  const activeCustomerCount = contacts.filter((c) => c.type === 'customer' && c.has_messages !== false).length
  const onlineVisitorCount = contacts.filter((c) => c.type === 'customer' && c.is_online).length

  const [markingAll, setMarkingAll] = useState(false)
  const totalUnreadCount = contacts.reduce(
    (sum, c) => sum + archive.unreadCount(c.conversation_id || c.id, c.unread_count, c.last_message_at || c.updated_at),
    0
  )
  const handleMarkAllRead = async () => {
    if (markingAll) return
    setMarkingAll(true)
    try {
      await archive.markAllRead()
    } catch {
      // ignore
    } finally {
      setMarkingAll(false)
    }
  }

  const filteredContacts = contacts.filter((c) => {
    if (archive.isArchived(c.conversation_id || c.id, c.last_message_at || c.updated_at) !== showArchived) return false
    if (activeTab === 'all') {
      if (c.type === 'customer' && c.has_messages === false) return false
    }
    if (activeTab === 'agents' && c.type !== 'agent') return false
    if (activeTab === 'customers') {
      if (c.type !== 'customer') return false
      if (customerSubFilter === 'chats' && c.has_messages === false) return false
      if (customerSubFilter === 'online' && !c.is_online) return false
    }
    if (query.trim()) {
      const q = query.toLowerCase()
      const matchName = [c.name, c.agent_name]
        .join(' ')
        .toLowerCase()
        .includes(q)
      const matchMsg = (c.last_message || '').toLowerCase().includes(q)
      const matchProduct = (c.product || '').toLowerCase().includes(q)
      const matchUrl = (c.current_url || '').toLowerCase().includes(q)
      const matchPage = (c.page_title || '').toLowerCase().includes(q)
      return matchName || matchMsg || matchProduct || matchUrl || matchPage
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
        <ListCreateAction label="New message" kind="inbox" onClick={onNewMessage} />
        {/* Search Bar */}
        <ItemActions
          label="Inbox"
          actions={[
            ...(totalUnreadCount > 0 ? [{ label: t('Mark all as read'), run: handleMarkAllRead }] : []),
            { label: 'Live Chat settings', run: () => navigate('/settings/integrations/live-chat') },
          ]}
        >
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

        </ItemActions>

        {/* Filter Chips (All, Agents, Customers, Archived) */}
        <div className="flex items-center gap-1.5 overflow-x-auto py-1 [scrollbar-width:none] [-ms-overflow-style:none] [&::-webkit-scrollbar]:hidden" role="group" aria-label={t('Conversation filters')}>
          <button
            type="button"
            aria-pressed={activeTab === 'all'}
            onClick={() => handleTabClick('all')}
            className={cn(
              'inline-flex min-h-11 shrink-0 items-center justify-center gap-1.5 rounded-full px-3.5 py-1.5 text-xs font-medium transition-all focus-visible:outline-2 focus-visible:outline-ring',
              activeTab === 'all'
                ? 'bg-primary text-primary-foreground font-semibold shadow-sm'
                : 'bg-muted/60 text-muted-foreground hover:bg-muted hover:text-foreground border border-border/40',
            )}
          >
            <span>{t('All')}</span>
          </button>
          <button
            type="button"
            aria-pressed={activeTab === 'agents'}
            onClick={() => handleTabClick('agents')}
            className={cn(
              'inline-flex min-h-11 shrink-0 items-center justify-center gap-1.5 rounded-full px-3.5 py-1.5 text-xs font-medium transition-all focus-visible:outline-2 focus-visible:outline-ring',
              activeTab === 'agents'
                ? 'bg-primary text-primary-foreground font-semibold shadow-sm'
                : 'bg-muted/60 text-muted-foreground hover:bg-muted hover:text-foreground border border-border/40',
            )}
          >
            <span>{t('Agents')}</span>
            <span
              className={cn(
                'inline-flex min-w-4 items-center justify-center rounded-full px-1.5 text-[10px] font-mono leading-none',
                activeTab === 'agents'
                  ? 'bg-primary-foreground/20 text-primary-foreground font-semibold'
                  : 'bg-muted-foreground/15 text-muted-foreground',
              )}
            >
              {agentCount}
            </span>
          </button>
          <button
            type="button"
            aria-pressed={activeTab === 'customers'}
            onClick={() => handleTabClick('customers')}
            className={cn(
              'inline-flex min-h-11 shrink-0 items-center justify-center gap-1.5 rounded-full px-3.5 py-1.5 text-xs font-medium transition-all focus-visible:outline-2 focus-visible:outline-ring',
              activeTab === 'customers'
                ? 'bg-primary text-primary-foreground font-semibold shadow-sm'
                : 'bg-muted/60 text-muted-foreground hover:bg-muted hover:text-foreground border border-border/40',
            )}
          >
            <span>{t('Customers')}</span>
            {onlineVisitorCount > 0 && (
              <span className="relative flex h-2 w-2" title={`${onlineVisitorCount} ${t('online')}`}>
                <span className="animate-ping absolute inline-flex h-full w-full rounded-full bg-emerald-400 opacity-75"></span>
                <span className="relative inline-flex rounded-full h-2 w-2 bg-emerald-500"></span>
              </span>
            )}
            <span
              className={cn(
                'inline-flex min-w-4 items-center justify-center rounded-full px-1.5 text-[10px] font-mono leading-none',
                activeTab === 'customers'
                  ? 'bg-primary-foreground/20 text-primary-foreground font-semibold'
                  : 'bg-muted-foreground/15 text-muted-foreground',
              )}
            >
              {customerCount}
            </span>
          </button>
          <button
            type="button"
            aria-label={t(showArchived ? 'Back to Inbox' : 'Archived conversations')}
            title={t(showArchived ? 'Back to Inbox' : 'Archived conversations')}
            aria-pressed={showArchived}
            onClick={() => setShowArchived(value => !value)}
            className={cn(
              'inline-flex min-h-11 min-w-11 shrink-0 items-center justify-center gap-1.5 rounded-full px-3 py-1.5 text-xs font-medium transition-all focus-visible:outline-2 focus-visible:outline-ring',
              showArchived
                ? 'bg-primary text-primary-foreground font-semibold shadow-sm'
                : 'bg-muted/60 text-muted-foreground hover:bg-muted hover:text-foreground border border-border/40',
            )}
          >
            <Archive className="h-3.5 w-3.5" aria-hidden="true" />
            {contacts.filter(c => archive.isArchived(c.conversation_id || c.id, c.last_message_at || c.updated_at)).length > 0 && (
              <span
                className={cn(
                  'inline-flex min-w-4 items-center justify-center rounded-full px-1.5 text-[10px] font-mono leading-none',
                  showArchived
                    ? 'bg-primary-foreground/20 text-primary-foreground font-semibold'
                    : 'bg-muted-foreground/15 text-muted-foreground',
                )}
              >
                {contacts.filter(c => archive.isArchived(c.conversation_id || c.id, c.last_message_at || c.updated_at)).length}
              </span>
            )}
          </button>
          {totalUnreadCount > 0 && (
            <button
              type="button"
              disabled={markingAll}
              aria-label={t('Mark all as read')}
              title={t('Mark all as read')}
              onClick={handleMarkAllRead}
              className={cn(
                'min-h-11 min-w-11 flex items-center justify-center gap-1 border-b-2 text-xs border-transparent text-muted-foreground hover:text-primary transition-colors',
                markingAll && 'opacity-50 cursor-not-allowed'
              )}
            >
              <CheckCheck className="h-4 w-4 text-primary" aria-hidden="true" />
              <span className="sr-only">{t('Mark all as read')}</span>
            </button>
          )}
        </div>

        {/* Customer Sub-Filter (Chats vs Online Visitors) */}
        {activeTab === 'customers' && (
          <div className="flex items-center gap-1.5 pt-1 pb-2">
            <button
              type="button"
              onClick={() => setCustomerSubFilter('chats')}
              className={cn(
                'px-2.5 py-1 rounded-full text-xs font-medium transition-colors flex items-center gap-1.5',
                customerSubFilter === 'chats'
                  ? 'bg-primary text-primary-foreground shadow-xs'
                  : 'bg-muted/50 text-muted-foreground hover:text-foreground hover:bg-muted'
              )}
            >
              <span>{t('Chats')}</span>
              <span className={cn(
                'text-[10px] px-1.5 py-0.2 rounded-full tabular-nums',
                customerSubFilter === 'chats' ? 'bg-primary-foreground/20 text-primary-foreground' : 'bg-background/80 text-muted-foreground'
              )}>
                {activeCustomerCount}
              </span>
            </button>

            <button
              type="button"
              onClick={() => setCustomerSubFilter('online')}
              className={cn(
                'px-2.5 py-1 rounded-full text-xs font-medium transition-colors flex items-center gap-1.5',
                customerSubFilter === 'online'
                  ? 'bg-emerald-600 text-white shadow-xs'
                  : 'bg-muted/50 text-muted-foreground hover:text-foreground hover:bg-muted'
              )}
            >
              <span className="relative flex h-2 w-2">
                {onlineVisitorCount > 0 && (
                  <span className="animate-ping absolute inline-flex h-full w-full rounded-full bg-emerald-400 opacity-75"></span>
                )}
                <span className="relative inline-flex rounded-full h-2 w-2 bg-emerald-500"></span>
              </span>
              <span>{t('Online')}</span>
              <span className={cn(
                'text-[10px] px-1.5 py-0.2 rounded-full tabular-nums',
                customerSubFilter === 'online' ? 'bg-white/20 text-white' : 'bg-background/80 text-muted-foreground'
              )}>
                {onlineVisitorCount}
              </span>
            </button>
          </div>
        )}
      </div>

      {archive.error && <div role="alert" className="px-3 text-xs">{t(archive.error)} <button className="min-h-11 underline" onClick={() => void archive.refresh()}>{t('Try again')}</button></div>}
      {/* Contacts List */}
      <div className="flex-1 overflow-y-auto">
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
            {t(
              !query.trim() && !showArchived && activeTab === 'customers' && customerSubFilter === 'online'
                ? 'No active online visitors right now.'
                : !query.trim() && !showArchived && activeTab === 'customers'
                ? 'No customer conversations yet.'
                : showArchived && !query.trim()
                ? 'No archived conversations.'
                : 'No matching conversations.'
            )}
            {!query.trim() && !showArchived && customerCount === 0 && activeTab !== 'agents' && (
              <div className="mt-3 space-y-2">
                <p>{t('Add live chat to your website to receive messages here.')}</p>
                <Link className="action-secondary inline-flex" to="/settings/integrations/live-chat">
                  {t('Connect Live Chat')}
                </Link>
              </div>
            )}
          </div>
        ) : (
          filteredContacts.map((contact, index) => {
            const isSelected =
              activeContactId === contact.id ||
              activeContactId === contact.conversation_id
            const isLast = index === filteredContacts.length - 1
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
            const activeTasks = typeof contact.active_tasks === 'number' ? contact.active_tasks : Number(contact.active_tasks) || 0
            const totalTasks = typeof contact.task_count === 'number' ? contact.task_count : Number(contact.task_count) || 0
            const failedTasks = typeof contact.failed_tasks === 'number' ? contact.failed_tasks : Number(contact.failed_tasks) || 0
            const doneTasks = typeof contact.done_tasks === 'number'
              ? contact.done_tasks
              : Math.max(0, totalTasks - activeTasks - failedTasks)
            const taskBadgeCount = activeTasks > 0 ? activeTasks : (doneTasks > 0 ? doneTasks : totalTasks)
            const hasTasks = totalTasks > 0 || activeTasks > 0 || doneTasks > 0 || failedTasks > 0

            const rawAgent = contact.agent_name || ''
            const formattedAgent = rawAgent
              ? (rawAgent.startsWith('@')
                  ? rawAgent
                  : (rawAgent.startsWith('agent_')
                      ? '@' + rawAgent.slice(6)
                      : '@' + rawAgent))
              : ''
            const showAgentName = Boolean(formattedAgent && contact.name !== formattedAgent && contact.name !== rawAgent)

            const openContact = () => {
              onSelectContact(contact)
              void archive.markRead(id, true, contact.last_message_at).catch(() => {})
            }
            const archiveAction = { label: showArchived ? 'Restore conversation' : 'Archive conversation', previewLabel: showArchived ? 'Restore' : 'Archive', disabled: !archive.loaded, run: () => archive.archive(id, !showArchived) }
            return (
              <ItemActions key={contact.id} label={contact.name} swipeAction={archiveAction} className={cn('inbox-item-row', !isLast && 'has-divider')} actions={[
                { label: 'Open conversation', run: openContact },
                { label: unread ? 'Mark as read' : 'Mark as unread', disabled: !archive.loaded, run: () => archive.markRead(id, unread > 0) },
                { label: 'Copy link', run: () => copyItemLink('/inbox/' + encodeURIComponent(id)) },
                archiveAction,
              ]}>
              <button
                key={contact.id}
                onClick={openContact}
                aria-pressed={isSelected}
                className={cn(
                  'inbox-contact-row w-full min-h-[4.5rem] text-left px-3 py-2.5 transition-colors flex items-center gap-3 hover:bg-accent/40 focus-visible:outline focus-visible:outline-2 focus-visible:outline-ring focus-visible:-outline-offset-2',
                  isSelected && 'bg-accent text-accent-foreground font-medium',
                )}
              >
                {/* Avatar with Online Dot */}
                <div className="relative shrink-0 mt-0.5">
                  <div className="h-9 w-9 rounded-full bg-secondary flex items-center justify-center text-sm font-semibold border border-border shadow-xs">
                    {contact.avatar ? (
                      <span>{contact.avatar}</span>
                    ) : contact.type === 'agent' ? (
                      <Bot className="h-4 w-4 text-primary" />
                    ) : (
                      <User className="h-4 w-4 text-muted-foreground" />
                    )}
                  </div>
                  {contact.is_online && (
                    <span
                      className="absolute bottom-0 right-0 block h-2.5 w-2.5 rounded-full bg-emerald-500 ring-2 ring-background"
                      title={t('Online on website')}
                    />
                  )}
                </div>

                {/* Details */}
                <div className="flex-1 min-w-0">
                  <div className="flex items-center justify-between gap-1.5 mb-0.5">
                    <span className={cn("text-sm text-foreground truncate", unread ? "font-semibold" : "font-normal")}>
                      {contact.name}
                    </span>
                    <span className={cn("text-xs shrink-0 tabular-nums", unread ? "text-primary font-medium" : "text-muted-foreground")}>
                      {formatTime(
                        contact.last_message_at || contact.updated_at,
                      )}
                    </span>
                  </div>

                  {/* Second Line: Agent Handle & Smart Task Badges */}
                  {(showAgentName || hasTasks) && (
                    <div data-testid="task-count-badge" className="flex items-center gap-1.5 mb-0.5 min-w-0 flex-wrap">
                      {showAgentName && (
                        <span className="text-xs text-muted-foreground font-medium truncate shrink-0">
                          {formattedAgent}
                        </span>
                      )}
                      {activeTasks > 0 && (
                        <span
                          data-testid="task-badge-active"
                          aria-label={`${activeTasks} active tasks`}
                          title={`${activeTasks} active ${activeTasks === 1 ? t('task') : t('tasks')}`}
                          className="inline-flex shrink-0 items-center gap-1 px-1.5 py-0.5 rounded-full text-[10px] font-bold tabular-nums leading-none border shadow-xs transition-colors bg-blue-500/15 text-blue-600 dark:text-blue-400 border-blue-500/25"
                        >
                          <Zap className="h-2.5 w-2.5 fill-current" aria-hidden="true" />
                          <span>{activeTasks}</span>
                        </span>
                      )}
                      {failedTasks > 0 && (
                        <span
                          data-testid="task-badge-failed"
                          aria-label={`${failedTasks} failed or blocked tasks`}
                          title={`${failedTasks} blocked or failed ${failedTasks === 1 ? t('task') : t('tasks')}`}
                          className="inline-flex shrink-0 items-center gap-1 px-1.5 py-0.5 rounded-full text-[10px] font-bold tabular-nums leading-none border shadow-xs transition-colors bg-rose-500/15 text-rose-600 dark:text-rose-400 border-rose-500/25"
                        >
                          <XCircle className="h-2.5 w-2.5" aria-hidden="true" />
                          <span>{failedTasks}</span>
                        </span>
                      )}
                      {doneTasks > 0 && (
                        <span
                          data-testid="task-badge-done"
                          aria-label={`${doneTasks} completed tasks`}
                          title={`${doneTasks} completed ${doneTasks === 1 ? t('task') : t('tasks')}`}
                          className="inline-flex shrink-0 items-center gap-1 px-1.5 py-0.5 rounded-full text-[10px] font-bold tabular-nums leading-none border shadow-xs transition-colors bg-emerald-500/15 text-emerald-600 dark:text-emerald-400 border-emerald-500/25"
                        >
                          <CheckSquare className="h-2.5 w-2.5" aria-hidden="true" />
                          <span>{doneTasks}</span>
                        </span>
                      )}
                    </div>
                  )}
                  <div className="flex items-center justify-between gap-2">
                    <p className="text-xs text-muted-foreground truncate leading-5">
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
              {!isLast && (
                <div
                  aria-hidden="true"
                  className="inbox-item-divider"
                />
              )}
              </ItemActions>
            )
          })
        )}
      </div>
      <ArchiveNotice state={archive} />
    </div>
  )
}
