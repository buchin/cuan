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
import { CuanMark } from '../../components/brand/CuanMark'
import { ChatContact } from '../../api/chat'
import { Search, Bot, User, Archive, CheckCheck, Plus } from 'lucide-react'
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
  const id = contact.conversation_id || contact.id
  const unread = archive.unreadCount(id, contact.unread_count, contact.last_message_at || contact.updated_at)
  const activeTasks = typeof contact.active_tasks === 'number' ? contact.active_tasks : Number(contact.active_tasks) || 0
  const hasActiveWork = activeTasks > 0
  const isAgentConnectionIssue =
    contact.type === 'agent' &&
    !hasActiveWork &&
    ['login_required', 'quota_limited', 'unavailable'].includes(status.state)

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

  const isSubjectRedundant =
    !subject ||
    subject === formattedAgent ||
    subject === rawAgent ||
    subject === contact.name ||
    subject === t('Percakapan') ||
    subject === t('Asisten AI')

  return (
    <ItemActions
      key={contact.id}
      label={contact.name}
      swipeAction={archiveAction}
      className={cn(
        'inbox-item-row border-b border-border/40 transition-colors hover:bg-accent/40',
        isSelected && 'bg-accent/70 text-accent-foreground font-medium',
        !isLast && 'has-divider',
      )}
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
          'inbox-contact-row flex items-start gap-3 w-full min-h-[4.5rem] p-3 text-left focus-visible:outline focus-visible:outline-2 focus-visible:outline-ring border-0 bg-transparent',
          isSelected && 'text-accent-foreground font-medium shadow-none',
        )}
      >
        {/* Avatar with Online Dot (inbox-app-01 style with circular rounded-full avatar & dot) */}
        <div className="relative shrink-0 mt-0.5">
          <div
            data-avatar="true"
            className="h-9 w-9 rounded-full bg-secondary/80 flex items-center justify-center font-bold text-xs border border-border/50 text-foreground overflow-hidden avatar"
          >
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
              className="absolute -bottom-0.5 -right-0.5 h-2.5 w-2.5 bg-emerald-500 rounded-full border-2 border-background status-dot"
              aria-label="Online"
              title="Online"
            />
          )}
        </div>

        {/* Content Body */}
        <div className="flex-1 min-w-0 flex flex-col gap-1">
          <div className="flex items-center justify-between gap-1.5">
            <div className="flex items-center gap-1.5 min-w-0">
              <span className={cn("text-xs truncate", unread > 0 ? "font-semibold text-foreground" : "font-medium text-foreground/90")}>
                {contact.name}
              </span>
              {unread > 0 && (
                <span
                  aria-label={`${unread} unread messages`}
                  className="h-2 w-2 shrink-0 bg-blue-500"
                  title={`${unread} unread`}
                />
              )}
            </div>
            <span className={cn("text-[11px] shrink-0 font-mono tabular-nums", unread > 0 ? "text-primary font-medium" : "text-muted-foreground")}>
              {formatConversationTime(contact.last_message_at || contact.updated_at, language)}
            </span>
          </div>

          {!isSubjectRedundant && (
            <div className="text-xs font-medium text-foreground/80 truncate">
              {subject}
            </div>
          )}

          <p className="text-xs text-muted-foreground dark:text-zinc-400 line-clamp-2 leading-relaxed">
            {conversationPreview(contact.last_message)}
          </p>

          {(hasActiveWork || (showAgentName && contact.type === 'agent') || (contact.type === 'customer' && contact.product)) && (
            <div className="flex items-center gap-1.5 pt-0.5">
              {hasActiveWork && (
                <span
                  data-testid="task-badge-active"
                  aria-label={`${activeTasks} active tasks`}
                  className="inline-flex items-center gap-1 border border-blue-500/30 bg-blue-500/10 px-1.5 py-0.5 text-[10px] font-medium text-blue-400"
                >
                  <StatusDot status="working" size="xs" pulse />
                  <span data-testid="task-count-badge">{activeTasks} {t('bekerja')}</span>
                </span>
              )}
              {showAgentName && contact.type === 'agent' && (
                <span className="inline-flex items-center gap-1 border border-border/50 bg-muted/40 px-1.5 py-0.5 text-[10px] font-medium text-muted-foreground">
                  <Bot className="h-3 w-3 text-primary" />
                  <span>{contact.agent_name || 'agent'}</span>
                </span>
              )}
              {contact.product && (
                <span className="inline-flex items-center border border-border/50 bg-muted/40 px-1.5 py-0.5 text-[10px] font-medium text-muted-foreground truncate max-w-[120px]">
                  {contact.product}
                </span>
              )}
            </div>
          )}

          {isAgentConnectionIssue && (
            <p className="mt-0.5 text-[11px] text-amber-700 dark:text-amber-400">
              {status.label}
            </p>
          )}
        </div>

        {unread > 0 && (
          <span className="shrink-0 bg-primary text-primary-foreground font-mono font-bold text-[10px] px-1.5 py-0.2">
            {unread}
          </span>
        )}
      </button>
      {!isLast && (
        <div
          aria-hidden="true"
          className="inbox-item-divider border-b border-border/40"
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

  const customerCount = contacts.filter((c) => c.type === 'customer').length
  const activeCustomerCount = contacts.filter((c) => c.type === 'customer' && c.has_messages !== false).length
  const onlineVisitorCount = contacts.filter((c) => c.type === 'customer' && c.is_online).length

  const [markingAll, setMarkingAll] = useState(false)
  const totalUnreadCount = contacts.reduce(
    (sum, c) => sum + archive.unreadCount(c.conversation_id || c.id, c.unread_count, c.last_message_at || c.updated_at),
    0
  )
  const agentUnreadCount = contacts
    .filter((c) => c.type === 'agent')
    .reduce(
      (sum, c) => sum + archive.unreadCount(c.conversation_id || c.id, c.unread_count, c.last_message_at || c.updated_at),
      0
    )
  const customerUnreadCount = contacts
    .filter((c) => c.type === 'customer')
    .reduce(
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
      {/* ── Single Unified Header (matching shadcn Mail h-13) ── */}
      <div className="flex h-13 shrink-0 items-center justify-between border-b border-border/40 px-3 md:px-4">
        <div className="flex items-center gap-1.5 md:gap-2 min-w-0">
          <Link
            to="/chat"
            aria-label={t('Go to Chat')}
            className="icon-action md:hidden shrink-0 inline-flex items-center justify-center -ml-1"
          >
            <CuanMark decorative className="h-6 w-6 shrink-0" />
          </Link>
          <div className="flex items-center gap-1.5 min-w-0">
            <h1 className="page-title text-base md:text-sm font-semibold tracking-tight text-foreground truncate">
              {t('Inbox')}
            </h1>
            {totalUnreadCount > 0 && (
              <span className="font-mono text-xs text-muted-foreground font-normal">
                {totalUnreadCount}
              </span>
            )}
          </div>
        </div>
        <div className="flex items-center gap-1">
          {onNewMessage && (
            <button
              type="button"
              onClick={onNewMessage}
              aria-label={t('New message')}
              title={t('New message')}
              className="inline-flex h-8 w-8 items-center justify-center rounded-md text-muted-foreground hover:bg-muted hover:text-foreground transition-colors"
            >
              <Plus className="h-4 w-4" />
            </button>
          )}
          {totalUnreadCount > 0 && (
            <button
              type="button"
              disabled={markingAll}
              aria-label={t('Mark all as read')}
              title={t('Mark all as read')}
              onClick={handleMarkAllRead}
              className="inline-flex h-8 w-8 min-h-11 min-w-11 items-center justify-center rounded-md text-muted-foreground hover:bg-muted hover:text-foreground transition-colors"
            >
              <CheckCheck className="h-4 w-4 text-primary" />
            </button>
          )}
          <ItemActions
            label="Inbox"
            actions={[
              ...(totalUnreadCount > 0 ? [{ label: t('Mark all as read'), run: handleMarkAllRead }] : []),
              { label: 'Live Chat settings', run: () => navigate('/settings/integrations/live-chat') },
            ]}
          >
            <span className="sr-only">Actions</span>
          </ItemActions>
        </div>
      </div>

      {/* ── Search & Filter Toolbar ── */}
      <div className="p-3 pb-2 space-y-2 border-b border-border/40">
        <div className="relative w-full">
          <Search className="absolute left-2.5 top-2.5 h-4 w-4 text-muted-foreground pointer-events-none" />
          <input
            type="text"
            value={query}
            onChange={(e) => handleQueryChange(e.target.value)}
            aria-label={t('Search conversations')}
            placeholder="Search conversations…"
            className="min-h-11 pl-8 pr-3 py-2 text-base rounded-full inbox-search w-full md:min-h-8 md:h-8 md:text-xs md:py-1 md:rounded-none border border-input bg-background/60 text-foreground placeholder:text-muted-foreground focus:outline-none focus:ring-1 focus:ring-ring"
          />
        </div>

        {/* Filter Categories: All, Agents, Customers, Archived */}
        <div className="flex items-center gap-1 overflow-x-auto [scrollbar-width:none] [-ms-overflow-style:none] [&::-webkit-scrollbar]:hidden" role="group" aria-label={t('Conversation filters')}>
          <button
            type="button"
            aria-pressed={activeTab === 'all' && !showArchived}
            onClick={() => {
              setShowArchived(false)
              handleTabClick('all')
            }}
            className={cn(
              'inline-flex min-h-11 md:min-h-7 shrink-0 items-center gap-1 rounded-full px-3.5 py-1.5 md:rounded-md md:px-2.5 md:py-1 text-xs font-medium transition-colors',
              activeTab === 'all' && !showArchived
                ? 'bg-secondary text-foreground font-semibold shadow-xs'
                : 'text-muted-foreground hover:bg-muted hover:text-foreground',
            )}
          >
            <span>{t('All')}</span>
          </button>
          <button
            type="button"
            aria-pressed={activeTab === 'agents' && !showArchived}
            onClick={() => {
              setShowArchived(false)
              handleTabClick('agents')
            }}
            className={cn(
              'inline-flex min-h-11 md:min-h-7 shrink-0 items-center gap-1.5 rounded-full px-3.5 py-1.5 md:rounded-md md:px-2.5 md:py-1 text-xs font-medium transition-colors',
              activeTab === 'agents' && !showArchived
                ? 'bg-secondary text-foreground font-semibold shadow-xs'
                : 'text-muted-foreground hover:bg-muted hover:text-foreground',
            )}
          >
            <span>{t('Agents')}</span>
            {agentUnreadCount > 0 && (
              <span className="font-mono text-[10px] px-1.5 py-0.2 bg-primary text-primary-foreground font-bold">
                {agentUnreadCount}
              </span>
            )}
          </button>
          <button
            type="button"
            aria-pressed={activeTab === 'customers' && !showArchived}
            onClick={() => {
              setShowArchived(false)
              handleTabClick('customers')
            }}
            className={cn(
              'inline-flex min-h-11 md:min-h-7 shrink-0 items-center gap-1.5 rounded-full px-3.5 py-1.5 md:rounded-md md:px-2.5 md:py-1 text-xs font-medium transition-colors',
              activeTab === 'customers' && !showArchived
                ? 'bg-secondary text-foreground font-semibold shadow-xs'
                : 'text-muted-foreground hover:bg-muted hover:text-foreground',
            )}
          >
            <span>{t('Customers')}</span>
            {onlineVisitorCount > 0 && (
              <span className="relative flex h-2 w-2" title={`${onlineVisitorCount} ${t('online')}`}>
                <span className="animate-ping absolute inline-flex h-full w-full rounded-full bg-emerald-400 opacity-75"></span>
                <span className="relative inline-flex rounded-full h-2 w-2 bg-emerald-500 status-dot"></span>
              </span>
            )}
            {customerUnreadCount > 0 && (
              <span className="font-mono text-[10px] px-1.5 py-0.2 bg-primary text-primary-foreground font-bold">
                {customerUnreadCount}
              </span>
            )}
          </button>
          <button
            type="button"
            aria-label={t(showArchived ? 'Back to Inbox' : 'Archived conversations')}
            title={t(showArchived ? 'Back to Inbox' : 'Archived conversations')}
            aria-pressed={showArchived}
            onClick={() => setShowArchived((v) => !v)}
            className={cn(
              'inline-flex min-h-11 md:min-h-7 shrink-0 items-center gap-1 rounded-full px-3.5 py-1.5 md:rounded-md md:px-2.5 md:py-1 text-xs font-medium transition-colors',
              showArchived
                ? 'bg-secondary text-foreground font-semibold shadow-xs'
                : 'text-muted-foreground hover:bg-muted hover:text-foreground',
            )}
          >
            <Archive className="h-3.5 w-3.5" />
          </button>
        </div>

        {/* Customer Sub-Filter (Chats vs Online Visitors) */}
        {activeTab === 'customers' && !showArchived && (
          <div className="flex items-center gap-1 pt-0.5">
            <button
              type="button"
              onClick={() => setCustomerSubFilter('chats')}
              className={cn(
                'px-2 py-0.5 rounded text-xs font-medium transition-colors flex items-center gap-1',
                customerSubFilter === 'chats'
                  ? 'bg-secondary text-foreground font-semibold'
                  : 'text-muted-foreground hover:text-foreground'
              )}
            >
              <span>{t('Chats')}</span>
              <span className="text-[10px] tabular-nums font-mono">({activeCustomerCount})</span>
            </button>
            <button
              type="button"
              onClick={() => setCustomerSubFilter('online')}
              className={cn(
                'px-2 py-0.5 rounded text-xs font-medium transition-colors flex items-center gap-1',
                customerSubFilter === 'online'
                  ? 'bg-emerald-500/15 text-emerald-400 font-semibold'
                  : 'text-muted-foreground hover:text-foreground'
              )}
            >
              <span>{t('Online')}</span>
              <span className="text-[10px] tabular-nums font-mono">({onlineVisitorCount})</span>
            </button>
          </div>
        )}
      </div>

      {archive.error && <div role="alert" className="px-3 text-xs">{t(archive.error)} <button className="min-h-11 underline" onClick={() => void archive.refresh()}>{t('Try again')}</button></div>}
      {/* Contacts List */}
      <div className="flex-1 overflow-y-auto divide-y divide-border/40">
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
