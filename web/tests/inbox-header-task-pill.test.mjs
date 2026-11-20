import test from 'node:test'
import assert from 'node:assert/strict'
import fs from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'

const __dirname = path.dirname(fileURLToPath(import.meta.url))
const inboxViewPath = path.resolve(__dirname, '../src/features/inbox/InboxView.tsx')
const composerTasksPath = path.resolve(__dirname, '../src/features/inbox/ComposerTasks.tsx')
const enJsonPath = path.resolve(__dirname, '../src/locales/en.json')
const idJsonPath = path.resolve(__dirname, '../src/locales/id.json')

test('1. InboxView header active task pill exists and connects to SlideDrawer', () => {
  const inboxViewSrc = fs.readFileSync(inboxViewPath, 'utf8')
  assert.ok(
    inboxViewSrc.includes('data-testid="header-active-task-pill"'),
    'InboxView must define header-active-task-pill for glanceable task monitoring in the room header'
  )
  assert.ok(
    inboxViewSrc.includes('handleOpenTaskDrawer(activeConversationTasks[0])'),
    'Clicking header-active-task-pill must open SlideDrawer for the active task'
  )
  assert.ok(
    inboxViewSrc.includes('StatusDot status="working"'),
    'Header pill must use StatusDot micro-indicator'
  )
})

test('2. ComposerTasks maintains WCAG AA 44px mobile touch target and calm 32px desktop target', () => {
  const composerTasksSrc = fs.readFileSync(composerTasksPath, 'utf8')
  assert.ok(
    composerTasksSrc.includes('min-h-11 sm:min-h-8'),
    'ComposerTasks collapsed badge must satisfy 44px touch target on mobile and 32px on desktop'
  )
})

test('3. Localization completeness for active task details title in both en and id locales', () => {
  const enJson = JSON.parse(fs.readFileSync(enJsonPath, 'utf8'))
  const idJson = JSON.parse(fs.readFileSync(idJsonPath, 'utf8'))

  assert.equal(enJson['View active task details'], 'View active task details')
  assert.equal(idJson['View active task details'], 'Lihat detail tugas aktif')
})
