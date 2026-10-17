import { useState, useEffect, useRef } from 'react'
import { Button } from '../ui/button'
import { Input } from '../ui/input'
import { Badge } from '../ui/badge'
import {
  MessageSquare,
  Send,
  Plus,
  RefreshCw,
  Sparkles,
  User,
  Bot,
  Shield,
  Trash2,
  Reply,
  Copy,
  Check,
  Kanban,
  CheckCircle2,
  AlertCircle,
  X,
  Layers,
  Hash
} from 'lucide-react'
import { ProjectRoomDTO, ProjectMessageDTO, ProjectTaskDTO } from './types'
import { DistillTaskModal } from './DistillTaskModal'

interface ProjectDiscussionsViewProps {
  projectId?: string
  projects?: Array<{ id: string; name: string }>
  onOpenTaskBoard?: (projectId: string) => void
  onProjectChange?: (projectId: string) => void
}

// Markdown helper to render message content with code blocks, bold, lists, and checklists
function MarkdownContent({ content }: { content: string }) {
  const [copiedIndex, setCopiedIndex] = useState<number | null>(null)

  const handleCopyCode = (code: string, idx: number) => {
    navigator.clipboard.writeText(code)
    setCopiedIndex(idx)
    setTimeout(() => setCopiedIndex(null), 2000)
  }

  // Split into lines/blocks
  const lines = content.split('\n')
  const blocks: Array<{ type: 'p' | 'h1' | 'h2' | 'h3' | 'code' | 'ul' | 'quote'; text: string; lang?: string }> = []
  
  let inCodeBlock = false
  let codeBuffer: string[] = []
  let codeLang = ''

  for (let i = 0; i < lines.length; i++) {
    const line = lines[i]

    if (line.trim().startsWith('```')) {
      if (inCodeBlock) {
        blocks.push({ type: 'code', text: codeBuffer.join('\n'), lang: codeLang })
        codeBuffer = []
        codeLang = ''
        inCodeBlock = false
      } else {
        inCodeBlock = true
        codeLang = line.trim().slice(3).trim()
      }
      continue
    }

    if (inCodeBlock) {
      codeBuffer.push(line)
      continue
    }

    if (line.startsWith('# ')) {
      blocks.push({ type: 'h1', text: line.slice(2) })
    } else if (line.startsWith('## ')) {
      blocks.push({ type: 'h2', text: line.slice(3) })
    } else if (line.startsWith('### ')) {
      blocks.push({ type: 'h3', text: line.slice(4) })
    } else if (line.startsWith('> ')) {
      blocks.push({ type: 'quote', text: line.slice(2) })
    } else if (line.trim().startsWith('- ') || line.trim().startsWith('* ')) {
      blocks.push({ type: 'ul', text: line.trim().slice(2) })
    } else if (line.trim() !== '') {
      blocks.push({ type: 'p', text: line })
    }
  }

  if (inCodeBlock && codeBuffer.length > 0) {
    blocks.push({ type: 'code', text: codeBuffer.join('\n'), lang: codeLang })
  }

  // Render inline formatting: **bold**, `code`, - [ ] checklist
  const renderInline = (str: string) => {
    // Checkbox checklist format: [ ] or [x]
    if (str.startsWith('[ ] ')) {
      return (
        <span className="flex items-center gap-1.5 text-slate-700 dark:text-slate-300">
          <input type="checkbox" disabled className="rounded text-emerald-600 h-3.5 w-3.5" />
          <span>{renderInline(str.slice(4))}</span>
        </span>
      )
    }
    if (str.startsWith('[x] ') || str.startsWith('[X] ')) {
      return (
        <span className="flex items-center gap-1.5 text-slate-500 line-through">
          <input type="checkbox" checked disabled className="rounded text-emerald-600 h-3.5 w-3.5" />
          <span>{renderInline(str.slice(4))}</span>
        </span>
      )
    }

    // Split on inline code `...` and bold **...**
    const parts = str.split(/(`[^`]+`|\*\*[^*]+\*\*)/g)
    return parts.map((part, pIdx) => {
      if (part.startsWith('`') && part.endsWith('`')) {
        return (
          <code
            key={pIdx}
            className="px-1.5 py-0.5 rounded bg-slate-200 dark:bg-slate-800 text-emerald-600 dark:text-emerald-400 font-mono text-[11px]"
          >
            {part.slice(1, -1)}
          </code>
        )
      }
      if (part.startsWith('**') && part.endsWith('**')) {
        return (
          <strong key={pIdx} className="font-bold text-slate-900 dark:text-white">
            {part.slice(2, -2)}
          </strong>
        )
      }
      return <span key={pIdx}>{part}</span>
    })
  }

  return (
    <div className="space-y-1.5 text-xs leading-relaxed break-words">
      {blocks.map((block, idx) => {
        if (block.type === 'h1') {
          return (
            <h1 key={idx} className="text-sm font-bold text-slate-900 dark:text-white mt-1 mb-0.5">
              {renderInline(block.text)}
            </h1>
          )
        }
        if (block.type === 'h2') {
          return (
            <h2 key={idx} className="text-xs font-bold text-slate-800 dark:text-slate-100 mt-1 mb-0.5">
              {renderInline(block.text)}
            </h2>
          )
        }
        if (block.type === 'h3') {
          return (
            <h3 key={idx} className="text-xs font-semibold text-slate-800 dark:text-slate-200 mt-1">
              {renderInline(block.text)}
            </h3>
          )
        }
        if (block.type === 'code') {
          return (
            <div key={idx} className="my-2 rounded-lg bg-slate-950 border border-slate-800 overflow-hidden shadow-xs">
              <div className="flex items-center justify-between px-3 py-1 bg-slate-900/80 border-b border-slate-800/80 text-[10px] text-slate-400 font-mono">
                <span>{block.lang || 'code'}</span>
                <button
                  type="button"
                  onClick={() => handleCopyCode(block.text, idx)}
                  className="flex items-center gap-1 hover:text-white transition-colors"
                >
                  {copiedIndex === idx ? (
                    <>
                      <Check className="h-3 w-3 text-emerald-400" /> Tersalin
                    </>
                  ) : (
                    <>
                      <Copy className="h-3 w-3" /> Salin
                    </>
                  )}
                </button>
              </div>
              <pre className="p-3 font-mono text-[11px] text-emerald-300 overflow-x-auto whitespace-pre">
                {block.text}
              </pre>
            </div>
          )
        }
        if (block.type === 'quote') {
          return (
            <blockquote
              key={idx}
              className="border-l-2 border-emerald-500 pl-2.5 py-0.5 text-slate-600 dark:text-slate-400 italic bg-emerald-500/5 rounded-r"
            >
              {renderInline(block.text)}
            </blockquote>
          )
        }
        if (block.type === 'ul') {
          return (
            <div key={idx} className="flex items-start gap-1.5 pl-2">
              <span className="text-emerald-500 mt-0.5">•</span>
              <div className="flex-1">{renderInline(block.text)}</div>
            </div>
          )
        }
        return <p key={idx}>{renderInline(block.text)}</p>
      })}
    </div>
  )
}

export function ProjectDiscussionsView({
  projectId: propProjectId,
  projects: propProjects = [],
  onOpenTaskBoard,
  onProjectChange,
}: ProjectDiscussionsViewProps) {
  // Project state
  const [internalProjects, setInternalProjects] = useState<Array<{ id: string; name: string }>>(propProjects)
  const effectiveProjects = propProjects.length > 0 ? propProjects : internalProjects
  const [selectedProjectId, setSelectedProjectId] = useState<string>(
    propProjectId || (effectiveProjects.length > 0 ? effectiveProjects[0].id : 'default')
  )

  useEffect(() => {
    if (propProjectId && propProjectId !== selectedProjectId) {
      setSelectedProjectId(propProjectId)
    }
  }, [propProjectId])

  // Fetch available projects if none provided
  useEffect(() => {
    if (effectiveProjects.length === 0) {
      const loadProjects = async () => {
        try {
          const res = await fetch('/api/v1/projects')
          if (res.ok) {
            const data = await res.json()
            if (data.projects && data.projects.length > 0) {
              setInternalProjects(data.projects)
              if (!propProjectId) {
                setSelectedProjectId(data.projects[0].id)
              }
            }
          }
        } catch {
          // If v1 fails, check /api/deployments
          try {
            const res2 = await fetch('/api/deployments')
            if (res2.ok) {
              const data2 = await res2.json()
              if (data2.deployments && data2.deployments.length > 0) {
                const list = data2.deployments.map((d: any) => ({ id: d.id, name: d.project_name }))
                setInternalProjects(list)
                if (!propProjectId) {
                  setSelectedProjectId(list[0].id)
                }
              }
            }
          } catch {}
        }
      }
      loadProjects()
    }
  }, [propProjectId])

  // Rooms state
  const [rooms, setRooms] = useState<ProjectRoomDTO[]>([])
  const [activeRoomId, setActiveRoomId] = useState<string | null>(null)
  const [roomsLoading, setRoomsLoading] = useState(false)
  const [isNewRoomModalOpen, setIsNewRoomModalOpen] = useState(false)
  const [newRoomTitle, setNewRoomTitle] = useState('')
  const [newRoomTopic, setNewRoomTopic] = useState('')
  const [creatingRoom, setCreatingRoom] = useState(false)

  // Messages state
  const [messages, setMessages] = useState<ProjectMessageDTO[]>([])
  const [messagesLoading, setMessagesLoading] = useState(false)
  const [inputContent, setInputContent] = useState('')
  const [authorType, setAuthorType] = useState<'user' | 'agent' | 'system'>('user')
  const [authorName, setAuthorName] = useState('Buchin (PO)')
  const [sendingMessage, setSendingMessage] = useState(false)
  const [replyToMessage, setReplyToMessage] = useState<ProjectMessageDTO | null>(null)

  // SSE connection status: 'connected' | 'connecting' | 'disconnected'
  const [sseStatus, setSseStatus] = useState<'connected' | 'connecting' | 'disconnected'>('disconnected')
  const eventSourceRef = useRef<EventSource | null>(null)

  // Distill modal state
  const [isDistillModalOpen, setIsDistillModalOpen] = useState(false)
  const [distillTargetMessageIds, setDistillTargetMessageIds] = useState<string[]>([])
  const [distillTargetContent, setDistillTargetContent] = useState<string | undefined>(undefined)

  // Feedback notifications
  const [notification, setNotification] = useState<{ type: 'success' | 'error'; message: string } | null>(null)

  const feedBottomRef = useRef<HTMLDivElement>(null)

  // Fetch Rooms for selected Project
  const fetchRooms = async (projId: string) => {
    setRoomsLoading(true)
    try {
      const res = await fetch(`/api/projects/${projId}/rooms`)
      if (res.ok) {
        const data = await res.json()
        const roomList: ProjectRoomDTO[] = data.rooms || []
        setRooms(roomList)
        if (roomList.length > 0) {
          // If no active room or active room belongs to different project, select first
          if (!activeRoomId || !roomList.some((r) => r.id === activeRoomId)) {
            setActiveRoomId(roomList[0].id)
          }
        } else {
          setActiveRoomId(null)
          setMessages([])
        }
      }
    } catch {
      // ignore transient error
    } finally {
      setRoomsLoading(false)
    }
  }

  useEffect(() => {
    if (selectedProjectId) {
      fetchRooms(selectedProjectId)
      onProjectChange?.(selectedProjectId)
    }
  }, [selectedProjectId])

  // Fetch Messages for active room
  const fetchMessages = async (roomId: string) => {
    setMessagesLoading(true)
    try {
      const res = await fetch(`/api/projects/rooms/${roomId}/messages`)
      if (res.ok) {
        const data = await res.json()
        setMessages(data.messages || [])
      }
    } catch {
      // ignore
    } finally {
      setMessagesLoading(false)
    }
  }

  // Connect SSE real-time listener for active room
  useEffect(() => {
    if (!activeRoomId) {
      if (eventSourceRef.current) {
        eventSourceRef.current.close()
        eventSourceRef.current = null
      }
      setSseStatus('disconnected')
      return
    }

    fetchMessages(activeRoomId)

    // Close any previous SSE connection
    if (eventSourceRef.current) {
      eventSourceRef.current.close()
      eventSourceRef.current = null
    }

    setSseStatus('connecting')
    const sseUrl = `/api/projects/rooms/${activeRoomId}/events`
    const es = new EventSource(sseUrl)
    eventSourceRef.current = es

    es.addEventListener('connected', () => {
      setSseStatus('connected')
    })

    es.addEventListener('message.created', (e) => {
      try {
        const newMsg: ProjectMessageDTO = JSON.parse(e.data)
        setMessages((prev) => {
          if (prev.some((m) => m.id === newMsg.id)) return prev
          return [...prev, newMsg]
        })
      } catch {}
    })

    es.addEventListener('message.deleted', (e) => {
      try {
        const deletedData = JSON.parse(e.data)
        setMessages((prev) => prev.filter((m) => m.id !== deletedData.id))
      } catch {}
    })

    es.addEventListener('task.distilled', (e) => {
      try {
        const task: ProjectTaskDTO = JSON.parse(e.data)
        setNotification({
          type: 'success',
          message: `✨ Task disuling otomatis: "${task.title}" (${task.dod_checklist?.length || 0} DoD criteria)`,
        })
      } catch {}
    })

    es.addEventListener('task.created', (e) => {
      try {
        const task: ProjectTaskDTO = JSON.parse(e.data)
        setNotification({
          type: 'success',
          message: `📌 Task baru dibuat: "${task.title}" [${task.priority.toUpperCase()}]`,
        })
      } catch {}
    })

    es.onopen = () => {
      setSseStatus('connected')
    }

    es.onerror = () => {
      // EventSource automatically retries
      setSseStatus('connecting')
    }

    return () => {
      es.close()
      eventSourceRef.current = null
      setSseStatus('disconnected')
    }
  }, [activeRoomId])

  // Scroll to bottom on new messages
  useEffect(() => {
    feedBottomRef.current?.scrollIntoView({ behavior: 'smooth' })
  }, [messages])

  // Create New Room
  const handleCreateRoom = async (e: React.FormEvent) => {
    e.preventDefault()
    if (!newRoomTitle.trim()) return

    setCreatingRoom(true)
    try {
      const res = await fetch(`/api/projects/${selectedProjectId}/rooms`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          title: newRoomTitle.trim(),
          topic: newRoomTopic.trim(),
        }),
      })

      if (!res.ok) {
        throw new Error('Gagal membuat room diskusi baru')
      }

      const data = await res.json()
      setRooms((prev) => [...prev, data.room])
      setActiveRoomId(data.room.id)
      setIsNewRoomModalOpen(false)
      setNewRoomTitle('')
      setNewRoomTopic('')
      setNotification({ type: 'success', message: `Room "${data.room.title}" berhasil dibuat!` })
    } catch (err: any) {
      setNotification({ type: 'error', message: err.message || 'Gagal membuat room' })
    } finally {
      setCreatingRoom(false)
    }
  }

  // Post Message
  const handleSendMessage = async (e?: React.FormEvent) => {
    e?.preventDefault()
    if (!activeRoomId || !inputContent.trim()) return

    setSendingMessage(true)
    try {
      const payload: Record<string, any> = {
        author_type: authorType,
        author_id: authorType === 'user' ? 'user-buchin' : 'agent-agy',
        author_name: authorName.trim() || (authorType === 'agent' ? 'agy (Agent)' : 'Buchin'),
        content: inputContent.trim(),
      }
      if (replyToMessage) {
        payload.reply_to_id = replyToMessage.id
      }

      const res = await fetch(`/api/projects/rooms/${activeRoomId}/messages`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(payload),
      })

      if (!res.ok) {
        throw new Error('Gagal mengirim pesan')
      }

      const data = await res.json()
      setMessages((prev) => {
        if (prev.some((m) => m.id === data.message.id)) return prev
        return [...prev, data.message]
      })
      setInputContent('')
      setReplyToMessage(null)
    } catch (err: any) {
      setNotification({ type: 'error', message: err.message || 'Gagal mengirim pesan' })
    } finally {
      setSendingMessage(false)
    }
  }

  // Delete Message
  const handleDeleteMessage = async (msgId: string) => {
    try {
      const res = await fetch(`/api/messages/${msgId}`, { method: 'DELETE' })
      if (res.ok) {
        setMessages((prev) => prev.filter((m) => m.id !== msgId))
      }
    } catch {}
  }

  // Trigger Distill from Message
  const handleDistillFromMessage = (msg: ProjectMessageDTO) => {
    setDistillTargetMessageIds([msg.id])
    setDistillTargetContent(msg.content)
    setIsDistillModalOpen(true)
  }

  // Trigger Distill Full Room
  const handleDistillFullRoom = () => {
    setDistillTargetMessageIds([])
    setDistillTargetContent(undefined)
    setIsDistillModalOpen(true)
  }

  const activeRoom = rooms.find((r) => r.id === activeRoomId)

  // Map messages by ID for quick reply resolution
  const messageMap = new Map(messages.map((m) => [m.id, m]))

  return (
    <div className="space-y-6 animate-in fade-in duration-300">
      {/* Toast Notification Banner */}
      {notification && (
        <div
          className={`p-3.5 rounded-xl flex items-center justify-between text-xs font-semibold shadow-md animate-in slide-in-from-top duration-200 ${
            notification.type === 'success'
              ? 'bg-emerald-500 text-white shadow-emerald-500/20'
              : 'bg-red-500 text-white shadow-red-500/20'
          }`}
        >
          <div className="flex items-center gap-2">
            {notification.type === 'success' ? (
              <CheckCircle2 className="h-4 w-4 shrink-0" />
            ) : (
              <AlertCircle className="h-4 w-4 shrink-0" />
            )}
            <span>{notification.message}</span>
          </div>
          <button onClick={() => setNotification(null)} className="opacity-80 hover:opacity-100">
            <X className="h-4 w-4" />
          </button>
        </div>
      )}

      {/* Top Bar: Project Selector & Quick Stats */}
      <div className="flex flex-col sm:flex-row items-start sm:items-center justify-between gap-3 p-4 bg-white dark:bg-slate-900 rounded-2xl border border-slate-200 dark:border-slate-800 shadow-xs">
        <div className="flex items-center gap-3">
          <div className="h-10 w-10 rounded-xl bg-emerald-500/10 border border-emerald-500/20 text-emerald-600 dark:text-emerald-400 flex items-center justify-center shrink-0">
            <MessageSquare className="h-5 w-5" />
          </div>
          <div>
            <div className="flex items-center gap-2">
              <h1 className="text-base sm:text-lg font-bold text-slate-900 dark:text-white tracking-tight">
                Project Discussion Rooms
              </h1>
              <Badge variant="outline" className="border-emerald-500/40 text-emerald-600 dark:text-emerald-400 text-[10px] font-mono">
                Real-Time SSE
              </Badge>
            </div>
            <p className="text-xs text-slate-400">
              Ruang percakapan terstruktur multi-aktor (User, AI Agent, System) dengan ekstraksi draf task terintegrasi.
            </p>
          </div>
        </div>

        <div className="flex items-center gap-2.5 w-full sm:w-auto justify-between sm:justify-end">
          {/* Project Selector */}
          <div className="flex items-center gap-1.5">
            <Layers className="h-3.5 w-3.5 text-slate-400 shrink-0" />
            <select
              value={selectedProjectId}
              onChange={(e) => setSelectedProjectId(e.target.value)}
              className="text-xs py-1.5 px-2.5 rounded-lg border border-slate-200 dark:border-slate-800 bg-slate-50 dark:bg-slate-950 text-slate-800 dark:text-slate-200 font-semibold focus:outline-none focus:ring-1 focus:ring-emerald-500 max-w-[180px] truncate"
            >
              {effectiveProjects.length > 0 ? (
                effectiveProjects.map((p) => (
                  <option key={p.id} value={p.id}>
                    {p.name}
                  </option>
                ))
              ) : (
                <option value="default">Default Project</option>
              )}
            </select>
          </div>

          {/* Jump to Tasks Board Button */}
          {onOpenTaskBoard && (
            <Button
              variant="outline"
              size="sm"
              onClick={() => onOpenTaskBoard(selectedProjectId)}
              className="h-8 text-xs font-semibold gap-1.5 border-slate-300 dark:border-slate-700"
              title="Buka Kanban Board Task"
            >
              <Kanban className="h-3.5 w-3.5 text-indigo-500" />
              <span className="hidden sm:inline">Tasks Board</span>
            </Button>
          )}

          {/* New Room Button */}
          <Button
            size="sm"
            onClick={() => setIsNewRoomModalOpen(true)}
            className="h-8 text-xs font-semibold bg-emerald-600 hover:bg-emerald-500 text-white gap-1 shadow-xs"
          >
            <Plus className="h-3.5 w-3.5" />
            <span>Room Baru</span>
          </Button>
        </div>
      </div>

      {/* Main Discussion Layout: Sidebar Rooms + Chat Area */}
      <div className="grid grid-cols-1 md:grid-cols-4 gap-4 items-start">
        {/* Left Column: Rooms List */}
        <div className="md:col-span-1 bg-white dark:bg-slate-900 rounded-2xl border border-slate-200 dark:border-slate-800 p-3 space-y-2.5 shadow-xs">
          <div className="flex items-center justify-between px-1">
            <span className="text-[11px] font-bold text-slate-400 uppercase tracking-wider">
              Rooms ({rooms.length})
            </span>
            <button
              onClick={() => fetchRooms(selectedProjectId)}
              className="text-slate-400 hover:text-slate-600 dark:hover:text-slate-200 p-1 rounded"
              title="Refresh daftar room"
            >
              <RefreshCw className={`h-3 w-3 ${roomsLoading ? 'animate-spin' : ''}`} />
            </button>
          </div>

          <div className="space-y-1 max-h-[500px] overflow-y-auto">
            {rooms.length === 0 ? (
              <div className="p-4 text-center text-xs text-slate-400">
                {roomsLoading ? (
                  <span className="flex items-center justify-center gap-1.5">
                    <RefreshCw className="h-3 w-3 animate-spin" /> Memuat rooms...
                  </span>
                ) : (
                  <>
                    <p>Belum ada room diskusi.</p>
                    <button
                      onClick={() => setIsNewRoomModalOpen(true)}
                      className="text-emerald-500 hover:underline mt-1 font-semibold block mx-auto text-[11px]"
                    >
                      + Buat Room Pertama
                    </button>
                  </>
                )}
              </div>
            ) : (
              rooms.map((room) => {
                const isActive = room.id === activeRoomId
                return (
                  <button
                    key={room.id}
                    onClick={() => setActiveRoomId(room.id)}
                    className={`w-full text-left p-2.5 rounded-xl text-xs transition-all flex items-start gap-2 ${
                      isActive
                        ? 'bg-emerald-50 dark:bg-emerald-950/40 border border-emerald-300 dark:border-emerald-800/80 text-emerald-900 dark:text-emerald-200 font-semibold shadow-xs'
                        : 'text-slate-600 dark:text-slate-400 hover:bg-slate-50 dark:hover:bg-slate-800/50'
                    }`}
                  >
                    <Hash className={`h-3.5 w-3.5 shrink-0 mt-0.5 ${isActive ? 'text-emerald-600 dark:text-emerald-400' : 'text-slate-400'}`} />
                    <div className="min-w-0 flex-1">
                      <p className="truncate font-medium">{room.title}</p>
                      {room.topic && (
                        <p className="text-[10px] text-slate-400 truncate font-normal mt-0.5">
                          {room.topic}
                        </p>
                      )}
                    </div>
                  </button>
                )
              })
            )}
          </div>
        </div>

        {/* Right Column: Active Room Chat Feed & Composer */}
        <div className="md:col-span-3 bg-white dark:bg-slate-900 rounded-2xl border border-slate-200 dark:border-slate-800 shadow-xs flex flex-col h-[650px] overflow-hidden">
          {/* Room Header */}
          {activeRoom ? (
            <div className="p-4 border-b border-slate-100 dark:border-slate-800 flex items-center justify-between bg-slate-50/50 dark:bg-slate-900/50">
              <div className="space-y-0.5 min-w-0 pr-3">
                <div className="flex items-center gap-2">
                  <h2 className="text-sm font-bold text-slate-900 dark:text-white truncate">
                    #{activeRoom.title}
                  </h2>
                  {/* SSE Status Indicator */}
                  {sseStatus === 'connected' ? (
                    <Badge variant="outline" className="border-emerald-500/40 text-emerald-600 dark:text-emerald-400 text-[10px] py-0 gap-1 font-mono">
                      <span className="h-1.5 w-1.5 rounded-full bg-emerald-500 animate-pulse" />
                      Live Stream
                    </Badge>
                  ) : sseStatus === 'connecting' ? (
                    <Badge variant="outline" className="border-amber-400 text-amber-600 dark:text-amber-400 text-[10px] py-0 gap-1 font-mono">
                      <RefreshCw className="h-2.5 w-2.5 animate-spin" />
                      Connecting...
                    </Badge>
                  ) : (
                    <Badge variant="outline" className="text-slate-400 text-[10px] py-0 font-mono">
                      Offline
                    </Badge>
                  )}
                </div>
                {activeRoom.topic && (
                  <p className="text-xs text-slate-400 truncate">{activeRoom.topic}</p>
                )}
              </div>

              {/* Room Action: Distill to Task Button */}
              <div className="flex items-center gap-2 shrink-0">
                <Button
                  size="sm"
                  onClick={handleDistillFullRoom}
                  className="h-8 text-xs font-semibold bg-gradient-to-r from-emerald-600 to-teal-600 hover:from-emerald-500 hover:to-teal-500 text-white gap-1.5 shadow-sm shadow-emerald-600/20"
                  title="Ekstrak obrolan ruangan ini menjadi task backlog ber-DoD"
                >
                  <Sparkles className="h-3.5 w-3.5" />
                  <span>Distill to Task</span>
                </Button>
              </div>
            </div>
          ) : (
            <div className="p-4 border-b border-slate-100 dark:border-slate-800 text-xs text-slate-400">
              Pilih salah satu room di panel kiri untuk membuka obrolan.
            </div>
          )}

          {/* Chat Messages Feed */}
          <div className="flex-1 overflow-y-auto p-4 space-y-4 bg-slate-50/30 dark:bg-slate-950/20">
            {messagesLoading ? (
              <div className="h-full flex items-center justify-center text-xs text-slate-400 gap-2">
                <RefreshCw className="h-3.5 w-3.5 animate-spin text-emerald-500" />
                <span>Memuat riwayat pesan...</span>
              </div>
            ) : messages.length === 0 ? (
              <div className="h-full flex flex-col items-center justify-center text-center p-6 text-slate-400 space-y-2">
                <div className="h-10 w-10 rounded-xl bg-slate-100 dark:bg-slate-800 flex items-center justify-center text-slate-400">
                  <MessageSquare className="h-5 w-5" />
                </div>
                <p className="text-xs font-medium">Belum ada percakapan di room ini.</p>
                <p className="text-[11px] text-slate-400 max-w-sm">
                  Kirim pesan pertama atau diskusikan perencanaan sprint dengan tim developer dan AI agent!
                </p>
              </div>
            ) : (
              messages.map((msg) => {
                const isAgent = msg.author_type === 'agent'
                const isSystem = msg.author_type === 'system'
                const repliedMessage = msg.reply_to_id ? messageMap.get(msg.reply_to_id) : null

                return (
                  <div
                    key={msg.id}
                    className={`flex items-start gap-3 group animate-in fade-in duration-200 ${
                      isSystem ? 'justify-center my-2' : ''
                    }`}
                  >
                    {/* System messages get a minimal banner */}
                    {isSystem ? (
                      <div className="py-1 px-3 rounded-full bg-slate-100 dark:bg-slate-800/80 border border-slate-200 dark:border-slate-700/60 text-[11px] text-slate-500 flex items-center gap-1.5">
                        <Shield className="h-3 w-3 text-purple-400" />
                        <span>{msg.content}</span>
                      </div>
                    ) : (
                      <>
                        {/* Avatar */}
                        <div
                          className={`h-8 w-8 rounded-xl flex items-center justify-center text-xs font-bold shrink-0 mt-0.5 shadow-xs ${
                            isAgent
                              ? 'bg-emerald-500/15 border border-emerald-500/30 text-emerald-600 dark:text-emerald-400'
                              : 'bg-slate-200 dark:bg-slate-800 text-slate-700 dark:text-slate-300'
                          }`}
                        >
                          {isAgent ? <Bot className="h-4 w-4" /> : <User className="h-4 w-4" />}
                        </div>

                        {/* Bubble and Header */}
                        <div className="flex-1 min-w-0 space-y-1">
                          <div className="flex items-center gap-2">
                            <span className="text-xs font-bold text-slate-900 dark:text-white">
                              {msg.author_name || (isAgent ? 'AI Agent' : 'User')}
                            </span>
                            <Badge
                              variant="outline"
                              className={`text-[9px] py-0 px-1 font-mono ${
                                isAgent
                                  ? 'border-emerald-500/40 text-emerald-600 dark:text-emerald-400 bg-emerald-500/5'
                                  : 'border-slate-300 dark:border-slate-700 text-slate-500'
                              }`}
                            >
                              {msg.author_type.toUpperCase()}
                            </Badge>
                            <span className="text-[10px] text-slate-400 font-mono">
                              {new Date(msg.created_at).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })}
                            </span>
                          </div>

                          {/* Quoted parent message (Threading) */}
                          {repliedMessage && (
                            <div className="p-2 rounded-lg bg-slate-100 dark:bg-slate-800/60 border-l-2 border-emerald-500 text-[11px] text-slate-500 space-y-0.5">
                              <span className="font-semibold text-slate-700 dark:text-slate-300 block">
                                Balasan ke {repliedMessage.author_name}:
                              </span>
                              <p className="truncate italic">{repliedMessage.content}</p>
                            </div>
                          )}

                          {/* Content Bubble */}
                          <div
                            className={`p-3.5 rounded-2xl border text-xs shadow-xs relative group ${
                              isAgent
                                ? 'bg-emerald-50/50 dark:bg-emerald-950/20 border-emerald-200/80 dark:border-emerald-800/50 text-slate-800 dark:text-slate-200'
                                : 'bg-white dark:bg-slate-900 border-slate-200 dark:border-slate-800 text-slate-900 dark:text-slate-100'
                            }`}
                          >
                            <MarkdownContent content={msg.content} />

                            {/* Hover Action Bar */}
                            <div className="opacity-0 group-hover:opacity-100 transition-opacity absolute top-2 right-2 flex items-center gap-1 bg-white/90 dark:bg-slate-900/90 backdrop-blur-xs p-0.5 rounded-lg border border-slate-200 dark:border-slate-800 shadow-sm">
                              {/* Reply Button */}
                              <button
                                type="button"
                                onClick={() => setReplyToMessage(msg)}
                                className="p-1 text-slate-400 hover:text-slate-700 dark:hover:text-slate-200 rounded hover:bg-slate-100 dark:hover:bg-slate-800"
                                title="Balas pesan ini"
                              >
                                <Reply className="h-3 w-3" />
                              </button>

                              {/* Distill to Task Button (Per message) */}
                              <button
                                type="button"
                                onClick={() => handleDistillFromMessage(msg)}
                                className="p-1 text-emerald-600 dark:text-emerald-400 hover:bg-emerald-50 dark:hover:bg-emerald-950 rounded flex items-center gap-0.5 text-[10px] font-semibold"
                                title="Distill pesan ini ke Task"
                              >
                                <Sparkles className="h-3 w-3" />
                                <span className="hidden sm:inline">Distill</span>
                              </button>

                              {/* Delete Message Button */}
                              <button
                                type="button"
                                onClick={() => handleDeleteMessage(msg.id)}
                                className="p-1 text-slate-400 hover:text-red-500 rounded hover:bg-slate-100 dark:hover:bg-slate-800"
                                title="Hapus pesan"
                              >
                                <Trash2 className="h-3 w-3" />
                              </button>
                            </div>
                          </div>
                        </div>
                      </>
                    )}
                  </div>
                )
              })
            )}
            <div ref={feedBottomRef} />
          </div>

          {/* Reply Context Banner */}
          {replyToMessage && (
            <div className="px-4 py-2 bg-emerald-50 dark:bg-emerald-950/40 border-t border-emerald-200 dark:border-emerald-800 flex items-center justify-between text-xs text-emerald-800 dark:text-emerald-300">
              <div className="flex items-center gap-2 truncate pr-2">
                <Reply className="h-3.5 w-3.5 text-emerald-600 shrink-0" />
                <span className="font-semibold">Membalas {replyToMessage.author_name}:</span>
                <span className="truncate italic text-slate-500 dark:text-slate-400">{replyToMessage.content}</span>
              </div>
              <button
                onClick={() => setReplyToMessage(null)}
                className="text-slate-400 hover:text-slate-600 dark:hover:text-slate-200 p-0.5 rounded"
              >
                <X className="h-3.5 w-3.5" />
              </button>
            </div>
          )}

          {/* Message Composer Footer */}
          <div className="p-3 border-t border-slate-100 dark:border-slate-800 bg-white dark:bg-slate-900 space-y-2">
            {/* Identity Switcher Bar */}
            <div className="flex items-center justify-between text-[11px] text-slate-500">
              <div className="flex items-center gap-2">
                <span>Kirim sebagai:</span>
                <div className="flex rounded-md bg-slate-100 dark:bg-slate-800 p-0.5">
                  {(['user', 'agent', 'system'] as const).map((role) => (
                    <button
                      key={role}
                      type="button"
                      onClick={() => {
                        setAuthorType(role)
                        if (role === 'user') setAuthorName('Buchin (PO)')
                        if (role === 'agent') setAuthorName('agy (Dev Team)')
                        if (role === 'system') setAuthorName('Cuan System')
                      }}
                      className={`px-2 py-0.5 rounded text-[10px] font-semibold transition-all ${
                        authorType === role
                          ? 'bg-white dark:bg-slate-900 text-slate-900 dark:text-white shadow-xs'
                          : 'text-slate-500 hover:text-slate-900 dark:text-slate-400'
                      }`}
                    >
                      {role.toUpperCase()}
                    </button>
                  ))}
                </div>
                <Input
                  type="text"
                  placeholder="Nama pengirim..."
                  value={authorName}
                  onChange={(e) => setAuthorName(e.target.value)}
                  className="h-6 text-[11px] px-2 w-32 border-slate-200 dark:border-slate-800"
                />
              </div>

              <span className="text-[10px] text-slate-400 hidden sm:inline">
                Tekan <strong>Enter</strong> kirim, <strong>Shift+Enter</strong> baris baru
              </span>
            </div>

            {/* Input & Send Button */}
            <div className="flex items-end gap-2">
              <textarea
                rows={2}
                placeholder={
                  activeRoom
                    ? `Tulis pesan di #${activeRoom.title}... (Mendukung markdown & code snippet)`
                    : 'Pilih room untuk mulai mengobrol...'
                }
                value={inputContent}
                onChange={(e) => setInputContent(e.target.value)}
                onKeyDown={(e) => {
                  if (e.key === 'Enter' && !e.shiftKey) {
                    e.preventDefault()
                    handleSendMessage()
                  }
                }}
                disabled={!activeRoom || sendingMessage}
                className="flex-1 text-xs p-2.5 rounded-xl border border-slate-200 dark:border-slate-800 bg-slate-50 dark:bg-slate-950 text-slate-900 dark:text-slate-100 focus:outline-none focus:ring-1 focus:ring-emerald-500 resize-none font-sans"
              />
              <Button
                onClick={() => handleSendMessage()}
                disabled={!activeRoom || !inputContent.trim() || sendingMessage}
                className="h-10 px-4 bg-emerald-600 hover:bg-emerald-500 text-white font-semibold text-xs shadow-sm shadow-emerald-600/20 shrink-0 gap-1.5"
              >
                {sendingMessage ? (
                  <RefreshCw className="h-3.5 w-3.5 animate-spin" />
                ) : (
                  <Send className="h-3.5 w-3.5" />
                )}
                <span>Kirim</span>
              </Button>
            </div>
          </div>
        </div>
      </div>

      {/* New Room Modal */}
      {isNewRoomModalOpen && (
        <div className="fixed inset-0 z-50 bg-slate-900/60 backdrop-blur-xs flex items-center justify-center p-4 animate-in fade-in duration-200">
          <div className="w-full max-w-md bg-white dark:bg-slate-900 rounded-2xl shadow-2xl border border-slate-200 dark:border-slate-800 overflow-hidden animate-in zoom-in-95 duration-200">
            <div className="p-5 border-b border-slate-100 dark:border-slate-800 flex items-center justify-between">
              <div className="flex items-center gap-2.5">
                <div className="h-8 w-8 rounded-lg bg-emerald-500/10 text-emerald-600 dark:text-emerald-400 flex items-center justify-center">
                  <Hash className="h-4 w-4" />
                </div>
                <h3 className="text-sm font-bold text-slate-900 dark:text-white">
                  Buat Room Diskusi Baru
                </h3>
              </div>
              <button
                onClick={() => setIsNewRoomModalOpen(false)}
                className="p-1 text-slate-400 hover:text-slate-600 dark:hover:text-slate-200"
              >
                <X className="h-4 w-4" />
              </button>
            </div>

            <form onSubmit={handleCreateRoom} className="p-5 space-y-3.5">
              <div className="space-y-1">
                <label className="text-xs font-semibold text-slate-600 dark:text-slate-300">
                  Judul Room <span className="text-red-500">*</span>
                </label>
                <Input
                  type="text"
                  placeholder="e.g. Sprint 7 Planning & Architecture"
                  value={newRoomTitle}
                  onChange={(e) => setNewRoomTitle(e.target.value)}
                  required
                  className="text-xs"
                />
              </div>

              <div className="space-y-1">
                <label className="text-xs font-semibold text-slate-600 dark:text-slate-300">
                  Topik / Deskripsi Singkat
                </label>
                <Input
                  type="text"
                  placeholder="e.g. Pembahasan arsitektur Discussion-to-Task Distiller"
                  value={newRoomTopic}
                  onChange={(e) => setNewRoomTopic(e.target.value)}
                  className="text-xs"
                />
              </div>

              <div className="pt-2 flex items-center justify-end gap-2">
                <Button
                  type="button"
                  variant="outline"
                  size="sm"
                  onClick={() => setIsNewRoomModalOpen(false)}
                  className="text-xs h-8"
                >
                  Batal
                </Button>
                <Button
                  type="submit"
                  size="sm"
                  disabled={creatingRoom || !newRoomTitle.trim()}
                  className="text-xs h-8 bg-emerald-600 hover:bg-emerald-500 text-white font-semibold"
                >
                  {creatingRoom ? 'Membuat...' : 'Buat Room'}
                </Button>
              </div>
            </form>
          </div>
        </div>
      )}

      {/* Distill Task Modal */}
      <DistillTaskModal
        isOpen={isDistillModalOpen}
        onClose={() => setIsDistillModalOpen(false)}
        projectId={selectedProjectId}
        roomId={activeRoomId || undefined}
        roomTitle={activeRoom?.title}
        selectedMessageIds={distillTargetMessageIds}
        selectedMessageContent={distillTargetContent}
        onTaskCreated={(task) => {
          setNotification({
            type: 'success',
            message: `✨ Task "${task.title}" berhasil dibuat dan disimpan ke backlog!`,
          })
        }}
      />
    </div>
  )
}
