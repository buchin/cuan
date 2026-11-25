import test from 'node:test'
import assert from 'node:assert/strict'
import fs from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'

const __dirname = path.dirname(fileURLToPath(import.meta.url))
const rootDir = path.resolve(__dirname, '..')
const srcDir = path.join(rootDir, 'src')

const projectsViewPath = path.join(srcDir, 'features/projects/ProjectsView.tsx')
const schedulesViewPath = path.join(srcDir, 'features/schedules/SchedulesView.tsx')
const contactsViewPath = path.join(srcDir, 'features/contacts/ContactsView.tsx')
const settingsViewPath = path.join(srcDir, 'features/settings/SettingsView.tsx')

test('1. S59-T4 ProjectsView: Card Diet & Progressive Disclosure with HairlineList & SlideDrawer', () => {
  assert.ok(fs.existsSync(projectsViewPath), 'ProjectsView.tsx must exist')
  const content = fs.readFileSync(projectsViewPath, 'utf8')

  // Primitives usage
  assert.match(content, /HairlineList/, 'ProjectsView must use HairlineList canvas')
  assert.match(content, /HairlineListItem/, 'ProjectsView must use HairlineListItem')
  assert.match(content, /SlideDrawer/, 'ProjectsView must use SlideDrawer for deep technical details')
  assert.match(content, /StatusDot/, 'ProjectsView must use StatusDot micro-indicator for attempt status')

  // Quick actions preserved
  assert.match(content, /handleOpenProjectTerminal/, 'ProjectsView must preserve handleOpenProjectTerminal')
  assert.match(content, /Terminal className="h-3\.5 w-3\.5 text-muted-foreground/, 'ProjectsView must have calm terminal icon')
  assert.match(content, /title=\{t\('Open terminal for this project'\)\}/, 'ProjectsView must have terminal accessible title')
  assert.match(content, /onSelectProject/, 'ProjectsView must provide direct quick open board action')

  // Zero nested cards
  assert.ok(!content.includes('CardContent'), 'ProjectsView must not use CardContent')
  assert.ok(!content.includes('border border-border bg-card p-5 flex flex-col justify-between'), 'ProjectsView must eliminate thick grid card boxes')
})

test('2. S59-T4 SchedulesView: Things 3 Repeating Routines & Progressive Disclosure', () => {
  assert.ok(fs.existsSync(schedulesViewPath), 'SchedulesView.tsx must exist')
  const content = fs.readFileSync(schedulesViewPath, 'utf8')

  // Layout primitives
  assert.match(content, /HairlineList/, 'SchedulesView must use HairlineList')
  assert.match(content, /StatusDot/, 'SchedulesView must use StatusDot')

  // Things 3 repeating routines: time on left, routine in center, switch toggle on right
  assert.match(content, /Clock className="h-3\.5 w-3\.5/, 'SchedulesView must render time indicator on the left')
  assert.match(content, /formatScheduleTime/, 'SchedulesView must format routine time')
  assert.match(content, /role="switch"/, 'SchedulesView must provide switch toggle on the right')
  assert.match(content, /ChevronDown/, 'SchedulesView must provide expandable detail trigger')

  // Progressive disclosure: prompt hidden on main surface, revealed in expandable area
  assert.match(content, /isExpanded &&/, 'SchedulesView must conditionally disclose prompt and cron configuration')
  assert.match(content, /Agent Instruction \(Prompt\)/, 'SchedulesView must disclose prompt in expandable detail')

  // Zero nested cards
  assert.ok(!content.includes('CardContent'), 'SchedulesView must not use CardContent')
})

test('3. S59-T4 ContactsView: Clean Scanning List (Tag Diet) & Bear Note Pad Detail', () => {
  assert.ok(fs.existsSync(contactsViewPath), 'ContactsView.tsx must exist')
  const content = fs.readFileSync(contactsViewPath, 'utf8')

  // Scanning list must eliminate tag badge accumulation from left list items
  // Check that the left list button does not map through c.tags.slice(0, 2)
  const leftListSection = content.split('/* Right Detail:')[0]
  assert.ok(!leftListSection.includes('c.tags.slice(0, 2).map'), 'ContactsView left list must NOT render stacked tag badge pills')

  // Bear Note Pad styling in right pane
  assert.match(content, /max-w-\[75ch\]/, 'ContactsView detail must enforce 65-80ch typographic constraint')
  assert.match(content, /ContactMemoryPanel/, 'ContactsView must integrate ContactMemoryPanel seamlessly')
  assert.match(content, /border-b border-border\/40/, 'ContactsView detail must use clean hairline header divider')
  assert.match(content, /font-mono text-muted-foreground">#<\/span>/, 'ContactsView must render Bear-style hashtag pills at bottom')

  // Zero nested cards
  assert.ok(!content.includes('CardContent'), 'ContactsView must not use CardContent')
})

test('4. S59-T4 SettingsView: Apple & Things 3 Inset Grouped Rows', () => {
  assert.ok(fs.existsSync(settingsViewPath), 'SettingsView.tsx must exist')
  const content = fs.readFileSync(settingsViewPath, 'utf8')

  // GroupedSection & GroupedRow primitives
  assert.match(content, /GroupedSection/, 'SettingsView must use GroupedSection primitive')
  assert.match(content, /GroupedRow/, 'SettingsView must use GroupedRow primitive')

  // Semantic grouping
  assert.match(content, /title=\{t\('Interface & Language'\)\}/, 'SettingsView must group appearance settings')
  assert.match(content, /<GeneralSettings/, 'General settings must delegate to its shared component')
  assert.match(fs.readFileSync(new URL('../src/features/settings/GeneralSettings.tsx', import.meta.url), 'utf8'), /Productivity & Shortcuts/, 'General settings retains shortcuts')
  assert.ok(!content.includes("title={t('Project settings')}"), 'Project settings must stay in the project workspace')

  // Zero nested cards
  assert.ok(!content.includes('CardContent'), 'SettingsView must not use CardContent')
})

test('5. S59-T4 Quality Gate: Zero Nested Cards Audit across Core Pages', () => {
  const files = [projectsViewPath, schedulesViewPath, contactsViewPath, settingsViewPath]
  for (const filePath of files) {
    const code = fs.readFileSync(filePath, 'utf8')
    assert.ok(!code.includes('<Card>'), `${path.basename(filePath)} must not use <Card>`)
    assert.ok(!code.includes('<CardContent>'), `${path.basename(filePath)} must not use <CardContent>`)
    assert.ok(!code.includes('<CardHeader>'), `${path.basename(filePath)} must not use <CardHeader>`)
  }
})
