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
import type { FitAddon } from '@xterm/addon-fit'
import '@xterm/xterm/css/xterm.css'
import './task-workspace.css'
import { MobileTerminalKeyboard } from './terminal/MobileTerminalKeyboard'
import { TerminalContextMenu } from './terminal/TerminalContextMenu'
import { TerminalComposer } from './terminal/TerminalComposer'
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
  const fitRef = useRef<FitAddon | null>(null)
  const socket = useRef<WebSocket | null>(null)

  const [connection, setConnection] = useState('loading')
  const [error, setError] = useState('')
  const [notice, setNotice] = useState('')
  const [follow, setFollow] = useState(true)
  const [input, setInput] = useState(false)
  const [search, setSearch] = useState('')
  const [match, setMatch] = useState('')
  const [isFullscreen, setIsFullscreen] = useState(false)
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

  const isTaskActive = ['running', 'queued'].includes(attempt.status)
  const isTaskActiveRef = useRef(isTaskActive)
  isTaskActiveRef.current = isTaskActive

  const isLive =
    !!live &&
    live.attempt_id === attempt.id &&
    isTaskActive
  const liveId = isLive ? live.session_id : ''
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
    if (ws?.readyState === WebSocket.OPEN) {
      if (!interactive.current) {
        if (terminal) terminal.options.disableStdin = false
        interactive.current = true
        setInput(true)
        ws.send(JSON.stringify({ type: 'mode', mode: 'interactive' }))
      }
      ws.send(JSON.stringify({ type: 'stdin', data: key }))
      if (following.current) terminal?.scrollToBottom()
    }
  }, [])

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

  const handleInterrupt = useCallback(() => {
    sendKey('\x03')
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

  const handleReconnect = useCallback(() => {
    setError('')
    setNotice('')
    if (reconnectTimer.current) clearTimeout(reconnectTimer.current)
    retryCount.current = 0
    isReconnecting.current = true
    setConnection('connecting')
    onRefresh()
    connectSocketRef.current()
  }, [onRefresh])

  // Handle active / online recovery
  const lastAutoRetry = useRef(0)
  useEffect(() => {
    const handleActive = () => {
      if (document.visibilityState !== 'visible') return
      if (!isTaskActiveRef.current) return
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
      fitRef.current?.fit()
      const ws = socket.current
      const terminal = term.current
      if (ws?.readyState === WebSocket.OPEN && terminal) {
        ws.send(
          JSON.stringify({
            type: 'resize',
            cols: terminal.cols,
            rows: terminal.rows,
          }),
        )
      }
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
      if (!currentLiveId) return

      if (socket.current) {
        try {
          socket.current.close()
        } catch {}
        socket.current = null
      }

      setConnection('connecting')
      const url = new URL(
        `/api/v1/tasks/${encodeURIComponent(taskId)}/terminal`,
        window.location.href,
      )
      url.protocol = location.protocol === 'https:' ? 'wss:' : 'ws:'
      url.searchParams.set('mode', 'interactive')
      url.searchParams.set('attach', '1')
      url.searchParams.set('session_id', currentLiveId)
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

      ws.onmessage = (event) => {
        if (disposed || !term.current) return
        const t = term.current

        if (event.data instanceof ArrayBuffer) {
          t.write(new Uint8Array(event.data), () => {
            if (following.current) t.scrollToBottom()
          })
          return
        }

        try {
          const data = JSON.parse(String(event.data))
          if (data.type === 'connected') {
            if (data.session_id !== currentLiveId) {
              ws.close()
              setError(
                'The terminal belongs to a different session. Refresh and open the selected session again.',
              )
              return
            }

            // Zero-flicker reset before replay
            if (isReconnecting.current) {
              t.reset()
              isReconnecting.current = false
            }
            retryCount.current = 0

            const enabled = data.mode === 'interactive'
            interactive.current = enabled
            setInput(enabled)
            t.options.disableStdin = !enabled
            t.options.cursorBlink = false
            setConnection('connected')
            setNotice('')
            setError('')
            fitRef.current?.fit()
            if (t && ws.readyState === WebSocket.OPEN) {
              ws.send(
                JSON.stringify({
                  type: 'resize',
                  cols: t.cols,
                  rows: t.rows,
                }),
              )
            }
          } else if (data.type === 'status') {
            const enabled = data.mode === 'interactive'
            interactive.current = enabled
            setInput(enabled)
            t.options.disableStdin = !enabled
            t.options.cursorBlink = false
          } else if (data.type === 'error') {
            setError(data.message || 'Terminal request failed.')
          }
        } catch {
          setError(
            'Unexpected terminal response. Reconnect to the existing session.',
          )
        }
      }

      ws.onerror = () => {
        if (!disposed) {
          setConnection('disconnected')
        }
      }

      ws.onclose = (event) => {
        if (disposed) return
        setConnection('disconnected')
        interactive.current = false
        setInput(false)
        if (term.current) term.current.options.disableStdin = true

        // Soft backoff reconnect if task is still actively running
        if (
          isTaskActiveRef.current &&
          event.code !== 1000 &&
          retryCount.current < 5
        ) {
          const delay = Math.min(
            1000 * Math.pow(2, retryCount.current),
            8000,
          )
          retryCount.current++
          isReconnecting.current = true
          setConnection('connecting')
          if (reconnectTimer.current) clearTimeout(reconnectTimer.current)
          reconnectTimer.current = setTimeout(() => {
            if (!disposed) connectSocket()
          }, delay)
        } else {
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
        fitRef.current = fit
        terminal.loadAddon(fit)

        // Clickable web links
        const webLinks = new WebLinksAddon()
        terminal.loadAddon(webLinks)

        // Custom Link Provider for workspace file paths (code, images, docs) in terminal output
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

        // Prevent iOS Safari auto-zoom on textarea focus
        if (terminal.textarea) {
          terminal.textarea.style.fontSize = '16px'
          terminal.textarea.setAttribute('autocorrect', 'off')
          terminal.textarea.setAttribute('autocapitalize', 'none')
          terminal.textarea.setAttribute('spellcheck', 'false')
        }

        // Live screen cell metrics helper
        const cellMetrics = (t: Terminal) => {
          const screen = t.element?.querySelector('.xterm-screen') as HTMLElement | null
          if (screen && t.cols > 0 && t.rows > 0) {
            try {
              const rect = screen.getBoundingClientRect()
              if (rect.width >= t.cols && rect.height >= t.rows) {
                return {
                  width: rect.width / t.cols,
                  height: rect.height / t.rows,
                }
              }
            } catch {}
          }
          const core = (t as any)._core
          const width = core?._renderService?.dimensions?.css?.cell?.width ?? 0
          const height = core?._renderService?.dimensions?.css?.cell?.height ?? 0
          if (width > 0 && height > 0) return { width, height }
          return { width: terminalFontSize * 0.6, height: terminalFontSize * 1.2 }
        }

        const schedulePtyResize = (cols: number, rows: number) => {
          if (ptyResizeTimer) clearTimeout(ptyResizeTimer)
          ptyResizeTimer = setTimeout(() => {
            const ws = socket.current
            if (ws?.readyState === WebSocket.OPEN && terminal) {
              ws.send(JSON.stringify({ type: 'resize', cols, rows }))
            }
          }, 60)
        }

        // Fural-style precision cell fitting with subpixel cell reclaim
        const fitTerminal = () => {
          if (
            disposed ||
            !mount.current?.clientWidth ||
            !mount.current?.clientHeight ||
            !terminal
          )
            return
          try {
            const widthPx = mount.current.clientWidth
            const heightPx = mount.current.clientHeight
            const { width: cellW, height: cellH } = cellMetrics(terminal)
            if (cellW > 0 && cellH > 0) {
              let cols = Math.max(2, Math.floor(widthPx / cellW))
              let rows = Math.max(1, Math.floor(heightPx / cellH))

              // Fural cell reclaim: if leftover margin > 2/3 cell width/height, absorb extra col/row
              if (widthPx - cols * cellW > cellW * (2 / 3)) {
                cols += 1
              }
              if (heightPx - rows * cellH > cellH * (2 / 3)) {
                rows += 1
              }

              if (terminal.cols !== cols || terminal.rows !== rows) {
                terminal.resize(cols, rows)
              }
            } else {
              fit.fit()
            }
            schedulePtyResize(terminal.cols, terminal.rows)
          } catch {
            /* Detached during responsive navigation */
          }
        }

        fitTerminal()
        resize = new ResizeObserver(fitTerminal)
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
          if (interactive.current && ws?.readyState === WebSocket.OPEN) {
            ws.send(JSON.stringify({ type: 'stdin', data }))
          }
        })

        // If no live session is active, load saved paginated logs
        if (!liveId) {
          let after = 0
          for (let page = 0; page < 8 && !disposed; page++) {
            const result = await tasksApi.logs(taskId, attempt.id, after)
            if (disposed) return
            const next = result.logs || []
            logs.current.push(...next)
            if (next.length) {
              terminal.write(next.map((l) => l.line).join('\r\n') + '\r\n')
            }
            if (next.length < 250) break
            after = next[next.length - 1].id
          }
          if (disposed) return
          setConnection('saved')
          if (!logs.current.length) {
            setNotice(
              ['running', 'queued'].includes(attempt.status)
                ? 'No live terminal is available. This attempt is still recorded as active; refresh its status or cancel it before retrying.'
                : 'No output was saved for this session.',
            )
          }
          return
        }

        // Live session: connect WebSocket
        connectSocket()
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
      if (reconnectTimer.current) clearTimeout(reconnectTimer.current)
      if (refreshTimer.current) clearTimeout(refreshTimer.current)
      if (socket.current) {
        socket.current.close()
        socket.current = null
      }
      terminal?.dispose()
      if (term.current === terminal) term.current = null
      fitRef.current = null
    }
  }, [taskId, attempt.id]) // Recreate terminal only if task or attempt identity changes!

  // Re-connect when liveId transitions from empty to active without recreating Terminal
  useEffect(() => {
    if (liveId && term.current && connection === 'saved') {
      setNotice('')
      setError('')
      connectSocketRef.current()
    }
  }, [liveId, connection])

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

  const showDisconnectBanner =
    isTaskActive &&
    (connection === 'disconnected' ||
      (connection === 'connecting' && retryCount.current > 0) ||
      (connection === 'error' && !logs.current.length))

  const controls = (
    <div
      className={
        controlsTarget && !isFullscreen
          ? 'flex items-center gap-1'
          : 'flex shrink-0 items-center gap-1 bg-background px-2 py-1'
      }
    >
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
              saved: t('Saved output'),
              disconnected: t(
                'Disconnected · task status is checked separately',
              ),
              error: t('Output unavailable'),
            } as Record<string, string>
          )[connection]
        }
      </span>
      {(!controlsTarget || isFullscreen) && (
        <span
          aria-hidden
          className={
            'mx-1 h-1.5 w-1.5 shrink-0 rounded-full ' +
            (connection === 'connected'
              ? 'bg-emerald-500'
              : connection === 'saved'
                ? 'bg-muted-foreground'
                : 'bg-amber-500')
          }
          title={connection}
        />
      )}

      {/* Quick Action Toolbar */}
      <div className="flex flex-wrap items-center gap-1">
        {/* Quick Stop / Interrupt (Ctrl+C) */}
        {liveId && (
          <button
            type="button"
            className="icon-action text-rose-400 hover:bg-rose-500/20 hover:text-rose-300 active:bg-rose-600 active:text-white"
            aria-label={t('Send Ctrl+C (Interrupt)')}
            title={t('Send Ctrl+C (Interrupt)')}
            disabled={connection !== 'connected'}
            onClick={handleInterrupt}
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

        {/* Mobile keyboard trigger */}
        {liveId && (
          <button
            type="button"
            className="icon-action sm:hidden"
            aria-label={t('Show keyboard')}
            title={t('Show keyboard')}
            disabled={connection !== 'connected'}
            onClick={showKeyboard}
          >
            <Keyboard className="h-4 w-4" />
          </button>
        )}
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
        className="min-h-0 min-w-0 flex-1 overflow-hidden bg-[#141414] p-1.5 sm:p-2 overscroll-contain"
        aria-label="Terminal output"
        onContextMenu={handleContextMenu}
      >
        <div
          ref={mount}
          className="task-terminal-viewport h-full w-full overscroll-contain touch-none"
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

      {/* Universal Modern Terminal Composer Capsule (Desktop & Mobile) */}
      {liveId && (
        <TerminalComposer
          taskId={taskId}
          onSend={sendKey}
          onClearScreen={handleClear}
          onInterrupt={handleInterrupt}
          onToggleMobileKbd={() => setIsMobileKeyboardActive((prev) => !prev)}
          isMobileKbdActive={isMobileKeyboardActive}
          disabled={connection !== 'connected'}
        />
      )}

      {/* Mobile Accessory Keyboard Bar (Blink & Developer style) - Conditional on active focus/toggle */}
      {liveId && isMobileKeyboardActive && (
        <MobileTerminalKeyboard
          onInject={sendKey}
          onCopy={handleCopy}
          onPaste={handlePaste}
          onClear={handleClear}
          onToggleKeyboard={showKeyboard}
          onClose={hideMobileKeyboard}
          disabled={connection !== 'connected'}
          compact
        />
      )}

      <p className="sr-only">
        {liveId
          ? 'Closing this view does not stop the task. Use Cancel session to stop execution.'
          : 'Saved output is bounded and redacted. This view does not start a process.'}
      </p>
    </section>
  )
}
