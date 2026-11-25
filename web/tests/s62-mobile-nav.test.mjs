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
const appPath = path.resolve(webSrcRoot, 'App.tsx')
const settingsViewPath = path.resolve(webSrcRoot, 'features', 'settings', 'SettingsView.tsx')
const s62DocPath = path.resolve(repoRoot, 'docs', 'sprints', 'S62.md')

test('1. Mobile navigation retains Inbox, Projects, New work, Contacts, and Settings', () => {
  const appShellSource = fs.readFileSync(appShellPath, 'utf8')

  // Pillar 1: Inbox (/inbox)
  assert.ok(
    appShellSource.includes('data-testid="bottom-nav-inbox"'),
    'Bottom navigation must render Pillar 1: Inbox with data-testid="bottom-nav-inbox"'
  )
  assert.ok(
    appShellSource.includes("navigate('inbox')"),
    'Pillar 1 Inbox must navigate to inbox tab'
  )
  assert.ok(
    appShellSource.includes('Inbox'),
    'Pillar 1 Inbox must render Inbox icon'
  )

  // Pillar 2: Projects (/projects)
  assert.ok(
    appShellSource.includes('data-testid="bottom-nav-projects"'),
    'Bottom navigation must render Pillar 2: Projects with data-testid="bottom-nav-projects"'
  )
  assert.ok(
    appShellSource.includes("navigate('projects')"),
    'Pillar 2 Projects must navigate to projects tab'
  )
  assert.ok(
    appShellSource.includes('FolderGit2'),
    'Pillar 2 Projects must render FolderGit2 icon'
  )

  // Pillar 3: New work keeps the existing creation route and gains the Plus icon.
  assert.ok(
    appShellSource.includes('data-testid="bottom-nav-studio"'),
    'Bottom navigation must retain the work entry'
  )
  assert.ok(
    appShellSource.includes("navigate('studio')"),
    'New work must open the existing Studio route'
  )
  const start = appShellSource.indexOf('data-testid="bottom-nav-studio"')
  const workEntry = appShellSource.slice(start, appShellSource.indexOf('data-testid="bottom-nav-contacts"', start))
  assert.match(workEntry, /<Plus\b/, 'New work must show the Plus icon')
  assert.ok(
    appShellSource.includes("currentTab === 'studio' ? 'page' : undefined"),
    'New work must expose its active page state'
  )

  // Pillar 4 & 5: Contacts & Settings
  assert.ok(
    appShellSource.includes('data-testid="bottom-nav-contacts"'),
    'Bottom navigation must render Pillar 4: Contacts with data-testid="bottom-nav-contacts"'
  )
  assert.ok(
    appShellSource.includes('data-testid="bottom-nav-settings"'),
    'Bottom navigation must render Pillar 5: Settings with data-testid="bottom-nav-settings"'
  )

  // Ensure touch accessibility on all pillars
  const minHeightMatches = appShellSource.match(/min-h-14/g)
  assert.ok(
    minHeightMatches && minHeightMatches.length >= 5,
    'All bottom navigation buttons must meet touch target min-h-14 (56px >= 44px)'
  )
})

test('2. S62-T1: Elimination of Duplicated Schedules from Desktop Sidebar NAV_GROUPS', () => {
  const appShellSource = fs.readFileSync(appShellPath, 'utf8')

  // Find NAV_GROUPS definition block
  const navGroupsStart = appShellSource.indexOf('export const NAV_GROUPS: NavGroupConfig[] = [')
  const navGroupsEnd = appShellSource.indexOf('export function AppShell', navGroupsStart)
  assert.ok(navGroupsStart !== -1 && navGroupsEnd !== -1, 'NAV_GROUPS must be defined')

  const navGroupsBlock = appShellSource.slice(navGroupsStart, navGroupsEnd)

  // Verify Schedules does not use legacy standalone route to: /schedules
  assert.ok(
    !navGroupsBlock.includes("to: '/schedules'"),
    'NAV_GROUPS must not contain legacy to: /schedules'
  )

  // Verify Manage or Operations contains Contacts
  assert.ok(
    navGroupsBlock.includes("id: 'manage'") || navGroupsBlock.includes("id: 'operations'"),
    'NAV_GROUPS must contain manage or operations group'
  )
  assert.ok(
    navGroupsBlock.includes("to: '/contacts'"),
    'NAV_GROUPS must contain /contacts'
  )
})

