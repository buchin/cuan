import { TerminalInputBuffer } from './terminal-input-buffer'
import { TerminalHelperTextareaInputFilter, terminalInputInvalidatesHelperTextarea } from './terminal/mobile-input'
import { createPortal } from 'react-dom'
import { useCallback, useEffect, useRef, useState } from 'react'
import {
  Copy,
  Download,
  Search,
  ArrowDown,
  ChevronDown,
  ChevronUp,
  RefreshCw,
  Keyboard,
  MoreHorizontal,
  WifiOff,
  Ban,
  Trash2,
  Maximize2,
  Minimize2,
} from 'lucide-react'
import { useLanguage } from '../../context/LanguageContext'
import { getAuthToken } from '../../api/client'
import { tasksApi, type LiveTaskSession, type TaskLog } from '../../api/tasks'
import type { TaskAttempt } from '../../api/types'
import type { Terminal } from '@xterm/xterm'
import { terminalGrid, TerminalResizeGate } from './terminal/terminal-geometry'
import '@xterm/xterm/css/xterm.css'
import './task-workspace.css'
import { MobileTerminalKeyboard } from './terminal/MobileTerminalKeyboard'
import { TerminalContextMenu } from './terminal/TerminalContextMenu'
import { TerminalComposer } from './terminal/TerminalComposer'
import { formatLogLineForTerminal } from './terminal/ndjsonFormatter'
import { FilePreviewModal } from '../../components/dialogs/FilePreviewModal'
import type { Attachment } from '../inbox/Attachments'

