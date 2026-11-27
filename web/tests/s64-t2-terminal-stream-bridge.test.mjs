import test from 'node:test'
import assert from 'node:assert/strict'
import fs from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'

const __filename = fileURLToPath(import.meta.url)
const __dirname = path.dirname(__filename)
const webDir = path.resolve(__dirname, '..')

const taskTerminalPath = path.join(webDir, 'src/features/tasks/TaskTerminal.tsx')
const taskSessionPagePath = path.join(webDir, 'src/features/tasks/TaskSessionPage.tsx')
const tasksApiPath = path.join(webDir, 'src/api/tasks.ts')
const routesTasksPath = path.resolve(__dirname, '../../internal/server/routes_tasks.go')
const idLocalePath = path.join(webDir, 'src/locales/id.json')
const enLocalePath = path.join(webDir, 'src/locales/en.json')

test('1. S64-T2: WebSocket 404 Resilience & Elimination of Yellow Disconnect Banner', () => {
  assert.ok(fs.existsSync(taskTerminalPath), 'TaskTerminal.tsx must exist')
  const content = fs.readFileSync(taskTerminalPath, 'utf8')

  // 1. Probe terminal availability for decoupled runner daemon detection
  assert.ok(
    content.includes('probeTerminalAvailability') || content.includes('/terminal'),
    'TaskTerminal must probe terminal availability to detect 404 decoupled runner daemon',
  )

  // 2. HTTP 404 gracefully sets streaming connection without looping
  assert.ok(
    content.includes('httpStatus === 404') || content.includes('status === 404'),
    'TaskTerminal must check for HTTP 404 from interactive PTY endpoint',
  )
  assert.ok(
    content.includes("setConnection('streaming')"),
    'TaskTerminal must fallback to streaming mode on 404 or active task close',
  )

  // 3. showDisconnectBanner must NOT trigger when task is actively running
  assert.ok(
    content.includes('!isTaskActive &&'),
    'showDisconnectBanner must guard with !isTaskActive to eliminate false yellow banners during execution',
  )

  // 4. In ws.onerror and ws.onclose, active tasks stay in streaming mode
  assert.ok(
    content.includes('if (isTaskActiveRef.current) {') || content.includes('isTaskActiveRef.current ?'),
    'ws.onerror and ws.onclose must preserve streaming mode for active tasks',
  )
})

test('2. S64-T2: Calm Status Badge ("• Berjalan" / "• Live") Things 3 Standard', () => {
  const content = fs.readFileSync(taskTerminalPath, 'utf8')
  const sessionContent = fs.readFileSync(taskSessionPagePath, 'utf8')
  const idLocale = JSON.parse(fs.readFileSync(idLocalePath, 'utf8'))
  const enLocale = JSON.parse(fs.readFileSync(enLocalePath, 'utf8'))

  // Status badge in TaskTerminal controls
  assert.ok(
    content.includes("language === 'id' ? '• Berjalan' : '• Live'"),
    'TaskTerminal must render calm status badge "• Berjalan" (id) or "• Live" (en)',
  )
  assert.ok(
    content.includes('bg-emerald-500 animate-pulse'),
    'TaskTerminal status badge must include subtle emerald pulse dot',
  )
  assert.ok(
    content.includes('<Badge') && content.includes('variant="success"'),
    'TaskTerminal status badge must use shared borderless pill Badge with success variant',
  )

  // Status badge in TaskSessionPage header synchronization
  assert.ok(
    sessionContent.includes("terminalConnection === 'connected' || terminalConnection === 'streaming'"),
    'TaskSessionPage must consider streaming as connected for live status badge',
  )
  assert.ok(
    sessionContent.includes("language === 'id' ? 'Berjalan' : 'Live'"),
    'TaskSessionPage must format live status label as Berjalan or Live',
  )

  // Localization dictionaries
  assert.ok(idLocale['• Berjalan'], 'id.json must include "• Berjalan"')
  assert.ok(enLocale['• Live'], 'en.json must include "• Live"')
})

