import test from 'node:test'
import assert from 'node:assert'
import fs from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'

const __filename = fileURLToPath(import.meta.url)
const __dirname = path.dirname(__filename)

const connectionPath = path.join(__dirname, '../src/features/agents/connection.ts')
const studioViewPath = path.join(__dirname, '../src/features/inbox/StudioView.tsx')
const selectorPath = path.join(__dirname, '../src/features/inbox/StudioAgentSelector.tsx')

const connectionSource = fs.readFileSync(connectionPath, 'utf8')
const studioViewSource = fs.readFileSync(studioViewPath, 'utf8')
const selectorSource = fs.readFileSync(selectorPath, 'utf8')

test('1. connection.ts: No 5-minute degradation for verified connection state', () => {
  assert.ok(
    !connectionSource.includes('verified + 5 * 60_000 <= now'),
    'connection.ts must not degrade verified connection state to unknown after 5 minutes'
  )
  assert.ok(
    connectionSource.includes("verified: 'Connected'"),
    'connection.ts must maintain label Connected for verified state'
  )
})

test('2. StudioAgentSelector: Compact pill button & modern popover structure', () => {
  assert.ok(
    selectorSource.includes('studio-agent-selector-pill'),
    'StudioAgentSelector must render a pill button with id studio-agent-selector-pill'
  )
  assert.ok(
    selectorSource.includes('rounded-full'),
    'Pill button must have rounded-full compact styling'
  )
  assert.ok(
    selectorSource.includes("t('Auto (choose best)')"),
    'StudioAgentSelector must provide Auto (choose best) option'
  )
  assert.ok(
    selectorSource.includes("t('Recommended')"),
    'Auto option must display Recommended badge'
  )
  assert.ok(
    selectorSource.includes('isThinking'),
    'StudioAgentSelector must differentiate Pro / Deep Thinking models from Fast models'
  )
})

test('3. StudioView: Composer toolbar integration without raw select block', () => {
  assert.ok(
    studioViewSource.includes('<StudioAgentSelector'),
    'StudioView must render StudioAgentSelector in composer toolbar'
  )
  assert.ok(
    !studioViewSource.includes('<select') && !studioViewSource.includes('id="studio-agent"'),
    'StudioView must no longer render the crude full-width <select id="studio-agent">'
  )
  assert.ok(
    studioViewSource.includes('AttachmentPicker'),
    'StudioView must keep AttachmentPicker beside StudioAgentSelector'
  )
})
