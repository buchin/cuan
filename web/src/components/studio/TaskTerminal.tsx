import { useEffect, useRef, useState, useCallback } from 'react'
import { Terminal } from '@xterm/xterm'
import { FitAddon } from '@xterm/addon-fit'
import { WebLinksAddon } from '@xterm/addon-web-links'
import '@xterm/xterm/css/xterm.css'
import { Button } from '../ui/button'
import { Badge } from '../ui/badge'
import {
  Terminal as TerminalIcon,
  Eye,
  Keyboard,
  RefreshCw,
  Maximize2,
  Minimize2,
  Trash2,
  AlertCircle,
  Copy,
  Check,
  Zap,
  ShieldAlert
} from 'lucide-react'

export type TerminalMode = 'watch' | 'interactive'
export type ConnectionState = 'connecting' | 'connected' | 'reconnecting' | 'disconnected'

interface TaskTerminalProps {
  taskId: string
  taskTitle?: string
  initialMode?: TerminalMode
  onClose?: () => void
  isCompact?: boolean
  className?: string
}

export function TaskTerminal({
  taskId,
  taskTitle,
  initialMode = 'watch',
  onClose,
  isCompact = false,
  className = '',
}: TaskTerminalProps) {
  const containerRef = useRef<HTMLDivElement>(null)
  const termRef = useRef<Terminal | null>(null)
  const fitAddonRef = useRef<FitAddon | null>(null)
  const wsRef = useRef<WebSocket | null>(null)
  const reconnectTimeoutRef = useRef<any>(null)
  const pingIntervalRef = useRef<any>(null)

  const [mode, setMode] = useState<TerminalMode>(initialMode)
  const [connectionState, setConnectionState] = useState<ConnectionState>('connecting')
  const [isFullscreen, setIsFullscreen] = useState(false)
  const [copied, setCopied] = useState(false)
  const [terminalDimensions, setTerminalDimensions] = useState<{ cols: number; rows: number }>({ cols: 80, rows: 24 })
  const [errorMessage, setErrorMessage] = useState<string | null>(null)
  const [reconnectCount, setReconnectCount] = useState(0)

  // Sync mode state ref for event handlers
  const modeRef = useRef<TerminalMode>(mode)
  useEffect(() => {
    modeRef.current = mode
  }, [mode])

  // Handle Mode Change (Watch vs Interactive Take-Over)
  const handleModeChange = useCallback((newMode: TerminalMode) => {
    setMode(newMode)
    modeRef.current = newMode

    if (wsRef.current && wsRef.current.readyState === WebSocket.OPEN) {
      if (newMode === 'interactive') {
        wsRef.current.send(JSON.stringify({ type: 'takeover' }))
      } else {
        wsRef.current.send(JSON.stringify({ type: 'mode', mode: 'watch' }))
      }
    }

    if (termRef.current) {
      if (newMode === 'interactive') {
        termRef.current.write('\r\n\x1b[1;33m[Interactive Mode Activated]\x1b[0m Anda dapat mengetik perintah langsung ke shell PTY workspace.\r\n$ ')
        termRef.current.focus()
      } else {
        termRef.current.write('\r\n\x1b[1;36m[Watch Mode Activated]\x1b[0m Terminal beralih ke mode observasi stream (read-only).\r\n')
      }
    }
  }, [])

  // Clear Terminal
  const handleClear = useCallback(() => {
    if (termRef.current) {
      termRef.current.clear()
    }
  }, [])

  // Copy Selected Text
  const handleCopySelection = useCallback(() => {
    if (termRef.current) {
      const selection = termRef.current.getSelection()
      if (selection) {
        navigator.clipboard.writeText(selection)
        setCopied(true)
        setTimeout(() => setCopied(false), 2000)
      } else {
        // Select all text if nothing selected
        termRef.current.selectAll()
        const all = termRef.current.getSelection()
        if (all) {
          navigator.clipboard.writeText(all)
          setCopied(true)
          setTimeout(() => setCopied(false), 2000)
        }
        termRef.current.clearSelection()
      }
    }
  }, [])

  // Connect WebSocket
  const connectWebSocket = useCallback(() => {
    if (!taskId) return

    if (wsRef.current) {
      try {
        wsRef.current.close()
      } catch {}
      wsRef.current = null
    }

    setConnectionState((prev) => (prev === 'disconnected' ? 'reconnecting' : 'connecting'))
    setErrorMessage(null)

    const loc = window.location
    const protocol = loc.protocol === 'https:' ? 'wss:' : 'ws:'
    const host = loc.host
    const token = localStorage.getItem('cuan_token') || ''

    const cols = termRef.current ? termRef.current.cols : 80
    const rows = termRef.current ? termRef.current.rows : 24

    const queryParams = new URLSearchParams()
    queryParams.set('mode', modeRef.current)
    queryParams.set('cols', cols.toString())
    queryParams.set('rows', rows.toString())
    if (token) {
      queryParams.set('token', token)
    }

    const wsUrl = `${protocol}//${host}/api/tasks/${encodeURIComponent(taskId)}/terminal?${queryParams.toString()}`

    try {
      const protocols = token ? ['cuan-terminal', `token.${token}`] : ['cuan-terminal']
      const ws = new WebSocket(wsUrl, protocols)
      ws.binaryType = 'arraybuffer'
      wsRef.current = ws

      ws.onopen = () => {
        setConnectionState('connected')
        setReconnectCount(0)
        setErrorMessage(null)

        // Fit & notify terminal size
        if (fitAddonRef.current && termRef.current) {
          try {
            fitAddonRef.current.fit()
            const c = termRef.current.cols
            const r = termRef.current.rows
            setTerminalDimensions({ cols: c, rows: r })
            ws.send(JSON.stringify({ type: 'resize', cols: c, rows: r }))
          } catch {}
        }

        // Start ping heartbeat
        if (pingIntervalRef.current) clearInterval(pingIntervalRef.current)
        pingIntervalRef.current = setInterval(() => {
          if (ws.readyState === WebSocket.OPEN) {
            ws.send(JSON.stringify({ type: 'ping' }))
          }
        }, 15000)
      }

      ws.onmessage = (event) => {
        if (!termRef.current) return

        if (event.data instanceof ArrayBuffer) {
          const uint8 = new Uint8Array(event.data)
          termRef.current.write(uint8)
        } else if (typeof event.data === 'string') {
          try {
            const parsed = JSON.parse(event.data)
            if (parsed.type === 'connected') {
              if (parsed.mode && (parsed.mode === 'watch' || parsed.mode === 'interactive')) {
                setMode(parsed.mode)
                modeRef.current = parsed.mode
              }
              if (parsed.cols && parsed.rows) {
                setTerminalDimensions({ cols: parsed.cols, rows: parsed.rows })
              }
            } else if (parsed.type === 'status') {
              if (parsed.mode && (parsed.mode === 'watch' || parsed.mode === 'interactive')) {
                setMode(parsed.mode)
                modeRef.current = parsed.mode
              }
            } else if (parsed.type === 'error') {
              setErrorMessage(parsed.message || 'Terminal server error')
              termRef.current.write(`\r\n\x1b[31;1m[Terminal Error]\x1b[0m ${parsed.message || 'Unknown error'}\r\n`)
            }
          } catch {
            // Not a JSON message, write raw string output
            termRef.current.write(event.data)
          }
        }
      }

      ws.onerror = (err) => {
        console.error('[TaskTerminal] WebSocket error:', err)
        setErrorMessage('Gagal terhubung ke PTY session task.')
      }

      ws.onclose = (event) => {
        if (pingIntervalRef.current) {
          clearInterval(pingIntervalRef.current)
          pingIntervalRef.current = null
        }

        if (event.wasClean) {
          setConnectionState('disconnected')
        } else {
          setConnectionState('reconnecting')
          // Auto-reconnect with backoff (up to 5 tries)
          setReconnectCount((prev) => {
            const next = prev + 1
            if (next <= 5) {
              const delay = Math.min(1000 * Math.pow(1.5, next), 8000)
              reconnectTimeoutRef.current = setTimeout(() => {
                connectWebSocket()
              }, delay)
            } else {
              setConnectionState('disconnected')
              setErrorMessage('Koneksi terputus. Klik tombol Sambung Ulang untuk mencoba kembali.')
            }
            return next
          })
        }
      }
    } catch (e: any) {
      setConnectionState('disconnected')
      setErrorMessage(e?.message || 'Inisialisasi WebSocket gagal.')
    }
  }, [taskId])

  // Initialize xterm.js
  useEffect(() => {
    if (!containerRef.current) return

    const term = new Terminal({
      cursorBlink: true,
      cursorStyle: 'block',
      fontFamily: "'JetBrains Mono', 'Fira Code', Menlo, Monaco, Consolas, monospace",
      fontSize: 13,
      lineHeight: 1.25,
      theme: {
        background: '#090d16',
        foreground: '#e2e8f0',
        cursor: '#10b981',
        cursorAccent: '#090d16',
        selectionBackground: 'rgba(56, 189, 248, 0.3)',
        black: '#1e293b',
        red: '#ef4444',
        green: '#10b981',
        yellow: '#f59e0b',
        blue: '#3b82f6',
        magenta: '#d946ef',
        cyan: '#06b6d4',
        white: '#f8fafc',
        brightBlack: '#64748b',
        brightRed: '#f87171',
        brightGreen: '#34d399',
        brightYellow: '#fbbf24',
        brightBlue: '#60a5fa',
        brightMagenta: '#e879f9',
        brightCyan: '#22d3ee',
        brightWhite: '#ffffff',
      },
      convertEol: true,
      scrollback: 5000,
      allowTransparency: false,
    })

    const fitAddon = new FitAddon()
    const webLinksAddon = new WebLinksAddon()

    term.loadAddon(fitAddon)
    term.loadAddon(webLinksAddon)

    term.open(containerRef.current)

    // Initial Welcome Banner
    term.write('\x1b[1;32m╔════════════════════════════════════════════════════════════════════════════╗\x1b[0m\r\n')
    term.write('\x1b[1;32m║\x1b[0m \x1b[1;37mCUAN IN-BROWSER INTERACTIVE PTY TERMINAL (creack/pty + xterm.js)\x1b[0m           \x1b[1;32m║\x1b[0m\r\n')
    term.write('\x1b[1;32m║\x1b[0m \x1b[36mTask ID:\x1b[0m \x1b[1;33m' + taskId.padEnd(65) + '\x1b[0m\x1b[1;32m║\x1b[0m\r\n')
    term.write('\x1b[1;32m║\x1b[0m \x1b[90mWorktree terisolasi pada Git workspace task. Watch Mode aktif secara default.\x1b[0m\x1b[1;32m║\x1b[0m\r\n')
    term.write('\x1b[1;32m╚════════════════════════════════════════════════════════════════════════════╝\x1b[0m\r\n\r\n')

    termRef.current = term
    fitAddonRef.current = fitAddon

    // Handle Keyboard Input
    term.onData((data) => {
      if (modeRef.current === 'interactive') {
        if (wsRef.current && wsRef.current.readyState === WebSocket.OPEN) {
          wsRef.current.send(data)
        }
      } else {
        // In Watch Mode: inform user typing is disabled until takeover
        term.write('\x07') // Bell
        setErrorMessage('Mode saat ini adalah Watch Mode (Read-Only). Klik tombol "Take-Over Interactive" di atas untuk mengetik perintah.')
        setTimeout(() => setErrorMessage(null), 3500)
      }
    })

    // Fit on load
    try {
      fitAddon.fit()
      setTerminalDimensions({ cols: term.cols, rows: term.rows })
    } catch {}

    // Resize Observer for responsive fluid sizing
    const resizeObserver = new ResizeObserver(() => {
      if (fitAddonRef.current && termRef.current) {
        try {
          fitAddonRef.current.fit()
          const c = termRef.current.cols
          const r = termRef.current.rows
          setTerminalDimensions({ cols: c, rows: r })
          if (wsRef.current && wsRef.current.readyState === WebSocket.OPEN) {
            wsRef.current.send(JSON.stringify({ type: 'resize', cols: c, rows: r }))
          }
        } catch {}
      }
    })

    if (containerRef.current) {
      resizeObserver.observe(containerRef.current)
    }

    // Connect WebSocket
    connectWebSocket()

    return () => {
      resizeObserver.disconnect()
      if (pingIntervalRef.current) {
        clearInterval(pingIntervalRef.current)
      }
      if (reconnectTimeoutRef.current) {
        clearTimeout(reconnectTimeoutRef.current)
      }
      if (wsRef.current) {
        try {
          wsRef.current.close()
        } catch {}
      }
      term.dispose()
      termRef.current = null
      fitAddonRef.current = null
    }
  }, [taskId, connectWebSocket])

  // Trigger fit when fullscreen toggles
  useEffect(() => {
    const timer = setTimeout(() => {
      if (fitAddonRef.current && termRef.current) {
        try {
          fitAddonRef.current.fit()
          const c = termRef.current.cols
          const r = termRef.current.rows
          setTerminalDimensions({ cols: c, rows: r })
          if (wsRef.current && wsRef.current.readyState === WebSocket.OPEN) {
            wsRef.current.send(JSON.stringify({ type: 'resize', cols: c, rows: r }))
          }
        } catch {}
      }
    }, 150)
    return () => clearTimeout(timer)
  }, [isFullscreen])

  return (
    <div
      className={`flex flex-col bg-[#090d16] text-slate-100 rounded-xl overflow-hidden border border-slate-800 shadow-2xl transition-all duration-200 ${
        isFullscreen
          ? 'fixed inset-4 z-50 rounded-2xl border-emerald-500/50 shadow-emerald-950/50'
          : className || (isCompact ? 'h-[360px]' : 'h-[500px]')
      }`}
    >
      {/* Terminal Title Bar & Control Header */}
      <div className="flex items-center justify-between px-3 py-2 bg-slate-900/90 border-b border-slate-800/80 backdrop-blur-sm select-none gap-2">
        {/* Left: Window Dots & Title */}
        <div className="flex items-center gap-2.5 min-w-0">
          <div className="flex items-center gap-1.5 shrink-0">
            <span className="h-3 w-3 rounded-full bg-red-500/80 hover:bg-red-500 inline-block transition-colors" />
            <span className="h-3 w-3 rounded-full bg-amber-500/80 hover:bg-amber-500 inline-block transition-colors" />
            <span className="h-3 w-3 rounded-full bg-emerald-500/80 hover:bg-emerald-500 inline-block transition-colors" />
          </div>

          <div className="flex items-center gap-2 min-w-0">
            <TerminalIcon className="h-4 w-4 text-emerald-400 shrink-0" />
            <span className="font-mono text-xs font-bold text-slate-200 truncate">
              {taskTitle || `Task Terminal (#${taskId.slice(0, 8)})`}
            </span>
          </div>
        </div>

        {/* Center: Mode Toggle Switch (Watch Mode vs Interactive Take-Over Mode) */}
        <div className="flex items-center bg-slate-950 p-0.5 rounded-lg border border-slate-800 shrink-0">
          <button
            type="button"
            onClick={() => handleModeChange('watch')}
            className={`flex items-center gap-1.5 px-2.5 py-1 rounded-md text-[11px] font-semibold transition-all ${
              mode === 'watch'
                ? 'bg-blue-600/30 text-blue-400 border border-blue-500/40 shadow-xs'
                : 'text-slate-400 hover:text-slate-200'
            }`}
            title="Watch Mode: Tinjau output pengerjaan agent secara read-only"
          >
            <Eye className="h-3 w-3" />
            <span className="hidden sm:inline">Watch Mode</span>
          </button>

          <button
            type="button"
            onClick={() => handleModeChange('interactive')}
            className={`flex items-center gap-1.5 px-2.5 py-1 rounded-md text-[11px] font-semibold transition-all ${
              mode === 'interactive'
                ? 'bg-emerald-600/30 text-emerald-400 border border-emerald-500/40 shadow-xs'
                : 'text-slate-400 hover:text-slate-200'
            }`}
            title="Interactive Take-Over Mode: Ketik langsung perintah shell ke PTY workspace"
          >
            <Keyboard className="h-3 w-3" />
            <span className="hidden sm:inline">Take-Over</span>
          </button>
        </div>

        {/* Right: Status Indicators & Actions */}
        <div className="flex items-center gap-2 shrink-0">
          {/* WebSocket Status Indicator */}
          {connectionState === 'connected' && (
            <Badge className="bg-emerald-500/10 text-emerald-400 border-emerald-500/30 text-[10px] py-0 px-2 font-mono flex items-center gap-1.5">
              <span className="h-1.5 w-1.5 rounded-full bg-emerald-400 animate-pulse" />
              <span className="hidden md:inline">Connected</span>
            </Badge>
          )}
          {connectionState === 'connecting' && (
            <Badge className="bg-blue-500/10 text-blue-400 border-blue-500/30 text-[10px] py-0 px-2 font-mono flex items-center gap-1.5">
              <RefreshCw className="h-2.5 w-2.5 animate-spin" />
              <span className="hidden md:inline">Connecting</span>
            </Badge>
          )}
          {connectionState === 'reconnecting' && (
            <Badge className="bg-amber-500/10 text-amber-400 border-amber-500/30 text-[10px] py-0 px-2 font-mono flex items-center gap-1.5">
              <RefreshCw className="h-2.5 w-2.5 animate-spin" />
              <span className="hidden md:inline">{reconnectCount > 0 ? `Reconnecting (${reconnectCount})` : 'Reconnecting'}</span>
            </Badge>
          )}
          {connectionState === 'disconnected' && (
            <Badge className="bg-red-500/10 text-red-400 border-red-500/30 text-[10px] py-0 px-2 font-mono flex items-center gap-1.5">
              <span className="h-1.5 w-1.5 rounded-full bg-red-400" />
              <span className="hidden md:inline">Disconnected</span>
            </Badge>
          )}

          {/* Dimension badge */}
          <span className="text-[10px] font-mono text-slate-500 hidden lg:inline">
            {terminalDimensions.cols}x{terminalDimensions.rows}
          </span>

          {/* Action Buttons */}
          <div className="flex items-center gap-1">
            <Button
              variant="ghost"
              size="sm"
              onClick={handleCopySelection}
              className="h-7 w-7 p-0 text-slate-400 hover:text-white hover:bg-slate-800"
              title="Copy selection / buffer"
            >
              {copied ? <Check className="h-3.5 w-3.5 text-emerald-400" /> : <Copy className="h-3.5 w-3.5" />}
            </Button>

            <Button
              variant="ghost"
              size="sm"
              onClick={handleClear}
              className="h-7 w-7 p-0 text-slate-400 hover:text-white hover:bg-slate-800"
              title="Clear terminal buffer"
            >
              <Trash2 className="h-3.5 w-3.5" />
            </Button>

            <Button
              variant="ghost"
              size="sm"
              onClick={connectWebSocket}
              className="h-7 w-7 p-0 text-slate-400 hover:text-white hover:bg-slate-800"
              title="Reconnect terminal"
            >
              <RefreshCw className={`h-3.5 w-3.5 ${connectionState === 'connecting' || connectionState === 'reconnecting' ? 'animate-spin' : ''}`} />
            </Button>

            <Button
              variant="ghost"
              size="sm"
              onClick={() => setIsFullscreen(!isFullscreen)}
              className="h-7 w-7 p-0 text-slate-400 hover:text-white hover:bg-slate-800"
              title={isFullscreen ? 'Keluar Fullscreen' : 'Fullscreen'}
            >
              {isFullscreen ? <Minimize2 className="h-3.5 w-3.5" /> : <Maximize2 className="h-3.5 w-3.5" />}
            </Button>

            {onClose && (
              <Button
                variant="ghost"
                size="sm"
                onClick={onClose}
                className="h-7 w-7 p-0 text-slate-400 hover:text-red-400 hover:bg-slate-800"
                title="Tutup terminal"
              >
                ✕
              </Button>
            )}
          </div>
        </div>
      </div>

      {/* Floating Notice / Error Bar */}
      {errorMessage && (
        <div className="bg-amber-500/10 border-b border-amber-500/30 px-3 py-1.5 flex items-center justify-between text-xs text-amber-300">
          <div className="flex items-center gap-1.5">
            <AlertCircle className="h-3.5 w-3.5 text-amber-400 shrink-0" />
            <span>{errorMessage}</span>
          </div>
          <button
            type="button"
            onClick={() => setErrorMessage(null)}
            className="text-amber-400 hover:text-amber-200 text-xs ml-2"
          >
            ✕
          </button>
        </div>
      )}

      {/* Main Terminal Body */}
      <div className="relative flex-1 w-full h-full p-2 overflow-hidden bg-[#090d16]">
        <div ref={containerRef} className="w-full h-full select-text" />

        {/* Reconnect overlay if disconnected */}
        {connectionState === 'disconnected' && (
          <div className="absolute inset-0 bg-slate-950/80 backdrop-blur-xs flex flex-col items-center justify-center gap-3 p-4 z-10 text-center">
            <ShieldAlert className="h-8 w-8 text-red-400" />
            <div>
              <p className="text-sm font-semibold text-slate-200">Koneksi PTY Terputus</p>
              <p className="text-xs text-slate-400 max-w-sm mt-1">
                Sesi terminal terputus atau proses shell telah selesai. Anda dapat menyambung ulang untuk memulai sesi baru.
              </p>
            </div>
            <Button
              size="sm"
              onClick={connectWebSocket}
              className="bg-emerald-600 hover:bg-emerald-500 text-white font-semibold text-xs gap-1.5"
            >
              <RefreshCw className="h-3.5 w-3.5" />
              <span>Sambung Ulang PTY Terminal</span>
            </Button>
          </div>
        )}
      </div>

      {/* Interactive Helper Footer Bar */}
      <div className="px-3 py-1 bg-slate-950 border-t border-slate-800/80 text-[11px] text-slate-500 flex items-center justify-between font-mono">
        <div className="flex items-center gap-2">
          <span className="flex items-center gap-1 text-slate-400">
            {mode === 'interactive' ? (
              <>
                <Zap className="h-3 w-3 text-emerald-400" />
                <span className="text-emerald-400 font-semibold">Take-Over Active</span> (Direct stdin enabled)
              </>
            ) : (
              <>
                <Eye className="h-3 w-3 text-blue-400" />
                <span>Watch Mode</span> (Read-only stdout/stderr)
              </>
            )}
          </span>
        </div>
        <div className="hidden sm:flex items-center gap-3 text-slate-500 text-[10px]">
          <span>xterm.js v6.0</span>
          <span>•</span>
          <span>PTY: bash/sh</span>
        </div>
      </div>
    </div>
  )
}