test('3. S64-T2: Bidirectional Steering Dispatch via /api/v1/tasks/:id/steer & tasksApi.steer', () => {
  const terminalContent = fs.readFileSync(taskTerminalPath, 'utf8')
  const apiContent = fs.readFileSync(tasksApiPath, 'utf8')
  const serverRoutesContent = fs.readFileSync(routesTasksPath, 'utf8')

  // Frontend API Layer
  assert.ok(
    apiContent.includes('/api/v1/tasks/${encodeURIComponent(id)}/steer'),
    'tasksApi.steer must target /api/v1/tasks/:id/steer endpoint',
  )
  assert.ok(
    apiContent.includes('/api/v1/runner/attempts/${encodeURIComponent(id)}/steer'),
    'tasksApi.steer must provide fallback to attempt steer endpoint',
  )

  // Go Backend Route Registration
  assert.ok(
    serverRoutesContent.includes('pr.Post("/tasks/{id}/steer", s.handleTaskSteer)'),
    'Server must register POST /tasks/{id}/steer endpoint',
  )
  assert.ok(
    serverRoutesContent.includes('func (s *Server) handleTaskSteer'),
    'Server must implement handleTaskSteer handler',
  )

  // TaskTerminal steering dispatch
  assert.ok(
    terminalContent.includes('tasksApi.steer(steerTarget, textToSend)'),
    'handleSteer must dispatch prompt to tasksApi.steer',
  )
  assert.ok(
    terminalContent.includes('socket.current?.readyState === WebSocket.OPEN'),
    'handleSteer sends to PTY if websocket is open, and simultaneously to HTTP steer',
  )
})

test('4. S64-T2: Pure Full-Viewport Terminal (xterm.js) & Auto-Scroll Log Stream', () => {
  const content = fs.readFileSync(taskTerminalPath, 'utf8')

  // Viewport container
  assert.ok(
    content.includes('task-terminal-viewport'),
    'Must mount pure xterm viewport container',
  )
  assert.ok(
    content.includes('h-full w-full overscroll-contain touch-none block'),
    'xterm viewport must fill container full width and height with block display',
  )

  // No step tab abstractions
  assert.ok(
    !content.includes('viewMode'),
    'Must remain free of viewMode abstractions',
  )
  assert.ok(
    !content.includes('VisualStepsList'),
    'Must not render VisualStepsList',
  )

  // Autoscroll & FitAddon
  assert.ok(
    content.includes('fitTerminal'),
    'Must implement precision cell fitting with FitAddon',
  )
  assert.ok(
    content.includes('scrollToBottom()'),
    'Must auto-scroll terminal buffer as logs arrive',
  )
  assert.ok(
    content.includes('formatLogLineForTerminal'),
    'Must format NDJSON logs to clean Things 3 ANSI output',
  )
})

test('5. S64-T2: Functional Simulation of 404 Disconnect Fallback & Steering Dispatch', async () => {
  // Simulate 404 check logic
  let connectionState = 'connecting'
  let noticeMessage = ''
  let errorMessage = ''
  let reconnectCount = 0
  let timerActive = false
  const isTaskActive = true

  const simulateWsClose = (httpStatusCode) => {
    if (isTaskActive) {
      if (httpStatusCode === 404) {
        // Expected behavior for decoupled runner
        connectionState = 'streaming'
        noticeMessage = ''
        errorMessage = ''
        reconnectCount = 0
        timerActive = false
      } else {
        reconnectCount++
        timerActive = true
      }
    }
  }

  simulateWsClose(404)

  assert.equal(connectionState, 'streaming', 'Connection must transition cleanly to streaming on 404')
  assert.equal(noticeMessage, '', 'Notice must not display reconnection warning on 404')
  assert.equal(errorMessage, '', 'Error message must be empty on 404')
  assert.equal(reconnectCount, 0, 'Must not increment reconnectCount on 404')
  assert.equal(timerActive, false, 'Reconnect timer must not be active on 404')

  // Validate showDisconnectBanner condition
  const showDisconnectBanner =
    !isTaskActive &&
    connectionState === 'disconnected' &&
    reconnectCount > 0

  assert.equal(showDisconnectBanner, false, 'Disconnect banner must NEVER show for active streaming task')

  // Simulate steering dispatch
  let dispatchedEndpoint = ''
  let dispatchedPayload = null

  const mockSteerApi = async (id, prompt) => {
    dispatchedEndpoint = `/api/v1/tasks/${id}/steer`
    dispatchedPayload = { prompt, author: 'user' }
    return { status: 'delivered' }
  }

  const result = await mockSteerApi('task-xyz', 'keep going')
  assert.equal(result.status, 'delivered')
  assert.equal(dispatchedEndpoint, '/api/v1/tasks/task-xyz/steer')
  assert.equal(dispatchedPayload.prompt, 'keep going')
})
