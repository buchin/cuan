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
    /<input[^>]*type="time"[^>]*value=\{timeOfDay\}/,
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
    /hidden sm:flex w-32 shrink-0 items-center gap-2 text-xs font-mono/,
    'SchedulesView must hide dedicated left column on mobile to save horizontal space',
  )
  assert.match(
    content,
    /sm:hidden font-mono font-medium text-foreground\/80/,
    'SchedulesView must render cadence/time cleanly on mobile subtitle line',
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
  ]

  for (const key of requiredKeys) {
    assert.ok(key in id, `Missing "${key}" in id.json`)
    assert.ok(key in en, `Missing "${key}" in en.json`)
    assert.ok(id[key], `Empty translation for "${key}" in id.json`)
  }
})
