import { ConversationResolutionAction } from './conversation-resolution'
import { mergeConversationMessages } from './message-reconciliation'
import { Composer, ComposerAccessory, ComposerSendButton } from '../../components/ui/Composer'
import { ComposerInput, resizeComposerInput } from '../../components/ui/ComposerInput'
import { ReplyDraftCard } from './ReplyDraftCard'
import { useMessageReceipts } from './useMessageReceipts'
import { InternalVisibility } from '../../components/ui/InternalVisibility'
import { workStatus } from '../tasks/task-ui'
import { startsMessageGroup } from './message-group'
import { PageHeader, PageTitle } from '../../components/layout/PageHeader'
import { IconButton } from '../../components/ui/IconButton'
import { updateItemPreference } from '../../hooks/useArchivedItems'
import { ItemActions, copyItemLink } from '../../components/ui/ItemActions'
import { useConnectionClock } from '../agents/useConnectionClock'
import { useLanguage } from '../../context/LanguageContext'
import {
  AttachmentPicker,
  AttachmentPreviews,
  AttachmentFile,
  isImageAttachment,
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
  sendCustomerReplyEmail,
} from '../../api/chat'
import {
  Send,
  Bot,
  User,
  Check,
  CheckCheck,
  MailCheck,
  Play,
  CheckSquare,
  Sparkles,
  RefreshCw,
  ChevronLeft,
  ChevronDown,
  Lock,
  Copy,
  Inbox as InboxIcon,
} from 'lucide-react'
import { Task } from '../../api/types'
import { tasksApi } from '../../api/tasks'
import './chat-calm.css'
import { StudioView } from './StudioView'
import { MessageContent } from './MessageContent'
import { withoutTaskLaunchAcknowledgement } from './task-launch-text'
import { taskMessageBindings, isBackgroundWorkUpdate } from './task-message-bindings'
import { InboxActivity } from './InboxActivity'
import { InboxTaskAction } from './InboxTaskAction'
import { ApprovalCard } from './ApprovalCard'
import { PromptChoiceChips } from './PromptChoiceChips'
import { ReceiptPreviewCard, parseReceiptExtraction } from './ReceiptPreviewCard'
import { InboxWorkPanel } from './InboxWorkPanel'
import { ComposerTasks } from './ComposerTasks'

