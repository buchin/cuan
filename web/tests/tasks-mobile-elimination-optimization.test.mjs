import test from 'node:test'
import assert from 'node:assert/strict'
import fs from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'

const __filename = fileURLToPath(import.meta.url)
const __dirname = path.dirname(__filename)

const appShellPath = path.join(__dirname, '..', 'src', 'components', 'layout', 'AppShell.tsx')
const universalSearchPath = path.join(__dirname, '..', 'src', 'components', 'layout', 'UniversalSearch.tsx')
const taskBoardViewPath = path.join(__dirname, '..', 'src', 'features', 'tasks', 'TaskBoardView.tsx')
const taskItemCardPath = path.join(__dirname, '..', 'src', 'features', 'tasks', 'TaskItemCard.tsx')
const idLocalePath = path.join(__dirname, '..', 'src', 'locales', 'id.json')

test('1. Requirement 1: Top Navigation Bar Search Icon Elimination on Tasks, Inbox, Projects, and Contacts', () => {
  const appShellSource = fs.readFileSync(appShellPath, 'utf8')
  const universalSearchSource = fs.readFileSync(universalSearchPath, 'utf8')

  // Verify AppShell conditionally hides the search icon on Tasks, Inbox, Projects, and Contacts
  assert.ok(
    appShellSource.includes("currentTab === 'tasks'") &&
    appShellSource.includes("currentTab === 'inbox'") &&
    appShellSource.includes("currentTab === 'projects'") &&
    appShellSource.includes("currentTab === 'contacts'"),
    'AppShell must hide the top nav search icon on tasks, inbox, projects, and contacts'
  )
  assert.ok(
    appShellSource.includes('UniversalSearch hideTrigger={shouldHideSearch}'),
    'AppShell must pass shouldHideSearch to UniversalSearch'
  )

  // Verify UniversalSearch supports hideTrigger prop and omits trigger button
  assert.ok(
    universalSearchSource.includes('hideTrigger = false'),
    'UniversalSearch must support hideTrigger parameter with default false'
  )
  assert.ok(
    universalSearchSource.includes('!hideTrigger &&'),
    'UniversalSearch must conditionally render the trigger button based on !hideTrigger'
  )
})

test('2. Requirement 2: Elimination of Show Row and Dropdown Select', () => {
  const taskBoardSource = fs.readFileSync(taskBoardViewPath, 'utf8')

  // Verify redundant 'Show' label and selectbox are completely gone
  assert.ok(
    !taskBoardSource.includes('>Show</span>'),
    'TaskBoardView must completely eliminate the "Show" label text'
  )
  assert.ok(
    !taskBoardSource.includes('aria-label="Task status filter"'),
    'TaskBoardView must completely eliminate the legacy dropdown select'
  )
  assert.ok(
    !taskBoardSource.includes('All tasks ('),
    'TaskBoardView must eliminate the legacy dropdown "All tasks (" option'
  )
})

test('3. Requirement 2: Horizontal Scrollable Filter Chips (Semua, Aktif, Selesai, Dibatalkan/Review)', () => {
  const taskBoardSource = fs.readFileSync(taskBoardViewPath, 'utf8')
  const idLocaleSource = fs.readFileSync(idLocalePath, 'utf8')
  const idLocale = JSON.parse(idLocaleSource)

  // Verify horizontal scrollable container
  assert.ok(
    taskBoardSource.includes('overflow-x-auto') && taskBoardSource.includes('flex sm:hidden'),
    'TaskBoardView must provide a mobile horizontal scrollable container'
  )

  // Verify chip categories: Semua, Aktif, Selesai, Dibatalkan/Review
  assert.ok(
    taskBoardSource.includes("label: t('Semua')"),
    'TaskBoardView must define Semua chip'
  )
  assert.ok(
    taskBoardSource.includes("label: t('Aktif')"),
    'TaskBoardView must define Aktif chip'
  )
  assert.ok(
    taskBoardSource.includes("label: t('Selesai')"),
    'TaskBoardView must define Selesai chip'
  )
  assert.ok(
    taskBoardSource.includes("label: t('Dibatalkan/Review')"),
    'TaskBoardView must define Dibatalkan/Review chip'
  )

  // Verify chip pill styling and touch sizing
  assert.ok(
    taskBoardSource.includes('rounded-full') && taskBoardSource.includes('min-h-9'),
    'Filter chips must have rounded-full pill styling and minimum 36px touch height'
  )

  // Verify translation mappings in id.json
  assert.equal(idLocale['Semua'], 'Semua')
  assert.equal(idLocale['Aktif'], 'Aktif')
  assert.equal(idLocale['Selesai'], 'Selesai')
  assert.equal(idLocale['Dibatalkan/Review'], 'Dibatalkan/Review')
})

