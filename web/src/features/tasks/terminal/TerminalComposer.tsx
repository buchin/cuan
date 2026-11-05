import { Composer, ComposerSendButton } from '../../../components/ui/Composer'
import { ComposerInput, resizeComposerInput } from '../../../components/ui/ComposerInput'
import { useCallback, useEffect, useRef, useState } from 'react'
import {
  Camera,
  Images,
  FolderOpen,
  Paperclip,
  Send,
  X,
  FileText,
  Clock,
  GitBranch,
  Trash2,
  Ban,
  Sparkles,
  Loader2,
  LoaderCircle,
  Check,
  ChevronRight,
  Mic,
  MicOff,
} from 'lucide-react'
import { getAuthToken } from '../../../api/client'
import { ActionDialog } from '../../../components/dialogs/ActionDialog'
import { useLanguage } from '../../../context/LanguageContext'
import { isImageFile } from '../../../components/dialogs/filePreviewUtils'
import '../../inbox/chat-calm.css'

export type TerminalComposerProps = {
  taskId: string
  attemptId?: string
  onSend: (data: string) => void
  onSteer?: (prompt: string) => Promise<boolean | void> | void
  onStop?: () => void
  onClearScreen: () => void
  onInterrupt: () => void
  showQuickActions?: boolean
  disabled?: boolean
  className?: string
}

interface SlashCommand {
  command: string
  label: string
  description: string
  action: (composer: {
    sendInput: (text: string) => void
    openUpload: () => void
    openUsage: () => void
    openDiff: () => void
  }) => void
}

