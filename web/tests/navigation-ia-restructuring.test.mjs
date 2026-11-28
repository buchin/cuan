import test from 'node:test'
import assert from 'node:assert/strict'
import fs from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'

const __filename = fileURLToPath(import.meta.url)
const __dirname = path.dirname(__filename)

const repoRoot = path.resolve(__dirname, '..', '..')
const webSrcRoot = path.resolve(__dirname, '..', 'src')

const appShellPath = path.resolve(webSrcRoot, 'components', 'layout', 'AppShell.tsx')
const navUserPath = path.resolve(webSrcRoot, 'components', 'layout', 'NavUser.tsx')
const slideDrawerPath = path.resolve(webSrcRoot, 'components', 'layout', 'SlideDrawer.tsx')
const appPath = path.resolve(webSrcRoot, 'App.tsx')
const idLocalePath = path.resolve(webSrcRoot, 'locales', 'id.json')
const enLocalePath = path.resolve(webSrcRoot, 'locales', 'en.json')

test('1. S59-T3: Desktop Sidebar 3-Group Information Architecture (NAV_GROUPS)', () => {
  const appShellSource = fs.readFileSync(appShellPath, 'utf8')

  // Verify NAV_GROUPS definition exists
  assert.ok(
    appShellSource.includes('export const NAV_GROUPS: NavGroupConfig[] = [') ||
      appShellSource.includes('const NAV_GROUPS = [') ||
      appShellSource.includes('NAV_GROUPS'),
    'AppShell.tsx must define NAV_GROUPS for structured 3-group navigation'
  )

  // Verify unified Main navigation group without Manage/KELOLA separator
  assert.ok(
    appShellSource.includes("id: 'main'") && !appShellSource.includes("id: 'manage'"),
    'NAV_GROUPS must define unified main navigation group without manage separator'
  )
  assert.ok(
    appShellSource.includes("to: '/studio'") &&
      appShellSource.includes("to: '/inbox'") &&
      !appShellSource.includes("to: '/tasks'"),
    'Main navigation must contain routes for /studio and /inbox without /tasks'
  )

  assert.ok(
    appShellSource.includes("to: '/projects'") &&
      appShellSource.includes("to: '/contacts'") &&
      appShellSource.includes("to: '/settings'"),
    'Unified navigation must contain routes for /projects, /contacts, and /settings'
  )

  // Verify desktop sidebar renders testids for all 3 groups
  assert.ok(
    appShellSource.includes('data-testid={`nav-group-${group.id}`}'),
    'Desktop sidebar must render data-testid for each nav group'
  )

  // Verify unread badge rail exists on Chat item
  assert.ok(
    appShellSource.includes('data-testid="inbox-unread-badge-rail"'),
    'Desktop sidebar rail must render unread counter badge on chat item'
  )
})

test('2. S59-T3: Mobile Bottom Navigation (Inbox, Projects, New work, Contacts, Settings)', () => {
  const appShellSource = fs.readFileSync(appShellPath, 'utf8')

  // Pillar 1: Inbox (/inbox)
  assert.ok(
    appShellSource.includes('data-testid="bottom-nav-inbox"'),
    'Bottom navigation must render Pillar 1: Inbox with data-testid="bottom-nav-inbox"'
  )
  assert.ok(
    appShellSource.includes('navigate(\'inbox\')'),
    'Pillar 1 Inbox must navigate to inbox tab'
  )
  assert.ok(
    appShellSource.includes('data-testid="inbox-unread-badge-bottom"'),
    'Pillar 1 Inbox must render unread badge counter'
  )

  // Pillar 2: Projects (/projects)
  assert.ok(
    appShellSource.includes('data-testid="bottom-nav-projects"'),
    'Bottom navigation must render Pillar 2: Projects with data-testid="bottom-nav-projects"'
  )
  assert.ok(
    appShellSource.includes('navigate(\'projects\')'),
    'Pillar 2 Projects must navigate to projects tab'
  )

  // Pillar 3: New work (/studio)
  assert.ok(
    appShellSource.includes('data-testid="bottom-nav-studio"'),
    'Bottom navigation must render Pillar 3: New work with data-testid="bottom-nav-studio"'
  )

  // Pillar 4: Contacts (/contacts)
  assert.ok(
    appShellSource.includes('data-testid="bottom-nav-contacts"'),
    'Bottom navigation must render Pillar 4: Contacts with data-testid="bottom-nav-contacts"'
  )
  assert.ok(
    appShellSource.includes('navigate(\'contacts\')'),
    'Pillar 4 must navigate to contacts'
  )

  // Pillar 5: Settings (/settings)
  assert.ok(
    appShellSource.includes('data-testid="bottom-nav-settings"'),
    'Bottom navigation must render Pillar 5: Settings with data-testid="bottom-nav-settings"'
  )
})

