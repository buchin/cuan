import { useEffect, useRef, useState, useCallback } from 'react'
import { Terminal } from '@xterm/xterm'
import { FitAddon } from '@xterm/addon-fit'
import { WebLinksAddon } from '@xterm/addon-web-links'
import '@xterm/xterm/css/xterm.css'
import { Button } from '../../ui/button'
import { Badge } from '../../ui/badge'
import {
  Terminal as TerminalIcon,
  RefreshCw,
  Copy,
  Check,
  Trash2,
  ExternalLink,
  RotateCcw,
  CheckCircle2,
  AlertCircle
} from 'lucide-react'
import { ScriptDeploymentDTO, SupervisedProcessStatus, ScriptDeployLogEntry } from '../types'

interface ScriptDeployerConsoleProps {
  projectId: string
  deployment?: ScriptDeploymentDTO | null
  process?: SupervisedProcessStatus | null
  initialLogs?: ScriptDeployLogEntry[]
  isBuilding?: boolean
  onRestart?: () => Promise<void> | void
  onOpenLiveApp?: () => void
  liveAppUrl?: string
  className?: string
}

export function ScriptDeployerConsole({
  projectId,
  deployment,
  process,
  initialLogs = [],
  isBuilding = false,
  onRestart,
  onOpenLiveApp,
  liveAppUrl,
  className = '',
}: ScriptDeployerConsoleProps) {
  const containerRef = useRef<HTMLDivElement>(null)
  const termRef = useRef<Terminal | null>(null)
  const fitAddonRef = useRef<FitAddon | null>(null)
  const wsRef = useRef<WebSocket | null>(null)
  const sseRef = useRef<EventSource | null>(null)
  const reconnectTimeoutRef = useRef<any>(null)
  const isMountedRef = useRef(true)

  const [connectionStatus, setConnectionStatus] = useState<'connecting' | 'connected' | 'polling' | 'disconnected'>('connecting')
  const [copied, setCopied] = useState(false)
  const [logsCount, setLogsCount] = useState(0)
  const [rawLogs, setRawLogs] = useState<ScriptDeployLogEntry[]>(initialLogs)

  // Keep a maximum buffer of 1000 lines to prevent memory leak
  const MAX_LOG_BUFFER = 1000

  // Format log entry into terminal ANSI line
  const formatLogLine = useCallback((entry: ScriptDeployLogEntry): string => {
    const timeStr = entry.timestamp
      ? new Date(entry.timestamp).toLocaleTimeString([], { hour12: false })
      : new Date().toLocaleTimeString([], { hour12: false })

    let prefix = `\x1b[90m[${timeStr}]\x1b[0m `
    if (entry.stream === 'stderr') {
      prefix += `\x1b[1;31m[STDERR]\x1b[0m `
    } else if (entry.stream === 'system') {
      prefix += `\x1b[1;36m[SYSTEM]\x1b[0m `
    } else {
      prefix += `\x1b[1;32m[STDOUT]\x1b[0m `
    }

    const message = entry.message || ''
    // Ensure newline if not present
    return `${prefix}${message}\r\n`
  }, [])

  // Initialize Terminal
  useEffect(() => {
    isMountedRef.current = true
    if (!containerRef.current) return

    const term = new Terminal({
      theme: {
        background: '#090d16',
        foreground: '#e2e8f0',
        cursor: '#38bdf8',
        selectionBackground: 'rgba(56, 189, 248, 0.3)',
        black: '#0f172a',
        red: '#f43f5e',
        green: '#10b981',
        yellow: '#f59e0b',
        blue: '#3b82f6',
        magenta: '#a855f7',
        cyan: '#06b6d4',
        white: '#f8fafc',
      },
      fontFamily: 'ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, "Liberation Mono", "Courier New", monospace',
      fontSize: 12,
      lineHeight: 1.4,
      scrollback: MAX_LOG_BUFFER,
      cursorBlink: false,
      disableStdin: true, // Read-only streaming console
      convertEol: true,
    })

    const fitAddon = new FitAddon()
    const webLinksAddon = new WebLinksAddon()

    term.loadAddon(fitAddon)
    term.loadAddon(webLinksAddon)

    term.open(containerRef.current)
    try {
      fitAddon.fit()
    } catch {}

    termRef.current = term
    fitAddonRef.current = fitAddon

    // Initial banner
    term.write('\x1b[1;36m=== Cuan 1-Click Script Deployer & Live Build Console ===\x1b[0m\r\n')
    term.write(`\x1b[90mProject ID: ${projectId} | Connecting to live build streamer...\x1b[0m\r\n\r\n`)

    const handleResize = () => {
      try {
        fitAddon.fit()
      } catch {}
    }
    window.addEventListener('resize', handleResize)

    return () => {
      isMountedRef.current = false
      window.removeEventListener('resize', handleResize)
      term.dispose()
      termRef.current = null
      fitAddonRef.current = null
    }
  }, [projectId])

  // Append a log entry to terminal
  const appendLog = useCallback((entry: ScriptDeployLogEntry) => {
    if (!termRef.current) return

    setRawLogs((prev) => {
      const next = [...prev, entry]
      if (next.length > MAX_LOG_BUFFER) {
        return next.slice(next.length - MAX_LOG_BUFFER)
      }
      return next
    })
    setLogsCount((prev) => prev + 1)

    termRef.current.write(formatLogLine(entry))
  }, [formatLogLine])

  // Setup streaming (WebSocket with fallback to SSE and polling)
  useEffect(() => {
    let ws: WebSocket | null = null
    let es: EventSource | null = null
    let pollInterval: any = null

    const connectWS = () => {
      if (!isMountedRef.current) return
      setConnectionStatus('connecting')

      const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:'
      const wsUrl = `${protocol}//${window.location.host}/api/v1/projects/${projectId}/script-deploy/logs`

      try {
        ws = new WebSocket(wsUrl)
        wsRef.current = ws

        ws.onopen = () => {
          if (!isMountedRef.current) return
          setConnectionStatus('connected')
          if (termRef.current) {
            termRef.current.write('\x1b[32m✔ Connected to live WebSocket stream\x1b[0m\r\n')
          }
        }

        ws.onmessage = (event) => {
          if (!isMountedRef.current) return
          try {
            const data = JSON.parse(event.data)
            if (data.type === 'init' && Array.isArray(data.logs)) {
              data.logs.forEach((item: ScriptDeployLogEntry) => appendLog(item))
            } else if (data.type === 'log' && data.entry) {
              appendLog(data.entry)
            } else if (data.message) {
              appendLog(data)
            }
          } catch {
            appendLog({
              stream: 'stdout',
              message: event.data,
              timestamp: new Date().toISOString(),
            })
          }
        }

        ws.onerror = () => {
          // Switch to SSE fallback
          if (ws) ws.close()
          fallbackToSSE()
        }

        ws.onclose = () => {
          if (isMountedRef.current && connectionStatus === 'connected') {
            setConnectionStatus('disconnected')
          }
        }
      } catch {
        fallbackToSSE()
      }
    }

    const fallbackToSSE = () => {
      if (!isMountedRef.current) return
      if (sseRef.current) sseRef.current.close()

      const sseUrl = `/api/v1/projects/${projectId}/script-deploy/logs?stream=true`
      try {
        es = new EventSource(sseUrl)
        sseRef.current = es

        es.onopen = () => {
          if (!isMountedRef.current) return
          setConnectionStatus('connected')
          if (termRef.current) {
            termRef.current.write('\x1b[32m✔ Connected to Server-Sent Events stream\x1b[0m\r\n')
          }
        }

        es.onmessage = (event) => {
          if (!isMountedRef.current) return
          try {
            const data = JSON.parse(event.data)
            appendLog(data)
          } catch {
            appendLog({
              stream: 'stdout',
              message: event.data,
              timestamp: new Date().toISOString(),
            })
          }
        }

        es.onerror = () => {
          if (es) es.close()
          fallbackToPolling()
        }
      } catch {
        fallbackToPolling()
      }
    }

    const fallbackToPolling = () => {
      if (!isMountedRef.current) return
      setConnectionStatus('polling')

      const fetchSnapshot = async () => {
        try {
          const res = await fetch(`/api/v1/projects/${projectId}/script-deploy/logs?limit=100`)
          if (res.ok) {
            const data = await res.json()
            if (Array.isArray(data.logs)) {
              setRawLogs(data.logs)
              setLogsCount(data.logs.length)
              if (termRef.current) {
                termRef.current.clear()
                data.logs.forEach((l: ScriptDeployLogEntry) => {
                  termRef.current?.write(formatLogLine(l))
                })
              }
            }
          }
        } catch {}
      }

      fetchSnapshot()
      pollInterval = setInterval(fetchSnapshot, 3000)
    }

    connectWS()

    return () => {
      if (ws) ws.close()
      if (es) es.close()
      if (pollInterval) clearInterval(pollInterval)
      if (reconnectTimeoutRef.current) clearTimeout(reconnectTimeoutRef.current)
    }
  }, [projectId, appendLog, formatLogLine])

  // Clear Terminal
  const handleClear = () => {
    if (termRef.current) {
      termRef.current.clear()
      termRef.current.write('\x1b[90m-- Log terminal dibersihkan --\x1b[0m\r\n')
    }
  }

  // Copy Logs
  const handleCopyLogs = () => {
    const text = rawLogs.map((l) => `[${l.timestamp}] [${l.stream}] ${l.message}`).join('\n')
    navigator.clipboard.writeText(text)
    setCopied(true)
    setTimeout(() => setCopied(false), 2000)
  }

  // Effective status badge
  const isHealthy = process?.health === 'healthy' || deployment?.health === 'healthy'
  const isRunning = process?.status === 'running' || deployment?.build_status === 'ready'
  const isFailed = process?.status === 'crashed' || deployment?.build_status === 'failed'

  const effectiveAppUrl =
    liveAppUrl ||
    (deployment?.domain ? `https://${deployment.domain}` : '') ||
    (process?.domain ? `https://${process.domain}` : '') ||
    (deployment?.port ? `http://127.0.0.1:${deployment.port}` : '')

  return (
    <div className={`flex flex-col bg-slate-950 border border-slate-800 rounded-2xl overflow-hidden shadow-2xl ${className}`}>
      {/* Console Top Toolbar */}
      <div className="flex flex-wrap items-center justify-between gap-2 px-4 py-3 bg-slate-900 border-b border-slate-800 text-xs">
        {/* Left: Terminal Title & Status Badge */}
        <div className="flex items-center gap-2.5">
          <div className="flex items-center gap-1.5 font-mono font-bold text-slate-200">
            <TerminalIcon className="h-4 w-4 text-cyan-400" />
            <span>Live Build Console</span>
          </div>

          {/* Live Status Badge */}
          {isBuilding ? (
            <Badge className="bg-amber-500/20 text-amber-400 border border-amber-500/40 gap-1 animate-pulse text-[11px]">
              <RefreshCw className="h-3 w-3 animate-spin" />
              <span>Building...</span>
            </Badge>
          ) : isHealthy && isRunning ? (
            <Badge className="bg-emerald-500/20 text-emerald-400 border border-emerald-500/40 gap-1 text-[11px]">
              <span className="h-2 w-2 rounded-full bg-emerald-400 animate-ping" />
              <span>Healthy (HTTP 200)</span>
            </Badge>
          ) : isRunning ? (
            <Badge className="bg-blue-500/20 text-blue-400 border border-blue-500/40 gap-1 text-[11px]">
              <CheckCircle2 className="h-3 w-3" />
              <span>Running</span>
            </Badge>
          ) : isFailed ? (
            <Badge className="bg-rose-500/20 text-rose-400 border border-rose-500/40 gap-1 text-[11px]">
              <AlertCircle className="h-3 w-3" />
              <span>Build Failed</span>
            </Badge>
          ) : (
            <Badge variant="outline" className="text-slate-400 border-slate-700 text-[11px]">
              <span>Idle / Stopped</span>
            </Badge>
          )}

          {/* Connection state */}
          <span className="text-[10px] text-slate-500 hidden sm:inline">
            ({connectionStatus})
          </span>
        </div>

        {/* Right: Actions */}
        <div className="flex items-center gap-1.5">
          {effectiveAppUrl && (isRunning || isHealthy) && (
            <a
              href={effectiveAppUrl}
              target="_blank"
              rel="noopener noreferrer"
              onClick={onOpenLiveApp}
              className="inline-flex items-center gap-1.5 px-3 py-1 rounded-lg text-xs font-semibold bg-emerald-600 hover:bg-emerald-500 text-white transition-colors shadow-xs"
            >
              <ExternalLink className="h-3.5 w-3.5" />
              <span>Open Live App</span>
            </a>
          )}

          {onRestart && (
            <Button
              variant="outline"
              size="sm"
              onClick={onRestart}
              className="h-7 text-xs gap-1 border-slate-700 bg-slate-800 text-slate-300 hover:text-white"
              title="Restart supervised process"
            >
              <RotateCcw className="h-3 w-3" />
              <span className="hidden sm:inline">Restart</span>
            </Button>
          )}

          <Button
            variant="outline"
            size="sm"
            onClick={handleCopyLogs}
            className="h-7 text-xs gap-1 border-slate-700 bg-slate-800 text-slate-300 hover:text-white px-2"
            title="Salin semua log"
          >
            {copied ? <Check className="h-3 w-3 text-emerald-400" /> : <Copy className="h-3 w-3" />}
          </Button>

          <Button
            variant="outline"
            size="sm"
            onClick={handleClear}
            className="h-7 text-xs border-slate-700 bg-slate-800 text-slate-400 hover:text-rose-400 px-2"
            title="Bersihkan layar konsol"
          >
            <Trash2 className="h-3 w-3" />
          </Button>
        </div>
      </div>

      {/* Terminal Viewport */}
      <div
        ref={containerRef}
        className="w-full h-80 sm:h-96 p-3 font-mono text-xs overflow-hidden"
      />

      {/* Console Bottom Info Bar */}
      <div className="flex items-center justify-between px-4 py-2 bg-slate-900/80 border-t border-slate-800/80 text-[11px] text-slate-400 font-mono">
        <div className="flex items-center gap-3">
          <span>Port: {process?.port || deployment?.port || 'Allocating...'}</span>
          <span>Domain: {process?.domain || deployment?.domain || 'auto.cuan.dojo.cc'}</span>
        </div>
        <div className="flex items-center gap-2">
          <span>{logsCount} baris log</span>
          <span className="text-slate-600">•</span>
          <span className="text-cyan-400">Stream Buffer Active</span>
        </div>
      </div>
    </div>
  )
}
