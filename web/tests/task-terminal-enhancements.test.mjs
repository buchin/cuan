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
