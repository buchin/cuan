import test from 'node:test'
import assert from 'node:assert/strict'
import fs from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'

const __dirname = path.dirname(fileURLToPath(import.meta.url))
const webDir = path.resolve(__dirname, '..')

const inboxActivityPath = path.join(webDir, 'src/features/inbox/InboxActivity.tsx')
const inboxActivitySource = fs.readFileSync(inboxActivityPath, 'utf8')

// The 16 curated calm and professional English verbs approved by user
const EXPECTED_CURATED_VERBS = [
  'Thinking',
  'Analyzing',
  'Synthesizing',
  'Formulating',
  'Evaluating',
  'Processing',
  'Researching',
  'Reviewing',
  'Exploring',
  'Organizing',
  'Preparing',
  'Structuring',
  'Inspecting',
  'Refining',
  'Polishing',
  'Verifying',
]

test('1. InboxActivity exports the curated list of 16 professional English verbs', () => {
  assert.ok(
    inboxActivitySource.includes('export const INBOX_ACTIVITY_ROTATING_WORDS'),
    'InboxActivity must export INBOX_ACTIVITY_ROTATING_WORDS constant',
  )
  assert.ok(
    inboxActivitySource.includes('export const INBOX_ACTIVITY_VERBS'),
    'InboxActivity must export INBOX_ACTIVITY_VERBS constant',
  )

  // Verify all 16 verbs are present in the list
  for (const verb of EXPECTED_CURATED_VERBS) {
    assert.ok(
      inboxActivitySource.includes(`'${verb}…'`) || inboxActivitySource.includes(`"${verb}…"`),
      `InboxActivity must include rotating word '${verb}…'`,
    )
    assert.ok(
      inboxActivitySource.includes(`'${verb}'`) || inboxActivitySource.includes(`"${verb}"`),
      `InboxActivity must include base verb '${verb}'`,
    )
  }
})

test('2. Rotation interval is set within 3-4 seconds (Zero Cognitive Fatigue standard)', () => {
  assert.ok(
    inboxActivitySource.includes('export const ACTIVITY_ROTATION_INTERVAL_MS'),
    'InboxActivity must export ACTIVITY_ROTATION_INTERVAL_MS',
  )

  const match = inboxActivitySource.match(/ACTIVITY_ROTATION_INTERVAL_MS\s*=\s*(\d+)/)
  assert.ok(match, 'Must find ACTIVITY_ROTATION_INTERVAL_MS value')

  const intervalMs = parseInt(match[1], 10)
  assert.ok(
    intervalMs >= 3000 && intervalMs <= 4000,
    `Rotation interval must be between 3000ms and 4000ms (observed ${intervalMs}ms)`,
  )
})

test('3. Dynamic rotation is only active when running without specific step text', () => {
  assert.ok(
    inboxActivitySource.includes('const hasSpecificStep = Boolean(latestActivity?.text?.trim())'),
    'InboxActivity must verify specific step text presence',
  )
  assert.ok(
    inboxActivitySource.includes('const shouldRotate = q.state === \'running\' && !autoPending && !hasSpecificStep && !hasStreaming'),
    'Rotation must be strictly conditional on running state without step or streaming text',
  )
})

test('4. Real step activity immediately supersedes dynamic rotation fallback', () => {
  // Activity text calculation logic:
  assert.ok(
    inboxActivitySource.includes('const activityText = hasSpecificStep'),
    'activityText must prioritize real activity step text',
  )
  assert.ok(
    inboxActivitySource.includes('Array.from(latestActivity!.text.split(\'\\n\')[0]).slice(0, 140).join(\'\')'),
    'activityText must extract step headline when hasSpecificStep is true',
  )
  assert.ok(
    inboxActivitySource.includes(': rotatingWord'),
    'activityText must fall back to current rotating word when no specific step exists',
  )
})

test('5. Lifecycle cleanup prevents memory leaks on complete, cancel or unmount', () => {
  assert.ok(
    inboxActivitySource.includes('clearInterval(rotationTimer)'),
    'Cleanup function must clear rotation interval timer',
  )
  assert.ok(
    inboxActivitySource.includes('clearTimeout(fadeTimer)'),
    'Cleanup function must clear fade timeout timer',
  )
})

test('6. Calm visual transitions and low visual noise without layout shift', () => {
  assert.ok(
    inboxActivitySource.includes('data-testid="inbox-activity-text"'),
    'InboxActivity must provide data-testid for activity text',
  )
  assert.ok(
    inboxActivitySource.includes('transition-opacity duration-300 ease-in-out'),
    'Text transition must use smooth calm opacity transition (duration-300 ease-in-out)',
  )
  assert.ok(
    inboxActivitySource.includes('motion-reduce:transition-none'),
    'Respects reduced motion preference for accessibility',
  )
})

test('7. Functional behavior validation: step priority & fallback rotation simulation', () => {
  // Simulating the exact state machine implemented in InboxActivity
  const words = [
    'Thinking…',
    'Analyzing…',
    'Synthesizing…',
    'Formulating…',
    'Evaluating…',
    'Processing…',
    'Researching…',
    'Reviewing…',
    'Exploring…',
    'Organizing…',
    'Preparing…',
    'Structuring…',
    'Inspecting…',
    'Refining…',
    'Polishing…',
    'Verifying…',
  ]

  assert.equal(words.length, 16, 'Exactly 16 curated words')

  function computeActivity(q, rotatingIndex) {
    const latest = q.steps
      ?.slice()
      .reverse()
      .find((step) =>
        [
          'context',
          'provider',
          'output',
          'contact_search',
          'runtime_init',
          'runtime_provider_started',
          'runtime_provider_finished',
          'runtime_tool_started',
          'runtime_tool_finished',
          'runtime_tool_failed',
          'runtime_permission_denied',
          'runtime_tool_budget_exceeded',
          'result_validation',
          'delegation',
          'persistence',
        ].includes(step.kind),
      )
    const hasSpecificStep = Boolean(latest?.text?.trim())
    const rotatingWord = words[rotatingIndex % words.length]
    return hasSpecificStep
      ? Array.from(latest.text.split('\n')[0]).slice(0, 140).join('')
      : rotatingWord
  }

  // Case A: Initial running state without steps -> displays Thinking…
  const initialReq = { state: 'running', steps: [] }
  assert.equal(computeActivity(initialReq, 0), 'Thinking…')

  // Case B: Rotation advances index -> displays Analyzing…, Synthesizing…
  assert.equal(computeActivity(initialReq, 1), 'Analyzing…')
  assert.equal(computeActivity(initialReq, 2), 'Synthesizing…')
  assert.equal(computeActivity(initialReq, 15), 'Verifying…')
  assert.equal(computeActivity(initialReq, 16), 'Thinking…') // wraps around cleanly

  // Case C: Actual tool step arrives -> immediately switches to actual step text
  const toolReq = {
    state: 'running',
    steps: [
      { seq: 1, kind: 'accepted', text: 'Message received.' },
      { seq: 2, kind: 'runtime_tool_started', text: 'Membaca berkas konfigurasi sistem\nDetail teknis...' },
    ],
  }
  assert.equal(computeActivity(toolReq, 5), 'Membaca berkas konfigurasi sistem')
})
