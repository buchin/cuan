import test from 'node:test'
import assert from 'node:assert/strict'
import fs from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'

const __dirname = path.dirname(fileURLToPath(import.meta.url))
const rootDir = path.resolve(__dirname, '..')

test('1. Mobile Keyboard Logic: Sticky modifiers, repeat handles, and key definitions', async () => {
  const keyboardLogicPath = path.join(
    rootDir,
    'src/features/tasks/terminal/mobile-keyboard.ts',
  )
  assert.ok(fs.existsSync(keyboardLogicPath), 'mobile-keyboard.ts must exist')

  const {
    applyStickyModifiers,
    toggleStickyMod,
    EMPTY_STICKY_MODS,
    MOBILE_KEYBOARD_ROW1,
    MOBILE_KEYBOARD_ROW2,
  } = await import('../src/features/tasks/terminal/mobile-keyboard.ts')

  // Row 1 layout verification
  const row1Ids = MOBILE_KEYBOARD_ROW1.map((k) => k.id)
  assert.deepEqual(row1Ids, ['esc', 'tab', 'slash', 'dash', 'up', 'end', 'pgup'])

  // Row 2 layout verification
  const row2Ids = MOBILE_KEYBOARD_ROW2.map((k) => k.id)
  assert.deepEqual(row2Ids, ['ctrl', 'alt', 'shift', 'left', 'down', 'right', 'pgdn'])

  // Sticky toggle
  let mods = { ...EMPTY_STICKY_MODS }
  mods = toggleStickyMod(mods, 'ctrl')
  assert.equal(mods.ctrl, true)
  assert.equal(mods.alt, false)

  // Ctrl+C (SIGINT interrupt) mapping
  const ctrlCResult = applyStickyModifiers('c', mods)
  assert.equal(ctrlCResult.payload, '\x03', 'Ctrl+C must map to ASCII 0x03 (^C)')
  assert.equal(ctrlCResult.consumed, true)
  assert.equal(ctrlCResult.nextMods.ctrl, false, 'Modifier must be consumed after character chord')

  // Ctrl+D (EOF) mapping
  const ctrlDResult = applyStickyModifiers('d', { ctrl: true, alt: false, shift: false })
  assert.equal(ctrlDResult.payload, '\x04', 'Ctrl+D must map to ASCII 0x04 (EOT/EOF)')

  // Ctrl+V -> Browser paste action
  const ctrlVResult = applyStickyModifiers('v', { ctrl: true, alt: false, shift: false })
  assert.equal(ctrlVResult.clipboardAction, 'paste', 'Sticky Ctrl+V must trigger browser clipboard paste')
  assert.equal(ctrlVResult.payload, '', 'Sticky Ctrl+V must not send raw SYN byte')

  // Navigation under sticky mod retains modifier latch
  const navResult = applyStickyModifiers('\x1b[A', { ctrl: true, alt: false, shift: false })
  assert.equal(navResult.consumed, false, 'Navigation keys must not unlatch sticky mod')
  assert.equal(navResult.nextMods.ctrl, true)

  // Alt modifier mapping
  const altResult = applyStickyModifiers('x', { ctrl: false, alt: true, shift: false })
  assert.equal(altResult.payload, '\x1bx', 'Alt+x must prepend ESC')

  // Shift modifier mapping
  const shiftResult = applyStickyModifiers('g', { ctrl: false, alt: false, shift: true })
  assert.equal(shiftResult.payload, 'G', 'Shift+g must uppercase')
})

