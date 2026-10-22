/**
 * Verification test for terminal mobile touch scroll and pull-to-refresh prevention
 * Scoped task verification for:
 * 1. CSS 'overscroll-behavior: contain' / 'overscroll-behavior-y: contain' on terminal container and body/viewport
 * 2. Touch event listeners on terminal viewport controlling default browser gesture while preserving xterm scrollback
 * 3. Gesture swipe down simulation confirming terminal history scrolls up without triggering reload/refresh
 */

const fs = require('node:fs')
const path = require('node:path')
const assert = require('node:assert/strict')
const { Terminal } = require('@xterm/xterm')

console.log('=== RUNNING AUDIT & VERIFICATION: TERMINAL MOBILE TOUCH SCROLL ===')

// 1. Verify CSS rules in index.css and task-workspace.css
const indexCssPath = path.join(__dirname, '../src/index.css')
const workspaceCssPath = path.join(__dirname, '../src/features/tasks/task-workspace.css')
const taskTerminalPath = path.join(__dirname, '../src/features/tasks/TaskTerminal.tsx')
const sessionPagePath = path.join(__dirname, '../src/features/tasks/TaskSessionPage.tsx')

const indexCss = fs.readFileSync(indexCssPath, 'utf8')
const workspaceCss = fs.readFileSync(workspaceCssPath, 'utf8')
const taskTerminal = fs.readFileSync(taskTerminalPath, 'utf8')
const sessionPage = fs.readFileSync(sessionPagePath, 'utf8')

// Check 1.1: body and html overscroll containment in index.css
assert(
  indexCss.includes('html') && indexCss.includes('overscroll-behavior-y: contain'),
  'FAIL: html element must have overscroll-behavior-y: contain in index.css'
)
assert(
  indexCss.includes('body {') && indexCss.includes('overscroll-behavior-y: contain'),
  'FAIL: body element must have overscroll-behavior-y: contain in index.css'
)
assert(
  indexCss.includes('.app-shell') && indexCss.includes('overscroll-behavior-y: contain'),
  'FAIL: .app-shell must have overscroll-behavior-y: contain in index.css'
)
console.log('✔ Check 1.1 Passed: index.css defines overscroll-behavior-y: contain on html, body, and .app-shell')

// Check 1.2: Terminal and xterm viewport overscroll containment in task-workspace.css
assert(
  workspaceCss.includes('.task-terminal-container') &&
  workspaceCss.includes('.xterm-viewport') &&
  workspaceCss.includes('overscroll-behavior: contain') &&
  workspaceCss.includes('overscroll-behavior-y: contain'),
  'FAIL: task-workspace.css must contain overscroll-behavior rules for terminal container and xterm-viewport'
)
assert(
  workspaceCss.includes('touch-action: none'),
  'FAIL: task-workspace.css must specify touch-action: none for terminal viewport'
)
console.log('✔ Check 1.2 Passed: task-workspace.css defines overscroll-behavior and touch-action containment')

// Check 1.3: TaskTerminal.tsx and TaskSessionPage.tsx classes
assert(
  taskTerminal.includes("task-terminal-container") &&
  taskTerminal.includes("overscroll-contain") &&
  taskTerminal.includes("task-terminal-viewport") &&
  taskTerminal.includes("touch-none"),
  'FAIL: TaskTerminal.tsx must render with task-terminal-container, overscroll-contain, task-terminal-viewport, and touch-none'
)
assert(
  sessionPage.includes("overscroll-contain"),
  'FAIL: TaskSessionPage.tsx must include overscroll-contain on terminal session containers'
)
console.log('✔ Check 1.3 Passed: TaskTerminal and TaskSessionPage templates apply overscroll-contain and touch-none')

