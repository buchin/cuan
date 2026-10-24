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
} from 'lucide-react'
import { getAuthToken } from '../../api/client'
import { tasksApi, type LiveTaskSession, type TaskLog } from '../../api/tasks'
import type { TaskAttempt } from '../../api/types'
import type { Terminal } from '@xterm/xterm'
import '@xterm/xterm/css/xterm.css'
import './task-workspace.css'

export function TaskTerminal({
  taskId,
  attempt,
  live,
  onRefresh,
  controlsTarget,
}: {
  taskId: string
  attempt: TaskAttempt
  live: LiveTaskSession | null
  onRefresh: () => void
  controlsTarget?: HTMLElement | null
}) {
  const menu = useRef<HTMLDetailsElement>(null)
  const mount = useRef<HTMLDivElement>(null),
    term = useRef<Terminal | null>(null),
    socket = useRef<WebSocket | null>(null)
  const [connection, setConnection] = useState('loading'),
    [error, setError] = useState(''),
    [notice, setNotice] = useState(''),
    [revision, setRevision] = useState(0),
    [follow, setFollow] = useState(true),
    [input, setInput] = useState(false),
    [search, setSearch] = useState(''),
    [match, setMatch] = useState('')
  const following = useRef(true),
    interactive = useRef(false),
    searchIndex = useRef(-1),
    logs = useRef<TaskLog[]>([])
  const isLive =
    !!live &&
    live.attempt_id === attempt.id &&
    ['running', 'queued'].includes(attempt.status)
  const liveId = isLive ? live.session_id : ''
  const refreshRef = useRef(onRefresh)
  refreshRef.current = onRefresh
  const bufferText = useCallback(() => {
    const t = term.current
    if (!t) return logs.current.map((l) => l.line).join('\n')
    const buffer = t.buffer.active
    const lines = []
    for (let i = 0; i < buffer.length; i++)
      lines.push(buffer.getLine(i)?.translateToString(true) || '')
    return lines.join('\n').trimEnd()
  }, [])
  const find = useCallback(
    (direction = 1) => {
      const t = term.current
      if (!t || !search) {
        t?.clearSelection()
        setMatch('')
        return
      }
      const matches: { line: number; column: number }[] = []
      const query = search.toLowerCase()
      for (let line = 0; line < t.buffer.active.length; line++) {
        const text =
          t.buffer.active
            .getLine(line)
            ?.translateToString(true)
            .toLowerCase() || ''
        let from = 0,
          index
        while ((index = text.indexOf(query, from)) >= 0) {
          matches.push({ line, column: index })
          from = index + Math.max(1, query.length)
        }
      }
      if (!matches.length) {
        t.clearSelection()
        setMatch('No matches')
        return
      }
      searchIndex.current =
        (searchIndex.current + direction + matches.length) % matches.length
      const target = matches[searchIndex.current]
      t.select(target.column, target.line, search.length)
      t.scrollToLine(Math.max(0, target.line - 2))
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
  useEffect(() => {
    let disposed = false,
      resize: ResizeObserver | undefined,
      refreshTimer: ReturnType<typeof setTimeout> | undefined,
      cleanupTouch: (() => void) | undefined
    let terminal: Terminal | undefined, ws: WebSocket | undefined
    setConnection('loading')
    setError('')
    setNotice('')
    setInput(false)
    interactive.current = false
    logs.current = []
    following.current = true
    setFollow(true)
    const connect = async () => {
      try {
        const [{ Terminal }, { FitAddon }] = await Promise.all([
          import('@xterm/xterm'),
          import('@xterm/addon-fit'),
        ])
        if (disposed || !mount.current) return
        terminal = new Terminal({
          fontFamily: 'ui-monospace, SFMono-Regular, Menlo, monospace',
          fontSize: 13,
          lineHeight: 1.25,
          scrollback: 10000,
          convertEol: true,
          disableStdin: true,
          cursorBlink: false,
          allowProposedApi: false,
          theme: {
            background: '#141414',
            foreground: '#e5e5e5',
            selectionBackground: '#3b82f655',
          },
        })
        const fit = new FitAddon()
        terminal.loadAddon(fit)
        terminal.open(mount.current)
        term.current = terminal
        const fitTerminal = () => {
          if (
            disposed ||
            !mount.current?.clientWidth ||
            !mount.current?.clientHeight
          )
            return
          try {
            fit.fit()
            if (ws?.readyState === WebSocket.OPEN)
              ws.send(
                JSON.stringify({
                  type: 'resize',
                  cols: terminal!.cols,
                  rows: terminal!.rows,
                }),
              )
          } catch {
            /* Detached during responsive navigation. */
          }
        }
        fitTerminal()
        resize = new ResizeObserver(fitTerminal)
        resize.observe(mount.current)

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
          passive: false,
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
          if (interactive.current && ws?.readyState === WebSocket.OPEN)
            ws.send(JSON.stringify({ type: 'stdin', data }))
        })
        if (!liveId) {
          let after = 0
          for (let page = 0; page < 8 && !disposed; page++) {
            const result = await tasksApi.logs(taskId, attempt.id, after)
            if (disposed) return
            const next = result.logs || []
            logs.current.push(...next)
            if (next.length)
              terminal.write(next.map((l) => l.line).join('\r\n') + '\r\n')
            if (next.length < 250) break
            after = next[next.length - 1].id
          }
          if (disposed) return
          setConnection('saved')
          if (!logs.current.length)
            setNotice(
              ['running', 'queued'].includes(attempt.status)
                ? 'No live terminal is available. This attempt is still recorded as active; refresh its status or cancel it before retrying.'
                : 'No output was saved for this session.',
            )
          return
        }
        setConnection('connecting')
        const url = new URL(
          `/api/v1/tasks/${encodeURIComponent(taskId)}/terminal`,
          window.location.href,
        )
        url.protocol = location.protocol === 'https:' ? 'wss:' : 'ws:'
        url.searchParams.set('mode', 'interactive')
        url.searchParams.set('attach', '1')
        url.searchParams.set('session_id', liveId)
        url.searchParams.set('cols', String(terminal.cols))
        url.searchParams.set('rows', String(terminal.rows))
        const token = getAuthToken()
        ws = new WebSocket(
          url.toString(),
          token ? ['cuan-terminal', `token.${token}`] : ['cuan-terminal'],
        )
        socket.current = ws
        ws.binaryType = 'arraybuffer'
        ws.onmessage = (event) => {
          if (disposed || !terminal) return
          if (event.data instanceof ArrayBuffer) {
            terminal.write(new Uint8Array(event.data), () => {
              if (following.current) terminal?.scrollToBottom()
            })
            return
          }
          try {
            const data = JSON.parse(String(event.data))
            if (data.type === 'connected') {
              if (data.session_id !== liveId) {
                ws?.close()
                setError(
                  'The terminal belongs to a different session. Refresh and open the selected session again.',
                )
                return
              }
              const enabled = data.mode === 'interactive'
              interactive.current = enabled
              setInput(enabled)
              terminal.options.disableStdin = !enabled
              terminal.options.cursorBlink = enabled
              setConnection('connected')
              fitTerminal()
            } else if (data.type === 'status') {
              const enabled = data.mode === 'interactive'
              interactive.current = enabled
              setInput(enabled)
              terminal.options.disableStdin = !enabled
              terminal.options.cursorBlink = enabled
            } else if (data.type === 'error')
              setError(data.message || 'Terminal request failed.')
          } catch {
            setError(
              'Unexpected terminal response. Reconnect to the existing session.',
            )
          }
        }
        ws.onerror = () => {
          if (!disposed)
            setError(
              'Could not connect to this terminal. The task was not restarted.',
            )
        }
        ws.onclose = () => {
          if (disposed) return
          setConnection('disconnected')
          interactive.current = false
          setInput(false)
          if (terminal) terminal.options.disableStdin = true
          refreshTimer = setTimeout(() => refreshRef.current(), 300)
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
    void connect()
    return () => {
      disposed = true
      cleanupTouch?.()
      resize?.disconnect()
      if (refreshTimer) clearTimeout(refreshTimer)
      ws?.close()
      if (socket.current === ws) socket.current = null
      terminal?.dispose()
      if (term.current === terminal) term.current = null
    }
  }, [taskId, attempt.id, liveId, revision])
  useEffect(() => {
    const outside = (event: PointerEvent) => {
      if (menu.current?.open && !menu.current.contains(event.target as Node))
        menu.current.open = false
    }
    const escape = (event: KeyboardEvent) => {
      if (event.key === 'Escape' && menu.current?.open) {
        menu.current.open = false
        menu.current.querySelector<HTMLElement>('summary')?.focus()
      }
    }
    document.addEventListener('pointerdown', outside)
    document.addEventListener('keydown', escape)
    return () => {
      document.removeEventListener('pointerdown', outside)
      document.removeEventListener('keydown', escape)
    }
  }, [])
  const showKeyboard = () => {
    const ws = socket.current
    const terminal = term.current
    if (ws?.readyState !== WebSocket.OPEN || !terminal) return
    // Mobile browsers require focus within the original user gesture, not in
    // the later WebSocket response. Input permission still follows server mode.
    if (!interactive.current) {
      terminal.options.disableStdin = false
      ws.send(JSON.stringify({ type: 'mode', mode: 'interactive' }))
    }
    terminal.focus()
  }
  const sendKey = (key: string) => {
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
  }
  const controls = (
    <div
      className={
        controlsTarget
          ? 'flex items-center gap-1'
          : 'flex shrink-0 items-center gap-1 bg-background px-2 py-1'
      }
    >
      <span
        role="status"
        className={
          controlsTarget
            ? 'sr-only'
            : 'min-w-0 flex-1 truncate text-xs text-muted-foreground'
        }
      >
        {
          (
            {
              loading: 'Loading output…',
              connecting: 'Connecting to existing session…',
              connected: input
                ? 'Connected · input enabled'
                : 'Connected · read-only',
              saved: 'Saved output',
              disconnected: 'Disconnected · task status is checked separately',
              error: 'Output unavailable',
            } as Record<string, string>
          )[connection]
        }
      </span>
      {controlsTarget && (
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
      <div className="flex flex-wrap items-center gap-1">
        {liveId && (
          <button
            className="icon-action"
            aria-label="Show keyboard"
            title="Show keyboard"
            disabled={connection !== 'connected'}
            onClick={showKeyboard}
          >
            <Keyboard className="h-4 w-4" />
          </button>
        )}
      </div>
      <details ref={menu} className="relative shrink-0">
        <summary
          aria-label="Terminal tools"
          title="Terminal tools"
          className="icon-action cursor-pointer list-none [&::-webkit-details-marker]:hidden"
        >
          <MoreHorizontal className="h-4 w-4" />
        </summary>
        <div className="absolute right-0 top-full z-20 w-[min(360px,calc(100vw-24px))] space-y-2 rounded-lg border bg-popover p-3 text-popover-foreground shadow-xl">
          <div className="flex flex-wrap justify-end gap-1">
            <button
              className="icon-action"
              aria-label="Reconnect terminal"
              title="Reconnect to this session"
              onClick={() => {
                setRevision((n) => n + 1)
                onRefresh()
              }}
            >
              <RefreshCw className="h-4 w-4" />
            </button>
            <button
              className="icon-action"
              aria-label="Copy output"
              title="Copy output"
              onClick={async () => {
                try {
                  await navigator.clipboard.writeText(bufferText())
                  setNotice('Output copied.')
                } catch {
                  setError(
                    'Could not copy output. Use Download output instead.',
                  )
                }
              }}
            >
              <Copy className="h-4 w-4" />
            </button>
            <button
              className="icon-action"
              aria-label="Download output"
              title="Download output"
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
                aria-label="Search terminal output"
                placeholder="Search output…"
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
                  className="icon-action"
                  aria-label="Previous match"
                  onClick={() => find(-1)}
                >
                  <ChevronUp className="h-4 w-4" />
                </button>
                <button
                  className="icon-action"
                  aria-label="Next match"
                  onClick={() => find(1)}
                >
                  <ChevronDown className="h-4 w-4" />
                </button>
              </>
            )}
            <button
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
              Follow
            </button>
          </div>
        </div>
      </details>
    </div>
  )
  return (
    <section
      aria-label="Session terminal"
      className="task-terminal-container flex h-full min-h-0 min-w-0 flex-col bg-[#141414] overscroll-contain"
    >
      {controlsTarget ? createPortal(controls, controlsTarget) : controls}
      {error && (
        <p role="alert" className="shrink-0 text-sm text-destructive">
          {error}
        </p>
      )}
      {notice && (
        <p role="status" className="shrink-0 text-xs text-muted-foreground">
          {notice}
        </p>
      )}
      <div
        className="min-h-0 min-w-0 flex-1 overflow-hidden bg-[#141414] p-2 overscroll-contain"
        aria-label="Terminal output"
      >
        <div
          ref={mount}
          className="task-terminal-viewport h-full w-full overscroll-contain touch-none"
        />
      </div>
      {liveId && (
        <div
          className="flex shrink-0 items-center gap-1.5 overflow-x-auto border-t border-[#262626] bg-[#161616] px-2 py-1.5 select-none touch-pan-x overscroll-contain [&::-webkit-scrollbar]:hidden [-ms-overflow-style:none] [scrollbar-width:none]"
          aria-label="Terminal keyboard shortcuts"
        >
          <button
            type="button"
            onMouseDown={(e) => e.preventDefault()}
            onClick={() => sendKey('\t')}
            disabled={connection !== 'connected'}
            className="inline-flex h-8 min-w-[38px] shrink-0 items-center justify-center rounded bg-[#262626] px-2 font-mono text-xs font-medium text-neutral-200 transition-colors hover:bg-[#333] active:bg-primary active:text-primary-foreground disabled:opacity-40"
            title="Tab"
          >
            Tab
          </button>
          <button
            type="button"
            onMouseDown={(e) => e.preventDefault()}
            onClick={() => sendKey('\x1b')}
            disabled={connection !== 'connected'}
            className="inline-flex h-8 min-w-[38px] shrink-0 items-center justify-center rounded bg-[#262626] px-2 font-mono text-xs font-medium text-neutral-200 transition-colors hover:bg-[#333] active:bg-primary active:text-primary-foreground disabled:opacity-40"
            title="Escape"
          >
            Esc
          </button>
          <button
            type="button"
            onMouseDown={(e) => e.preventDefault()}
            onClick={() => sendKey('\x03')}
            disabled={connection !== 'connected'}
            className="inline-flex h-8 min-w-[50px] shrink-0 items-center justify-center rounded bg-[#262626] px-2 font-mono text-xs font-medium text-rose-300 transition-colors hover:bg-[#333] active:bg-rose-500 active:text-white disabled:opacity-40"
            title="Ctrl+C (Interrupt)"
          >
            Ctrl+C
          </button>
          <div className="mx-0.5 h-4 w-px shrink-0 bg-[#333]" />
          <button
            type="button"
            onMouseDown={(e) => e.preventDefault()}
            onClick={() => sendKey('\x1b[A')}
            disabled={connection !== 'connected'}
            className="inline-flex h-8 min-w-[36px] shrink-0 items-center justify-center rounded bg-[#262626] px-2 font-mono text-xs font-medium text-neutral-200 transition-colors hover:bg-[#333] active:bg-primary active:text-primary-foreground disabled:opacity-40"
            title="Arrow Up"
            aria-label="Up"
          >
            ↑
          </button>
          <button
            type="button"
            onMouseDown={(e) => e.preventDefault()}
            onClick={() => sendKey('\x1b[B')}
            disabled={connection !== 'connected'}
            className="inline-flex h-8 min-w-[36px] shrink-0 items-center justify-center rounded bg-[#262626] px-2 font-mono text-xs font-medium text-neutral-200 transition-colors hover:bg-[#333] active:bg-primary active:text-primary-foreground disabled:opacity-40"
            title="Arrow Down"
            aria-label="Down"
          >
            ↓
          </button>
          <button
            type="button"
            onMouseDown={(e) => e.preventDefault()}
            onClick={() => sendKey('\x1b[D')}
            disabled={connection !== 'connected'}
            className="inline-flex h-8 min-w-[36px] shrink-0 items-center justify-center rounded bg-[#262626] px-2 font-mono text-xs font-medium text-neutral-200 transition-colors hover:bg-[#333] active:bg-primary active:text-primary-foreground disabled:opacity-40"
            title="Arrow Left"
            aria-label="Left"
          >
            ←
          </button>
          <button
            type="button"
            onMouseDown={(e) => e.preventDefault()}
            onClick={() => sendKey('\x1b[C')}
            disabled={connection !== 'connected'}
            className="inline-flex h-8 min-w-[36px] shrink-0 items-center justify-center rounded bg-[#262626] px-2 font-mono text-xs font-medium text-neutral-200 transition-colors hover:bg-[#333] active:bg-primary active:text-primary-foreground disabled:opacity-40"
            title="Arrow Right"
            aria-label="Right"
          >
            →
          </button>
          <div className="mx-0.5 h-4 w-px shrink-0 bg-[#333]" />
          <button
            type="button"
            onMouseDown={(e) => e.preventDefault()}
            onClick={() => sendKey('\r')}
            disabled={connection !== 'connected'}
            className="inline-flex h-8 min-w-[48px] shrink-0 items-center justify-center rounded bg-[#262626] px-2.5 font-mono text-xs font-medium text-neutral-200 transition-colors hover:bg-[#333] active:bg-primary active:text-primary-foreground disabled:opacity-40"
            title="Enter (Return)"
            aria-label="Enter"
          >
            ↵ Enter
          </button>
          <button
            type="button"
            onClick={showKeyboard}
            disabled={connection !== 'connected'}
            className="inline-flex h-8 min-w-[36px] shrink-0 items-center justify-center rounded bg-[#262626] px-2 font-mono text-xs text-muted-foreground transition-colors hover:bg-[#333] hover:text-foreground disabled:opacity-40 ml-auto"
            title="Open soft keyboard"
            aria-label="Keyboard"
          >
            <Keyboard className="h-4 w-4" />
          </button>
        </div>
      )}
      <p className="sr-only">
        {liveId
          ? 'Closing this view does not stop the task. Use Cancel session to stop execution.'
          : 'Saved output is bounded and redacted. This view does not start a process.'}
      </p>
    </section>
  )
}
