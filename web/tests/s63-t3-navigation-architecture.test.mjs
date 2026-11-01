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
const appPath = path.resolve(webSrcRoot, 'App.tsx')
const taskBoardViewPath = path.resolve(webSrcRoot, 'features', 'tasks', 'TaskBoardView.tsx')
const taskSessionPagePath = path.resolve(webSrcRoot, 'features', 'tasks', 'TaskSessionPage.tsx')
const taskUiPath = path.resolve(webSrcRoot, 'features', 'tasks', 'task-ui.ts')
const workspacePathPath = path.resolve(webSrcRoot, 'features', 'tasks', 'workspacePath.ts')

test('1. S63-T3: Top Header MoreVertical [⋮] Menu & Create Button Architecture', () => {
  const appShellSource = fs.readFileSync(appShellPath, 'utf8')

  // Header [+] Create Action button
  assert.ok(
    appShellSource.includes('id="header-new-action"'),
    'Header must render CreateAction button with id="header-new-action"'
  )
  assert.ok(
    appShellSource.includes('<CreateAction'),
    'Header must mount CreateAction component'
  )

  // MoreVertical [⋮] Trigger Button
  assert.ok(
    appShellSource.includes('data-testid="header-more-menu-trigger"'),
    'Header must provide MoreVertical trigger with data-testid="header-more-menu-trigger"'
  )
  assert.ok(
    appShellSource.includes('MoreVertical'),
    'Header must import and render MoreVertical icon'
  )

  // Dropdown Menu Popover
  assert.ok(
    appShellSource.includes('data-testid="header-more-menu"'),
    'Header must provide dropdown container with data-testid="header-more-menu"'
  )

  // Menu items: Agents, Schedules, Settings
  assert.ok(
    appShellSource.includes('data-testid="more-menu-agents"'),
    'MoreVertical menu must include Agents action with data-testid="more-menu-agents"'
  )
  assert.ok(
    appShellSource.includes("routerNavigate('/agents')"),
    'Agents menu item must route to /agents'
  )

  assert.ok(
    appShellSource.includes('data-testid="more-menu-schedules"'),
    'MoreVertical menu must include Schedules action with data-testid="more-menu-schedules"'
  )
  assert.ok(
    appShellSource.includes("routerNavigate('/schedules')"),
    'Schedules menu item must route to /schedules'
  )

  assert.ok(
    appShellSource.includes('data-testid="more-menu-settings"'),
    'MoreVertical menu must include Settings action with data-testid="more-menu-settings"'
  )
  assert.ok(
    appShellSource.includes("routerNavigate('/settings')"),
    'Settings menu item must route to /settings'
  )
})

test('2. S63-T3: 5-Pillar Mobile Bottom Navigation Architecture', () => {
  const appShellSource = fs.readFileSync(appShellPath, 'utf8')

  // Pillar 1: Chat (/chat, /inbox)
  assert.ok(
    appShellSource.includes('data-testid="bottom-nav-chat"'),
    'Bottom navigation must render Pillar 1: Chat with data-testid="bottom-nav-chat"'
  )
  assert.ok(
    appShellSource.includes("navigate('inbox')"),
    'Pillar 1 must navigate to inbox/chat'
  )

  // Pillar 2: Tasks (/tasks)
  assert.ok(
    appShellSource.includes('data-testid="bottom-nav-tasks"'),
    'Bottom navigation must render Pillar 2: Tasks with data-testid="bottom-nav-tasks"'
  )
  assert.ok(
    appShellSource.includes("navigate('tasks')"),
    'Pillar 2 must navigate to tasks'
  )
  assert.ok(
    appShellSource.includes("currentTab === 'tasks'"),
    'Pillar 2 must indicate active state when on tasks route'
  )

  // Pillar 3: Projects (/projects) — restored before Contacts
  assert.ok(
    appShellSource.includes('data-testid="bottom-nav-projects"'),
    'Bottom navigation must render Pillar 3: Projects with data-testid="bottom-nav-projects"'
  )
  assert.ok(
    appShellSource.includes("navigate('projects')"),
    'Pillar 3 must navigate to projects'
  )

  // Pillar 4: Contacts (/contacts)
  assert.ok(
    appShellSource.includes('data-testid="bottom-nav-contacts"'),
    'Bottom navigation must render Pillar 4: Contacts with data-testid="bottom-nav-contacts"'
  )
  assert.ok(
    appShellSource.includes("navigate('contacts')"),
    'Pillar 4 must navigate to contacts'
  )

  // Pillar 5: User Avatar (NavUser compact in bottom nav)
  assert.ok(
    appShellSource.includes('data-testid="bottom-nav-user"'),
    'Bottom navigation must render Pillar 5: User Profile with data-testid="bottom-nav-user"'
  )
  assert.ok(
    appShellSource.includes('<NavUser\n                bottomNavigation'),
    'Pillar 5 must mount NavUser with bottomNavigation={true}'
  )

  // Verify ordering: Chat -> Tasks -> Projects -> Contacts -> NavUser
  const chatIdx = appShellSource.indexOf('data-testid="bottom-nav-chat"')
  const tasksIdx = appShellSource.indexOf('data-testid="bottom-nav-tasks"')
  const projectsIdx = appShellSource.indexOf('data-testid="bottom-nav-projects"')
  const contactsIdx = appShellSource.indexOf('data-testid="bottom-nav-contacts"')
  const userIdx = appShellSource.indexOf('data-testid="bottom-nav-user"')

  assert.ok(chatIdx < tasksIdx, 'Chat must be before Tasks')
  assert.ok(tasksIdx < projectsIdx, 'Tasks must be before Projects')
  assert.ok(projectsIdx < contactsIdx, 'Projects must be before Contacts')
  assert.ok(contactsIdx < userIdx, 'Contacts must be before NavUser')
})

