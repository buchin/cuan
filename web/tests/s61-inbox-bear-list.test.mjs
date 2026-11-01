import test from 'node:test'
import assert from 'node:assert/strict'
import fs from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'

const __filename = fileURLToPath(import.meta.url)
const __dirname = path.dirname(__filename)

const sprintDocPath = path.resolve(__dirname, '../../docs/sprints/S61.md')
const subPanelPath = path.resolve(__dirname, '../src/features/inbox/InboxSubPanel.tsx')
const sprintDocContent = fs.readFileSync(sprintDocPath, 'utf8')
const subPanelContent = fs.readFileSync(subPanelPath, 'utf8')

test('1. Single Source of Truth: docs/sprints/S61.md sprint documentation validation', () => {
  // Metadata check
  assert.ok(
    sprintDocContent.includes('**Status:** In Progress'),
    'Sprint 61 must be marked as In Progress'
  )
  assert.ok(
    sprintDocContent.includes('**Sprint Number:** 61'),
    'Sprint number must be 61'
  )
  assert.ok(
    sprintDocContent.includes('feature/s61-t1-inbox-bear-inset-list'),
    'Active branch must be feature/s61-t1-inbox-bear-inset-list'
  )
  assert.ok(
    sprintDocContent.includes('RPcd484fc4229ae192d17a0f01'),
    'Repository ID must be recorded'
  )

  // 4 Tasks in backlog
  assert.ok(
    sprintDocContent.includes('owfgwrggwctdfvzzijascynr') &&
      sprintDocContent.includes('S61-T1'),
    'Sprint 61 must include Task 1 (S61-T1) with ID owfgwrggwctdfvzzijascynr'
  )
  assert.ok(
    sprintDocContent.includes('lqidrsayqatbywpmsuenamzx') &&
      sprintDocContent.includes('S61-T2'),
    'Sprint 61 must include Task 2 (S61-T2) with ID lqidrsayqatbywpmsuenamzx'
  )
  assert.ok(
    sprintDocContent.includes('oamnkpueucgokjgncxjdnglw') &&
      sprintDocContent.includes('S61-T3'),
    'Sprint 61 must include Task 3 (S61-T3) with ID oamnkpueucgokjgncxjdnglw'
  )
  assert.ok(
    sprintDocContent.includes('guqzzawzvfgjfoaqvgikgwir') &&
      sprintDocContent.includes('S61-T4'),
    'Sprint 61 must include Task 4 (S61-T4) with ID guqzzawzvfgjfoaqvgikgwir'
  )

  // Target Market & Outcome considerations
  assert.ok(
    sprintDocContent.includes('Solopreneur') || sprintDocContent.includes('solopreneur'),
    'Sprint 61 must document considerations for solopreneurs and non-developers'
  )
  assert.ok(
    sprintDocContent.includes('Apple Mail & Bear Inset List') ||
      sprintDocContent.includes('Apple Mail'),
    'Sprint 61 outcome must document Apple Mail & Bear Inset List'
  )

  // D2 Tala diagram block check
  assert.ok(
    sprintDocContent.includes('```d2') && sprintDocContent.includes('direction: right'),
    'Sprint 61 must include D2 Tala architectural diagram'
  )
})

test('2. Requirement 2: Elimination of New Idea / Task bar and ⌘K label from InboxSubPanel', () => {
  // Total elimination of full-width [+ New Idea / Task] bar
  assert.ok(
    !subPanelContent.includes('data-testid="new-goal-quick-bar"'),
    'InboxSubPanel must not contain full-width new-goal-quick-bar button'
  )
  assert.ok(
    !subPanelContent.includes('⌘K') && !subPanelContent.includes('⌘'),
    'InboxSubPanel must eliminate ⌘K shortcut label'
  )

  // Functional keyboard shortcut listener remains active for power users
  assert.ok(
    subPanelContent.includes("e.key.toLowerCase() === 'k'") &&
      subPanelContent.includes('(e.metaKey || e.ctrlKey)'),
    'InboxSubPanel must retain background Cmd+K / Ctrl+K keyboard shortcut listener'
  )
})

