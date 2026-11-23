import { resolvableConversation, conversationHasUnfinishedTasks, setConversationResolved } from './conversation-resolution'
import { workConversationPreview } from './conversation-work-preview'
import { ListSearch, ListTabs, ListTab } from '../../components/layout/ListControls'
import { IconButton } from '../../components/ui/IconButton'
import { PageHeader, PageHeaderBrand, PageTitle } from '../../components/layout/PageHeader'
import { HairlineList } from '../../components/layout/HairlineList'
import { taskProgressModel } from '../../components/ui/TaskProgress'
import { StatusDot } from '../../components/ui/StatusDot'
import { ItemActions, copyItemLink } from '../../components/ui/ItemActions'
import { useArchivedItems } from '../../hooks/useArchivedItems'
import { useConnectionClock } from '../agents/useConnectionClock'
import { useLanguage } from '../../context/LanguageContext'
import type { Agent } from '../../api/agents'
import { agentForContact, connectionStatus } from '../agents/connection'
import { useState, useEffect, memo } from 'react'
import { Link, useLocation } from 'react-router-dom'
import { ChatContact, chatApi } from '../../api/chat'
import { Bot, User, CheckCircle2, CheckCheck, Plus, Circle, Eye } from 'lucide-react'
import { NotificationsPopover } from '../notifications/NotificationsPopover'
import { cn } from '../../lib/utils'
import { getContactInitials } from './contact-initials'
import { TASK_STATUS_THEMES, workStatus, workDisplayStatus } from '../tasks/task-ui'
import { isTaskActive } from './task-progress'
import { getRoomActiveSubstage, setRoomActiveSubstage } from './task-substage'
import './chat-calm.css'