test('3. S63-T3: Compact NavUser Profile Popover (Account, Password, Language, Reload, Logout)', () => {
  const navUserSource = fs.readFileSync(navUserPath, 'utf8')

  // Bottom navigation trigger testid
  assert.ok(
    navUserSource.includes('bottom-nav-user-button'),
    'NavUser must provide testid="bottom-nav-user-button" when bottomNavigation is true'
  )

  // User identity header navigates to /account
  assert.ok(
    navUserSource.includes("navigate('/account')"),
    'NavUser identity header must navigate to /account'
  )

  // Profile & password action
  assert.ok(
    navUserSource.includes("navigate('/settings/profile')"),
    'NavUser must provide Profile & password navigation to /settings/profile'
  )

  // Language toggle action
  assert.ok(
    navUserSource.includes('data-testid="navuser-language-toggle"'),
    'NavUser must provide language toggle with data-testid="navuser-language-toggle"'
  )
  assert.ok(
    navUserSource.includes('setPreference'),
    'NavUser language toggle must call setPreference'
  )

  // Reload button
  assert.ok(
    navUserSource.includes('<ReloadButton'),
    'NavUser must render ReloadButton'
  )

  // Logout action
  assert.ok(
    navUserSource.includes('handleLogout'),
    'NavUser must provide logout action'
  )
})

test('4. S63-T3: Canonical Route Consolidation (/tasks/:taskId) & Smart Back-Link', () => {
  const appSource = fs.readFileSync(appPath, 'utf8')
  const taskUiSource = fs.readFileSync(taskUiPath, 'utf8')
  const workspacePathSource = fs.readFileSync(workspacePathPath, 'utf8')
  const taskSessionPageSource = fs.readFileSync(taskSessionPagePath, 'utf8')

  // Canonical taskTargetUrl returns /tasks/:taskId
  assert.ok(
    taskUiSource.includes("return `/tasks/${encodeURIComponent(task.id)}`"),
    'taskTargetUrl must consistently route to canonical /tasks/:taskId'
  )

  // workspacePath returns /tasks/:taskId
  assert.ok(
    workspacePathSource.includes("return `/tasks/${encodeURIComponent(taskId)}`"),
    'workspacePath must return canonical /tasks/:taskId'
  )

  // App.tsx handles canonical redirect from /projects/:projectId/tasks/:taskId -> /tasks/:taskId
  assert.ok(
    appSource.includes("rootSegment === 'projects' && pathSegments[2] === 'tasks' && pathSegments[3]"),
    'App.tsx must detect /projects/:projectId/tasks/:taskId routes'
  )
  assert.ok(
    appSource.includes("taskReturnTo: `/projects/${encodeURIComponent(subParam)}/tasks`"),
    'App.tsx must preserve smart project back-link in taskReturnTo state'
  )

  // App.tsx handleSelectTask navigates to canonical /tasks/:taskId
  assert.ok(
    appSource.includes("navigate(\n      `/tasks/${encodeURIComponent(t.id)}${suffix}`"),
    'handleSelectTask must navigate to canonical /tasks/:taskId'
  )

  // App.tsx alias redirect for /agents -> /settings/agents
  assert.ok(
    appSource.includes("rootSegment === 'agents'"),
    'App.tsx must handle /agents route alias'
  )

  // TaskSessionPage uses taskReturnTo as backURL
  assert.ok(
    taskSessionPageSource.includes('state.taskReturnTo'),
    'TaskSessionPage must read state.taskReturnTo for smart back-link'
  )
})

test('5. S63-T3: Task Layout Outer Padding & HairlineList Optimization', () => {
  const appShellSource = fs.readFileSync(appShellPath, 'utf8')
  const taskBoardViewSource = fs.readFileSync(taskBoardViewPath, 'utf8')

  // AppShell padding reduction for tasks: p-2 sm:p-3 md:p-4 lg:p-6
  assert.ok(
    appShellSource.includes("currentTab === 'tasks'") &&
      appShellSource.includes("'overflow-auto p-2 sm:p-3 md:p-4 lg:p-6'"),
    'AppShell must apply trimmed padding p-2 sm:p-3 md:p-4 lg:p-6 for tasks tab'
  )

  // TaskBoardView HairlineList has no redundant px-2
  assert.ok(
    taskBoardViewSource.includes('<HairlineList className="lg:hidden">'),
    'TaskBoardView must eliminate redundant outer px-2 on mobile HairlineList'
  )
})
