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
import { Link, useLocation, useNavigate } from 'react-router-dom'
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
  lazy,
  Suspense,
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
  EyeOff,
  Lock,
  Copy,
  Plus,
  Clock,
  CheckCircle2,
  ListChecks,
  GitBranch,
  ExternalLink,
  ArrowRight,
  Terminal,
  Rocket,
  ShieldCheck,
} from 'lucide-react'
import { Task } from '../../api/types'
import { tasksApi } from '../../api/tasks'
import './chat-calm.css'
import { StudioView } from './StudioView'
import { MessageContent } from './MessageContent'
import { InboxActivity } from './InboxActivity'
import { InboxTaskAction } from './InboxTaskAction'
import { ApprovalCard } from './ApprovalCard'
import { ReceiptPreviewCard, parseReceiptExtraction } from './ReceiptPreviewCard'
import { SlideDrawer } from '../../components/layout/SlideDrawer'
import { ChatMarkdown } from './ChatMarkdown'
import { parseTaskReviewEnvelope } from './task-envelope'
import {
  taskRef,
  statusLabel,
  taskTargetUrl,
  taskTerminalUrl,
  relativeTime,
  agentDisplayName,
} from '../tasks/task-ui'
import { isTaskActive } from './task-progress'
import { TaskCompletionCard } from './TaskCompletionCard'
import { notifyTaskCompleted } from './task-notifications'
import { cn } from '../../lib/utils'
import { InboxSubPanel, ContactFilterTab } from './InboxSubPanel'

const TaskTerminal = lazy(() =>
  import('../tasks/TaskTerminal').then((m) => ({
    default: m.TaskTerminal,
  })),
)

const CannedResponsesDialog = lazy(() =>
  import('./CannedResponsesDialog').then((m) => ({
    default: m.CannedResponsesDialog,
  })),
)
const EscalateTaskDialog = lazy(() =>
  import('./EscalateTaskDialog').then((m) => ({
    default: m.EscalateTaskDialog,
  })),
)

interface InboxViewProps {
  onNewMessage?: () => void
  onNewStudio?: () => void
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
  initialMode?: 'chat' | 'studio'
  // Studio Canvas props
  agentSetup?: 'loading' | 'needed' | 'ready' | 'error'
  firstGoal?: boolean
  hasProject?: boolean
  onAgentConnected?: () => void
  onCreateProject?: () => void
  checkRuntime?: boolean
  onRetrySetup?: () => void
}