export function TaskTerminal({
  taskId,
  attempt,
  live,
  onRefresh,
  controlsTarget,
  onConnectionChange,
}: {
  taskId: string
  attempt: TaskAttempt
  live: LiveTaskSession | null
  onRefresh: () => void
  controlsTarget?: HTMLElement | null
  onConnectionChange?: (connection: string) => void
}) {
  const { t, language } = useLanguage()
  const menu = useRef<HTMLDetailsElement>(null)
  const mount = useRef<HTMLDivElement>(null)
  const term = useRef<Terminal | null>(null)
  const fitTerminalRef = useRef<() => void>(() => {})
  const socket = useRef<WebSocket | null>(null)
  const inputQueue = useRef(new TerminalInputBuffer())
  const mobileInputFilter = useRef<TerminalHelperTextareaInputFilter | null>(null)
  const inputGeneration = taskId + ':' + attempt.id
  inputQueue.current.bind(inputGeneration)

  const [connection, setConnection] = useState('loading')
  const [error, setError] = useState('')
  const [notice, setNotice] = useState('')
  const [follow, setFollow] = useState(true)
  const [input, setInput] = useState(false)
  const [search, setSearch] = useState('')
  const [match, setMatch] = useState('')
  const [isFullscreen, setIsFullscreen] = useState(false)
  const [attachedSession, setAttachedSession] = useState({ generation: inputGeneration, id: '' })
  const activeSessionId = attachedSession.generation === inputGeneration ? attachedSession.id : ''
  const [contextMenu, setContextMenu] = useState<{
    isOpen: boolean
    x: number
    y: number
  }>({
    isOpen: false,
    x: 0,
    y: 0,
  })
  const [isMobileKeyboardActive, setIsMobileKeyboardActive] = useState(false)
  const [previewAttachment, setPreviewAttachment] = useState<Attachment | null>(null)

  const handlePreviewFile = useCallback(
    (filePath: string) => {
      const clean = filePath.replace(/^['"`(]+|[)'"`]+$/g, '').trim()
      if (!clean) return
      const name = clean.split('/').pop() || clean
      setPreviewAttachment({
        id: clean,
        name,
        type: '',
        size: 0,
        url: `/api/v1/tasks/${encodeURIComponent(taskId)}/files/raw?path=${encodeURIComponent(clean)}`,
      })
    },
    [taskId],
  )

  const following = useRef(true)
  const interactive = useRef(false)
  const searchIndex = useRef(-1)
  const logs = useRef<TaskLog[]>([])
  const retryCount = useRef(0)
  const isReconnecting = useRef(false)
  const reconnectTimer = useRef<ReturnType<typeof setTimeout> | null>(null)
  const refreshTimer = useRef<ReturnType<typeof setTimeout> | null>(null)
  const pollTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null)

  const isTaskActive = ['running', 'queued'].includes(attempt.status)
  const isTaskActiveRef = useRef(isTaskActive)
  isTaskActiveRef.current = isTaskActive

  const isLive =
    !!live &&
    (!live.attempt_id || live.attempt_id === attempt.id) &&
    isTaskActive
  const liveId = isLive
    ? live.session_id
    : activeSessionId || (isTaskActive ? attempt.id || 'active' : '')
  const liveIdRef = useRef(liveId)
  liveIdRef.current = liveId

  const refreshRef = useRef(onRefresh)
  refreshRef.current = onRefresh

  useEffect(() => {
    onConnectionChange?.(connection)
  }, [connection, onConnectionChange])

  const bufferText = useCallback(() => {
    const termInstance = term.current
    if (!termInstance) return logs.current.map((l) => l.line).join('\n')
    const buffer = termInstance.buffer.active
    const lines: string[] = []
    for (let i = 0; i < buffer.length; i++) {
      lines.push(buffer.getLine(i)?.translateToString(true) || '')
    }
    return lines.join('\n').trimEnd()
  }, [])

  const handleCopy = useCallback(async () => {
    try {
      const termInstance = term.current
      const selected = termInstance?.getSelection()
      const textToCopy = selected || bufferText()
      if (!textToCopy) return
      await navigator.clipboard.writeText(textToCopy)
      setNotice(selected ? t('Selected text copied.') : t('Output copied.'))
      setTimeout(() => setNotice(''), 3000)
    } catch {
      setError(t('Could not copy output. Use Download output instead.'))
    }
  }, [bufferText, t])

  const sendKey = useCallback((key: string) => {
    const ws = socket.current
    const terminal = term.current
    if (terminalInputInvalidatesHelperTextarea(key)) mobileInputFilter.current?.reset()
    if (!interactive.current && ws?.readyState !== WebSocket.OPEN) return
    if (ws?.readyState === WebSocket.OPEN && !interactive.current) {
      if (terminal) terminal.options.disableStdin = false
      interactive.current = true
      setInput(true)
      ws.send(JSON.stringify({ type: 'mode', mode: 'interactive' }))
    }
    const send = ws?.readyState === WebSocket.OPEN ? (data: string) => ws.send(JSON.stringify({ type: 'stdin', data })) : undefined
    if (!inputQueue.current.write(inputGeneration, key, send)) setError(t('Terminal input is pending. Reconnect to the same session before typing more.'))
    if (send && following.current) terminal?.scrollToBottom()
  }, [inputGeneration, t])

  const probeTerminalAvailability = useCallback(async (): Promise<number | null> => {
    try {
      const probeUrl = new URL(
        `/api/v1/tasks/${encodeURIComponent(taskId)}/terminal`,
        window.location.href,
      )
      probeUrl.searchParams.set('attach', '1')
      if (liveIdRef.current) {
        probeUrl.searchParams.set('session_id', liveIdRef.current)
      }
      const token = getAuthToken()
      const res = await fetch(probeUrl.toString(), {
        method: 'GET',
        headers: token ? { Authorization: `Bearer ${token}` } : {},
      })
      return res.status
    } catch {
      return null
    }
  }, [taskId])

  const handleSteer = useCallback(
    async (promptText: string) => {
      const textToSend = promptText.trim()
      if (!textToSend) return

      // 1. Deliver to task & runner steering API endpoint (/api/v1/tasks/:id/steer)
      // Note: Direct PTY keystrokes are handled by onSend in TerminalComposer.
      if (isTaskActiveRef.current) {
        const steerTarget = taskId || attempt?.id
        if (steerTarget) {
          try {
            const res = await tasksApi.steer(steerTarget, textToSend)
            if (res.status === 'delivered') {
              setNotice(t('Steering instruction delivered to agent.'))
            } else {
              setNotice(t('Steering instruction queued for agent.'))
            }
            setTimeout(() => setNotice(''), 3000)
          } catch {
            if (taskId && attempt?.id && steerTarget !== attempt.id) {
              try {
                const res = await tasksApi.steer(attempt.id, textToSend)
                if (res.status === 'delivered') {
                  setNotice(t('Steering instruction delivered to agent.'))
                } else {
                  setNotice(t('Steering instruction queued for agent.'))
                }
                setTimeout(() => setNotice(''), 3000)
                return
              } catch {}
            }
            setError(t('Could not deliver steering instruction.'))
            setTimeout(() => setError(''), 4000)
          }
        }
      }
    },
    [taskId, attempt?.id, sendKey, t],
  )

  const handleInterrupt = useCallback(() => {
    sendKey('\x03')
  }, [sendKey])

  const handleStop = useCallback(async () => {
    handleInterrupt()
    if (isTaskActiveRef.current && taskId) {
      try {
        setNotice(t('Stopping task execution…'))
        await tasksApi.cancel(taskId)
        setNotice(t('Task execution stopped.'))
        setTimeout(() => setNotice(''), 3000)
        refreshRef.current?.()
      } catch {
        // Transient error
      }
    }
  }, [handleInterrupt, taskId, t])

  const handlePaste = useCallback(async () => {
    try {
      const text = await navigator.clipboard.readText()
      if (text) {
        sendKey(text)
      }
    } catch {
      setError(t('Clipboard access denied by browser.'))
      setTimeout(() => setError(''), 4000)
    }
  }, [sendKey, t])

  const handleClear = useCallback(() => {
    term.current?.clear()
    sendKey('\x0c')
  }, [sendKey])

  const handleSelectAll = useCallback(() => {
    term.current?.selectAll()
  }, [])

  const handleContextMenu = useCallback((e: React.MouseEvent) => {
    e.preventDefault()
    setContextMenu({
      isOpen: true,
      x: e.clientX,
      y: e.clientY,
    })
  }, [])

  const find = useCallback(
    (direction = 1) => {
      const termInstance = term.current
      if (!termInstance || !search) {
        termInstance?.clearSelection()
        setMatch('')
        return
      }
      const matches: { line: number; column: number }[] = []
      const query = search.toLowerCase()
      for (let line = 0; line < termInstance.buffer.active.length; line++) {
        const text =
          termInstance.buffer.active
            .getLine(line)
            ?.translateToString(true)
            .toLowerCase() || ''
        let from = 0
        let index: number
        while ((index = text.indexOf(query, from)) >= 0) {
          matches.push({ line, column: index })
          from = index + Math.max(1, query.length)
        }
      }
      if (!matches.length) {
        termInstance.clearSelection()
        setMatch('No matches')
        return
      }
      searchIndex.current =
        (searchIndex.current + direction + matches.length) % matches.length
      const target = matches[searchIndex.current]
      termInstance.select(target.column, target.line, search.length)
      termInstance.scrollToLine(Math.max(0, target.line - 2))
      setMatch(`${searchIndex.current + 1} of ${matches.length}`)
      following.current = false
      setFollow(false)
    },
    [search],
  )

  useEffect(() => {
    searchIndex.current = -1
    find(1)
  }, [search, find])

  const showKeyboard = useCallback(() => {
    setIsMobileKeyboardActive(true)
    const ws = socket.current
    const terminal = term.current
    if (ws?.readyState !== WebSocket.OPEN || !terminal) return
    if (!interactive.current) {
      terminal.options.disableStdin = false
      interactive.current = true
      setInput(true)
      ws.send(JSON.stringify({ type: 'mode', mode: 'interactive' }))
    }
    terminal.focus()
  }, [])

  const hideMobileKeyboard = useCallback(() => {
    setIsMobileKeyboardActive(false)
    term.current?.blur()
  }, [])

  // Ref to hold connectSocket so reconnect and event listeners can call it
  const connectSocketRef = useRef<() => void>(() => {})
  const pollNextLogsRef = useRef<() => void>(() => {})

  const handleReconnect = useCallback(() => {
    setError('')
    setNotice('')
    if (reconnectTimer.current) clearTimeout(reconnectTimer.current)
    retryCount.current = 0
    isReconnecting.current = true
    setConnection(isTaskActiveRef.current ? 'streaming' : 'connecting')
    onRefresh()
    connectSocketRef.current()
  }, [onRefresh])

  // Handle active / online recovery
  const lastAutoRetry = useRef(0)
  useEffect(() => {
    const handleActive = () => {
      if (document.visibilityState !== 'visible') return
      if (!isTaskActiveRef.current) return
      // If task is actively streaming or connected, maintain calm Things 3 serenity
      if (connection === 'streaming' || connection === 'connected') return
      if (connection !== 'disconnected' && connection !== 'error') return

      const now = Date.now()
      if (now - lastAutoRetry.current < 2000) return
      lastAutoRetry.current = now

      handleReconnect()
    }

    document.addEventListener('visibilitychange', handleActive)
    window.addEventListener('focus', handleActive)
    window.addEventListener('online', handleActive)
    return () => {
      document.removeEventListener('visibilitychange', handleActive)
      window.removeEventListener('focus', handleActive)
      window.removeEventListener('online', handleActive)
    }
  }, [connection, handleReconnect])

  // Fullscreen resize re-fitting
  useEffect(() => {
    const timer = setTimeout(() => {
      fitTerminalRef.current()
    }, 60)
    return () => clearTimeout(timer)
  }, [isFullscreen])

  // Terminal lifecycle & WebSocket connection
  useEffect(() => {
    let disposed = false
    let resize: ResizeObserver | undefined
    let cleanupTouch: (() => void) | undefined
    let terminal: Terminal | undefined
    let linkDisposable: { dispose: () => void } | undefined
    let ptyResizeTimer: ReturnType<typeof setTimeout> | null = null
    let fitTimer: ReturnType<typeof setTimeout> | null = null
    const resizeGate = new TerminalResizeGate()

    setConnection('loading')
    setError('')
    setNotice('')
    setInput(false)
    interactive.current = false
    logs.current = []
    following.current = true
    setFollow(true)

    const connectSocket = () => {
      if (disposed) return
      const currentLiveId = liveIdRef.current

      // If task is not active and we don't have a liveId, nothing to attach to
      if (!isTaskActiveRef.current && !currentLiveId) return

      if (socket.current) {
        try {
          socket.current.close()
        } catch {}
        socket.current = null
      }

      // In decoupled runner architecture, default active tasks to calm 'streaming' status
      if (isTaskActiveRef.current) {
        setConnection('streaming')
      } else {
        setConnection('connecting')
      }
      const url = new URL(
        `/api/v1/tasks/${encodeURIComponent(taskId)}/terminal`,
        window.location.href,
      )
      url.protocol = location.protocol === 'https:' ? 'wss:' : 'ws:'
      url.searchParams.set('mode', 'interactive')
      url.searchParams.set('output_ack', '1')
      if (currentLiveId) {
        url.searchParams.set('attach', '1')
        url.searchParams.set('session_id', currentLiveId)
      }
      if (isReconnecting.current) {
        url.searchParams.set('reconnect', '1')
      }
      if (term.current) {
        url.searchParams.set('cols', String(term.current.cols))
        url.searchParams.set('rows', String(term.current.rows))
      }

      const token = getAuthToken()
      const ws = new WebSocket(
        url.toString(),
        token ? ['cuan-terminal', `token.${token}`] : ['cuan-terminal'],
      )
      socket.current = ws
      ws.binaryType = 'arraybuffer'
      let handshakeReceived = false
      let acknowledgeOutput = false
      let outputSequence = 0

      ws.onmessage = (event) => {
        if (disposed || socket.current !== ws || !term.current) return
        const t = term.current

        if (event.data instanceof ArrayBuffer) {
          const sequence = ++outputSequence
          // Binary frames are raw PTY bytes, including legitimate JSON output.
          // Preserve UTF-8 and ANSI sequences across frames for xterm to decode.
          t.write(new Uint8Array(event.data), () => {
            if (following.current) t.scrollToBottom()
            if (acknowledgeOutput) {
              // Yield between parsed output frames so keyboard events can run.
              setTimeout(() => {
                if (!disposed && socket.current === ws && ws.readyState === WebSocket.OPEN) {
                  ws.send(JSON.stringify({ type: 'output_ack', sequence }))
                }
              }, 0)
            }
          })
          return
        }

        try {
          const rawText = String(event.data)
          const data = JSON.parse(rawText)
          if (data.type === 'connected') {
            // One handshake per socket. A repeated control frame must not
            // erase the rendered screen or disturb an active IME composition.
            if (handshakeReceived) return
            handshakeReceived = true
            acknowledgeOutput = data.output_ack === true
            const sameSession = Boolean(data.session_id && data.session_id === liveIdRef.current)
            const retainedInput = sameSession ? t.textarea?.value || '' : ''
            if (data.session_id) {
              liveIdRef.current = data.session_id
              setAttachedSession({ generation: inputGeneration, id: data.session_id })
            }

            // Always reset terminal on connect so no stale ghost lines or duplicate logs remain
            t.reset()
            // Screen replay must not discard Android's pending composition.
            // Retain IME context only for the same PTY, never a new session.
            if (sameSession) {
              if (t.textarea) t.textarea.value = retainedInput
              mobileInputFilter.current?.sync()
            } else {
              mobileInputFilter.current?.reset()
            }
            isReconnecting.current = false
            retryCount.current = 0

            // Disarm incremental database log poller while live PTY is connected
            if (pollTimerRef.current) {
              clearTimeout(pollTimerRef.current)
              pollTimerRef.current = null
            }

            const enabled = data.mode === 'interactive'
            interactive.current = enabled
            setInput(enabled)
            t.options.disableStdin = !enabled
            t.options.cursorBlink = false
            if (enabled) inputQueue.current.flush(inputGeneration, data => ws.send(JSON.stringify({ type: 'stdin', data })))
            setConnection('connected')
            setNotice('')
            setError('')
            fitTerminalRef.current()
          } else if (data.type === 'status') {
            const enabled = data.mode === 'interactive'
            interactive.current = enabled
            setInput(enabled)
            t.options.disableStdin = !enabled
            t.options.cursorBlink = false
          } else if (data.type === 'error') {
            setError(data.message || 'Terminal request failed.')
          }
          // Control frames (stdin echo, pong, etc.) are consumed without writing to terminal canvas
        } catch {
          // Non-JSON string data: direct raw string output over terminal stream (filter out any leaked stdin JSON control text)
          const raw = String(event.data)
          if (
            !raw.includes('"type":"stdin"') &&
            !raw.includes('"type": "stdin"') &&
            !raw.includes('", "data": "}{') &&
            !raw.includes('{"type":')
          ) {
            term.current?.write(raw)
          }
        }
      }

      ws.onerror = () => {
        if (!disposed && socket.current === ws) {
          if (isTaskActiveRef.current) {
            setConnection('streaming')
          } else {
            setConnection('disconnected')
          }
        }
      }

      ws.onclose = async (event) => {
        if (disposed || socket.current !== ws) return
        const retainInput = interactive.current && isTaskActiveRef.current && event.code !== 1000
        if (!retainInput) {
          interactive.current = false
          setInput(false)
          if (term.current) term.current.options.disableStdin = true
        } else {
          setNotice(t('Reconnecting. Unsent input will wait for this terminal session.'))
        }

        if (isTaskActiveRef.current) {
          // Check if interactive endpoint returned 404 (decoupled runner daemon)
          const httpStatus = await probeTerminalAvailability()
          if (disposed || socket.current !== ws) return
          if (httpStatus === 404) {
            interactive.current = false
            setInput(false)
            if (term.current) term.current.options.disableStdin = true
            // Decoupled runner daemon: quiet live log stream fallback without panicky reconnect loop
            setConnection('streaming')
            setNotice(t('Terminal session unavailable. Reconnect before entering more input.'))
            setError('')
            isReconnecting.current = false
            retryCount.current = 0
            if (reconnectTimer.current) {
              clearTimeout(reconnectTimer.current)
              reconnectTimer.current = null
            }
            if (!pollTimerRef.current) {
              pollTimerRef.current = setTimeout(() => pollNextLogsRef.current(), 300)
            }
            return
          }

          // If real unexpected drop on active task, retry quietly while maintaining streaming view
          if (event.code !== 1000 && retryCount.current < 5) {
            const delay = Math.min(
              1000 * Math.pow(1.5, retryCount.current),
              5000,
            )
            retryCount.current++
            isReconnecting.current = true
            setConnection('streaming')
            if (reconnectTimer.current) clearTimeout(reconnectTimer.current)
            reconnectTimer.current = setTimeout(() => {
              if (!disposed && isTaskActiveRef.current) {
                refreshRef.current?.()
                connectSocket()
              }
            }, delay)
            return
          }

          setConnection(retainInput ? 'disconnected' : 'streaming')
          setNotice(retainInput ? t('Connection paused. Reconnect to send pending input to this session.') : '')
        } else {
          setConnection('saved')
          if (refreshTimer.current) clearTimeout(refreshTimer.current)
          refreshTimer.current = setTimeout(() => refreshRef.current(), 300)
        }
      }
    }

    connectSocketRef.current = connectSocket

    const initTerminal = async () => {
      try {
        const [{ Terminal: XtermTerminal }, { FitAddon }, { WebLinksAddon }] =
          await Promise.all([
            import('@xterm/xterm'),
            import('@xterm/addon-fit'),
            import('@xterm/addon-web-links'),
          ])

        if (disposed || !mount.current) return

        const isMobileScreen =
          typeof window !== 'undefined' && window.innerWidth < 480
        const terminalFontSize = isMobileScreen ? 12 : 13

        // Font gate: wait for terminal font face to be ready before computing metrics
        try {
          if (typeof document !== 'undefined' && document.fonts?.load) {
            await document.fonts.load(`${terminalFontSize}px "Geist Mono Variable"`)
          }
        } catch {}

        if (disposed || !mount.current) return

        terminal = new XtermTerminal({
          fontFamily: "'Geist Mono Variable', ui-monospace, SFMono-Regular, Menlo, monospace",
          fontSize: terminalFontSize,
          lineHeight: 1.2,
          letterSpacing: 0,
          scrollback: 10000,
          convertEol: true,
          disableStdin: true,
          cursorBlink: false,
          cursorStyle: 'bar',
          cursorInactiveStyle: 'none',
          allowProposedApi: false,
          scrollOnUserInput: false,
          reflowCursorLine: true,
          theme: {
            background: '#141414',
            foreground: '#e5e5e5',
            selectionBackground: '#3b82f655',
            cursor: 'transparent',
            cursorAccent: 'transparent',
          },
        })

        const fit = new FitAddon()
        terminal.loadAddon(fit)

        // Clickable web links
        const webLinks = new WebLinksAddon()
        terminal.loadAddon(webLinks)

        // Custom Link Provider for workspace file paths in terminal output
        const fileRegex = /(?:[\w.-]+\/)*[\w.-]+\.(?:md|json|ts|tsx|js|jsx|go|py|html|css|txt|csv|png|jpg|jpeg|svg|webp|gif|sh|yml|yaml|sql)\b/gi
        try {
          linkDisposable = terminal.registerLinkProvider({
            provideLinks(bufferLineNumber: number, callback: (links: any[] | undefined) => void) {
              const line = terminal?.buffer.active.getLine(bufferLineNumber - 1)
              if (!line) {
                callback(undefined)
                return
              }
              const lineText = line.translateToString(true)
              fileRegex.lastIndex = 0
              const found: any[] = []
              let m: RegExpExecArray | null
              while ((m = fileRegex.exec(lineText)) !== null) {
                const matched = m[0]
                if (matched.startsWith('http://') || matched.startsWith('https://')) continue
                found.push({
                  range: {
                    start: { x: m.index + 1, y: bufferLineNumber },
                    end: { x: m.index + matched.length, y: bufferLineNumber },
                  },
                  text: matched,
                  decorations: { underline: true, pointerCursor: true },
                  activate: (_e: MouseEvent, text: string) => handlePreviewFile(text),
                })
              }
              callback(found.length > 0 ? found : undefined)
            },
          })
        } catch {}

        terminal.open(mount.current)
        term.current = terminal
        mobileInputFilter.current = navigator.maxTouchPoints > 0
          ? new TerminalHelperTextareaInputFilter(() => terminal?.textarea)
          : null

        // Prevent iOS Safari auto-zoom on textarea focus
        if (terminal.textarea) {
          terminal.textarea.style.fontSize = '16px'
          terminal.textarea.setAttribute('autocorrect', 'off')
          terminal.textarea.setAttribute('autocapitalize', 'none')
          terminal.textarea.setAttribute('spellcheck', 'false')
        }

        // Match Fural's single measured-cell path. Mixing fit() with a
        // second manual resize alternates the PTY grid and triggers TUI redraws.
        const fitTerminal = () => {
          if (disposed || !terminal || !mount.current?.clientWidth || !mount.current?.clientHeight) return
          try {
            const cell = (terminal as any)._core?._renderService?.dimensions?.css?.cell
            const measured = cell ? terminalGrid(mount.current.clientWidth, mount.current.clientHeight, cell.width, cell.height) : null
            const next = measured || fit.proposeDimensions()
            if (!next || next.cols < 2 || next.rows < 1) return
            if (terminal.cols !== next.cols || terminal.rows !== next.rows) terminal.resize(next.cols, next.rows)
            if (ptyResizeTimer) clearTimeout(ptyResizeTimer)
            ptyResizeTimer = setTimeout(() => {
              const ws = socket.current
              if (disposed || ws?.readyState !== WebSocket.OPEN || !terminal) return
              resizeGate.send(ws, { cols: terminal.cols, rows: terminal.rows }, grid => ws.send(JSON.stringify({ type: 'resize', ...grid })))
            }, 80)
          } catch {}
        }
        const scheduleFit = () => {
          if (fitTimer) clearTimeout(fitTimer)
          if (ptyResizeTimer) clearTimeout(ptyResizeTimer)
          fitTimer = setTimeout(fitTerminal, 100)
        }
        fitTerminalRef.current = scheduleFit
        fitTerminal()
        resize = new ResizeObserver(scheduleFit)
        resize.observe(mount.current)

        // Touch scrolling & inertia handling
        const mountEl = mount.current
        let touchStartY = 0
        let touchStartX = 0
        let lastTouchY = 0
        let accumulatedY = 0
        let isTouchDragging = false
        let momentumRaf: number | null = null
        let lastTouchTime = 0
        let touchVelocityY = 0

        const cancelMomentum = () => {
          if (momentumRaf !== null) {
            cancelAnimationFrame(momentumRaf)
            momentumRaf = null
          }
        }

        const getRowHeight = () => {
          if (terminal && terminal.rows > 0 && mountEl.clientHeight > 0) {
            const computed = mountEl.clientHeight / terminal.rows
            if (computed > 4 && computed < 100) return computed
          }
          const fontSize = (terminal?.options.fontSize as number) || 13
          const lineHeight = (terminal?.options.lineHeight as number) || 1.25
          return fontSize * lineHeight || 16.25
        }

        const onTouchStart = (e: TouchEvent) => {
          cancelMomentum()
          if (e.touches.length !== 1) return
          const touch = e.touches[0]
          touchStartY = touch.clientY
          touchStartX = touch.clientX
          lastTouchY = touch.clientY
          accumulatedY = 0
          isTouchDragging = false
          lastTouchTime = Date.now()
          touchVelocityY = 0
        }

        const onTouchMove = (e: TouchEvent) => {
          if (e.touches.length !== 1 || !terminal) return
          const touch = e.touches[0]
          const deltaY = touch.clientY - lastTouchY
          const totalY = touch.clientY - touchStartY
          const totalX = touch.clientX - touchStartX

          if (!isTouchDragging) {
            if (Math.abs(totalY) > 3 && Math.abs(totalY) >= Math.abs(totalX)) {
              isTouchDragging = true
            }
          }

          if (isTouchDragging) {
            if (e.cancelable) {
              e.preventDefault()
            }
            const now = Date.now()
            const dt = now - lastTouchTime
            if (dt > 0) {
              const currentVelocity = deltaY / dt
              touchVelocityY = touchVelocityY * 0.4 + currentVelocity * 0.6
            }
            lastTouchTime = now
            lastTouchY = touch.clientY
            accumulatedY += deltaY

            const rowHeight = getRowHeight()
            const lines = Math.trunc(accumulatedY / rowHeight)
            if (lines !== 0) {
              terminal.scrollLines(-lines)
              accumulatedY -= lines * rowHeight
            }
          }
        }

        const onTouchEnd = () => {
          if (!isTouchDragging || !terminal) {
            isTouchDragging = false
            return
          }
          isTouchDragging = false

          if (Date.now() - lastTouchTime > 120) {
            touchVelocityY = 0
            return
          }

          let remainingVelocity = touchVelocityY * 16
          if (Math.abs(remainingVelocity) > 2) {
            let momentumAccumulator = 0
            const step = () => {
              if (disposed || !terminal || Math.abs(remainingVelocity) < 0.3) {
                momentumRaf = null
                return
              }
              momentumAccumulator += remainingVelocity
              const rowHeight = getRowHeight()
              const lines = Math.trunc(momentumAccumulator / rowHeight)
              if (lines !== 0) {
                terminal.scrollLines(-lines)
                momentumAccumulator -= lines * rowHeight
              }
              remainingVelocity *= 0.92
              momentumRaf = requestAnimationFrame(step)
            }
            momentumRaf = requestAnimationFrame(step)
          }
        }

        mountEl.addEventListener('touchstart', onTouchStart, {
          capture: true,
          passive: true,
        })
        mountEl.addEventListener('touchmove', onTouchMove, {
          capture: true,
          passive: false,
        })
        mountEl.addEventListener('touchend', onTouchEnd, {
          capture: true,
          passive: true,
        })
        mountEl.addEventListener('touchcancel', onTouchEnd, {
          capture: true,
          passive: true,
        })

        cleanupTouch = () => {
          mountEl.removeEventListener('touchstart', onTouchStart, {
            capture: true,
          } as EventListenerOptions)
          mountEl.removeEventListener('touchmove', onTouchMove, {
            capture: true,
          } as EventListenerOptions)
          mountEl.removeEventListener('touchend', onTouchEnd, {
            capture: true,
          } as EventListenerOptions)
          mountEl.removeEventListener('touchcancel', onTouchEnd, {
            capture: true,
          } as EventListenerOptions)
          cancelMomentum()
        }

        terminal.onScroll((position) => {
          if (disposed || !terminal) return
          if (position < terminal.buffer.active.baseY) {
            following.current = false
            setFollow(false)
          } else {
            following.current = true
            setFollow(true)
          }
        })

        terminal.onData((data) => {
          const ws = socket.current
          if (!interactive.current) return
          const payload = mobileInputFilter.current ? mobileInputFilter.current.filter(data) : data
          if (payload === null) return
          const send = ws?.readyState === WebSocket.OPEN ? (value: string) => ws.send(JSON.stringify({ type: 'stdin', data: value })) : undefined
          if (!inputQueue.current.write(inputGeneration, payload, send)) setError('Terminal input is pending. Reconnect to the same session before typing more.')
        })

        // Incremental polling loop used exclusively as fallback when PTY socket is unavailable
        let after = 0
        const pollNextLogs = async () => {
          if (disposed) return
          if (!isTaskActiveRef.current) {
            setConnection('saved')
            return
          }
          // Do NOT poll or write database logs while pure PTY WebSocket is actively streaming
          if (socket.current?.readyState === WebSocket.OPEN) {
            return
          }
          try {
            const result = await tasksApi.logs(taskId, attempt.id, after)
            if (disposed) return
            const next = result.logs || []
            const activeTerm = term.current || terminal
            if (next.length > 0 && activeTerm && socket.current?.readyState !== WebSocket.OPEN) {
              logs.current.push(...next)
              activeTerm.write(next.map((l) => formatLogLineForTerminal(l.line)).join('\r\n') + '\r\n')
              if (following.current) activeTerm.scrollToBottom()
              after = next[next.length - 1].id
            }
            const isFinal =
              (result as any).is_final ||
              ['succeeded', 'failed', 'canceled', 'cancelled'].includes(
                (result as any).status || '',
              )
            if (isFinal) {
              setConnection('saved')
              refreshRef.current?.()
              return
            }
          } catch {
            // Ignore transient polling failure
          }

          if (!disposed && isTaskActiveRef.current && socket.current?.readyState !== WebSocket.OPEN) {
            pollTimerRef.current = setTimeout(pollNextLogs, 1000)
          } else {
            if (!isTaskActiveRef.current) setConnection('saved')
          }
        }
        pollNextLogsRef.current = pollNextLogs

        // Always clear any previous poll timer
        if (pollTimerRef.current) {
          clearTimeout(pollTimerRef.current)
          pollTimerRef.current = null
        }

        // If task is active, connect socket directly for pure real-time PTY stream (zero log collision)
        if (isTaskActiveRef.current) {
          connectSocket()
          setConnection('streaming')
        } else {
          // Task is finished: load saved historical output from database
          setConnection('saved')
          for (let page = 0; page < 8 && !disposed; page++) {
            const result = await tasksApi.logs(taskId, attempt.id, after)
            if (disposed) return
            const next = result.logs || []
            logs.current.push(...next)
            if (next.length) {
              terminal.write(next.map((l) => formatLogLineForTerminal(l.line)).join('\r\n') + '\r\n')
              if (following.current) terminal.scrollToBottom()
              after = next[next.length - 1].id
            }
            if (next.length < 250) break
          }
          if (!logs.current.length) {
            setNotice(t('No output was saved for this session.'))
          }
        }
      } catch (e) {
        if (!disposed) {
          setConnection('error')
          setError(
            e instanceof Error ? e.message : 'Could not load saved output.',
          )
        }
      }
    }

    void initTerminal()

    return () => {
      disposed = true
      cleanupTouch?.()
      resize?.disconnect()
      linkDisposable?.dispose()
      if (ptyResizeTimer) clearTimeout(ptyResizeTimer)
      if (fitTimer) clearTimeout(fitTimer)
      if (reconnectTimer.current) clearTimeout(reconnectTimer.current)
      if (refreshTimer.current) clearTimeout(refreshTimer.current)
      if (pollTimerRef.current) clearTimeout(pollTimerRef.current)
      if (socket.current) {
        socket.current.close()
        socket.current = null
      }
      terminal?.dispose()
      mobileInputFilter.current = null
      if (term.current === terminal) term.current = null
      fitTerminalRef.current = () => {}
    }
  }, [taskId, attempt.id])

  // Context menu click-outside / escape
  useEffect(() => {
    const outside = (event: PointerEvent) => {
      if (menu.current?.open && !menu.current.contains(event.target as Node)) {
        menu.current.open = false
      }
    }
    const escape = (event: KeyboardEvent) => {
      if (event.key === 'Escape') {
        if (menu.current?.open) {
          menu.current.open = false
          menu.current.querySelector<HTMLElement>('summary')?.focus()
        }
        if (isFullscreen) {
          setIsFullscreen(false)
        }
      }
    }
    document.addEventListener('pointerdown', outside)
    document.addEventListener('keydown', escape)
    return () => {
      document.removeEventListener('pointerdown', outside)
      document.removeEventListener('keydown', escape)
    }
  }, [isFullscreen])

  // Eliminate false "Terminal disconnected" yellow banners when task is actively running.
  // When runner daemon executes decoupled, xterm gracefully consumes the live log stream without disruption.
  const showDisconnectBanner =
    !isTaskActive &&
    (connection === 'disconnected' || connection === 'connecting') &&
    retryCount.current > 0 &&
    !logs.current.length

  const controls = (
    <div
      className={
        controlsTarget && !isFullscreen
          ? 'flex items-center gap-1.5'
          : 'flex shrink-0 items-center justify-between gap-1.5 bg-background px-2 py-1'
      }
    >
      <div className="flex items-center gap-1.5 min-w-0">
        {isTaskActive && (connection === 'streaming' || connection === 'connected') ? (
          <span
            className="inline-flex items-center gap-1.5 rounded-full px-2 py-0.5 text-[11px] font-medium bg-emerald-500/10 text-emerald-400 border border-emerald-500/20 shadow-2xs"
            role="status"
          >
            <span className="h-1.5 w-1.5 rounded-full bg-emerald-500 animate-pulse" />
            <span>{language === 'id' ? '• Berjalan' : '• Live'}</span>
          </span>
        ) : (
          <span
            role="status"
            className={
              controlsTarget && !isFullscreen
                ? 'sr-only'
                : 'min-w-0 flex-1 truncate text-xs text-muted-foreground'
            }
          >
            {
              (
                {
                  loading: t('Loading output…'),
                  connecting: t('Connecting to existing session…'),
                  connected: input
                    ? t('Connected · input enabled')
                    : t('Connected · read-only'),
                  streaming: t('Streaming live logs…'),
                  saved: t('Saved output'),
                  disconnected: t(
                    'Disconnected · task status is checked separately',
                  ),
                  error: t('Output unavailable'),
                } as Record<string, string>
              )[connection]
            }
          </span>
        )}
        {(!controlsTarget || isFullscreen) && !(isTaskActive && (connection === 'streaming' || connection === 'connected')) && (
          <span
            aria-hidden
            className={
              'mx-1 h-1.5 w-1.5 shrink-0 rounded-full ' +
              (connection === 'connected' || connection === 'streaming'
                ? 'bg-emerald-500 animate-pulse'
                : connection === 'saved'
                  ? 'bg-muted-foreground'
                  : 'bg-amber-500')
            }
            title={connection}
          />
        )}
      </div>

      {/* Quick Action Toolbar */}
      <div className="flex flex-wrap items-center gap-1">
        {/* Quick Stop / Interrupt (Ctrl+C) - Active without legacy liveId gate */}
        {isTaskActive && (
          <button
            type="button"
            className="icon-action text-rose-400 hover:bg-rose-500/20 hover:text-rose-300 active:bg-rose-600 active:text-white"
            aria-label={t('Send Ctrl+C (Interrupt)')}
            title={t('Send Ctrl+C (Interrupt)')}
            onClick={handleStop}
          >
            <Ban className="h-3.5 w-3.5" />
          </button>
        )}

        {/* Clear buffer */}
        <button
          type="button"
          className="icon-action"
          aria-label={t('Clear terminal')}
          title={t('Clear terminal')}
          onClick={handleClear}
        >
          <Trash2 className="h-3.5 w-3.5" />
        </button>

        {/* Fullscreen / Maximize toggle */}
        <button
          type="button"
          className="icon-action"
          aria-label={isFullscreen ? t('Exit fullscreen') : t('Fullscreen')}
          title={isFullscreen ? t('Exit fullscreen') : t('Fullscreen')}
          onClick={() => setIsFullscreen((prev) => !prev)}
        >
          {isFullscreen ? (
            <Minimize2 className="h-3.5 w-3.5" />
          ) : (
            <Maximize2 className="h-3.5 w-3.5" />
          )}
        </button>

        {/* Mobile keyboard trigger - Active without legacy liveId gate */}
        <button
          type="button"
          className="icon-action sm:hidden"
          aria-label={t('Show keyboard')}
          title={t('Show keyboard')}
          onClick={showKeyboard}
        >
          <Keyboard className="h-4 w-4" />
        </button>
      </div>

      {/* Details dropdown for extra tools */}
      <details ref={menu} className="static sm:relative shrink-0">
        <summary
          aria-label={t('Terminal tools')}
          title={t('Terminal tools')}
          className="icon-action cursor-pointer list-none [&::-webkit-details-marker]:hidden"
        >
          <MoreHorizontal className="h-4 w-4" />
        </summary>
        <div className="absolute right-0 top-full z-20 w-[min(320px,calc(100vw-24px))] space-y-2 rounded-lg border bg-popover p-3 text-popover-foreground shadow-xl">
          <div className="flex flex-wrap justify-end gap-1">
            <button
              type="button"
              className="icon-action"
              aria-label={t('Reconnect terminal')}
              title={t('Reconnect to this session')}
              onClick={handleReconnect}
            >
              <RefreshCw className="h-4 w-4" />
            </button>
            <button
              type="button"
              className="icon-action"
              aria-label={t('Copy output')}
              title={t('Copy output')}
              onClick={handleCopy}
            >
              <Copy className="h-4 w-4" />
            </button>
            <button
              type="button"
              className="icon-action"
              aria-label={t('Download output')}
              title={t('Download output')}
              onClick={() => {
                const url = URL.createObjectURL(
                  new Blob([bufferText()], {
                    type: 'text/plain;charset=utf-8',
                  }),
                )
                const a = document.createElement('a')
                a.href = url
                a.download = `task-${taskId}-session-${attempt.attempt_number}.txt`
                a.click()
                setTimeout(() => URL.revokeObjectURL(url), 1000)
              }}
            >
              <Download className="h-4 w-4" />
            </button>
          </div>
          <div className="flex shrink-0 items-center gap-1">
            <label className="relative min-w-0 flex-1">
              <Search
                aria-hidden
                className="absolute left-2 top-3.5 h-4 w-4 text-muted-foreground"
              />
              <input
                className="field-control pl-8 text-sm"
                aria-label={t('Search output…')}
                placeholder={t('Search output…')}
                value={search}
                onChange={(e) => setSearch(e.target.value)}
                onKeyDown={(e) => {
                  if (e.key === 'Enter') {
                    e.preventDefault()
                    find(e.shiftKey ? -1 : 1)
                  }
                }}
              />
            </label>
            {search && (
              <>
                <span
                  role="status"
                  className="max-w-20 text-xs text-muted-foreground"
                >
                  {match}
                </span>
                <button
                  type="button"
                  className="icon-action"
                  aria-label={t('Previous match')}
                  onClick={() => find(-1)}
                >
                  <ChevronUp className="h-4 w-4" />
                </button>
                <button
                  type="button"
                  className="icon-action"
                  aria-label={t('Next match')}
                  onClick={() => find(1)}
                >
                  <ChevronDown className="h-4 w-4" />
                </button>
              </>
            )}
            <button
              type="button"
              className="action-secondary px-2 text-xs"
              aria-pressed={follow}
              onClick={() => {
                const next = !follow
                following.current = next
                setFollow(next)
                if (next) term.current?.scrollToBottom()
              }}
            >
              <ArrowDown className="h-4 w-4" />
              {t('Follow')}
            </button>
          </div>
        </div>
      </details>
    </div>
  )

  return (
    <section
      aria-label={t('Session terminal')}
      className={
        isFullscreen
          ? 'fixed inset-0 z-50 flex h-full w-full flex-col bg-[#141414] overscroll-contain select-text'
          : 'task-terminal-container flex h-full min-h-0 min-w-0 flex-col bg-[#141414] overscroll-contain'
      }
    >
      {controlsTarget && !isFullscreen
        ? createPortal(controls, controlsTarget)
        : controls}

      {showDisconnectBanner && (
        <div
          role="status"
          className="flex shrink-0 flex-wrap items-center justify-between gap-2 border-b border-amber-500/30 bg-amber-500/10 px-3 py-2 text-xs text-amber-200"
        >
          <div className="flex min-w-0 items-center gap-2">
            <WifiOff className="h-4 w-4 shrink-0 text-amber-400" />
            <div className="min-w-0 flex-1">
              <p className="font-medium text-amber-200">
                {connection === 'connecting'
                  ? t('Reconnecting to terminal…')
                  : t(
                      'Terminal disconnected. The task is still running in the background.',
                    )}
              </p>
              <p className="text-[11px] text-amber-300/80">
                {connection === 'connecting'
                  ? t('Connecting to active session…')
                  : t('Auto-retry when tab becomes active')}
              </p>
            </div>
          </div>
          <button
            type="button"
            disabled={connection === 'connecting'}
            onClick={handleReconnect}
            className="inline-flex shrink-0 items-center gap-1.5 rounded-md bg-amber-500 px-3 py-1.5 text-xs font-semibold text-black transition-colors hover:bg-amber-400 disabled:opacity-50"
            aria-label={language === 'id' ? 'Sambungkan Ulang' : t('Reconnect')}
          >
            <RefreshCw
              className={`h-3.5 w-3.5 ${connection === 'connecting' ? 'animate-spin' : ''}`}
            />
            {connection === 'connecting'
              ? t('Connecting…')
              : language === 'id'
                ? 'Sambungkan Ulang'
                : t('Reconnect')}
          </button>
        </div>
      )}

      {error && !showDisconnectBanner && (
        <p
          role="alert"
          className="shrink-0 px-3 py-1.5 text-xs text-destructive"
        >
          {error}
        </p>
      )}
      {notice && (
        <p
          role="status"
          className="shrink-0 px-3 py-1 text-xs text-muted-foreground"
        >
          {notice}
        </p>
      )}

      <div
        className={`min-h-0 min-w-0 flex-1 overflow-hidden bg-[#141414] p-1.5 sm:p-2 overscroll-contain flex flex-col ${
          !isTaskActive && !Boolean(activeSessionId || live?.session_id)
            ? 'pb-[max(0.75rem,env(safe-area-inset-bottom))]'
            : ''
        }`}
        aria-label="Terminal output"
        onContextMenu={handleContextMenu}
      >
        <div
          ref={mount}
          className="task-terminal-viewport h-full w-full overscroll-contain touch-none block"
        />
      </div>

      {/* Right-click & touch context menu */}
      <TerminalContextMenu
        isOpen={contextMenu.isOpen}
        x={contextMenu.x}
        y={contextMenu.y}
        hasSelection={Boolean(term.current?.getSelection())}
        selectedText={term.current?.getSelection() || ''}
        isFullscreen={isFullscreen}
        onClose={() => setContextMenu((c) => ({ ...c, isOpen: false }))}
        onCopy={handleCopy}
        onPaste={handlePaste}
        onSelectAll={handleSelectAll}
        onClear={handleClear}
        onInterrupt={handleInterrupt}
        onToggleFullscreen={() => setIsFullscreen((prev) => !prev)}
        onPreviewFile={handlePreviewFile}
        t={t}
      />

      {/* Inline File Preview Modal for files clicked or requested in workspace */}
      {previewAttachment && (
        <FilePreviewModal
          open={Boolean(previewAttachment)}
          attachment={previewAttachment}
          onClose={() => setPreviewAttachment(null)}
          onDownload={() => {
            window.open(
              `/api/v1/tasks/${encodeURIComponent(taskId)}/files/raw?path=${encodeURIComponent(previewAttachment.id)}&download=1`,
              '_blank',
            )
          }}
        />
      )}

      {/* Universal Modern Terminal Composer Capsule (Desktop & Mobile) - Active with safe-area padding */}
      {(isTaskActive || Boolean(activeSessionId || live?.session_id)) && (
        <TerminalComposer
          taskId={taskId}
          attemptId={attempt.id}
          onSend={sendKey}
          onSteer={handleSteer}
          onStop={handleStop}
          onClearScreen={handleClear}
          onInterrupt={handleInterrupt}
          onToggleMobileKbd={() => setIsMobileKeyboardActive((prev) => !prev)}
          isMobileKbdActive={isMobileKeyboardActive}
          disabled={false}
          className={
            isMobileKeyboardActive
              ? 'pb-2'
              : 'pb-[max(0.75rem,env(safe-area-inset-bottom))]'
          }
        />
      )}

      {/* Mobile Accessory Keyboard Bar (Blink & Developer style) - Conditional on active focus/toggle with safe-area padding */}
      {liveId && isMobileKeyboardActive && (
        <MobileTerminalKeyboard
          onInject={sendKey}
          onCopy={handleCopy}
          onPaste={handlePaste}
          onClear={handleClear}
          onToggleKeyboard={showKeyboard}
          onClose={hideMobileKeyboard}
          disabled={false}
          compact
          className="pb-[max(0.75rem,env(safe-area-inset-bottom))]"
        />
      )}

      <p className="sr-only">
        {activeSessionId || live?.session_id
          ? 'Closing this view does not stop the task. Use Cancel session to stop execution.'
          : 'Saved output is bounded and redacted. This view does not start a process.'}
      </p>
    </section>
  )
}