export function conversationPreview(message?: string, counts?: ChatContact, t: (value: string) => string = value => value) {
  const activeTasks = typeof counts?.active_tasks === 'number' ? counts.active_tasks : Number(counts?.active_tasks) || 0
  const queuedTasks = typeof counts?.queued_tasks === 'number' ? counts.queued_tasks : Number(counts?.queued_tasks) || 0
  const reviewTasks = typeof counts?.review_tasks === 'number' ? counts.review_tasks : Number(counts?.review_tasks) || 0

  const roomId = counts?.conversation_id || counts?.id
  const cachedSubstage = roomId ? getRoomActiveSubstage(roomId) : undefined
  const substage = counts?.active_substage || cachedSubstage

  if (activeTasks > 0) {
    if (substage) {
      return t(substage) || substage
    }
    return t('Sedang dikerjakan')
  }
  if (reviewTasks > 0) {
    if (substage) {
      return t(substage) || substage
    }
    return t('Siap ditinjau')
  }
  if (queuedTasks > 0) {
    if (substage) {
      return t(substage) || substage
    }
    return t('Menunggu antrean')
  }

  if (!message?.trim()) return 'No messages yet.'
  let text = message.trim()
  if (/^Files from this work session/i.test(text)) {
    if (substage) return t(substage) || substage
    if (reviewTasks > 0) return t('Siap ditinjau')
    if (activeTasks > 0) return t('Sedang dikerjakan')
    if (queuedTasks > 0) return t('Menunggu antrean')
    return t('File dari sesi kerja ini')
  }
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

const MemoizedConversationItem = memo(function MemoizedConversationItem({
  contact,
  isSelected,
  isLast,
  now,
  agents = [],
  archive,
  onSelectContact,
  language,
}: ConversationItemProps) {
  const { t } = useLanguage()
  const status = connectionStatus(
    agentForContact(contact, agents),
    now,
  )
  const id = contact.conversation_id || contact.id
  const unread = contact.only_system_notices ? 0 : archive.unreadCount(id, contact.unread_count, contact.last_message_at || contact.updated_at)
  const activeTasks = typeof contact.active_tasks === 'number' ? contact.active_tasks : Number(contact.active_tasks) || 0
  const reviewTasks = Number(contact.review_tasks) || 0
  const failedTasks = Number(contact.failed_tasks) || 0
  const doneTasks = typeof contact.done_tasks === 'number' ? contact.done_tasks : Number(contact.done_tasks) || 0
  const queuedTasks = typeof contact.queued_tasks === 'number' ? contact.queued_tasks : Number(contact.queued_tasks) || 0
  const progress = {
    total: contact.task_count === undefined ? undefined : contact.task_count - (contact.canceled_tasks || 0),
    done: doneTasks,
    canceled: contact.canceled_tasks,
    active: activeTasks,
    queued: queuedTasks,
    review: reviewTasks,
    blocked: failedTasks,
  }
  const progModel = taskProgressModel(progress)
  const hasActiveWork = activeTasks > 0
  const totalTasks = contact.task_count !== undefined
    ? Math.max(0, contact.task_count - (contact.canceled_tasks || 0))
    : (progModel?.total ?? 0)
  const hasTasks = totalTasks > 0 || !!progModel
  const allTasksDone = hasTasks && (progModel ? progModel.complete : doneTasks >= totalTasks)
  const isResolved = Boolean(contact.resolved)
  const isComplete = isResolved || allTasksDone
  const isWorking = hasActiveWork

  const fraction = progModel ? progModel.fraction : (totalTasks > 0 ? doneTasks / totalTasks : 0)
  const angle = fraction * Math.PI * 2
  const pieX = 8 + 4.5 * Math.sin(angle)
  const pieY = 8 - 4.5 * Math.cos(angle)

  const hasFailed = failedTasks > 0
  const hasReview = reviewTasks > 0

  const statusTitle = isResolved
    ? t('Resolved (click to reopen)')
    : allTasksDone
      ? t('All tasks completed')
      : hasTasks
        ? (isWorking
            ? t('Working on tasks')
            : hasReview
              ? t('Siap ditinjau')
              : hasFailed
                ? t('Action required / paused')
                : queuedTasks > 0
                  ? t('Task queued in background')
                  : t('{{done}} of {{total}} tasks completed')
                    .replace('{{done}}', String(doneTasks))
                    .replace('{{total}}', String(totalTasks)))
        : t('Unresolved (click to resolve)')

  const statusAriaLabel = isResolved
    ? t('Resolved')
    : allTasksDone
      ? t('All tasks completed')
      : hasTasks
        ? (hasReview ? t('Ready for review') : t('Task progress'))
        : t('Unresolved')

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

  const prefetchConversation = () => {
    if (!id) return
    try {
      if (sessionStorage.getItem('cuan_chat_cache:' + id)) return
    } catch {}
    chatApi.getConversation(id, { limit: 50 }).then((data) => {
      if (data && data.messages) {
        try {
          if (data.tasks?.length) {
            const activeTask = data.tasks.find(t => isTaskActive(t)) || data.tasks.find(t => workStatus(t) === 'Ready for review') || data.tasks[0]
            if (activeTask) {
              setRoomActiveSubstage(id, workDisplayStatus(activeTask))
            }
          }
          const trimmed = {
            messages: (data.messages || []).slice(-30),
            requests: (data.requests || []).slice(-5),
            tasks: data.tasks || [],
            hasMore: Boolean(data.has_more),
            cachedAt: Date.now(),
          }
          sessionStorage.setItem('cuan_chat_cache:' + id, JSON.stringify(trimmed))
        } catch {}
      }
    }).catch(() => {})
  }

  const handleResolveAction = async () => {
    if (!contact.resolved && (contact.active_tasks || 0) > 0) {
      const confirmMessage = t('Ada {{count}} tugas yang sedang berjalan di background. Batalkan tugas dan tuntaskan percakapan?')
        .replace('{{count}}', String(contact.active_tasks))
      if (!window.confirm(confirmMessage)) return
    }
    await setConversationResolved(contact, !contact.resolved, { cancelUnfinished: true })
  }

  const archiveAction = {
    label: contact.resolved ? 'Reopen conversation' : 'Resolve conversation',
    previewLabel: contact.resolved ? 'Reopen' : 'Resolve',
    disabled: false,
    run: handleResolveAction,
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
        {
          label: 'Rename conversation',
          run: async () => {
            const newTitle = window.prompt(t('Rename conversation'), contact.name)
            if (newTitle && newTitle.trim() && newTitle.trim() !== contact.name) {
              try {
                await chatApi.updateTitle(contact.conversation_id || contact.id, newTitle.trim())
                window.dispatchEvent(
                  new CustomEvent('cuan-conversation-updated', {
                    detail: { roomId: contact.conversation_id || contact.id, title: newTitle.trim() },
                  }),
                )
              } catch (e) {
                console.error('Failed to rename conversation:', e)
              }
            }
          },
        },
        { label: 'Copy link', run: () => copyItemLink('/inbox/' + encodeURIComponent(id)) },
        archiveAction,
      ]}
    >
      <div data-conversation-row className="relative">
      <button
        type="button"
        onClick={openContact}
        onMouseEnter={prefetchConversation}
        aria-pressed={isSelected}
        data-testid="conversation-item"
        className={cn(
          'inbox-contact-row flex items-start gap-3 w-full min-h-[4.5rem] p-3 text-left focus-visible:outline focus-visible:outline-2 focus-visible:outline-ring border-0 bg-transparent',
          isSelected && 'text-accent-foreground font-medium shadow-none',
        )}
      >
        {/* Unified Identity Avatar with Overlaid Status Badge */}
        <div className="relative shrink-0 mt-0.5">
          <div
            data-avatar="true"
            className="h-9 w-9 rounded-full bg-secondary/80 flex items-center justify-center font-semibold text-xs border border-border/50 text-foreground overflow-hidden avatar select-none"
          >
            {contact.avatar && contact.avatar !== '👤' ? (
              contact.avatar.startsWith('http') || contact.avatar.startsWith('/') ? (
                <img src={contact.avatar} alt="" className="h-full w-full object-cover rounded-full" />
              ) : (
                <span>{contact.avatar}</span>
              )
            ) : contact.type === 'agent' ? (
              <Bot className="h-4 w-4 text-primary" />
            ) : getContactInitials(contact.name) ? (
              <span className="tracking-wide select-none">{getContactInitials(contact.name)}</span>
            ) : (
              <User className="h-4 w-4 text-muted-foreground" />
            )}
          </div>

          {/* Overlaid Status Badge at Bottom-Right */}
          <span
            role="button"
            tabIndex={0}
            onClick={async (e) => {
              e.stopPropagation()
              try {
                await handleResolveAction()
              } catch (err) {
                console.error('Failed to toggle conversation resolved state:', err)
              }
            }}
            onKeyDown={async (e) => {
              if (e.key === 'Enter' || e.key === ' ') {
                e.preventDefault()
                e.stopPropagation()
                try {
                  await handleResolveAction()
                } catch (err) {
                  console.error('Failed to toggle conversation resolved state:', err)
                }
              }
            }}
            title={statusTitle}
            aria-label={statusAriaLabel}
            data-testid="workflow-status-indicator"
            className={cn(
              'workflow-status-toggle absolute -bottom-0.5 -right-0.5 rounded-full bg-transparent flex items-center justify-center transition-transform hover:scale-110 cursor-pointer select-none',
              isComplete
                ? TASK_STATUS_THEMES.done.textColor
                : isWorking
                  ? TASK_STATUS_THEMES.working.textColor
                  : hasReview
                    ? TASK_STATUS_THEMES.review.textColor
                    : hasFailed
                      ? TASK_STATUS_THEMES.attention.textColor
                      : queuedTasks > 0
                        ? 'text-amber-500/80 dark:text-amber-400/80'
                        : 'text-muted-foreground/60 hover:text-foreground',
            )}
          >
            {isComplete ? (
              <CheckCircle2 className="h-4 w-4" strokeWidth={2} />
            ) : isWorking ? (
              <StatusDot status="working" size="md" className="m-0.5" />
            ) : hasReview ? (
              <Eye className="h-3.5 w-3.5" strokeWidth={2} aria-label={t('Ready for review')} />
            ) : hasTasks ? (
              <svg
                viewBox="0 0 16 16"
                className="h-4 w-4"
                data-testid="task-pie-progress"
              >
                <circle
                  cx="8"
                  cy="8"
                  r="6.5"
                  fill="none"
                  stroke="currentColor"
                  strokeWidth="1.5"
                  strokeDasharray={queuedTasks > 0 && doneTasks === 0 ? "2.5 1.5" : undefined}
                  opacity={hasFailed || hasReview || queuedTasks > 0 ? "0.9" : "0.65"}
                />
                {fraction > 0 && (
                  <path
                    d={`M 8 8 L 8 3.5 A 4.5 4.5 0 ${fraction > 0.5 ? 1 : 0} 1 ${pieX} ${pieY} Z`}
                    fill="currentColor"
                  />
                )}
              </svg>
            ) : contact.type === 'customer' && contact.is_online === true ? (
              <span
                className="h-2.5 w-2.5 bg-emerald-500 rounded-full status-dot m-0.5"
                aria-label="Online"
                title="Online"
              />
            ) : (
              <Circle className="h-4 w-4" strokeWidth={1.5} />
            )}
          </span>
        </div>

        {/* Content Body */}
        <div className="flex-1 min-w-0 flex flex-col gap-1">
          <div className="flex items-center justify-between gap-1.5">
            <div className="flex items-center gap-1.5 min-w-0">
              <span className={cn("text-sm truncate", unread > 0 ? "font-bold text-foreground" : "font-medium text-foreground/90")}>
                {contact.name}
              </span>
            </div>
            <div className="flex items-center gap-1.5 shrink-0">
              <span className="text-xs tabular-nums text-muted-foreground">
                {formatConversationTime(contact.last_message_at || contact.updated_at, language)}
              </span>
              {unread > 0 && (
                <span
                  data-testid="unread-micro-dot"
                  aria-label={t('Unread')}
                  className="h-1.5 w-1.5 rounded-full bg-foreground shrink-0 shadow-xs"
                />
              )}
            </div>
            {unread > 0 && <span data-unread-count className="sr-only">{t('{{count}} unread messages').replace('{{count}}', String(unread))}</span>}
          </div>

          {subject && (
            <div className="text-sm font-medium text-foreground/80 truncate">
              {subject}
            </div>
          )}

          <p className={cn(
            "text-sm line-clamp-1 leading-normal",
            unread > 0 ? "font-bold text-foreground/90 dark:text-zinc-200" : "font-normal text-muted-foreground dark:text-zinc-400"
          )}>
            {conversationPreview(contact.last_message, contact, t)}
          </p>

          {isAgentConnectionIssue && (
            <p className="mt-0.5 text-[11px] text-amber-700 dark:text-amber-400">
              {status.label}
            </p>
          )}
        </div>

      </button>
      </div>
      {!isLast && (
        <div
          aria-hidden="true"
          className="inbox-item-divider border-b border-border/40"
        />
      )}
    </ItemActions>
  )
})