import { isTaskActive } from './task-progress'
import { cacheTasks } from './TaskCard'
import { notifyTaskCompleted } from './task-notifications'
import { isComposerWhisper, extractMentionHandle } from './mention-whisper'
import { cn } from '../../lib/utils'
import { InboxSubPanel, ContactFilterTab } from './InboxSubPanel'


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
    if (initialMode === 'studio' || isChatRoute) return 'studio'
    return 'chat'
  })

  useEffect(() => {
    if (location.pathname.startsWith('/chat') && !location.pathname.startsWith('/chat/')) {
      setCanvasMode('studio')
    } else if (activeContact) {
      setCanvasMode('chat')
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
  const httpFetchVersion = useRef(0)
  const prevLoadedConvId = useRef<string | null>(null)
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
    window.addEventListener('focus', visible)
    document.addEventListener('visibilitychange', visible)
    return () => {
      window.removeEventListener('focus', visible)
      document.removeEventListener('visibilitychange', visible)
    }
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
  const workChatScroll = useRef<number | null>(null)
  const [escalatingMessage, setEscalatingMessage] = useState<ChatMessage | null>(null)

  const handleOpenTaskDrawer = useCallback(
    (taskOrId: Task | string, tab: 'details' | 'terminal' = 'details') => {
      const query = new URLSearchParams(location.search)
      const alreadyOpen = query.has('work')
      if (!alreadyOpen) workChatScroll.current = messageListRef.current?.scrollTop ?? null
      query.set('work', typeof taskOrId === 'string' ? taskOrId : taskOrId.id)
      query.set('work_view', tab)
      navigate({ pathname: location.pathname, search: query.toString() }, { replace: alreadyOpen, state: { ...location.state, inboxWorkEntry: alreadyOpen ? location.state?.inboxWorkEntry : true } })
    }, [navigate, location.pathname, location.search, location.state],
  )
  const closeWorkPanel = useCallback(() => {
    if (location.state?.inboxWorkEntry) navigate(-1)
    else {
      const query = new URLSearchParams(location.search)
      query.delete('work'); query.delete('work_view')
      navigate({ pathname: location.pathname, search: query.toString() }, {replace:true, state:location.state})
    }
  }, [navigate, location.pathname, location.search, location.state])
  useEffect(() => {
    const query = new URLSearchParams(location.search), workID = query.get('work')
    if (!workID) {
      setPeekTask(null)
      const frame = requestAnimationFrame(() => { if (workChatScroll.current !== null && messageListRef.current) { messageListRef.current.scrollTop = workChatScroll.current; workChatScroll.current = null } })
      return () => cancelAnimationFrame(frame)
    }
    setPeekTab(query.get('work_view') === 'terminal' ? 'terminal' : 'details')
    setPeekTask(current => linkedTasks.find(task => task.id === workID) || (current?.id === workID ? current : null) || {id:workID,title:'',status:'todo',priority:'normal',created_at:'',updated_at:''})
  }, [location.search, linkedTasks])

  const handleTaskDetailsClick = useCallback(
    (task: Task) => handleOpenTaskDrawer(task, 'details'),
    [handleOpenTaskDrawer]
  )

  const handleTaskTerminalClick = useCallback(
    (task: Task) => handleOpenTaskDrawer(task, 'terminal'),
    [handleOpenTaskDrawer]
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
  const [regenerationStart, setRegenerationStart] = useState<{ convId: string; sourceId: string; requestId: string } | null>(null)
  const [requests, setRequests] = useState<InboxRequest[]>([])
  const [sendError, setSendError] = useState('')
  const [retryCount, setRetryCount] = useState(0)
  const pendingRef = useRef<{
    convId: string
    text: string
    draftText?: string
    id: string
    attachmentIds?: string[]
    approveMessageId?: string
    planAction?: 'approve' | 'defer'
    regenerateReplyId?: string
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
  useMessageReceipts(activeContact?.conversation_id || activeContact?.id, activeContact?.type === 'customer', messages)

  const latestMessageIdPerTask = useMemo(
    () => taskMessageBindings(linkedTasks, messages, requests, activeContact?.type === 'customer'),
    [linkedTasks, messages, requests, activeContact?.type],
  )
  const [inputMessage, setInputMessage] = useState('')
  const uploads = useAttachments(
    'cuan-inbox-attachments:' +
      (activeContact?.conversation_id || activeContact?.id || ''),
  )

  const activeAgent = agents?.find((a) => a.id === activeContact?.id || a.id === activeContact?.agent_id || a.name === activeContact?.name)
  const contactDisplayName = (() => {
    if (!activeContact) return ''
    if (activeContact.type === 'agent') {
      const matched = agents.find(a => a.id === 'agent_cuan') || agentForContact(activeContact, agents) || activeAgent
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

  // Real-time auto-whisper when mentioning an agent in customer conversations
  const isWhisper = isComposerWhisper(activeContact?.type, inputMessage)
  const detectedMention = activeContact?.type === 'customer' ? extractMentionHandle(inputMessage) : ''
  const whisperHandle = detectedMention || '@cuan'

  const setWhisperMode = useCallback((val: boolean) => {
    if (val && activeContact?.type === 'customer') {
      setInputMessage((current) => {
        const trimmed = current.trim()
        if (!trimmed.startsWith('@')) {
          return trimmed ? `@cuan ${trimmed}` : '@cuan '
        }
        return current
      })
    } else if (!val && activeContact?.type === 'customer') {
      setInputMessage((current) => {
        if (current.trim().startsWith('@')) {
          return current.replace(/^@(cuan|team|sm|dev|[a-zA-Z0-9_-]+)\s*/i, '')
        }
        return current
      })
    }
  }, [activeContact?.type])

  const handleTaskDiscussion = useCallback((draftText: string) => {
    if (!draftText) return
    const textToInsert = activeContact?.type === 'customer' && !draftText.trim().startsWith('@')
      ? `@cuan ${draftText}`
      : draftText
    setInputMessage(current => current.trim() ? current + '\n\n' + textToInsert : textToInsert)
    requestAnimationFrame(() => {
      textareaRef.current?.focus()
      if (textareaRef.current) autoResizeTextarea(textareaRef.current)
    })
  }, [activeContact?.type])

  // Mobile Single-View WhatsApp/Telegram state: 'contacts' | 'chat'
  const [mobileView, setMobileView] = useState<'contacts' | 'chat'>(() => activeContact ? 'chat' : 'contacts')

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
  const isComposingRef = useRef(false)
  const handleSendMessageRef = useRef<(approveMessageId?: string, approvalRequestId?: string, planAction?: 'approve' | 'defer', overrideText?: string, regenerateReplyId?: string) => Promise<void>>(() => Promise.resolve())
  const handle1ClickApproveRef = useRef<(card: AgentActionCard, convId: string) => Promise<void>>(() => Promise.resolve())

  const loadConversation = useCallback(
    async (convId: string, showSpinner = false, all = false) => {
      if (showSpinner) setLoading(true)
      const version = ++httpFetchVersion.current
      try {
        const data = await chatApi.getConversation(
          convId,
          all ? { all: true } : { limit: 50 },
        )
        if (activeConvRef.current !== convId || version !== httpFetchVersion.current)
          return
        setLoading(false)
        setMessages((prev) => mergeConversationMessages(prev, data.messages || []))
        setHasMoreOlder(!!data.has_more)
        markConversationSeen(convId, data.messages || [])
        setRequests((prev) => {
          const next = data.requests || []
          if (prev.length === next.length) {
            let same = true
            for (let i = 0; i < prev.length; i++) {
              if (
                prev[i].request_id !== next[i].request_id ||
                prev[i].state !== next[i].state ||
                prev[i].safe_error !== next[i].safe_error
              ) {
                same = false
                break
              }
            }
            if (same) return prev
          }
          return next
        })
        setLinkedTasks((prev) => {
          const next = data.tasks || []
          if (prev.length === next.length) {
            let same = true
            for (let i = 0; i < prev.length; i++) {
              if (
                prev[i].id !== next[i].id ||
                prev[i].status !== next[i].status ||
                prev[i].updated_at !== next[i].updated_at ||
                prev[i].title !== next[i].title
              ) {
                same = false
                break
              }
            }
            if (same) return prev
          }
          return next
        })
        if (data.tasks) cacheTasks(data.tasks)
        if (data.contact && activeContact) {
          let updatedMeta = false
          if (data.contact.name && activeContact.name !== data.contact.name) {
            activeContact.name = data.contact.name
            updatedMeta = true
            window.dispatchEvent(
              new CustomEvent('cuan-conversation-updated', {
                detail: { roomId: convId, title: data.contact.name },
              }),
            )
          }
          if (data.contact.type && activeContact.type !== data.contact.type) {
            activeContact.type = data.contact.type
            updatedMeta = true
          }
          if (data.contact.avatar && activeContact.avatar !== data.contact.avatar) {
            activeContact.avatar = data.contact.avatar
            updatedMeta = true
          }
          if (updatedMeta) {
            onContactUpdated?.()
          }
        }
        if (activeContact && data.tasks) {
          const states = data.tasks.map(task => task.status === 'done' ? 'Completed' : workStatus(task))
          // The server summary also understands canceled attempts saved as review.
          const summary = data.contact
          const count = (field: keyof ChatContact, fallback: number) => typeof summary?.[field] === 'number' ? Number(summary[field]) : fallback
          const total = count('task_count', data.tasks.length)
          const at = count('active_tasks', states.filter(state => state === 'Working').length)
          const rt = count('review_tasks', states.filter(state => ['Ready for review', 'Answered'].includes(state)).length)
          const qt = count('queued_tasks', states.filter(state => state === 'Queued').length)
          const dt = count('done_tasks', states.filter(state => state === 'Completed').length)
          const ct = count('canceled_tasks', states.filter(state => state === 'Cancelled').length)
          const ft = count('failed_tasks', states.filter(state => ['Needs attention', 'Not running'].includes(state)).length)

          const changed =
            activeContact.task_count !== total ||
            activeContact.active_tasks !== at ||
            activeContact.review_tasks !== rt ||
            activeContact.queued_tasks !== qt ||
            activeContact.done_tasks !== dt ||
            activeContact.canceled_tasks !== ct ||
            activeContact.failed_tasks !== ft
          activeContact.task_count = total
          activeContact.active_tasks = at
          activeContact.review_tasks = rt
          activeContact.queued_tasks = qt
          activeContact.done_tasks = dt
          activeContact.canceled_tasks = ct
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
          t('Reconnecting…'),
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
  const autoResizeTextarea = useCallback((el: HTMLTextAreaElement) => {
    const next = resizeComposerInput(el)
    setIsMultiline((prev) => (prev !== next ? next : prev))
  }, [])

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
  }, [activeContact?.id, autoResizeTextarea])

  // Load conversation when activeContact changes
  useEffect(() => {
    if (activeContact) {
      const convId = activeContact.conversation_id || activeContact.id
      const isDifferentConv = prevLoadedConvId.current !== convId
      prevLoadedConvId.current = convId

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
      if (isDifferentConv) {
        setMessages([])
        setRequests([])
        setLinkedTasks([])
        setSendError('')
        setRetryCount(0)
        pendingRef.current = null
      }
      try {
        const saved = localStorage.getItem('cuan_inbox_pending:' + convId)
        if (saved) {
          pendingRef.current = JSON.parse(saved)
          setSendError(
            t(
              'Reconnecting… Your message is saved.',
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
      loadConversation(convId, isDifferentConv)
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
          setMessages((prev) => mergeConversationMessages(prev, data.messages || []))
          markConversationSeen(convId, data.messages || [])
          setRequests((prev) => {
            const next = data.requests || []
            if (prev.length === next.length) {
              let same = true
              for (let i = 0; i < prev.length; i++) {
                if (
                  prev[i].request_id !== next[i].request_id ||
                  prev[i].state !== next[i].state ||
                  prev[i].safe_error !== next[i].safe_error
                ) {
                  same = false
                  break
                }
              }
              if (same) return prev
            }
            return next
          })
          setLinkedTasks((prev) => {
            const next = data.tasks || []
            if (prev.length === next.length) {
              let same = true
              for (let i = 0; i < prev.length; i++) {
                if (
                  prev[i].id !== next[i].id ||
                  prev[i].status !== next[i].status ||
                  prev[i].updated_at !== next[i].updated_at ||
                  prev[i].title !== next[i].title
                ) {
                  same = false
                  break
                }
              }
              if (same) return prev
            }
            return next
          })
          if (data.tasks) cacheTasks(data.tasks)
          if (data.room_title && activeContact && activeContact.name !== data.room_title) {
            window.dispatchEvent(
              new CustomEvent('cuan-conversation-updated', {
                detail: { roomId: convId, title: data.room_title },
              }),
            )
            onContactUpdated?.()
          }
          if (activeContact && data.tasks) {
            const states = data.tasks.map(task => task.status === 'done' ? 'Completed' : workStatus(task))
            const total = data.tasks.length
            const at = states.filter(state => state === 'Working').length
            const rt = states.filter(state => ['Ready for review', 'Answered'].includes(state)).length
            const qt = states.filter(state => state === 'Queued').length
            const dt = states.filter(state => state === 'Completed').length
            const ct = states.filter(state => state === 'Cancelled').length
            const ft = states.filter(state => ['Needs attention', 'Not running'].includes(state)).length

            const changed =
              activeContact.task_count !== total ||
              activeContact.active_tasks !== at ||
              activeContact.review_tasks !== rt ||
              activeContact.queued_tasks !== qt ||
              activeContact.done_tasks !== dt ||
              activeContact.canceled_tasks !== ct ||
              activeContact.failed_tasks !== ft
            activeContact.task_count = total
            activeContact.active_tasks = at
            activeContact.review_tasks = rt
            activeContact.queued_tasks = qt
            activeContact.done_tasks = dt
            activeContact.canceled_tasks = ct
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
    const onFocusOrResume = () => {
      if (document.visibilityState === 'visible') {
        void loadConversation(convId)
        if (!connected) {
          clearTimeout(retry)
          void connect()
        }
      }
    }
    const onNotificationNavigate = (e: Event) => {
      const custom = e as CustomEvent<{ href?: string }>
      const href = custom?.detail?.href || ''
      setMobileView('chat')
      const match = href.match(/\/inbox\/([^/?#]+)/)
      const targetRoomId = match ? decodeURIComponent(match[1]) : convId
      const hasTargetMessage = href.includes('message=') || href.includes('msg=')
      if (targetRoomId && (targetRoomId === convId || !convId)) {
        void loadConversation(targetRoomId, true, hasTargetMessage)
      } else if (custom?.detail?.href && custom.detail.href.includes(convId)) {
        void loadConversation(convId, true)
      }
    }
    window.addEventListener('focus', onFocusOrResume)
    document.addEventListener('visibilitychange', onFocusOrResume)
    window.addEventListener('cuan:notification-navigate', onNotificationNavigate)
    return () => {
      abort.abort()
      clearTimeout(retry)
      clearInterval(poll)
      window.removeEventListener('focus', onFocusOrResume)
      document.removeEventListener('visibilitychange', onFocusOrResume)
      window.removeEventListener('cuan:notification-navigate', onNotificationNavigate)
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

  const handleSendMessage = async (approveMessageId?: string, approvalRequestId?: string, planAction?: 'approve' | 'defer', overrideText?: string, regenerateReplyId?: string) => {
    if (!activeContact || sending || uploads.busy) return
    const convId = activeContact.conversation_id || activeContact.id
    const pending = pendingRef.current?.convId === convId ? pendingRef.current : null
    let text = pending?.convId === convId ? pending.text : overrideText ? overrideText : approveMessageId ? t(planAction === 'defer' ? 'Not now' : 'Approve this plan') : inputMessage.trim()
    if (!pending && !regenerateReplyId && !approveMessageId && activeContact.type === 'customer' && isWhisper && text && !extractMentionHandle(text) && !text.startsWith('@')) {
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
            draftText: overrideText !== undefined || approveMessageId ? '' : inputMessage.trim(),
            id: approvalRequestId || crypto.randomUUID(),
            attachmentIds:
              approveMessageId || overrideText !== undefined
                ? []
                : uploads.items.map((a) => a.id),
            approveMessageId,
            planAction,
            regenerateReplyId,
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
    if (request.regenerateReplyId) setRegenerationStart({ convId, sourceId: request.regenerateReplyId, requestId: request.id })
    followTail.current = true
    setSending(true)
    setRetryCount((n) => n + 1)
    try {
      await chatApi.sendMessage(
        convId,
        request.text,
        'You',
        request.id,
        request.attachmentIds,
        request.approveMessageId,
        request.planAction,
        request.regenerateReplyId,
      )
      // Acceptance is authoritative even if the subsequent history read fails.
      // A lost response still retries this exact frozen payload and identifier.
      try {
        const key = 'cuan_inbox_pending:' + convId
        const saved = JSON.parse(localStorage.getItem(key) || 'null')
        if (saved?.id === request.id) localStorage.removeItem(key)
      } catch { /* The saved ID remains safe to reconcile on reload. */ }
      if (pendingRef.current === request) pendingRef.current = null
      if (request.attachmentIds?.length) uploads.acknowledge(request.attachmentIds, 'cuan-inbox-attachments:' + convId)
      if (unsentDrafts.current[convId]?.trim() === request.text)
        delete unsentDrafts.current[convId]
      if (activeConvRef.current === convId) {
        setInputMessage((current) => {
          if (current.trim() === request.text || current.trim() === request.draftText) {
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
            'Reconnecting… Your message is saved.',
          ),
        )
    } finally {
      setSending(false)
    }
  }

  // Recover uncertain delivery automatically without creating another message.
  useEffect(() => {
    if (!sendError || sending || !pendingRef.current || retryCount >= 3) return
    const timer = setTimeout(() => { void handleSendMessage() }, Math.min(1000 * 2 ** retryCount, 8000))
    return () => clearTimeout(timer)
  }, [sendError, sending, retryCount, activeContact?.id])

  useEffect(() => {
    const reconnect = () => {
      if (!sending && pendingRef.current?.convId === activeConvRef.current) {
        setRetryCount(0)
        void handleSendMessage()
      }
    }
    window.addEventListener('online', reconnect)
    return () => window.removeEventListener('online', reconnect)
  }, [sending, activeContact?.id])

  handleSendMessageRef.current = handleSendMessage

  const handleKeyDown = (e: React.KeyboardEvent<HTMLTextAreaElement>) => {
    if (e.key === 'Enter' && !e.shiftKey && !e.nativeEvent.isComposing && !isComposingRef.current) {
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
      window.dispatchEvent(
        new CustomEvent('cuan-conversation-updated', {
          detail: { roomId: convId, title: card.task_title },
        }),
      )
      // Reload messages to get execution confirmation
      loadConversation(convId, false)
      if (onContactUpdated) onContactUpdated()
    } catch (err) {
      console.error('Failed to approve task:', err)
    } finally {
      setApproving(null)
    }
  }
  handle1ClickApproveRef.current = handle1ClickApprove

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
  const regeneratingDrafts = useMemo(() => {
    const sourceByControl = new Map(messages.filter((m) => m.regenerate_reply_id && m.uuid).map((m) => [m.uuid!, m.regenerate_reply_id!]))
    const active = new Set<string>()
    for (const q of requests) {
      const source = sourceByControl.get(q.message_id)
      if (source && ['queued', 'running', 'cancelling'].includes(q.state)) active.add(source)
    }
    if (regenerationStart?.convId === activeConvRef.current) {
      const request = requests.find((q) => q.request_id === regenerationStart.requestId)
      if ((!request && (!sendError || retryCount < 3 || sending)) || (request && ['queued', 'running', 'cancelling'].includes(request.state))) active.add(regenerationStart.sourceId)
    }
    const pending = pendingRef.current
    if (pending?.convId === activeConvRef.current && pending.regenerateReplyId && (sending || retryCount < 3)) active.add(pending.regenerateReplyId)
    return active
  }, [messages, requests, sending, retryCount, regenerationStart, sendError])

  const renderedMessages = useMemo(() => {
    let previousBubbleMessage: ChatMessage | undefined
    if (!activeContact) return null
    return messages.map((msg, index) => {
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
                // Internal agent delegation in a customer room has the same task
                // attachment as an agent conversation. This is the owner's Inbox.
                const isTaskAgentMessage = isAgentMessage ||
                  (msg.author_type === 'agent' && msg.is_internal === true) ||
                  (msg.author_type === 'system' && activeContact.type === 'agent')
                const isFormattedMessage = true

                const isOutgoing = isUserInAgentRoom || isOperatorInCustomerRoom
                const msgContent = msg.content || msg.message || ''

                // Suppress internal system cues (e.g. cuan_work_review) and stale notices
                if (
                  msg.author_id === 'cuan_work_review' ||
                  msgContent.startsWith('I’m checking the latest work update') ||
                  msgContent.startsWith("I'm checking the latest work update") ||
                  msgContent.startsWith('Review the result for [') ||
                  msgContent.startsWith('This update is no longer current')
                ) {
                  return null
                }

                const linkedTasksForMsg = linkedTasks.filter(task => latestMessageIdPerTask.get(task.id) === (msg.uuid || String(msg.id)))
                const linkedTaskForMsg = linkedTasksForMsg[0]

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

                const msgTaskIdMatch = msgContent.match(/\/tasks\/([A-Za-z0-9_-]+)/)
                const referencedTaskIds = [...msgContent.matchAll(/\/tasks\/([A-Za-z0-9_-]+)/g)].map(match => match[1])
                const isSupersededTaskMessage = !isOutgoing && referencedTaskIds.length > 0 && referencedTaskIds.every(id =>
                  latestMessageIdPerTask.has(id) && latestMessageIdPerTask.get(id) !== (msg.uuid || String(msg.id)))

                const date = new Date(msg.created_at)
                const showDate =
                  !msg.regenerate_reply_id && Number.isFinite(date.getTime()) &&
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
                const backgroundWork = isBackgroundWorkUpdate(msg)
                if (backgroundWork && (activeContact.type === 'agent' || !linkedTaskForMsg) && !msg.reply_draft) return null
                const groupStart = startsMessageGroup(msg, previousBubbleMessage)
                if (!msg.regenerate_reply_id && !backgroundWork) previousBubbleMessage = msg
                const hasSpeech = !backgroundWork && withoutTaskLaunchAcknowledgement(msgContent.replace(/\[[^\]]*\]\(\/tasks\/[^)]+\)/g, '').replace(/\/tasks\/[A-Za-z0-9_-]+/g, '').trim(), Boolean(msgTaskIdMatch || linkedTaskForMsg), isOutgoing).length > 0
                const hasBubbleContent = hasSpeech || (!msg.reply_draft && !(linkedTaskForMsg || isSupersededTaskMessage) && msgContent.trim().length > 0) || !!(msg.attachments?.length || msg.attachment_url || msg.approval_title || msg.approval_plan || msg.action_card || msg.receipt_extraction || msg.product)
                const showMessageSurface = !backgroundWork && !msg.regenerate_reply_id && (hasBubbleContent || !!msg.prompt_choices)
                const onlyImages = msg.attachments?.length
                  ? msg.attachments.every(isImageAttachment)
                  : !!msg.attachment_url && isImageAttachment({ name: msg.attachment_name || '', type: msg.attachment_type })
                const imageBubble = onlyImages && !msg.approval_title && !msg.action_card && !msg.product && !msg.receipt_extraction && !msg.prompt_choices
                const imageOnly = imageBubble && !hasSpeech && !msg.email_delivery
                const emailStatusLabel = msg.email_delivery ? t(({ sent: 'Email sent', delivered: 'Email delivered', delivery_delayed: 'Email delayed', bounced: 'Email bounced', failed: 'Email failed', complained: 'Email reported', suppressed: 'Email suppressed' } as Record<string, string>)[msg.email_delivery.status] || 'Email status unavailable') : ''
                const emailDescription = msg.email_delivery ? `${emailStatusLabel} · ${t(msg.email_delivery.recipient_type === 'operator' ? 'Email notification to your team; independent of chat read status' : 'Email delivery to customer; independent of chat read status')}` : ''
                const metadata = (
                  <span className="wa-message-meta">
                    {msg.email_delivery && !msg.is_internal && <span className="inline-flex items-center" title={emailDescription} role={msg.email_delivery.status === 'delivered' ? 'img' : undefined} aria-label={msg.email_delivery.status === 'delivered' ? emailDescription : undefined}>
                      {msg.email_delivery.status === 'delivered'
                        ? <MailCheck className="h-3.5 w-3.5" aria-hidden="true" />
                        : <>{msg.email_delivery.recipient_type === 'operator' ? t('Team') + ' · ' : ''}{emailStatusLabel}</>}
                    </span>}
                    <time dateTime={msg.created_at}>
                      {formatMessageTime(msg.created_at)}
                    </time>
                    {isOutgoing && (
                      (() => {
                        const publicCustomerMessage = activeContact.type === 'customer' && !msg.is_internal
                        const read = publicCustomerMessage && !!msg.read_confirmed_at
                        const delivered = publicCustomerMessage && !!msg.delivered_at
                        const ReceiptIcon = read || delivered ? CheckCheck : Check
                        const label = read ? t('Read in chat') : delivered ? t('Delivered to chat') : t('Message saved')
                        return <span title={label}><ReceiptIcon className={cn('h-3.5 w-3.5', read && 'text-[#4384ad] dark:text-[#7da6c1]')} role="img" aria-label={label} /></span>
                      })()
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
                    {showMessageSurface && <ItemActions subtle swipeAction={{ label: 'Quote message', previewLabel: 'Reply', run: quoteMessage }} label={t('Message') + ' ' + String(msg.id || index + 1)} actions={[
                      { label: 'Copy message', run: () => navigator.clipboard.writeText(msg.content || msg.message) },
                      { label: 'Quote message', run: quoteMessage },
                      { label: 'Copy link', run: () => copyItemLink('/inbox/' + encodeURIComponent(activeContact.conversation_id || activeContact.id) + (msg.uuid ? '?message=' + encodeURIComponent(msg.uuid) : '')) },
                      ...(activeContact.type === 'customer' ? [{
                        label: t('Whisper to @cuan'),
                        run: () => {
                          const body = msg.content || msg.message || ''
                          const snippet = body.length > 80 ? body.slice(0, 77) + '…' : body
                          setInputMessage(`@cuan regarding "${snippet}": `)
                          textareaRef.current?.focus()
                          if (textareaRef.current) autoResizeTextarea(textareaRef.current)
                        },
                      }] : []),
                      ...(msg.uuid ? [{
                        label: t('Escalate to task'),
                        run: () => setEscalatingMessage(msg),
                      }] : []),
                    ]}>
                    {hasBubbleContent && (
                    <div
                      data-bubble-tail={groupStart && hasSpeech ? (isOutgoing ? 'outgoing' : 'incoming') : undefined}
                      className={cn(
                        'wa-message-bubble relative min-w-0 rounded-2xl',
                        imageBubble && 'wa-image-bubble',
                        imageOnly && 'wa-image-only',
                        msg.is_internal && 'wa-whisper',
                        msg.uuid === requestedMessageId && 'ring-2 ring-ring',
                        isOutgoing ? 'wa-outgoing' : 'wa-incoming',
                      )}
                    >
                      {msg.is_internal && !imageBubble && (msgContent.trim() || !(msg.attachments?.length || msg.attachment_url)) && (
                        <div className="-ml-3 -mt-2 -mb-1 w-fit"><InternalVisibility /></div>
                      )}
                      {msg.approval_title && !msg.approval_plan && (
                        <p className="mb-2 text-xs font-medium">
                          {t(msg.approval_accepted ? 'Approved' : msg.plan_deferred ? 'Saved for later' : 'Needs approval')}
                        </p>
                      )}
                      {msg.attachments?.map((a) => (
                        <div key={a.id} className={imageBubble ? 'wa-image-attachment' : 'mb-2 last:mb-0'}><AttachmentFile attachment={a} imagePreview internal={msg.is_internal === true} /></div>
                      ))}
                      {msg.attachment_url && !msg.attachments?.length && (
                        <div className={imageBubble ? 'wa-image-attachment' : undefined}>
                        <AttachmentFile
                          imagePreview
                          internal={msg.is_internal === true}
                          attachment={{
                            id: msg.attachment_url,
                            name: msg.attachment_name || 'Customer file',
                            type: msg.attachment_type || 'application/octet-stream',
                            size: msg.attachment_size || 0,
                            url: msg.attachment_url,
                          }}
                        />
                        </div>
                      )}
                      {msg.product && (
                        <p className="mb-1 text-xs font-medium opacity-80">
                          {msg.product}
                        </p>
                      )}

                      {/* Message Body */}
                      {isFormattedMessage ? (
                        <MessageContent
                          content={msgContent}
                          hideTaskActions={Boolean(linkedTaskForMsg || isSupersededTaskMessage)}
                          receiptExtraction={msg.receipt_extraction}
                          onTaskClick={handleTaskDetailsClick}
                          onOpenTerminal={handleTaskTerminalClick}
                          onUseDraft={handleUseDraft}
                          onRetry={handleTaskRetry}
                          onContinueChat={handleTaskDiscussion}
                          isCustomerRoom={activeContact.type === 'customer'}
                          hasSiblingCard={!!(isAgentMessage && msg.approval_plan) || !!msg.action_card}
                          isOutgoing={isOutgoing}
                        />
                      ) : (
                        <div className="wa-plain-text whitespace-pre-wrap break-words font-sans">
                          <span>{msgContent}</span>
                          {metadata}
                        </div>
                      )}

                      {(isAgentMessage || msg.author_type === 'agent') && msg.approval_plan && (
                        <ApprovalCard
                          plan={msg.approval_plan}
                          accepted={msg.approval_accepted}
                          deferred={msg.plan_deferred}
                          disabled={sending || uploads.busy || retryCount >= 3 || !!pendingRef.current}
                          onApprove={() => handleSendMessageRef.current(msg.uuid, msg.approval_request_id)}
                          onChangePlan={() => {
                            setInputMessage(current => (current ? current + '\n\n' : '') + t('Changes to this plan') + ' — ' + msg.approval_plan!.title + ': ')
                            textareaRef.current?.focus()
                          }}
                          onDefer={() => handleSendMessageRef.current(msg.uuid, msg.plan_defer_request_id, 'defer')}
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
                                  handle1ClickApproveRef.current(
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

                      {isOutgoing && msg.uuid && linkedTasks.some((t) => t.source_message_id === msg.uuid) && (
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
                    )}
                    {(isAgentMessage || (msg.author_type === 'agent' && msg.is_internal === true)) && msg.prompt_choices && (
                      <PromptChoiceChips
                        prompt={msg.prompt_choices}
                        onOther={() => {
                          if (activeContact.type === 'customer') setWhisperMode(true)
                        }}
                        disabled={sending || uploads.busy || retryCount >= 3 || !!pendingRef.current}
                        onSelect={(option) => handleSendMessageRef.current(undefined, undefined, undefined, activeContact.type === 'customer' ? '@cuan ' + option.label : option.label)}
                        className={hasBubbleContent ? 'mt-2.5' : 'mt-0.5'}
                      />
                    )}
                    </ItemActions>}
                      {msg.uuid && isTaskAgentMessage && !isSupersededTaskMessage && linkedTasksForMsg.length === 0 && (
                        <InboxTaskAction
                          key={msg.uuid}
                          conversationId={
                            activeContact.conversation_id || activeContact.id
                          }
                          messageId={msg.uuid!}
                          draft={isTaskAgentMessage ? msg.task_draft : undefined}
                          onViewTask={(task) => handleOpenTaskDrawer(task, 'details')}
                          onOpenTerminal={(task) => handleOpenTaskDrawer(task, 'terminal')}
                          onUseDraft={handleUseDraft}
                          onRetry={handleTaskRetry}
                          onContinueChat={handleTaskDiscussion}
                          isCustomerRoom={activeContact.type === 'customer'}
                        />
                      )}
                    {msg.reply_draft && (
                      <ReplyDraftCard
                        draft={msg.reply_draft}
                        internal={msg.is_internal === true}
                        messageId={msg.uuid}
                        conversationId={activeContact.conversation_id || activeContact.id}
                        defaultRecipient={activeContact.email}
                        emailDelivery={msg.email_delivery}
                        regenerateDisabled={sending || uploads.busy || !!pendingRef.current || requests.some((q) => ['queued', 'running', 'retrying'].includes(q.state))}
                        regenerating={!!msg.uuid && regeneratingDrafts.has(msg.uuid)}
                        onRegenerate={msg.uuid ? () => { void handleSendMessageRef.current(undefined, undefined, undefined, 'Regenerate draft', msg.uuid) } : undefined}
                        onUse={(draft) => {
                          setInputMessage(draft)
                          textareaRef.current?.focus()
                          if (textareaRef.current) autoResizeTextarea(textareaRef.current)
                        }}
                        onSendEmail={async (recipient) => {
                          if (!msg.uuid) return
                          const convId = activeContact.conversation_id || activeContact.id
                          await sendCustomerReplyEmail(convId, msg.uuid, { recipient, draft: msg.reply_draft })
                          await loadConversation(convId)
                        }}
                      />
                    )}
                    {(!msg.work_update ? requestsByMessage.get(msg.uuid || '') || [] : []).map((q) => {
                      const placeholder =
                        (q.agent_id || activeContact.type === 'agent') &&
                        q.reply_id !== msg.uuid
                      const activity = (
                        <InboxActivity
                          request={q}
                          bubble={!!placeholder && !msg.regenerate_reply_id}
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
                          <div className="inbox-attachment relative">
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
              })
              }, [
                messages,
                activeContact,
                linkedTasks,
                latestMessageIdPerTask,
                approving,
                approvedTasks,
                regeneratingDrafts,
                requestsByMessage,
                requestAttempts,
                sending,
                uploads.busy,
                retryCount,
                requestedMessageId,
                language,
                t,
                handleTaskDetailsClick,
                handleTaskTerminalClick,
                handleUseDraft,
                handleTaskRetry,
                handleTaskDiscussion,
                handleOpenTaskDrawer,
  ])
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
          peekTask && 'min-[1200px]:hidden min-[1600px]:flex',
        )}
      >
        {canvasMode === 'studio' ? (
          <div className="flex h-full flex-1 flex-col overflow-hidden">
            <PageHeader className="wa-studio-header"><PageTitle>{t('New work')}</PageTitle></PageHeader>
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
          </div>
        ) : !activeContact ? (
          <div className="flex h-full flex-1 flex-col items-center justify-center p-8 text-center bg-background select-none">
            <div className="h-16 w-16 mb-4 flex items-center justify-center border border-border/60 bg-muted/20 text-muted-foreground">
              <InboxIcon className="h-8 w-8 stroke-[1.25]" />
            </div>
            <h3 className="text-xl font-bold tracking-tight text-foreground">{t('Select a conversation')}</h3>
            <p className="mt-1 text-sm text-muted-foreground max-w-sm">
              {t('Choose a conversation from your inbox to read the full thread, view customer details, and reply.')}
            </p>
          </div>
        ) : (
          <>
            {/* ── Top Header (aligning with shadcn Mail h-13) ── */}
            <PageHeader className="wa-chat-header border-b-0 min-[1200px]:border-b">
              <div className="flex flex-1 items-center gap-1.5 sm:gap-2.5 min-w-0">
                {/* Mobile Back Button (<) */}
                <IconButton label={t('Back to conversations')} onClick={handleBackToContacts} className="min-[1200px]:hidden"><ChevronLeft /></IconButton>

                <div className="relative shrink-0">
                  <div
                    data-avatar="true"
                    className="h-8 w-8 rounded-full bg-secondary flex items-center justify-center text-xs font-bold border border-border avatar overflow-hidden"
                  >
                    {activeContact.avatar ? (
                      <span>{activeContact.avatar}</span>
                    ) : activeContact.type === 'agent' ? (
                      <Bot className="h-4 w-4 text-primary" />
                    ) : (
                      <User className="h-4 w-4 text-muted-foreground" />
                    )}
                  </div>
                </div>

                <div className="min-w-0 flex-1 flex flex-col justify-center">
                  <div className="flex min-w-0 flex-col sm:flex-row sm:items-center sm:gap-2">
                    <h2 className="truncate text-base font-semibold leading-tight text-foreground">
                      {contactDisplayName || activeContact.name}
                    </h2>
                    {activeContact.type === 'customer' && activeContact.email && (
                      <div className="flex items-center gap-1 min-w-0" data-testid="customer-email-header">
                        <span className="text-muted-foreground text-xs hidden sm:inline" aria-hidden="true">·</span>
                        <span
                          className="truncate text-xs text-muted-foreground font-normal max-w-full sm:max-w-[200px]"
                          title={activeContact.email}
                          data-testid="customer-email-text"
                        >
                          {activeContact.email}
                        </span>

                      </div>
                    )}
                  </div>
                  {activeContact.type !== 'customer' && !(activeContact.type === 'agent' && agents.some(a => a.id === 'agent_cuan')) && <ContactStatus
                    contact={activeContact}
                    agents={agents}
                    now={now}
                  />}
                </div>
              </div>

              <div className="flex shrink-0 items-center gap-1">
                <ConversationResolutionAction key={activeContact.conversation_id} contact={activeContact} compact onChanged={onContactUpdated} />
                {activeContact.type === 'customer' && activeContact.email && (
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
                            <span className="inline-flex items-center text-emerald-600 dark:text-emerald-400">
                              <Check className="h-3.5 w-3.5 shrink-0" />
                            </span>
                          ) : (
                            <Copy className="h-3.5 w-3.5 shrink-0" />
                          )}
                        </button>
                )}
                {activeContact.type === 'customer' && activeContact.contact_id && (
                  <Link
                    className="icon-action text-muted-foreground hover:text-foreground"
                    aria-label="View contact details"
                    title={t('View contact details & memory')}
                    to={`/contacts?contact=${encodeURIComponent(activeContact.contact_id)}`}
                  >
                    <User className="h-4 w-4" />
                  </Link>
                )}
                {/* Hidden New Studio button preserved for test contracts */}
                <button
                  type="button"
                  data-testid="header-new-studio-btn"
                  onClick={handleOpenStudio}
                  className="hidden"
                  aria-hidden="true"
                />
              </div>
            </PageHeader>

            {/* Sub-tabs: Obrolan (Chat) & Pekerjaan (Tasks) eliminated for serene canvas in S59-T6 */}

            {/* ── Main Chat Stream (WhatsApp/Telegram Style) ── */}
            <div className="relative flex-1 min-h-0 min-w-0 flex flex-col">
              <div
                ref={messageListRef}
                onScroll={handleScroll}
                onClick={handleMessageContainerClick}
                className="inbox-message-list wa-conversation calm-conversation min-h-0 flex-1 overflow-y-auto overscroll-contain px-3 py-3 sm:px-6 sm:py-4 space-y-2"
              >
                <div className="calm-stream-constraint max-w-[75ch] mx-auto w-full space-y-2 pb-4">
              {messages.length === 0 && !loading && (
                <div className="flex flex-col items-center justify-center py-10 px-4 text-center text-muted-foreground space-y-3 max-w-sm mx-auto">
                  {activeContact?.type === 'customer' ? (
                    <>
                      <div className="relative">
                        <div data-avatar="true" className="h-12 w-12 rounded-full bg-secondary flex items-center justify-center text-lg border border-border shadow-xs">
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
                      <div data-avatar="true" className="h-12 w-12 rounded-full bg-secondary flex items-center justify-center text-lg border border-border/50 shadow-2xs">
                        <Bot className="h-6 w-6 text-muted-foreground" />
                      </div>
                      <div className="space-y-1">
                        <p className="text-sm font-semibold text-foreground">
                          {activeContact?.name || t('Cuan Assistant')}
                        </p>
                        <p className="text-xs text-muted-foreground">
                          {t('Sampaikan kebutuhan operasional atau pilih contoh tugas di bawah:')}
                        </p>
                      </div>
                      <div data-testid="agent-prompt-starters" className="flex flex-wrap justify-center gap-1.5 pt-1 max-w-md">
                        {[
                          {
                            emoji: '🎯',
                            label: t('Analisis CRM & Draf Broadcast'),
                            text: t('Tolong analisis segmen kontak pelanggan CRM dan siapkan draf email broadcast promosi yang relevan.'),
                          },
                          {
                            emoji: '🛠️',
                            label: t('Webhook & Bot WhatsApp'),
                            text: t('Tolong hubungkan webhook form order ke bot WhatsApp notifikasi admin.'),
                          },
                          {
                            emoji: '⚡',
                            label: t('Kesehatan Server & Antrean'),
                            text: t('Tolong periksa status kesehatan daemon runner, antrean tugas yang berjalan, dan metrik sistem VPS saat ini.'),
                          },
                          {
                            emoji: '📋',
                            label: t('Fitur Baru & Kriteria Selesai (DoD)'),
                            text: t('Saya ingin membuat peningkatan fitur baru di project. Tolong pelajari codebase, buatkan Approval Card dengan rencana kerja dan kriteria pengujian (DoD) sebelum eksekusi.'),
                          },
                        ].map((item) => (
                          <button
                            key={item.label}
                            type="button"
                            data-testid="starter-chip"
                            onClick={() => {
                              setInputMessage(item.text)
                              textareaRef.current?.focus()
                            }}
                            className="px-2.5 py-1 text-xs rounded-full border border-border/60 bg-muted/20 hover:bg-muted/50 text-foreground transition active:scale-95 text-left inline-flex items-center gap-1.5 cursor-pointer"
                          >
                            <span>{item.emoji}</span>
                            <span>{item.label}</span>
                          </button>
                        ))}
                      </div>
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
              {renderedMessages}

              {sendError && (
                <div
                  role="alert"
                  className="mx-auto max-w-2xl px-3 py-2 text-xs text-muted-foreground"
                >
                  <p>{sendError}</p>
                  {pendingRef.current && retryCount >= 3 && (
                    <button
                      className="mt-2 underline"
                      disabled={sending}
                      onClick={() => handleSendMessage()}
                    >
                      {t('Try again')}
                    </button>
                  )}
                </div>
              )}

              {/* Typing Indicator */}
              {sending && activeContact.type === 'agent' && (
                <div
                  className="flex flex-col items-start animate-fade-in my-2"
                  data-testid="inbox-working-indicator"
                >
                  <div className="bg-card text-card-foreground border border-border/80 rounded-2xl rounded-tl-xs p-3 shadow-xs flex items-center gap-2.5">
                    <div data-avatar="true" className="h-6 w-6 rounded-full bg-secondary flex items-center justify-center text-xs shrink-0">
                      {activeContact.avatar ? (
                        <span>{activeContact.avatar}</span>
                      ) : (
                        <Bot className="h-3.5 w-3.5 text-primary" />
                      )}
                    </div>
                    <span className="text-xs text-muted-foreground font-medium">
                      {t('Sending message to')} {activeContact.name}...
                    </span>
                    <div className="flex items-center gap-1 ml-1 shrink-0">
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
              className="wa-composer shrink-0 z-20 border-t-0 bg-transparent"
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
              <div className="mx-auto max-w-[75ch] bg-transparent">
                <AttachmentPreviews uploads={uploads} disabled={sending} internal={activeContact.type === 'customer' && isWhisper} />
                <ComposerTasks
                  key={activeContact.conversation_id || activeContact.id}
                  tasks={linkedTasks}
                  onDetails={task => handleOpenTaskDrawer(task)}
                  onTerminal={task => handleOpenTaskDrawer(task, 'terminal')}
                  modes={activeContact.type === 'customer' && isWhisper ? (
                    <div className="flex items-center gap-1.5 py-1 px-1 text-xs text-amber-600 dark:text-amber-400 font-medium animate-in fade-in duration-150">
                      <Lock className="h-3 w-3 shrink-0" />
                      <span>{detectedMention && detectedMention !== '@cuan' ? `${t('Whisper AI:')} ${detectedMention}` : t('Whisper @cuan')}</span>
                      <span className="text-[11px] text-muted-foreground font-normal">({t('Internal · Invisible to visitor')})</span>
                    </div>
                  ) : undefined}
                />
                <Composer
                  id="compose-panel"
                  aria-description={activeContact.type === 'customer' ? (isWhisper ? t('Whisper @cuan') : t('Public reply')) : undefined}
                  className={`${
                    isMultiline ? 'is-multiline' : ''
                  } ${
                    activeContact.type === 'customer' && isWhisper ? 'ring-1 ring-amber-500/50 border-amber-500/40 bg-amber-500/[0.02]' : ''
                  }`}
                >
                  <ComposerAccessory>
                  <AttachmentPicker
                    key={activeContact.conversation_id || activeContact.id}
                    uploads={uploads}
                    disabled={sending}
                    compact
                    disableImages={activeContact.type === 'agent' && !contactSupportsVision}
                  />
                  </ComposerAccessory>
                  <ComposerInput
                    ref={textareaRef}
                    data-primary-composer
                    aria-label={t('Message')}
                    rows={1}
                    value={inputMessage}
                    onCompositionStart={() => {
                      isComposingRef.current = true
                    }}
                    onCompositionEnd={(e) => {
                      isComposingRef.current = false
                      const val = (e.target as HTMLTextAreaElement).value
                      unsentDrafts.current[activeConvRef.current] = val
                      setInputMessage(val)
                      autoResizeTextarea(e.target as HTMLTextAreaElement)
                    }}
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
                        : isWhisper
                        ? t('Internal message…')
                        : t('Write a reply…')
                    }
                  />

                  {activeContact.type === 'customer' && (
                    <ComposerAccessory>
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
                    </ComposerAccessory>
                  )}

                  <ComposerSendButton
                    type="button"
                    onClick={() => handleSendMessage()}
                    disabled={
                      (!inputMessage.trim() && !uploads.items.length) ||
                      sending ||
                      uploads.busy
                    }
                    className={isWhisper ? 'text-amber-500 hover:text-amber-400 bg-amber-500/15 hover:bg-amber-500/25' : undefined}
                    title={isWhisper ? t('Send internal whisper to AI') : `${t('Send message (Enter)')} (${t('Public reply')})`}
                    aria-label={isWhisper ? t('Send internal whisper to AI') : t('Send message')}
                  >
                    {isWhisper ? (
                      <Lock className="h-4 w-4" />
                    ) : (
                      <Send className="h-[18px] w-[18px]" />
                    )}
                  </ComposerSendButton>
                </Composer>
              </div>
            </div>
          </>
        )}
      </div>

      {peekTask && <InboxWorkPanel key={peekTask.id + ':' + peekTab} task={peekTask} view={peekTab} onBack={closeWorkPanel} />}

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
                if (newTask.title) {
                  activeContact.name = newTask.title
                }
              }
              if (newTask.title) {
                const targetId = activeContact?.conversation_id || activeContact?.id || ''
                window.dispatchEvent(
                  new CustomEvent('cuan-conversation-updated', {
                    detail: { roomId: targetId, title: newTask.title },
                  }),
                )
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
