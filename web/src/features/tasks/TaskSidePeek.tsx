import { useState, useEffect, useRef } from 'react'
import { Task, TaskStatus, TaskPriority, Project } from '../../api/types'
import {
  X,
  Play,
  Bot,
  CheckSquare,
  Terminal as TerminalIcon,
  Save,
  Trash2,
  RefreshCw,
  Sparkles,
} from 'lucide-react'
import { Terminal } from '@xterm/xterm'
import { FitAddon } from '@xterm/addon-fit'
import { WebLinksAddon } from '@xterm/addon-web-links'
import '@xterm/xterm/css/xterm.css'
import { agentsApi } from '../../api/agents'
import { tasksApi } from '../../api/tasks'

interface TaskSidePeekProps {
  task: Task | null
  projects: Project[]
  isOpen: boolean
  onClose: () => void
  onUpdate: (taskId: string, updates: Partial<Task>) => Promise<void>
  onDelete: (taskId: string) => Promise<void>
  onRunAgent: (taskId: string) => Promise<void>
}

interface AgentRunnerOption {
  id: string
  name: string
  role?: string
  harness?: string
  binary: string
}


export function TaskSidePeek({
  task,
  projects,
  isOpen,
  onClose,
  onUpdate,
  onDelete,
  onRunAgent,
}: TaskSidePeekProps) {
  if (!isOpen || !task) return null

  const linkedProject = projects.find(
    (p) => p.id === task.project_id || task.project_ids?.includes(p.id)
  )
  const [activeTab, setActiveTab] = useState<'details' | 'runner'>('details')
  const [title, setTitle] = useState(task.title)
  const [description, setDescription] = useState(
    task.description || task.body || task.prompt || ''
  )
  const [status, setStatus] = useState<TaskStatus>(task.status)
  const [priority, setPriority] = useState<TaskPriority>(task.priority || 'normal')
  const [assignee, setAssignee] = useState<string>(task.assignee || task.assignee_id || '')
  const [saving, setSaving] = useState(false)
  const [agentRunning, setAgentRunning] = useState(task.execution_status === 'running')
  const [executionStatus, setExecutionStatus] = useState<string>(
    task.execution_status || 'idle'
  )
  const [availableAgents, setAvailableAgents] = useState<AgentRunnerOption[]>([])

  // Terminal state & refs
  const terminalContainerRef = useRef<HTMLDivElement>(null)
  const xtermRef = useRef<Terminal | null>(null)
  const fitAddonRef = useRef<FitAddon | null>(null)
  const wsRef = useRef<WebSocket | null>(null)
  const [wsStatus, setWsStatus] = useState<'connecting' | 'connected' | 'disconnected'>('disconnected')

  // Load custom agents from DB
  useEffect(() => {
    agentsApi.list().then((dbAgents) => {
      if (dbAgents && dbAgents.length > 0) {
        const extra = dbAgents.filter(agent => agent.enabled !== false).map((a) => {
          const harness = a.harness || a.provider || 'agy'
          const binary = harness === 'claude' ? 'claude' : (harness === 'codex' ? 'codex' : (harness === 'custom' ? 'shell' : 'agy'))
          return {
            id: a.id,
            name: `${a.name} (${harness})`,
            harness: harness,
            role: `Harness: ${harness}`,
            binary,
          }
        })
        const map = new Map<string, any>()
        for (const ag of extra) {
          map.set(ag.id, ag)
        }
        setAvailableAgents(Array.from(map.values()))
      }
    }).catch(() => {})
  }, [])

  // Update local state when task prop changes
  useEffect(() => {
    setTitle(task.title)
    setDescription(task.description || task.body || task.prompt || '')
    setStatus(task.status)
    setPriority(task.priority || 'normal')
    setAssignee(task.assignee || task.assignee_id || '')
    setExecutionStatus(task.execution_status || 'idle')
    setAgentRunning(task.execution_status === 'running')
  }, [task])

  // Initialize xterm.js when switching to runner tab
  useEffect(() => {
    if (activeTab !== 'runner') return

    let isDisposed = false
    let resizeObserver: ResizeObserver | null = null

    // Give DOM time to render terminalContainerRef
    const timer = setTimeout(() => {
      if (!terminalContainerRef.current || isDisposed) return

      // Clean up previous instance
      if (wsRef.current) {
        wsRef.current.close()
        wsRef.current = null
      }
      if (xtermRef.current) {
        xtermRef.current.dispose()
        xtermRef.current = null
      }
      terminalContainerRef.current.innerHTML = ''

      const term = new Terminal({
        cursorBlink: true,
        fontSize: 12,
        fontFamily: 'Menlo, Monaco, Consolas, "Liberation Mono", "Courier New", monospace',
        lineHeight: 1.25,
        theme: {
          background: '#09090b', // zinc-950
          foreground: '#f4f4f5', // zinc-100
          cursor: '#10b981',     // emerald-500
          cursorAccent: '#09090b',
          selectionBackground: '#27272a',
          black: '#09090b',
          red: '#ef4444',
          green: '#10b981',
          yellow: '#f59e0b',
          blue: '#3b82f6',
          magenta: '#a855f7',
          cyan: '#06b6d4',
          white: '#f4f4f5',
          brightBlack: '#71717a',
          brightRed: '#f87171',
          brightGreen: '#34d399',
          brightYellow: '#fbbf24',
          brightBlue: '#60a5fa',
          brightMagenta: '#c084fc',
          brightCyan: '#22d3ee',
          brightWhite: '#ffffff',
        },
        convertEol: true,
      })

      const fitAddon = new FitAddon()
      const webLinksAddon = new WebLinksAddon()
      term.loadAddon(fitAddon)
      term.loadAddon(webLinksAddon)

      term.open(terminalContainerRef.current)
      try {
        fitAddon.fit()
      } catch (err) {
        console.warn('fit error', err)
      }

      xtermRef.current = term
      fitAddonRef.current = fitAddon

      // Initial welcome banner in xterm
      term.writeln('\x1b[1;36m┌──────────────────────────────────────────────────────────┐\x1b[0m')
      term.writeln(`\x1b[1;36m│\x1b[0m \x1b[1;32mCuan Self-Hosted Agent CLI Runner & Web Terminal\x1b[0m         \x1b[1;36m│\x1b[0m`)
      term.writeln(`\x1b[1;36m│\x1b[0m Task: \x1b[1;33m#${task.id.slice(0, 16)}\x1b[0m                           \x1b[1;36m│\x1b[0m`)
      term.writeln('\x1b[1;36m└──────────────────────────────────────────────────────────┘\x1b[0m')
      term.writeln('\x1b[90mConnecting to live bidirectional WebSocket PTY...\x1b[0m\r\n')

      // Connect WebSocket
      setWsStatus('connecting')
      const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:'
      const token = localStorage.getItem('cuan_auth_token') || ''
      const wsUrl = `${protocol}//${window.location.host}/api/v1/tasks/${task.id}/terminal?token=${encodeURIComponent(token)}`

      const ws = new WebSocket(wsUrl)
      ws.binaryType = 'arraybuffer'
      wsRef.current = ws

      ws.onopen = () => {
        if (isDisposed) return
        setWsStatus('connected')
        term.writeln('\x1b[1;32m[Connected]\x1b[0m Terminal stream online (PTY mode).\r\n')
        ws.send(JSON.stringify({ type: 'resize', cols: term.cols, rows: term.rows }))
      }

      ws.onmessage = (evt) => {
        if (isDisposed) return
        if (evt.data instanceof ArrayBuffer) {
          term.write(new Uint8Array(evt.data))
        } else if (typeof evt.data === 'string') {
          try {
            const parsed = JSON.parse(evt.data)
            if (parsed.type === 'connected') {
              setWsStatus('connected')
            } else if (parsed.type === 'error') {
              term.write(`\r\n\x1b[1;31m[Runner Error]\x1b[0m ${parsed.message}\r\n`)
            }
          } catch {
            term.write(evt.data)
          }
        }
      }

      ws.onerror = () => {
        if (isDisposed) return
        setWsStatus('disconnected')
        term.writeln('\r\n\x1b[1;31m[WebSocket Connection Error]\x1b[0m Failed to communicate with PTY runner.\r\n')
      }

      ws.onclose = () => {
        if (isDisposed) return
        setWsStatus('disconnected')
        term.writeln('\r\n\x1b[90m[Disconnected] Session closed or idle timeout.\x1b[0m\r\n')
      }

      term.onData((data) => {
        if (ws.readyState === WebSocket.OPEN) {
          ws.send(data)
        }
      })

      // Setup container resize observer
      resizeObserver = new ResizeObserver(() => {
        if (!fitAddonRef.current || !xtermRef.current) return
        try {
          fitAddonRef.current.fit()
          if (ws.readyState === WebSocket.OPEN) {
            ws.send(JSON.stringify({
              type: 'resize',
              cols: xtermRef.current.cols,
              rows: xtermRef.current.rows,
            }))
          }
        } catch {
          // ignore layout fits
        }
      })
      resizeObserver.observe(terminalContainerRef.current)
    }, 50)

    return () => {
      isDisposed = true
      clearTimeout(timer)
      if (resizeObserver) resizeObserver.disconnect()
      if (wsRef.current) {
        wsRef.current.close()
        wsRef.current = null
      }
      if (xtermRef.current) {
        xtermRef.current.dispose()
        xtermRef.current = null
      }
    }
  }, [activeTab, task.id])

  const handleSave = async () => {
    setSaving(true)
    try {
      await onUpdate(task.id, {
        title,
        description,
        status,
        priority,
        assignee,
        assignee_id: assignee,
      })
      onClose()
    } catch (err) {
      console.error(err)
    } finally {
      setSaving(false)
    }
  }

  const handleTriggerAgent = async () => {
    if (!availableAgents.some(agent => agent.id === assignee)) return
    setAgentRunning(true)
    setExecutionStatus('running')
    setActiveTab('runner')

    const targetAgent = availableAgents.find((a) => a.id === assignee)
    const runnerBin = targetAgent?.binary || (targetAgent?.harness === 'claude' ? 'claude' : (targetAgent?.harness === 'codex' ? 'codex' : 'agy'))

    if (xtermRef.current) {
      xtermRef.current.writeln(`\r\n\x1b[1;33m[Trigger]\x1b[0m Spawning Agent runner (${targetAgent?.name || assignee})...\r\n`)
    }

    try {
      await tasksApi.runAgent(task.id, {
        agent_id: assignee,
        prompt: description || title,
        binary: runnerBin,
      })
      await onRunAgent(task.id)
    } catch (err: any) {
      if (xtermRef.current) {
        xtermRef.current.writeln(`\r\n\x1b[1;31m[Error]\x1b[0m ${err.message || 'Runner trigger failed'}\r\n`)
      }
      setExecutionStatus('failed')
      setAgentRunning(false)
    }
  }

  const handleClearTerminal = () => {
    if (xtermRef.current) {
      xtermRef.current.clear()
    }
  }

  return (
    <div className="fixed inset-y-0 right-0 z-40 w-full sm:w-[560px] md:w-[620px] bg-card text-card-foreground border-l border-border shadow-2xl flex flex-col font-mono select-none">
      {/* Header */}
      <div className="h-14 border-b border-border px-6 flex items-center justify-between shrink-0">
        <div className="flex items-center gap-2">
          <span className="text-[10px] text-muted-foreground uppercase tracking-wider font-bold">
            Task
          </span>
          <span className="text-xs font-mono text-primary font-bold">
            #{task.id.slice(0, 8)}
          </span>

          {/* Visual Execution Status Indicator */}
          {executionStatus === 'running' || agentRunning ? (
            <span className="flex items-center gap-1.5 px-2 py-0.5 rounded bg-emerald-500/10 text-emerald-500 border border-emerald-500/20 text-[10px] font-bold">
              <span className="relative flex h-2 w-2">
                <span className="animate-ping absolute inline-flex h-full w-full rounded-full bg-emerald-400 opacity-75"></span>
                <span className="relative inline-flex rounded-full h-2 w-2 bg-emerald-500"></span>
              </span>
              <span>Running</span>
            </span>
          ) : executionStatus === 'completed' ? (
            <span className="flex items-center gap-1 px-1.5 py-0.5 rounded bg-emerald-500/10 text-emerald-600 dark:text-emerald-400 text-[10px]">
              <span className="h-1.5 w-1.5 rounded-full bg-emerald-500"></span>
              <span>Completed</span>
            </span>
          ) : executionStatus === 'failed' ? (
            <span className="flex items-center gap-1 px-1.5 py-0.5 rounded bg-destructive/10 text-destructive text-[10px]">
              <span className="h-1.5 w-1.5 rounded-full bg-destructive"></span>
              <span>Error</span>
            </span>
          ) : (
            <span className="flex items-center gap-1 px-1.5 py-0.5 rounded bg-muted text-muted-foreground text-[10px]">
              <span className="h-1.5 w-1.5 rounded-full bg-muted-foreground/60"></span>
              <span>Idle</span>
            </span>
          )}

          {linkedProject && (
            <span className="text-[10px] px-1.5 py-0.5 bg-primary/10 text-primary border border-primary/20 truncate max-w-[120px] rounded">
              {linkedProject.name}
            </span>
          )}
        </div>

        <div className="flex items-center gap-2">
          <button
            onClick={handleTriggerAgent}
            disabled={agentRunning || !availableAgents.some(agent => agent.id === assignee)}
            className="flex items-center gap-1.5 px-2.5 py-1 bg-emerald-500 text-white font-bold text-xs hover:bg-emerald-600 transition-colors disabled:opacity-50 rounded"
            title="Execute task with AI Agent CLI in real PTY"
          >
            {agentRunning ? (
              <RefreshCw className="h-3 w-3 animate-spin" />
            ) : (
              <Play className="h-3 w-3 fill-current" />
            )}
            <span>{agentRunning ? 'Running...' : 'Run Agent'}</span>
          </button>
          <button
            onClick={onClose}
            className="p-1.5 text-muted-foreground hover:text-foreground transition-colors"
          >
            <X className="h-4 w-4" />
          </button>
        </div>
      </div>

      {/* Tabs */}
      <div className="flex border-b border-border text-xs px-6 bg-muted/40 shrink-0">
        <button
          onClick={() => setActiveTab('details')}
          className={`py-2.5 px-3 font-semibold border-b-2 transition-colors ${
            activeTab === 'details'
              ? 'border-primary text-primary'
              : 'border-transparent text-muted-foreground hover:text-foreground'
          }`}
        >
          Task Brief & DoD
        </button>
        <button
          onClick={() => setActiveTab('runner')}
          className={`py-2.5 px-3 font-semibold border-b-2 flex items-center gap-1.5 transition-colors ${
            activeTab === 'runner'
              ? 'border-primary text-primary'
              : 'border-transparent text-muted-foreground hover:text-foreground'
          }`}
        >
          <TerminalIcon className="h-3.5 w-3.5" />
          <span>Live Web Terminal (xterm.js)</span>
          {agentRunning && (
            <span className="h-2 w-2 rounded-full bg-emerald-500 animate-ping" />
          )}
        </button>
      </div>

      {/* Body */}
      <div className="flex-1 overflow-y-auto p-6 space-y-5">
        {activeTab === 'details' ? (
          <>
            <div>
              <label className="block text-[11px] font-bold text-muted-foreground uppercase tracking-wider mb-1">
                Title
              </label>
              <input
                type="text"
                value={title}
                onChange={(e) => setTitle(e.target.value)}
                className="w-full px-3 py-2 text-xs bg-background border border-input text-foreground font-semibold focus:outline-none focus:ring-1 focus:ring-ring rounded"
              />
            </div>

            <div className="grid grid-cols-2 gap-4">
              <div>
                <label className="block text-[11px] font-bold text-muted-foreground uppercase tracking-wider mb-1">
                  Status
                </label>
                <select
                  value={status}
                  onChange={(e) => setStatus(e.target.value as TaskStatus)}
                  className="w-full px-3 py-1.5 text-xs bg-background border border-input text-foreground focus:outline-none focus:ring-1 focus:ring-ring rounded"
                >
                  <option value="backlog">Backlog</option>
                  <option value="todo">To Do</option>
                  <option value="doing">In Progress</option>
                  <option value="review">In Review</option>
                  <option value="done">Completed</option>
                </select>
              </div>

              <div>
                <label className="block text-[11px] font-bold text-muted-foreground uppercase tracking-wider mb-1">
                  Priority
                </label>
                <select
                  value={priority}
                  onChange={(e) => setPriority(e.target.value as TaskPriority)}
                  className="w-full px-3 py-1.5 text-xs bg-background border border-input text-foreground focus:outline-none focus:ring-1 focus:ring-ring rounded"
                >
                  <option value="low">Low</option>
                  <option value="normal">Normal</option>
                  <option value="high">High</option>
                  <option value="urgent">Urgent</option>
                </select>
              </div>
            </div>

            {/* S22.4: Agent Assignment Selector */}
            <div>
              <label className="block text-[11px] font-bold text-muted-foreground uppercase tracking-wider mb-1 flex items-center justify-between">
                <span>Assigned Agent (Worker Account)</span>
                {assignee && (
                  <span className="text-[10px] text-primary lowercase flex items-center gap-1">
                    <Sparkles className="h-3 w-3" />
                    <span>No raw API key required</span>
                  </span>
                )}
              </label>
              <div className="relative">
                <select
                  value={assignee}
                  onChange={(e) => setAssignee(e.target.value)}
                  className="w-full px-3 py-2 text-xs bg-background border border-input text-foreground focus:outline-none focus:ring-1 focus:ring-ring rounded"
                >
                  <option value="">Belum Ditugaskan (Unassigned)</option>
                  {availableAgents.map((ag) => (
                    <option key={ag.id} value={ag.id}>
                      {ag.name} — {ag.role}
                    </option>
                  ))}
                </select>
              </div>
            </div>

            <div>
              <label className="block text-[11px] font-bold text-muted-foreground uppercase tracking-wider mb-1">
                Prompt / Task Description
              </label>
              <textarea
                rows={8}
                value={description}
                onChange={(e) => setDescription(e.target.value)}
                placeholder="Describe instructions for the human or AI Agent..."
                className="w-full px-3 py-2 text-xs bg-background border border-input text-foreground focus:outline-none focus:ring-1 focus:ring-ring rounded"
              />
            </div>

            {task.dod_checklist && task.dod_checklist.length > 0 && (
              <div>
                <label className="block text-[11px] font-bold text-muted-foreground uppercase tracking-wider mb-2">
                  Definition of Done (DoD)
                </label>
                <div className="space-y-1.5 bg-muted/40 p-3 border border-border rounded">
                  {task.dod_checklist.map((item, idx) => {
                    const text = typeof item === 'string' ? item : item.text
                    const completed = typeof item === 'object' && !!item.completed
                    return (
                      <div key={idx} className="flex items-start gap-2 text-xs">
                        <CheckSquare
                          className={`h-3.5 w-3.5 ${
                            completed ? 'text-emerald-500' : 'text-primary'
                          } shrink-0 mt-0.5`}
                        />
                        <span
                          className={
                            completed
                              ? 'line-through text-muted-foreground'
                              : 'text-foreground'
                          }
                        >
                          {text}
                        </span>
                      </div>
                    )
                  })}
                </div>
              </div>
            )}
          </>
        ) : (
          /* S22.3: Live Interactive Web Terminal (xterm.js) */
          <div className="flex flex-col h-full space-y-3">
            {/* Terminal Toolbar */}
            <div className="flex items-center justify-between text-xs pb-1 border-b border-border">
              <div className="flex items-center gap-2">
                <Bot className="h-4 w-4 text-primary" />
                <span className="font-bold text-foreground">
                  {assignee ? availableAgents.find((a) => a.id === assignee)?.name || assignee : 'Agent CLI Terminal'}
                </span>
                <span className="text-[10px] px-1.5 py-0.5 rounded bg-muted border border-border text-muted-foreground">
                  PTY Interactive
                </span>
              </div>

              <div className="flex items-center gap-2">
                {/* WebSocket status indicator */}
                <div className="flex items-center gap-1.5 text-[11px]">
                  <span
                    className={`h-2 w-2 rounded-full ${
                      wsStatus === 'connected'
                        ? 'bg-emerald-500 shadow-[0_0_8px_rgba(16,185,129,0.5)]'
                        : wsStatus === 'connecting'
                        ? 'bg-yellow-500 animate-pulse'
                        : 'bg-muted-foreground/50'
                    }`}
                  />
                  <span className="text-muted-foreground capitalize">{wsStatus}</span>
                </div>

                <button
                  onClick={handleClearTerminal}
                  className="px-2 py-0.5 bg-muted hover:bg-muted/80 text-[11px] rounded text-muted-foreground hover:text-foreground transition-colors"
                  title="Clear Terminal Display"
                >
                  Clear
                </button>
              </div>
            </div>

            {/* xterm.js Mount Container */}
            <div className="relative flex-1 bg-zinc-950 border border-border rounded overflow-hidden shadow-inner p-2 min-h-[380px]">
              <div
                ref={terminalContainerRef}
                className="w-full h-full"
                style={{ height: '380px' }}
              />
            </div>

            {/* Terminal Bottom Controls */}
            <div className="flex items-center justify-between gap-3 pt-1">
              <button
                onClick={handleTriggerAgent}
                disabled={agentRunning || !availableAgents.some(agent => agent.id === assignee)}
                className="flex-1 py-2 bg-primary text-primary-foreground font-bold text-xs hover:bg-primary/90 transition-colors flex items-center justify-center gap-2 disabled:opacity-50 rounded shadow-xs"
              >
                {agentRunning ? (
                  <RefreshCw className="h-3.5 w-3.5 animate-spin" />
                ) : (
                  <Play className="h-3.5 w-3.5 fill-current" />
                )}
                <span>{agentRunning ? 'Agent Runner is Active...' : 'Run Agent on Task (PTY)'}</span>
              </button>
            </div>
          </div>
        )}
      </div>

      {/* Footer */}
      {activeTab === 'details' && (
        <div className="h-14 border-t border-border px-6 flex items-center justify-between shrink-0">
          <button
            onClick={() => onDelete(task.id)}
            className="flex items-center gap-1.5 text-xs text-destructive hover:underline"
          >
            <Trash2 className="h-3.5 w-3.5" />
            <span>Delete</span>
          </button>

          <button
            onClick={handleSave}
            disabled={saving}
            className="flex items-center gap-1.5 px-4 py-2 bg-primary text-primary-foreground font-semibold text-xs hover:bg-primary/90 transition-colors disabled:opacity-50 rounded"
          >
            <Save className="h-3.5 w-3.5" />
            <span>{saving ? 'Saving...' : 'Save Changes'}</span>
          </button>
        </div>
      )}
    </div>
  )
}
