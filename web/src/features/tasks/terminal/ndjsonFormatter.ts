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
 * Extract clean filename / basename from a file path.
 */
export function getFilename(filePath?: string): string {
  if (!filePath || typeof filePath !== 'string') return ''
  const normalized = filePath.replace(/\\/g, '/')
  const parts = normalized.split('/').filter(Boolean)
  return parts.length > 0 ? parts[parts.length - 1] : filePath
}

/**
 * Maps common shell commands to clean, human-friendly action descriptions.
 */
export function getFriendlyCommandDescription(cmd?: string): string | undefined {
  if (!cmd || typeof cmd !== 'string') return undefined
  const trimmed = cmd.trim()
  if (!trimmed) return undefined

  if (/^git\s+status\b/i.test(trimmed)) return 'Check Git status'
  if (/^git\s+diff\b/i.test(trimmed)) return 'Check Git diff'
  if (/^git\s+log\b/i.test(trimmed)) return 'Check Git history'
  if (/^git\s+branch\b/i.test(trimmed)) return 'List Git branches'
  if (/^git\s+checkout\b/i.test(trimmed)) return 'Checkout Git branch'
  if (/^git\s+commit\b/i.test(trimmed)) return 'Commit Git changes'
  if (/^git\s+push\b/i.test(trimmed)) return 'Push Git changes'
  if (/^git\s+pull\b/i.test(trimmed)) return 'Pull Git changes'
  if (
    /^(npm|pnpm|yarn|bun)\s+(run\s+)?test\b/i.test(trimmed) ||
    /^(vitest|jest|node\s+--test)\b/i.test(trimmed)
  ) {
    return 'Run test suite'
  }
  if (/^(npm|pnpm|yarn|bun)\s+(run\s+)?build\b/i.test(trimmed)) {
    return 'Build project assets'
  }
  if (/^(npm|pnpm|yarn|bun)\s+install\b/i.test(trimmed)) {
    return 'Install dependencies'
  }
  if (/^go\s+test\b/i.test(trimmed)) return 'Run Go tests'
  if (/^go\s+build\b/i.test(trimmed)) return 'Build Go binary'
  if (/^find\b/i.test(trimmed)) return 'Find workspace files'
  if (/^(grep|ripgrep|rg)\b/i.test(trimmed)) return 'Search code'
  if (/^ls\b/i.test(trimmed)) return 'List directory'
  if (/^cat\b/i.test(trimmed)) return 'Read file content'

  return undefined
}

/**
 * Resolves a human-friendly action card title from tool name and arguments.
 */
export function getActionTitle(
  tool: string,
  args?: Record<string, any>,
  fallback = 'Agent Action',
): string {
  const lower = (tool || '').toLowerCase().trim()

  // cuan-inspect & MCP tool mapping
  if (lower.includes('cuan-inspect') || lower === 'list_tasks' || lower === 'get_task') {
    if (lower.includes('list_tasks')) return 'Inspect Tasks'
    if (lower.includes('get_task')) return 'Inspect Task'
    if (lower.includes('list_projects')) return 'Inspect Projects'
    if (lower.includes('get_project')) return 'Inspect Project'
    if (lower.includes('create_task_draft')) return 'Create Task Draft'
    if (lower.includes('inspect_git_status')) return 'Inspect Git Status'
    if (lower.includes('inspect_git_log')) return 'Inspect Git Log'
    if (lower.includes('inspect_git_diff')) return 'Inspect Git Diff'
    if (lower.includes('inspect_system_health')) return 'Inspect System Health'
    if (lower.includes('inspect_typecheck')) return 'Inspect Typecheck'
    if (lower.includes('list_inbox_conversations')) return 'List Inbox Conversations'
    if (lower.includes('get_inbox_messages')) return 'Get Inbox Messages'
    if (lower.includes('list_contacts')) return 'List Contacts'
    if (lower.includes('get_contact')) return 'Get Contact'
    if (lower.includes('list_scheduled_jobs')) return 'List Scheduled Jobs'
    if (lower.includes('inspect_database_query')) return 'Inspect Database Query'
    if (lower.includes('inspect_database_schema')) return 'Inspect Database Schema'
    if (lower.includes('inspect_app_logs')) return 'Inspect App Logs'

    const subTool = lower.split(/[:/]/)[1]?.trim()
    if (subTool) {
      return subTool
        .replace(/^inspect[_-]?/, 'Inspect ')
        .replace(/^list[_-]?/, 'Inspect ')
        .replace(/[_-]/g, ' ')
        .replace(/\b\w/g, (c) => c.toUpperCase())
    }
    return 'Inspect Tasks'
  }

  // File operations
  if (lower === 'read_file' || lower === 'cat' || lower === 'file_read') {
    const rawPath = args?.Path || args?.path || args?.file_path || args?.filename || args?.file
    if (rawPath && typeof rawPath === 'string') {
      const filename = getFilename(rawPath)
      return `Read ${filename}`
    }
    return 'Read File'
  }

  if (
    lower === 'write_file' ||
    lower === 'edit_file' ||
    lower === 'create_file' ||
    lower === 'file_write'
  ) {
    const rawPath = args?.Path || args?.path || args?.file_path || args?.filename || args?.file
    if (rawPath && typeof rawPath === 'string') {
      const filename = getFilename(rawPath)
      return `Update ${filename}`
    }
    return lower === 'create_file' ? 'Create File' : 'Update File'
  }

  // Command execution
  if (
    lower === 'run_command' ||
    lower === 'command' ||
    lower === 'bash' ||
    lower === 'sh' ||
    lower === 'exec'
  ) {
    if (args?.toolSummary && typeof args.toolSummary === 'string') {
      return args.toolSummary
    }
    const friendly = getFriendlyCommandDescription(args?.CommandLine || args?.command)
    if (friendly) return friendly
    return 'Run Command'
  }

  if (lower === 'search_web' || lower === 'web_search') {
    return 'Search Web'
  }
  if (lower === 'thought' || lower === 'thinking') {
    return 'Agent Thought'
  }
  if (lower === 'invoke_subagent' || lower === 'subagent' || lower === 'delegate') {
    return 'Subagent Delegation'
  }
  if (lower.startsWith('git')) {
    return 'Git Operation'
  }
  if (lower === 'schedule' || lower === 'timer') {
    return 'Schedule Job'
  }

  if (!tool || lower === 'agent step' || lower === 'agent_step') {
    return fallback
  }

  const formatted = tool
    .replace(/[_-]/g, ' ')
    .replace(/\b\w/g, (c) => c.toUpperCase())

  return formatted === 'Agent Step' ? fallback : (formatted || fallback)
}

