import test from 'node:test'
import assert from 'node:assert/strict'
import fs from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'

const __dirname = path.dirname(fileURLToPath(import.meta.url))
const rootDir = path.resolve(__dirname, '..')
const srcDir = path.resolve(rootDir, 'src')

test('1. SchedulesView: uses flexible HTML5 time input instead of rigid 7-option select', () => {
  const viewPath = path.join(srcDir, 'features/schedules/SchedulesView.tsx')
  assert.ok(fs.existsSync(viewPath), 'SchedulesView.tsx must exist')
  const content = fs.readFileSync(viewPath, 'utf8')

  assert.match(
    content,
    /<(?:input|Input)[^>]*type="time"[^>]*value=\{timeOfDay\}/,
    'SchedulesView must provide flexible type="time" input for 24h schedule selection',
  )
  assert.ok(
    !content.includes('08:00 WIB (Pagi)'),
    'SchedulesView must eliminate hardcoded 7-option select dropdown',
  )
})

test('2. SchedulesView: responsive mobile layout prevents aggressive title truncation', () => {
  const viewPath = path.join(srcDir, 'features/schedules/SchedulesView.tsx')
  const content = fs.readFileSync(viewPath, 'utf8')

  assert.match(
    content,
    /hidden sm:flex w-28 shrink-0 items-center gap-1.5 text-xs font-mono/,
    'SchedulesView must hide dedicated left column on mobile to save horizontal space',
  )
  assert.match(
    content,
    /sm:hidden font-mono font-medium text-foreground\/85/,
    'SchedulesView must render cadence/time cleanly on mobile subtitle line',
  )
  assert.match(
    content,
    /line-clamp-2 block/,
    'SchedulesView title must be allowed to wrap naturally without aggressive early truncation',
  )
})