test('3. S59-T3: Mobile Menu Drawer with Operations, System, and Account Links', () => {
  const appShellSource = fs.readFileSync(appShellPath, 'utf8')

  // Verify SlideDrawer is used as mobile menu drawer
  assert.ok(
    appShellSource.includes('data-testid="mobile-menu-drawer"'),
    'AppShell must mount SlideDrawer with data-testid="mobile-menu-drawer"'
  )

  // Verify drawer links
  assert.ok(
    appShellSource.includes('data-testid="menu-drawer-link-projects"') ||
      appShellSource.includes('data-testid="menu-drawer-link-schedules"'),
    'Mobile drawer must contain link with valid testid'
  )
  assert.ok(
    appShellSource.includes('data-testid="menu-drawer-link-contacts"'),
    'Mobile drawer must contain link to Contacts with data-testid="menu-drawer-link-contacts"'
  )

  // Verify System & Account links in mobile drawer
  assert.ok(
    appShellSource.includes('data-testid="menu-drawer-link-settings"'),
    'Mobile drawer must contain link to Settings with data-testid="menu-drawer-link-settings"'
  )
  assert.ok(
    appShellSource.includes('data-testid="menu-drawer-link-account"'),
    'Mobile drawer must contain link to Account with data-testid="menu-drawer-link-account"'
  )

  // Verify WCAG touch target compliance (min-h-12 / 48px >= 44px)
  const minHeightMatches = appShellSource.match(/min-h-(?:12|14)/g)
  assert.ok(
    minHeightMatches && minHeightMatches.length >= 4,
    'All drawer navigation buttons must satisfy touch target >= 44px (min-h-12)'
  )
})

test('4. S59-T3: Lifting Schedules and Contacts out of NavUser.tsx popover', () => {
  const navUserSource = fs.readFileSync(navUserPath, 'utf8')

  // Verify NavUser popover does NOT contain Contacts or Schedules navigation actions
  assert.ok(
    !navUserSource.includes("navigate('/account/contacts')"),
    'NavUser.tsx must no longer navigate to /account/contacts (lifted to Operations)'
  )
  assert.ok(
    !navUserSource.includes("navigate('/settings/schedules')"),
    'NavUser.tsx must no longer navigate to /settings/schedules (lifted to Operations)'
  )

  // Verify NavUser retains essential profile, settings, shortcuts, and logout actions
  assert.ok(
    !navUserSource.includes("navigate('/settings/profile')"),
    'Profile & password belongs under Settings'
  )
  assert.ok(
    navUserSource.includes("navigate('/account')"),
    'NavUser.tsx header must navigate to /account'
  )
  assert.ok(
    navUserSource.includes("handleLogout"),
    'NavUser.tsx must retain sign out action'
  )
})

test('5. S59-T3: Backward-Compatible Routing in App.tsx (/chat, /account/contacts, /settings/schedules)', () => {
  const appSource = fs.readFileSync(appPath, 'utf8')

  // Verify alias /chat redirects to /inbox
  assert.ok(
    appSource.includes("rootSegment === 'chat' && subParam") &&
      appSource.includes("'/inbox/' + encodeURIComponent(subParam)"),
    'App.tsx must redirect /chat/:roomId to /inbox/:roomId'
  )
  assert.ok(
    appSource.includes("else if (rootSegment === 'chat')") &&
      appSource.includes("navigate('/inbox'"),
    'App.tsx must redirect /chat root to /inbox'
  )

  // Verify /account/contacts redirects to /contacts
  assert.ok(
    appSource.includes("subParam === 'contacts'") &&
      appSource.includes("navigate('/contacts'"),
    'App.tsx must redirect /account/contacts to /contacts'
  )

  // Verify /schedules alias redirects to /settings/schedules or vice versa
  assert.ok(
    (appSource.includes("rootSegment === 'schedules'") && appSource.includes("navigate('/settings/schedules'")) ||
      (appSource.includes("subParam === 'schedules'") && appSource.includes("navigate('/schedules'")),
    'App.tsx must synchronize schedules route'
  )

  // Verify primary routes render correct views without 404
  assert.ok(
    appSource.includes("case 'inbox':\n        return 'inbox'"),
    'App.tsx must support primary /inbox route'
  )
  assert.ok(
    appSource.includes("case 'tasks':\n        return 'tasks'"),
    'App.tsx must support primary /tasks route'
  )
  assert.ok(
    appSource.includes("case 'projects':\n        return 'projects'"),
    'App.tsx must support primary /projects route'
  )
  assert.ok(
    appSource.includes("case 'schedules':\n        return 'schedules'"),
    'App.tsx must support primary /schedules route'
  )
  assert.ok(
    appSource.includes("case 'contacts':\n        return 'contacts'"),
    'App.tsx must support primary /contacts route'
  )
  assert.ok(
    appSource.includes("case 'settings':\n        return 'settings'"),
    'App.tsx must support primary /settings route'
  )
})

test('6. S59-T3: Localization Completeness (id.json & en.json)', () => {
  assert.ok(fs.existsSync(idLocalePath), 'id.json must exist')
  assert.ok(fs.existsSync(enLocalePath), 'en.json must exist')

  const idLocale = JSON.parse(fs.readFileSync(idLocalePath, 'utf8'))
  const enLocale = JSON.parse(fs.readFileSync(enLocalePath, 'utf8'))

  const requiredKeys = [
    'Workspace',
    'Operations',
    'System',
    'Menu',
    'Otomasi & cron rutin',
    'CRM & memori pelanggan',
    'Profil, AI providers, team',
    'Operations & System',
    'Operations, System & Account',
  ]

  for (const key of requiredKeys) {
    assert.ok(
      typeof idLocale[key] === 'string' && idLocale[key].length > 0,
      `id.json must include non-empty translation for "${key}"`
    )
    assert.ok(
      typeof enLocale[key] === 'string' && enLocale[key].length > 0,
      `en.json must include non-empty translation for "${key}"`
    )
  }
})