/**
 * Maps a tool identifier to a human-readable display name and icon
 */
export function mapToolMetadata(
  tool: string,
  args?: Record<string, any>,
): { displayName: string; icon: StepIconType } {
  const lower = (tool || '').toLowerCase().trim()

  // cuan-inspect & MCP tool mapping
  if (lower.includes('cuan-inspect') || lower === 'list_tasks' || lower === 'get_task') {
    const displayName = getActionTitle(tool, args, 'Inspect Tasks')
    const icon: StepIconType =
      lower.includes('inspect_git') || lower.includes('inspect_typecheck')
        ? 'command'
        : lower.includes('create_task')
          ? 'file-write'
          : lower.includes('list_scheduled')
            ? 'clock'
            : 'search'
    return { displayName, icon }
  }

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
    const rawPath = args?.Path || args?.path || args?.file_path || args?.filename || args?.file
    if (rawPath && typeof rawPath === 'string') {
      const filename = getFilename(rawPath)
      return { displayName: `Update ${filename}`, icon: 'file-write' }
    }
    return { displayName: 'Write File', icon: 'file-write' }
  }

  if (lower === 'read_file' || lower === 'cat' || lower === 'file_read') {
    const rawPath = args?.Path || args?.path || args?.file_path || args?.filename || args?.file
    if (rawPath && typeof rawPath === 'string') {
      const filename = getFilename(rawPath)
      return { displayName: `Read ${filename}`, icon: 'file-read' }
    }
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

  if (!tool || lower === 'agent step' || lower === 'agent_step') {
    return { displayName: 'Agent Action', icon: 'tool' }
  }

  // Fallback: title-case the tool name
  const formatted = tool
    .replace(/[_-]/g, ' ')
    .replace(/\b\w/g, (c) => c.toUpperCase())

  const cleanTitle = formatted === 'Agent Step' ? 'Agent Action' : (formatted || 'Agent Action')

  return { displayName: cleanTitle, icon: 'tool' }
}

