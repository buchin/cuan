import test from 'node:test'
import assert from 'node:assert/strict'
import fs from 'node:fs'
import path from 'node:path'

const webSrcDir = fs.existsSync(path.resolve('src')) ? path.resolve('src') : path.resolve('web/src')

test('1. AppUpdateCard component exists and includes version, check update, and auto-update preference', () => {
  const file = path.join(webSrcDir, 'features/settings/AppUpdateCard.tsx')
  assert.ok(fs.existsSync(file), 'AppUpdateCard.tsx must exist')

  const content = fs.readFileSync(file, 'utf8')
  assert.ok(content.includes('/api/system/update'), 'Must query /api/system/update endpoint')
  assert.ok(content.includes('/api/system/update/check'), 'Must trigger /api/system/update/check endpoint')
  assert.ok(content.includes('auto_update_enabled'), 'Must handle auto_update_enabled preference')
  assert.ok(content.includes('auto_update_time'), 'Must handle auto_update_time selection')
  assert.ok(content.includes('RefreshCw'), 'Must use refresh icon for update check')
})

test('2. ScheduledJobsCard component exists and handles background routines inspection and execution', () => {
  const file = path.join(webSrcDir, 'features/settings/ScheduledJobsCard.tsx')
  assert.ok(fs.existsSync(file), 'ScheduledJobsCard.tsx must exist')

  const content = fs.readFileSync(file, 'utf8')
  assert.ok(content.includes('/api/system/scheduler/jobs'), 'Must query /api/system/scheduler/jobs endpoint')
  assert.ok(content.includes('/trigger'), 'Must support triggering job on-demand')
  assert.ok(content.includes('CalendarClock'), 'Must use calendar clock icon')
})

test('3. SettingsView integrates AppUpdateCard and ScheduledJobsCard in General settings tab', () => {
  const file = path.join(webSrcDir, 'features/settings/SettingsView.tsx')
  assert.ok(fs.existsSync(file), 'SettingsView.tsx must exist')

  const content = fs.readFileSync(file, 'utf8')
  assert.ok(fs.readFileSync(new URL('../src/features/settings/GeneralSettings.tsx', import.meta.url), 'utf8').includes('<AppUpdateCard />'), 'SettingsView must render AppUpdateCard')
  assert.ok(content.includes('<SchedulesView embedded />'), 'SettingsView must render ScheduledJobsCard')
})
