import test from 'node:test'
import assert from 'node:assert/strict'
import fs from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'

const __filename = fileURLToPath(import.meta.url)
const __dirname = path.dirname(__filename)
const webDir = path.resolve(__dirname, '..')

// Source file paths
const taskTerminalPath = path.join(webDir, 'src/features/tasks/TaskTerminal.tsx')
const terminalComposerPath = path.join(webDir, 'src/features/tasks/terminal/TerminalComposer.tsx')
const visualStepsListPath = path.join(webDir, 'src/features/tasks/terminal/VisualStepsList.tsx')
const ndjsonFormatterPath = path.join(webDir, 'src/features/tasks/terminal/ndjsonFormatter.ts')
const tasksApiPath = path.join(webDir, 'src/api/tasks.ts')
const idLocalePath = path.join(webDir, 'src/locales/id.json')
const enLocalePath = path.join(webDir, 'src/locales/en.json')

test('1. S63-T2: NDJSON Log Formatter & Visual Step Parser (Things 3 Standard)', async () => {
  assert.ok(fs.existsSync(ndjsonFormatterPath), 'ndjsonFormatter.ts must exist')
  const {
    parseNdjsonLine,
    formatLogLineForTerminal,
    parseLogsToVisualSteps,
    formatDuration,
    mapToolMetadata,
  } = await import('../src/features/tasks/terminal/ndjsonFormatter.ts')

  // Tool mapping
  assert.equal(mapToolMetadata('run_command').displayName, 'Run Command')
  assert.equal(mapToolMetadata('write_file').displayName, 'Write File')
  assert.equal(mapToolMetadata('read_file').displayName, 'Read File')
  assert.equal(mapToolMetadata('search_web').displayName, 'Search Web')
  assert.equal(mapToolMetadata('thought').displayName, 'Agent Thought')

  // Duration formatting
  assert.equal(formatDuration(150), '150ms')
  assert.equal(formatDuration(1500), '1.5s')

  // Parse sample raw NDJSON tool_call
  const sampleToolCall = JSON.stringify({
    type: 'tool_call',
    step: 1,
    tool_call: {
      name: 'run_command',
      args: { CommandLine: 'git status', toolSummary: 'Git status check' },
    },
    duration_ms: 120,
    status: 'completed',
  })

  const parsed = parseNdjsonLine(sampleToolCall, 0)
  assert.ok(parsed, 'Must successfully parse valid NDJSON line')
  assert.equal(parsed.tool, 'run_command')
  assert.equal(parsed.toolDisplayName, 'Run Command')
  assert.equal(parsed.summary, 'Git status check')
  assert.equal(parsed.duration, '120ms')
  assert.equal(parsed.status, 'completed')

  // Formatted ANSI output for xterm buffer (replaces ugly raw JSON blobs)
  const ansiLine = formatLogLineForTerminal(sampleToolCall)
  assert.ok(!ansiLine.includes('{"type":"tool_call"'), 'Must not dump raw JSON in ANSI line')
  assert.ok(ansiLine.includes('Run Command'), 'Must include tool display name')
  assert.ok(ansiLine.includes('Git status check'), 'Must include summary')
  assert.ok(ansiLine.includes('(120ms)'), 'Must include duration badge')

  // Regular non-JSON lines must remain unchanged
  const plainLine = 'Compiling TypeScript assets...'
  assert.equal(formatLogLineForTerminal(plainLine), plainLine)

  // Aggregate multiple logs
  const logs = [
    { id: 1, line: sampleToolCall },
    {
      id: 2,
      line: JSON.stringify({
        step: 2,
        name: 'write_file',
        args: { Path: 'src/App.tsx' },
        duration_ms: 85,
        status: 'completed',
      }),
    },
    { id: 3, line: 'Just a plain output string' },
  ]
  const steps = parseLogsToVisualSteps(logs)
  assert.equal(steps.length, 2, 'Must extract exactly 2 visual steps from logs')
  assert.equal(steps[0].tool, 'run_command')
  assert.equal(steps[1].tool, 'write_file')
})

