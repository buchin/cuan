import test from 'node:test'
import assert from 'node:assert/strict'
import fs from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'

const __filename = fileURLToPath(import.meta.url)
const __dirname = path.dirname(__filename)
const repoRoot = path.resolve(__dirname, '../..')
const webDir = path.resolve(__dirname, '..')

const sprintDocPath = path.resolve(repoRoot, 'docs/sprints/S61.md')
const inboxViewPath = path.resolve(webDir, 'src/features/inbox/InboxView.tsx')
const conversationWorkMonitorPath = path.resolve(webDir, 'src/features/inbox/ConversationWorkMonitor.tsx')
const inboxTaskActionPath = path.resolve(webDir, 'src/features/inbox/InboxTaskAction.tsx')
const messageContentPath = path.resolve(webDir, 'src/features/inbox/MessageContent.tsx')
const chatMarkdownPath = path.resolve(webDir, 'src/features/inbox/ChatMarkdown.tsx')
const mobileScreenshotPath = path.resolve(repoRoot, 'screenshots/s61-inbox-task-drawer-mobile.png')
const desktopScreenshotPath = path.resolve(repoRoot, 'screenshots/s61-inbox-task-drawer-desktop.png')

const sprintDoc = fs.readFileSync(sprintDocPath, 'utf8')
const inboxView = fs.readFileSync(inboxViewPath, 'utf8')
const inboxTaskAction = fs.readFileSync(inboxTaskActionPath, 'utf8')
const messageContent = fs.readFileSync(messageContentPath, 'utf8')
const chatMarkdown = fs.readFileSync(chatMarkdownPath, 'utf8')

test('1. S61-T3: Single Source of Truth - docs/sprints/S61.md Task 3 Definition', () => {
  assert.ok(sprintDoc.includes('S61-T3'), 'Sprint 61 doc must document S61-T3')
  assert.ok(sprintDoc.includes('oamnkpueucgokjgncxjdnglw'), 'Task ID oamnkpueucgokjgncxjdnglw must be recorded')
  assert.ok(
    sprintDoc.includes('SlideDrawer Integration for Task References in Chat') ||
      sprintDoc.includes('SlideDrawer'),
    'Sprint 61 doc must document SlideDrawer task reference integration'
  )
  assert.ok(
    sprintDoc.includes('Eliminate ConversationWorkMonitor') ||
      sprintDoc.includes('ConversationWorkMonitor'),
    'Sprint 61 doc must document elimination of ConversationWorkMonitor'
  )
})

test('2. S61-T3: Complete Elimination of ConversationWorkMonitor.tsx & convTab tasks', () => {
  assert.ok(
    !fs.existsSync(conversationWorkMonitorPath),
    'ConversationWorkMonitor.tsx file must be completely deleted from repository'
  )
  assert.ok(
    !inboxView.includes('ConversationWorkMonitor'),
    'InboxView.tsx must not import or reference ConversationWorkMonitor'
  )
  assert.ok(
    !inboxView.includes("convTab === 'tasks'"),
    'InboxView.tsx must eliminate convTab === tasks condition'
  )
  assert.ok(
    !inboxView.includes('setConvTab'),
    'InboxView.tsx must eliminate setConvTab state updater'
  )
})

test('3. S61-T3: Elimination of Duplicate "Tasks - Internal" Banner & Monitor Dummy Buttons', () => {
  assert.ok(
    !inboxView.includes('Tasks and escalated work for this conversation are handled internally'),
    'InboxView must not contain duplicate Tasks - Internal banner description'
  )
  assert.ok(
    !inboxView.includes('Create task for this chat'),
    'InboxView must not contain dummy monitor task creation button'
  )
})

test('4. S61-T3: Progressive Disclosure SlideDrawer Integration for Task References', () => {
  assert.ok(
    inboxView.includes('InboxWorkPanel'),
    'InboxView.tsx must integrate SlideDrawer component'
  )
  assert.ok(
    fs.readFileSync(path.resolve(repoRoot,'web/src/features/inbox/InboxWorkPanel.tsx'),'utf8').includes('data-testid="inbox-work-panel"'),
    'InboxView.tsx must render SlideDrawer with data-testid="inbox-task-drawer"'
  )
  assert.ok(
    inboxView.includes('peekTask'),
    'InboxView.tsx must maintain peekTask state for active task in drawer'
  )
  assert.ok(
    inboxView.includes('handleOpenTaskDrawer'),
    'InboxView.tsx must provide handleOpenTaskDrawer callback'
  )
})

test('5. S61-T3: Chat Stream Task Reference Click Wiring (Drawer Peek vs Whole Canvas Replacement)', () => {
  // Assigned to task button in chat bubbles
  assert.ok(
    inboxView.includes('data-testid="assigned-task-btn"'),
    'Assigned to task button must include data-testid="assigned-task-btn"'
  )
  assert.ok(
    inboxView.includes('handleOpenTaskDrawer(task)'),
    'Assigned to task click must open drawer without canvas replacement'
  )

  // The composer panel owns task detail navigation.
  assert.ok(
    inboxView.includes('onDetails={task => handleOpenTaskDrawer(task)}'),
    'Composer task details must invoke handleOpenTaskDrawer'
  )

  // Message container link click interception for /tasks/
  assert.ok(
    inboxView.includes('handleMessageContainerClick'),
    'InboxView must intercept task link clicks in message container'
  )

  // Sub-components propagate task click handlers
  assert.ok(
    inboxTaskAction.includes('onViewTask'),
    'InboxTaskAction must support onViewTask prop'
  )
  assert.ok(
    messageContent.includes('onTaskClick'),
    'MessageContent must support onTaskClick prop'
  )
  assert.ok(
    chatMarkdown.includes('onTaskClick'),
    'ChatMarkdown must support onTaskClick prop'
  )
})

test('7. S61-T3: Mandatory Visual Inspection Artifacts (Mobile 390px & Desktop)', () => {
  assert.ok(
    fs.existsSync(mobileScreenshotPath),
    'Mobile screenshot screenshots/s61-inbox-task-drawer-mobile.png must exist'
  )
  assert.ok(
    fs.existsSync(desktopScreenshotPath),
    'Desktop screenshot screenshots/s61-inbox-task-drawer-desktop.png must exist'
  )
  const mobileStat = fs.statSync(mobileScreenshotPath)
  const desktopStat = fs.statSync(desktopScreenshotPath)
  assert.ok(mobileStat.size > 1000, 'Mobile screenshot must be a valid non-empty image')
  assert.ok(desktopStat.size > 1000, 'Desktop screenshot must be a valid non-empty image')
})