test('3. S62-T1: Clean Mobile Drawer Menu without Schedules Duplication', () => {
  const appShellSource = fs.readFileSync(appShellPath, 'utf8')

  // Find Mobile Menu Drawer SlideDrawer section
  const drawerStart = appShellSource.indexOf('data-testid="mobile-menu-drawer"')
  assert.ok(drawerStart !== -1, 'SlideDrawer mobile menu must exist')

  const drawerSection = appShellSource.slice(drawerStart)

  // Verify Schedules link is completely removed from mobile drawer
  assert.ok(
    !drawerSection.includes('data-testid="menu-drawer-link-schedules"'),
    'Mobile drawer must no longer contain menu-drawer-link-schedules'
  )
  assert.ok(
    !drawerSection.includes("navigate('schedules')"),
    'Mobile drawer must no longer navigate directly to standalone schedules'
  )

  // Verify Projects, Contacts, Settings, and Account are accessible in mobile drawer
  assert.ok(
    drawerSection.includes('data-testid="menu-drawer-link-projects"'),
    'Mobile drawer must provide link to Projects'
  )
  assert.ok(
    drawerSection.includes('data-testid="menu-drawer-link-contacts"'),
    'Mobile drawer must provide link to Contacts'
  )
  assert.ok(
    drawerSection.includes('data-testid="menu-drawer-link-settings"'),
    'Mobile drawer must provide link to Settings'
  )
  assert.ok(
    drawerSection.includes('data-testid="menu-drawer-link-account"'),
    'Mobile drawer must provide link to Account'
  )

  // Verify touch target compliance on drawer items (min-h-12 / 48px >= 44px)
  const minH12Matches = drawerSection.match(/min-h-(?:12|14)/g)
  assert.ok(
    minH12Matches && minH12Matches.length >= 4,
    'All drawer navigation buttons must satisfy 48px or larger touch targets'
  )
})

test('4. S62-T1: Route Consolidation & Deep Link Forwarding to SettingsView (/settings/schedules)', () => {
  const appSource = fs.readFileSync(appPath, 'utf8')
  const settingsSource = fs.readFileSync(settingsViewPath, 'utf8')

  // Verify /schedules deep link automatically redirects to /settings/schedules
  assert.ok(
    appSource.includes("if (rootSegment === 'schedules')") &&
      appSource.includes("navigate('/settings/schedules'"),
    'App.tsx must redirect /schedules to /settings/schedules'
  )

  // Verify handleSelectTab navigates to /settings/schedules when schedules tab is selected
  assert.ok(
    appSource.includes("else if (tab === 'schedules')") &&
      appSource.includes("navigate('/settings/schedules')"),
    'App.tsx handleSelectTab must navigate to /settings/schedules'
  )

  // Verify /settings/schedules is rendered within SettingsView master-detail
  assert.ok(
    settingsSource.includes("section === 'schedules'") &&
      settingsSource.includes('<SchedulesView embedded />'),
    'SettingsView must render SchedulesView embedded in the calm master-detail layout'
  )
})

test('5. S62-T1: Sprint Documentation & Quality Gate Artifacts', () => {
  assert.ok(fs.existsSync(s62DocPath), 'docs/sprints/S62.md must exist')

  const s62Doc = fs.readFileSync(s62DocPath, 'utf8')
  assert.ok(s62Doc.includes('Sprint 62'), 'S62 doc must mention Sprint 62')
  assert.ok(s62Doc.includes('S62-T1'), 'S62 doc must document Task 1')
  assert.ok(s62Doc.includes('S62-T2'), 'S62 doc must document Task 2')
  assert.ok(s62Doc.includes('S62-T3'), 'S62 doc must document Task 3')
  assert.ok(s62Doc.includes('S62-T4'), 'S62 doc must document Task 4')
  assert.ok(s62Doc.includes('```d2'), 'S62 doc must use D2 diagram with TALA layout')

  // Verify Before screenshot exists
  const beforeScreenshotPath = path.resolve(repoRoot, 'screenshots', 's62-t1-mobile-nav-before.png')
  assert.ok(
    fs.existsSync(beforeScreenshotPath),
    'screenshots/s62-t1-mobile-nav-before.png must exist'
  )
})
