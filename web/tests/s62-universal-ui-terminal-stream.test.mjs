import test from 'node:test'
import assert from 'node:assert/strict'
import fs from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'

const sharedListSource = fs.readFileSync(new URL('../src/components/layout/ListControls.tsx', import.meta.url), 'utf8')
const __filename = fileURLToPath(import.meta.url)
const __dirname = path.dirname(__filename)
const repoRoot = path.resolve(__dirname, '../..')
const webDir = path.resolve(__dirname, '..')

const taskTerminalPath = path.join(webDir, 'src/features/tasks/TaskTerminal.tsx')
const taskBoardViewPath = path.join(webDir, 'src/features/tasks/TaskBoardView.tsx')
const taskItemCardPath = path.join(webDir, 'src/features/tasks/TaskItemCard.tsx')
const createTaskModalPath = path.join(webDir, 'src/features/tasks/CreateTaskModal.tsx')
const projectsViewPath = path.join(webDir, 'src/features/projects/ProjectsView.tsx')
const contactsViewPath = path.join(webDir, 'src/features/contacts/ContactsView.tsx')
const contactMemoryPanelPath = path.join(webDir, 'src/features/contacts/ContactMemoryPanel.tsx')
const appUpdateCardPath = path.join(webDir, 'src/features/settings/AppUpdateCard.tsx')
const scheduledJobsCardPath = path.join(webDir, 'src/features/settings/ScheduledJobsCard.tsx')
const installAppCardPath = path.join(webDir, 'src/features/settings/InstallAppCard.tsx')
const profileSettingsPath = path.join(webDir, 'src/features/settings/ProfileSettings.tsx')
const agyAdapterPath = path.join(repoRoot, 'internal/agent/agy_adapter.go')
const runnerPath = path.join(repoRoot, 'internal/runner/runner.go')
const idLocalePath = path.join(webDir, 'src/locales/id.json')

test('1. S62-T3: Live Terminal Streaming & Incremental Poller in TaskTerminal.tsx & agy_adapter.go', () => {
  assert.ok(fs.existsSync(taskTerminalPath), 'TaskTerminal.tsx must exist')
  assert.ok(fs.existsSync(agyAdapterPath), 'agy_adapter.go must exist')
  assert.ok(fs.existsSync(runnerPath), 'runner.go must exist')

  const terminalSource = fs.readFileSync(taskTerminalPath, 'utf8')
  const adapterSource = fs.readFileSync(agyAdapterPath, 'utf8')
  const runnerSource = fs.readFileSync(runnerPath, 'utf8')

  // agy_adapter.go and runner.go streaming configuration
  assert.ok(
    adapterSource.includes('outputFormat = "stream-json"'),
    'agy_adapter.go must default to stream-json output format',
  )
  assert.ok(
    adapterSource.includes('{"event":"result"'),
    'agy_adapter.go must parse stream-json result events',
  )
  assert.ok(
    runnerSource.includes('OutputFormat: "stream-json"'),
    'runner.go must specify stream-json output format for real-time chunk streaming',
  )

  // TaskTerminal.tsx incremental poller
  assert.ok(
    terminalSource.includes('pollTimerRef'),
    'TaskTerminal.tsx must declare pollTimerRef for cleanup safety',
  )
  assert.ok(
    terminalSource.includes('pollNextLogs'),
    'TaskTerminal.tsx must define pollNextLogs incremental poller function',
  )
  assert.ok(
    terminalSource.includes('setConnection(\'streaming\')'),
    'TaskTerminal.tsx must set streaming connection state when attempt is active',
  )
  assert.ok(
    terminalSource.includes('streaming: t(\'Streaming live logs…\')'),
    'TaskTerminal.tsx must map streaming status label',
  )
  assert.ok(
    terminalSource.includes('connection === \'streaming\''),
    'TaskTerminal.tsx status indicator must pulse during streaming',
  )
})

