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

  // Terminal quick action toolbar and ReloadButton must be hidden on mobile to prevent overlapping Working badge
  assert.match(
    content,
    /ref=\{setTerminalControls\}\s+className="hidden sm:flex shrink-0 items-center gap-1"/,
    'Terminal action buttons must be hidden on mobile (hidden sm:flex)'
  )
  assert.match(
    content,
    /<ReloadButton[^>]*hidden sm:inline-flex/,
    'Reload button must be hidden on mobile (hidden sm:inline-flex)'
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

  // TaskTerminal must render floating Kbd button when keyboard is collapsed
  assert.match(
    terminalContent,
    /\{liveId && !isMobileKeyboardActive && connection === 'connected' && \(\s*<button[^>]*sm:hidden/,
    'Must render floating Kbd button when keyboard is collapsed'
  )
})
