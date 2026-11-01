import test from 'node:test'
import assert from 'node:assert/strict'
import fs from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'

const __filename = fileURLToPath(import.meta.url)
const __dirname = path.dirname(__filename)
const webDir = path.resolve(__dirname, '..')

const ndjsonFormatterPath = path.join(webDir, 'src/features/tasks/terminal/ndjsonFormatter.ts')
const visualStepsListPath = path.join(webDir, 'src/features/tasks/terminal/VisualStepsList.tsx')

test('1. S64-T1: Deep Nested Runner Payload Parsing (cuan-inspect: list_tasks)', async () => {
  assert.ok(fs.existsSync(ndjsonFormatterPath), 'ndjsonFormatter.ts must exist')
  const {
    parseNdjsonLine,
    formatLogLineForTerminal,
    mapToolMetadata,
    getActionTitle,
  } = await import('../src/features/tasks/terminal/ndjsonFormatter.ts')

  // Exact runner NDJSON payload reported by user
  const runnerPayload = JSON.stringify({
    event: 'step_update',
    step_update: {
      step_index: 16,
      state: 'ACTIVE',
      step_type: 'tool',
      tool_name: 'call_mcp_tool',
      tool_info: {
        name: 'call_mcp_tool',
        parameters: {
          Arguments: {},
          ServerName: 'cuan-inspect',
          ToolName: 'list_tasks',
        },
      },
    },
  })

  const parsed = parseNdjsonLine(runnerPayload, 0)
  assert.ok(parsed, 'Must successfully parse deep nested step_update payload')
  assert.equal(parsed.stepNumber, 16, 'Must unwrap step_index 16')
  assert.equal(parsed.status, 'running', 'ACTIVE state must map to running status')
  assert.equal(parsed.tool, 'cuan-inspect: list_tasks', 'Must extract real MCP tool name')
  assert.equal(parsed.toolDisplayName, 'Inspect Tasks', 'Must map to human-friendly title "Inspect Tasks"')
  assert.equal(parsed.icon, 'search', 'cuan-inspect inspection tools must use search icon')

  // Critical deduplication test: summary must NOT be "Agent Step" or "Inspect Tasks"
  assert.notEqual(parsed.toolDisplayName, 'Agent Step', 'Must eliminate generic "Agent Step"')
  assert.notEqual(parsed.summary, 'Agent Step', 'Summary must eliminate generic "Agent Step"')
  assert.notEqual(
    parsed.summary,
    parsed.toolDisplayName,
    'Summary must not duplicate toolDisplayName when redundant',
  )
  assert.equal(parsed.summary, '', 'Summary should be empty when no distinct subtitle exists')

  // Collapsible detail payload should contain parameters
  assert.ok(parsed.details, 'Must populate details with tool parameters')
  assert.ok(parsed.details.includes('cuan-inspect'), 'Details must retain ServerName')
  assert.ok(parsed.details.includes('list_tasks'), 'Details must retain ToolName')

  // Terminal ANSI line check: no raw JSON dump, no duplicate words
  const ansiLine = formatLogLineForTerminal(runnerPayload)
  assert.ok(!ansiLine.includes('step_update'), 'Terminal line must not dump raw NDJSON')
  assert.ok(ansiLine.includes('Inspect Tasks'), 'Terminal line must show "Inspect Tasks"')
  // Ensure "Inspect Tasks" appears only once in the line
  const occurrences = (ansiLine.match(/Inspect Tasks/g) || []).length
  assert.equal(occurrences, 1, 'Terminal line must not repeat title in subtitle')
})

test('2. S64-T1: Human-Readable Action Titles & Friendly Descriptions', async () => {
  const {
    getActionTitle,
    mapToolMetadata,
    getFilename,
    getFriendlyCommandDescription,
    parseNdjsonLine,
  } = await import('../src/features/tasks/terminal/ndjsonFormatter.ts')

  // Filename extraction helper
  assert.equal(getFilename('web/src/features/tasks/terminal/ndjsonFormatter.ts'), 'ndjsonFormatter.ts')
  assert.equal(getFilename('/home/dev/repos/cuan/package.json'), 'package.json')
  assert.equal(getFilename('App.tsx'), 'App.tsx')

  // Friendly command descriptions
  assert.equal(getFriendlyCommandDescription('git status'), 'Check Git status')
  assert.equal(getFriendlyCommandDescription('git diff'), 'Check Git diff')
  assert.equal(getFriendlyCommandDescription('npm test'), 'Run test suite')
  assert.equal(getFriendlyCommandDescription('npm run build'), 'Build project assets')

  // Tool title mapping
  assert.equal(getActionTitle('cuan-inspect: list_tasks'), 'Inspect Tasks')
  assert.equal(getActionTitle('cuan-inspect: get_task'), 'Inspect Task')
  assert.equal(getActionTitle('cuan-inspect: list_projects'), 'Inspect Projects')
  assert.equal(getActionTitle('cuan-inspect: inspect_git_status'), 'Inspect Git Status')

  assert.equal(
    getActionTitle('read_file', { Path: 'web/src/App.tsx' }),
    'Read App.tsx',
  )
  assert.equal(
    getActionTitle('write_file', { Path: 'web/src/App.tsx' }),
    'Update App.tsx',
  )
  assert.equal(
    getActionTitle('edit_file', { Path: 'web/src/components/Button.tsx' }),
    'Update Button.tsx',
  )

  // Command with toolSummary takes priority
  assert.equal(
    getActionTitle('run_command', { CommandLine: 'git status', toolSummary: 'Git status check' }),
    'Git status check',
  )

  // Fallback title must be "Agent Action" (NOT "Agent Step")
  assert.equal(getActionTitle(''), 'Agent Action')
  assert.equal(getActionTitle('agent_step'), 'Agent Action')
  assert.equal(mapToolMetadata('').displayName, 'Agent Action')
  assert.equal(mapToolMetadata('agent_step').displayName, 'Agent Action')
})

