import test from 'node:test'
import assert from 'node:assert'
import fs from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'

const sharedIconSource = fs.readFileSync(new URL('../src/components/ui/IconButton.tsx', import.meta.url), 'utf8')
const __filename = fileURLToPath(import.meta.url)
const __dirname = path.dirname(__filename)

const subPanelPath = path.join(__dirname, '../src/features/inbox/InboxSubPanel.tsx')
const archivedHookPath = path.join(__dirname, '../src/hooks/useArchivedItems.tsx')
const subPanelSource = fs.readFileSync(subPanelPath, 'utf8')
const archivedHookSource = fs.readFileSync(archivedHookPath, 'utf8')

const distDir = path.join(__dirname, '../dist/assets')
const distJsFile = fs.readdirSync(distDir).find((f) => f.startsWith('index-') && f.endsWith('.js'))
const distJsContent = distJsFile ? fs.readFileSync(path.join(distDir, distJsFile), 'utf8') : ''

test('1. InboxSubPanel: Mark All Read button with CheckCheck icon and accessible label', () => {
  assert.ok(
    subPanelSource.includes('CheckCheck'),
    'InboxSubPanel must import and render the CheckCheck icon for mark all read'
  )
  assert.ok(
    subPanelSource.includes("t('Mark all as read')"),
    'InboxSubPanel must translate Mark all as read label for internationalization'
  )
  assert.ok(
    subPanelSource.includes('totalUnreadCount > 0'),
    'Mark All Read button must only be active/rendered when there are unread items'
  )
  assert.ok(
    subPanelSource.includes('<IconButton') && sharedIconSource.includes('min-h-11 min-w-11'),
    'Mark All Read button must satisfy WCAG 44x44px minimum touch target size (min-h-11 min-w-11)'
  )
})

test('2. useArchivedItems: batch mark-all API integration and event dispatch', () => {
  assert.ok(
    archivedHookSource.includes('markAllItemPreferencesRead'),
    'useArchivedItems must export markAllItemPreferencesRead'
  )
  assert.ok(
    archivedHookSource.includes('all: true, read: true') || archivedHookSource.includes('all: true'),
    'markAllItemPreferencesRead must send batch mark read payload to /api/v1/item-preferences'
  )
  assert.ok(
    archivedHookSource.includes('cuan-item-preferences'),
    'markAllItemPreferencesRead must dispatch cuan-item-preferences event for instant navbar state sync'
  )
  assert.ok(
    archivedHookSource.includes('markAllRead:'),
    'useArchivedItems must expose markAllRead method'
  )
})

test('3. InboxSubPanel: Action menu fallback in ItemActions', () => {
  assert.ok(
    subPanelSource.includes("label: t('Mark all as read')"),
    'ItemActions dropdown must include Mark all as read when totalUnreadCount > 0'
  )
})

test('4. Production bundle contains compiled mark-all logic', () => {
  assert.ok(distJsContent.length > 0, 'Production bundle must exist')
  assert.ok(
    distJsContent.includes('markAllItemPreferencesRead') || distJsContent.includes('cuan-item-preferences'),
    'Production bundle must retain preferences event sync'
  )
})
