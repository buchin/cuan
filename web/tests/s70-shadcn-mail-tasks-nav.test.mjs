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
const appPath = path.resolve(webDir, 'src/App.tsx')
const taskBoardViewPath = path.resolve(webDir, 'src/features/tasks/TaskBoardView.tsx')
const inboxSubPanelPath = path.resolve(webDir, 'src/features/inbox/InboxSubPanel.tsx')
const inboxViewPath = path.resolve(webDir, 'src/features/inbox/InboxView.tsx')

test('1. S70-T1: Main & Manage Navigation Architecture (NAV_GROUPS)', () => {
  const appShellSource = fs.readFileSync(appShellPath, 'utf8')
  const appSource = fs.readFileSync(appPath, 'utf8')

  // Verify unified Main group contains New work, Inbox, Projects, Contacts, and Settings without Manage/KELOLA separator
  assert.ok(
    appShellSource.includes("id: 'main'"),
    'NAV_GROUPS must define id: main group'
  )
  assert.ok(
    !appShellSource.includes("id: 'manage'") && !appShellSource.includes("label: 'Manage'"),
    'NAV_GROUPS must not define Manage/KELOLA group or label'
  )
  assert.ok(
    appShellSource.includes("to: '/studio'") &&
    appShellSource.includes("to: '/inbox'") &&
    appShellSource.includes("to: '/projects'") &&
    appShellSource.includes("to: '/contacts'") &&
    !appShellSource.includes("to: '/settings'") &&
    !appShellSource.includes("to: '/tasks'") &&
    appShellSource.includes("navButton('settings', 'Settings', Settings)"),
    'NAV_GROUPS main group must unify daily work routes (/studio, /inbox, /projects, /contacts) while Settings sits in the bottom-left footer'
  )

  // Verify AppTab includes agents, team, skills
  assert.ok(
    appShellSource.includes("'agents'") &&
    appShellSource.includes("'team'") &&
    appShellSource.includes("'skills'"),
    'AppTab must include agents, team, and skills'
  )

  // Verify App.tsx tab navigation handlers
  assert.ok(
    appSource.includes("navigate('/settings/agent')") &&
    appSource.includes("navigate('/settings/team')") &&
    appSource.includes("navigate('/settings/apps-skills')"),
    'App.tsx must route Manage tabs to their respective settings destinations'
  )
})

test('2. S70-T2: Shadcn Tasks Data Table Layout & Faceted Toolbar', () => {
  const taskBoardSource = fs.readFileSync(taskBoardViewPath, 'utf8')

  // Verify Page Header
  assert.ok(
    !taskBoardSource.includes("Here's a list of your tasks for this workspace!") && taskBoardSource.includes('aria-label="Task list"'),
    'Task list has a clear accessible identity without a redundant subtitle'
  )

  // Verify Faceted Filter Buttons
  assert.ok(
    taskBoardSource.includes('PlusCircle') &&
    taskBoardSource.includes('<span>Status</span>') &&
    taskBoardSource.includes('<span>Priority</span>'),
    'TaskBoardView must render Faceted Filter buttons for Status and Priority'
  )

  // Verify Data Table Header Columns: Checkbox, Task, Title, Status, Priority, Time, Actions
  assert.ok(
    taskBoardSource.includes('Select all tasks'),
    'TaskBoardView table header must include Select all tasks checkbox'
  )
  assert.ok(
    taskBoardSource.includes('<span>Task</span>') &&
    taskBoardSource.includes('<span>Title</span>') &&
    taskBoardSource.includes('<span>Status</span>') &&
    taskBoardSource.includes('<span>Priority</span>') &&
    !taskBoardSource.includes('<span>Agent</span>') &&
    taskBoardSource.includes("language === 'id' ? 'Waktu' : 'Time'"),
    'TaskBoardView table header must include all standard data columns'
  )

  // Verify Row Actions dropdown trigger
  assert.ok(
    taskBoardSource.includes('<TaskRowActions') &&
    fs.readFileSync(path.resolve(webDir, 'src/features/tasks/TaskRowActions.tsx'), 'utf8').includes('<ItemActions'),
    'Task rows must use the shared action menu rather than opening task details'
  )

  const pagerSource = fs.readFileSync(new URL('../src/features/tasks/TaskPagePager.tsx', import.meta.url), 'utf8')
  assert.ok(taskBoardSource.includes('<TaskPagePager') && taskBoardSource.includes('total={total}'), 'Task board delegates server totals to shared pagination')
  assert.ok(pagerSource.includes("t('Previous page')") && pagerSource.includes("t('Next page')") && pagerSource.includes('disabled={page >= last}'), 'Shared pager supports accessible page navigation with correct limits')
})

test('3. S70-T3: Shadcn Mail Multi-Pane & Thread Format Layout', () => {
  const inboxSubPanelSource = fs.readFileSync(inboxSubPanelPath, 'utf8')
  const inboxViewSource = fs.readFileSync(inboxViewPath, 'utf8')

  // Verify 3-tier conversation list item
  assert.ok(
    inboxSubPanelSource.includes('inbox-contact-row') &&
    inboxSubPanelSource.includes('formatConversationTime') &&
    inboxSubPanelSource.includes('conversationPreview'),
    'InboxSubPanel must format thread list with 3-tier Apple Mail structure'
  )

  // Verify Search Bar and Category Tabs in InboxSubPanel
  assert.ok(
    inboxSubPanelSource.includes('Search conversations…'),
    'InboxSubPanel must include search bar'
  )
  assert.ok(
    inboxSubPanelSource.includes("<ListTabs aria-label={t('Conversation filters')}"),
    'InboxSubPanel must provide conversation filter tabs'
  )

  // Verify Conversation Reader Header in InboxView
  assert.ok(
    inboxViewSource.includes('contactDisplayName') ||
    inboxViewSource.includes('activeContact.name'),
    'InboxView must display active contact profile in conversation reader'
  )

  // Verify Floating Soft Surface Composer with Whisper mode
  assert.ok(
    inboxViewSource.includes('data-inbox-composer') &&
    inboxViewSource.includes('Public reply') &&
    inboxViewSource.includes('Whisper @cuan'),
    'InboxView must provide floating composer with Public vs Whisper @cuan toggle'
  )
})

test('4. S70-T4: Visual Quality Gate - Screenshots Existence and Validity', () => {
  const screenshots = [
    's70-desktop-tasks-datatable.png',
    's70-mobile-390px-tasks.png',
    's70-desktop-inbox-mail.png',
    's70-mobile-390px-inbox.png',
  ]

  for (const file of screenshots) {
    const screenshotPath = path.resolve(rootDir, 'screenshots', file)
    assert.ok(fs.existsSync(screenshotPath), `Screenshot ${file} must exist in screenshots/`)
    const stats = fs.statSync(screenshotPath)
    assert.ok(stats.size > 5000, `Screenshot ${file} must be a valid non-empty image (> 5KB)`)
  }
})