test('2. S63-T2: VisualStepsList Component & Collapsible Step Disclosure', () => {
  assert.ok(fs.existsSync(visualStepsListPath), 'VisualStepsList.tsx must exist')
  const content = fs.readFileSync(visualStepsListPath, 'utf8')

  // Check component exports and structure
  assert.ok(content.includes('export function VisualStepsList'), 'Must export VisualStepsList')
  assert.ok(content.includes('getStepIcon'), 'Must map icons based on step category and status')
  assert.ok(content.includes('toggleExpand'), 'Must support interactive collapse/expand of step details')
  assert.ok(content.includes('handleCopy'), 'Must provide 1-click copy for step details/output')

  // Things 3 Low Visual Noise & Hairline Card architecture
  assert.ok(content.includes('border-border/40'), 'Must use subtle hairline border border-border/40')
  assert.ok(content.includes('bg-card/40'), 'Must use calm flat background bg-card/40')
  assert.ok(content.includes('shadow-2xs'), 'Must use minimal 2xs shadow instead of harsh drop-shadow')
})

test('3. S63-T2: TaskTerminal Direct PTY Terminal (Pure Full-Viewport xterm.js)', () => {
  const content = fs.readFileSync(taskTerminalPath, 'utf8')

  // Direct full-viewport mount
  assert.ok(content.includes('task-terminal-viewport'), 'Must mount direct task-terminal-viewport')
  assert.ok(!content.includes('viewMode'), 'Must eliminate viewMode and steps toggle')
  assert.ok(!content.includes('<VisualStepsList'), 'Must eliminate VisualStepsList abstraction in favor of live terminal')

  // ANSI formatting on log write
  assert.ok(
    content.includes('formatLogLineForTerminal'),
    'TaskTerminal must format raw NDJSON into human-readable ANSI before writing to xterm',
  )
})

test('4. S63-T2: TerminalComposer & Action Controls Unlocked (No Legacy liveId Protection)', () => {
  const content = fs.readFileSync(taskTerminalPath, 'utf8')

  // TerminalComposer must not be gated behind legacy liveId
  assert.ok(
    content.includes('(isTaskActive || Boolean(activeSessionId || live?.session_id))'),
    'TerminalComposer must be active during active task attempts regardless of legacy liveId',
  )

  // Floating Stop / Interrupt control
  assert.ok(
    content.includes('handleStop'),
    'TaskTerminal must define handleStop for clean cancellation and interrupt',
  )
  assert.ok(
    content.includes('tasksApi.cancel(taskId)'),
    'handleStop must call tasksApi.cancel to stop task execution',
  )


})

test('5. S63-T2: Interactive Mid-Run Steering via POST /api/v1/runner/attempts/{id}/steer', () => {
  const terminalContent = fs.readFileSync(taskTerminalPath, 'utf8')
  const composerContent = fs.readFileSync(terminalComposerPath, 'utf8')
  const apiContent = fs.readFileSync(tasksApiPath, 'utf8')

  // tasksApi.steer endpoint contract
  assert.ok(
    apiContent.includes('/api/v1/runner/attempts/'),
    'tasksApi must define runner attempt endpoint',
  )
  assert.ok(
    apiContent.includes('/steer'),
    'tasksApi must define steer endpoint',
  )

  // TaskTerminal connects handleSteer to tasksApi.steer and PTY stdin
  assert.ok(
    terminalContent.includes('tasksApi.steer(attempt.id, textToSend)'),
    'TaskTerminal must inject steering directives to tasksApi.steer',
  )
  assert.ok(
    terminalContent.includes('onSteer={handleSteer}'),
    'TaskTerminal must pass handleSteer to TerminalComposer',
  )

  // TerminalComposer wires onSteer to user submission
  assert.ok(
    composerContent.includes('onSteer?: (prompt: string) =>'),
    'TerminalComposer must accept onSteer callback',
  )
  assert.ok(
    composerContent.includes('void onSteer(toSend)'),
    'TerminalComposer must invoke onSteer on submission',
  )
  assert.ok(
    composerContent.includes('data-attempt-id={attemptId}'),
    'TerminalComposer must support attemptId binding',
  )
})

test('6. S63-T2: Localization Completeness (id.json & en.json)', () => {
  const idLocale = JSON.parse(fs.readFileSync(idLocalePath, 'utf8'))
  const enLocale = JSON.parse(fs.readFileSync(enLocalePath, 'utf8'))

  const requiredKeys = [
    'Visual Steps',
    'Terminal',
    'Steering instruction delivered to agent.',
    'Steering instruction queued for agent.',
    'Could not deliver steering instruction.',
    'Stopping task execution…',
    'Task execution stopped.',
    'Stop / Interrupt',
    'Stop / Interrupt (Ctrl+C)',
  ]

  for (const key of requiredKeys) {
    assert.ok(idLocale[key], `id.json must contain translation for "${key}"`)
    assert.ok(enLocale[key], `en.json must contain entry for "${key}"`)
  }
})
