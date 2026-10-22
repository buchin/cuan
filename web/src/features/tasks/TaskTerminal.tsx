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
} from 'lucide-react'
import { useLanguage } from '../../context/LanguageContext'
import { getAuthToken } from '../../api/client'
import { tasksApi, type LiveTaskSession, type TaskLog } from '../../api/tasks'
import type { TaskAttempt } from '../../api/types'
import type { Terminal } from '@xterm/xterm'
import '@xterm/xterm/css/xterm.css'

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
  const { t, language } = useLanguage()
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
  const isTaskActive = ['running', 'queued'].includes(attempt.status)
  const isLive =
    !!live &&
    live.attempt_id === attempt.id &&
    isTaskActive
  const liveId = isLive ? live.session_id : ''
  const refreshRef = useRef(onRefresh)
  refreshRef.current = onRefresh

  const handleReconnect = useCallback(() => {
    setError('')
    setNotice('')
    setConnection('connecting')
    setRevision((n) => n + 1)
    onRefresh()
  }, [onRefresh])

  const lastAutoRetry = useRef(0)
  useEffect(() => {
    const handleActive = () => {
      if (document.visibilityState !== 'visible') return
      if (!['running', 'queued'].includes(attempt.status)) return
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
  }, [attempt.status, connection, handleReconnect])

  const showDisconnectBanner =
    isTaskActive &&
    (connection === 'disconnected' ||
      (connection === 'connecting' && revision > 0) ||
      (connection === 'error' && !logs.current.length))
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
      refreshTimer: ReturnType<typeof setTimeout> | undefined
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
        terminal.onScroll((position) => {
          if (disposed || !terminal) return
          if (position < terminal.buffer.active.baseY) {
            following.current = false
            setFollow(false)
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
          if (!disposed) {
            setConnection('disconnected')
          }
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
              loading: t('Loading output…'),
              connecting: t('Connecting to existing session…'),
              connected: input
                ? t('Connected · input enabled')
                : t('Connected · read-only'),
              saved: t('Saved output'),
              disconnected: t('Disconnected · task status is checked separately'),
              error: t('Output unavailable'),
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
            aria-label={t('Show keyboard')}
            title={t('Show keyboard')}
            disabled={connection !== 'connected'}
            onClick={showKeyboard}
          >
            <Keyboard className="h-4 w-4" />
          </button>
        )}
      </div>
      <details ref={menu} className="relative shrink-0">
        <summary
          aria-label={t('Terminal tools')}
          title={t('Terminal tools')}
          className="icon-action cursor-pointer list-none [&::-webkit-details-marker]:hidden"
        >
          <MoreHorizontal className="h-4 w-4" />
        </summary>
        <div className="absolute right-0 top-full z-20 w-[min(360px,calc(100vw-24px))] space-y-2 rounded-lg border bg-popover p-3 text-popover-foreground shadow-xl">
          <div className="flex flex-wrap justify-end gap-1">
            <button
              className="icon-action"
              aria-label={t('Reconnect terminal')}
              title={t('Reconnect to this session')}
              onClick={handleReconnect}
            >
              <RefreshCw className="h-4 w-4" />
            </button>
            <button
              className="icon-action"
              aria-label={t('Copy output')}
              title={t('Copy output')}
              onClick={async () => {
                try {
                  await navigator.clipboard.writeText(bufferText())
                  setNotice(t('Output copied.'))
                } catch {
                  setError(
                    t('Could not copy output. Use Download output instead.'),
                  )
                }
              }}
            >
              <Copy className="h-4 w-4" />
            </button>
            <button
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
                  className="icon-action"
                  aria-label={t('Previous match')}
                  onClick={() => find(-1)}
                >
                  <ChevronUp className="h-4 w-4" />
                </button>
                <button
                  className="icon-action"
                  aria-label={t('Next match')}
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
      className="flex h-full min-h-0 min-w-0 flex-col bg-[#141414]"
    >
      {controlsTarget ? createPortal(controls, controlsTarget) : controls}
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
        <p role="alert" className="shrink-0 px-3 py-1.5 text-xs text-destructive">
          {error}
        </p>
      )}
      {notice && (
        <p role="status" className="shrink-0 px-3 py-1 text-xs text-muted-foreground">
          {notice}
        </p>
      )}
      <div
        className="min-h-0 min-w-0 flex-1 overflow-hidden bg-[#141414] p-2"
        aria-label="Terminal output"
      >
        <div ref={mount} className="h-full w-full" />
      </div>
      <p className="sr-only">
        {liveId
          ? 'Closing this view does not stop the task. Use Cancel session to stop execution.'
          : 'Saved output is bounded and redacted. This view does not start a process.'}
      </p>
    </section>
  )
}
