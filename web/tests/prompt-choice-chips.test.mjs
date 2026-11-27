import test from 'node:test'
import assert from 'node:assert/strict'
import fs from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'

const __dirname = path.dirname(fileURLToPath(import.meta.url))
const chipsPath = path.resolve(__dirname, '../src/features/inbox/PromptChoiceChips.tsx')
const inboxViewPath = path.resolve(__dirname, '../src/features/inbox/InboxView.tsx')
const chatApiPath = path.resolve(__dirname, '../src/api/chat.ts')
const chatCalmCssPath = path.resolve(__dirname, '../src/features/inbox/chat-calm.css')

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
  // Shared Button component enforces the square bordered button design system
  assert.ok(chipsSrc.includes("from '../../components/ui/button'") && chipsSrc.includes('<Button'), 'Buttons must reuse shared Button component')
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

test('4. PromptChoiceChips container is transparent without nested box card', () => {
  const chipsSrc = fs.readFileSync(chipsPath, 'utf8')
  assert.ok(chipsSrc.includes('bg-transparent'), 'Container must have bg-transparent')
  assert.ok(chipsSrc.includes('border-0'), 'Container must have border-0')
  assert.ok(chipsSrc.includes('shadow-none'), 'Container must have shadow-none')
  assert.ok(!chipsSrc.includes('CardActions'), 'PromptChoiceChips must not use CardActions with solid background')

  const cssSrc = fs.readFileSync(chatCalmCssPath, 'utf8')
  assert.ok(cssSrc.includes('.prompt-choice-container'), 'chat-calm.css must style .prompt-choice-container')
  assert.ok(cssSrc.includes('background: transparent'), '.prompt-choice-container must have transparent background')
})

test('5. Option buttons feature clean subtle pill/border ghost styling with smooth hover transitions', () => {
  const chipsSrc = fs.readFileSync(chipsPath, 'utf8')
  assert.ok(chipsSrc.includes('border-border/40'), 'Buttons must use subtle border-border/40')
  assert.ok(chipsSrc.includes('hover:border-border/70'), 'Buttons must have smooth hover border transition')
  assert.ok(chipsSrc.includes('bg-transparent hover:bg-muted/20'), 'Buttons must have subtle ghost hover background')
  assert.ok(chipsSrc.includes('transition-all duration-150'), 'Buttons must have smooth 150ms transitions')
})

test('6. InboxView avoids wrapping prompt choices in solid message bubbles when no bubble content exists', () => {
  const inboxSrc = fs.readFileSync(inboxViewPath, 'utf8')
  assert.ok(inboxSrc.includes('hasBubbleContent && ('), 'wa-message-bubble must only render when bubble content exists')
})