test('2. S62-T3: Tasks UI/UX Inset Group & Hairline Audit (Kanban cards, list items, headers, new task modal)', () => {
  const boardSource = fs.readFileSync(taskBoardViewPath, 'utf8')
  const cardSource = fs.readFileSync(taskItemCardPath, 'utf8')
  const modalSource = fs.readFileSync(createTaskModalPath, 'utf8')

  // TaskBoardView desktop table styling
  assert.ok(
    boardSource.includes('border-border/40 bg-card/30'),
    'TaskBoardView desktop table container must use border-border/40 bg-card/30',
  )
  assert.ok(
    boardSource.includes('divide-y divide-border/40'),
    'TaskBoardView table tbody must use hairline divider divide-border/40',
  )
  assert.ok(
    boardSource.includes('border-border/40 bg-muted/20 px-2 text-xs hover:bg-muted/40'),
    'TaskBoardView status picker must use calm border-border/40 bg-muted/20 instead of 1px wireframe border',
  )

  // TaskItemCard Kanban card styling
  assert.ok(
    cardSource.includes('border-border/40 bg-card/60') && cardSource.includes('shadow-2xs'),
    'TaskItemCard card variant must use border-border/40 and shadow-2xs instead of harsh border-border',
  )

  // CreateTaskModal dialog & form controls
  assert.ok(
    modalSource.includes('border-border/40 bg-card/95 backdrop-blur-md'),
    'CreateTaskModal dialog must use border-border/40 bg-card/95',
  )
  assert.ok(
    modalSource.includes('border-border/40 bg-muted/20'),
    'CreateTaskModal controls must use calm border-border/40 bg-muted/20 surface',
  )
})

test('3. S62-T3: Projects & Contacts View Things 3 & Bear Notes Audit', () => {
  const projectsSource = fs.readFileSync(projectsViewPath, 'utf8')
  const contactsSource = fs.readFileSync(contactsViewPath, 'utf8')
  const memoryPanelSource = fs.readFileSync(contactMemoryPanelPath, 'utf8')

  // ProjectsView
  assert.ok(
    projectsSource.includes('HairlineList'),
    'ProjectsView must utilize HairlineList for clean zero-nested-card canvas',
  )
  assert.ok(
    projectsSource.includes('border-border/40 bg-card/40'),
    'ProjectsView HairlineList container must use border-border/40 bg-card/40',
  )

  // ContactsView & ContactMemoryPanel
  assert.ok(
    contactsSource.includes('<ListSearch') && sharedListSource.includes('border border-border/40 bg-muted/20'),
    'ContactsView search input container must use rounded-xl border-border/40 bg-muted/20',
  )
  assert.ok(
    memoryPanelSource.includes('rounded-xl border border-border/40 bg-card/50'),
    'ContactMemoryPanel note cards must use rounded-xl border-border/40 bg-card/50',
  )
  assert.ok(
    !memoryPanelSource.includes('className="rounded-lg border p-3 space-y-2"'),
    'ContactMemoryPanel must eliminate harsh 1px wireframe box borders',
  )
})

test('4. S62-T3: Settings Inset Group Refactoring (AppUpdate, ScheduledJobs, InstallApp, Profile)', () => {
  const appUpdateSource = fs.readFileSync(appUpdateCardPath, 'utf8')
  const scheduledJobsSource = fs.readFileSync(scheduledJobsCardPath, 'utf8')
  const installAppSource = fs.readFileSync(installAppCardPath, 'utf8')
  const profileSource = fs.readFileSync(profileSettingsPath, 'utf8')

  // AppUpdateCard uses GroupedSection
  assert.ok(
    appUpdateSource.includes('<GroupedSection'),
    'AppUpdateCard must use GroupedSection primitive',
  )
  assert.ok(
    !appUpdateSource.includes('className="space-y-4 rounded-lg border border-border bg-card p-5"'),
    'AppUpdateCard must eliminate legacy 1px wireframe border box',
  )

  // ScheduledJobsCard uses GroupedSection & divide-border/40
  assert.ok(
    scheduledJobsSource.includes('<GroupedSection'),
    'ScheduledJobsCard must use GroupedSection primitive',
  )
  assert.ok(
    scheduledJobsSource.includes('divide-border/40'),
    'ScheduledJobsCard must use hairline divider divide-border/40',
  )

  // InstallAppCard uses GroupedSection
  assert.ok(
    installAppSource.includes('<GroupedSection'),
    'InstallAppCard must use GroupedSection primitive',
  )

  // ProfileSettings role card
  assert.ok(
    profileSource.includes('border-border/40 bg-card/50'),
    'ProfileSettings role card must use border-border/40 bg-card/50',
  )
})

test('5. S62-T3: Indonesian Localization Completeness for Live Terminal Streaming', () => {
  const idLocaleSource = fs.readFileSync(idLocalePath, 'utf8')
  const idLocale = JSON.parse(idLocaleSource)

  assert.ok(
    idLocale['Streaming live logs…'] === 'Mengalirkan log langsung…',
    'id.json must contain translation for "Streaming live logs…"',
  )
})
