import { updateItemPreference } from '../../hooks/useArchivedItems'
import { ItemActions, copyItemLink } from '../../components/ui/ItemActions'
import { Button } from "../../components/ui/button"
import { useConnectionClock } from '../agents/useConnectionClock'
import { useLanguage } from '../../context/LanguageContext'
import {
  AttachmentPicker,
  AttachmentPreviews,
  AttachmentFile,
  useAttachments,
} from './Attachments'
import { CannedResponsesPicker } from './CannedResponsesPicker'
import { CannedResponsesDialog } from './CannedResponsesDialog'
import { Link, useLocation } from 'react-router-dom'
import { ActionDialog } from '../../components/dialogs/ActionDialog'
import { type Agent, isModelVisionCapable } from '../../api/agents'
import { ContactStatus } from '../agents/ContactStatus'
import { agentForContact } from '../agents/connection'
import {
  useState,
  useEffect,
  useLayoutEffect,
  useMemo,
  useRef,
  useCallback,
} from 'react'
import {
  ChatContact,
  ChatMessage,
  InboxRequest,
  chatApi,
  AgentActionCard,
  streamConversation,
  cancelInboxRequest,
  retryInboxRequest,
} from '../../api/chat'
import {
  Send,
  Bot,
  User,
  Check,
  CheckCheck,
  Play,
  CheckSquare,
  Sparkles,
  RefreshCw,
  Settings,
  ChevronLeft,
  ChevronDown,
} from 'lucide-react'
import { Task } from '../../api/types'
import './chat-whatsapp.css'
import { MessageContent } from './MessageContent'
import { InboxActivity } from './InboxActivity'
import { InboxTaskAction } from './InboxTaskAction'
import { ConversationWorkMonitor } from './ConversationWorkMonitor'
import { TaskItemCard } from '../tasks/TaskItemCard'
import { EscalateTaskDialog } from './EscalateTaskDialog'
import { cn } from '../../lib/utils'
import { InboxSubPanel, ContactFilterTab } from './InboxSubPanel'

interface InboxViewProps {
  onNewMessage?: () => void
  contacts?: ChatContact[]
  agents?: Agent[]
  now?: number
  loading?: boolean
  error?: string
  onRetry?: () => void
  activeContact?: ChatContact
  onSelectContact?: (contact: ChatContact) => void
  onContactUpdated?: () => void
  contactTab?: ContactFilterTab
  onContactTabChange?: (tab: ContactFilterTab) => void
  contactSearch?: string
  onContactSearchChange?: (q: string) => void
  onBackToContacts?: () => void
  onMobileChatActiveChange?: (active: boolean) => void
}

