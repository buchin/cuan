import React, { useState, useEffect, useRef } from 'react'
import { InboxThreadItem, InboxMessage, inboxApi } from '../../api/inbox'
import { Task } from '../../api/types'
import {
  Send,
  Bot,
  User,
  CheckCircle2,
  Clock,
  Play,
  Loader2,
  MessageSquare,
  CheckSquare,
  ShieldCheck,
  FolderGit2,
} from 'lucide-react'
import { cn } from '../../lib/utils'

interface InboxViewProps {
  activeThread?: InboxThreadItem
  onThreadUpdated?: () => void
  onCreateThreadClick?: () => void
}

export function InboxView({
  activeThread,
  onThreadUpdated,
  onCreateThreadClick,
}: InboxViewProps) {
  const [messages, setMessages] = useState<InboxMessage[]>([])
  const [tasks, setTasks] = useState<Task[]>([])
  const [loading, setLoading] = useState(false)
  const [sending, setSending] = useState(false)
  const [approving, setApproving] = useState(false)
  const [inputContent, setInputContent] = useState('')
  const messagesEndRef = useRef<HTMLDivElement>(null)

  const scrollToBottom = () => {
    messagesEndRef.current?.scrollIntoView({ behavior: 'smooth' })
  }

  // Load thread detail whenever activeThread changes
  useEffect(() => {
    if (!activeThread?.id) {
      setMessages([])
      setTasks([])
      return
    }

    let isMounted = true
    setLoading(true)

    inboxApi
      .get(activeThread.id)
      .then((detail) => {
        if (isMounted) {
          setMessages(detail.messages || [])
          setTasks(detail.tasks || [])
        }
      })
      .catch((err) => {
        console.error('Failed to load thread details:', err)
      })
      .finally(() => {
        if (isMounted) setLoading(false)
      })

    return () => {
      isMounted = false
    }
  }, [activeThread?.id])

  useEffect(() => {
    scrollToBottom()
  }, [messages])

  // Send message
  const handleSendMessage = async (e?: React.FormEvent) => {
    if (e) e.preventDefault()
    if (!inputContent.trim() || !activeThread?.id || sending) return

    const content = inputContent.trim()
    setInputContent('')
    setSending(true)

    try {
      const newMsg = await inboxApi.createMessage(activeThread.id, {
        content,
        author_type: 'user',
        author_name: 'Product Owner',
      })
      setMessages((prev) => [...prev, newMsg])
      if (onThreadUpdated) onThreadUpdated()
    } catch (err) {
      console.error('Failed to send message:', err)
      setInputContent(content)
    } finally {
      setSending(false)
    }
  }

  // 1-Click Approve Sprint Task
  const handleApproveSprint = async () => {
    if (!activeThread?.id || approving) return
    setApproving(true)

    try {
      const res = await inboxApi.approve(activeThread.id)
      if (res.message) {
        setMessages((prev) => [...prev, res.message])
      }
      if (res.task) {
        setTasks((prev) => [...prev, res.task])
      }
      if (onThreadUpdated) onThreadUpdated()
    } catch (err) {
      console.error('Failed to approve sprint task:', err)
    } finally {
      setApproving(false)
    }
  }

  const handleKeyDown = (e: React.KeyboardEvent<HTMLTextAreaElement>) => {
    if (e.key === 'Enter' && !e.shiftKey) {
      e.preventDefault()
      handleSendMessage()
    }
  }

  // Empty State: No thread selected
  if (!activeThread) {
    return (
      <div className="h-full flex flex-col items-center justify-center text-center p-8">
        <div className="h-12 w-12 rounded-full bg-primary/10 text-primary flex items-center justify-center mb-4">
          <MessageSquare className="h-6 w-6" />
        </div>
        <h3 className="text-base font-bold text-foreground">
          Agentic Scrum Command Center
        </h3>
        <p className="text-xs text-muted-foreground max-w-md mt-1 mb-6 leading-relaxed">
          Pilih thread percakapan di sub-panel kiri atau buat pesan baru. Sampaikan masalah
          atau ide fitur secara natural ke Scrum Master untuk dirumuskan menjadi Definition of Done (DoD).
        </p>
        {onCreateThreadClick && (
          <button
            onClick={onCreateThreadClick}
            className="flex items-center gap-2 px-4 py-2 text-xs font-semibold bg-primary text-primary-foreground hover:bg-primary/90 transition-colors rounded-md shadow-xs"
          >
            <Bot className="h-4 w-4" />
            <span>Mulai Diskusi Baru</span>
          </button>
        )}
      </div>
    )
  }

  const hasRunningTask = tasks.some((t) => t.status === 'doing' || t.status === 'review')
  const hasCompletedTask = tasks.some((t) => t.status === 'done')

  return (
    <div className="h-full flex flex-col bg-background overflow-hidden -m-6">
      {/* ── Thread Header ── */}
      <div className="h-16 px-6 border-b border-border bg-card/60 flex items-center justify-between shrink-0">
        <div className="flex items-center gap-3 min-w-0">
          <div className="h-8 w-8 rounded-md bg-primary/10 text-primary flex items-center justify-center shrink-0">
            <Bot className="h-4 w-4" />
          </div>
          <div className="min-w-0">
            <div className="flex items-center gap-2">
              <h2 className="text-sm font-bold text-foreground truncate">
                {activeThread.title}
              </h2>
              <span className="text-[10px] font-mono px-2 py-0.5 rounded bg-muted text-muted-foreground shrink-0 flex items-center gap-1">
                <FolderGit2 className="h-3 w-3" />
                {activeThread.project_name}
              </span>
            </div>
            <p className="text-[11px] text-muted-foreground truncate mt-0.5">
              Topic: {activeThread.topic}
            </p>
          </div>
        </div>

        {/* 1-Click Action Header */}
        <div className="flex items-center gap-3 shrink-0">
          {hasRunningTask ? (
            <span className="flex items-center gap-1.5 px-3 py-1.5 rounded-md bg-blue-500/10 text-blue-600 dark:text-blue-400 text-xs font-mono font-medium border border-blue-500/20">
              <Clock className="h-3.5 w-3.5 animate-spin" />
              <span>Sprint Running</span>
            </span>
          ) : hasCompletedTask ? (
            <span className="flex items-center gap-1.5 px-3 py-1.5 rounded-md bg-emerald-500/10 text-emerald-600 dark:text-emerald-400 text-xs font-mono font-medium border border-emerald-500/20">
              <CheckCircle2 className="h-3.5 w-3.5" />
              <span>DoD Completed</span>
            </span>
          ) : (
            <button
              onClick={handleApproveSprint}
              disabled={approving}
              className="flex items-center gap-1.5 px-3.5 py-1.5 text-xs font-semibold bg-emerald-600 hover:bg-emerald-500 text-white rounded-md transition-colors shadow-xs disabled:opacity-50"
              title="Setujui DoD dan tugaskan worker agent otomatis"
            >
              {approving ? (
                <>
                  <Loader2 className="h-3.5 w-3.5 animate-spin" />
                  <span>Mendelegasikan...</span>
                </>
              ) : (
                <>
                  <Play className="h-3.5 w-3.5 fill-current" />
                  <span>Setujui & Jalankan Sprint</span>
                </>
              )}
            </button>
          )}
        </div>
      </div>

      {/* ── Chat Messages Stream ── */}
      <div className="flex-1 overflow-y-auto p-6 space-y-4">
        {loading ? (
          <div className="flex items-center justify-center h-full text-muted-foreground text-xs font-mono">
            <Loader2 className="h-5 w-5 animate-spin mr-2" />
            Memuat histori percakapan...
          </div>
        ) : messages.length === 0 ? (
          <div className="text-center text-muted-foreground text-xs italic py-8">
            Belum ada pesan dalam thread ini.
          </div>
        ) : (
          messages.map((msg) => {
            const isUser = msg.author_type === 'user'
            const isAgent = msg.author_type === 'agent'

            return (
              <div
                key={msg.id}
                className={cn(
                  'flex gap-3 max-w-3xl',
                  isUser ? 'ml-auto flex-row-reverse' : 'mr-auto'
                )}
              >
                {/* Avatar */}
                <div
                  className={cn(
                    'h-7 w-7 rounded-full flex items-center justify-center shrink-0 text-xs font-bold font-mono',
                    isUser
                      ? 'bg-primary text-primary-foreground'
                      : 'bg-indigo-600 text-white'
                  )}
                >
                  {isUser ? <User className="h-3.5 w-3.5" /> : <Bot className="h-3.5 w-3.5" />}
                </div>

                {/* Message Bubble */}
                <div
                  className={cn(
                    'rounded-lg p-4 text-xs shadow-xs leading-relaxed',
                    isUser
                      ? 'bg-primary text-primary-foreground max-w-xl'
                      : 'bg-card border border-border text-card-foreground w-full max-w-2xl'
                  )}
                >
                  <div className="flex items-center justify-between gap-3 mb-1.5 pb-1 border-b border-current/10">
                    <span className="font-semibold text-[11px] flex items-center gap-1.5">
                      {msg.author_name || (isUser ? 'Product Owner' : 'Scrum Master')}
                      {isAgent && (
                        <span className="text-[9px] font-mono px-1 py-0.2 rounded bg-indigo-500/15 text-indigo-400 border border-indigo-500/30">
                          AI MANAGER
                        </span>
                      )}
                    </span>
                    <span className="text-[10px] opacity-70 font-mono">
                      {new Date(msg.created_at).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })}
                    </span>
                  </div>

                  <div className="whitespace-pre-wrap font-mono text-[11px]">
                    {msg.content}
                  </div>
                </div>
              </div>
            )
          })
        )}
        <div ref={messagesEndRef} />
      </div>

      {/* ── Active Task Banner (if any) ── */}
      {tasks.length > 0 && (
        <div className="px-6 py-2.5 bg-muted/40 border-t border-border flex items-center justify-between text-xs">
          <div className="flex items-center gap-2">
            <CheckSquare className="h-3.5 w-3.5 text-primary" />
            <span className="font-semibold text-foreground">
              Terkait Task: {tasks[0].title}
            </span>
            <span className="text-[10px] font-mono px-1.5 py-0.5 rounded bg-primary/10 text-primary">
              Status: {tasks[0].status.toUpperCase()}
            </span>
          </div>
          {tasks[0].assignee && (
            <span className="text-[11px] text-muted-foreground font-mono flex items-center gap-1">
              <ShieldCheck className="h-3.5 w-3.5 text-emerald-500" />
              Worker: {tasks[0].assignee}
            </span>
          )}
        </div>
      )}

      {/* ── Composer Form ── */}
      <div className="p-4 bg-card border-t border-border shrink-0">
        <form onSubmit={handleSendMessage} className="relative flex items-center gap-2">
          <textarea
            rows={2}
            value={inputContent}
            onChange={(e) => setInputContent(e.target.value)}
            onKeyDown={handleKeyDown}
            placeholder="Sampaikan ide, klarifikasi, atau instruksi ke Scrum Master... (Enter untuk kirim)"
            className="w-full pl-3 pr-12 py-2 text-xs bg-background border border-input rounded-md text-foreground placeholder:text-muted-foreground focus:outline-hidden focus:ring-1 focus:ring-ring resize-none font-mono"
          />
          <button
            type="submit"
            disabled={!inputContent.trim() || sending}
            className="absolute right-2.5 bottom-3.5 p-2 rounded-md bg-primary text-primary-foreground hover:bg-primary/90 transition-colors disabled:opacity-40"
            title="Kirim pesan (Enter)"
          >
            {sending ? (
              <Loader2 className="h-3.5 w-3.5 animate-spin" />
            ) : (
              <Send className="h-3.5 w-3.5" />
            )}
          </button>
        </form>
      </div>
    </div>
  )
}
