import { ListCreateAction } from '../../components/layout/ListCreateAction'
import { HairlineList } from '../../components/layout/HairlineList'
import { StatusDot } from '../../components/ui/StatusDot'
import { ItemActions, copyItemLink } from '../../components/ui/ItemActions'
import { useArchivedItems, ArchiveNotice } from '../../hooks/useArchivedItems'
import { useConnectionClock } from '../agents/useConnectionClock'
import { useLanguage } from '../../context/LanguageContext'
import type { Agent } from '../../api/agents'
import { agentForContact, connectionStatus } from '../agents/connection'
import { useState, useEffect } from 'react'
import { Link, useNavigate } from 'react-router-dom'
import { ChatContact } from '../../api/chat'
import { Search, Bot, User, Archive, CheckCheck } from 'lucide-react'
import { cn } from '../../lib/utils'
import './chat-calm.css'

export function conversationPreview(message?: string) {
  if (!message?.trim()) return 'No messages yet.'
  let text = message.trim()

  // Clean raw JSON strings (e.g. {"decision": "revise", ...}) to friendly summary text
  try {
    const rawCandidate = text.replace(/^\s*```(?:json)?\s*/i, '').replace(/\s*```\s*$/, '').trim()
    if ((rawCandidate.startsWith('{') && rawCandidate.endsWith('}')) || (rawCandidate.startsWith('[') && rawCandidate.endsWith(']'))) {
      const parsed = JSON.parse(rawCandidate)
      if (parsed && typeof parsed === 'object') {
        if ('decision' in parsed) {
          const dec = String(parsed.decision).toLowerCase()
          const reason = parsed.reason || parsed.feedback || parsed.notes || parsed.message || ''
          if (dec === 'approve' || dec === 'approved') {
            return reason ? `Disetujui: ${reason}` : 'Persetujuan diberikan'
          }
          if (dec === 'revise' || dec === 'revision') {
            return reason ? `Revisi: ${reason}` : 'Permintaan revisi'
          }
          if (dec === 'reject' || dec === 'rejected') {
            return reason ? `Ditolak: ${reason}` : 'Keputusan: Ditolak'
          }
          return `Keputusan: ${parsed.decision}${reason ? ` · ${reason}` : ''}`
        }
        if (parsed.summary) return String(parsed.summary)
        if (parsed.plan) return String(parsed.plan)
        if (parsed.title) return String(parsed.title)
        if (parsed.message) return String(parsed.message)
        if (parsed.text) return String(parsed.text)
        if (parsed.description) return String(parsed.description)
        if (parsed.action) return `Aksi: ${parsed.action}`
      }
    }
  } catch {
    // Regex clean-up if JSON is embedded in text
    text = text.replace(/\{[^{}]*"decision"\s*:\s*"([^"]+)"[^{}]*\}/g, (_m, dec) => `Keputusan: ${dec}`)
    text = text.replace(/\{[^{}]*"summary"\s*:\s*"([^"]+)"[^{}]*\}/g, '$1')
  }

  // Strip residual raw JSON string blocks like {"key": "value", ...}
  text = text.replace(/\{[^{}]*"(?:decision|action|plan|status|reason|type)"\s*:[^{}]*\}/gi, ' ')
  text = text.replace(/\{[^{}]*"[a-zA-Z0-9_]+"\s*:[^{}]*\}/g, ' ')

  return (
    text
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

export function formatConversationTime(iso?: string, language: string = 'en') {
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

export interface ConversationItemProps {
  contact: ChatContact
  isSelected: boolean
  isLast: boolean
  now: number
  agents?: Agent[]
  archive: ReturnType<typeof useArchivedItems>
  showArchived: boolean
  onSelectContact: (contact: ChatContact) => void
  language?: string
}

export function ConversationItem({
  contact,
  isSelected,
  isLast,
  now,
  agents = [],
  archive,
  showArchived,
  onSelectContact,
  language,
}: ConversationItemProps) {
  const { t } = useLanguage()
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

  const rawAgent = contact.agent_name || ''
  const formattedAgent = rawAgent
    ? (rawAgent.startsWith('@')
        ? rawAgent
        : (rawAgent.startsWith('agent_')
            ? '@' + rawAgent.slice(6)
            : '@' + rawAgent))
    : ''
  const showAgentName = Boolean(formattedAgent && contact.name !== formattedAgent && contact.name !== rawAgent)

  const subject =
    contact.product ||
    (showAgentName ? formattedAgent : null) ||
    contact.page_title ||
    (contact.type === 'agent' ? (formattedAgent || t('Asisten AI')) : t('Percakapan'))

  const openContact = () => {
    onSelectContact(contact)
    void archive.markRead(id, true, contact.last_message_at).catch(() => {})
  }
  const archiveAction = {
    label: showArchived ? 'Restore conversation' : 'Archive conversation',
    previewLabel: showArchived ? 'Restore' : 'Archive',
    disabled: !archive.loaded,
    run: () => archive.archive(id, !showArchived),
  }

  return (
    <ItemActions
      key={contact.id}
      label={contact.name}
      swipeAction={archiveAction}
      className={cn('inbox-item-row border-b border-border/40', !isLast && 'has-divider')}
      actions={[
        { label: 'Open conversation', run: openContact },
        { label: unread ? 'Mark as read' : 'Mark as unread', disabled: !archive.loaded, run: () => archive.markRead(id, unread > 0) },
        { label: 'Copy link', run: () => copyItemLink('/inbox/' + encodeURIComponent(id)) },
        archiveAction,
      ]}
    >
      <button
        type="button"
        onClick={openContact}
        aria-pressed={isSelected}
        data-testid="conversation-item"
        className={cn(
          'inbox-contact-row w-full min-h-[4.5rem] text-left px-3.5 py-3 transition-colors flex items-start gap-3 hover:bg-accent/40 focus-visible:outline focus-visible:outline-2 focus-visible:outline-ring focus-visible:-outline-offset-2',
          isSelected && 'bg-accent/70 text-accent-foreground font-medium',
        )}
      >
        {/* Avatar with StatusDot micro-indicator */}
        <div className="relative shrink-0 mt-0.5">
          <div className="h-9 w-9 rounded-full bg-secondary/80 flex items-center justify-center text-sm font-semibold border border-border/60 shadow-xs">
            {contact.avatar ? (
              <span>{contact.avatar}</span>
            ) : contact.type === 'agent' ? (
              <Bot className="h-4 w-4 text-primary" />
            ) : (
              <User className="h-4 w-4 text-muted-foreground" />
            )}
          </div>
          {contact.type === 'agent' ? (
            <div className="absolute -bottom-0.5 -right-0.5">
              <StatusDot
                status={warning ? 'exception' : status.state === 'ready' || status.state === 'online' ? 'healthy' : 'neutral'}
                pulse={status.state === 'running'}
                size="sm"
              />
            </div>
          ) : contact.is_online ? (
            <div className="absolute -bottom-0.5 -right-0.5">
              <StatusDot status="healthy" size="sm" title={t('Online on website')} />
            </div>
          ) : null}
        </div>

        {/* 3-Tier Apple Mail & Bear Inset List Content */}
        <div className="flex-1 min-w-0">
          {/* Tingkat 1: Pengirim + timestamp tabular-nums di kanan */}
          <div className="flex items-center justify-between gap-1.5 mb-0.5">
            <div className="flex items-center gap-1.5 min-w-0 truncate">
              {unread > 0 && (
                <span
                  aria-label={`${unread} unread messages`}
                  className="h-2 w-2 shrink-0 rounded-full bg-primary"
                  title={`${unread} unread`}
                />
              )}
              <span className={cn("text-sm text-foreground truncate", unread ? "font-semibold" : "font-medium")}>
                {contact.name}
              </span>
            </div>
            <span className={cn("text-xs shrink-0 tabular-nums", unread ? "text-primary font-medium" : "text-muted-foreground")}>
              {formatConversationTime(contact.last_message_at || contact.updated_at, language)}
            </span>
          </div>

          {/* Tingkat 2: Subjek / Judul tipografi bersih Geist & Status-by-Exception */}
          <div className="flex items-center justify-between gap-2 mb-1 min-w-0">
            <span className={cn("text-xs truncate font-medium", unread ? "text-foreground font-semibold" : "text-foreground/80")}>
              {subject}
            </span>
            {/* Active task micro-indicator & unread count badge */}
            <div data-testid="task-count-badge" className="flex items-center gap-1.5 shrink-0">
              {activeTasks > 0 && (
                <span
                  data-testid="task-badge-active"
                  className="inline-flex shrink-0 items-center gap-1.5"
                  title={`${activeTasks} ${activeTasks === 1 ? t('task') : t('tasks')} ${t('aktif')}`}
                  aria-label={`${activeTasks} active tasks`}
                >
                  <StatusDot status="working" size="xs" pulse aria-label={`${activeTasks} active tasks`} />
                  <span className="text-[10px] font-medium text-blue-600 dark:text-blue-400">
                    {activeTasks} {t('bekerja')}
                  </span>
                </span>
              )}
              {unread > 0 && (
                <span
                  aria-label={`${unread} unread messages`}
                  className="shrink-0 h-4 min-w-4 px-1 rounded-full bg-primary text-primary-foreground text-[10px] font-bold flex items-center justify-center tabular-nums"
                >
                  {unread}
                </span>
              )}
            </div>
          </div>

          {/* Tingkat 3: Cuplikan 2 baris line-clamp-2 warna muted zinc-400 */}
          <p className="text-xs text-muted-foreground text-zinc-500 dark:text-zinc-400 line-clamp-2 leading-relaxed">
            {conversationPreview(contact.last_message)}
          </p>

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
}

export type ContactFilterTab = 'all' | 'agents' | 'customers'

interface InboxSubPanelProps {
  onNewMessage?: () => void
  onNewStudio?: () => void
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
  onNewStudio,
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


  useEffect(() => {
    const handleKeyDown = (e: KeyboardEvent) => {
      if ((e.metaKey || e.ctrlKey) && e.key.toLowerCase() === 'k') {
        e.preventDefault()
        if (onNewStudio) onNewStudio()
        else if (onNewMessage) onNewMessage()
      }
    }
    window.addEventListener('keydown', handleKeyDown)
    return () => window.removeEventListener('keydown', handleKeyDown)
  }, [onNewStudio, onNewMessage])

  return (
    <div className="flex flex-col h-full inbox-list text-foreground select-none">
      <div className="px-3 pt-2 pb-0 border-b border-border/40 space-y-1.5">
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

        {/* Filter Chips (All, Agents, Customers, Archived) - Calm Things 3 / Bear Notes Segments */}
        <div className="flex items-center gap-1.5 overflow-x-auto py-1 [scrollbar-width:none] [-ms-overflow-style:none] [&::-webkit-scrollbar]:hidden" role="group" aria-label={t('Conversation filters')}>
          <button
            type="button"
            aria-pressed={activeTab === 'all'}
            onClick={() => handleTabClick('all')}
            className={cn(
              'inline-flex min-h-11 shrink-0 items-center justify-center gap-1.5 rounded-full px-3.5 py-1.5 text-xs font-medium transition-all focus-visible:outline-2 focus-visible:outline-ring',
              activeTab === 'all'
                ? 'bg-secondary text-foreground font-semibold shadow-none border border-border/60'
                : 'bg-muted/40 text-muted-foreground hover:bg-muted/70 hover:text-foreground border border-transparent',
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
                ? 'bg-secondary text-foreground font-semibold shadow-none border border-border/60'
                : 'bg-muted/40 text-muted-foreground hover:bg-muted/70 hover:text-foreground border border-transparent',
            )}
          >
            <span>{t('Agents')}</span>
            <span
              className={cn(
                'inline-flex min-w-4 items-center justify-center rounded-full px-1.5 text-[10px] font-mono leading-none',
                activeTab === 'agents'
                  ? 'bg-background/80 text-foreground font-semibold'
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
                ? 'bg-secondary text-foreground font-semibold shadow-none border border-border/60'
                : 'bg-muted/40 text-muted-foreground hover:bg-muted/70 hover:text-foreground border border-transparent',
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
                  ? 'bg-background/80 text-foreground font-semibold'
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
                ? 'bg-secondary text-foreground font-semibold shadow-none border border-border/60'
                : 'bg-muted/40 text-muted-foreground hover:bg-muted/70 hover:text-foreground border border-transparent',
            )}
          >
            <Archive className="h-3.5 w-3.5" aria-hidden="true" />
            {contacts.filter(c => archive.isArchived(c.conversation_id || c.id, c.last_message_at || c.updated_at)).length > 0 && (
              <span
                className={cn(
                  'inline-flex min-w-4 items-center justify-center rounded-full px-1.5 text-[10px] font-mono leading-none',
                  showArchived
                    ? 'bg-background/80 text-foreground font-semibold'
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
                  ? 'bg-secondary text-foreground font-semibold border border-border/50'
                  : 'bg-muted/40 text-muted-foreground hover:text-foreground hover:bg-muted/60'
              )}
            >
              <span>{t('Chats')}</span>
              <span className={cn(
                'text-[10px] px-1.5 py-0.2 rounded-full tabular-nums',
                customerSubFilter === 'chats' ? 'bg-background/80 text-foreground font-semibold' : 'bg-background/60 text-muted-foreground'
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
                  ? 'bg-emerald-600/20 text-emerald-400 font-semibold border border-emerald-500/30'
                  : 'bg-muted/40 text-muted-foreground hover:text-foreground hover:bg-muted/60'
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
                customerSubFilter === 'online' ? 'bg-emerald-500/20 text-emerald-300 font-semibold' : 'bg-background/60 text-muted-foreground'
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
          <HairlineList className="divide-y divide-border/40">
            {filteredContacts.map((contact, index) => {
              const isSelected =
                activeContactId === contact.id ||
                activeContactId === contact.conversation_id
              const isLast = index === filteredContacts.length - 1
              return (
                <ConversationItem
                  key={contact.id}
                  contact={contact}
                  isSelected={isSelected}
                  isLast={isLast}
                  now={now}
                  agents={agents}
                  archive={archive}
                  showArchived={showArchived}
                  onSelectContact={onSelectContact}
                  language={language}
                />
              )
            })}
          </HairlineList>
        )}
      </div>
      <ArchiveNotice state={archive} />
    </div>
  )
}
