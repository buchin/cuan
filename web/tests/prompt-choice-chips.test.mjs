import test from 'node:test'
import assert from 'node:assert/strict'
import fs from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'

const __dirname = path.dirname(fileURLToPath(import.meta.url))
const chipsPath = path.resolve(__dirname, '../src/features/inbox/PromptChoiceChips.tsx')
const inboxViewPath = path.resolve(__dirname, '../src/features/inbox/InboxView.tsx')
const chatApiPath = path.resolve(__dirname, '../src/api/chat.ts')

test('1. PromptChoiceChips component and TypeScript contracts exist', () => {
  assert.ok(fs.existsSync(chipsPath), 'features/inbox/PromptChoiceChips.tsx must exist')

  const chipsSrc = fs.readFileSync(chipsPath, 'utf8')
  assert.ok(chipsSrc.includes('export function PromptChoiceChips'), 'PromptChoiceChips must be exported')
  assert.ok(chipsSrc.includes('export interface PromptChoiceChipsProps'), 'PromptChoiceChipsProps must be exported')

  const chatApiSrc = fs.readFileSync(chatApiPath, 'utf8')
  assert.ok(chatApiSrc.includes('export interface PromptOption'), 'api/chat.ts must export PromptOption')
  assert.ok(chatApiSrc.includes('export interface PromptChoices'), 'api/chat.ts must export PromptChoices')
  assert.ok(chatApiSrc.includes('prompt_choices?: PromptChoices'), 'ChatMessage must include prompt_choices field')
})

test('2. PromptChoiceChips conforms to Zero Cognitive Fatigue and Mobile Touch Targets (>=44px min-h-11)', () => {
  const chipsSrc = fs.readFileSync(chipsPath, 'utf8')

  // WCAG touch target: min-h-11 (44px)
  assert.ok(chipsSrc.includes('min-h-11'), 'PromptChoiceChips must have min-h-11 for 44px mobile touch targets')
  // Things 3 / Bear Notes calm design: rounded-xl
  assert.ok(chipsSrc.includes('rounded-xl'), 'Buttons must use rounded-xl for clean, calm visual hierarchy')
  // Interactive role group & accessibility
  assert.ok(chipsSrc.includes('role="group"'), 'Chips container must declare role="group"')
  assert.ok(chipsSrc.includes('data-testid="prompt-choice-chips"'), 'Container must have data-testid="prompt-choice-chips"')
})

test('3. InboxView renders PromptChoiceChips and wires 1-tap dispatch via handleSendMessage', () => {
  const inboxSrc = fs.readFileSync(inboxViewPath, 'utf8')

  assert.ok(inboxSrc.includes("import { PromptChoiceChips } from './PromptChoiceChips'"), 'InboxView must import PromptChoiceChips')
  assert.ok(inboxSrc.includes('<PromptChoiceChips'), 'InboxView must render PromptChoiceChips')
  assert.ok(inboxSrc.includes('msg.prompt_choices'), 'InboxView must guard with msg.prompt_choices')
  assert.ok(inboxSrc.includes('overrideText?: string'), 'handleSendMessage must support overrideText for 1-tap sending')
})
