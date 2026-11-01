import test from 'node:test'
import assert from 'node:assert/strict'
import fs from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'

const __filename = fileURLToPath(import.meta.url)
const __dirname = path.dirname(__filename)
const webDir = path.resolve(__dirname, '..')

const inboxViewPath = path.join(webDir, 'src/features/inbox/InboxView.tsx')
const chatCalmCssPath = path.join(webDir, 'src/features/inbox/chat-calm.css')
const agentMemoryViewPath = path.join(webDir, 'src/features/settings/AgentMemoryView.tsx')

test('1. S62-T2: 0 horizontal wireframe 1px borders cutting chat canvas & composer', () => {
  assert.ok(fs.existsSync(chatCalmCssPath), 'chat-calm.css must exist')
  assert.ok(fs.existsSync(inboxViewPath), 'InboxView.tsx must exist')

  const cssSource = fs.readFileSync(chatCalmCssPath, 'utf8')
  const inboxSource = fs.readFileSync(inboxViewPath, 'utf8')

  // Header border-bottom elimination
  assert.ok(
    !cssSource.includes('border-bottom: 1px solid hsl(var(--border) / 0.4);'),
    'chat-calm.css must eliminate horizontal 1px border-bottom from chat header',
  )
  assert.ok(
    cssSource.includes('border-bottom: none;'),
    'chat-calm.css must explicitly set border-bottom: none for header',
  )
  assert.ok(
    inboxSource.includes('wa-chat-header') && inboxSource.includes('border-b-0'),
    'InboxView.tsx chat header container must include border-b-0',
  )

  // Composer border-top elimination
  assert.ok(
    !cssSource.includes('border-top: 1px solid hsl(var(--border) / 0.35);'),
    'chat-calm.css must eliminate horizontal 1px border-top from chat composer',
  )
  assert.ok(
    cssSource.includes('border-top: none;'),
    'chat-calm.css must explicitly set border-top: none for composer',
  )
  assert.ok(
    inboxSource.includes('data-inbox-composer') && inboxSource.includes('border-t-0'),
    'InboxView.tsx composer container must include border-t-0',
  )
})

test('2. S62-T2: Humanized Mobile & Desktop Chat Header with Quiet Status Indicator and Back Nav', () => {
  const inboxSource = fs.readFileSync(inboxViewPath, 'utf8')

  // Contact display name resolution avoiding developer ticket titles
  assert.ok(
    inboxSource.includes('contactDisplayName'),
    'InboxView.tsx must compute humanized contactDisplayName',
  )
  assert.ok(
    inboxSource.includes('agentForContact'),
    'InboxView.tsx must look up agentForContact to resolve clean agent identity',
  )
  assert.ok(
    inboxSource.includes('Hermes Support'),
    'InboxView.tsx must provide friendly default agent name fallback',
  )

  // Header identity rendering
  assert.ok(
    inboxSource.includes('{contactDisplayName || activeContact.name}'),
    'InboxView.tsx must render resolved humanized contact/agent name in header',
  )

  // Mobile Back Button (<) preserved
  assert.ok(
    inboxSource.includes('onClick={handleBackToContacts}'),
    'InboxView.tsx must preserve handleBackToContacts back navigation button',
  )
  assert.ok(
    inboxSource.includes('ChevronLeft'),
    'InboxView.tsx must render ChevronLeft icon for mobile back button',
  )
  assert.ok(
    inboxSource.includes('min-[1200px]:hidden'),
    'Mobile back button must be visible on mobile and tablet viewports',
  )

  // Quiet status indicator
  assert.ok(
    inboxSource.includes('<ContactStatus'),
    'InboxView.tsx must render ContactStatus component',
  )
  assert.ok(
    !inboxSource.includes("className={activeContact.type === 'agent' ? 'hidden md:block' : ''}"),
    'ContactStatus must not be hidden on mobile viewports for agent contacts',
  )
})

test('3. S62-T2: Total elimination of TasteGroundingChips from chat bubble canvas', () => {
  const inboxSource = fs.readFileSync(inboxViewPath, 'utf8')

  // TasteGroundingChips must not be imported in InboxView
  assert.ok(
    !inboxSource.includes("import { TasteGroundingChips } from './TasteGroundingChips'"),
    'InboxView.tsx must NOT import TasteGroundingChips',
  )

  // TasteGroundingChips must not be rendered in InboxView
  assert.ok(
    !inboxSource.includes('<TasteGroundingChips'),
    'InboxView.tsx must NOT render TasteGroundingChips on chat bubbles',
  )

  // Configuration remains centralized in Settings > AI Memory & Taste
  assert.ok(
    fs.existsSync(agentMemoryViewPath),
    'AgentMemoryView.tsx must remain intact as central memory configuration view',
  )
  const memorySource = fs.readFileSync(agentMemoryViewPath, 'utf8')
  assert.ok(
    memorySource.includes('Taste') || memorySource.includes('Memory'),
    'AgentMemoryView must manage taste and memory configuration',
  )
})

test('4. S62-T2: Floating borderless soft surface composer and quick reply lightning guard', () => {
  const cssSource = fs.readFileSync(chatCalmCssPath, 'utf8')
  const inboxSource = fs.readFileSync(inboxViewPath, 'utf8')

  // Borderless floating surface styling
  assert.ok(
    cssSource.includes('border: 1px solid transparent;'),
    'wa-input-capsule must use border: 1px solid transparent (borderless)',
  )
  assert.ok(
    cssSource.includes('.dark .wa-input-capsule') && cssSource.includes('#1e2023'),
    'Dark mode wa-input-capsule must use #1e2023 soft surface tint',
  )
  assert.ok(
    cssSource.includes('border-color: transparent;'),
    'Dark mode wa-input-capsule must have border-color: transparent',
  )
  assert.ok(
    cssSource.includes('box-shadow: 0 2px 10px rgb(0 0 0 / 0.25);'),
    'Dark mode wa-input-capsule must have floating soft surface elevation',
  )

  // Quick reply lightning bolt (⚡) hidden on agent chat
  assert.ok(
    inboxSource.includes("activeContact.type === 'customer' && (\n                    <CannedResponsesPicker") ||
    inboxSource.includes("activeContact.type === 'customer' && (\n                      <CannedResponsesPicker") ||
    (inboxSource.includes("activeContact.type === 'customer'") && inboxSource.includes('<CannedResponsesPicker')),
    'CannedResponsesPicker (⚡) must only be rendered for customer chats and hidden for AI agents',
  )
})
