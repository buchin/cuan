import test from 'node:test'
import assert from 'node:assert'
import fs from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'

const __filename = fileURLToPath(import.meta.url)
const __dirname = path.dirname(__filename)

const studioViewPath = path.join(__dirname, '../src/features/inbox/StudioView.tsx')
const executionSelectorPath = path.join(__dirname, '../src/features/inbox/StudioExecutionSelector.tsx')

const studioViewSource = fs.readFileSync(studioViewPath, 'utf8')
const selectorSource = fs.readFileSync(executionSelectorPath, 'utf8')

test('1. StudioExecutionSelector: Single unified smart pill & tabbed popover', () => {
  assert.ok(
    selectorSource.includes('studio-execution-selector-pill'),
    'StudioExecutionSelector must render unified pill button with id studio-execution-selector-pill'
  )
  assert.ok(
    selectorSource.includes('rounded-full'),
    'Pill button must have rounded-full compact styling'
  )
  assert.ok(
    selectorSource.includes("t('Auto')"),
    'Default pill label must simply be Auto (clean and non-intimidating for non-developers)'
  )
  assert.ok(
    selectorSource.includes("activeTab === 'project'") && selectorSource.includes("activeTab === 'agent'"),
    'StudioExecutionSelector must provide clean tabbed organization for Project and Agent'
  )
  assert.ok(
    selectorSource.includes("t('Reset ke Otomatis')"),
    'StudioExecutionSelector must provide quick reset to Auto when customized'
  )
  assert.ok(
    selectorSource.includes("onConnectProjectClick"),
    'StudioExecutionSelector must preserve connect project action'
  )
  assert.ok(
    selectorSource.includes("onAddAgentClick"),
    'StudioExecutionSelector must preserve add agent action'
  )
})

test('2. StudioView: Clean single-row composer toolbar on mobile and desktop', () => {
  assert.ok(
    !studioViewSource.includes('<StudioProjectSelector'),
    'Studio leaves project discovery to the conversation'
  )
  assert.ok(
    studioViewSource.includes('flex items-center justify-between'),
    'Toolbar must use items-center justify-between for single-row alignment'
  )
  assert.ok(
    studioViewSource.includes('action-primary shrink-0'),
    'Submit button must be shrink-0 so it stays on the same line on mobile'
  )
  assert.ok(
    !studioViewSource.includes('<StudioAgentSelector') && !studioViewSource.includes('<StudioExecutionSelector'),
    'StudioView must not render two separate competing pill buttons side-by-side'
  )
})

test('3. StudioView: Add agent trigger opens TeamView dialog without premature unmounting', () => {
  assert.ok(
    studioViewSource.includes("agents.find((a) => a.id === 'agent_cuan')") && !studioViewSource.includes('onAddAgentClick='),
    'Studio must use the public assistant without account selection'
  )
  assert.ok(
    studioViewSource.includes('dialogOnly={agentSetup !== \'needed\'}'),
    'StudioView must pass dialogOnly so TeamView only shows the modal dialog without page content'
  )
  assert.ok(
    studioViewSource.includes('onClose={() => {'),
    'StudioView must pass onClose callback to cleanly dismiss modal on cancel or completion'
  )
})
