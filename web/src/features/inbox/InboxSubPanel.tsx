import { workConversationPreview } from './conversation-work-preview'
import { ListSearch, ListTabs, ListTab } from '../../components/layout/ListControls'
import { IconButton } from '../../components/ui/IconButton'
import { PageHeader, PageHeaderBrand, PageTitle } from '../../components/layout/PageHeader'
import { HairlineList } from '../../components/layout/HairlineList'
import { TaskProgress, taskProgressModel } from '../../components/ui/TaskProgress'
import { StatusDot } from '../../components/ui/StatusDot'
import { ItemActions, copyItemLink } from '../../components/ui/ItemActions'
import { useArchivedItems, ArchiveNotice } from '../../hooks/useArchivedItems'
import { useConnectionClock } from '../agents/useConnectionClock'
import { useLanguage } from '../../context/LanguageContext'
import type { Agent } from '../../api/agents'
import { agentForContact, connectionStatus } from '../agents/connection'
import { useState, useEffect } from 'react'
import { Link, useLocation } from 'react-router-dom'
import { ChatContact } from '../../api/chat'
import { Bot, User, Archive, CheckCheck, Plus, AlertCircle } from 'lucide-react'
import { cn } from '../../lib/utils'
import './chat-calm.css'

export function conversationPreview(message?: string, counts?: ChatContact, t: (value: string) => string = value => value) {
  if (!message?.trim()) return 'No messages yet.'
  let text = message.trim()
  const workPreview = workConversationPreview(text, counts)
  if (workPreview) return t(workPreview)
  if (/the work is saved, but could not start|create attempt:|invalid task status/i.test(text)) return t('The work is saved, but could not start.')

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
  const reviewTasks = Number(contact.review_tasks) || 0
  const failedTasks = Number(contact.failed_tasks) || 0
  const progress = { total: contact.task_count === undefined ? undefined : contact.task_count - (contact.canceled_tasks || 0), done: contact.done_tasks, canceled: contact.canceled_tasks, active: activeTasks, queued: contact.queued_tasks, review: reviewTasks, blocked: contact.failed_tasks, minTasks: 2 }
  const showProgress = !!taskProgressModel(progress)
  const hasActiveWork = activeTasks > 0
  const isAgentConnectionIssue =
    contact.type === 'agent' && contact.agent_id !== 'agent_cuan' &&
    !hasActiveWork &&
    ['login_required', 'quota_limited', 'unavailable'].includes(status.state)

  const subject = [contact.product, contact.page_title].find(value =>
    value && value.toLowerCase() !== 'general' && value !== contact.name,
  )

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
      quiet
      swipeAction={archiveAction}
      className={cn(
        'inbox-item-row border-b border-border/40 transition-colors hover:bg-accent/40',
        isSelected && 'bg-accent/35 text-accent-foreground',
        !isLast && 'has-divider',
      )}
      actions={[
        { label: 'Open conversation', run: openContact },
        { label: unread ? 'Mark as read' : 'Mark as unread', disabled: !archive.loaded, run: () => archive.markRead(id, unread > 0) },
        { label: 'Copy link', run: () => copyItemLink('/inbox/' + encodeURIComponent(id)) },
        archiveAction,
      ]}
    >
      <div data-conversation-row className="relative">
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
        {/* Circular identity avatar; task progress stays separate */}
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
          {contact.type === 'customer' && contact.is_online === true && (
            <span
              className="absolute -bottom-0.5 -right-0.5 h-2.5 w-2.5 bg-emerald-500 rounded-full border-2 border-background status-dot"
              aria-label="Online"
              title="Online"
            />
          )}
        </div>

        {/* Content Body */}
        <div className="flex-1 min-w-0 flex flex-col gap-1">
          <div className={cn("flex items-center justify-between gap-1.5", showProgress && "pr-9")}>
            <div className="flex items-center gap-1.5 min-w-0">
              <span className={cn("text-sm truncate", unread > 0 ? "font-semibold text-foreground" : "font-medium text-foreground/90")}>
                {contact.name}
              </span>

            </div>
            <span className="text-xs shrink-0 tabular-nums text-muted-foreground">
              {formatConversationTime(contact.last_message_at || contact.updated_at, language)}
            </span>
            {unread > 0 && <span data-unread-count className="sr-only">{t('{{count}} unread messages').replace('{{count}}', String(unread))}</span>}
          </div>

          {subject && (
            <div className="text-sm font-medium text-foreground/80 truncate">
              {subject}
            </div>
          )}

          <p className="text-sm text-muted-foreground dark:text-zinc-400 line-clamp-1 leading-normal">
            {conversationPreview(contact.last_message, contact, t)}
          </p>

          {(hasActiveWork || reviewTasks > 0 || failedTasks > 0) && (
            <div className="flex flex-wrap items-center gap-x-2 gap-y-1 pt-0.5">
              {failedTasks > 0 && (
                <span data-testid="task-badge-attention" className="inline-flex items-center gap-1 text-xs font-medium text-muted-foreground">
                  <AlertCircle aria-hidden="true" className="h-3 w-3 text-[color:var(--status-warning)]" />
                  <span>{t(failedTasks === 1 ? '{{count}} needs attention' : '{{count}} need attention').replace('{{count}}', String(failedTasks))}</span>
                </span>
              )}
              {hasActiveWork && (
                <span
                  data-testid="task-badge-active"
                  aria-label={`${activeTasks} active tasks`}
                  className="inline-flex items-center gap-1 text-xs font-medium text-muted-foreground"
                >
                  <StatusDot status="working" size="xs" pulse />
                  <span data-testid="task-count-badge">{activeTasks} {t('bekerja')}</span>
                </span>
              )}
              {reviewTasks > 0 && <span data-testid="task-badge-review" className="text-xs text-muted-foreground">{reviewTasks} {t('meninjau hasil')}</span>}

            </div>
          )}

          {isAgentConnectionIssue && (
            <p className="mt-0.5 text-[11px] text-amber-700 dark:text-amber-400">
              {status.label}
            </p>
          )}
        </div>

      </button>
      {showProgress && <div className="absolute right-0 top-0 z-10"><TaskProgress {...progress} /></div>}
      </div>
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
  const location = useLocation()
  const liveChatSettings = '/settings/integrations/live-chat?returnTo=' + encodeURIComponent(location.pathname + location.search)
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
      <PageHeader>
        <div className="flex min-w-0 items-center gap-2"><PageHeaderBrand /><PageTitle>{t('Inbox')}</PageTitle></div>
        <div className="flex items-center gap-1">
          {onNewMessage && (
            <IconButton
              type="button"
              onClick={onNewMessage}
              label={t('New message')}
              title={t('New message')}

            >
              <Plus className="h-4 w-4" />
            </IconButton>
          )}
          {totalUnreadCount > 0 && (
            <IconButton
              type="button"
              disabled={markingAll}
              label={t('Mark all as read')}
              title={t('Mark all as read')}
              onClick={handleMarkAllRead}

            >
              <CheckCheck className="h-4 w-4 text-primary" />
            </IconButton>
          )}

        </div>
      </PageHeader>

      {/* ── Search & Filter Toolbar ── */}
      <div className="px-4 pt-3 space-y-2 border-b border-border/40">
        <ListSearch aria-label={t('Search conversations')} placeholder={t('Search conversations…')} value={query} onChange={event=>handleQueryChange(event.target.value)} />
        <ListTabs aria-label={t('Conversation filters')} className="border-b-0">
          <ListTab selected={activeTab==='all'&&!showArchived} onClick={()=>{setShowArchived(false);handleTabClick('all')}}>{t('All')}</ListTab>
          <ListTab selected={activeTab==='agents'&&!showArchived} onClick={()=>{setShowArchived(false);handleTabClick('agents')}}>{t('Assistant')}</ListTab>
          <ListTab selected={activeTab==='customers'&&!showArchived} onClick={()=>{setShowArchived(false);handleTabClick('customers')}}>{t('Customers')}{onlineVisitorCount>0&&<StatusDot status="healthy" size="sm" title={onlineVisitorCount+' '+t('online')} />}</ListTab>
          <IconButton className="ml-auto" label={t(showArchived?'Back to Inbox':'Archived conversations')} active={showArchived} aria-pressed={showArchived} onClick={()=>setShowArchived(value=>!value)}><Archive /></IconButton>
        </ListTabs>

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
              <span className="text-xs tabular-nums">({activeCustomerCount})</span>
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
              <span className="text-xs tabular-nums">({onlineVisitorCount})</span>
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
                <Link className="action-secondary inline-flex" to={liveChatSettings}>
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
