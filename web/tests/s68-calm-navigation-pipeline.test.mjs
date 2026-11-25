import test from 'node:test'
import assert from 'node:assert/strict'
import fs from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'
import { spawnSync } from 'node:child_process'

const __filename = fileURLToPath(import.meta.url)
const __dirname = path.dirname(__filename)
const rootDir = path.resolve(__dirname, '../..')
const webDir = path.resolve(rootDir, 'web')
const appShellPath = path.resolve(webDir, 'src/components/layout/AppShell.tsx')
const navUserPath = path.resolve(webDir, 'src/components/layout/NavUser.tsx')
const deployScriptPath = path.resolve(rootDir, 'scripts/deploy.sh')
const packageJsonPath = path.resolve(webDir, 'package.json')

test('1. S68-T1: Type-safe frontend and verified parallel build gates', () => {
  const pkg = JSON.parse(fs.readFileSync(packageJsonPath, 'utf8'))
  const deployScript = fs.readFileSync(deployScriptPath, 'utf8')

  // Frontend test gate includes tsc --noEmit
  assert.ok(
    pkg.scripts.test.includes('tsc --noEmit'),
    'web/package.json test script must run tsc --noEmit for fail-fast typechecking'
  )

  // Exercise the actual build block with process fixtures. Matching the old
  // collective wait text incorrectly required the release-gate bug to remain.
  const buildGate = spawnSync(
    'python3', ['-B', path.join(rootDir, 'scripts/test-release-build-gate.py')],
    { cwd: rootDir, encoding: 'utf8', timeout: 60_000 }
  )
  assert.ifError(buildGate.error)
  assert.equal(buildGate.status, 0,
    `Parallel build failures, successful builds and child reaping must pass:\n${buildGate.stdout}\n${buildGate.stderr}`)

  // Deploy script uses rsync for atomic payload deployment
  assert.ok(
    deployScript.includes('rsync -a --delete'),
    'scripts/deploy.sh must perform atomic rsync to release directory'
  )
})

test('2. S68-T2: Eradication of Redundant MoreVertical Menu & Zero Visual Noise', () => {
  const appShellSource = fs.readFileSync(appShellPath, 'utf8')

  // In S68, redundant MoreVertical popover is completely eliminated from header
  assert.ok(
    !appShellSource.includes('data-testid="header-more-menu-trigger"'),
    'Header must not contain duplicate MoreVertical trigger'
  )
  assert.ok(
    !appShellSource.includes('data-testid="header-more-menu"'),
    'Header must not contain duplicate header-more-menu dropdown'
  )
  assert.ok(
    !appShellSource.includes('moreMenuOpen'),
    'Header must eliminate unnecessary moreMenuOpen state and event listeners'
  )
})

test('3. S68-T3: NavUser Profile Popover & Preferences Alignment', () => {
  const navUserSource = fs.readFileSync(navUserPath, 'utf8')

  // Identity header to /account
  assert.ok(
    navUserSource.includes("navigate('/account')"),
    'NavUser identity header must navigate to /account'
  )

  // Profile & password to /settings/profile
  assert.ok(
    !navUserSource.includes("navigate('/settings/profile')"),
    'Profile settings belong under Settings'
  )

  // Language toggle
  assert.ok(
    !navUserSource.includes('data-testid="navuser-language-toggle"'),
    'Language belongs in Settings'
  )

  // Hairline dividers
  assert.ok(
    navUserSource.includes('bg-border/40'),
    'NavUser popover must use subtle hairline dividers (bg-border/40)'
  )
})

test('4. S68-T4: Mobile Navigation Ergonomics & Preservation of Projects', () => {
  const appShellSource = fs.readFileSync(appShellPath, 'utf8')

  // Projects must be preserved in mobile bottom nav
  assert.ok(
    appShellSource.includes('data-testid="bottom-nav-projects"'),
    'Bottom navigation must retain Pillar 3: Projects (data-testid="bottom-nav-projects")'
  )
  assert.ok(
    appShellSource.includes("navigate('projects')"),
    'Projects button must navigate to projects'
  )

  // Verify symmetrical navigation ordering: Inbox -> Projects -> Studio (+) -> Contacts -> Settings
  const inboxIdx = appShellSource.indexOf('data-testid="bottom-nav-inbox"')
  const projectsIdx = appShellSource.indexOf('data-testid="bottom-nav-projects"')
  const studioIdx = appShellSource.indexOf('data-testid="bottom-nav-studio"')
  const contactsIdx = appShellSource.indexOf('data-testid="bottom-nav-contacts"')
  const settingsIdx = appShellSource.indexOf('data-testid="bottom-nav-settings"')

  assert.ok(inboxIdx !== -1 && projectsIdx !== -1 && studioIdx !== -1 && contactsIdx !== -1 && settingsIdx !== -1, 'All bottom navigation items must exist')
  assert.ok(inboxIdx < projectsIdx, 'Inbox must be before Projects')
  assert.ok(projectsIdx < studioIdx, 'Projects must be before Studio (New work +)')
  assert.ok(studioIdx < contactsIdx, 'Studio (New work +) must be before Contacts')
  assert.ok(contactsIdx < settingsIdx, 'Contacts must be before Settings')

  // Verify subtle circle accent container on New work (+)
  const indexCss = fs.readFileSync(path.resolve(webDir, 'src/index.css'), 'utf8')
  assert.ok(
    appShellSource.includes('data-action-trigger="true"'),
    'New work (+) button must have data-action-trigger attribute for circular container styling'
  )
  assert.ok(
    appShellSource.includes('action-trigger-circle'),
    'New work (+) button must have action-trigger-circle class'
  )
  assert.ok(
    indexCss.includes('[data-action-trigger]'),
    'index.css must exempt [data-action-trigger] from flat border-radius reset'
  )
})