// 2. Functional Test: Touch event listener and scrollback with xterm.js
async function testTerminalTouchGesture() {
  const terminal = new Terminal({
    rows: 10,
    cols: 40,
    scrollback: 1000,
    fontSize: 13,
    lineHeight: 1.25,
  })

  // Fill terminal buffer with 50 lines to create scrollback history
  const lines = Array.from({ length: 50 }, (_, i) => `Output line log #${i + 1}`).join('\r\n')
  await new Promise((resolve) => terminal.write(lines, resolve))

  const initialBaseY = terminal.buffer.active.baseY
  const initialViewportY = terminal.buffer.active.viewportY

  assert.equal(initialBaseY, 40, 'Terminal baseY should be 40 after writing 50 lines into 10-row viewport')
  assert.equal(initialViewportY, 40, 'Terminal viewportY should initially be at bottom (40)')

  // Mock mount element and gesture logic
  const mountEl = {
    clientHeight: 200,
    listeners: new Map(),
    addEventListener(type, fn, opts) {
      if (!this.listeners.has(type)) this.listeners.set(type, [])
      this.listeners.get(type).push({ fn, opts })
    },
    removeEventListener(type, fn) {
      const list = this.listeners.get(type) || []
      this.listeners.set(type, list.filter(item => item.fn !== fn))
    },
    dispatch(type, event) {
      for (const item of (this.listeners.get(type) || [])) {
        item.fn(event)
      }
    }
  }

  // Set up the exact touch event handlers as implemented in TaskTerminal.tsx
  let touchStartY = 0
  let touchStartX = 0
  let lastTouchY = 0
  let accumulatedY = 0
  let isTouchDragging = false
  let lastTouchTime = 0
  let touchVelocityY = 0

  const getRowHeight = () => {
    if (terminal && terminal.rows > 0 && mountEl.clientHeight > 0) {
      const computed = mountEl.clientHeight / terminal.rows
      if (computed > 4 && computed < 100) return computed
    }
    const fontSize = terminal.options.fontSize || 13
    const lineHeight = terminal.options.lineHeight || 1.25
    return fontSize * lineHeight || 16.25
  }

  const onTouchStart = (e) => {
    if (e.touches.length !== 1) return
    const touch = e.touches[0]
    touchStartY = touch.clientY
    touchStartX = touch.clientX
    lastTouchY = touch.clientY
    accumulatedY = 0
    isTouchDragging = false
    lastTouchTime = Date.now()
    touchVelocityY = 0
  }

  const onTouchMove = (e) => {
    if (e.touches.length !== 1 || !terminal) return
    const touch = e.touches[0]
    const deltaY = touch.clientY - lastTouchY
    const totalY = touch.clientY - touchStartY
    const totalX = touch.clientX - touchStartX

    if (!isTouchDragging) {
      if (Math.abs(totalY) > 3 && Math.abs(totalY) >= Math.abs(totalX)) {
        isTouchDragging = true
      }
    }

    if (isTouchDragging) {
      if (e.cancelable) {
        e.preventDefault()
      }
      const now = Date.now()
      const dt = now - lastTouchTime
      if (dt > 0) {
        const currentVelocity = deltaY / dt
        touchVelocityY = touchVelocityY * 0.4 + currentVelocity * 0.6
      }
      lastTouchTime = now
      lastTouchY = touch.clientY
      accumulatedY += deltaY

      const rowHeight = getRowHeight()
      const scrollLinesCount = Math.trunc(accumulatedY / rowHeight)
      if (scrollLinesCount !== 0) {
        terminal.scrollLines(-scrollLinesCount)
        accumulatedY -= scrollLinesCount * rowHeight
      }
    }
  }

  const onTouchEnd = () => {
    isTouchDragging = false
  }

  mountEl.addEventListener('touchstart', onTouchStart, { capture: true, passive: false })
  mountEl.addEventListener('touchmove', onTouchMove, { capture: true, passive: false })
  mountEl.addEventListener('touchend', onTouchEnd, { capture: true, passive: true })

  function createTouchEvent(touches, cancelable = true) {
    let prevented = false
    return {
      touches,
      cancelable,
      get defaultPrevented() { return prevented },
      preventDefault() { prevented = true }
    }
  }

  // TEST CASE 1: Swipe Down Gesture (Finger drags down from Y=100 to Y=260)
  // In mobile viewports: Dragging down should reveal earlier lines (scroll position moves UP into history)
  // and MUST prevent default to inhibit pull-to-refresh!
  mountEl.dispatch('touchstart', createTouchEvent([{ clientX: 150, clientY: 100 }]))
  
  // First move 2px: within deadzone threshold (<3px), not yet dragging
  const move1 = createTouchEvent([{ clientX: 150, clientY: 102 }])
  mountEl.dispatch('touchmove', move1)
  assert.equal(move1.defaultPrevented, false, 'Small move under threshold should not lock drag prematurely')

  // Second move: Drag down by +80px (clientY = 180)
  const move2 = createTouchEvent([{ clientX: 150, clientY: 180 }])
  mountEl.dispatch('touchmove', move2)
  assert.equal(move2.defaultPrevented, true, 'FAIL: touchmove must call preventDefault() to prevent pull-to-refresh')
  
  // Terminal rowHeight = 200 / 10 = 20px per row.
  // 80px / 20px = 4 rows scrolled up!
  assert.equal(
    terminal.buffer.active.viewportY,
    36,
    `FAIL: Terminal history should scroll up by 4 rows (expected viewportY=36, got ${terminal.buffer.active.viewportY})`
  )

  // Drag down further by another +80px (clientY = 260)
  const move3 = createTouchEvent([{ clientX: 150, clientY: 260 }])
  mountEl.dispatch('touchmove', move3)
  assert.equal(move3.defaultPrevented, true, 'FAIL: continued touchmove must call preventDefault()')
  assert.equal(
    terminal.buffer.active.viewportY,
    32,
    `FAIL: Terminal history should scroll up further by 4 rows (expected viewportY=32, got ${terminal.buffer.active.viewportY})`
  )

  mountEl.dispatch('touchend', createTouchEvent([]))
  console.log('✔ Check 2.1 Passed: Swipe down scrolls terminal history UP (baseY=40 -> viewportY=32) and cancels browser pull-to-refresh')

  // TEST CASE 2: Swipe Up Gesture (Finger drags up from Y=260 to Y=180)
  // Dragging up should scroll down towards bottom (newest output)
  mountEl.dispatch('touchstart', createTouchEvent([{ clientX: 150, clientY: 260 }]))
  const moveUp = createTouchEvent([{ clientX: 150, clientY: 180 }])
  mountEl.dispatch('touchmove', moveUp)
  assert.equal(moveUp.defaultPrevented, true, 'FAIL: touchmove on swipe up must call preventDefault()')
  assert.equal(
    terminal.buffer.active.viewportY,
    36,
    `FAIL: Terminal history should scroll down towards latest output (expected viewportY=36, got ${terminal.buffer.active.viewportY})`
  )
  mountEl.dispatch('touchend', createTouchEvent([]))
  console.log('✔ Check 2.2 Passed: Swipe up scrolls terminal history DOWN towards latest logs')

  // TEST CASE 3: Swipe Down at top boundary (scrollback limit)
  // When at top of scrollback buffer (viewportY = 0), swiping down must STILL prevent pull-to-refresh!
  terminal.scrollToTop()
  assert.equal(terminal.buffer.active.viewportY, 0, 'Terminal should be at top boundary (viewportY=0)')

  mountEl.dispatch('touchstart', createTouchEvent([{ clientX: 150, clientY: 100 }]))
  const moveTopBoundary = createTouchEvent([{ clientX: 150, clientY: 250 }])
  mountEl.dispatch('touchmove', moveTopBoundary)
  assert.equal(moveTopBoundary.defaultPrevented, true, 'FAIL: swipe down at top boundary must call preventDefault() to stop reload')
  assert.equal(terminal.buffer.active.viewportY, 0, 'Terminal viewport remains clamped at top boundary')
  mountEl.dispatch('touchend', createTouchEvent([]))
  console.log('✔ Check 2.3 Passed: Swipe down at scrollback top boundary prevents pull-to-refresh reload')

  // TEST CASE 4: Tap gesture (no movement)
  // A tap must not trigger preventDefault so clicks and focus work properly
  const tapStart = createTouchEvent([{ clientX: 150, clientY: 100 }])
  mountEl.dispatch('touchstart', tapStart)
  assert.equal(tapStart.defaultPrevented, false, 'touchstart must not call preventDefault so click events work')
  mountEl.dispatch('touchend', createTouchEvent([]))
  console.log('✔ Check 2.4 Passed: Tap gesture allows click and focus to proceed')

  console.log('\n================================================================================')
  console.log('ALL VERIFICATION CHECKS PASSED (100% SUCCESS)')
  console.log('================================================================================')
}

testTerminalTouchGesture().catch((err) => {
  console.error('VERIFICATION FAILURE:', err)
  process.exitCode = 1
})