export function InboxView({
  onNewMessage: _onNewMessage,
  contacts = [],
  agents = [],
  now: suppliedNow,
  loading: contactsLoading = false,
  error: contactsError = '',
  onRetry,
  activeContact,
  onSelectContact,
  onContactUpdated,
  contactTab = 'all',
  onContactTabChange,
  contactSearch = '',
  onContactSearchChange,
  onBackToContacts,
  onMobileChatActiveChange,
}: InboxViewProps) {
  const clock = useConnectionClock(agents)
  const now = suppliedNow ?? clock
  const location = useLocation()
  const requestedMessageId =
    new URLSearchParams(location.search).get('message') || ''
  const openedSearchMessage = useRef('')
  const { t, language } = useLanguage()

  const unsentDrafts = useRef<Record<string, string>>({})
  const loadVersion = useRef(0)
  const readReceipts = useRef(new Map<string, string>())
  const latestLoaded = useRef<{ id: string; messages: ChatMessage[] }>()
  function markConversationSeen(id: string, next: ChatMessage[]) {
    latestLoaded.current = { id, messages: next }
    if (document.visibilityState !== 'visible' || activeConvRef.current !== id) return
    const last = next[next.length - 1]
    if (!last) return
    const signature = String(last?.uuid || last?.id || 'empty') + (last?.created_at || '')
    if (readReceipts.current.get(id) === signature) return
    readReceipts.current.set(id, signature)
    void updateItemPreference('conversation', id, { read: true, seen_at: last.created_at }).catch(() => {
      if (readReceipts.current.get(id) === signature) readReceipts.current.delete(id)
    })
  }
  useEffect(() => {
    const visible = () => {
      const loaded = latestLoaded.current
      if (loaded) markConversationSeen(loaded.id, loaded.messages)
    }
    document.addEventListener('visibilitychange', visible)
    return () => document.removeEventListener('visibilitychange', visible)
  }, [])

  const followTail = useRef(true)
  const [hasNewMessages, setHasNewMessages] = useState(false)
  const hasNewMessagesRef = useRef(false)
  hasNewMessagesRef.current = hasNewMessages
  const [showScrollBottom, setShowScrollBottom] = useState(false)
  const [unreadCount, setUnreadCount] = useState(0)
  const isScrollingToBottomRef = useRef(false)
  const scrollTimeoutRef = useRef<ReturnType<typeof setTimeout>>()
  const prevMessageKeyRef = useRef<string | number | undefined>()
  const prevContactIdRef = useRef<string | undefined>()
  const prevMessageCountRef = useRef(0)
  const [streamState, setStreamState] = useState('connecting')
  const [linkedTasks, setLinkedTasks] = useState<Task[]>([])
  const [showTasksModal, setShowTasksModal] = useState(false)
  const [convTab, setConvTab] = useState<'chat' | 'tasks'>('chat')
  const [escalatingMessage, setEscalatingMessage] = useState<ChatMessage | null>(null)

  useEffect(() => {
    setConvTab('chat')
  }, [activeContact?.id])
  const [requests, setRequests] = useState<InboxRequest[]>([])
  const [sendError, setSendError] = useState('')
  const [retryCount, setRetryCount] = useState(0)
  const pendingRef = useRef<{
    convId: string
    text: string
    id: string
    attachmentIds?: string[]
    approveMessageId?: string
    planAction?: 'approve' | 'defer'
  } | null>(null)
  const activeConvRef = useRef('')
  activeConvRef.current =
    activeContact?.conversation_id || activeContact?.id || ''
  const [hasMoreOlder, setHasMoreOlder] = useState(false)
  const [loadingOlder, setLoadingOlder] = useState(false)
  const olderAnchor = useRef<{ height: number; top: number } | null>(null)
  useEffect(() => {
    setHasMoreOlder(false)
    setLoadingOlder(false)
    olderAnchor.current = null
  }, [activeContact?.id])
  const [messages, setMessages] = useState<ChatMessage[]>([])
  const [inputMessage, setInputMessage] = useState('')
  const uploads = useAttachments(
    'cuan-inbox-attachments:' +
      (activeContact?.conversation_id || activeContact?.id || ''),
  )

  const activeAgent = agents?.find((a) => a.id === activeContact?.id || a.id === activeContact?.agent_id || a.name === activeContact?.name)
  const contactSupportsVision = activeAgent ? isModelVisionCapable(activeAgent.model_id, activeAgent.provider || activeAgent.harness) : true

  useEffect(() => {
    if (!contactSupportsVision && uploads.hasImages()) {
      uploads.removeImages()
      setSendError(t('Model ini tidak mendukung input gambar (vision). Lampiran gambar telah dibatalkan.'))
    }
  }, [contactSupportsVision, activeContact?.id])
  const [loading, setLoading] = useState(false)
  const [sending, setSending] = useState(false)
  const [approving, setApproving] = useState<string | null>(null)
  const [approvedTasks, setApprovedTasks] = useState<Record<string, boolean>>(
    {},
  )
  const [showCannedManageModal, setShowCannedManageModal] = useState(false)
  const [isMultiline, setIsMultiline] = useState(false)

  // Mobile Single-View WhatsApp/Telegram state: 'contacts' | 'chat'
  const [mobileView, setMobileView] = useState<'contacts' | 'chat'>('contacts')

  useEffect(() => {
    setMobileView(activeContact ? 'chat' : 'contacts')
  }, [activeContact?.id])

  // Notify parent of mobile chat active state to auto-hide bottom nav
  useEffect(() => {
    const isActive = mobileView === 'chat' && !!activeContact
    if (onMobileChatActiveChange) {
      onMobileChatActiveChange(isActive)
    }
  }, [mobileView, activeContact, onMobileChatActiveChange])

  const messageListRef = useRef<HTMLDivElement>(null)
  const textareaRef = useRef<HTMLTextAreaElement>(null)

  const loadConversation = useCallback(
    async (convId: string, showSpinner = false, all = false) => {
      if (showSpinner) setLoading(true)
      const version = ++loadVersion.current
      try {
        const data = await chatApi.getConversation(
          convId,
          all ? { all: true } : { limit: 50 },
        )
        if (activeConvRef.current !== convId || version !== loadVersion.current)
          return
        setMessages(data.messages || [])
        setHasMoreOlder(!!data.has_more)
        markConversationSeen(convId, data.messages || [])
        setRequests(data.requests || [])
        setLinkedTasks(data.tasks || [])
        const pending = pendingRef.current
        if (
          pending?.convId === convId &&
          data.requests?.some((q) => q.request_id === pending.id)
        ) {
          localStorage.removeItem('cuan_inbox_pending:' + convId)
          uploads.acknowledge(
            pending.attachmentIds || [],
            'cuan-inbox-attachments:' + convId,
          )
          pendingRef.current = null
          setSendError('')
          if (unsentDrafts.current[convId]?.trim() === pending.text)
            delete unsentDrafts.current[convId]
          setInputMessage((current) => {
            if (current.trim() === pending.text) {
              setIsMultiline(false)
              if (textareaRef.current) {
                const minH = window.innerWidth < 640 ? 36 : 38
                textareaRef.current.style.height = `${minH}px`
                textareaRef.current.style.overflowY = 'hidden'
              }
              return ''
            }
            return current
          })
          setRetryCount(0)
        }
      } catch (err) {
        if (activeConvRef.current !== convId || version !== loadVersion.current)
          return
        setSendError(
          t('Could not load history. Refresh to check your message status.'),
        )
      } finally {
        if (showSpinner) setLoading(false)
      }
    },
    [t],
  )

  const loadOlderMessages = useCallback(async () => {
    if (loadingOlder || !activeContact || messages.length === 0) return
    const convId = activeContact.conversation_id || activeContact.id
    const oldest = messages[0]
    if (!oldest?.uuid && !oldest?.id) return
    setLoadingOlder(true)
    const list = messageListRef.current
    if (list) {
      olderAnchor.current = {
        height: list.scrollHeight,
        top: list.scrollTop,
      }
    }
    followTail.current = false
    try {
      const data = await chatApi.getConversation(convId, {
        limit: 50,
        before_id: oldest.uuid || String(oldest.id),
      })
      if (data.messages && data.messages.length > 0) {
        setMessages((prev) => {
          const existingIds = new Set(prev.map((m) => m.uuid || String(m.id)))
          const newOlder = data.messages.filter(
            (m) => !existingIds.has(m.uuid || String(m.id)),
          )
          return [...newOlder, ...prev]
        })
      }
      setHasMoreOlder(!!data.has_more)
    } catch (err) {
      console.error('Failed to load older messages', err)
    } finally {
      setLoadingOlder(false)
    }
  }, [loadingOlder, activeContact, messages])

  // Auto-resize textarea
  const autoResizeTextarea = (el: HTMLTextAreaElement) => {
    el.style.height = 'auto'
    const scrollH = el.scrollHeight
    const minH = window.innerWidth < 640 ? 36 : 38
    const cap = Math.max(
      minH,
      Math.min(
        140,
        Math.max(120, (window.visualViewport?.height || window.innerHeight) * 0.35),
      ),
    )
    const newH = Math.min(Math.max(scrollH, minH), cap)
    el.style.height = newH + 'px'
    el.style.overflowY = scrollH > cap ? 'auto' : 'hidden'
    setIsMultiline(newH > minH + 4)
  }

  useEffect(() => {
    const resize = () => {
      if (textareaRef.current) autoResizeTextarea(textareaRef.current)
    }
    resize()
    window.visualViewport?.addEventListener('resize', resize)
    window.addEventListener('resize', resize)
    return () => {
      window.visualViewport?.removeEventListener('resize', resize)
      window.removeEventListener('resize', resize)
    }
  }, [inputMessage, activeContact?.id])

  // Load conversation when activeContact changes
  useEffect(() => {
    if (activeContact) {
      const convId = activeContact.conversation_id || activeContact.id
      followTail.current = !requestedMessageId
      setHasNewMessages(false)
      const draft = unsentDrafts.current[convId] || ''
      setInputMessage(draft)
      setIsMultiline(draft.includes('\n') || draft.length > 50)
      hasNewMessagesRef.current = false
      setShowScrollBottom(false)
      setUnreadCount(0)
      isScrollingToBottomRef.current = false
      if (scrollTimeoutRef.current) clearTimeout(scrollTimeoutRef.current)
      setMessages([])
      setRequests([])
      setLinkedTasks([])
      setSendError('')
      setRetryCount(0)
      pendingRef.current = null
      try {
        const saved = localStorage.getItem('cuan_inbox_pending:' + convId)
        if (saved) {
          pendingRef.current = JSON.parse(saved)
          setSendError(
            t(
              'Your previous message is not confirmed. Refresh or check again using the same ID.',
            ),
          )
        }
      } catch {
        setSendError(
          t(
            'Browser storage is unavailable. Refresh history before sending again.',
          ),
        )
      }
      loadConversation(convId, true)
    }
  }, [activeContact?.conversation_id, activeContact?.id, loadConversation])

  // SSE updates carry authoritative snapshots. Poll only while disconnected.
  useEffect(() => {
    if (!activeContact) return
    const convId = activeContact.conversation_id || activeContact.id
    const abort = new AbortController()
    let retry: ReturnType<typeof setTimeout> | undefined
    let failures = 0
    let connected = false
    const connect = async () => {
      try {
        await streamConversation(convId, abort.signal, (data) => {
          if (abort.signal.aborted || activeConvRef.current !== convId) return
          ++loadVersion.current
          connected = true
          failures = 0
          setStreamState('live')
          setLoading(false)
          setMessages((prev) => {
            if (prev.length === 0) return data.messages || []
            const byUuid = new Map(prev.map((m) => [m.uuid || String(m.id), m]))
            for (const m of data.messages || []) {
              byUuid.set(m.uuid || String(m.id), m)
            }
            return Array.from(byUuid.values()).sort(
              (a, b) => (a.id || 0) - (b.id || 0),
            )
          })
        markConversationSeen(convId, data.messages || [])
          setRequests(data.requests || [])
          setLinkedTasks(data.tasks || [])
          const pending = pendingRef.current
          if (
            pending?.convId === convId &&
            data.requests?.some((q) => q.request_id === pending.id)
          ) {
            localStorage.removeItem('cuan_inbox_pending:' + convId)
            uploads.acknowledge(
              pending.attachmentIds || [],
              'cuan-inbox-attachments:' + convId,
            )
            pendingRef.current = null
            setSendError('')
            setRetryCount(0)
            setInputMessage((current) =>
              current.trim() === pending.text ? '' : current,
            )
          }
        })
      } catch (e: any) {
        if (abort.signal.aborted) return
        if (e.status === 401) {
          setStreamState('login')
          return
        }
        failures++
      }
      connected = false
      if (!abort.signal.aborted) {
        setStreamState('reconnecting')
        retry = setTimeout(
          connect,
          Math.min(1000 * 2 ** Math.min(failures, 4), 15000),
        )
      }
    }
    setStreamState('connecting')
    void connect()
    const poll = setInterval(() => {
      if (!connected) void loadConversation(convId)
    }, 10000)
    const visible = () => {
      if (document.visibilityState === 'visible') void loadConversation(convId)
    }
    document.addEventListener('visibilitychange', visible)
    return () => {
      abort.abort()
      clearTimeout(retry)
      clearInterval(poll)
      document.removeEventListener('visibilitychange', visible)
    }
  }, [activeContact?.conversation_id, activeContact?.id, loadConversation])

  useLayoutEffect(() => {
    if (!requestedMessageId || !activeContact) return
    const key = activeContact.id + ':' + requestedMessageId
    if (openedSearchMessage.current === key) return
    const index = messages.findIndex(
      (message) => message.uuid === requestedMessageId,
    )
    if (index < 0) {
      const convId = activeContact.conversation_id || activeContact.id
      void loadConversation(convId, false, true)
      return
    }
    followTail.current = false
    const element = Array.from(
      messageListRef.current?.querySelectorAll<HTMLElement>(
        '[data-message-id]',
      ) || [],
    ).find((node) => node.dataset.messageId === requestedMessageId)
    if (element) {
      element.scrollIntoView({ block: 'center' })
      openedSearchMessage.current = key
    }
  }, [requestedMessageId, activeContact?.id, messages, loadConversation])

  const lastMessageKey =
    messages[messages.length - 1]?.uuid || messages[messages.length - 1]?.id

  // Only scroll for a new message, not identical polling results.
  useEffect(() => {
    const currentContactId = activeContact?.id
    const isNewContact = currentContactId !== prevContactIdRef.current
    prevContactIdRef.current = currentContactId

    if (isNewContact) {
      prevMessageKeyRef.current = lastMessageKey
      prevMessageCountRef.current = messages.length
      setHasNewMessages(false)
      hasNewMessagesRef.current = false
      setUnreadCount(0)
      setShowScrollBottom(false)
      if (messageListRef.current && followTail.current) {
        messageListRef.current.scrollTop = messageListRef.current.scrollHeight
      }
      return
    }

    const isNewMessage =
      !!lastMessageKey && lastMessageKey !== prevMessageKeyRef.current
    const addedCount = Math.max(
      0,
      messages.length - prevMessageCountRef.current,
    )
    prevMessageKeyRef.current = lastMessageKey
    prevMessageCountRef.current = messages.length

    if (messageListRef.current && followTail.current) {
      messageListRef.current.scrollTop = messageListRef.current.scrollHeight
      setHasNewMessages(false)
      hasNewMessagesRef.current = false
      setUnreadCount(0)
      setShowScrollBottom(false)
    } else if (messageListRef.current && (isNewMessage || addedCount > 0)) {
      setHasNewMessages(true)
      hasNewMessagesRef.current = true
      setUnreadCount((c) => c + (addedCount > 0 ? addedCount : 1))
      setShowScrollBottom(true)
    }
  }, [
    lastMessageKey,
    activeContact?.id,
    sendError,
    requests.map((q) => q.request_id + q.state).join(','),
  ])

  const SCROLL_THRESHOLD = 200
  const BOTTOM_THRESHOLD = 80

  const handleScroll = useCallback((e: React.UIEvent<HTMLDivElement>) => {
    const el = e.currentTarget
    const distanceFromBottom =
      el.scrollHeight - el.scrollTop - el.clientHeight
    const atBottom = distanceFromBottom <= BOTTOM_THRESHOLD

    followTail.current = atBottom

    if (isScrollingToBottomRef.current) {
      if (atBottom) {
        isScrollingToBottomRef.current = false
        if (scrollTimeoutRef.current) clearTimeout(scrollTimeoutRef.current)
        setShowScrollBottom(false)
        setHasNewMessages(false)
        hasNewMessagesRef.current = false
        setUnreadCount(0)
      }
      return
    }

    if (atBottom) {
      setShowScrollBottom(false)
      setHasNewMessages(false)
      hasNewMessagesRef.current = false
      setUnreadCount(0)
    } else if (distanceFromBottom > SCROLL_THRESHOLD) {
      setShowScrollBottom(true)
    } else {
      setShowScrollBottom(hasNewMessagesRef.current)
    }
  }, [])

  const handleScrollToBottom = useCallback(() => {
    followTail.current = true
    isScrollingToBottomRef.current = true
    setShowScrollBottom(false)
    setHasNewMessages(false)
    hasNewMessagesRef.current = false
    setUnreadCount(0)

    if (scrollTimeoutRef.current) clearTimeout(scrollTimeoutRef.current)
    scrollTimeoutRef.current = setTimeout(() => {
      isScrollingToBottomRef.current = false
    }, 1000)

    const list = messageListRef.current
    if (list) {
      const prefersReducedMotion =
        typeof window !== 'undefined' &&
        window.matchMedia('(prefers-reduced-motion: reduce)').matches
      list.scrollTo({
        top: list.scrollHeight,
        behavior: prefersReducedMotion ? 'auto' : 'smooth',
      })
    }
  }, [])

  useEffect(() => {
    return () => {
      if (scrollTimeoutRef.current) clearTimeout(scrollTimeoutRef.current)
    }
  }, [])

  const handleSendMessage = async (approveMessageId?: string, approvalRequestId?: string, planAction?: 'approve' | 'defer') => {
    if (!activeContact || sending || uploads.busy || retryCount >= 3) return
    const convId = activeContact.conversation_id || activeContact.id
    const pending = pendingRef.current
    const text = pending?.convId === convId ? pending.text : approveMessageId ? t(planAction === 'defer' ? 'Not now' : 'Approve this plan') : inputMessage.trim()
    if (!text && !uploads.items.length && !pending?.attachmentIds?.length)
      return
    const request =
      pending?.convId === convId
        ? pending
        : {
            convId,
            text,
            id: approvalRequestId || crypto.randomUUID(),
            attachmentIds:
              approveMessageId || activeContact.type !== 'agent'
                ? []
                : uploads.items.map((a) => a.id),
            approveMessageId,
            planAction,
          }
    // Save before the first network byte; reload/retry reuses the same key.
    try {
      localStorage.setItem(
        'cuan_inbox_pending:' + convId,
        JSON.stringify(request),
      )
    } catch {
      setSendError(
        t(
          'Could not save the message ID. Enable browser storage and try again.',
        ),
      )
      return
    }
    pendingRef.current = request
    followTail.current = true
    setSending(true)
    setRetryCount((n) => n + 1)
    try {
      await chatApi.sendMessage(
        convId,
        text,
        'You',
        request.id,
        request.attachmentIds,
        request.approveMessageId,
        request.planAction,
      )
      if (!request.approveMessageId) uploads.clear()
      if (unsentDrafts.current[convId]?.trim() === request.text)
        delete unsentDrafts.current[convId]
      if (activeConvRef.current === convId) {
        setInputMessage((current) => {
          if (current.trim() === request.text) {
            setIsMultiline(false)
            if (textareaRef.current) {
              const minH = window.innerWidth < 640 ? 36 : 38
              textareaRef.current.style.height = `${minH}px`
              textareaRef.current.style.overflowY = 'hidden'
            }
            return ''
          }
          return current
        })
        setSendError('')
        setRetryCount(0)
        await loadConversation(convId)
      }
      onContactUpdated?.()
    } catch {
      if (activeConvRef.current === convId)
        setSendError(
          t(
            'Delivery is not confirmed. Your message may have been received. Refresh or check again using the same ID to avoid a duplicate run.',
          ),
        )
    } finally {
      setSending(false)
    }
  }

  const handleKeyDown = (e: React.KeyboardEvent<HTMLTextAreaElement>) => {
    if (e.key === 'Enter' && !e.shiftKey && !e.nativeEvent.isComposing) {
      e.preventDefault()
      handleSendMessage()
    }
  }

  const handle1ClickApprove = async (card: AgentActionCard, convId: string) => {
    setApproving(card.task_title)
    try {
      await chatApi.approve(convId, {
        title: card.task_title,
        assignee: card.worker_agent,
        dod_checklist: card.dod_checklist,
      })
      setApprovedTasks((prev) => ({ ...prev, [card.task_title]: true }))
      // Reload messages to get execution confirmation
      loadConversation(convId, false)
      if (onContactUpdated) onContactUpdated()
    } catch (err) {
      console.error('Failed to approve task:', err)
    } finally {
      setApproving(null)
    }
  }

  const formatMessageTime = (iso?: string) => {
    if (!iso) return ''
    try {
      const d = new Date(iso)
      return d.toLocaleTimeString(language, {
        hour: '2-digit',
        minute: '2-digit',
        hourCycle: 'h23',
      })
    } catch {
      return ''
    }
  }

  const handleSelectContactInternal = (contact: ChatContact) => {
    if (onSelectContact) {
      onSelectContact(contact)
    }
    setMobileView('chat')
  }

  const handleBackToContacts = () => {
    setMobileView('contacts')
    if (onBackToContacts) {
      onBackToContacts()
    }
  }

  const visibleMessageIds = new Set(
    messages.map((message) => message.uuid).filter(Boolean),
  )
  const requestAttempts = useMemo(() => {
    const grouped = new Map<string, InboxRequest[]>()
    for (const request of requests) {
      const attempts = grouped.get(request.message_id) || []
      attempts.push(request)
      grouped.set(request.message_id, attempts)
    }
    return grouped
  }, [requests])
  const requestsByMessage = useMemo(() => {
    const ids = new Set(messages.map((message) => message.uuid).filter(Boolean))
    const index = new Map<string, InboxRequest[]>()
    const latest = new Map<string, InboxRequest>()
    for (const request of requests) {
      const previous = latest.get(request.message_id)
      if (!previous || Date.parse(request.created_at) > Date.parse(previous.created_at) ||
          (Date.parse(request.created_at) === Date.parse(previous.created_at) && request.request_id > previous.request_id)) {
        latest.set(request.message_id, request)
      }
    }
    for (const request of latest.values()) {
      const key = ids.has(request.reply_id)
        ? request.reply_id
        : request.message_id
      if (key) index.set(key, [...(index.get(key) || []), request])
    }
    return index
  }, [messages, requests])
  useLayoutEffect(() => {
    const list = messageListRef.current
    if (list && olderAnchor.current) {
      list.scrollTop =
        olderAnchor.current.top + list.scrollHeight - olderAnchor.current.height
      olderAnchor.current = null
    }
  }, [messages.length])
  return (
    <div className="h-full min-h-0 min-w-0 w-full flex overflow-hidden">
      {/* ── Mobile Contacts List View (100% width on < md when in 'contacts' state) ── */}
      <div
        className={cn(
          'w-full h-full min-[1200px]:hidden flex-col',
          mobileView === 'contacts' ? 'flex' : 'hidden',
        )}
      >
        <InboxSubPanel
          contacts={contacts}
          agents={agents}
          now={now}
          loading={contactsLoading}
          error={contactsError}
          onRetry={onRetry}
          activeContactId={activeContact?.id}
          onSelectContact={handleSelectContactInternal}
          selectedTab={contactTab}
          onTabChange={onContactTabChange}
          searchQuery={contactSearch}
          onSearchChange={onContactSearchChange}
        />
      </div>

      {/* ── Main Chat Stream Container (100% width on mobile when 'chat', flex-1 on desktop >= md) ── */}
      <div
        className={cn(
          'wa-chat flex-1 min-h-0 min-w-0 flex-col h-full overflow-hidden',
          mobileView === 'chat' ? 'flex' : 'hidden min-[1200px]:flex',
        )}
      >
        {!activeContact ? (
          <div className="h-full flex flex-col items-center justify-center p-8 text-center bg-background select-none">
            <div className="h-16 w-16 rounded-full bg-secondary flex items-center justify-center text-secondary-foreground mb-4 shadow-sm border border-border">
              <Bot className="h-8 w-8 text-primary" />
            </div>
            <h3 className="text-base font-bold text-foreground">
              Your conversations
            </h3>
            <p className="text-xs text-muted-foreground max-w-sm mt-1 leading-relaxed">
              Choose a conversation to continue, or start something new in Studio.
            </p>
          </div>
        ) : (
          <>
            {/* ── Top Header ── */}
            <div className="wa-chat-header h-14 sm:h-16 px-2 sm:px-4 flex items-center justify-between shrink-0">
              <div className="flex items-center gap-1.5 sm:gap-3 min-w-0">
                {/* Mobile Back Button (<) */}
                <button
                  type="button"
                  onClick={handleBackToContacts}
                  title="Back to conversations"
                  className="min-[1200px]:hidden inline-flex h-11 w-11 items-center justify-center -ml-1 text-muted-foreground hover:text-foreground hover:bg-muted rounded-full transition-colors shrink-0"
                >
                  <ChevronLeft className="h-5 w-5" />
                </button>

                <div className="relative shrink-0">
                  <div className="h-9 w-9 rounded-full bg-secondary flex items-center justify-center text-sm font-bold border border-border">
                    {activeContact.avatar ? (
                      <span>{activeContact.avatar}</span>
                    ) : activeContact.type === 'agent' ? (
                      <Bot className="h-4 w-4 text-primary" />
                    ) : (
                      <User className="h-4 w-4 text-muted-foreground" />
                    )}
                  </div>
                </div>

                <div className="min-w-0">
                  {activeContact.type === 'agent' && (
                    <Link
                      className="flex min-h-11 min-w-0 flex-col justify-center md:hidden"
                      aria-label="Manage agent connection"
                      to={`/settings/agents?agent=${encodeURIComponent(agentForContact(activeContact, agents)?.id || '')}&returnTo=${encodeURIComponent('/inbox/' + (activeContact.conversation_id || activeContact.id))}`}
                    >
                      <span className="truncate text-base font-semibold">
                        {activeContact.name}
                      </span>
                    </Link>
                  )}
                  <div
                    className={
                      activeContact.type === 'agent' ? 'hidden md:block' : ''
                    }
                  >
                    <h2 className="truncate text-base font-semibold leading-tight">
                      {activeContact.name}
                    </h2>
                    <ContactStatus
                      contact={activeContact}
                      agents={agents}
                      now={now}
                    />
                  </div>
                </div>
              </div>

              <div className="flex shrink-0 items-center gap-1">
                {activeContact.type === 'agent' && (
                  <Link
                    className="icon-action hidden md:inline-flex"
                    aria-label="Manage agent connection"
                    title="Manage agent connection"
                    to={`/settings/agents?agent=${encodeURIComponent(agentForContact(activeContact, agents)?.id || '')}&returnTo=${encodeURIComponent('/inbox/' + (activeContact.conversation_id || activeContact.id))}`}
                  >
                    <Settings className="h-5 w-5" />
                  </Link>
                )}
                <button
                  onClick={() =>
                    loadConversation(
                      activeContact.conversation_id || activeContact.id,
                      false,
                    )
                  }
                  title={t('Refresh messages')}
                  className="icon-action text-muted-foreground"
                >
                  <RefreshCw
                    className={cn('h-4 w-4', loading && 'animate-spin')}
                  />
                </button>
                <button
                  type="button"
                  onClick={() => setConvTab(convTab === 'tasks' ? 'chat' : 'tasks')}
                  title={convTab === 'tasks' ? t('Switch to chat') : t('Tasks')}
                  aria-label={convTab === 'tasks' ? t('Switch to chat') : t('Tasks')}
                  className={cn(
                    "icon-action relative transition-colors",
                    convTab === 'tasks'
                      ? "bg-accent text-primary"
                      : "text-muted-foreground hover:text-foreground"
                  )}
                >
                  <CheckSquare className="h-4 w-4" />
                  {linkedTasks.length > 0 && (
                    <span className={cn(
                      "absolute -top-1 -right-1 flex h-4 min-w-[16px] items-center justify-center rounded-full px-1 text-[10px] font-bold shadow-sm",
                      convTab === 'tasks'
                        ? "bg-primary text-primary-foreground"
                        : "bg-primary text-primary-foreground"
                    )}>
                      {linkedTasks.length}
                    </span>
                  )}
                </button>
              </div>
            </div>

            {/* ── Sub-tabs: Obrolan (Chat) & Pekerjaan (Tasks) ── */}
            <div className="flex shrink-0 items-center border-b border-border bg-background px-3 sm:px-4 text-xs font-medium">
              <button
                type="button"
                onClick={() => setConvTab('chat')}
                className={cn(
                  "flex items-center gap-1.5 py-2 px-3 border-b-2 transition-colors",
                  convTab === 'chat'
                    ? "border-primary text-primary font-semibold"
                    : "border-transparent text-muted-foreground hover:text-foreground"
                )}
              >
                <Bot className="h-4 w-4" />
                <span>{t('Chat')}</span>
              </button>
              <button
                type="button"
                onClick={() => setConvTab('tasks')}
                className={cn(
                  "flex items-center gap-1.5 py-2 px-3 border-b-2 transition-colors",
                  convTab === 'tasks'
                    ? "border-primary text-primary font-semibold"
                    : "border-transparent text-muted-foreground hover:text-foreground"
                )}
              >
                <CheckSquare className="h-4 w-4" />
                <span>{t('Tasks')}</span>
                {linkedTasks.length > 0 && (
                  <span className={cn(
                    "ml-1 rounded-full px-1.5 py-0.2 text-[10px] font-bold",
                    convTab === 'tasks'
                      ? "bg-primary text-primary-foreground"
                      : "bg-primary/10 text-primary"
                  )}>
                    {linkedTasks.length}
                  </span>
                )}
              </button>
            </div>

            {convTab === 'tasks' ? (
              <ConversationWorkMonitor
                tasks={linkedTasks}
                activeContact={activeContact}
                onRefresh={() =>
                  loadConversation(
                    activeContact.conversation_id || activeContact.id,
                    false,
                  )
                }
                onNewTask={() => {
                  const latest = messages[messages.length - 1]
                  setEscalatingMessage(
                    latest || ({
                      id: 0,
                      uuid: '',
                      author_type: 'user',
                      content: '',
                      created_at: new Date().toISOString(),
                    } as any)
                  )
                }}
                onBackToChat={() => setConvTab('chat')}
                loading={loading}
              />
            ) : (
              <>

            {/* ── Main Chat Stream (WhatsApp/Telegram Style) ── */}
            <div className="relative flex-1 min-h-0 min-w-0 flex flex-col">
              <div
                ref={messageListRef}
                onScroll={handleScroll}
                className="inbox-message-list wa-conversation min-h-0 flex-1 overflow-y-auto overscroll-contain px-3 py-3 sm:px-6 sm:py-4 space-y-2"
              >
              {messages.length === 0 && !loading && (
                <div className="flex flex-col items-center justify-center py-12 text-center text-muted-foreground space-y-2">
                  <Bot className="h-8 w-8 text-muted-foreground/60" />
                  <p className="text-xs">
                    {t('No messages with this contact yet.')}
                  </p>
                  <p className="text-[11px] text-muted-foreground/80">
                    {t('Type a message below to start a conversation.')}
                  </p>
                </div>
              )}

              {hasMoreOlder && (
                <div className="flex justify-center py-2">
                  <button
                    type="button"
                    disabled={loadingOlder}
                    className="min-h-11 rounded-md border border-border bg-card px-4 py-2 text-xs font-medium text-foreground hover:bg-muted active:scale-95 transition-all inline-flex items-center gap-2 shadow-sm disabled:opacity-50"
                    onClick={loadOlderMessages}
                  >
                    {loadingOlder && <RefreshCw className="h-3.5 w-3.5 animate-spin" />}
                    <span>
                      {loadingOlder
                        ? t('Loading older messages…')
                        : t('Load older messages')}
                    </span>
                  </button>
                </div>
              )}
              {messages.map((msg, index) => {
                // In WhatsApp/Telegram: User is right-aligned, Contact (Agent/Customer) is left-aligned
                const isUser =
                  msg.author_type === 'user' && activeContact.type === 'agent'
                const isOperatorReply =
                  msg.author_type === 'agent' &&
                  activeContact.type === 'customer'
                const isAgentMessage =
                  msg.author_type === 'agent' && activeContact.type === 'agent'
                const isFormattedMessage = true

                const isOutgoing = isUser || isOperatorReply
                const linkedTaskForMsg = msg.uuid
                  ? linkedTasks.find(
                      (task) =>
                        (task.source_message_id === msg.uuid ||
                          requests.some(
                            (q) =>
                              q.reply_id === msg.uuid &&
                              q.message_id === task.source_message_id,
                          )) &&
                        (isAgentMessage ||
                          !requests.some(
                            (q) =>
                              q.message_id === msg.uuid &&
                              visibleMessageIds.has(q.reply_id),
                          )),
                    )
                  : undefined

                // Suppress pure duplicate intermediate runner progress reports that do not carry
                // any linked task cards, drafts, action cards, or approval plans.
                const msgContent = msg.content || msg.message || ''
                if (
                  !linkedTaskForMsg &&
                  !msg.action_card &&
                  !msg.approval_plan &&
                  !msg.task_draft &&
                  (msg.author_name === 'Cuan' || msg.author_type === 'agent') &&
                  msgContent.includes('[View work](/tasks/') &&
                  !msgContent.includes('Work marked completed') &&
                  !msgContent.includes('stopped') &&
                  !msgContent.includes('cancelled')
                ) {
                  return null
                }
                const date = new Date(msg.created_at)
                const showDate =
                  Number.isFinite(date.getTime()) &&
                  (!index ||
                    date.toDateString() !==
                      new Date(messages[index - 1].created_at).toDateString())
                const today = new Date(),
                  yesterday = new Date()
                yesterday.setDate(today.getDate() - 1)
                const dayLabel =
                  date.toDateString() === today.toDateString()
                    ? 'Today'
                    : date.toDateString() === yesterday.toDateString()
                      ? 'Yesterday'
                      : date.toLocaleDateString(language, {
                          day: 'numeric',
                          month: 'long',
                          year: 'numeric',
                        })

                const quoteMessage = () => {
                  const raw = msg.content || msg.message || ''
                  const cleanLines = raw
                    .split('\n')
                    .filter((line) => !line.trim().startsWith('>'))
                  const textToQuote = (
                    cleanLines.length > 0 ? cleanLines.join('\n') : raw
                  ).trim()
                  const quote = textToQuote
                    .split('\n')
                    .map((line) => '> ' + line)
                    .join('\n')
                  setInputMessage(
                    (current) => (current ? current + '\n\n' : '') + quote + '\n\n',
                  )
                  textareaRef.current?.focus()
                }
                const metadata = (
                  <span className="wa-message-meta">
                    <time dateTime={msg.created_at}>
                      {formatMessageTime(msg.created_at)}
                    </time>
                    {isOutgoing && (
                      <Check
                        className="h-3.5 w-3.5"
                        role="img"
                        aria-label="Message saved"
                      />
                    )}
                  </span>
                )

                return (
                  <div
                    key={msg.uuid || msg.id || index}
                    data-message-id={msg.uuid}
                    data-search-message={
                      msg.uuid === requestedMessageId || undefined
                    }
                    className={cn(
                      'wa-message-row group relative flex flex-col',
                      isOutgoing ? 'items-end' : 'items-start',
                    )}
                  >
                    {showDate && (
                      <time
                        dateTime={msg.created_at}
                        className="wa-day-label mb-3 mt-1 self-center rounded-md px-3 py-1 text-xs"
                      >
                        {dayLabel}
                      </time>
                    )}
                    <ItemActions subtle swipeAction={{ label: 'Quote message', previewLabel: 'Reply', run: quoteMessage }} label={t('Message') + ' ' + String(msg.id || index + 1)} actions={[
                      { label: 'Copy message', run: () => navigator.clipboard.writeText(msg.content || msg.message) },
                      { label: 'Quote message', run: quoteMessage },
                      { label: 'Copy link', run: () => copyItemLink('/inbox/' + encodeURIComponent(activeContact.conversation_id || activeContact.id) + (msg.uuid ? '?message=' + encodeURIComponent(msg.uuid) : '')) },
                      ...(msg.uuid ? [{
                        label: t('Escalate to task'),
                        run: () => setEscalatingMessage(msg),
                      }] : []),
                    ]}>
                    <div
                      className={cn(
                        'wa-message-bubble relative min-w-0',
                        msg.uuid === requestedMessageId && 'ring-2 ring-ring',
                        isOutgoing ? 'wa-outgoing' : 'wa-incoming',
                      )}
                    >
                      {msg.approval_title && (
                        <p className="mb-2 text-xs font-medium">
                          {t(msg.approval_accepted ? 'Approved' : msg.plan_deferred ? 'Saved for later' : 'Needs approval')}
                        </p>
                      )}
                      {msg.attachments?.map((a) => (
                        <AttachmentFile key={a.id} attachment={a} />
                      ))}
                      {msg.attachment_url && !msg.attachments?.length && (
                        <AttachmentFile
                          attachment={{
                            id: msg.attachment_url,
                            name: msg.attachment_name || 'Customer file',
                            type: msg.attachment_type || 'application/octet-stream',
                            size: msg.attachment_size || 0,
                            url: msg.attachment_url,
                          }}
                        />
                      )}
                      {msg.product && (
                        <p className="mb-1 text-xs font-medium opacity-80">
                          {msg.product}
                        </p>
                      )}

                      {/* Message Body */}
                      {isFormattedMessage ? (
                        <MessageContent
                          content={msg.content || msg.message}
                          hideTaskActions={!!linkedTaskForMsg}
                        />
                      ) : (
                        <div className="wa-plain-text whitespace-pre-wrap break-words font-sans">
                          <span>{msg.content || msg.message}</span>
                          {metadata}
                        </div>
                      )}

                      {isAgentMessage && msg.approval_plan && (
                        <div className="mt-3 space-y-2 border-l-2 border-primary/30 pl-3">
                          <p className="text-sm font-medium">{msg.approval_plan.title}</p>
                          <p className="whitespace-pre-wrap break-words text-sm">{msg.approval_plan.description}</p>
                          <div className="flex flex-wrap gap-2">
                            <Button size="sm" className="min-h-11" disabled={msg.approval_accepted || sending || uploads.busy || retryCount >= 3 || !!pendingRef.current} onClick={() => handleSendMessage(msg.uuid, msg.approval_request_id)}>
                              {t(msg.approval_accepted ? 'Approved' : 'Approve and start')}
                            </Button>
                            {!msg.approval_accepted && <>
                              <Button size="sm" variant="outline" className="min-h-11" disabled={sending || !!pendingRef.current} onClick={() => {
                                setInputMessage(current => (current ? current + '\n\n' : '') + t('Changes to this plan') + ' — ' + msg.approval_plan!.title + ': ')
                                textareaRef.current?.focus()
                              }}>{t('Change plan')}</Button>
                              <Button size="sm" variant="ghost" className="min-h-11" disabled={msg.plan_deferred || sending || uploads.busy || retryCount >= 3 || !!pendingRef.current} onClick={() => handleSendMessage(msg.uuid, msg.plan_defer_request_id, 'defer')}>
                                {t(msg.plan_deferred ? 'Saved for later' : 'Not now')}
                              </Button>
                            </>}
                          </div>
                        </div>
                      )}

                      {/* 1-Click Execution Action Card for Agent Messages */}
                      {isAgentMessage && msg.action_card && (
                        <div className="mt-3 p-3 rounded-xl bg-background/90 border border-border text-foreground space-y-2.5 shadow-xs">
                          <div className="flex items-center justify-between gap-2 border-b border-border/60 pb-2">
                            <div className="flex items-center gap-1.5 font-bold text-xs">
                              <Sparkles className="h-3.5 w-3.5 text-primary" />
                              <span>Sprint execution plan</span>
                            </div>
                            <span className="text-[10px] font-mono px-2 py-0.5 rounded bg-secondary text-secondary-foreground">
                              {msg.action_card.worker_agent}
                            </span>
                          </div>

                          <div className="space-y-1.5">
                            <div className="text-[11px] font-semibold text-muted-foreground uppercase tracking-wider">
                              Definition of Done (DoD):
                            </div>
                            <div className="space-y-1 bg-muted/30 p-2 rounded-lg border border-border/40">
                              {msg.action_card.dod_checklist.map(
                                (item, idx) => (
                                  <div
                                    key={idx}
                                    className="flex items-start gap-2 text-[11px]"
                                  >
                                    <CheckSquare className="h-3.5 w-3.5 text-primary shrink-0 mt-0.5" />
                                    <span className="text-foreground">
                                      {item.text}
                                    </span>
                                  </div>
                                ),
                              )}
                            </div>
                          </div>

                          {/* 1-Click Button */}
                          <div className="pt-1">
                            {approvedTasks[msg.action_card.task_title] ? (
                              <div className="w-full py-2 px-3 rounded-lg bg-emerald-500/10 border border-emerald-500/30 text-emerald-600 dark:text-emerald-400 text-xs font-bold flex items-center justify-center gap-2">
                                <CheckCheck className="h-4 w-4" />
                                <span>Task started successfully!</span>
                              </div>
                            ) : (
                              <button
                                type="button"
                                disabled={
                                  approving === msg.action_card.task_title
                                }
                                onClick={() =>
                                  handle1ClickApprove(
                                    msg.action_card!,
                                    activeContact.conversation_id ||
                                      activeContact.id,
                                  )
                                }
                                className="w-full py-2 px-3 rounded-lg bg-primary text-primary-foreground font-bold text-xs hover:bg-primary/90 active:scale-98 transition-all flex items-center justify-center gap-2 disabled:opacity-50 cursor-pointer shadow-xs"
                              >
                                <Play className="h-3.5 w-3.5 fill-current" />
                                <span>
                                  {approving === msg.action_card.task_title
                                    ? 'Starting task…'
                                    : 'Approve and start task'}
                                </span>
                              </button>
                            )}
                          </div>
                        </div>
                      )}

                      {msg.uuid && (
                        <InboxTaskAction
                          linkedTask={linkedTaskForMsg}
                          conversationId={
                            activeContact.conversation_id || activeContact.id
                          }
                          messageId={msg.uuid}
                          draft={isAgentMessage ? msg.task_draft : undefined}
                        />
                      )}
                      {msg.uuid && linkedTasks.some((t) => t.source_message_id === msg.uuid) && (
                        <button
                          type="button"
                          onClick={() => setConvTab('tasks')}
                          className="mt-1.5 inline-flex items-center gap-1.5 rounded-full bg-emerald-500/10 border border-emerald-500/20 px-2 py-0.5 text-[10px] font-semibold text-emerald-600 dark:text-emerald-400 hover:bg-emerald-500/20 transition-colors cursor-pointer"
                          title={t('View in Tasks')}
                        >
                          <CheckSquare className="h-3 w-3" />
                          <span>{t('Assigned to task')}</span>
                        </button>
                      )}
                      {isFormattedMessage && (
                        <div className="wa-markdown-footer">{metadata}</div>
                      )}
                    </div>
                    </ItemActions>
                    {(requestsByMessage.get(msg.uuid || '') || []).map((q) => {
                      const placeholder =
                        (q.agent_id || activeContact.type === 'agent') &&
                        q.reply_id !== msg.uuid
                      const activity = (
                        <InboxActivity
                          request={q}
                          bubble={!!placeholder}
                          previousAttempts={(requestAttempts.get(q.message_id) || []).filter((attempt) => attempt.request_id !== q.request_id)}
                          onRetry={async () => {
                            const id = activeContact.conversation_id || activeContact.id
                            await retryInboxRequest(id, q.request_id)
                            await loadConversation(id)
                          }}
                          onCheckStatus={async () => {
                            await loadConversation(activeContact.conversation_id || activeContact.id)
                          }}
                          onCancel={async () => {
                            const id =
                              activeContact.conversation_id || activeContact.id
                            await cancelInboxRequest(id, q.request_id)
                            await loadConversation(id)
                          }}
                        />
                      )
                      return placeholder ? (
                        <div
                          key={q.request_id}
                          aria-label="Agent response progress"
                          className="mt-3 flex w-full self-stretch flex-col items-start"
                        >
                          <div className="wa-message-bubble wa-incoming relative min-w-0">
                            {activity}
                          </div>
                        </div>
                      ) : (
                        <div key={q.request_id} className="max-w-full">
                          {activity}
                        </div>
                      )
                    })}
                  </div>
                )
              })}

              {sendError && (
                <div
                  role="alert"
                  className="mx-auto max-w-2xl rounded-xl border border-border bg-card p-3 text-xs"
                >
                  <p>{sendError}</p>
                  {pendingRef.current && (
                    <button
                      className="mt-2 underline"
                      disabled={sending || retryCount >= 3}
                      onClick={() => handleSendMessage()}
                    >
                      Check the same message ({Math.max(0, 3 - retryCount)}{' '}
                      remaining)
                    </button>
                  )}
                  <button
                    className="ml-3 underline"
                    onClick={() =>
                      loadConversation(
                        activeContact.conversation_id || activeContact.id,
                      )
                    }
                  >
                    Refresh history
                  </button>
                </div>
              )}

              {/* Typing Indicator */}
              {sending && activeContact.type === 'agent' && (
                <div className="flex flex-col items-start animate-fade-in">
                  <div className="bg-card text-card-foreground border border-border rounded-2xl rounded-tl-xs p-3 shadow-xs flex items-center gap-2.5">
                    <div className="h-6 w-6 rounded-full bg-secondary flex items-center justify-center text-xs">
                      {activeContact.avatar ? (
                        <span>{activeContact.avatar}</span>
                      ) : (
                        <Bot className="h-3.5 w-3.5 text-primary" />
                      )}
                    </div>
                    <span className="text-xs text-muted-foreground font-medium">
                      Sending message to {activeContact.name}...
                    </span>
                    <div className="flex items-center gap-1 ml-1">
                      <span
                        className="h-1.5 w-1.5 rounded-full bg-primary/70 animate-bounce"
                        style={{ animationDelay: '0ms' }}
                      />
                      <span
                        className="h-1.5 w-1.5 rounded-full bg-primary/70 animate-bounce"
                        style={{ animationDelay: '150ms' }}
                      />
                      <span
                        className="h-1.5 w-1.5 rounded-full bg-primary/70 animate-bounce"
                        style={{ animationDelay: '300ms' }}
                      />
                    </div>
                  </div>
                </div>
              )}
            </div>

            {/* ── Floating 'Scroll to Bottom' Button ── */}
            <div
              className="pointer-events-none absolute inset-x-0 bottom-3 z-30 px-3 sm:px-6"
              aria-hidden={!showScrollBottom}
            >
              <div className="mx-auto flex max-w-4xl justify-end">
                <button
                  type="button"
                  data-testid="scroll-to-bottom-button"
                  onClick={handleScrollToBottom}
                  tabIndex={showScrollBottom ? 0 : -1}
                  aria-label={
                    hasNewMessages
                      ? `${unreadCount || 1} ${t('new messages')}. ${t('Scroll to bottom')}`
                      : t('Scroll to bottom')
                  }
                  title={
                    hasNewMessages
                      ? `${unreadCount || 1} ${t('new messages')}. ${t('Scroll to bottom')}`
                      : t('Scroll to bottom')
                  }
                  className={cn(
                    'pointer-events-auto relative flex items-center justify-center rounded-full wa-scroll-btn cursor-pointer select-none',
                    'h-10 w-10 sm:h-11 sm:w-11',
                    'focus:outline-none focus-visible:ring-2 focus-visible:ring-primary',
                    showScrollBottom
                      ? 'opacity-100 translate-y-0 scale-100'
                      : 'opacity-0 translate-y-3 scale-90 pointer-events-none',
                  )}
                >
                  <ChevronDown className="h-5 w-5 sm:h-5.5 sm:w-5.5" />
                  {hasNewMessages && (
                    <span
                      data-testid="scroll-to-bottom-badge"
                      aria-label={`${unreadCount || 1} ${t('new messages')}`}
                      className="wa-scroll-badge absolute -top-1.5 -right-1.5 flex h-5 min-w-[20px] sm:h-5.5 sm:min-w-[22px] items-center justify-center rounded-full px-1 text-[10px] sm:text-[11px] font-bold"
                    >
                      {unreadCount > 99 ? '99+' : unreadCount || 1}
                    </span>
                  )}
                </button>
              </div>
            </div>
          </div>
            {streamState !== 'live' && (
              <p
                className="px-4 py-1 text-xs text-muted-foreground"
                role="status"
              >
                {streamState === 'login'
                  ? t('Your session has expired. Sign in again to continue.')
                  : t('Connecting… Your conversation history is saved.')}
              </p>
            )}
            {/* ── Modern Composer (Bottom) ── */}
            <div
              data-inbox-composer
              className="wa-composer shrink-0 z-20"
              onPaste={(e) => {
                if (!sending && activeContact.type === 'agent')
                  uploads.onPaste(e)
              }}
              onDrop={(e) => {
                e.preventDefault()
                if (!sending && activeContact.type === 'agent')
                  uploads.onDrop(e)
              }}
              onDragOver={uploads.onDragOver}
            >
              <div className="mx-auto max-w-4xl space-y-2">
                {activeContact.type === 'agent' && (
                  <AttachmentPreviews uploads={uploads} disabled={sending} />
                )}
                <div
                  className={`wa-input-capsule flex min-w-0 items-end gap-1 sm:gap-1.5 p-1 sm:p-1.5 ${
                    isMultiline ? 'is-multiline' : ''
                  }`}
                >
                  {activeContact.type === 'agent' && (
                    <AttachmentPicker
                      key={activeContact.conversation_id}
                      uploads={uploads}
                      disabled={sending}
                      compact
                      disableImages={!contactSupportsVision}
                    />
                  )}
                  <textarea
                    ref={textareaRef}
                    data-primary-composer
                    aria-label={t('Message')}
                    rows={1}
                    value={inputMessage}
                    onChange={(e) => {
                      unsentDrafts.current[activeConvRef.current] =
                        e.target.value
                      setInputMessage(e.target.value)
                      autoResizeTextarea(e.target)
                    }}
                    onKeyDown={handleKeyDown}
                    placeholder={
                      activeContact.type === 'agent'
                        ? t('Write a message…')
                        : `Reply to ${activeContact.name}…`
                    }
                    className={`wa-input min-w-0 flex-1 min-h-[36px] sm:min-h-[38px] max-h-[140px] resize-none overflow-hidden py-2 sm:py-2 text-[15px] sm:text-sm leading-5 sm:leading-snug ${
                      activeContact.type === 'agent'
                        ? 'pl-1.5 pr-2 sm:pl-2 sm:pr-2.5'
                        : 'pl-3 pr-2 sm:pl-3.5 sm:pr-2.5'
                    }`}
                  />

                  <CannedResponsesPicker
                    inputMessage={inputMessage}
                    onSelect={(content) => {
                      unsentDrafts.current[activeConvRef.current] = content
                      setInputMessage(content)
                      if (textareaRef.current) {
                        textareaRef.current.value = content
                        autoResizeTextarea(textareaRef.current)
                      }
                      textareaRef.current?.focus()
                    }}
                    onOpenManage={() => setShowCannedManageModal(true)}
                    disabled={sending}
                  />

                  <button
                    type="button"
                    onClick={() => handleSendMessage()}
                    disabled={
                      (!inputMessage.trim() &&
                        (activeContact.type !== 'agent' || !uploads.items.length)) ||
                      sending ||
                      uploads.busy
                    }
                    className="wa-send h-9 w-9 sm:h-9.5 sm:w-9.5 rounded-full flex items-center justify-center active:scale-95 disabled:opacity-40 disabled:pointer-events-none transition-all cursor-pointer shrink-0 mr-0.5"
                    title={t('Send message (Enter)')}
                    aria-label={t('Send message')}
                  >
                    <Send className="h-[18px] w-[18px]" />
                  </button>
                </div>
              </div>
            </div>
              </>
            )}
          </>
        )}
      </div>

      <ActionDialog
        open={showTasksModal}
        title={t('Connected tasks')}
        onClose={() => setShowTasksModal(false)}
      >
        <div className="space-y-3">
          {linkedTasks.length === 0 ? (
            <div className="flex flex-col items-center justify-center py-8 text-center text-muted-foreground space-y-2">
              <CheckSquare className="h-8 w-8 text-muted-foreground/50" />
              <p className="text-sm font-medium">{t('No tasks connected to this conversation')}</p>
              <p className="text-xs text-muted-foreground/80 max-w-xs">
                {t('Tasks created by agents during this conversation will appear here.')}
              </p>
            </div>
          ) : (
            <div className="space-y-3">
              {linkedTasks.map((task) => (
                <TaskItemCard
                  key={task.id}
                  task={task}
                  returnTo={location.pathname}
                  onNavigate={() => setShowTasksModal(false)}
                />
              ))}
            </div>
          )}
        </div>
      </ActionDialog>

      <EscalateTaskDialog
        open={!!escalatingMessage}
        message={escalatingMessage}
        conversationId={activeContact?.conversation_id || activeContact?.id || ''}
        onClose={() => setEscalatingMessage(null)}
        onTaskCreated={(newTask) => {
          setLinkedTasks((prev) => [newTask, ...prev.filter((t) => t.id !== newTask.id)])
          setConvTab('tasks')
        }}
      />

      <CannedResponsesDialog
        open={showCannedManageModal}
        onClose={() => setShowCannedManageModal(false)}
        onSelect={(content) => {
          setInputMessage(content)
          textareaRef.current?.focus()
          if (textareaRef.current) autoResizeTextarea(textareaRef.current)
        }}
      />
    </div>
  )
}