/**
 * Parse a single NDJSON log line into a VisualStep if it matches JSON structure.
 * Supports deep nested structures emitted by agent runner (step_update, tool_info, parameters).
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

    // Deep payload unwrap: inspect step_update, tool_info, tool_call, parameters
    const stepUpdate =
      data.step_update && typeof data.step_update === 'object' ? data.step_update : null
    const toolInfo =
      stepUpdate?.tool_info && typeof stepUpdate.tool_info === 'object'
        ? stepUpdate.tool_info
        : data.tool_info && typeof data.tool_info === 'object'
          ? data.tool_info
          : null
    const toolCall =
      data.tool_call && typeof data.tool_call === 'object'
        ? data.tool_call
        : stepUpdate?.tool_call && typeof stepUpdate.tool_call === 'object'
          ? stepUpdate.tool_call
          : null

    // Extract raw tool identifier
    let tool =
      toolInfo?.name ||
      stepUpdate?.tool_name ||
      stepUpdate?.tool ||
      data.tool_name ||
      data.tool ||
      toolCall?.name ||
      data.action ||
      stepUpdate?.action ||
      (data.type === 'tool_use' || data.type === 'tool_call' ? data.name : '') ||
      data.name ||
      (stepUpdate?.step_type === 'thought' ? 'thought' : '') ||
      (data.step_type === 'thought' ? 'thought' : '') ||
      (data.type === 'thought' ? 'thought' : '') ||
      ''

    // Extract parameters / arguments across nested levels
    let rawParams =
      toolInfo?.parameters ||
      toolInfo?.arguments ||
      toolInfo?.args ||
      toolCall?.args ||
      toolCall?.parameters ||
      stepUpdate?.parameters ||
      stepUpdate?.args ||
      stepUpdate?.arguments ||
      data.parameters ||
      data.args ||
      data.arguments ||
      data.input ||
      {}

    if (typeof rawParams === 'string') {
      try {
        rawParams = JSON.parse(rawParams)
      } catch {
        // Keep string or raw
      }
    }

    // Inspect MCP parameters (e.g. call_mcp_tool -> ServerName / ToolName / Arguments)
    let mcpServer = ''
    let mcpTool = ''
    let effectiveArgs: Record<string, any> =
      typeof rawParams === 'object' && rawParams !== null ? { ...rawParams } : {}

    if (typeof rawParams === 'object' && rawParams !== null) {
      mcpServer = rawParams.ServerName || rawParams.server_name || rawParams.server || ''
      mcpTool = rawParams.ToolName || rawParams.tool_name || rawParams.tool || ''

      const nestedMcpArgs = rawParams.Arguments || rawParams.arguments || rawParams.args
      if (nestedMcpArgs !== undefined && nestedMcpArgs !== null) {
        if (typeof nestedMcpArgs === 'string') {
          try {
            effectiveArgs = JSON.parse(nestedMcpArgs)
          } catch {
            effectiveArgs = { raw: nestedMcpArgs }
          }
        } else if (typeof nestedMcpArgs === 'object') {
          effectiveArgs = nestedMcpArgs
        }
      }
    }

    // If tool is call_mcp_tool or MCP parameters exist, resolve real tool name
    if (tool === 'call_mcp_tool' || (!tool && mcpTool)) {
      if (mcpServer && mcpTool) {
        tool = `${mcpServer}: ${mcpTool}`
      } else if (mcpTool) {
        tool = mcpTool
      } else if (mcpServer) {
        tool = mcpServer
      }
    }

    if (!tool && !data.step && !data.event && !stepUpdate) return null

    // Metadata mapping
    const meta = mapToolMetadata(tool, effectiveArgs)

    // Duration extraction
    let duration: string | undefined
    const rawDur =
      stepUpdate?.duration_ms ??
      data.duration_ms ??
      stepUpdate?.duration ??
      data.duration
    if (typeof rawDur === 'number') {
      duration = formatDuration(rawDur)
    } else if (typeof rawDur === 'string') {
      duration = rawDur
    }

    // Status resolution (ACTIVE -> running, DONE/COMPLETED -> completed, ERROR/FAILED -> failed)
    let status: StepStatus = 'completed'
    const statusVal = String(
      stepUpdate?.state ||
      stepUpdate?.status ||
      data.status ||
      data.state ||
      ''
    ).toLowerCase()

    if (
      statusVal === 'failed' ||
      statusVal === 'error' ||
      data.is_error === true ||
      stepUpdate?.is_error === true ||
      data.exit_code > 0 ||
      stepUpdate?.exit_code > 0
    ) {
      status = 'failed'
    } else if (
      statusVal === 'running' ||
      statusVal === 'in_progress' ||
      statusVal === 'started' ||
      statusVal === 'active'
    ) {
      status = 'running'
    } else if (statusVal === 'pending' || statusVal === 'queued') {
      status = 'pending'
    }

    // Summary extraction
    let summary = ''
    if (stepUpdate?.summary) {
      summary = stepUpdate.summary
    } else if (data.summary) {
      summary = data.summary
    } else if (stepUpdate?.description) {
      summary = stepUpdate.description
    } else if (data.description) {
      summary = data.description
    } else if (effectiveArgs.toolSummary) {
      summary = effectiveArgs.toolSummary
    } else if (effectiveArgs.CommandLine) {
      summary = effectiveArgs.CommandLine
    } else if (effectiveArgs.command) {
      summary = effectiveArgs.command
    } else if (effectiveArgs.Path) {
      summary = effectiveArgs.Path
    } else if (effectiveArgs.path) {
      summary = effectiveArgs.path
    } else if (effectiveArgs.query) {
      summary = effectiveArgs.query
    } else if (typeof data.content === 'string') {
      summary = data.content.slice(0, 100)
    } else if (typeof stepUpdate?.content === 'string') {
      summary = stepUpdate.content.slice(0, 100)
    } else if (typeof data.message === 'string') {
      summary = data.message.slice(0, 100)
    } else if (typeof data.stdout === 'string') {
      summary = data.stdout.slice(0, 100)
    }

    // Deduplication: Never duplicate step title as summary subtitle
    if (
      !summary ||
      summary.trim().toLowerCase() === meta.displayName.trim().toLowerCase() ||
      summary.trim().toLowerCase() === tool.toLowerCase() ||
      summary.trim() === 'Agent Step' ||
      summary.trim() === 'Agent Action' ||
      summary.trim() === 'step'
    ) {
      summary = ''
    }

    // Details formatting
    let details: string | undefined
    const detailPayload =
      stepUpdate?.output ||
      data.output ||
      stepUpdate?.result ||
      data.result ||
      stepUpdate?.tool_result?.content ||
      data.tool_result?.content ||
      (Object.keys(effectiveArgs).length > 0 ? effectiveArgs : undefined) ||
      (rawParams && Object.keys(rawParams).length > 0 ? rawParams : undefined) ||
      stepUpdate?.details ||
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

    const stepNumber =
      typeof stepUpdate?.step_index === 'number'
        ? stepUpdate.step_index
        : typeof data.step_index === 'number'
          ? data.step_index
          : typeof data.step === 'number'
            ? data.step
            : index + 1

    const stepId =
      stepUpdate?.id ||
      data.id ||
      data.step_id ||
      stepUpdate?.step_id ||
      `step-${stepNumber}-${index}`

    return {
      id: String(stepId),
      stepNumber,
      tool: tool || 'step',
      toolDisplayName: meta.displayName,
      icon: meta.icon,
      status,
      duration,
      summary: summary.trim(),
      details,
      raw: clean,
      timestamp:
        stepUpdate?.timestamp ||
        data.timestamp ||
        data.logged_at ||
        data.created_at,
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

  const hasSummary =
    step.summary &&
    step.summary.trim() !== '' &&
    step.summary.trim().toLowerCase() !== step.toolDisplayName.trim().toLowerCase() &&
    step.summary !== 'Agent Step' &&
    step.summary !== 'Agent Action'

  const summaryText = hasSummary ? `\x1b[38;2;229;229;229m${step.summary}\x1b[0m` : ''

  let lineOut = `${iconSymbol} ${toolBadge} ${durBadge}${summaryText}`.trimEnd()
  if (step.status === 'completed' && step.details) {
    const cleanDetails = stripAnsi(step.details).trim()
    if (
      cleanDetails &&
      cleanDetails !== step.summary &&
      !cleanDetails.startsWith('{"event":') &&
      cleanDetails.length < 3000
    ) {
      const indented = cleanDetails
        .split('\n')
        .slice(0, 12)
        .map((l) => `  \x1b[38;2;163;163;163m${l.trimEnd()}\x1b[0m`)
        .join('\r\n')
      lineOut += '\r\n' + indented
    }
  }
  return lineOut
}

/**
 * Parses an array of raw log records and aggregates them into VisualSteps.
 * Merges active and completed updates for the same step cleanly without duplication.
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

    const stepKey = parsed.stepNumber ? `step-${parsed.stepNumber}` : parsed.tool
    const existing = stepMap.get(stepKey) || stepMap.get(parsed.tool)

    // If an existing running step matches, merge results
    if (existing && existing.status === 'running' && parsed.status !== 'running') {
      existing.status = parsed.status
      if (parsed.duration) existing.duration = parsed.duration
      if (parsed.details) existing.details = parsed.details
      if (parsed.summary && parsed.summary !== parsed.toolDisplayName) {
        existing.summary = parsed.summary
      }
    } else {
      if (parsed.stepNumber === undefined) {
        parsed.stepNumber = stepCounter++
      }
      steps.push(parsed)
      if (stepKey) {
        stepMap.set(stepKey, parsed)
      }
      if (parsed.tool) {
        stepMap.set(parsed.tool, parsed)
      }
    }
  }

  return steps
}
