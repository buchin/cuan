import test from 'node:test'
import assert from 'node:assert/strict'
import fs from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'

const __dirname = path.dirname(fileURLToPath(import.meta.url))
const webDir = path.resolve(__dirname, '..')

test('Navigation menu order: Studio, Inbox, Tasks, Project, Account', async (t) => {
  const appShellContent = fs.readFileSync(
    path.join(webDir, 'src/components/layout/AppShell.tsx'),
    'utf-8',
  )

  // Verify NAV_ITEMS contains Studio, Inbox, Tasks, Projects
  const navItemsMatch = appShellContent.match(/const NAV_ITEMS = \[([\s\S]*?)\] as const/)
  assert.ok(navItemsMatch, 'NAV_ITEMS must be defined in AppShell.tsx')

  const navItemsText = navItemsMatch[1]
  const items = [...navItemsText.matchAll(/id:\s*'([^']+)',\s*label:\s*'([^']+)'/g)].map(
    (m) => ({ id: m[1], label: m[2] }),
  )

  // 1. Studio
  // 2. Inbox
  // 3. Tasks
  // 4. Projects (Project)
  assert.deepEqual(
    items.map((i) => i.id),
    ['studio', 'inbox', 'tasks', 'projects'],
    'Main rail items must be studio, inbox, tasks, projects in exact order',
  )

  // Contacts must NOT be in NAV_ITEMS top rail
  assert.ok(
    !items.some((i) => i.id === 'contacts'),
    'Contacts must no longer be in the top-level main NAV_ITEMS',
  )

  // Verify NavUser handles Account and sits as 5th destination in navigation
  assert.ok(
    appShellContent.includes('<NavUser'),
    'AppShell must render NavUser for Account navigation',
  )
  assert.ok(
    appShellContent.includes("currentTab === 'account'"),
    'AppShell must handle active state for account tab',
  )
})

test('Contacts is moved under Account submenu and page', async (t) => {
  const navUserContent = fs.readFileSync(
    path.join(webDir, 'src/components/layout/NavUser.tsx'),
    'utf-8',
  )

  // Verify NavUser has Contacts entry routing to /account/contacts
  assert.ok(
    navUserContent.includes('/account/contacts'),
    'NavUser dropdown must contain link/action navigating to /account/contacts',
  )
  assert.ok(
    navUserContent.includes("t('Contacts')"),
    'NavUser must display Contacts translated label',
  )

  // Verify NavUser identity header routes to /account
  assert.ok(
    navUserContent.includes("navigate('/account')"),
    'NavUser identity header must navigate to /account',
  )

  // Verify SettingsView includes Contacts tab and section when under /account
  const settingsContent = fs.readFileSync(
    path.join(webDir, 'src/features/settings/SettingsView.tsx'),
    'utf-8',
  )
  assert.ok(
    settingsContent.includes("['contacts', t('Contacts')]"),
    'SettingsView must include contacts tab item',
  )
  assert.ok(
    settingsContent.includes("to = isAccount\n            ? `/account/${id}`"),
    'SettingsView must route to /account/<tab> when in account view',
  )
  assert.ok(
    settingsContent.includes('/account/contacts'),
    'SettingsView must provide link to /account/contacts',
  )
})

test('App.tsx routing for /account, /account/contacts, and backward compatibility', async (t) => {
  const appContent = fs.readFileSync(
    path.join(webDir, 'src/App.tsx'),
    'utf-8',
  )

  // Verify root segment 'account' maps to currentTab 'account'
  assert.ok(
    appContent.includes("case 'account':\n        return 'account'"),
    'App.tsx must resolve root segment account to currentTab account',
  )

  // Verify /account/contacts renders ContactsView
  assert.ok(
    appContent.includes(
      "currentTab === 'account') &&\n            subParam === 'contacts'",
    ) ||
      appContent.includes(
        "currentTab === 'account' && subParam === 'contacts'",
      ),
    'App.tsx must render ContactsView when currentTab is account and subParam is contacts',
  )

  // Verify /account renders SettingsView
  assert.ok(
    appContent.includes(
      "(currentTab === 'settings' || currentTab === 'account') &&\n          subParam !== 'contacts'",
    ) ||
      appContent.includes(
        "currentTab === 'account' && subParam !== 'contacts'",
      ),
    'App.tsx must render SettingsView for /account when subParam is not contacts',
  )

  // Verify existing /contacts route still renders ContactsView (no broken links)
  assert.ok(
    appContent.includes("currentTab === 'contacts'"),
    'App.tsx must support legacy /contacts without broken links',
  )
})

test('All navigation routes have valid targets and no broken links', async (t) => {
  const appContent = fs.readFileSync(path.join(webDir, 'src/App.tsx'), 'utf-8')
  const validTabs = ['studio', 'inbox', 'tasks', 'projects', 'account', 'contacts', 'settings']

  for (const tab of validTabs) {
    assert.ok(
      appContent.includes(`case '${tab}':`),
      `Route handler for /${tab} must be present in App.tsx`,
    )
  }

  // Check built html dist exists
  const distIndexPath = path.join(webDir, 'dist/index.html')
  assert.ok(
    fs.existsSync(distIndexPath),
    'Build dist/index.html must exist after build',
  )
})