export function ConversationItem(props: ConversationItemProps) {
  return <MemoizedConversationItem {...props} />
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
  const [unresolvedOnly, setUnresolvedOnly] = useState(true)
  const [resolutionError, setResolutionError] = useState('')
  const [, setSubstageTick] = useState(0)
  const clock = useConnectionClock(agents)
  const now = suppliedNow ?? clock
  const { t, language } = useLanguage()

  useEffect(() => {
    const handler = (e: Event) => {
      const custom = e as CustomEvent<{ roomId: string; substage?: string }>
      if (custom?.detail?.roomId) {
        setRoomActiveSubstage(custom.detail.roomId, custom.detail.substage)
        setSubstageTick((v) => v + 1)
      }
    }
    window.addEventListener('cuan:task-substage-changed', handler)
    return () => window.removeEventListener('cuan:task-substage-changed', handler)
  }, [])

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
  const resolvable = contacts.filter(contact => !contact.resolved && resolvableConversation(contact))
  const handleResolveCompleted = async () => {
    const skipped = contacts.filter(contact => !contact.resolved && contact.has_messages !== false && conversationHasUnfinishedTasks(contact)).length
    const confirmation = t('Resolve {{count}} conversations?').replace('{{count}}', String(resolvable.length)) +
      (skipped ? '\n' + t('{{count}} conversations with unfinished work will remain in your inbox.').replace('{{count}}', String(skipped)) : '')
    if (markingAll || !window.confirm(confirmation)) return
    setMarkingAll(true)
    setResolutionError('')
    try {
      let failed = 0
      for (let offset = 0; offset < resolvable.length; offset += 4) {
        const results = await Promise.allSettled(resolvable.slice(offset, offset + 4).map(contact => setConversationResolved(contact, true)))
        failed += results.filter(result => result.status === 'rejected').length
      }
      if (failed) setResolutionError(t('Some conversations could not be resolved. Review their latest activity.'))
    } finally { setMarkingAll(false) }
  }

  const filteredContacts = contacts.filter((c) => {
    if (showArchived ? !c.resolved : unresolvedOnly && (c.resolved || c.only_system_notices)) return false
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
      const preview = conversationPreview(c.last_message, c, t).toLowerCase()
      const matchMsg = (c.last_message || '').toLowerCase().includes(q) || preview.includes(q)
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
          <NotificationsPopover />
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
          {resolvable.length > 0 && (
            <IconButton
              type="button"
              disabled={markingAll}
              label={t('Resolve conversations')}
              title={t('Resolve conversations')}
              onClick={handleResolveCompleted}

            >
              <CheckCheck className="h-4 w-4 text-primary" />
            </IconButton>
          )}

        </div>
      </PageHeader>

      {/* ── Search & Filter Toolbar ── */}
      <div className="px-4 pt-3 space-y-2 border-b border-border/40">
        <ListSearch aria-label={t('Search conversations')} placeholder={t('Search conversations…')} value={query} onChange={event=>handleQueryChange(event.target.value)} />
        <div className="flex min-w-0 items-center gap-1">
        <ListTabs aria-label={t('Conversation filters')} className="min-w-0 flex-1 gap-2 border-b-0">
          <ListTab selected={activeTab==='all'&&!showArchived&&!unresolvedOnly} onClick={()=>{setShowArchived(false);setUnresolvedOnly(false);handleTabClick('all')}}>{t('All')}</ListTab>
          <ListTab selected={unresolvedOnly&&!showArchived} onClick={()=>{setShowArchived(false);setUnresolvedOnly(true)}}>{t('Unresolved')}</ListTab>
          <ListTab selected={activeTab==='agents'&&!showArchived} onClick={()=>{setShowArchived(false);setUnresolvedOnly(false);handleTabClick('agents')}}>{t('Assistant')}</ListTab>
          <ListTab selected={activeTab==='customers'&&!showArchived} onClick={()=>{setShowArchived(false);setUnresolvedOnly(false);handleTabClick('customers')}}>{t('Customers')}{onlineVisitorCount>0&&<StatusDot status="healthy" size="sm" title={onlineVisitorCount+' '+t('online')} />}</ListTab>
        </ListTabs>
          <IconButton className="ml-auto" label={t(showArchived?'Back to Inbox':'Resolved conversations')} active={showArchived} aria-pressed={showArchived} onClick={()=>setShowArchived(value=>!value)}><CheckCircle2 /></IconButton>
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
                  : 'text-muted-foreground hover:text-foreground bg-transparent'
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
                  : 'text-muted-foreground hover:text-foreground bg-transparent'
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
                ? 'No resolved conversations.'
                : unresolvedOnly && !query.trim() ? 'Inbox zero. Nothing needs attention.' : 'No matching conversations.'
            )}
            {!query.trim() && !showArchived && !unresolvedOnly && customerCount === 0 && activeTab !== 'agents' && (
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
                <MemoizedConversationItem
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
      {resolutionError && <p role="alert" className="px-4 py-2 text-xs text-destructive">{resolutionError}</p>}
    </div>
  )
}