test('3. Requirement 3: Neutral & calm filter tabs (Things 3 / Bear Notes segment style)', () => {
  // No loud neon blue buttons on filter tabs
  assert.ok(
    !subPanelContent.includes("activeTab === 'all'\n                ? 'bg-primary text-primary-foreground"),
    'Filter tab All must not use loud primary neon button style'
  )

  // Uses calm neutral segment styling (bg-secondary text-foreground)
  assert.ok(
    subPanelContent.includes('bg-secondary text-foreground font-semibold'),
    'Active filter tabs must adopt calm neutral Things 3 / Bear Notes segment styling'
  )

  // Preserves accessibility touch target >= 44px
  assert.ok(
    subPanelContent.includes('min-h-11') &&
      subPanelContent.includes('rounded-full px-3.5 py-1.5'),
    'Filter buttons must maintain WCAG min-h-11 touch target and standard padding'
  )

  // Preserves All, Agents, Customers, and Archived filter options
  assert.ok(subPanelContent.includes("t('All')"), 'Filter must include All tab')
  assert.ok(subPanelContent.includes("t('Agents')"), 'Filter must include Agents tab')
  assert.ok(subPanelContent.includes("t('Customers')"), 'Filter must include Customers tab')
  assert.ok(subPanelContent.includes('Archive'), 'Filter must include Archive toggle')
})

test('4. Requirement 4: ConversationItem 3-Tier Apple Mail & Bear Inset List Architecture', () => {
  // Exported component and props interface exist
  assert.ok(
    subPanelContent.includes('export function ConversationItem') &&
      subPanelContent.includes('export interface ConversationItemProps'),
    'InboxSubPanel must export ConversationItem and its props interface'
  )

  // Hairline divider and no nested box cards
  assert.ok(
    subPanelContent.includes('border-b border-border/40'),
    'ConversationItem must enforce hairline divider (border-b border-border/40)'
  )

  // Tier 1: Sender name + timestamp tabular-nums
  assert.ok(
    subPanelContent.includes('tabular-nums') &&
      subPanelContent.includes('formatConversationTime'),
    'Tier 1 must format time with tabular-nums at the right side'
  )

  // Tier 2: Clean typography Subject/Title & Status-by-Exception active task indicator
  assert.ok(
    subPanelContent.includes('data-testid="task-count-badge"') &&
      subPanelContent.includes('data-testid="task-badge-active"'),
    'Tier 2 must maintain active task indicator with data-testid="task-count-badge"'
  )
  assert.ok(
    subPanelContent.includes('<StatusDot status="working"'),
    'Tier 2 must render StatusDot working state for active background tasks'
  )

  // Tier 3: 2-line snippet in muted zinc-400
  assert.ok(
    subPanelContent.includes('line-clamp-2') &&
      (subPanelContent.includes('text-zinc-400') || subPanelContent.includes('dark:text-zinc-400')),
    'Tier 3 must display message snippet in 2 lines with line-clamp-2 and muted zinc-400'
  )

  // Conversation item button accessibility
  assert.ok(
    subPanelContent.includes('min-h-[4.5rem]'),
    'ConversationItem button must satisfy min-h-[4.5rem] touch target'
  )
  assert.ok(
    subPanelContent.includes('data-testid="conversation-item"'),
    'ConversationItem must include data-testid="conversation-item"'
  )
})

test('5. Helper: formatConversationTime definition & implementation in InboxSubPanel', () => {
  assert.ok(
    subPanelContent.includes('export function formatConversationTime('),
    'InboxSubPanel must export formatConversationTime helper'
  )
  assert.ok(
    subPanelContent.includes('Yesterday'),
    'formatConversationTime must handle Yesterday formatting'
  )
  assert.ok(
    subPanelContent.includes('toLocaleTimeString'),
    'formatConversationTime must handle today time formatting'
  )
})

test('6. Functional Logic: conversationPreview parses clean summaries and handles JSON', () => {
  assert.ok(
    subPanelContent.includes('export function conversationPreview('),
    'InboxSubPanel must export conversationPreview helper'
  )
  assert.ok(
    subPanelContent.includes('JSON.parse(rawCandidate)'),
    'conversationPreview must parse JSON candidate payloads'
  )
  assert.ok(
    subPanelContent.includes('No messages yet.'),
    'conversationPreview must return fallback string for empty messages'
  )
})