test('2b. SchedulesView: projectsApi resolution, zero raw UUID fallback, and metadata placement', () => {
  const viewPath = path.join(srcDir, 'features/schedules/SchedulesView.tsx')
  const content = fs.readFileSync(viewPath, 'utf8')

  assert.match(
    content,
    /import\s*\{\s*projectsApi\s*\}\s*from\s*['"]\.\.\/\.\.\/api\/projects['"]/,
    'SchedulesView must import projectsApi from api/projects',
  )
  assert.match(
    content,
    /await projectsApi\.list\(\)/,
    'SchedulesView must fetch projects using projectsApi.list()',
  )
  assert.ok(
    !content.includes('task.project_id ? projects.find((p) => p.id === task.project_id)?.name || task.project_id : null'),
    'SchedulesView must never fallback to raw project UUID hash',
  )
  assert.match(
    content,
    /StatusDot[\s\S]*pt-0\.5 shrink-0/,
    'StatusDot must be top-aligned to title row with pt-0.5',
  )
  assert.match(
    content,
    /bg-emerald-600 dark:bg-emerald-500/,
    'Switch toggle must use calm emerald tone instead of screaming neon',
  )
})

test('3. SchedulesView: satisfies WCAG & Cuan touch targets (min-h-11)', () => {
  const viewPath = path.join(srcDir, 'features/schedules/SchedulesView.tsx')
  const content = fs.readFileSync(viewPath, 'utf8')

  assert.match(
    content,
    /min-h-11 sm:min-h-9/,
    'Action buttons in progressive disclosure and modal must meet min-h-11 touch targets on mobile',
  )
})

test('4. SchedulesView: eliminates hardcoded "WIB" string from formatScheduleTime', () => {
  const viewPath = path.join(srcDir, 'features/schedules/SchedulesView.tsx')
  const content = fs.readFileSync(viewPath, 'utf8')

  assert.ok(
    !content.includes('`\${task.time_of_day || \'08:00\'} WIB`'),
    'formatScheduleTime must not hardcode WIB',
  )
})

test('5. Locales: id.json and en.json provide 100% complete dictionaries for Schedules', () => {
  const idPath = path.join(srcDir, 'locales/id.json')
  const enPath = path.join(srcDir, 'locales/en.json')
  const id = JSON.parse(fs.readFileSync(idPath, 'utf8'))
  const en = JSON.parse(fs.readFileSync(enPath, 'utf8'))

  const requiredKeys = [
    'Scheduled Routines',
    'Automated recurring routines that agents execute without requiring manual prompts.',
    'New Schedule',
    'Schedule updated successfully',
    'Schedule created successfully',
    'Task triggered in background',
    'No scheduled routines yet',
    'Create Schedule',
    'Paused',
    'Pause schedule',
    'Resume schedule',
    'Daily',
    'Weekly',
    'Interval',
    'Interval (Minutes)',
    'Next run:',
    'Last run:',
    'Never executed yet',
    'Agent Instruction (Prompt)',
    'Edit schedule',
    'Delete schedule',
    'Run now',
    'Running...',
    'Edit Scheduled Routine',
    'New Scheduled Routine',
    'Routine Title',
    'Target Project',
    'None / System-wide',
    'Frequency',
    'Time',
    'Day of Week',
    'Monday',
    'Tuesday',
    'Wednesday',
    'Thursday',
    'Friday',
    'Saturday',
    'Sunday',
    'Save Changes',
    'Save Schedule',
    'routine(s) configured',
    'Device timezone',
    'Update routine parameters and agent execution instructions.',
    'Configure routine parameters and agent execution instructions.',
    'Markdown and multi-step workflows supported',
    'characters',
    'This instruction will be executed automatically by the AI assistant on schedule.',
  ]

  for (const key of requiredKeys) {
    assert.ok(key in id, `Missing "${key}" in id.json`)
    assert.ok(key in en, `Missing "${key}" in en.json`)
    assert.ok(id[key], `Empty translation for "${key}" in id.json`)
  }
})

test('6. SchedulesView: SlideDrawer side-sheet replaces cramped modal with spacious prompt editor', () => {
  const viewPath = path.join(srcDir, 'features/schedules/SchedulesView.tsx')
  const content = fs.readFileSync(viewPath, 'utf8')

  assert.match(
    content,
    /<SlideDrawer[\s\S]*isOpen=\{(?:isModalOpen|isDrawerOpen)\}[\s\S]*size="lg"/,
    'SchedulesView must use SlideDrawer with size="lg" instead of cramped centered modal',
  )
  assert.match(
    content,
    /min-h-\[260px\]/,
    'Prompt textarea must have min-h-[260px] to prevent text cutoff and cramped keyhole editing',
  )
  assert.match(
    content,
    /resize-y/,
    'Prompt textarea must allow vertical resizing for comfortable multi-step prompt editing',
  )
  assert.match(
    content,
    /prompt\.md/,
    'Prompt editor must include document studio header with prompt.md title',
  )
})

test('7. SlideDrawer: data-slide-drawer attribute and index.css true slide-in edge pinning', () => {
  const drawerPath = path.join(srcDir, 'components/layout/SlideDrawer.tsx')
  const drawerContent = fs.readFileSync(drawerPath, 'utf8')
  assert.match(
    drawerContent,
    /data-slide-drawer/,
    'SlideDrawer must declare data-slide-drawer attribute on dialog',
  )

  const cssPath = path.join(srcDir, 'index.css')
  const cssContent = fs.readFileSync(cssPath, 'utf8')
  assert.match(
    cssContent,
    /dialog\[data-slide-drawer\]:modal[\s\S]*margin:\s*0 0 0 auto !important/,
    'index.css must pin slide drawer to right edge and override native modal centering',
  )
})

test('8. SchedulesView: deep linking binds SlideDrawer to URL search params (?edit= and ?action=new)', () => {
  const viewPath = path.join(srcDir, 'features/schedules/SchedulesView.tsx')
  const content = fs.readFileSync(viewPath, 'utf8')

  assert.match(
    content,
    /import\s*\{\s*useSearchParams\s*\}\s*from\s*['"]react-router-dom['"]/,
    'SchedulesView must import useSearchParams from react-router-dom',
  )
  assert.match(
    content,
    /const\s*\[searchParams,\s*setSearchParams\]\s*=\s*useSearchParams\(\)/,
    'SchedulesView must bind state to searchParams',
  )
  assert.match(
    content,
    /searchParams\.get\(['"]edit['"]\)/,
    'SchedulesView must inspect edit search parameter',
  )
  assert.match(
    content,
    /searchParams\.get\(['"]action['"]\)\s*===\s*['"]new['"]/,
    'SchedulesView must inspect action=new search parameter',
  )
  assert.match(
    content,
    /\/api\/system\/scheduler\/tasks\/\$\{encodeURIComponent\(editParam\)\}/,
    'SchedulesView must support direct fetch of task when refreshing deep-linked edit URL',
  )
})

test('9. SchedulesView: unified fieldControlClass guarantees symmetrical 44px field sizing and zero empty gaps', () => {
  const viewPath = path.join(srcDir, 'features/schedules/SchedulesView.tsx')
  const content = fs.readFileSync(viewPath, 'utf8')

  assert.match(
    content,
    /const fieldControlClass =\s*['"]h-11 min-h-11 w-full rounded-xl border border-border\/60 bg-background\/80 px-3\.5 text-sm/,
    'SchedulesView must define unified fieldControlClass with exact 44px (h-11) and rounded-xl token',
  )
  assert.match(
    content,
    /scheduleType === 'daily'[\s\S]*grid grid-cols-1 sm:grid-cols-2 gap-3/,
    'Daily cadence must render exactly 2 equal 50% columns without empty holes',
  )
  assert.match(
    content,
    /scheduleType === 'weekly'[\s\S]*grid grid-cols-1 sm:grid-cols-3 gap-3/,
    'Weekly cadence must render exactly 3 equal 33% columns',
  )
})
