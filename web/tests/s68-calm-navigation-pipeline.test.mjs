import test from 'node:test'
import assert from 'node:assert/strict'
import fs from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'

const __filename = fileURLToPath(import.meta.url)
const __dirname = path.dirname(__filename)
const rootDir = path.resolve(__dirname, '../..')
const webDir = path.resolve(rootDir, 'web')
const appShellPath = path.resolve(webDir, 'src/components/layout/AppShell.tsx')
const navUserPath = path.resolve(webDir, 'src/components/layout/NavUser.tsx')
const deployScriptPath = path.resolve(rootDir, 'scripts/deploy.sh')
const packageJsonPath = path.resolve(webDir, 'package.json')

test('1. S68-T1: High-Speed & Fail-Fast Pipeline Gate Architecture', () => {
  const pkg = JSON.parse(fs.readFileSync(packageJsonPath, 'utf8'))
  const deployScript = fs.readFileSync(deployScriptPath, 'utf8')

  // Frontend test gate includes tsc --noEmit
  assert.ok(
    pkg.scripts.test.includes('tsc --noEmit'),
    'web/package.json test script must run tsc --noEmit for fail-fast typechecking'
  )

  // Deploy script executes parallel Go binary compilations
  assert.ok(
    deployScript.includes('pid_server=$!') &&
    deployScript.includes('pid_migrate=$!') &&
    deployScript.includes('pid_runner=$!') &&
    deployScript.includes('wait "$pid_server" "$pid_migrate" "$pid_runner"'),
    'scripts/deploy.sh must compile Go server, migrate, and runner binaries in parallel'
  )

  // Deploy script uses rsync for atomic payload deployment
  assert.ok(
    deployScript.includes('rsync -a --delete'),
    'scripts/deploy.sh must perform atomic rsync to release directory'
  )
})

test('2. S68-T2: MoreVertical Top Header Menu & Calm Design Standard', () => {
  const appShellSource = fs.readFileSync(appShellPath, 'utf8')

  // MoreVertical trigger and container
  assert.ok(
    appShellSource.includes('data-testid="header-more-menu-trigger"'),
    'Header must render MoreVertical trigger'
  )
  assert.ok(
    appShellSource.includes('data-testid="header-more-menu"'),
    'Header must render more options menu container'
  )

  // Settings action is top priority
  assert.ok(
    appShellSource.includes('data-testid="more-menu-settings"'),
    'MoreVertical menu must include Settings action'
  )
  assert.ok(
    appShellSource.includes("routerNavigate('/settings')"),
    'Settings action must route to /settings'
  )

  // Quick Language Toggle
  assert.ok(
    appShellSource.includes('data-testid="more-menu-language-toggle"'),
    'MoreVertical menu must provide inline language toggle'
  )

  // Calm styling: rounded-xl, border-border/40, shadow-lg
  assert.ok(
    appShellSource.includes('rounded-xl') && appShellSource.includes('border-border/40'),
    'MoreVertical menu must use Bear Notes calm styling: rounded-xl and border-border/40'
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
    navUserSource.includes("navigate('/settings/profile')"),
    'NavUser must provide Profile & password link'
  )

  // Language toggle
  assert.ok(
    navUserSource.includes('data-testid="navuser-language-toggle"'),
    'NavUser must provide language toggle'
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

  // Verify navigation ordering: Chat -> Tasks -> Projects -> Contacts -> Account (NavUser)
  const chatIdx = appShellSource.indexOf('data-testid="bottom-nav-chat"')
  const tasksIdx = appShellSource.indexOf('data-testid="bottom-nav-tasks"')
  const projectsIdx = appShellSource.indexOf('data-testid="bottom-nav-projects"')
  const contactsIdx = appShellSource.indexOf('data-testid="bottom-nav-contacts"')
  const userIdx = appShellSource.indexOf('data-testid="bottom-nav-user"')

  assert.ok(chatIdx !== -1 && tasksIdx !== -1 && projectsIdx !== -1 && contactsIdx !== -1 && userIdx !== -1, 'All bottom navigation items must exist')
  assert.ok(chatIdx < tasksIdx, 'Chat must be before Tasks')
  assert.ok(tasksIdx < projectsIdx, 'Tasks must be before Projects')
  assert.ok(projectsIdx < contactsIdx, 'Projects must be before Contacts')
  assert.ok(contactsIdx < userIdx, 'Contacts must be before NavUser Account')
})
