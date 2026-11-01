/**
 * NDJSON Log Formatter for Cuan Live Terminal & Agentic Scrum.
 * Transforms raw, noisy agent tool JSON logs into serene, human-friendly Things 3 steps.
 */

export type StepStatus = 'pending' | 'running' | 'completed' | 'failed'

export type StepIconType =
  | 'command'
  | 'file-write'
  | 'file-read'
  | 'search'
  | 'thought'
  | 'check'
  | 'alert'
  | 'tool'
  | 'clock'
  | 'subagent'

export interface VisualStep {
  id: string
  stepNumber?: number
  tool: string
  toolDisplayName: string
  icon: StepIconType
  status: StepStatus
  duration?: string
  summary: string
  details?: string
  raw?: string
  timestamp?: string
  isCollapsed?: boolean
}

// Strip ANSI terminal escape codes
export function stripAnsi(str: string): string {
  // eslint-disable-next-line no-control-regex
  return str.replace(/\x1b\[[0-9;]*[a-zA-Z]/g, '')
}

/**
 * Format duration ms into human-readable string (e.g. 120ms or 1.4s)
 */
export function formatDuration(durationMs?: number): string | undefined {
  if (durationMs === undefined || durationMs === null || isNaN(durationMs)) return undefined
  if (durationMs < 1000) {
    return `${Math.round(durationMs)}ms`
  }
  return `${(durationMs / 1000).toFixed(1)}s`
}

/**
 * Maps a tool identifier to a human-readable display name and icon
 */
export function mapToolMetadata(tool: string): { displayName: string; icon: StepIconType } {
  const lower = (tool || '').toLowerCase().trim()

  if (
    lower === 'run_command' ||
    lower === 'command' ||
    lower === 'bash' ||
    lower === 'sh' ||
    lower === 'exec'
  ) {
    return { displayName: 'Run Command', icon: 'command' }
  }
  if (
    lower === 'write_file' ||
    lower === 'edit_file' ||
    lower === 'create_file' ||
    lower === 'file_write'
  ) {
    return { displayName: 'Write File', icon: 'file-write' }
  }
  if (lower === 'read_file' || lower === 'cat' || lower === 'file_read') {
    return { displayName: 'Read File', icon: 'file-read' }
  }
  if (lower === 'search_web' || lower === 'web_search') {
    return { displayName: 'Search Web', icon: 'search' }
  }
  if (lower === 'thought' || lower === 'thinking') {
    return { displayName: 'Agent Thought', icon: 'thought' }
  }
  if (lower === 'invoke_subagent' || lower === 'subagent' || lower === 'delegate') {
    return { displayName: 'Subagent Delegation', icon: 'subagent' }
  }
  if (lower.startsWith('git')) {
    return { displayName: 'Git Operation', icon: 'command' }
  }
  if (lower === 'schedule' || lower === 'timer') {
    return { displayName: 'Schedule Job', icon: 'clock' }
  }

  // Fallback: title-case the tool name
  const formatted = tool
    .replace(/[_-]/g, ' ')
    .replace(/\b\w/g, (c) => c.toUpperCase())

  return { displayName: formatted || 'Agent Step', icon: 'tool' }
}

/**
 * Parse a single NDJSON log line into a VisualStep if it matches JSON structure.
 * Returns null if the line is not valid JSON or not an agent action event.
 */
export function parseNdjsonLine(rawLine: string, index = 0): VisualStep | null {
  const clean = stripAnsi(rawLine).trim()
  if (!clean.startsWith('{') || !clean.endsWith('}')) {
    return null
  }

  try {
    const data = JSON.parse(clean)
    if (typeof data !== 'object' || data === null) return null

    // Determine tool name
    const tool =
      data.tool_call?.name ||
      data.tool_name ||
      data.tool ||
      data.action ||
      (data.type === 'tool_use' || data.type === 'tool_call' ? data.name : '') ||
      data.name ||
      data.type ||
      ''

    if (!tool && !data.step && !data.event) return null

    const meta = mapToolMetadata(tool)

    // Duration extraction
    let duration: string | undefined
    if (typeof data.duration_ms === 'number') {
      duration = formatDuration(data.duration_ms)
    } else if (typeof data.duration === 'string') {
      duration = data.duration
    } else if (typeof data.duration === 'number') {
      duration = formatDuration(data.duration)
    }

    // Status resolution
    let status: StepStatus = 'completed'
    const statusVal = String(data.status || data.state || '').toLowerCase()
    if (
      statusVal === 'failed' ||
      statusVal === 'error' ||
      data.is_error === true ||
      data.exit_code > 0
    ) {
      status = 'failed'
    } else if (statusVal === 'running' || statusVal === 'in_progress' || statusVal === 'started') {
      status = 'running'
    } else if (statusVal === 'pending' || statusVal === 'queued') {
      status = 'pending'
    }

    // Summary extraction
    const args = data.tool_call?.args || data.args || data.input || data.parameters || {}
    let summary = ''
    if (data.summary) {
      summary = data.summary
    } else if (data.description) {
      summary = data.description
    } else if (args.toolSummary) {
      summary = args.toolSummary
    } else if (args.CommandLine) {
      summary = args.CommandLine
    } else if (args.Path) {
      summary = args.Path
    } else if (args.query) {
      summary = args.query
    } else if (data.content && typeof data.content === 'string') {
      summary = data.content.slice(0, 100)
    } else if (data.message && typeof data.message === 'string') {
      summary = data.message.slice(0, 100)
    } else if (data.stdout && typeof data.stdout === 'string') {
      summary = data.stdout.slice(0, 100)
    } else if (tool) {
      summary = meta.displayName
    }

    // Details formatting
    let details: string | undefined
    const detailPayload =
      data.output ||
      data.result ||
      data.tool_result?.content ||
      (Object.keys(args).length > 0 ? args : undefined) ||
      data.details

    if (detailPayload) {
      if (typeof detailPayload === 'string') {
        details = detailPayload
      } else {
        try {
          details = JSON.stringify(detailPayload, null, 2)
        } catch {
          details = String(detailPayload)
        }
      }
    }

    const stepId = data.id || data.step_id || `step-${index}-${Date.now()}`
    const stepNumber = typeof data.step === 'number' ? data.step : index + 1

    return {
      id: String(stepId),
      stepNumber,
      tool: tool || 'step',
      toolDisplayName: meta.displayName,
      icon: meta.icon,
      status,
      duration,
      summary: summary.trim() || meta.displayName,
      details,
      raw: clean,
      timestamp: data.timestamp || data.logged_at || data.created_at,
      isCollapsed: true,
    }
  } catch {
    return null
  }
}

/**
 * Format a log line into a clean ANSI line for terminal output.
 * If the line is NDJSON, it replaces the raw JSON dump with a Things 3 styled ANSI line.
 */
export function formatLogLineForTerminal(line: string): string {
  const step = parseNdjsonLine(line)
  if (!step) {
    return line
  }

  // Things 3 inspired calm ANSI colors:
  // Emerald for success, Rose for failure, Amber for running/pending
  const iconSymbol =
    step.status === 'completed'
      ? '\x1b[38;2;16;185;129m✔\x1b[0m' // Emerald check
      : step.status === 'failed'
        ? '\x1b[38;2;244;63;94m✖\x1b[0m' // Rose cross
        : '\x1b[38;2;245;158;11m●\x1b[0m' // Amber dot

  const toolBadge = `\x1b[1m\x1b[38;2;56;189;248m${step.toolDisplayName}\x1b[0m`
  const durBadge = step.duration ? `\x1b[38;2;163;163;163m(${step.duration})\x1b[0m ` : ''
  const summaryText = `\x1b[38;2;229;229;229m${step.summary}\x1b[0m`

  return `${iconSymbol} ${toolBadge} ${durBadge}${summaryText}`
}

/**
 * Parses an array of raw log records and aggregates them into VisualSteps.
 * Merges tool_call and tool_result pairs when emitted as separate sequential events.
 */
export function parseLogsToVisualSteps(
  logs: Array<{ id?: number; line: string; logged_at?: string }>,
): VisualStep[] {
  const steps: VisualStep[] = []
  const stepMap = new Map<string, VisualStep>()

  let stepCounter = 1

  for (let i = 0; i < logs.length; i++) {
    const rawLine = logs[i].line
    const parsed = parseNdjsonLine(rawLine, i)
    if (!parsed) continue

    // If an existing running step matches, merge results
    const existing = stepMap.get(parsed.tool)
    if (existing && existing.status === 'running' && parsed.status !== 'running') {
      existing.status = parsed.status
      if (parsed.duration) existing.duration = parsed.duration
      if (parsed.details) existing.details = parsed.details
      if (parsed.summary && parsed.summary !== parsed.toolDisplayName) {
        existing.summary = parsed.summary
      }
    } else {
      parsed.stepNumber = stepCounter++
      steps.push(parsed)
      if (parsed.tool) {
        stepMap.set(parsed.tool, parsed)
      }
    }
  }

  return steps
}
