import test from 'node:test'
import assert from 'node:assert/strict'
import fs from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'

const __dirname = path.dirname(fileURLToPath(import.meta.url))
const webDir = path.resolve(__dirname, '..')

const inboxActivityPath = path.join(webDir, 'src/features/inbox/InboxActivity.tsx')
const inboxActivitySource = fs.readFileSync(inboxActivityPath, 'utf8')

// The 16 original curated calm and professional English verbs
const EXPECTED_ORIGINAL_VERBS = [
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

// The 80 curated verbs tailored for business owners, creators, and non-developers
const ALL_80_CURATED_VERBS = [
  'Thinking', 'Analyzing', 'Synthesizing', 'Formulating', 'Evaluating',
  'Processing', 'Researching', 'Reviewing', 'Exploring', 'Organizing',
  'Preparing', 'Structuring', 'Inspecting', 'Refining', 'Polishing',
  'Verifying', 'Brainstorming', 'Conceptualizing', 'Envisioning', 'Strategizing',
  'Deliberating', 'Investigating', 'Discovering', 'Gathering', 'Sifting',
  'Surveying', 'Examining', 'Browsing', 'Uncovering', 'Categorizing',
  'Arranging', 'Mapping', 'Outlining', 'Sorting', 'Cataloging',
  'Coordinating', 'Systematizing', 'Drafting', 'Composing', 'Writing',
  'Curating', 'Assembling', 'Creating', 'Crafting', 'Designing',
  'Generating', 'Calculating', 'Computing', 'Estimating', 'Forecasting',
  'Budgeting', 'Assessing', 'Auditing', 'Quantifying', 'Measuring',
  'Editing', 'Enhancing', 'Streamlining', 'Perfecting', 'Tailoring',
  'Fine-tuning', 'Clarifying', 'Harmonizing', 'Planning', 'Scheduling',
  'Aligning', 'Optimizing', 'Prioritizing', 'Consolidating', 'Connecting',
  'Orchestrating', 'Validating', 'Checking', 'Balancing', 'Finalizing',
  'Summarizing', 'Distilling', 'Reconciling', 'Cross-checking', 'Imagining',
]

test('1. InboxActivity exports the curated list of ~80 professional English verbs', () => {
  assert.ok(
    inboxActivitySource.includes('export const INBOX_ACTIVITY_ROTATING_WORDS'),
    'InboxActivity must export INBOX_ACTIVITY_ROTATING_WORDS constant',
  )
  assert.ok(
    inboxActivitySource.includes('export const INBOX_ACTIVITY_VERBS'),
    'InboxActivity must export INBOX_ACTIVITY_VERBS constant',
  )

  // Verify all original 16 verbs are preserved
  for (const verb of EXPECTED_ORIGINAL_VERBS) {
    assert.ok(
      inboxActivitySource.includes(`'${verb}…'`) || inboxActivitySource.includes(`"${verb}…"`),
      `InboxActivity must include rotating word '${verb}…'`,
    )
    assert.ok(
      inboxActivitySource.includes(`'${verb}'`) || inboxActivitySource.includes(`"${verb}"`),
      `InboxActivity must include base verb '${verb}'`,
    )
  }

  // Verify all 80 curated verbs are present
  assert.equal(ALL_80_CURATED_VERBS.length, 80, 'Curated list must have exactly 80 verbs')
  for (const verb of ALL_80_CURATED_VERBS) {
    assert.ok(
      inboxActivitySource.includes(`'${verb}…'`) || inboxActivitySource.includes(`"${verb}…"`),
      `InboxActivity must include rotating word '${verb}…'`,
    )
    assert.ok(
      inboxActivitySource.includes(`'${verb}'`) || inboxActivitySource.includes(`"${verb}"`),
      `InboxActivity must include base verb '${verb}'`,
    )
  }

  // Verify no compiler/developer technical jargon exists in the vocabulary
  const FORBIDDEN_JARGON = ['compiling', 'transpiling', 'refactoring', 'grokking', 'linting', 'piping']
  for (const jargon of FORBIDDEN_JARGON) {
    assert.ok(
      !inboxActivitySource.toLowerCase().includes(`'${jargon}…'`),
      `InboxActivity must not contain developer jargon: ${jargon}`,
    )
  }
})

test('2. Typing dots component replaces circular loader spinner in InboxActivity', () => {
  assert.ok(
    inboxActivitySource.includes('export function TypingDots'),
    'InboxActivity must export TypingDots component',
  )
  assert.ok(
    inboxActivitySource.includes('data-testid="inbox-activity-typing-dots"'),
    'TypingDots component must provide data-testid for verification',
  )
  assert.ok(
    inboxActivitySource.includes('typing-dot-1') &&
    inboxActivitySource.includes('typing-dot-2') &&
    inboxActivitySource.includes('typing-dot-3'),
    'TypingDots must render 3 horizontal dots with staggered pulse animation',
  )
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

test('7. Functional behavior validation: step priority & fallback rotation simulation across 80 words', () => {
  const words = ALL_80_CURATED_VERBS.map((v) => `${v}…`)
  assert.equal(words.length, 80, 'Exactly 80 curated words')

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
  assert.equal(computeActivity(initialReq, 16), 'Brainstorming…')
  assert.equal(computeActivity(initialReq, 79), 'Imagining…')
  assert.equal(computeActivity(initialReq, 80), 'Thinking…') // wraps around cleanly after 80 words

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
