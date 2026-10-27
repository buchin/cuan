import { useCallback, useRef, useState } from 'react'
import {
  Paperclip,
  Send,
  X,
  FileText,
  Clock,
  GitBranch,
  Trash2,
  Ban,
  Keyboard,
  Sparkles,
  Loader2,
  Check,
  ChevronRight,
} from 'lucide-react'
import { getAuthToken } from '../../../api/client'

export type TerminalComposerProps = {
  taskId: string
  onSend: (data: string) => void
  onClearScreen: () => void
  onInterrupt: () => void
  onToggleMobileKbd?: () => void
  isMobileKbdActive?: boolean
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
  onSend,
  onClearScreen,
  onInterrupt,
  onToggleMobileKbd,
  isMobileKbdActive = false,
  disabled = false,
  className = '',
}: TerminalComposerProps) {
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

  // Upload states
  const [uploading, setUploading] = useState(false)
  const [uploadedFiles, setUploadedFiles] = useState<{ name: string; path: string; size: number }[]>([])
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

  const inputRef = useRef<HTMLInputElement>(null)
  const fileInputRef = useRef<HTMLInputElement>(null)

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
      description: 'Unggah file lokal langsung ke folder workspace task',
      action: () => fileInputRef.current?.click(),
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
      label: 'Clear Terminal Screen',
      description: 'Bersihkan tampilan buffer terminal (Ctrl+L)',
      action: () => onClearScreen(),
    },
    {
      command: '/stop',
      label: 'Stop Execution (Ctrl+C)',
      description: 'Kirim sinyal SIGINT untuk menghentikan proses',
      action: () => onInterrupt(),
    },
  ]

  const filteredCommands = slashCommands.filter(
    (c) =>
      c.command.toLowerCase().includes(slashQuery.toLowerCase()) ||
      c.label.toLowerCase().includes(slashQuery.toLowerCase()),
  )

  // Handle text input changes & detect slash commands
  const handleInputChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    const val = e.target.value
    setText(val)
    if (val.startsWith('/')) {
      setShowSlashPopup(true)
      setSlashQuery(val.slice(1).trim())
      setActiveSlashIndex(0)
    } else {
      setShowSlashPopup(false)
    }
  }

  const executeSlashCommand = useCallback((cmd: SlashCommand) => {
    setShowSlashPopup(false)
    setText('')
    addToHistory(cmd.command)
    cmd.action({
      sendInput: onSend,
      openUpload: () => fileInputRef.current?.click(),
      openUsage: handleOpenUsage,
      openDiff: handleOpenDiff,
    })
    inputRef.current?.focus()
  }, [addToHistory, onSend, handleOpenUsage, handleOpenDiff])

  const handleSend = useCallback((customText?: string) => {
    if (disabled) return
    const toSend = typeof customText === 'string' ? customText : text

    // Check if it's an exact slash command
    if (toSend.startsWith('/')) {
      const match = slashCommands.find(
        (c) => c.command.toLowerCase() === toSend.trim().toLowerCase(),
      )
      if (match) {
        executeSlashCommand(match)
        return
      }
    }

    if (!toSend.trim()) {
      onSend('\r')
      return
    }

    onSend(toSend + '\r')
    addToHistory(toSend)
    setText('')
    setShowSlashPopup(false)
    setUploadedFiles([])
  }, [disabled, text, slashCommands, executeSlashCommand, onSend, addToHistory])

  const handleKeyDown = (e: React.KeyboardEvent<HTMLInputElement>) => {
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
      if (e.key === 'Enter' || e.key === 'Tab') {
        e.preventDefault()
        if (filteredCommands[activeSlashIndex]) {
          executeSlashCommand(filteredCommands[activeSlashIndex])
        }
        return
      }
      if (e.key === 'Escape') {
        e.preventDefault()
        setShowSlashPopup(false)
        return
      }
    }

    if (e.key === 'Enter') {
      e.preventDefault()
      handleSend()
    } else if (e.key === 'ArrowUp') {
      if (history.length > 0) {
        e.preventDefault()
        const nextIdx = Math.min(historyIdx + 1, history.length - 1)
        setHistoryIdx(nextIdx)
        setText(history[nextIdx] || '')
      }
    } else if (e.key === 'ArrowDown') {
      if (historyIdx > 0) {
        e.preventDefault()
        const nextIdx = historyIdx - 1
        setHistoryIdx(nextIdx)
        setText(history[nextIdx] || '')
      } else if (historyIdx === 0) {
        e.preventDefault()
        setHistoryIdx(-1)
        setText('')
      }
    }
  }

  // File upload handler
  const handleFileUpload = async (e: React.ChangeEvent<HTMLInputElement>) => {
    const files = e.target.files
    if (!files || files.length === 0) return

    setUploading(true)
    setUploadError('')

    const formData = new FormData()
    for (let i = 0; i < files.length; i++) {
      formData.append('file', files[i])
    }

    try {
      const token = getAuthToken()
      const res = await fetch(`/api/v1/tasks/${encodeURIComponent(taskId)}/files`, {
        method: 'POST',
        headers: token ? { Authorization: `Bearer ${token}` } : {},
        body: formData,
      })

      if (!res.ok) {
        const errJson = await res.json().catch(() => ({}))
        throw new Error(errJson.error || errJson.message || 'Upload failed')
      }

      const data = await res.json()
      const newUploaded: { name: string; path: string; size: number }[] = []

      if (data.files && Array.isArray(data.files)) {
        for (const f of data.files) {
          newUploaded.push({ name: f.filename, path: f.path, size: f.size })
        }
      } else if (data.filename) {
        newUploaded.push({ name: data.filename, path: data.path, size: data.size })
      }

      setUploadedFiles((prev) => [...prev, ...newUploaded])

      // Automatically append uploaded paths into composer input
      const paths = newUploaded.map((f) => f.path).join(' ')
      setText((prev) => (prev ? `${prev.trim()} ${paths} ` : `${paths} `))
      inputRef.current?.focus()
    } catch (err) {
      setUploadError(err instanceof Error ? err.message : 'Upload failed')
      setTimeout(() => setUploadError(''), 4000)
    } finally {
      setUploading(false)
      if (fileInputRef.current) fileInputRef.current.value = ''
    }
  }

  return (
    <div
      className={`relative shrink-0 select-none bg-[#161616] border-t border-[#262626] px-2.5 py-2 space-y-1.5 ${className}`}
    >
      {/* Hidden file input */}
      <input
        ref={fileInputRef}
        type="file"
        multiple
        onChange={handleFileUpload}
        className="hidden"
        aria-hidden="true"
      />

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
          {usageData.workspace?.branch && (
            <div className="flex items-center gap-1.5 text-[11px] text-neutral-400 font-mono">
              <GitBranch className="h-3 w-3 text-amber-400" />
              <span>Active branch: <strong>{usageData.workspace.branch}</strong></span>
            </div>
          )}
        </div>
      )}

      {/* ── Git Diff Overlay Card ── */}
      {diffData && (
        <div className="absolute left-2.5 right-2.5 bottom-full mb-2 z-40 rounded-xl border border-neutral-700 bg-[#18181b] p-3 shadow-2xl text-xs space-y-2">
          <div className="flex items-center justify-between border-b border-neutral-800 pb-2">
            <div className="flex items-center gap-2 font-semibold text-neutral-200">
              <GitBranch className="h-4 w-4 text-emerald-400" />
              <span>Workspace Git Changes</span>
            </div>
            <button
              type="button"
              onClick={() => setDiffData(null)}
              className="text-neutral-400 hover:text-white p-1 rounded"
            >
              <X className="h-3.5 w-3.5" />
            </button>
          </div>
          {diffData.stat || diffData.status ? (
            <pre className="max-h-48 overflow-y-auto p-2 bg-black/60 rounded-lg text-[11px] font-mono text-neutral-300 leading-relaxed border border-neutral-800 whitespace-pre-wrap">
              {diffData.stat && `=== Diff Stats ===\n${diffData.stat}\n\n`}
              {diffData.status && `=== Changed Files ===\n${diffData.status}`}
            </pre>
          ) : (
            <p className="text-neutral-400 py-2">Workspace clean: tidak ada perubahan file git yang belum di-commit.</p>
          )}
        </div>
      )}

      {/* ── Upload feedback badges ── */}
      {uploadedFiles.length > 0 && (
        <div className="flex items-center gap-1.5 flex-wrap px-1">
          {uploadedFiles.map((f, i) => (
            <span
              key={i}
              className="inline-flex items-center gap-1 px-2 py-0.5 rounded-full bg-emerald-500/15 border border-emerald-500/30 text-emerald-400 text-[11px] font-mono"
            >
              <FileText className="h-3 w-3" />
              <span className="truncate max-w-[120px]">{f.name}</span>
              <button
                type="button"
                onClick={() => setUploadedFiles((prev) => prev.filter((_, idx) => idx !== i))}
                className="hover:text-emerald-200 ml-0.5"
              >
                <X className="h-2.5 w-2.5" />
              </button>
            </span>
          ))}
        </div>
      )}

      {uploadError && (
        <p className="text-xs text-destructive px-1">{uploadError}</p>
      )}

      {/* ── Quick Action Chips Row ── */}
      <div className="flex items-center gap-1 overflow-x-auto select-none [&::-webkit-scrollbar]:hidden [-ms-overflow-style:none] [scrollbar-width:none]">
        {/* Mobile Accessory Keyboard Toggle (Prominently first on mobile) */}
        {onToggleMobileKbd && (
          <button
            type="button"
            disabled={disabled}
            onClick={onToggleMobileKbd}
            className={`inline-flex sm:hidden h-6 shrink-0 items-center gap-1 rounded-md border px-2 text-[11px] font-mono font-semibold transition-colors disabled:opacity-40 ${
              isMobileKbdActive
                ? 'border-emerald-500 bg-emerald-600 text-white shadow-xs'
                : 'border-neutral-700 bg-neutral-800 text-emerald-400 hover:bg-neutral-700 active:bg-neutral-600'
            }`}
            title="Toggle extra virtual keys drawer (ESC, Arrows, CTRL, ALT)"
          >
            <Keyboard className="h-3 w-3" />
            <span>Kbd</span>
          </button>
        )}

        <button
          type="button"
          disabled={disabled}
          onClick={() => handleSend('y')}
          className="inline-flex h-6 shrink-0 items-center gap-1 rounded-md border border-[#333] bg-[#222] px-2 text-[11px] font-mono font-medium text-neutral-200 hover:bg-[#333] active:bg-neutral-700 transition-colors disabled:opacity-40"
          title="Send 'y' (confirm Yes)"
        >
          <Check className="h-2.5 w-2.5 text-emerald-400" />
          <span>y (Yes)</span>
        </button>

        <button
          type="button"
          disabled={disabled}
          onClick={() => handleSend('n')}
          className="inline-flex h-6 shrink-0 items-center gap-1 rounded-md border border-[#333] bg-[#222] px-2 text-[11px] font-mono font-medium text-neutral-200 hover:bg-[#333] active:bg-neutral-700 transition-colors disabled:opacity-40"
          title="Send 'n' (confirm No)"
        >
          <X className="h-2.5 w-2.5 text-rose-400" />
          <span>n (No)</span>
        </button>

        <button
          type="button"
          disabled={disabled}
          onClick={onInterrupt}
          className="inline-flex h-6 shrink-0 items-center gap-1 rounded-md border border-[#333] bg-[#222] px-2 text-[11px] font-mono font-medium text-rose-300 hover:bg-[#333] active:bg-neutral-700 transition-colors disabled:opacity-40"
          title="Send SIGINT (Ctrl+C) to cancel process"
        >
          <Ban className="h-2.5 w-2.5" />
          <span>Ctrl+C</span>
        </button>

        <button
          type="button"
          disabled={disabled}
          onClick={onClearScreen}
          className="inline-flex h-6 shrink-0 items-center gap-1 rounded-md border border-[#333] bg-[#222] px-2 text-[11px] font-mono font-medium text-neutral-300 hover:bg-[#333] active:bg-neutral-700 transition-colors disabled:opacity-40"
          title="Clear screen buffer"
        >
          <Trash2 className="h-2.5 w-2.5 text-neutral-400" />
          <span>Clear</span>
        </button>

        <button
          type="button"
          disabled={disabled}
          onClick={() => onSend('\t')}
          className="inline-flex h-6 shrink-0 items-center gap-1 rounded-md border border-[#333] bg-[#222] px-2 text-[11px] font-mono font-medium text-neutral-300 hover:bg-[#333] active:bg-neutral-700 transition-colors disabled:opacity-40"
          title="Send TAB (autocomplete)"
        >
          <span>Tab</span>
        </button>
      </div>

      {/* ── Inbox-Style Capsule Composer ── */}
      <form
        onSubmit={(e) => {
          e.preventDefault()
          handleSend()
        }}
        className="flex items-center gap-1.5 sm:gap-2 rounded-xl sm:rounded-2xl border border-neutral-700 bg-[#121214] px-2 py-1.5 focus-within:ring-2 focus-within:ring-primary/40 focus-within:border-primary/50 transition-all shadow-inner"
      >
        {/* Upload Button */}
        <button
          type="button"
          disabled={disabled || uploading}
          onClick={() => fileInputRef.current?.click()}
          className="p-1.5 text-neutral-400 hover:text-white hover:bg-neutral-800 rounded-lg transition-colors shrink-0 disabled:opacity-40"
          title="Upload file ke workspace tugas (/upload)"
          aria-label="Upload file"
        >
          {uploading ? (
            <Loader2 className="h-4 w-4 animate-spin text-primary" />
          ) : (
            <Paperclip className="h-4 w-4" />
          )}
        </button>

        {/* Prompt Icon Indicator */}
        <span className="text-emerald-400 font-mono text-sm font-bold select-none shrink-0 pl-0.5">
          &gt;
        </span>

        {/* Text Input */}
        <div className="relative flex-1 min-w-0">
          <input
            ref={inputRef}
            type="text"
            value={text}
            onChange={handleInputChange}
            onKeyDown={handleKeyDown}
            placeholder="Ketik perintah, jawaban (y/n), atau / untuk slash command..."
            disabled={disabled}
            autoCapitalize="none"
            autoCorrect="off"
            autoComplete="off"
            spellCheck={false}
            className="w-full bg-transparent text-xs sm:text-sm font-mono text-neutral-100 placeholder:text-neutral-500 outline-none border-none py-1 px-1 pr-6"
          />
          {text && (
            <button
              type="button"
              onClick={() => {
                setText('')
                setShowSlashPopup(false)
                inputRef.current?.focus()
              }}
              className="absolute right-0 top-1/2 -translate-y-1/2 p-1 text-neutral-400 hover:text-white"
              title="Clear input"
            >
              <X className="h-3.5 w-3.5" />
            </button>
          )}
        </div>

        {/* Send Button */}
        <button
          type="submit"
          disabled={disabled || (!text.trim() && uploadedFiles.length === 0)}
          className="inline-flex h-7 sm:h-8 shrink-0 items-center justify-center gap-1 rounded-lg bg-primary hover:bg-primary/90 active:bg-primary/80 text-primary-foreground px-2.5 sm:px-3 text-xs font-semibold font-mono transition-colors disabled:opacity-35 disabled:cursor-not-allowed shadow-xs"
          title="Kirim ke terminal (Enter)"
          aria-label="Send input"
        >
          <Send className="h-3.5 w-3.5" />
          <span className="hidden sm:inline">Send</span>
        </button>
      </form>
    </div>
  )
}