test('4. Requirement 3: Elimination of Repetitive "Agent: @sm" on Card Footer', () => {
  const taskItemCardSource = fs.readFileSync(taskItemCardPath, 'utf8')

  // Verify footer does not contain repetitive Agent: label
  assert.ok(
    !taskItemCardSource.includes("{t('Agent')}: <strong>{agentDisplayName(agent)}</strong>"),
    'TaskItemCard footer must not render repetitive Agent label'
  )
  assert.ok(
    !taskItemCardSource.includes("{t('Agent')}:"),
    'TaskItemCard must not display "Agent:" prefix on footer or compact view'
  )

  // Verify footer gives priority to CTA buttons
  assert.ok(
    taskItemCardSource.includes('{/* Footer: Action buttons */}'),
    'TaskItemCard must render dedicated footer for CTA action buttons'
  )

  // Verify CTA buttons have generous touch targets (min-h-9)
  assert.ok(
    taskItemCardSource.includes('min-h-9 inline-flex items-center gap-1.5'),
    'Primary CTA action buttons must have min-h-9 touch targets'
  )
  assert.ok(
    taskItemCardSource.includes('min-h-9 inline-flex items-center gap-1'),
    'Secondary CTA action buttons must have min-h-9 touch targets'
  )
})

test('5. Production Bundle Verification', () => {
  const distDir = path.join(__dirname, '..', 'dist', 'assets')
  assert.ok(fs.existsSync(distDir), 'web/dist/assets must exist')
  const jsFiles = fs.readdirSync(distDir).filter((f) => f.endsWith('.js'))
  assert.ok(jsFiles.length > 0, 'Compiled JS files must exist in web/dist/assets')

  const bundleContent = jsFiles.map((f) => fs.readFileSync(path.join(distDir, f), 'utf8')).join('\n')
  assert.ok(
    bundleContent.includes('Dibatalkan/Review'),
    'Compiled JS bundle must contain Dibatalkan/Review filter chip'
  )
  assert.ok(
    bundleContent.includes('hideTrigger'),
    'Compiled JS bundle must contain hideTrigger prop logic'
  )
})

test('6. Requirement 1 & 2: Complete Audit & Clean Mobile Top Bar Layout (360-390px)', () => {
  const appShellSource = fs.readFileSync(appShellPath, 'utf8')
  const inboxSubPanelPath = path.join(__dirname, '..', 'src', 'features', 'inbox', 'InboxSubPanel.tsx')
  const projectsWorkspacePath = path.join(__dirname, '..', 'src', 'features', 'projects', 'ProjectsWorkspace.tsx')
  const contactsViewPath = path.join(__dirname, '..', 'src', 'features', 'contacts', 'ContactsView.tsx')
  const taskBoardViewPath = path.join(__dirname, '..', 'src', 'features', 'tasks', 'TaskBoardView.tsx')

  const inboxSource = fs.readFileSync(inboxSubPanelPath, 'utf8')
  const projectsSource = fs.readFileSync(projectsWorkspacePath, 'utf8')
  const contactsSource = fs.readFileSync(contactsViewPath, 'utf8')
  const taskSource = fs.readFileSync(taskBoardViewPath, 'utf8')

  // 1. Verify internal search inputs exist in content areas
  assert.ok(taskSource.includes('Search tasks'), 'TaskBoardView must retain internal task search')
  assert.ok(inboxSource.includes('Search conversations'), 'InboxSubPanel must retain internal conversation search')
  assert.ok(projectsSource.includes('Search projects'), 'ProjectsWorkspace must retain internal project search')
  assert.ok(contactsSource.includes('Search contacts and notes'), 'ContactsView must retain internal contact search')

  // 2. Verify AppShell eliminates top nav search on views with internal search
  assert.ok(
    appShellSource.includes("currentTab === 'tasks'") &&
    appShellSource.includes("currentTab === 'inbox'") &&
    appShellSource.includes("currentTab === 'projects'") &&
    appShellSource.includes("currentTab === 'contacts'") &&
    appShellSource.includes("location.pathname.startsWith('/account/contacts')") &&
    appShellSource.includes("location.pathname.startsWith('/settings/agents')"),
    'AppShell must hide top nav search icon on all views having internal search'
  )

  // 3. Verify clean, proportional, and consistent top bar layout on mobile (360–390px)
  assert.ok(
    appShellSource.includes('h-14 md:h-16'),
    'AppShell top bar must maintain standard 56px (h-14) height on mobile'
  )
  assert.ok(
    appShellSource.includes('px-3 md:px-5'),
    'AppShell top bar must maintain px-3 padding on mobile to maximize content width'
  )
  assert.ok(
    appShellSource.includes('page-title min-w-0 truncate'),
    'Page title must include min-w-0 truncate preventing header overflow on 360-390px'
  )
  assert.ok(
    appShellSource.includes('flex shrink-0 items-center gap-1'),
    'Action icons container must be shrink-0 with gap-1 to avoid overlapping or clipping'
  )
})
