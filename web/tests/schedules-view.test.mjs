import test from 'node:test'
import assert from 'node:assert/strict'
import fs from 'node:fs'
import path from 'node:path'

const webSrcDir = fs.existsSync(path.resolve('src')) ? path.resolve('src') : path.resolve('web/src')

test('1. SchedulesView component exists and supports task listing, creation modal, and execution trigger', () => {
  const file = path.join(webSrcDir, 'features/schedules/SchedulesView.tsx')
  assert.ok(fs.existsSync(file), 'SchedulesView.tsx must exist')

  const content = fs.readFileSync(file, 'utf8')
  assert.ok(content.includes('/api/system/scheduler/tasks'), 'Must query /api/system/scheduler/tasks')
  assert.ok(content.includes('CalendarClock'), 'Must use CalendarClock icon')
  assert.ok(content.includes('New Schedule'), 'Must provide New Schedule trigger')
  assert.ok(content.includes('/toggle'), 'Must support toggle endpoint')
  assert.ok(content.includes('Daily'), 'Must support Daily frequency')
  assert.ok(content.includes('Weekly'), 'Must support Weekly frequency')
})

test('2. NavUser includes Schedules link in preferences/automations list', () => {
  const file = path.join(webSrcDir, 'components/layout/NavUser.tsx')
  assert.ok(fs.existsSync(file), 'NavUser.tsx must exist')

  const content = fs.readFileSync(file, 'utf8')
  assert.ok(content.includes('/settings/schedules'), 'Must link to /settings/schedules')
  assert.ok(content.includes("t('Schedules')"), 'Must display translated Schedules label')
})

test('3. SettingsView includes Schedules tab item', () => {
  const file = path.join(webSrcDir, 'features/settings/SettingsView.tsx')
  assert.ok(fs.existsSync(file), 'SettingsView.tsx must exist')

  const content = fs.readFileSync(file, 'utf8')
  assert.ok(content.includes("['schedules', t('Schedules')]"), 'Must include schedules tab')
  assert.ok(content.includes('<SchedulesView'), 'Must render SchedulesView component')
})

test('4. App.tsx routes /schedules to SchedulesView', () => {
  const file = path.join(webSrcDir, 'App.tsx')
  assert.ok(fs.existsSync(file), 'App.tsx must exist')

  const content = fs.readFileSync(file, 'utf8')
  assert.ok(content.includes("case 'schedules':\n        return 'schedules'"), 'Must map schedules rootSegment to currentTab schedules')
  assert.ok(content.includes('<SchedulesView'), 'Must render SchedulesView in App.tsx')
})
