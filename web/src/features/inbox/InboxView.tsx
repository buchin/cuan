import { useState, useEffect, useRef, useCallback } from 'react'
import { ChatContact, ChatMessage, InboxRequest, chatApi, AgentActionCard } from '../../api/chat'
import {
  Send,
  Bot,
  User,
  CheckCheck,
  Play,
  CheckSquare,
  Sparkles,
  RefreshCw,
  ShoppingBag,
  ChevronLeft,
  Menu,
} from 'lucide-react'
import { cn } from '../../lib/utils'
import { InboxSubPanel, ContactFilterTab } from './InboxSubPanel'

interface InboxViewProps {
  contacts?: ChatContact[]
  activeContact?: ChatContact
  onSelectContact?: (contact: ChatContact) => void
  onContactUpdated?: () => void
  contactTab?: ContactFilterTab
  onContactTabChange?: (tab: ContactFilterTab) => void
  contactSearch?: string
  onContactSearchChange?: (q: string) => void
  onBackToContacts?: () => void
  onMobileChatActiveChange?: (active: boolean) => void
  onOpenMobileDrawer?: () => void
}

export function InboxView({
  contacts = [],
  activeContact,
  onSelectContact,
  onContactUpdated,
  contactTab = 'all',
  onContactTabChange,
  contactSearch = '',
  onContactSearchChange,
  onBackToContacts,
  onMobileChatActiveChange,
  onOpenMobileDrawer,
}: InboxViewProps) {
  const [requests, setRequests] = useState<InboxRequest[]>([])
  const [sendError, setSendError] = useState('')
  const [retryCount, setRetryCount] = useState(0)
  const pendingRef = useRef<{ convId: string; text: string; id: string } | null>(null)
  const activeConvRef = useRef('')
  activeConvRef.current = activeContact?.conversation_id || activeContact?.id || ''
  const [messages, setMessages] = useState<ChatMessage[]>([])
  const [inputMessage, setInputMessage] = useState('')
  const [loading, setLoading] = useState(false)
  const [sending, setSending] = useState(false)
  const [approving, setApproving] = useState<string | null>(null)
  const [approvedTasks, setApprovedTasks] = useState<Record<string, boolean>>({})

  // Mobile Single-View WhatsApp/Telegram state: 'contacts' | 'chat'
  const [mobileView, setMobileView] = useState<'contacts' | 'chat'>('contacts')

  useEffect(() => {
    if (activeContact) setMobileView('chat')
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

  const loadConversation = useCallback(async (convId: string, showSpinner = false) => {
    if (showSpinner) setLoading(true)
    try {
      const data = await chatApi.getConversation(convId)
      if (activeConvRef.current !== convId) return
      setMessages(data.messages || [])
      setRequests(data.requests || [])
      const pending = pendingRef.current
      if (pending?.convId === convId && data.requests?.some(q => q.request_id === pending.id)) {
        localStorage.removeItem('cuan_inbox_pending:' + convId)
        pendingRef.current = null
        setSendError('')
        setInputMessage('')
        setRetryCount(0)
      }
    } catch (err) {
      setSendError('Riwayat belum dapat dimuat. Segarkan lagi untuk memeriksa status pesan.')
    } finally {
      if (showSpinner) setLoading(false)
    }
  }, [])

  // Auto-resize textarea
  const autoResizeTextarea = (el: HTMLTextAreaElement) => {
    el.style.height = 'auto'
    const scrollH = el.scrollHeight
    const newH = Math.min(Math.max(scrollH, 42), 140)
    el.style.height = newH + 'px'
    el.style.overflowY = scrollH > 140 ? 'auto' : 'hidden'
  }

  // Load conversation when activeContact changes
  useEffect(() => {
    if (activeContact) {
      const convId = activeContact.conversation_id || activeContact.id
      setMessages([])
      setRequests([])
      setSendError('')
      setRetryCount(0)
      pendingRef.current = null
      try {
        const saved = localStorage.getItem('cuan_inbox_pending:' + convId)
        if (saved) { pendingRef.current = JSON.parse(saved); setSendError('Pengiriman sebelumnya belum terkonfirmasi. Segarkan atau periksa lagi dengan ID yang sama.') }
      } catch { setSendError('Penyimpanan browser tidak tersedia. Segarkan riwayat sebelum mengirim lagi.') }
      loadConversation(convId, true)
    }
  }, [activeContact?.conversation_id, activeContact?.id, loadConversation])

  // Polling every 3.5 seconds
  useEffect(() => {
    if (!activeContact) return
    const timer = setInterval(() => {
      loadConversation(activeContact.conversation_id || activeContact.id, false)
    }, 3500)
    return () => clearInterval(timer)
  }, [activeContact?.conversation_id, activeContact?.id, loadConversation])

  const lastMessageKey = messages[messages.length - 1]?.uuid || messages[messages.length - 1]?.id

  // Only scroll for a new message, not identical polling results.
  useEffect(() => {
    if (messageListRef.current) {
      messageListRef.current.scrollTop = messageListRef.current.scrollHeight
    }
  }, [lastMessageKey, activeContact?.id, sendError, requests.map(q => q.request_id + q.state).join(',')])

  const handleSendMessage = async () => {
    if (!activeContact || sending || retryCount >= 3) return
    const convId = activeContact.conversation_id || activeContact.id
    const pending = pendingRef.current
    const text = pending?.convId === convId ? pending.text : inputMessage.trim()
    if (!text) return
    const request = pending?.convId === convId ? pending : { convId, text, id: crypto.randomUUID() }
    // Save before the first network byte; reload/retry reuses the same key.
    try { localStorage.setItem('cuan_inbox_pending:' + convId, JSON.stringify(request)) }
    catch { setSendError('Browser tidak dapat menyimpan ID pesan. Aktifkan penyimpanan browser lalu coba lagi.'); return }
    pendingRef.current = request
    setSending(true)
    setRetryCount(n => n + 1)
    try {
      await chatApi.sendMessage(convId, text, 'Buchin', request.id)
      if (activeConvRef.current === convId) {
        setInputMessage('')
        setSendError('')
        setRetryCount(0)
        await loadConversation(convId)
      }
      onContactUpdated?.()
    } catch {
      if (activeConvRef.current === convId) setSendError('Pengiriman belum terkonfirmasi. Pesan mungkin sudah diterima. Segarkan atau periksa lagi dengan ID yang sama; tidak membuat eksekusi ganda.')
    } finally { setSending(false) }
  }

  const handleKeyDown = (e: React.KeyboardEvent<HTMLTextAreaElement>) => {
    if (e.key === 'Enter' && !e.shiftKey) {
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
      return d.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })
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

  return (
    <div className="h-full min-h-0 min-w-0 w-full flex overflow-hidden">
      {/* ── Mobile Contacts List View (100% width on < md when in 'contacts' state) ── */}
      <div
        className={cn(
          'w-full h-full md:hidden flex-col',
          mobileView === 'contacts' ? 'flex' : 'hidden'
        )}
      >
        <InboxSubPanel
          contacts={contacts}
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
          'flex-1 min-h-0 min-w-0 flex-col h-full bg-background overflow-hidden',
          mobileView === 'chat' ? 'flex' : 'hidden md:flex'
        )}
      >
        {!activeContact ? (
          <div className="h-full flex flex-col items-center justify-center p-8 text-center bg-background select-none">
            <div className="h-16 w-16 rounded-full bg-secondary flex items-center justify-center text-secondary-foreground mb-4 shadow-sm border border-border">
              <Bot className="h-8 w-8 text-primary" />
            </div>
            <h3 className="text-base font-bold text-foreground">Direct Messenger Hub</h3>
            <p className="text-xs text-muted-foreground max-w-sm mt-1 leading-relaxed">
              Pilih kontak untuk mulai mengobrol, atau tambahkan agen Anda di halaman Agen.
            </p>
          </div>
        ) : (
          <>
            {/* ── Top Header ── */}
            <div className="h-14 px-3 sm:px-6 border-b border-border bg-card text-card-foreground flex items-center justify-between shrink-0 shadow-xs">
              <div className="flex items-center gap-1.5 sm:gap-3 min-w-0">
                {/* Mobile Back Button (<) */}
                <button
                  type="button"
                  onClick={handleBackToContacts}
                  title="Kembali ke Daftar Kontak"
                  className="md:hidden p-1.5 -ml-1 text-muted-foreground hover:text-foreground hover:bg-muted rounded-full transition-colors shrink-0"
                >
                  <ChevronLeft className="h-5 w-5" />
                </button>

                {/* Mobile Drawer Trigger (☰) */}
                {onOpenMobileDrawer && (
                  <button
                    type="button"
                    onClick={onOpenMobileDrawer}
                    title="Buka Navigasi Utama (Tasks, Projects, Team)"
                    className="md:hidden p-1.5 text-muted-foreground hover:text-foreground hover:bg-muted rounded-md transition-colors shrink-0"
                    aria-label="Open Navigation Drawer"
                  >
                    <Menu className="h-5 w-5" />
                  </button>
                )}

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
            <span
              className={cn(
                'absolute bottom-0 right-0 h-2.5 w-2.5 rounded-full ring-2 ring-card',
                activeContact.status === 'online'
                  ? 'bg-emerald-500'
                  : activeContact.status === 'idle'
                  ? 'bg-amber-500'
                  : 'bg-zinc-400'
              )}
            />
          </div>

          <div className="min-w-0">
            <div className="flex items-center gap-2">
              <h2 className="text-sm font-bold text-foreground truncate">{activeContact.name}</h2>
              {activeContact.type === 'agent' ? (
                <div className="flex items-center gap-1.5">
                  <span className="text-[10px] font-mono font-medium px-2 py-0.5 rounded bg-primary/10 text-primary">
                    {activeContact.name.toLowerCase().includes('scrum') ? 'Agent Manager' : 'BYO Agent'}
                  </span>
                  <span className="text-[10px] font-mono font-medium px-2 py-0.5 rounded bg-emerald-500/10 text-emerald-600 dark:text-emerald-400 flex items-center gap-1">
                    <span className="h-1.5 w-1.5 rounded-full bg-emerald-500" />
                    <span>Terdaftar</span>
                  </span>
                </div>
              ) : (
                <span className="text-[10px] font-mono font-medium px-2 py-0.5 rounded bg-rose-500/10 text-rose-600 dark:text-rose-400 flex items-center gap-1">
                  <ShoppingBag className="h-3 w-3" />
                  <span>{activeContact.product ? `Dojo: ${activeContact.product}` : 'Live Chat Customer'}</span>
                </span>
              )}
            </div>
            <p className="text-[11px] text-muted-foreground flex items-center gap-1.5">
              <span className="h-1.5 w-1.5 rounded-full bg-emerald-500 animate-pulse" />
              <span>{activeContact.type === 'agent' ? 'Status koneksi tersedia di Team' : 'Online • Sesi Tamu Dojo'}</span>
            </p>
          </div>
        </div>

        <div className="flex items-center gap-2">
          <button
            onClick={() => loadConversation(activeContact.conversation_id || activeContact.id, false)}
            title="Segarkan Pesan"
            className="p-2 rounded-md text-muted-foreground hover:text-foreground hover:bg-muted transition-colors"
          >
            <RefreshCw className={cn('h-4 w-4', loading && 'animate-spin')} />
          </button>
        </div>
      </div>

      {/* ── Main Chat Stream (WhatsApp/Telegram Style) ── */}
      <div
        ref={messageListRef}
        className="inbox-message-list min-h-0 flex-1 overflow-y-auto overscroll-contain p-4 sm:p-6 space-y-4 bg-muted/20"
      >
        {/* Connection Status & Guidance Banner */}
        {activeContact.type === 'agent' && (
          <div className="mx-auto max-w-2xl my-2 p-3 rounded-xl bg-primary/5 border border-primary/20 text-foreground flex flex-col sm:flex-row items-start sm:items-center justify-between gap-2 text-xs shadow-2xs">
            <div className="flex items-center gap-2">
              <Sparkles className="h-4 w-4 text-primary shrink-0" />
              <span className="text-[11px] font-medium leading-relaxed">
                Pesan dikirim ke provider agen ini. Jika koneksi atau login bermasalah, pesan kesalahan akan ditampilkan.
              </span>
            </div>
            <span className="text-[10px] font-mono px-2 py-0.5 rounded bg-emerald-500/10 text-emerald-600 dark:text-emerald-400 font-semibold flex items-center gap-1 shrink-0 self-end sm:self-auto">
              <span className="h-1.5 w-1.5 rounded-full bg-emerald-500 animate-pulse" />
              Agen terdaftar
            </span>
          </div>
        )}

        {/* Welcome Pill */}
        <div className="flex justify-center my-2">
          <span className="text-[11px] font-medium px-3 py-1 rounded-full bg-card/80 border border-border text-muted-foreground shadow-2xs">
            Riwayat pesan tersimpan
          </span>
        </div>

        {messages.length === 0 && !loading && (
          <div className="flex flex-col items-center justify-center py-12 text-center text-muted-foreground space-y-2">
            <Bot className="h-8 w-8 text-muted-foreground/60" />
            <p className="text-xs">Belum ada riwayat pesan dengan kontak ini.</p>
            <p className="text-[11px] text-muted-foreground/80">Ketik pesan di bawah untuk memulai percakapan langsung.</p>
          </div>
        )}

        {messages.map((msg, index) => {
          // In WhatsApp/Telegram: User is right-aligned, Contact (Agent/Customer) is left-aligned
          const isUser = msg.author_type === 'user' && activeContact.type === 'agent'
          const isOperatorReply = msg.author_type === 'agent' && activeContact.type === 'customer'
          const isAgentMessage = msg.author_type === 'agent' && activeContact.type === 'agent'

          const isOutgoing = isUser || isOperatorReply

          return (
            <div
              key={msg.uuid || msg.id || index}
              className={cn('flex flex-col', isOutgoing ? 'items-end' : 'items-start')}
            >
              <div
                className={cn(
                  'relative max-w-[85%] sm:max-w-[70%] p-3.5 shadow-xs transition-all',
                  isOutgoing
                    ? 'bg-primary text-primary-foreground rounded-2xl rounded-tr-xs'
                    : 'bg-card text-card-foreground border border-border rounded-2xl rounded-tl-xs'
                )}
              >
                {/* Author info for incoming messages */}
                {!isOutgoing && (
                  <div className="flex items-center gap-1.5 mb-1 pb-1 border-b border-border/40">
                    <span className="text-xs font-bold text-primary">
                      {msg.author_name || activeContact.name}
                    </span>
                    {msg.product && (
                      <span className="text-[9px] font-mono px-1.5 py-0.2 rounded bg-rose-500/10 text-rose-600 dark:text-rose-400">
                        {msg.product}
                      </span>
                    )}
                  </div>
                )}

                {/* Message Body */}
                <div className="text-xs leading-relaxed whitespace-pre-wrap break-words font-sans">
                  {msg.content || msg.message}
                </div>

                {/* 1-Click Execution Action Card for Agent Messages */}
                {isAgentMessage && msg.action_card && (
                  <div className="mt-3 p-3 rounded-xl bg-background/90 border border-border text-foreground space-y-2.5 shadow-xs">
                    <div className="flex items-center justify-between gap-2 border-b border-border/60 pb-2">
                      <div className="flex items-center gap-1.5 font-bold text-xs">
                        <Sparkles className="h-3.5 w-3.5 text-primary" />
                        <span>Rencana Eksekusi Sprint</span>
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
                        {msg.action_card.dod_checklist.map((item, idx) => (
                          <div key={idx} className="flex items-start gap-2 text-[11px]">
                            <CheckSquare className="h-3.5 w-3.5 text-primary shrink-0 mt-0.5" />
                            <span className="text-foreground">{item.text}</span>
                          </div>
                        ))}
                      </div>
                    </div>

                    {/* 1-Click Button */}
                    <div className="pt-1">
                      {approvedTasks[msg.action_card.task_title] ? (
                        <div className="w-full py-2 px-3 rounded-lg bg-emerald-500/10 border border-emerald-500/30 text-emerald-600 dark:text-emerald-400 text-xs font-bold flex items-center justify-center gap-2">
                          <CheckCheck className="h-4 w-4" />
                          <span>Task Berhasil Dijalankan di Sprint!</span>
                        </div>
                      ) : (
                        <button
                          type="button"
                          disabled={approving === msg.action_card.task_title}
                          onClick={() => handle1ClickApprove(msg.action_card!, activeContact.conversation_id || activeContact.id)}
                          className="w-full py-2 px-3 rounded-lg bg-primary text-primary-foreground font-bold text-xs hover:bg-primary/90 active:scale-98 transition-all flex items-center justify-center gap-2 disabled:opacity-50 cursor-pointer shadow-xs"
                        >
                          <Play className="h-3.5 w-3.5 fill-current" />
                          <span>
                            {approving === msg.action_card.task_title ? 'Menjalankan Task...' : 'Mulai Kerjakan (1-Click Approve)'}
                          </span>
                        </button>
                      )}
                    </div>
                  </div>
                )}

                {/* Bubble Footer: Timestamp & Read Status */}
                <div className="flex items-center justify-end gap-1 mt-1 text-[10px] opacity-70 font-mono">
                  <span>{formatMessageTime(msg.created_at)}</span>
                  {isOutgoing && <CheckCheck className="h-3 w-3" />}
                </div>
              </div>
        {requests.filter(q => q.message_id === msg.uuid && q.state !== 'completed').map(q => (
          <div key={q.request_id} role="status" className="mt-2 max-w-[85%] sm:max-w-[70%] rounded-xl border border-border bg-card p-3 text-xs break-words">
            <p className="font-medium">{q.state === 'running' ? 'Pesan tersimpan · agen sedang memproses' : 'Balasan belum tersedia'}</p>
            <p className="mt-1 text-muted-foreground">{q.safe_error || 'Anda boleh menyegarkan atau membuka kembali Inbox. Permintaan yang sama tidak dijalankan ulang.'}</p>
            <p className="mt-1 text-muted-foreground">Pesan: {q.message_id.slice(0, 8)} · {formatMessageTime(q.created_at)}</p>
          </div>
        ))}
            </div>
          )
        })}

        {sendError && <div role="alert" className="mx-auto max-w-2xl rounded-xl border border-border bg-card p-3 text-xs">
          <p>{sendError}</p>
          {pendingRef.current && <button className="mt-2 underline" disabled={sending || retryCount >= 3} onClick={handleSendMessage}>Periksa pengiriman yang sama ({Math.max(0, 3 - retryCount)} tersisa)</button>}
          <button className="ml-3 underline" onClick={() => loadConversation(activeContact.conversation_id || activeContact.id)}>Segarkan riwayat</button>
        </div>}

        {/* Typing Indicator */}
        {sending && activeContact.type === 'agent' && (
          <div className="flex flex-col items-start animate-fade-in">
            <div className="bg-card text-card-foreground border border-border rounded-2xl rounded-tl-xs p-3 shadow-xs flex items-center gap-2.5">
              <div className="h-6 w-6 rounded-full bg-secondary flex items-center justify-center text-xs">
                {activeContact.avatar ? <span>{activeContact.avatar}</span> : <Bot className="h-3.5 w-3.5 text-primary" />}
              </div>
              <span className="text-xs text-muted-foreground font-medium">
                Mengirim pesan ke {activeContact.name}...
              </span>
              <div className="flex items-center gap-1 ml-1">
                <span className="h-1.5 w-1.5 rounded-full bg-primary/70 animate-bounce" style={{ animationDelay: '0ms' }} />
                <span className="h-1.5 w-1.5 rounded-full bg-primary/70 animate-bounce" style={{ animationDelay: '150ms' }} />
                <span className="h-1.5 w-1.5 rounded-full bg-primary/70 animate-bounce" style={{ animationDelay: '300ms' }} />
              </div>
            </div>
          </div>
        )}
      </div>

      {/* ── Modern Composer (Bottom) ── */}
      <div data-inbox-composer className="p-3 sm:p-4 pb-[max(0.75rem,env(safe-area-inset-bottom))] border-t border-border bg-card shrink-0 z-20">
        <div className="flex items-end gap-2 max-w-4xl mx-auto">
          <textarea
            ref={textareaRef}
            rows={1}
            value={inputMessage}
            onChange={(e) => {
              setInputMessage(e.target.value)
              autoResizeTextarea(e.target)
            }}
            onKeyDown={handleKeyDown}
            placeholder={
              activeContact.type === 'agent'
                ? 'Tulis pesan untuk agen... (Enter untuk kirim)'
                : `Balas pesan untuk ${activeContact.name}... (Enter untuk kirim)`
            }
            className="min-w-0 flex-1 min-h-[42px] max-h-[140px] resize-none overflow-hidden rounded-xl bg-background border border-input text-foreground px-3.5 py-2.5 text-base md:text-xs placeholder:text-muted-foreground focus:outline-none focus:ring-1 focus:ring-ring leading-relaxed"
          />

          <button
            type="button"
            onClick={handleSendMessage}
            disabled={!inputMessage.trim() || sending}
            className="h-10 w-10 rounded-xl bg-primary text-primary-foreground flex items-center justify-center hover:bg-primary/90 active:scale-95 disabled:opacity-40 disabled:pointer-events-none transition-all shadow-xs cursor-pointer shrink-0"
            title="Kirim Pesan (Enter)"
          >
            <Send className="h-4 w-4" />
          </button>
        </div>
      </div>
          </>
        )}
      </div>
    </div>
  )
}