test('3. S64-T1: Deduplication in ndjsonFormatter & VisualStepsList', () => {
  assert.ok(fs.existsSync(visualStepsListPath), 'VisualStepsList.tsx must exist')
  const content = fs.readFileSync(visualStepsListPath, 'utf8')

  // VisualStepsList must filter out redundant/duplicate subtitle
  assert.ok(
    content.includes('step.summary.trim().toLowerCase() !== step.toolDisplayName.trim().toLowerCase()'),
    'VisualStepsList must suppress subtitle when summary matches title',
  )
  assert.ok(
    content.includes("step.summary.trim() !== 'Agent Step'"),
    'VisualStepsList must never render generic "Agent Step" subtitle',
  )
  assert.ok(
    content.includes("step.summary.trim() !== 'Agent Action'"),
    'VisualStepsList must never render generic "Agent Action" subtitle',
  )

  // Things 3 / Bear Notes Calm Styling checks
  assert.ok(content.includes('border-border/40'), 'Must use subtle hairline border border-border/40')
  assert.ok(content.includes('bg-card/40'), 'Must use calm card background bg-card/40')
  assert.ok(content.includes('shadow-2xs'), 'Must use calm shadow-2xs without harsh visual noise')
  assert.ok(content.includes('font-mono'), 'Must use monospace font for step indicators')
})

test('4. S64-T1: Multi-Step Aggregation & State Transition Merging', async () => {
  const { parseLogsToVisualSteps } = await import('../src/features/tasks/terminal/ndjsonFormatter.ts')

  // Simulate streaming: Active event followed by Completed event with duration
  const logStream = [
    {
      id: 1,
      line: JSON.stringify({
        event: 'step_update',
        step_update: {
          step_index: 8,
          state: 'ACTIVE',
          step_type: 'tool',
          tool_name: 'call_mcp_tool',
          tool_info: {
            name: 'call_mcp_tool',
            parameters: {
              ServerName: 'cuan-inspect',
              ToolName: 'list_tasks',
            },
          },
        },
      }),
    },
    {
      id: 2,
      line: JSON.stringify({
        event: 'step_update',
        step_update: {
          step_index: 8,
          state: 'DONE',
          duration_ms: 185,
          step_type: 'tool',
          tool_name: 'call_mcp_tool',
          tool_info: {
            name: 'call_mcp_tool',
            parameters: {
              ServerName: 'cuan-inspect',
              ToolName: 'list_tasks',
            },
          },
        },
      }),
    },
    {
      id: 3,
      line: JSON.stringify({
        event: 'step_update',
        step_update: {
          step_index: 9,
          state: 'DONE',
          duration_ms: 45,
          step_type: 'tool',
          tool_name: 'read_file',
          tool_info: {
            name: 'read_file',
            parameters: {
              Path: 'web/src/App.tsx',
            },
          },
        },
      }),
    },
  ]

  const visualSteps = parseLogsToVisualSteps(logStream)

  // Step 8 active and done must merge into 1 step
  assert.equal(visualSteps.length, 2, 'Step 8 active and done must merge cleanly without duplication')
  assert.equal(visualSteps[0].stepNumber, 8)
  assert.equal(visualSteps[0].status, 'completed', 'Step 8 status must transition to completed')
  assert.equal(visualSteps[0].duration, '185ms', 'Step 8 duration must be recorded')
  assert.equal(visualSteps[0].toolDisplayName, 'Inspect Tasks')
  assert.equal(visualSteps[0].summary, '', 'Step 8 must not have duplicate summary')

  // Step 9 is read_file
  assert.equal(visualSteps[1].stepNumber, 9)
  assert.equal(visualSteps[1].status, 'completed')
  assert.equal(visualSteps[1].toolDisplayName, 'Read App.tsx')
})
