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

test('2. Schedules consolidated into Settings (S62-T1) eliminating redundant AppShell Operations link', () => {
  const file = path.join(webSrcDir, 'components/layout/AppShell.tsx')
  assert.ok(fs.existsSync(file), 'AppShell.tsx must exist')

  const content = fs.readFileSync(file, 'utf8')
  // In S62-T1, Schedules is consolidated into Settings (/settings/schedules) and eliminated from primary AppShell nav
  assert.ok(!content.includes("to: '/schedules'"), 'AppShell NAV_GROUPS must not contain redundant to: /schedules')
  assert.ok(
    content.includes("currentTab === 'schedules'") && content.includes("t('Schedules')"),
    'Must handle Schedules label if referenced'
  )
})

test('3. SettingsView includes Schedules tab item', () => {
  const file = path.join(webSrcDir, 'features/settings/SettingsView.tsx')
  assert.ok(fs.existsSync(file), 'SettingsView.tsx must exist')

  const content = fs.readFileSync(file, 'utf8')
  assert.ok(fs.readFileSync(path.join(webSrcDir, 'features/settings/SettingsMasterNav.tsx'), 'utf8').includes("id: 'schedules'"), 'Must include schedules tab')
  assert.ok(content.includes('<SchedulesView'), 'Must render SchedulesView component')
})

test('4. App.tsx routes /schedules to SchedulesView', () => {
  const file = path.join(webSrcDir, 'App.tsx')
  assert.ok(fs.existsSync(file), 'App.tsx must exist')

  const content = fs.readFileSync(file, 'utf8')
  assert.ok(content.includes("case 'schedules':\n        return 'schedules'"), 'Must map schedules rootSegment to currentTab schedules')
  assert.ok(content.includes('<SchedulesView'), 'Must render SchedulesView in App.tsx')
})