test('2. MobileTerminalKeyboard: Quick actions and anti-blur pointer events', () => {
  const componentPath = path.join(
    rootDir,
    'src/features/tasks/terminal/MobileTerminalKeyboard.tsx',
  )
  assert.ok(fs.existsSync(componentPath), 'MobileTerminalKeyboard.tsx must exist')
  const content = fs.readFileSync(componentPath, 'utf8')

  // Verify quick action buttons exist
  assert.match(content, /Ctrl\+C/, 'Quick Ctrl+C button must be rendered')
  assert.match(content, /Enter/, 'Enter button must be rendered')
  assert.match(content, /Copy/, 'Copy button must be rendered')
  assert.match(content, /Paste/, 'Paste button must be rendered')
  assert.match(content, /Clear/, 'Clear button must be rendered')

  // Verify anti-blur prevention for mobile soft keyboard
  assert.match(
    content,
    /onPointerDown=\{.*?preventDefault\(\)/s,
    'PointerDown must preventDefault to keep soft keyboard open',
  )
  assert.match(content, /touch-manipulation/, 'Must have touch-manipulation CSS class')
})

test('3. TerminalContextMenu: Right-click and touch power tools', () => {
  const contextMenuPath = path.join(
    rootDir,
    'src/features/tasks/terminal/TerminalContextMenu.tsx',
  )
  assert.ok(fs.existsSync(contextMenuPath), 'TerminalContextMenu.tsx must exist')
  const content = fs.readFileSync(contextMenuPath, 'utf8')

  assert.match(content, /onCopy/, 'Must have onCopy action')
  assert.match(content, /onPaste/, 'Must have onPaste action')
  assert.match(content, /onSelectAll/, 'Must have onSelectAll action')
  assert.match(content, /onInterrupt/, 'Must have onInterrupt action')
  assert.match(content, /onToggleFullscreen/, 'Must have onToggleFullscreen action')
  assert.match(content, /onClear/, 'Must have onClear action')
})

test('4. TaskTerminal: Clickable web links, soft-reconnect, and fullscreen support', () => {
  const terminalPath = path.join(
    rootDir,
    'src/features/tasks/TaskTerminal.tsx',
  )
  const content = fs.readFileSync(terminalPath, 'utf8')

  // WebLinksAddon integration
  assert.match(content, /@xterm\/addon-web-links/, 'Must import @xterm/addon-web-links')
  assert.match(content, /new WebLinksAddon\(\)/, 'Must instantiate and load WebLinksAddon')

  // Mobile accessory keyboard integration
  assert.match(content, /MobileTerminalKeyboard/, 'Must render MobileTerminalKeyboard')

  // Context menu integration
  assert.match(content, /TerminalContextMenu/, 'Must render TerminalContextMenu')

  // Fullscreen support
  assert.match(content, /isFullscreen/, 'Must have isFullscreen state')
  assert.match(content, /fixed inset-0 z-50/, 'Must support full viewport overlay')

  // Soft reconnect resilience: does not call terminal.dispose on reconnect
  assert.match(content, /isReconnecting/, 'Must manage isReconnecting state')
  assert.match(content, /t\.reset\(\)/, 'Must reset buffer cleanly on soft reconnect')
})

test('5. Localization: Indonesian dictionary contains all new terminal action strings', () => {
  const idLocalePath = path.join(rootDir, 'src/locales/id.json')
  const idLocale = JSON.parse(fs.readFileSync(idLocalePath, 'utf8'))

  const requiredKeys = [
    'Send Ctrl+C (Interrupt)',
    'Clear terminal',
    'Clear screen',
    'Fullscreen',
    'Exit fullscreen',
    'Toggle fullscreen',
    'Copy selection',
    'Copy all output',
    'Paste into terminal',
    'Select all',
    'Selected text copied.',
    'Output copied.',
    'Clipboard access denied by browser.',
  ]

  for (const key of requiredKeys) {
    assert.ok(idLocale[key], `id.json must contain translation for "${key}"`)
  }
})

test('6. Mobile-Safe Top Navbar Layout (< 360px): Single clean row and no button overlap', () => {
  const sessionPagePath = path.join(
    rootDir,
    'src/features/tasks/TaskSessionPage.tsx',
  )
  assert.ok(fs.existsSync(sessionPagePath), 'TaskSessionPage.tsx must exist')
  const content = fs.readFileSync(sessionPagePath, 'utf8')

  // Header must be a single responsive row, not stacked rows
  assert.match(
    content,
    /<header className="flex h-11 sm:h-12 shrink-0 items-center justify-between border-b bg-background px-2 sm:px-3/,
    'Header must be single unified row (h-11 sm:h-12)'
  )

  // Secondary Row on Mobile must be eliminated to prevent vertical waste and collision
  assert.ok(
    !content.includes('Secondary Row on Mobile'),
    'Secondary row on mobile must be eliminated'
  )

  // Title must have min-w-0 and truncate
  assert.match(
    content,
    /<h2[^>]*truncate[^>]*>/,
    'Task title must truncate with min-w-0 to prevent layout blow-out'
  )

  // Terminal quick action toolbar must be hidden on mobile to prevent overlapping Working badge
  assert.match(
    content,
    /ref=\{setTerminalControls\}\s+className="hidden sm:flex shrink-0 items-center gap-1"/,
    'Terminal action buttons must be hidden on mobile (hidden sm:flex)'
  )
  assert.ok(
    !content.includes('<ReloadButton'),
    'Reload button eliminated from terminal navbar to prevent header clutter'
  )

  // Working badge and info button must be present in clean right container
  assert.match(content, /\{statusBadge\}/, 'Status badge must be rendered')
  assert.match(content, /aria-label="Detail"/, 'Detail/Info button must be rendered')
})

test('7. Conditional Virtual Keyboard & Expandable Accessory Drawer', () => {
  const keyboardPath = path.join(
    rootDir,
    'src/features/tasks/terminal/MobileTerminalKeyboard.tsx',
  )
  const terminalPath = path.join(
    rootDir,
    'src/features/tasks/TaskTerminal.tsx',
  )

  const keyboardContent = fs.readFileSync(keyboardPath, 'utf8')
  const terminalContent = fs.readFileSync(terminalPath, 'utf8')

  // MobileTerminalKeyboard must have sm:hidden so it never takes space on desktop
  assert.match(
    keyboardContent,
    /className=\{`sm:hidden shrink-0/,
    'MobileTerminalKeyboard must include sm:hidden'
  )

  // Must have expandable drawer toggle (Kbd button)
  assert.match(
    keyboardContent,
    /setShowExtended/,
    'Must have showExtended state for expandable drawer'
  )
  assert.match(
    keyboardContent,
    /<span>Kbd<\/span>/,
    'Must render Kbd toggle button'
  )

  // Must have hide/close button
  assert.match(
    keyboardContent,
    /title="Hide keyboard toolbar"/,
    'Must have hide toolbar button'
  )

  // TaskTerminal must manage conditional mobile keyboard state
  assert.match(
    terminalContent,
    /isMobileKeyboardActive/,
    'TaskTerminal must manage isMobileKeyboardActive state'
  )
  assert.match(
    terminalContent,
    /\{liveId && isMobileKeyboardActive && \(\s*<MobileTerminalKeyboard/,
    'MobileTerminalKeyboard must only render when active'
  )

  // TaskTerminal integrates mobile keyboard toggle with TerminalComposer without floating collision
  assert.match(
    terminalContent,
    /onToggleMobileKbd=\{.*?setIsMobileKeyboardActive/,
    'TaskTerminal must integrate onToggleMobileKbd into TerminalComposer'
  )
})

test('8. Mobile Floating Prompt Bar (Isolated Input Capture)', () => {
  const keyboardPath = path.join(
    rootDir,
    'src/features/tasks/terminal/MobileTerminalKeyboard.tsx',
  )
  const content = fs.readFileSync(keyboardPath, 'utf8')

  // Isolated floating prompt form
  assert.match(content, /handleSendPrompt/, 'Must have handleSendPrompt handler')
  assert.match(content, /promptInputRef/, 'Must have ref for isolated input capture')
  assert.match(content, /<form[^>]*onSubmit=\{handleSendPrompt\}/, 'Must render isolated prompt form')
  assert.match(content, /<input[^>]*ref=\{promptInputRef\}/, 'Must render native prompt input field')
  assert.match(content, /placeholder="Type command or input\.\.\."/, 'Must provide prompt placeholder')
  assert.match(content, /<Send className="h-3\.5 w-3\.5"/, 'Must render Send button icon')

  // History recall (Up/Down arrows)
  assert.match(content, /handlePromptKeyDown/, 'Must have keyboard event handler for command history')
  assert.match(content, /ArrowUp/, 'Must support ArrowUp command history recall')

  // HTML viewport interactive-widget setting for Android virtual keyboard
  const htmlPath = path.join(rootDir, 'index.html')
  const htmlContent = fs.readFileSync(htmlPath, 'utf8')
  assert.match(
    htmlContent,
    /interactive-widget=resizes-content/,
    'Viewport meta must include interactive-widget=resizes-content for soft keyboard resilience',
  )
})

test('9. Navigation & Deep Link Task Card Inbox: Detail vs Terminal routing', () => {
  const taskUiPath = path.join(rootDir, 'src/features/tasks/task-ui.ts')
  const cardPath = path.join(rootDir, 'src/features/tasks/TaskItemCard.tsx')
  const appPath = path.join(rootDir, 'src/App.tsx')

  const taskUiContent = fs.readFileSync(taskUiPath, 'utf8')
  const cardContent = fs.readFileSync(cardPath, 'utf8')
  const appContent = fs.readFileSync(appPath, 'utf8')

  // task-ui.ts must export taskDetailUrl and taskTerminalUrl
  assert.match(taskUiContent, /export function taskDetailUrl/, 'task-ui.ts must export taskDetailUrl')
  assert.match(taskUiContent, /export function taskTerminalUrl/, 'task-ui.ts must export taskTerminalUrl')

  // TaskItemCard must import and use taskDetailUrl and taskTerminalUrl
  assert.match(cardContent, /taskDetailUrl/, 'TaskItemCard must import taskDetailUrl')
  assert.match(cardContent, /taskTerminalUrl/, 'TaskItemCard must import taskTerminalUrl')
  assert.match(cardContent, /to=\{detailUrl\}/, 'Detail button must link to detailUrl')
  assert.match(cardContent, /to=\{terminalUrl\}/, 'Live Terminal button must link to terminalUrl')
  assert.doesNotMatch(cardContent, /to=\{terminalUrl\}[\s\S]{1,250}?onClick\(currentTask\)/, 'Live Terminal button must not intercept navigation with onClick')

  // App.tsx must support deep link task detail via activeDetailTaskId
  assert.match(appContent, /activeDetailTaskId/, 'App.tsx must define activeDetailTaskId')
  assert.match(appContent, /handleCloseSidePeek/, 'App.tsx must support smooth close back to taskReturnTo')
})

test('10. Work not found & Terminal Session Error Boundary Handling', () => {
  const sessionPagePath = path.join(rootDir, 'src/features/tasks/TaskSessionPage.tsx')
  const projectWorkViewPath = path.join(rootDir, 'src/features/projects/ProjectWorkView.tsx')

  const sessionContent = fs.readFileSync(sessionPagePath, 'utf8')
  const projectContent = fs.readFileSync(projectWorkViewPath, 'utf8')

  // TaskSessionPage must render error header and structured card on !detail (no raw unstyled p)
  assert.match(sessionContent, /AlertCircle/, 'TaskSessionPage must import AlertCircle for error boundary')
  assert.match(sessionContent, /role=\{error \? 'alert' : 'status'\}/, 'TaskSessionPage must render structured error boundary container')
  assert.match(sessionContent, /t\('Work or session not found'\)/, 'TaskSessionPage must render fallback message')

  // TaskSessionPage on !attempt must provide clear finished / ended state
  assert.match(sessionContent, /t\('Terminal session completed'\)/, 'TaskSessionPage must handle finished attempt with clear title')

  // ProjectWorkView must fetch direct task with tasksApi.get(taskId) and provide error boundary with Back button
  assert.match(projectContent, /tasksApi\.get\(taskId\)/, 'ProjectWorkView must fetch missing task by ID directly')
  assert.match(projectContent, /t\('Work or session not found'\)/, 'ProjectWorkView must render structured error state')
  assert.match(projectContent, /to=\{backURL\}/, 'ProjectWorkView must provide back button')
})

test('11. Chat timeout & interrupted reply recovery handling', () => {
  const inboxActivityPath = path.join(rootDir, 'src/features/inbox/InboxActivity.tsx')
  const inboxActivityContent = fs.readFileSync(inboxActivityPath, 'utf8')

  // Functional failed/interrupted cases live in inbox-recovery-controls.test.mjs.
  assert.match(inboxActivityContent, /inboxRecoveryControls\(q.state, q.recovery_action, accessBlocked\)/, 'Reply recovery must use the shared policy')
  assert.match(inboxActivityContent, /t\(recovering \? 'Trying again…' : 'Try again'\)/, 'Try again action must be rendered')
})