export function InboxView({
  onNewMessage: _onNewMessage,
  onNewStudio: _onNewStudio,
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
  initialMode,
  agentSetup = 'ready',
  firstGoal = false,
  hasProject = true,
  onAgentConnected,
  onCreateProject,
  checkRuntime = true,
  onRetrySetup,
}: InboxViewProps) {
  const navigate = useNavigate()
  const clock = useConnectionClock(agents)
  const now = suppliedNow ?? clock
  const location = useLocation()
  const isChatRoute = location.pathname.startsWith('/chat')
  const [canvasMode, setCanvasMode] = useState<'chat' | 'studio'>(() => {
    if (initialMode === 'studio' || isChatRoute || !activeContact) return 'studio'
    return 'chat'
  })

  useEffect(() => {
    if (location.pathname.startsWith('/chat') && !location.pathname.startsWith('/chat/')) {
      setCanvasMode('studio')
    } else if (activeContact) {
      setCanvasMode('chat')
    } else if (!activeContact) {
      setCanvasMode('studio')
    }
  }, [location.pathname, activeContact])

  const handleStudioConversationCreated = useCallback((conversationId: string) => {
    setCanvasMode('chat')
    setMobileView('chat')
    onMobileChatActiveChange?.(true)
    onContactUpdated?.()
    const target = contacts.find(
      (c) => (c.conversation_id || c.id) === conversationId,
    )
    if (target) {
      onSelectContact?.(target)
    } else {
      onSelectContact?.({
        id: conversationId,
        conversation_id: conversationId,
        name: 'Agent Task',
        type: 'agent',
        avatar: '',
        unread_count: 0,
        status: 'online',
        updated_at: new Date().toISOString(),
      })
    }
    navigate('/inbox/' + encodeURIComponent(conversationId), { replace: true })
  }, [contacts, onSelectContact, onMobileChatActiveChange, onContactUpdated, navigate])

  const handleOpenStudio = useCallback(() => {
    navigate('/studio' + location.search)
  }, [navigate, location.search])
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
  const activeConversationTasks = useMemo(() => {
    return linkedTasks.filter(isTaskActive)
  }, [linkedTasks])

  // Track task completion transitions to trigger audio chime & background tab badge
  const prevActiveTasksRef = useRef<Set<string>>(new Set())
  useEffect(() => {
    const currentActiveIds = new Set(activeConversationTasks.map((t) => t.id))
    if (prevActiveTasksRef.current.size > 0) {
      let newlyCompletedCount = 0
      for (const id of prevActiveTasksRef.current) {
        if (!currentActiveIds.has(id)) {
          const taskObj = linkedTasks.find((t) => t.id === id)
          const st = (taskObj?.status || '').toLowerCase().trim()
          if (st === 'done' || st === 'review') {
            newlyCompletedCount++
          }
        }
      }
      if (newlyCompletedCount > 0) {
        notifyTaskCompleted({ count: newlyCompletedCount })
      }
    }
    prevActiveTasksRef.current = currentActiveIds
  }, [activeConversationTasks, linkedTasks])
  const [peekTask, setPeekTask] = useState<Task | null>(null)
  const [peekTab, setPeekTab] = useState<'details' | 'terminal'>('details')
  const [_peekTaskLoading, setPeekTaskLoading] = useState(false)
  const [escalatingMessage, setEscalatingMessage] = useState<ChatMessage | null>(null)

  const handleOpenTaskDrawer = useCallback(
    async (taskOrId: Task | string, tab: 'details' | 'terminal' = 'details') => {
      setPeekTab(tab)
      if (typeof taskOrId === 'string') {
        const existing = linkedTasks.find((t) => t.id === taskOrId)
        if (existing) {
          setPeekTask(existing)
          return
        }
        setPeekTaskLoading(true)
        try {
          const fetched = await tasksApi.get(taskOrId)
          if (fetched) setPeekTask(fetched)
        } catch {
          // ignore error
        } finally {
          setPeekTaskLoading(false)
        }
      } else {
        setPeekTask(taskOrId)
      }
    },
    [linkedTasks]
  )

  const handleUseDraft = useCallback((draftText: string) => {
    if (!draftText) return
    setInputMessage(draftText)
    if (textareaRef.current) {
      textareaRef.current.focus()
      autoResizeTextarea(textareaRef.current)
    }
  }, [])

  const handleTaskRetry = useCallback(
    async (taskToRetry: Task) => {
      const agentId =
        taskToRetry.latest_attempt?.agent_name ||
        taskToRetry.assignee_id ||
        taskToRetry.assignee ||
        'agy'
      await tasksApi.runAgent(taskToRetry.id, {
        request_id: crypto.randomUUID(),
        agent_id: agentId,
      })
      const updated = await tasksApi.get(taskToRetry.id)
      if (updated) {
        setLinkedTasks((prev) =>
          prev.map((t) => (t.id === updated.id ? updated : t))
        )
        if (peekTask?.id === updated.id) {
          setPeekTask(updated)
        }
      }
    },
    [peekTask?.id]
  )

  const handleMessageContainerClick = useCallback(
    (e: React.MouseEvent) => {
      const target = e.target as HTMLElement | null
      const anchor = target?.closest('a')
      if (anchor) {
        const href = anchor.getAttribute('href') || ''
        const match = href.split(/[?#]/)[0].match(/^\/tasks\/([A-Za-z0-9_-]+)$/)
        if (match && match[1]) {
          e.preventDefault()
          e.stopPropagation()
          handleOpenTaskDrawer(match[1])
        }
      }
    },
    [handleOpenTaskDrawer]
  )

  const [copiedEmail, setCopiedEmail] = useState(false)
  const copyEmailTimeoutRef = useRef<ReturnType<typeof setTimeout> | null>(null)

  useEffect(() => {
    setCopiedEmail(false)
    if (copyEmailTimeoutRef.current) {
      clearTimeout(copyEmailTimeoutRef.current)
      copyEmailTimeoutRef.current = null
    }
  }, [activeContact?.id])

  useEffect(() => {
    return () => {
      if (copyEmailTimeoutRef.current) {
        clearTimeout(copyEmailTimeoutRef.current)
      }
    }
  }, [])

  const handleCopyCustomerEmail = useCallback(async (e: React.MouseEvent) => {
    e.preventDefault()
    e.stopPropagation()
    if (!activeContact?.email) return
    try {
      if (typeof navigator !== 'undefined' && navigator.clipboard?.writeText) {
        await navigator.clipboard.writeText(activeContact.email)
      }
    } catch {
      // ignore
    }
    setCopiedEmail(true)
    if (copyEmailTimeoutRef.current) {
      clearTimeout(copyEmailTimeoutRef.current)
    }
    copyEmailTimeoutRef.current = setTimeout(() => {
      setCopiedEmail(false)
      copyEmailTimeoutRef.current = null
    }, 2000)
  }, [activeContact?.email])
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
  const unattachedCompletedTasks = useMemo(() => {
    const visibleMsgIds = new Set(messages.map((m) => m.uuid).filter(Boolean))
    return linkedTasks.filter((t) => {
      const isCompleted =
        (t.status || '').toLowerCase().trim() === 'done' ||
        (t.status || '').toLowerCase().trim() === 'review'
      if (!isCompleted) return false
      if (t.source_message_id && visibleMsgIds.has(t.source_message_id)) return false
      return true
    })
  }, [linkedTasks, messages])
  const [inputMessage, setInputMessage] = useState('')
  const uploads = useAttachments(
    'cuan-inbox-attachments:' +
      (activeContact?.conversation_id || activeContact?.id || ''),
  )

  const activeAgent = agents?.find((a) => a.id === activeContact?.id || a.id === activeContact?.agent_id || a.name === activeContact?.name)
  const contactDisplayName = (() => {
    if (!activeContact) return ''
    if (activeContact.type === 'agent') {
      const matched = agentForContact(activeContact, agents) || activeAgent
      if (matched?.name) return matched.name
      if (activeContact.agent_name) return activeContact.agent_name
      // If activeContact.name is a technical task title, fallback to friendly agent name
      if (
        activeContact.name &&
        !activeContact.id.startsWith('topic_') &&
        !activeContact.name.startsWith('Verifikasi') &&
        !activeContact.name.startsWith('Task ')
      ) {
        return activeContact.name
      }
      return 'Hermes Support'
    }
    return activeContact.name || t('Customer')
  })()
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
  const [whisperMode, setWhisperMode] = useState(false)
  const [whisperHandle, setWhisperHandle] = useState<'@team' | '@sm' | '@dev'>('@team')

  useEffect(() => {
    if (activeContact?.type === 'customer') {
      const trimmed = inputMessage.trim().toLowerCase()
      if (trimmed.startsWith('@team')) {
        setWhisperMode(true)
        setWhisperHandle('@team')
      } else if (trimmed.startsWith('@sm')) {
        setWhisperMode(true)
        setWhisperHandle('@sm')
      } else if (trimmed.startsWith('@dev')) {
        setWhisperMode(true)
        setWhisperHandle('@dev')
      }
    }
  }, [inputMessage, activeContact?.type])

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
        if (activeContact && data.tasks) {
          const at = data.tasks.filter((t) => {
            const s = (t.status || '').toLowerCase().trim()
            return s !== 'done' && s !== 'blocked' && s !== 'failed' && s !== 'queued' && s !== 'pending'
          }).length
          const qt = data.tasks.filter((t) => {
            const s = (t.status || '').toLowerCase().trim()
            return s === 'queued' || s === 'pending'
          }).length
          const dt = data.tasks.filter((t) => (t.status || '').toLowerCase().trim() === 'done').length
          const ft = data.tasks.filter((t) => {
            const s = (t.status || '').toLowerCase().trim()
            return s === 'blocked' || s === 'failed'
          }).length
          const changed =
            activeContact.task_count !== data.tasks.length ||
            activeContact.active_tasks !== at ||
            activeContact.queued_tasks !== qt ||
            activeContact.done_tasks !== dt ||
            activeContact.failed_tasks !== ft
          activeContact.task_count = data.tasks.length
          activeContact.active_tasks = at
          activeContact.queued_tasks = qt
          activeContact.done_tasks = dt
          activeContact.failed_tasks = ft
          if (changed) {
            onContactUpdated?.()
          }
        }
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

  // Adaptive task poller: 3-5s (4000ms) when in-flight tasks exist, idle backoff (20000ms) otherwise
  const hasActiveTasks = activeConversationTasks.length > 0
  useEffect(() => {
    if (!activeContact) return
    const convId = activeContact.conversation_id || activeContact.id
    if (!convId) return

    const intervalMs = hasActiveTasks ? 4000 : 20000

    const timer = setInterval(() => {
      if (typeof document !== 'undefined' && document.visibilityState === 'visible') {
        void loadConversation(convId)
      }
    }, intervalMs)

    return () => clearInterval(timer)
  }, [activeContact?.conversation_id, activeContact?.id, hasActiveTasks, loadConversation])

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
    let text = pending?.convId === convId ? pending.text : approveMessageId ? t(planAction === 'defer' ? 'Not now' : 'Approve this plan') : inputMessage.trim()
    if (!approveMessageId && activeContact.type === 'customer' && whisperMode && text && !text.startsWith('@')) {
      text = `${whisperHandle} ${text}`
    }
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
              approveMessageId
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
          'w-full h-full min-[1200px]:hidden flex-col relative',
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
          onNewStudio={handleOpenStudio}
          onNewMessage={handleOpenStudio}
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
        {canvasMode === 'studio' || !activeContact ? (
          <div data-testid="studio-dynamic-canvas" className="h-full flex-1 overflow-y-auto px-4 py-6 md:px-8 bg-background">
            <div className="calm-stream-constraint max-w-2xl mx-auto">
              <StudioView
                onAccepted={() => onContactUpdated?.()}
                onConversationCreated={handleStudioConversationCreated}
                agentSetup={agentSetup}
                firstGoal={firstGoal}
                hasProject={hasProject}
                onAgentConnected={onAgentConnected ?? (() => {})}
                onCreateProject={onCreateProject ?? (() => {})}
                onRetrySetup={onRetrySetup ?? (() => {})}
                checkRuntime={checkRuntime}
                autoFocus={true}
              />
            </div>
          </div>
        ) : (
          <>
            {/* ── Top Header ── */}
            <div className="wa-chat-header h-14 sm:h-16 px-2 sm:px-4 flex items-center justify-between shrink-0 border-b-0">
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

                <div className="min-w-0 flex flex-col justify-center">
                  <div className="flex items-center gap-1.5 sm:gap-2 min-w-0 flex-wrap sm:flex-nowrap">
                    <h2 className="truncate text-base font-semibold leading-tight">
                      {contactDisplayName || activeContact.name}
                    </h2>
                    {activeContact.type === 'customer' && activeContact.email && (
                      <div className="flex items-center gap-1 min-w-0" data-testid="customer-email-header">
                        <span className="text-muted-foreground text-xs hidden sm:inline" aria-hidden="true">·</span>
                        <span
                          className="truncate text-xs text-muted-foreground font-normal max-w-[140px] sm:max-w-[200px]"
                          title={activeContact.email}
                          data-testid="customer-email-text"
                        >
                          {activeContact.email}
                        </span>
                        <button
                          type="button"
                          onClick={handleCopyCustomerEmail}
                          aria-label={copiedEmail ? t('Disalin!') : t('Copy email')}
                          title={copiedEmail ? t('Disalin!') : `${t('Copy email')}: ${activeContact.email}`}
                          data-testid="copy-customer-email-button"
                          className={cn(
                            "inline-flex min-h-11 min-w-11 items-center justify-center rounded-md px-1.5 py-1 text-xs font-medium transition-colors cursor-pointer shrink-0",
                            copiedEmail
                              ? "text-emerald-600 dark:text-emerald-400 bg-emerald-500/10"
                              : "text-muted-foreground hover:text-foreground hover:bg-muted"
                          )}
                        >
                          {copiedEmail ? (
                            <span className="inline-flex items-center gap-1 text-[11px] font-medium text-emerald-600 dark:text-emerald-400">
                              <Check className="h-3.5 w-3.5 shrink-0" />
                              <span>{t('Disalin!')}</span>
                            </span>
                          ) : (
                            <Copy className="h-3.5 w-3.5 shrink-0" />
                          )}
                        </button>
                      </div>
                    )}
                  </div>
                  <ContactStatus
                    contact={activeContact}
                    agents={agents}
                    now={now}
                  />
                </div>
              </div>

              <div className="flex shrink-0 items-center gap-1">
                {activeContact.type === 'customer' && (
                  <Link
                    className="icon-action text-muted-foreground hover:text-foreground"
                    aria-label="View contact details"
                    title={t('View contact details & memory')}
                    to={`/contacts?contact=${encodeURIComponent(activeContact.id)}`}
                  >
                    <User className="h-4 w-4" />
                  </Link>
                )}
                {activeContact.type === 'agent' && (
                  <Link
                    className="icon-action inline-flex"
                    aria-label="Manage agent connection"
                    title="Manage agent connection"
                    to={`/settings/agents?agent=${encodeURIComponent(agentForContact(activeContact, agents)?.id || '')}&returnTo=${encodeURIComponent('/inbox/' + (activeContact.conversation_id || activeContact.id))}`}
                  >
                    <Settings className="h-5 w-5" />
                  </Link>
                )}
                <button
                  type="button"
                  data-testid="header-new-studio-btn"
                  onClick={handleOpenStudio}
                  title={t('Ide / Tugas Baru (Cmd+K)')}
                  aria-label={t('Ide / Tugas Baru (Cmd+K)')}
                  className="hidden md:inline-flex items-center gap-1.5 px-2.5 py-1.5 rounded-lg border border-border/60 bg-muted/30 text-xs font-medium text-muted-foreground hover:text-foreground hover:bg-muted/70 transition-colors"
                >
                  <Plus className="h-3.5 w-3.5 text-primary" />
                  <span>{t('Ide Baru')}</span>
                </button>
              </div>
            </div>

            {/* Sub-tabs: Obrolan (Chat) & Pekerjaan (Tasks) eliminated for serene canvas in S59-T6 */}

            {/* ── Main Chat Stream (WhatsApp/Telegram Style) ── */}
            <div className="relative flex-1 min-h-0 min-w-0 flex flex-col">
              <div
                ref={messageListRef}
                onScroll={handleScroll}
                onClick={handleMessageContainerClick}
                className="inbox-message-list wa-conversation calm-conversation min-h-0 flex-1 overflow-y-auto overscroll-contain px-3 py-3 sm:px-6 sm:py-4 space-y-2"
              >
                <div className="calm-stream-constraint max-w-[75ch] mx-auto w-full space-y-2">
              {messages.length === 0 && !loading && (
                <div className="flex flex-col items-center justify-center py-10 px-4 text-center text-muted-foreground space-y-3 max-w-sm mx-auto">
                  {activeContact?.type === 'customer' ? (
                    <>
                      <div className="relative">
                        <div className="h-12 w-12 rounded-full bg-secondary flex items-center justify-center text-lg border border-border shadow-xs">
                          {activeContact.avatar ? (
                            <span>{activeContact.avatar}</span>
                          ) : (
                            <User className="h-6 w-6 text-muted-foreground" />
                          )}
                        </div>
                        {activeContact.is_online && (
                          <span
                            className="absolute bottom-0 right-0 block h-3 w-3 rounded-full bg-emerald-500 ring-2 ring-background"
                            title={t('Online on website')}
                          />
                        )}
                      </div>
                      <div className="space-y-1">
                        <p className="text-sm font-semibold text-foreground">
                          {activeContact.is_online
                            ? `${activeContact.name} ${t('is online right now')}`
                            : activeContact.name}
                        </p>
                        {activeContact.page_title || activeContact.current_url || activeContact.product ? (
                          <p className="text-xs text-muted-foreground inline-flex items-center gap-1 px-2.5 py-1 rounded-full bg-muted/60 border border-border/40">
                            <span className="truncate max-w-[240px]">
                              {activeContact.page_title || activeContact.current_url || activeContact.product}
                            </span>
                          </p>
                        ) : (
                          <p className="text-xs text-muted-foreground">
                            {t('No messages with this contact yet.')}
                          </p>
                        )}
                      </div>
                      <p className="text-xs text-muted-foreground/80">
                        {t('Send a greeting to start a proactive chat with this visitor.')}
                      </p>
                      {/* Quick proactive greetings */}
                      <div className="flex flex-wrap justify-center gap-1.5 pt-1">
                        <button
                          type="button"
                          onClick={() => {
                            setInputMessage(t('Hello! Anything we can help you with today?'))
                            textareaRef.current?.focus()
                          }}
                          className="px-2.5 py-1 text-xs rounded-full border border-border bg-background hover:bg-accent hover:text-accent-foreground transition active:scale-95"
                        >
                          👋 {t('Say hello')}
                        </button>
                        <button
                          type="button"
                          onClick={() => {
                            setInputMessage(t('Hi there! Looking for specific information or need help?'))
                            textareaRef.current?.focus()
                          }}
                          className="px-2.5 py-1 text-xs rounded-full border border-border bg-background hover:bg-accent hover:text-accent-foreground transition active:scale-95"
                        >
                          💬 {t('Offer assistance')}
                        </button>
                      </div>
                    </>
                  ) : (
                    <>
                      <Bot className="h-8 w-8 text-muted-foreground/60" />
                      <p className="text-xs">
                        {t('No messages with this contact yet.')}
                      </p>
                      <p className="text-[11px] text-muted-foreground/80">
                        {t('Type a message below to start a conversation.')}
                      </p>
                    </>
                  )}
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
                // In WhatsApp/Telegram: User/Operator is right-aligned, Contact (Agent/Customer) is left-aligned
                const isUserInAgentRoom =
                  msg.author_type === 'user' && activeContact.type === 'agent'
                const isOperatorInCustomerRoom =
                  activeContact.type === 'customer' &&
                  (msg.sender_type === 'operator' ||
                    msg.author_name === 'You' ||
                    msg.author_name === 'Owner' ||
                    msg.author_id === 'operator_user') &&
                  !(msg.author_type === 'agent' && msg.is_internal) &&
                  msg.author_name !== msg.mention_handle &&
                  !msg.author_name?.startsWith('@')
                const isAgentMessage =
                  msg.author_type === 'agent' && activeContact.type === 'agent'
                const isFormattedMessage = true

                const isOutgoing = isUserInAgentRoom || isOperatorInCustomerRoom
                const msgContent = msg.content || msg.message || ''

                // Suppress internal system cues (e.g. cuan_work_review)
                if (
                  msg.author_id === 'cuan_work_review' ||
                  msgContent.startsWith('I’m checking the latest work update') ||
                  msgContent.startsWith("I'm checking the latest work update") ||
                  msgContent.startsWith('Review the result for [')
                ) {
                  return null
                }

                const linkedTaskForMsg = msg.uuid
                  ? linkedTasks.find(
                      (task) =>
                        (task.source_message_id === msg.uuid ||
                          msgContent.includes(`/tasks/${task.id}`) ||
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

                // Suppress obsolete stopped launch notifications if the task already succeeded/review/done
                // or if there is a DoD evaluation or completion in the room
                if (msgContent.includes('This pending launch has stopped') || msgContent.includes('Pekerjaan telah dihentikan')) {
                  const taskIdMatch = msgContent.match(/\/tasks\/([A-Za-z0-9_-]+)/)
                  if (taskIdMatch) {
                    const matchedTask = linkedTasks.find((t) => t.id === taskIdMatch[1])
                    if (
                      matchedTask &&
                      (matchedTask.status === 'review' ||
                        matchedTask.status === 'done' ||
                        matchedTask.status === 'doing')
                    ) {
                      return null
                    }
                  }
                  const hasSubsequentReport = messages.some(
                    (m) =>
                      new Date(m.created_at).getTime() >=
                        new Date(msg.created_at).getTime() &&
                      (m.content?.includes('Definition of Done') ||
                        m.content?.includes('Evaluasi') ||
                        m.content?.includes('Work marked completed') ||
                        m.content?.includes('Pekerjaan ditandai selesai')),
                  )
                  if (hasSubsequentReport) {
                    return null
                  }
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
                        className="wa-day-label calm-day-label my-3 self-center text-xs font-medium text-muted-foreground"
                      >
                        {dayLabel}
                      </time>
                    )}
                    <ItemActions subtle swipeAction={{ label: 'Quote message', previewLabel: 'Reply', run: quoteMessage }} label={t('Message') + ' ' + String(msg.id || index + 1)} actions={[
                      { label: 'Copy message', run: () => navigator.clipboard.writeText(msg.content || msg.message) },
                      { label: 'Quote message', run: quoteMessage },
                      { label: 'Copy link', run: () => copyItemLink('/inbox/' + encodeURIComponent(activeContact.conversation_id || activeContact.id) + (msg.uuid ? '?message=' + encodeURIComponent(msg.uuid) : '')) },
                      ...(activeContact.type === 'customer' ? [{
                        label: t('Whisper to @team'),
                        run: () => {
                          const body = msg.content || msg.message || ''
                          const snippet = body.length > 80 ? body.slice(0, 77) + '…' : body
                          setWhisperMode(true)
                          setWhisperHandle('@team')
                          setInputMessage(`@team regarding "${snippet}": `)
                          textareaRef.current?.focus()
                          if (textareaRef.current) autoResizeTextarea(textareaRef.current)
                        },
                      }] : []),
                      ...(msg.uuid ? [{
                        label: t('Escalate to task'),
                        run: () => setEscalatingMessage(msg),
                      }] : []),
                    ]}>
                    <div
                      className={cn(
                        'wa-message-bubble relative min-w-0',
                        msg.uuid === requestedMessageId && 'ring-2 ring-ring',
                        msg.is_internal && 'ring-1 ring-amber-500/40 border border-amber-500/30',
                        isOutgoing ? 'wa-outgoing' : 'wa-incoming',
                      )}
                    >
                      {msg.is_internal && (
                        <div className="mb-2 flex items-center gap-1.5 text-[10px] font-medium text-amber-600 dark:text-amber-400 bg-amber-500/10 dark:bg-amber-500/15 border border-amber-500/25 px-2 py-0.5 rounded-full w-fit">
                          <EyeOff className="h-3 w-3" />
                          <span>{t('Internal · Invisible to visitor')}</span>
                          {(msg.mention_handle || (msg.author_name && msg.author_name !== 'Owner' && msg.author_name !== 'You')) && (
                            <span className="font-semibold text-foreground bg-background/60 px-1 rounded ml-0.5">
                              {msg.mention_handle || msg.author_name}
                            </span>
                          )}
                        </div>
                      )}
                      {msg.approval_title && !msg.approval_plan && (
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
                          receiptExtraction={msg.receipt_extraction}
                          onTaskClick={(task) => handleOpenTaskDrawer(task, 'details')}
                          onOpenTerminal={(task) => handleOpenTaskDrawer(task, 'terminal')}
                          onUseDraft={handleUseDraft}
                          onRetry={handleTaskRetry}
                          onContinueChat={handleUseDraft}
                        />
                      ) : (
                        <div className="wa-plain-text whitespace-pre-wrap break-words font-sans">
                          <span>{msg.content || msg.message}</span>
                          {metadata}
                        </div>
                      )}

                      {/* Ready-to-Use Customer Draft Reply Card */}
                      {msg.reply_draft && (
                        <div className="mt-3 p-3 rounded-xl bg-card border border-primary/30 shadow-xs text-foreground space-y-2">
                          <div className="flex items-center justify-between gap-2 border-b border-border/50 pb-2">
                            <div className="flex items-center gap-1.5 font-semibold text-xs text-primary">
                              <Sparkles className="h-3.5 w-3.5" />
                              <span>{t('Customer reply draft · Not sent yet')}</span>
                            </div>
                            <Button
                              size="sm"
                              onClick={() => {
                                setInputMessage(msg.reply_draft || '')
                                textareaRef.current?.focus()
                                if (textareaRef.current) autoResizeTextarea(textareaRef.current)
                              }}
                              className="h-7 text-xs font-medium px-2.5 py-1 gap-1 cursor-pointer"
                            >
                              <Send className="h-3 w-3" />
                              <span>{t('Review & Use')}</span>
                            </Button>
                          </div>
                          <p className="text-xs text-muted-foreground italic bg-muted/40 p-2.5 rounded-lg border border-border/40 whitespace-pre-wrap leading-relaxed">
                            "{msg.reply_draft}"
                          </p>
                          <p className="text-[10px] text-muted-foreground">
                            {t('Click "Review & Use" to load into the message composer for the visitor.')}
                          </p>
                        </div>
                      )}

                      {isAgentMessage && msg.approval_plan && (
                        <ApprovalCard
                          plan={msg.approval_plan}
                          accepted={msg.approval_accepted}
                          deferred={msg.plan_deferred}
                          disabled={sending || uploads.busy || retryCount >= 3 || !!pendingRef.current}
                          onApprove={() => handleSendMessage(msg.uuid, msg.approval_request_id)}
                          onChangePlan={() => {
                            setInputMessage(current => (current ? current + '\n\n' : '') + t('Changes to this plan') + ' — ' + msg.approval_plan!.title + ': ')
                            textareaRef.current?.focus()
                          }}
                          onDefer={() => handleSendMessage(msg.uuid, msg.plan_defer_request_id, 'defer')}
                        />
                      )}

                      {/* Receipt Extraction Preview Card */}
                      {(msg.receipt_extraction || parseReceiptExtraction(msg.content || msg.message || '')) && (
                        <ReceiptPreviewCard
                          extraction={msg.receipt_extraction || parseReceiptExtraction(msg.content || msg.message || '')!}
                          onConfirm={(ext) => {
                            msg.receipt_extraction = { ...ext, confirmed: true }
                          }}
                        />
                      )}

                      {/* 1-Click Execution Action Card for Agent Messages */}
                      {isAgentMessage && msg.action_card && (
                        <div className="mt-3 p-3 rounded-xl bg-card/40 border border-border/40 text-foreground space-y-2">
                          <div className="flex items-center justify-between gap-2 border-b border-border/40 pb-2">
                            <div className="flex items-center gap-1.5 font-semibold text-xs">
                              <Sparkles className="h-3.5 w-3.5 text-primary" />
                              <span>Sprint execution plan</span>
                            </div>
                            <span className="text-[10px] font-mono px-2 py-0.5 rounded bg-secondary text-secondary-foreground">
                              {msg.action_card.worker_agent}
                            </span>
                          </div>

                          <div className="pt-0.5 text-xs text-muted-foreground flex items-center justify-between">
                            <span className="font-medium text-foreground">
                              {msg.action_card.task_title}
                            </span>
                            <span className="text-[11px] font-mono text-muted-foreground">
                              {msg.action_card.dod_checklist?.length || 0} kriteria rencana
                            </span>
                          </div>

                          {/* 1-Click Button */}
                          <div className="pt-1 border-t border-border/30">
                            {approvedTasks[msg.action_card.task_title] ? (
                              <div className="w-full py-1.5 px-2 text-emerald-600 dark:text-emerald-400 text-xs font-medium flex items-center justify-center gap-1.5">
                                <CheckCheck className="h-3.5 w-3.5" />
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
                                className="w-full min-h-8 py-1.5 px-3 rounded-md bg-primary text-primary-foreground font-medium text-xs hover:bg-primary/90 active:scale-98 transition-all flex items-center justify-center gap-2 disabled:opacity-50 cursor-pointer shadow-xs"
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
                          onViewTask={(task) => handleOpenTaskDrawer(task, 'details')}
                          onOpenTerminal={(task) => handleOpenTaskDrawer(task, 'terminal')}
                          onUseDraft={handleUseDraft}
                          onRetry={handleTaskRetry}
                          onContinueChat={handleUseDraft}
                        />
                      )}
                      {msg.uuid && linkedTasks.some((t) => t.source_message_id === msg.uuid) && (
                        <button
                          type="button"
                          data-testid="assigned-task-btn"
                          onClick={() => {
                            const task = linkedTasks.find((t) => t.source_message_id === msg.uuid)
                            if (task) handleOpenTaskDrawer(task)
                          }}
                          className="mt-1.5 inline-flex items-center gap-1.5 text-xs font-medium text-muted-foreground hover:text-foreground transition-colors cursor-pointer bg-transparent border-0 p-0"
                          title={t('View in Tasks')}
                        >
                          <CheckSquare className="h-3.5 w-3.5 text-muted-foreground" />
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
                          className="mt-2.5 flex w-full self-stretch flex-col items-start"
                        >
                          <div className="wa-message-bubble wa-incoming !bg-transparent !border-0 !shadow-none !p-0 relative min-w-0 max-w-xl w-full">
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

              {/* Unattached or latest completed conversation tasks */}
              {unattachedCompletedTasks.map((completedTask) => (
                <div
                  key={`standalone-completed-${completedTask.id}`}
                  className="mt-3 flex w-full self-stretch flex-col items-start animate-fade-in"
                >
                  <div className="wa-message-bubble wa-incoming !bg-transparent !border-0 !shadow-none !p-0 relative min-w-0 max-w-xl w-full">
                    <TaskCompletionCard
                      task={completedTask}
                      conversationId={
                        activeContact.conversation_id || activeContact.id
                      }
                      onContinueChat={handleUseDraft}
                      onUseDraft={handleUseDraft}
                      onOpenTerminal={(task) => handleOpenTaskDrawer(task, 'terminal')}
                      onRetry={handleTaskRetry}
                      onViewDetails={() => handleOpenTaskDrawer(completedTask)}
                    />
                  </div>
                </div>
              ))}

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

              {/* Typing / In-flight Working Indicator */}
              {((sending && activeContact.type === 'agent') || activeConversationTasks.length > 0) && (
                <div className="flex flex-col items-start animate-fade-in my-2" data-testid="inbox-working-indicator">
                  <div className="bg-card text-card-foreground border border-border/80 rounded-2xl rounded-tl-xs p-3 shadow-xs flex items-center gap-2.5 max-w-md">
                    <div className="h-6 w-6 rounded-full bg-secondary flex items-center justify-center text-xs shrink-0">
                      {activeContact.avatar ? (
                        <span>{activeContact.avatar}</span>
                      ) : (
                        <Bot className="h-3.5 w-3.5 text-primary" />
                      )}
                    </div>
                    <div className="flex flex-col min-w-0 pr-1">
                      {activeConversationTasks.length > 0 ? (
                        <>
                          <span className="text-xs text-foreground font-medium truncate">
                            {activeConversationTasks[0].title || t('Task is running')}
                          </span>
                          <span className="text-[11px] text-muted-foreground flex items-center gap-1">
                            <span>{activeConversationTasks[0].assignee ? `@${activeConversationTasks[0].assignee}` : (activeConversationTasks[0] as any).assigned_agent ? `@${(activeConversationTasks[0] as any).assigned_agent}` : t('Agent')}</span>
                            <span>·</span>
                            <span>{t('sedang mengeksekusi di background')}</span>
                          </span>
                        </>
                      ) : (
                        <span className="text-xs text-muted-foreground font-medium">
                          {t('Sending message to')} {activeContact.name}...
                        </span>
                      )}
                    </div>
                    <div className="flex items-center gap-1 ml-auto shrink-0 pl-1">
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
                    {activeConversationTasks.length > 0 && (
                      <button
                        type="button"
                        onClick={() => handleOpenTaskDrawer(activeConversationTasks[0], 'terminal')}
                        className="ml-2 text-xs font-medium text-primary hover:underline shrink-0"
                      >
                        {t('Live Terminal')}
                      </button>
                    )}
                  </div>
                </div>
              )}
                </div>
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
              className="wa-composer shrink-0 z-20 border-t-0"
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
              <div className="mx-auto max-w-[75ch] space-y-2">
                {activeContact.type === 'customer' && (
                  <div className="flex flex-wrap items-center justify-between gap-1.5 px-1.5 text-xs">
                    <div className="flex items-center gap-1.5 flex-wrap">
                      <div className="inline-flex rounded-lg border border-border/60 bg-muted/40 p-0.5">
                        <button
                          type="button"
                          onClick={() => {
                            setWhisperMode(false)
                            if (inputMessage.trim().startsWith('@')) {
                              setInputMessage(inputMessage.replace(/^@(team|sm|dev)\s*/i, ''))
                            }
                          }}
                          className={cn(
                            'rounded-md px-2 py-0.5 text-[11px] font-medium transition-colors cursor-pointer',
                            !whisperMode
                              ? 'bg-background text-foreground font-semibold shadow-xs'
                              : 'text-muted-foreground hover:text-foreground'
                          )}
                        >
                          {t('Public reply')}
                        </button>
                        <button
                          type="button"
                          onClick={() => setWhisperMode(true)}
                          className={cn(
                            'flex items-center gap-1 rounded-md px-2 py-0.5 text-[11px] font-medium transition-colors cursor-pointer',
                            whisperMode
                              ? 'bg-amber-500/20 text-amber-700 dark:text-amber-300 font-semibold shadow-xs'
                              : 'text-muted-foreground hover:text-foreground'
                          )}
                        >
                          <Lock className="h-3 w-3 text-amber-500" />
                          <span>{t('Whisper AI:')}</span>
                        </button>
                      </div>

                      {[
                        { handle: '@team' as const, label: '@team (Auto)' },
                        { handle: '@sm' as const, label: '@sm (Scrum Master)' },
                        { handle: '@dev' as const, label: '@dev (Developer)' },
                      ].map((item) => {
                        const isSelected = (whisperMode && whisperHandle === item.handle) || inputMessage.toLowerCase().startsWith(item.handle)
                        return (
                          <button
                            key={item.handle}
                            type="button"
                            onClick={() => {
                              setWhisperMode(true)
                              setWhisperHandle(item.handle)
                              const trimmed = inputMessage.trim()
                              const next = trimmed
                                ? `${item.handle} ${trimmed.replace(/^@(team|sm|dev)\s*/i, '')}`
                                : `${item.handle} `
                              setInputMessage(next)
                              textareaRef.current?.focus()
                              if (textareaRef.current) autoResizeTextarea(textareaRef.current)
                            }}
                            className={cn(
                              'rounded-full px-2 py-0.5 text-[11px] font-medium border transition-colors cursor-pointer',
                              isSelected
                                ? 'bg-amber-500/20 text-amber-700 dark:text-amber-300 border-amber-500/40 font-semibold shadow-xs'
                                : 'bg-muted/60 text-muted-foreground hover:text-foreground border-border/60 hover:bg-muted'
                            )}
                          >
                            {item.label}
                          </button>
                        )
                      })}
                    </div>
                    {(whisperMode || inputMessage.trim().startsWith('@')) && (
                      <span className="text-[10px] text-amber-600 dark:text-amber-400 font-semibold inline-flex items-center gap-1 bg-amber-500/10 px-2 py-0.5 rounded-full border border-amber-500/20">
                        <EyeOff className="h-3 w-3" />
                        {t('Internal · Invisible to visitor')}
                      </span>
                    )}
                  </div>
                )}
                <AttachmentPreviews uploads={uploads} disabled={sending} />
                <div
                  className={`wa-input-capsule flex min-w-0 items-end gap-1 sm:gap-1.5 p-1 sm:p-1.5 ${
                    isMultiline ? 'is-multiline' : ''
                  } ${
                    activeContact.type === 'customer' && (whisperMode || inputMessage.trim().startsWith('@')) ? 'ring-1 ring-amber-500/50 border-amber-500/40' : ''
                  }`}
                >
                  <AttachmentPicker
                    key={activeContact.conversation_id || activeContact.id}
                    uploads={uploads}
                    disabled={sending}
                    compact
                    disableImages={activeContact.type === 'agent' && !contactSupportsVision}
                  />
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
                        : whisperMode || inputMessage.trim().startsWith('@')
                        ? t('Whisper to AI team (invisible to visitor)…')
                        : `Reply to ${activeContact.name} or type @team to whisper AI…`
                    }
                    className="wa-input min-w-0 flex-1 min-h-[36px] sm:min-h-[38px] max-h-[140px] resize-none overflow-hidden py-2 sm:py-2 text-[15px] sm:text-sm leading-5 sm:leading-snug pl-1.5 pr-2 sm:pl-2 sm:pr-2.5"
                  />

                  {activeContact.type === 'customer' && (
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
                  )}

                  <button
                    type="button"
                    onClick={() => handleSendMessage()}
                    disabled={
                      (!inputMessage.trim() && !uploads.items.length) ||
                      sending ||
                      uploads.busy
                    }
                    className="wa-send h-9 w-9 sm:h-9.5 sm:w-9.5 rounded-full flex items-center justify-center active:scale-95 disabled:opacity-40 disabled:pointer-events-none transition-all cursor-pointer shrink-0 mr-0.5"
                    title={inputMessage.trim().startsWith('@') ? t('Send internal whisper to AI') : t('Send message (Enter)')}
                    aria-label={inputMessage.trim().startsWith('@') ? t('Send internal whisper to AI') : t('Send message')}
                  >
                    {inputMessage.trim().startsWith('@') ? (
                      <Lock className="h-4 w-4" />
                    ) : (
                      <Send className="h-[18px] w-[18px]" />
                    )}
                  </button>
                </div>
              </div>
            </div>
          </>
        )}
      </div>

      {/* Progressive Disclosure SlideDrawer for Linked Task Peek */}
      <SlideDrawer
        isOpen={Boolean(peekTask)}
        onClose={() => setPeekTask(null)}
        title={peekTask?.title || t('Detail Tugas')}
        description={
          peekTask
            ? `${taskRef(peekTask.id)} · ${statusLabel(peekTask.status || 'not_started')}`
            : undefined
        }
        size="lg"
        data-testid="inbox-task-drawer"
        badge={
          peekTask && (
            <span
              className={cn(
                'text-[11px] font-mono font-medium px-2 py-0.5 rounded border border-border/40',
                peekTask.status === 'done'
                  ? 'text-emerald-600 dark:text-emerald-400 bg-emerald-500/10'
                  : peekTask.status === 'review'
                    ? 'text-purple-600 dark:text-purple-400 bg-purple-500/10'
                    : peekTask.status === 'doing'
                      ? 'text-blue-600 dark:text-blue-400 bg-blue-500/10'
                      : 'text-muted-foreground bg-muted/40'
              )}
            >
              {taskRef(peekTask.id)}
            </span>
          )
        }
        headerActions={
          peekTask && (
            <Link
              to={taskTargetUrl(peekTask)}
              className="rounded-md p-2 text-muted-foreground hover:bg-muted/60 hover:text-foreground transition-colors min-h-[36px] min-w-[36px] flex items-center justify-center"
              title={t('Buka Halaman Tugas Penuh')}
              aria-label={t('Buka Halaman Tugas Penuh')}
            >
              <ExternalLink className="h-4 w-4" />
            </Link>
          )
        }
        footer={
          peekTask && (
            <div className="flex items-center justify-between w-full gap-2">
              <button
                type="button"
                onClick={() => setPeekTask(null)}
                className="action-secondary min-h-10 px-3.5 text-xs"
              >
                <span>{t('Tutup')}</span>
              </button>
              <Link
                to={taskTargetUrl(peekTask)}
                className="action-primary min-h-10 px-3.5 text-xs inline-flex items-center gap-1.5"
              >
                <span>{t('Buka Halaman Tugas')}</span>
                <ArrowRight className="h-3.5 w-3.5" />
              </Link>
            </div>
          )
        }
      >
        {peekTask && (() => {
          const envelopeResult = parseTaskReviewEnvelope(peekTask)
          return (
            <div className="space-y-4 text-sm" data-testid="inbox-task-drawer-content">
              {/* Tab Toggle: Detail Diagnostik & Status Deploy vs Live Terminal */}
              <div className="flex items-center gap-1.5 p-1 rounded-lg bg-muted/40 border border-border/40">
                <button
                  type="button"
                  onClick={() => setPeekTab('details')}
                  className={cn(
                    'flex-1 py-1.5 px-3 rounded-md text-xs font-medium transition-colors text-center cursor-pointer',
                    peekTab === 'details'
                      ? 'bg-background text-foreground shadow-xs font-semibold'
                      : 'text-muted-foreground hover:text-foreground'
                  )}
                >
                  <span>{t('Detail Diagnostik & Status Deploy')}</span>
                </button>
                <button
                  type="button"
                  onClick={() => setPeekTab('terminal')}
                  data-testid="drawer-live-terminal-tab"
                  className={cn(
                    'flex-1 py-1.5 px-3 rounded-md text-xs font-medium transition-colors text-center inline-flex items-center justify-center gap-1.5 cursor-pointer',
                    peekTab === 'terminal'
                      ? 'bg-background text-foreground shadow-xs font-semibold'
                      : 'text-muted-foreground hover:text-foreground'
                  )}
                >
                  <Terminal className="h-3.5 w-3.5 text-emerald-500" />
                  <span>{t('Live Terminal')}</span>
                </button>
              </div>

              {peekTab === 'terminal' ? (
                /* Embedded Live Terminal View */
                <div className="space-y-3">
                  <div className="flex items-center justify-between text-xs text-muted-foreground">
                    <span className="inline-flex items-center gap-1.5">
                      <span className="relative flex h-2 w-2">
                        <span className="animate-ping absolute inline-flex h-full w-full rounded-full bg-emerald-400 opacity-75" />
                        <span className="relative inline-flex rounded-full h-2 w-2 bg-emerald-500" />
                      </span>
                      <span className="font-medium text-foreground">Streaming live logs… (xterm.js ANSI stream)</span>
                    </span>
                    <Link
                      to={taskTerminalUrl(peekTask)}
                      className="inline-flex items-center gap-1 text-primary hover:underline font-medium text-[11px]"
                    >
                      <span>Buka Halaman Penuh</span>
                      <ExternalLink className="h-3 w-3" />
                    </Link>
                  </div>
                  <div className="h-[460px] w-full rounded-xl overflow-hidden border border-border/40 bg-black/95 relative shadow-inner">
                    <Suspense fallback={<div className="h-full flex items-center justify-center text-xs font-mono text-muted-foreground">Menghubungkan ke live terminal stream...</div>}>
                      <TaskTerminal
                        taskId={peekTask.id}
                        attempt={peekTask.latest_attempt || ({
                          id: peekTask.id,
                          task_id: peekTask.id,
                          status: peekTask.status === 'doing' ? 'running' : 'succeeded',
                          agent_name: peekTask.assignee_id || 'cuan-runner',
                          created_at: peekTask.created_at,
                        } as any)}
                        live={null}
                        onRefresh={() => { handleOpenTaskDrawer(peekTask.id, 'terminal') }}
                      />
                    </Suspense>
                  </div>
                </div>
              ) : (
                /* Diagnostic Details & Status Deploy View */
                <div className="space-y-4">
                  {/* Meta Strip: Status, Assignee, Time */}
                  <div className="rounded-xl border border-border/40 bg-card/60 p-3.5 space-y-2.5">
                    <div className="flex items-center justify-between gap-2 flex-wrap">
                      <div className="flex items-center gap-2">
                        <span
                          className={cn(
                            'inline-flex items-center gap-1.5 text-xs font-medium',
                            peekTask.status === 'done'
                              ? 'text-emerald-600 dark:text-emerald-400'
                              : peekTask.status === 'review'
                                ? 'text-purple-600 dark:text-purple-400'
                                : peekTask.status === 'doing'
                                  ? 'text-blue-600 dark:text-blue-400'
                                  : 'text-muted-foreground'
                          )}
                        >
                          {peekTask.status === 'done' ? (
                            <CheckCircle2 className="h-4 w-4" />
                          ) : (
                            <Clock className="h-4 w-4" />
                          )}
                          <span>{statusLabel(peekTask.status || 'not_started')}</span>
                        </span>
                      </div>

                      <div className="flex items-center gap-1 text-[11px] text-muted-foreground font-mono">
                        <span>{relativeTime(peekTask.updated_at || peekTask.created_at)}</span>
                      </div>
                    </div>

                    {/* Assignee, Branch & Commit */}
                    <div className="flex flex-wrap items-center gap-2 pt-1 border-t border-border/30 text-xs">
                      <span className="inline-flex items-center gap-1.5 text-muted-foreground">
                        <Bot className="h-3.5 w-3.5 text-primary" />
                        <span>
                          {agentDisplayName(
                            peekTask.latest_attempt?.agent_name ||
                              peekTask.assignee_id ||
                              peekTask.assignee ||
                              'cuan-runner'
                          )}
                        </span>
                      </span>

                      {peekTask.branch_name && (
                        <span className="inline-flex items-center gap-1 font-mono text-[11px] text-muted-foreground bg-muted/40 px-2 py-0.5 rounded">
                          <GitBranch className="h-3 w-3" />
                          <span>{peekTask.branch_name}</span>
                        </span>
                      )}

                      {peekTask.pr_url && (
                        <a
                          href={peekTask.pr_url}
                          target="_blank"
                          rel="noopener noreferrer"
                          className="inline-flex items-center gap-1 text-primary hover:underline font-medium text-xs ml-auto"
                        >
                          <span>Pull Request</span>
                          <ExternalLink className="h-3 w-3" />
                        </a>
                      )}
                    </div>
                  </div>

                  {/* Status Deploy Section */}
                  <div className="rounded-xl border border-border/40 bg-card/60 p-3 space-y-1.5 text-xs">
                    <div className="flex items-center gap-1.5 font-semibold text-foreground">
                      <Rocket className="h-3.5 w-3.5 text-sky-500" />
                      <span>{t('Status Deploy')}</span>
                    </div>
                    <div className="text-muted-foreground leading-relaxed">
                      {envelopeResult.requiresDeploy ? (
                        <span className="inline-flex items-center gap-1.5 text-blue-600 dark:text-blue-400 font-medium">
                          <CheckCircle2 className="h-3.5 w-3.5 shrink-0" />
                          <span>Tugas Perubahan Kode: Wajib build, test & live auto-deploy VPS.</span>
                        </span>
                      ) : (
                        <span className="inline-flex items-center gap-1.5 text-emerald-600 dark:text-emerald-400 font-medium">
                          <ShieldCheck className="h-3.5 w-3.5 shrink-0" />
                          <span>Tugas Investigasi / Non-Kode: Lulus tanpa modifikasi berkas & tanpa pemaksaan deploy VPS.</span>
                        </span>
                      )}
                    </div>
                  </div>

                  {/* Summary & Technical Evidence */}
                  {(envelopeResult.summary || envelopeResult.evidence) && (
                    <div className="space-y-1.5">
                      <div className="text-[11px] font-semibold uppercase tracking-wider text-muted-foreground flex items-center gap-1.5">
                        <Sparkles className="h-3.5 w-3.5 text-primary" />
                        <span>{t('Ringkasan Hasil')}</span>
                      </div>
                      <div className="rounded-xl border border-border/40 bg-card/40 p-3 text-xs leading-relaxed text-foreground space-y-2">
                        <ChatMarkdown content={envelopeResult.summary} />
                        {envelopeResult.evidence && envelopeResult.evidence !== envelopeResult.summary && (
                          <div className="pt-2 border-t border-border/30">
                            <span className="font-semibold text-muted-foreground uppercase text-[10px] block mb-1">
                              Bukti Teknis & Analisis:
                            </span>
                            <p className="text-muted-foreground break-words">{envelopeResult.evidence}</p>
                          </div>
                        )}
                      </div>
                    </div>
                  )}

                  {/* Definition of Done Checklist */}
                  {peekTask.dod_checklist && Array.isArray(peekTask.dod_checklist) && peekTask.dod_checklist.length > 0 && (
                    <div className="space-y-2">
                      <div className="flex items-center justify-between text-[11px] font-semibold uppercase tracking-wider text-muted-foreground">
                        <span className="flex items-center gap-1.5">
                          <ListChecks className="h-3.5 w-3.5 text-primary" />
                          <span>{t('Kriteria Selesai (DoD)')}</span>
                        </span>
                        <span className="font-mono text-[11px]">
                          {
                            peekTask.dod_checklist.filter((item) =>
                              typeof item === 'string'
                                ? item.startsWith('[x]') || item.startsWith('[X]') || item.startsWith('✓')
                                : !!item.completed
                            ).length
                          }
                          /{peekTask.dod_checklist.length}
                        </span>
                      </div>
                      <ul className="rounded-xl border border-border/40 bg-card/40 p-3 space-y-2 text-xs">
                        {peekTask.dod_checklist.map((item, idx) => {
                          const isChecked =
                            typeof item === 'string'
                              ? item.startsWith('[x]') || item.startsWith('[X]') || item.startsWith('✓')
                              : !!item.completed
                          const text =
                            typeof item === 'string'
                              ? item.replace(/^(\[[ xX]\]|✓)\s*/, '').trim()
                              : item.text
                          return (
                            <li key={idx} className="flex items-start gap-2 leading-relaxed">
                              <span
                                className={cn(
                                  'mt-0.5 flex h-3.5 w-3.5 shrink-0 items-center justify-center rounded text-[10px]',
                                  isChecked
                                    ? 'text-emerald-600 dark:text-emerald-400 font-bold'
                                    : 'text-muted-foreground/50'
                                )}
                              >
                                {isChecked ? '✓' : '·'}
                              </span>
                              <span className={cn('break-words text-foreground/90', isChecked && 'font-medium text-foreground')}>
                                {text}
                              </span>
                            </li>
                          )
                        })}
                      </ul>
                    </div>
                  )}

                  {/* Task Instructions / Description */}
                  {peekTask.description && (
                    <div className="space-y-1.5">
                      <div className="text-[11px] font-semibold uppercase tracking-wider text-muted-foreground">
                        {t('Instruksi / Deskripsi')}
                      </div>
                      <div className="rounded-xl border border-border/40 bg-card/40 p-3 text-xs leading-relaxed text-muted-foreground whitespace-pre-wrap">
                        {peekTask.description}
                      </div>
                    </div>
                  )}

                  {/* Active Execution / Terminal link */}
                  <div className="pt-2">
                    <button
                      type="button"
                      onClick={() => setPeekTab('terminal')}
                      className="action-secondary w-full text-xs py-2.5 inline-flex items-center justify-center gap-2 rounded-lg cursor-pointer"
                    >
                      <Terminal className="h-3.5 w-3.5 text-emerald-500" />
                      <span>{t('Buka Terminal Langsung')}</span>
                    </button>
                  </div>
                </div>
              )}
            </div>
          )
        })()}
      </SlideDrawer>

      {escalatingMessage && (
        <Suspense fallback={null}>
          <EscalateTaskDialog
            open={!!escalatingMessage}
            message={escalatingMessage}
            conversationId={activeContact?.conversation_id || activeContact?.id || ''}
            onClose={() => setEscalatingMessage(null)}
            onTaskCreated={(newTask) => {
              setLinkedTasks((prev) => [newTask, ...prev.filter((t) => t.id !== newTask.id)])
              handleOpenTaskDrawer(newTask)
              if (activeContact) {
                activeContact.task_count = (activeContact.task_count || 0) + 1
                activeContact.active_tasks = (activeContact.active_tasks || 0) + 1
              }
              onContactUpdated?.()
            }}
          />
        </Suspense>
      )}

      {showCannedManageModal && (
        <Suspense fallback={null}>
          <CannedResponsesDialog
            open={showCannedManageModal}
            onClose={() => setShowCannedManageModal(false)}
            onSelect={(content) => {
              setInputMessage(content)
              textareaRef.current?.focus()
              if (textareaRef.current) autoResizeTextarea(textareaRef.current)
            }}
          />
        </Suspense>
      )}
    </div>
  )
}
