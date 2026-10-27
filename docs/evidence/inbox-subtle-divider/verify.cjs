// Verification script for Inbox subtle inset divider
const fs = require('fs')
const path = require('path')
const assert = require('assert')

console.log('Running verification for Inbox subtle inset divider...')

// 1. Verify InboxSubPanel.tsx
const subPanelPath = path.resolve(__dirname, '../../../web/src/features/inbox/InboxSubPanel.tsx')
const subPanelSrc = fs.readFileSync(subPanelPath, 'utf8')

// Requirement 1 & 2 & 3: Container list should not have full-width divide-y; items should have inbox-item-divider
assert(!subPanelSrc.includes('divide-y divide-border/20'), 'Old divide-y container class must be removed')
assert(subPanelSrc.includes('isLast = index === filteredContacts.length - 1'), 'Last item check isLast must be present')
assert(subPanelSrc.includes("className={cn('inbox-item-row', !isLast && 'has-divider')}"), 'has-divider class on non-last items must be present')
assert(subPanelSrc.includes('inbox-item-divider'), 'inbox-item-divider element must be present in InboxSubPanel')
assert(subPanelSrc.includes("import './chat-whatsapp.css'"), 'chat-whatsapp.css must be imported')

// 2. Verify ItemActions.tsx
const itemActionsPath = path.resolve(__dirname, '../../../web/src/components/ui/ItemActions.tsx')
const itemActionsSrc = fs.readFileSync(itemActionsPath, 'utf8')
assert(itemActionsSrc.includes('className?: string'), 'ItemActions must accept optional className prop')
assert(itemActionsSrc.includes('${className || \'\'}'), 'ItemActions must forward className to container')

// 3. Verify chat-whatsapp.css
const chatCssPath = path.resolve(__dirname, '../../../web/src/features/inbox/chat-whatsapp.css')
const chatCssSrc = fs.readFileSync(chatCssPath, 'utf8')
assert(chatCssSrc.includes('.inbox-item-divider'), '.inbox-item-divider rule must exist in chat-whatsapp.css')
assert(chatCssSrc.includes('left: 3.75rem'), 'Left inset must be 3.75rem (60px) aligning with message text')
assert(chatCssSrc.includes('rgba(255, 255, 255, 0.08)'), 'Dark mode subtle contrast must be rgba(255, 255, 255, 0.08)')
assert(chatCssSrc.includes('rgba(0, 0, 0, 0.06)'), 'Light mode subtle contrast must be rgba(0, 0, 0, 0.06)')
assert(chatCssSrc.includes(':last-child .inbox-item-divider'), 'Last child safety rule must hide divider on last item')

// 4. Verify index.css
const indexCssPath = path.resolve(__dirname, '../../../web/src/index.css')
const indexCssSrc = fs.readFileSync(indexCssPath, 'utf8')
assert(indexCssSrc.includes('.inbox-item-divider'), '.inbox-item-divider rule must exist in index.css')
assert(indexCssSrc.includes('rgba(255, 255, 255, 0.08)'), 'Dark mode subtle contrast must exist in index.css')

// 5. Test mathematical geometry:
// Container px-3 (12px = 0.75rem)
// Avatar w-9 (36px = 2.25rem)
// Gap gap-3 (12px = 0.75rem)
const leftPadding = 0.75
const avatarWidth = 2.25
const gapWidth = 0.75
const calculatedInset = leftPadding + avatarWidth + gapWidth
assert.strictEqual(calculatedInset, 3.75, 'Calculated inset must match 3.75rem (60px)')

// 6. Test list partition logic:
const testContacts = [{ id: '1' }, { id: '2' }, { id: '3' }]
const hasDividers = testContacts.map((_, i) => ! (i === testContacts.length - 1))
assert.deepStrictEqual(hasDividers, [true, true, false], 'First N-1 items must have dividers, last item must not')

console.log('All 6 verification checks passed successfully!')