export function TerminalComposer({
  taskId,
  attemptId,
  onSend,
  onSteer,
  onStop,
  onClearScreen,
  onInterrupt,
  showQuickActions = false,
  disabled = false,
  className = '',
}: TerminalComposerProps) {
  const { t } = useLanguage()

  const [text, setText] = useState('')
  const [history, setHistory] = useState<string[]>(() => {
    try {
      const saved = localStorage.getItem(`cuan_terminal_history_${taskId}`)
      return saved ? JSON.parse(saved) : []
    } catch {
      return []
    }
  })
  const [historyIdx, setHistoryIdx] = useState(-1)
  const [activeSlashIndex, setActiveSlashIndex] = useState(0)
  const [showSlashPopup, setShowSlashPopup] = useState(false)
  const [slashQuery, setSlashQuery] = useState('')

  // Upload states & Attachment Picker
  const [showAttachmentMenu, setShowAttachmentMenu] = useState(false)
  const [uploading, setUploading] = useState(false)
  const [uploadedFiles, setUploadedFiles] = useState<{
    name: string
    path: string
    size: number
    previewUrl?: string
  }[]>([])
  const [uploadError, setUploadError] = useState('')

  // Modals for slash commands
  const [usageData, setUsageData] = useState<{
    status?: string
    duration_seconds?: number
    runner_id?: string
    agent_name?: string
    workspace?: { branch?: string; files_count?: number }
  } | null>(null)
  const [diffData, setDiffData] = useState<{ stat?: string; status?: string } | null>(null)
  const [loadingUsage, setLoadingUsage] = useState(false)
  const [loadingDiff, setLoadingDiff] = useState(false)

  const textareaRef = useRef<HTMLTextAreaElement>(null)
  const cameraInputRef = useRef<HTMLInputElement>(null)
  const photosInputRef = useRef<HTMLInputElement>(null)
  const filesInputRef = useRef<HTMLInputElement>(null)

  // Speech-to-Text (Voice Dictation)
  const [speechSupported, setSpeechSupported] = useState(false)
  const [isListening, setIsListening] = useState(false)
  const recognitionRef = useRef<any>(null)

  useEffect(() => {
    if (typeof window !== 'undefined') {
      const SpeechRecognition =
        (window as any).SpeechRecognition || (window as any).webkitSpeechRecognition
      setSpeechSupported(Boolean(SpeechRecognition))
    }
  }, [])

  // Auto-resize textarea to match Inbox experience (min 38-44px, max 140px, auto overflow)
  const autoResizeTextarea = (el: HTMLTextAreaElement) => {
    resizeComposerInput(el)
  }

  // Restore draft prompt from localStorage on mount
  useEffect(() => {
    try {
      const draft = localStorage.getItem(`cuan_task_draft_${taskId}`)
      if (draft && !text) {
        setText(draft)
        if (textareaRef.current) {
          setTimeout(() => {
            if (textareaRef.current) autoResizeTextarea(textareaRef.current)
          }, 0)
        }
      }
    } catch {}
  }, [taskId])

  // Autosave draft prompt (debounced)
  useEffect(() => {
    const timer = setTimeout(() => {
      try {
        if (text.trim()) {
          localStorage.setItem(`cuan_task_draft_${taskId}`, text)
        } else {
          localStorage.removeItem(`cuan_task_draft_${taskId}`)
        }
      } catch {}
    }, 400)
    return () => clearTimeout(timer)
  }, [text, taskId])

  // Toggle Voice Dictation (Speech to Text)
  const toggleListening = useCallback(() => {
    if (isListening) {
      if (recognitionRef.current) {
        try {
          recognitionRef.current.stop()
        } catch {}
      }
      setIsListening(false)
      return
    }

    const SpeechRecognition =
      (window as any).SpeechRecognition || (window as any).webkitSpeechRecognition
    if (!SpeechRecognition) return

    try {
      const recognition = new SpeechRecognition()
      recognitionRef.current = recognition
      recognition.continuous = true
      recognition.interimResults = true
      recognition.lang = navigator.language || 'id-ID'

      recognition.onstart = () => {
        setIsListening(true)
      }

      recognition.onresult = (event: any) => {
        let interim = ''
        let final = ''
        for (let i = event.resultIndex; i < event.results.length; ++i) {
          if (event.results[i].isFinal) {
            final += event.results[i][0].transcript
          } else {
            interim += event.results[i][0].transcript
          }
        }
        const transcript = (final || interim).trim()
        if (transcript) {
          setText((prev) => {
            const base = prev.trim()
            return base ? `${base} ${transcript}` : transcript
          })
          if (textareaRef.current) {
            setTimeout(() => {
              if (textareaRef.current) autoResizeTextarea(textareaRef.current)
            }, 0)
          }
        }
      }

      recognition.onerror = () => {
        setIsListening(false)
      }

      recognition.onend = () => {
        setIsListening(false)
      }

      recognition.start()
    } catch {
      setIsListening(false)
    }
  }, [isListening])

  // Revoke object URLs on unmount or file removal
  useEffect(() => {
    return () => {
      for (const f of uploadedFiles) {
        if (f.previewUrl) URL.revokeObjectURL(f.previewUrl)
      }
    }
  }, [uploadedFiles])

  // Save history to local storage
  const addToHistory = useCallback((cmd: string) => {
    const trimmed = cmd.trim()
    if (!trimmed) return
    setHistory((prev) => {
      const updated = [trimmed, ...prev.filter((h) => h !== trimmed)].slice(0, 50)
      try {
        localStorage.setItem(`cuan_terminal_history_${taskId}`, JSON.stringify(updated))
      } catch {}
      return updated
    })
    setHistoryIdx(-1)
  }, [taskId])

  const handleOpenUsage = useCallback(async () => {
    try {
      setLoadingUsage(true)
      const token = getAuthToken()
      const res = await fetch(`/api/v1/tasks/${encodeURIComponent(taskId)}/usage`, {
        headers: token ? { Authorization: `Bearer ${token}` } : {},
      })
      if (res.ok) {
        const data = await res.json()
        setUsageData(data)
      }
    } catch {
      // ignore
    } finally {
      setLoadingUsage(false)
    }
  }, [taskId])

  const handleOpenDiff = useCallback(async () => {
    try {
      setLoadingDiff(true)
      const token = getAuthToken()
      const res = await fetch(`/api/v1/tasks/${encodeURIComponent(taskId)}/diff`, {
        headers: token ? { Authorization: `Bearer ${token}` } : {},
      })
      if (res.ok) {
        const data = await res.json()
        setDiffData(data)
      }
    } catch {
      // ignore
    } finally {
      setLoadingDiff(false)
    }
  }, [taskId])

  const slashCommands: SlashCommand[] = [
    {
      command: '/usage',
      label: 'Resource & Task Usage',
      description: 'Lihat durasi eksekusi, runner, branch, dan statistik workspace',
      action: () => handleOpenUsage(),
    },
    {
      command: '/upload',
      label: 'Upload File ke Workspace',
      description: 'Unggah kamera, foto, atau file ke workspace task',
      action: () => setShowAttachmentMenu(true),
    },
    {
      command: '/diff',
      label: 'Git Diff Summary',
      description: 'Intip ringkasan perubahan kode git yang dibuat agent',
      action: () => handleOpenDiff(),
    },
    {
      command: '/files',
      label: 'List Workspace Files',
      description: 'Kirim ls -la untuk melihat daftar file di folder tugas',
      action: () => onSend('ls -la\r'),
    },
    {
      command: '/clear',
      label: 'Clear Screen',
      description: 'Bersihkan tampilan buffer terminal',
      action: () => onClearScreen(),
    },
    {
      command: '/stop',
      label: 'Stop Process (Ctrl+C)',
      description: 'Kirim sinyal interupsi SIGINT ke proses yang berjalan',
      action: () => (onStop ? onStop() : onInterrupt()),
    },
  ]

  const filteredCommands = slashCommands.filter((cmd) =>
    cmd.command.toLowerCase().includes(slashQuery.toLowerCase()) ||
    cmd.label.toLowerCase().includes(slashQuery.toLowerCase())
  )

  const executeSlashCommand = (cmd: SlashCommand) => {
    setShowSlashPopup(false)
    setText('')
    cmd.action({
      sendInput: (val: string) => onSend(val),
      openUpload: () => setShowAttachmentMenu(true),
      openUsage: handleOpenUsage,
      openDiff: handleOpenDiff,
    })
  }

  const handleSend = (cmdText?: string) => {
    if (disabled) return
    const toSend = cmdText !== undefined ? cmdText : text
    if (!toSend.trim() && uploadedFiles.length === 0) return

    if (toSend.trim().startsWith('/')) {
      const matched = slashCommands.find((c) => c.command.toLowerCase() === toSend.trim().toLowerCase())
      if (matched) {
        executeSlashCommand(matched)
        setText('')
        setShowSlashPopup(false)
        return
      }
    }

    addToHistory(toSend)
    onSend(toSend + '\n')
    if (onSteer) {
      void onSteer(toSend)
    }
    setText('')
    setShowSlashPopup(false)
    try {
      localStorage.removeItem(`cuan_task_draft_${taskId}`)
    } catch {}
    if (textareaRef.current) {
      textareaRef.current.style.height = 'auto'
    }
    // Clear uploaded files previews after sending
    for (const f of uploadedFiles) {
      if (f.previewUrl) URL.revokeObjectURL(f.previewUrl)
    }
    setUploadedFiles([])
  }

  const handleInputChange = (e: React.ChangeEvent<HTMLTextAreaElement>) => {
    const val = e.target.value
    setText(val)
    autoResizeTextarea(e.target)

    if (val.startsWith('/')) {
      setShowSlashPopup(true)
      setSlashQuery(val.slice(1))
      setActiveSlashIndex(0)
    } else {
      setShowSlashPopup(false)
      setSlashQuery('')
    }
  }

  const handleKeyDown = (e: React.KeyboardEvent<HTMLTextAreaElement>) => {
    if (showSlashPopup && filteredCommands.length > 0) {
      if (e.key === 'ArrowDown') {
        e.preventDefault()
        setActiveSlashIndex((prev) => (prev + 1) % filteredCommands.length)
        return
      }
      if (e.key === 'ArrowUp') {
        e.preventDefault()
        setActiveSlashIndex((prev) => (prev - 1 + filteredCommands.length) % filteredCommands.length)
        return
      }
      if (e.key === 'Enter') {
        e.preventDefault()
        executeSlashCommand(filteredCommands[activeSlashIndex])
        return
      }
      if (e.key === 'Escape') {
        e.preventDefault()
        setShowSlashPopup(false)
        return
      }
    }

    if (e.key === 'Enter' && !e.shiftKey && !e.nativeEvent.isComposing) {
      e.preventDefault()
      handleSend()
      return
    }

    // Command history navigation (when input is single line or empty)
    if (!text.includes('\n')) {
      if (e.key === 'ArrowUp') {
        if (history.length > 0) {
          e.preventDefault()
          const nextIdx = Math.min(historyIdx + 1, history.length - 1)
          setHistoryIdx(nextIdx)
          setText(history[nextIdx] || '')
          if (textareaRef.current) {
            setTimeout(() => {
              if (textareaRef.current) autoResizeTextarea(textareaRef.current)
            }, 0)
          }
        }
      } else if (e.key === 'ArrowDown') {
        if (historyIdx > 0) {
          e.preventDefault()
          const nextIdx = historyIdx - 1
          setHistoryIdx(nextIdx)
          setText(history[nextIdx] || '')
          if (textareaRef.current) {
            setTimeout(() => {
              if (textareaRef.current) autoResizeTextarea(textareaRef.current)
            }, 0)
          }
        } else if (historyIdx === 0) {
          e.preventDefault()
          setHistoryIdx(-1)
          setText('')
          if (textareaRef.current) textareaRef.current.style.height = 'auto'
        }
      }
    }
  }

  // Intercept pasted images from clipboard directly into the composer
  const handleTextareaPaste = (e: React.ClipboardEvent<HTMLTextAreaElement>) => {
    const items = e.clipboardData?.items
    if (!items || items.length === 0) return
    const imageFiles: File[] = []
    for (let i = 0; i < items.length; i++) {
      const item = items[i]
      if (item.kind === 'file' && item.type.startsWith('image/')) {
        const file = item.getAsFile()
        if (file) imageFiles.push(file)
      }
    }
    if (imageFiles.length > 0) {
      e.preventDefault()
      void handleFileUpload(imageFiles)
    }
  }

  // Upload handler for Camera, Photos, or Files
  const handleFileUpload = async (files: FileList | File[]) => {
    if (!files || files.length === 0) return

    setUploading(true)
    setUploadError('')

    const fileList = Array.from(files)
    const formData = new FormData()
    for (const f of fileList) {
      formData.append('file', f)
    }

    try {
      const token = getAuthToken()
      const headers: Record<string, string> = {}
      if (token) {
        headers['Authorization'] = `Bearer ${token}`
      }
      const res = await fetch(`/api/v1/tasks/${encodeURIComponent(taskId)}/files`, {
        method: 'POST',
        headers,
        credentials: 'include',
        body: formData,
      })

      if (!res.ok) {
        const errJson = await res.json().catch(() => ({}))
        throw new Error(errJson.error || errJson.message || `Upload failed (${res.status})`)
      }

      const newUploaded: { name: string; path: string; size: number; previewUrl?: string }[] = []

      for (const f of fileList) {
        const isImg = isImageFile(f.name, f.type)
        const previewUrl = isImg ? URL.createObjectURL(f) : undefined
        newUploaded.push({
          name: f.name,
          path: f.name,
          size: f.size,
          previewUrl,
        })
      }

      setUploadedFiles((prev) => [...prev, ...newUploaded])

      // Automatically append uploaded paths into composer input
      const paths = newUploaded.map((f) => `./${f.name}`).join(' ')
      setText((prev) => (prev ? `${prev.trim()} ${paths} ` : `${paths} `))
      if (textareaRef.current) {
        setTimeout(() => {
          if (textareaRef.current) {
            autoResizeTextarea(textareaRef.current)
            textareaRef.current.focus()
          }
        }, 0)
      }
    } catch (err) {
      setUploadError(err instanceof Error ? err.message : 'Upload failed')
      setTimeout(() => setUploadError(''), 4000)
    } finally {
      setUploading(false)
    }
  }

  const handleRemoveFile = (idx: number) => {
    setUploadedFiles((prev) => {
      const target = prev[idx]
      if (target?.previewUrl) URL.revokeObjectURL(target.previewUrl)
      const next = prev.filter((_, i) => i !== idx)
      if (target) {
        setText((current) => current.replace(`./${target.name}`, '').replace(target.name, '').trim())
      }
      return next
    })
  }

  return (
    <div
      data-attempt-id={attemptId}
      className={`wa-chat relative shrink-0 select-none bg-[#141414] border-t border-[#262626] px-2.5 pt-2 pb-[max(0.75rem,env(safe-area-inset-bottom))] space-y-1.5 ${className}`}
    >
      {/* ── Slash Commands Autocomplete Popup ── */}
      {showSlashPopup && filteredCommands.length > 0 && (
        <div className="absolute left-2.5 right-2.5 bottom-full mb-1.5 z-40 max-h-56 overflow-y-auto rounded-xl border border-neutral-700 bg-[#1f1f23] p-1 shadow-2xl backdrop-blur-md">
          <div className="flex items-center gap-1.5 px-2.5 py-1 text-[11px] font-semibold uppercase tracking-wider text-neutral-400 border-b border-neutral-800">
            <Sparkles className="h-3 w-3 text-amber-400" />
            <span>Slash Commands</span>
          </div>
          <div className="py-1">
            {filteredCommands.map((cmd, idx) => (
              <button
                key={cmd.command}
                type="button"
                onClick={() => executeSlashCommand(cmd)}
                className={`w-full flex items-center justify-between gap-2 px-2.5 py-1.5 rounded-lg text-left text-xs font-mono transition-colors ${
                  idx === activeSlashIndex
                    ? 'bg-primary/25 text-primary-foreground font-medium'
                    : 'text-neutral-300 hover:bg-neutral-800'
                }`}
              >
                <div className="flex items-center gap-2 min-w-0">
                  <span className="font-bold text-emerald-400 shrink-0">{cmd.command}</span>
                  <span className="truncate text-neutral-400 text-[11px] font-sans">{cmd.description}</span>
                </div>
                <ChevronRight className="h-3 w-3 text-neutral-500 shrink-0" />
              </button>
            ))}
          </div>
        </div>
      )}

      {/* ── Loading Overlay ── */}
      {(loadingUsage || loadingDiff) && (
        <div className="absolute left-2.5 right-2.5 bottom-full mb-2 z-40 rounded-xl border border-neutral-700 bg-[#18181b] p-3 shadow-2xl flex items-center gap-2 text-xs text-neutral-300 font-mono">
          <Loader2 className="h-4 w-4 animate-spin text-primary" />
          <span>Memuat data {loadingUsage ? 'resource usage' : 'git diff'}...</span>
        </div>
      )}

      {/* ── Usage Overlay Card ── */}
      {usageData && (
        <div className="absolute left-2.5 right-2.5 bottom-full mb-2 z-40 rounded-xl border border-neutral-700 bg-[#18181b] p-3 shadow-2xl text-xs space-y-2">
          <div className="flex items-center justify-between border-b border-neutral-800 pb-2">
            <div className="flex items-center gap-2 font-semibold text-neutral-200">
              <Clock className="h-4 w-4 text-primary" />
              <span>Task Execution & Resource Usage</span>
            </div>
            <button
              type="button"
              onClick={() => setUsageData(null)}
              className="text-neutral-400 hover:text-white p-1 rounded"
            >
              <X className="h-3.5 w-3.5" />
            </button>
          </div>
          <div className="grid grid-cols-2 sm:grid-cols-4 gap-2 text-neutral-300 font-mono text-[11px]">
            <div className="bg-neutral-900/80 p-2 rounded-lg border border-neutral-800">
              <span className="text-neutral-500 block text-[10px] uppercase">Status</span>
              <span className="font-bold capitalize text-emerald-400">{usageData.status || 'Active'}</span>
            </div>
            <div className="bg-neutral-900/80 p-2 rounded-lg border border-neutral-800">
              <span className="text-neutral-500 block text-[10px] uppercase">Duration</span>
              <span className="font-bold text-neutral-100">{usageData.duration_seconds ? `${usageData.duration_seconds}s` : '—'}</span>
            </div>
            <div className="bg-neutral-900/80 p-2 rounded-lg border border-neutral-800">
              <span className="text-neutral-500 block text-[10px] uppercase">Runner / Agent</span>
              <span className="font-bold text-neutral-100 truncate block">{usageData.agent_name || usageData.runner_id || 'local_terminal'}</span>
            </div>
            <div className="bg-neutral-900/80 p-2 rounded-lg border border-neutral-800">
              <span className="text-neutral-500 block text-[10px] uppercase">Files Count</span>
              <span className="font-bold text-neutral-100">{usageData.workspace?.files_count ?? '—'}</span>
            </div>
          </div>
        </div>
      )}

      {/* ── Diff Overlay Card ── */}
      {diffData && (
        <div className="absolute left-2.5 right-2.5 bottom-full mb-2 z-40 rounded-xl border border-neutral-700 bg-[#18181b] p-3 shadow-2xl text-xs space-y-2 max-h-60 overflow-y-auto">
          <div className="flex items-center justify-between border-b border-neutral-800 pb-2">
            <div className="flex items-center gap-2 font-semibold text-neutral-200">
              <GitBranch className="h-4 w-4 text-emerald-400" />
              <span>Workspace Git Diff Summary</span>
            </div>
            <button
              type="button"
              onClick={() => setDiffData(null)}
              className="text-neutral-400 hover:text-white p-1 rounded"
            >
              <X className="h-3.5 w-3.5" />
            </button>
          </div>
          <pre className="font-mono text-[11px] text-neutral-300 bg-neutral-950 p-2 rounded border border-neutral-800 overflow-x-auto whitespace-pre-wrap">
            {diffData.stat || diffData.status || 'No uncommitted changes in task workspace.'}
          </pre>
        </div>
      )}

      {uploadError && (
        <p className="text-xs text-destructive px-1">{uploadError}</p>
      )}

      {/* ── Quick Action Chips Row (Hidden by default for non-developer zero visual noise) ── */}
      {showQuickActions && (
        <div
          role="toolbar"
          aria-label="Terminal Quick Action Chips"
          className="flex items-center gap-1.5 overflow-x-auto select-none py-0.5 px-0.5 [&::-webkit-scrollbar]:hidden [-ms-overflow-style:none] [scrollbar-width:none]"
        >


          <button
            type="button"
            disabled={disabled}
            onClick={() => handleSend('y')}
            className="inline-flex min-h-[44px] min-w-[44px] sm:min-h-7 sm:min-w-0 shrink-0 items-center justify-center gap-1.5 rounded-lg border border-[#333] bg-[#222] px-3 text-xs font-mono font-medium text-neutral-200 hover:bg-[#333] active:bg-neutral-700 transition-colors disabled:opacity-40 cursor-pointer"
            title="Send 'y' (confirm Yes)"
            aria-label="Send y (confirm Yes)"
          >
            <Check className="h-3 w-3 text-emerald-400" />
            <span>y (Yes)</span>
          </button>

          <button
            type="button"
            disabled={disabled}
            onClick={() => handleSend('n')}
            className="inline-flex min-h-[44px] min-w-[44px] sm:min-h-7 sm:min-w-0 shrink-0 items-center justify-center gap-1.5 rounded-lg border border-[#333] bg-[#222] px-3 text-xs font-mono font-medium text-neutral-200 hover:bg-[#333] active:bg-neutral-700 transition-colors disabled:opacity-40 cursor-pointer"
            title="Send 'n' (confirm No)"
            aria-label="Send n (confirm No)"
          >
            <X className="h-3 w-3 text-rose-400" />
            <span>n (No)</span>
          </button>

          <button
            type="button"
            disabled={disabled}
            onClick={onStop || onInterrupt}
            className="inline-flex min-h-[44px] min-w-[44px] sm:min-h-7 sm:min-w-0 shrink-0 items-center justify-center gap-1.5 rounded-lg border border-rose-500/30 bg-rose-500/10 px-3 text-xs font-mono font-semibold text-rose-300 hover:bg-rose-500/20 active:bg-rose-500/30 transition-colors disabled:opacity-40 cursor-pointer"
            title="Send SIGINT (Ctrl+C) to cancel process"
            aria-label="Stop Process (Ctrl+C)"
          >
            <Ban className="h-3 w-3 text-rose-400" />
            <span>Stop (Ctrl+C)</span>
          </button>

          <button
            type="button"
            disabled={disabled}
            onClick={onClearScreen}
            className="inline-flex min-h-[44px] min-w-[44px] sm:min-h-7 sm:min-w-0 shrink-0 items-center justify-center gap-1.5 rounded-lg border border-[#333] bg-[#222] px-3 text-xs font-mono font-medium text-neutral-300 hover:bg-[#333] active:bg-neutral-700 transition-colors disabled:opacity-40 cursor-pointer"
            title="Clear screen buffer"
            aria-label="Clear screen buffer"
          >
            <Trash2 className="h-3 w-3 text-neutral-400" />
            <span>Clear</span>
          </button>

          <button
            type="button"
            disabled={disabled}
            onClick={() => onSend('\t')}
            className="inline-flex min-h-[44px] min-w-[44px] sm:min-h-7 sm:min-w-0 shrink-0 items-center justify-center gap-1.5 rounded-lg border border-[#333] bg-[#222] px-3 text-xs font-mono font-medium text-neutral-300 hover:bg-[#333] active:bg-neutral-700 transition-colors disabled:opacity-40 cursor-pointer"
            title="Send TAB (autocomplete)"
            aria-label="Send Tab autocomplete"
          >
            <span>Tab</span>
          </button>
        </div>
      )}

      {/* ── Attachment Previews Strip (Inbox-style) ── */}
      {(uploadedFiles.length > 0 || uploading) && (
        <div className="flex items-center gap-2 overflow-x-auto py-1 px-0.5 select-none [&::-webkit-scrollbar]:hidden [-ms-overflow-style:none] [scrollbar-width:none]">
          {uploadedFiles.map((f, idx) => (
            <div
              key={f.name + idx}
              className="flex items-center gap-2 rounded-xl bg-neutral-900 border border-neutral-700/80 pl-1.5 pr-2 py-1 shrink-0 max-w-[220px]"
            >
              {f.previewUrl ? (
                <img
                  src={f.previewUrl}
                  alt={f.name}
                  className="h-8 w-8 rounded-lg object-cover border border-neutral-700 shrink-0"
                />
              ) : (
                <div className="h-8 w-8 rounded-lg bg-neutral-800 border border-neutral-700 flex items-center justify-center shrink-0">
                  <FileText className="h-4 w-4 text-emerald-400" />
                </div>
              )}
              <div className="min-w-0 flex flex-col justify-center">
                <span className="truncate text-xs font-medium text-neutral-200 leading-tight">
                  {f.name}
                </span>
                <span className="text-[10px] text-neutral-400 leading-tight">
                  {Math.max(1, Math.ceil(f.size / 1024))} KB
                </span>
              </div>
              <button
                type="button"
                onClick={() => handleRemoveFile(idx)}
                className="p-1 text-neutral-400 hover:text-white rounded-full hover:bg-neutral-800 transition-colors ml-auto shrink-0"
                title="Remove attachment"
              >
                <X className="h-3 w-3" />
              </button>
            </div>
          ))}

          {uploading && (
            <div className="flex items-center gap-1.5 text-xs text-emerald-400 font-mono py-1 px-2 shrink-0">
              <LoaderCircle className="h-4 w-4 animate-spin" />
              <span>{t('Uploading…')}</span>
            </div>
          )}
        </div>
      )}

      {/* ── Inbox-Style Capsule Composer (Div container to prevent browser/Android autofill bars) ── */}
      <Composer role="group" aria-label="Terminal input composer">
        {/* Attachment Picker Button (WCAG >= 44x44px touch target on mobile) */}
        <button
          type="button"
          disabled={disabled || uploading}
          onClick={() => setShowAttachmentMenu(true)}
          className="wa-action-btn min-h-[44px] min-w-[44px] h-11 w-11 sm:h-10 sm:w-10 flex items-center justify-center rounded-full transition-all shrink-0 disabled:opacity-40 mb-0.5 cursor-pointer"
          title="Upload file ke workspace tugas (/upload)"
          aria-label={t('Add attachment')}
        >
          {uploading ? (
            <LoaderCircle className="h-5 w-5 animate-spin text-emerald-400" />
          ) : (
            <Paperclip className="h-5 w-5" />
          )}
        </button>

        {/* Multiline Command / Text Input (Auto-expanding like Inbox with min-h-[44px] on mobile) */}
        <div className="relative min-w-0 flex flex-1 items-center">
          <ComposerInput
            ref={textareaRef}
            rows={1}
            name="terminal_query"
            id="terminal_composer_input"
            value={text}
            onChange={handleInputChange}
            onKeyDown={handleKeyDown}
            onPaste={handleTextareaPaste}
            placeholder={
              uploading
                ? 'Mengunggah file ke workspace…'
                : 'Ketik perintah atau pesan…'
            }
            disabled={disabled}
            autoCapitalize="none"
            autoCorrect="off"
            autoComplete="off"
            spellCheck={false}
            enterKeyHint="send"
            data-form-type="other"
            data-lpignore="true"
            data-1p-ignore="true"
            className="w-full pr-8"
          />
          {text && (
            <button
              type="button"
              onClick={() => {
                setText('')
                setShowSlashPopup(false)
                if (textareaRef.current) {
                  textareaRef.current.style.height = 'auto'
                  textareaRef.current.focus()
                }
              }}
              className="absolute right-1.5 top-1/2 -translate-y-1/2 min-h-[36px] min-w-[36px] flex items-center justify-center p-1 text-neutral-400 hover:text-white transition-colors cursor-pointer"
              title="Clear input"
              aria-label="Clear input"
            >
              <X className="h-4 w-4" />
            </button>
          )}
        </div>

        {/* Voice Dictation (Speech-to-Text) Button (Shown when not typing; replaces send on empty text) */}
        {speechSupported && !text.trim() && (
          <button
            type="button"
            disabled={disabled}
            onClick={toggleListening}
            className={`min-h-[44px] min-w-[44px] h-11 w-11 sm:h-10 sm:w-10 rounded-full flex items-center justify-center transition-all shrink-0 mb-0.5 cursor-pointer ${
              isListening
                ? 'bg-rose-600 text-white animate-pulse shadow-md'
                : 'text-neutral-400 hover:text-emerald-400 hover:bg-neutral-800'
            }`}
            title={isListening ? 'Hentikan dikte suara' : 'Dikte suara (Speech-to-Text)'}
            aria-label="Voice input"
          >
            {isListening ? <MicOff className="h-5 w-5" /> : <Mic className="h-5 w-5" />}
          </button>
        )}

        {/* Stop / Cancel Process Button (Desktop only; on mobile, header already provides prominent red stop button) */}
        {(Boolean(onStop) || typeof onInterrupt === 'function') && (
          <button
            type="button"
            disabled={disabled}
            onClick={onStop || onInterrupt}
            className="hidden sm:flex min-h-[44px] min-w-[44px] sm:h-10 sm:w-10 rounded-full items-center justify-center text-rose-400 hover:text-rose-300 hover:bg-rose-500/10 active:scale-95 transition-all cursor-pointer shrink-0 mb-0.5"
            title="Hentikan proses"
            aria-label="Hentikan proses"
          >
            <Ban className="h-5 w-5" />
          </button>
        )}

        <ComposerSendButton
          type="button"
          disabled={disabled || (!text.trim() && uploadedFiles.length === 0)}
          onClick={() => handleSend()}
          title="Kirim ke terminal (Enter)"
          aria-label="Send input"
        >
          <Send className="h-4 w-4" />
        </ComposerSendButton>
      </Composer>

      {/* ── Hidden File Inputs (Always mounted in DOM root to ensure mobile file pickers never unmount) ── */}
      <input
        ref={cameraInputRef}
        type="file"
        accept="image/jpeg,image/png"
        capture="environment"
        className="sr-only"
        tabIndex={-1}
        aria-label={t('Take a photo')}
        onChange={(e) => {
          if (e.target.files) void handleFileUpload(e.target.files)
          e.target.value = ''
        }}
      />
      <input
        ref={photosInputRef}
        type="file"
        accept="image/jpeg,image/png,image/gif,image/webp"
        multiple
        className="sr-only"
        tabIndex={-1}
        aria-label={t('Choose photos')}
        onChange={(e) => {
          if (e.target.files) void handleFileUpload(e.target.files)
          e.target.value = ''
        }}
      />
      <input
        ref={filesInputRef}
        type="file"
        accept="*/*"
        multiple
        className="sr-only"
        tabIndex={-1}
        aria-label={t('Choose files')}
        onChange={(e) => {
          if (e.target.files) void handleFileUpload(e.target.files)
          e.target.value = ''
        }}
      />

      {/* ── Attachment Choices Dialog (Camera, Photos, Files) ── */}
      <ActionDialog
        open={showAttachmentMenu}
        title={t('Add attachment')}
        onClose={() => setShowAttachmentMenu(false)}
      >
        <div className="space-y-1">
          {[
            {
              label: t('Take a photo'),
              description: 'Ambil foto kamera langsung ke workspace',
              Icon: Camera,
              ref: cameraInputRef,
            },
            {
              label: t('Choose photos'),
              description: 'Pilih gambar dari galeri perangkat',
              Icon: Images,
              ref: photosInputRef,
            },
            {
              label: t('Choose files'),
              description: 'Unggah file kode, teks, dokumen, dsb.',
              Icon: FolderOpen,
              ref: filesInputRef,
            },
          ].map(({ label, description, Icon, ref }) => (
            <button
              key={label}
              type="button"
              className="flex min-h-12 w-full items-center gap-3 rounded-lg px-3 py-2 text-left hover:bg-accent text-neutral-100 transition-colors"
              onClick={() => {
                ref.current?.click()
                setShowAttachmentMenu(false)
              }}
            >
              <div className="h-9 w-9 rounded-full bg-emerald-500/10 flex items-center justify-center text-emerald-400 shrink-0">
                <Icon className="h-5 w-5" />
              </div>
              <div className="min-w-0 flex flex-col">
                <span className="text-sm font-medium text-neutral-100">{label}</span>
                <span className="text-xs text-neutral-400">{description}</span>
              </div>
            </button>
          ))}
        </div>
      </ActionDialog>
    </div>
  )
}
