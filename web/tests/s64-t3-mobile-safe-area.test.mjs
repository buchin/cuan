import test from 'node:test'
import assert from 'node:assert/strict'
import fs from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'

const __filename = fileURLToPath(import.meta.url)
const __dirname = path.dirname(__filename)
const webDir = path.resolve(__dirname, '..')

const taskTerminalPath = path.join(webDir, 'src/features/tasks/TaskTerminal.tsx')
const terminalComposerPath = path.join(webDir, 'src/features/tasks/terminal/TerminalComposer.tsx')
const mobileKeyboardPath = path.join(webDir, 'src/features/tasks/terminal/MobileTerminalKeyboard.tsx')

test('1. S64-T3: Safe-Area-Inset-Bottom Padding on Composer & Bottom Docked Controls', () => {
  assert.ok(fs.existsSync(terminalComposerPath), 'TerminalComposer.tsx must exist')
  assert.ok(fs.existsSync(taskTerminalPath), 'TaskTerminal.tsx must exist')
  assert.ok(fs.existsSync(mobileKeyboardPath), 'MobileTerminalKeyboard.tsx must exist')

  const composerContent = fs.readFileSync(terminalComposerPath, 'utf8')
  const terminalContent = fs.readFileSync(taskTerminalPath, 'utf8')
  const keyboardContent = fs.readFileSync(mobileKeyboardPath, 'utf8')

  // 1. TerminalComposer root container must specify safe-area-inset-bottom
  assert.ok(
    composerContent.includes('pb-[max(0.75rem,env(safe-area-inset-bottom))]') ||
    composerContent.includes('env(safe-area-inset-bottom)'),
    'TerminalComposer container must apply pb-[max(0.75rem,env(safe-area-inset-bottom))]',
  )

  // 2. TaskTerminal passes or ensures safe-area-inset-bottom on TerminalComposer
  assert.ok(
    terminalContent.includes('pb-[max(0.75rem,env(safe-area-inset-bottom))]') ||
    terminalContent.includes('env(safe-area-inset-bottom)'),
    'TaskTerminal must ensure safe-area padding for bottom composer and terminal viewports',
  )

  // 3. MobileTerminalKeyboard accessory drawer also respects safe-area-inset-bottom
  assert.ok(
    keyboardContent.includes('pb-[max(0.75rem,env(safe-area-inset-bottom))]') ||
    keyboardContent.includes('env(safe-area-inset-bottom)'),
    'MobileTerminalKeyboard must respect safe-area-inset-bottom to prevent bottom navigation bar overlap',
  )
})

test('2. S64-T3: Zero Visual Noise Calm Terminal Composer & Human-Friendly Stop Button', () => {
  const composerContent = fs.readFileSync(terminalComposerPath, 'utf8')

  // 1. Noisy CLI quick action toolbar must be eliminated for zero visual noise (Bear Notes / Things 3 standard)
  assert.ok(
    !composerContent.includes('role="toolbar" aria-label="Terminal Quick Action Chips"'),
    'Noisy CLI developer toolbar must be eliminated for non-developer calm design',
  )

  // 2. Human-friendly stop button must be integrated for rapid process cancellation
  assert.ok(
    composerContent.includes('onStop || onInterrupt'),
    'Must provide onStop or onInterrupt handling for process cancellation',
  )
  assert.ok(
    composerContent.includes('Hentikan proses') || composerContent.includes('Stop'),
    'Must render human-friendly Stop action',
  )
})

test('3. S64-T3: WCAG 2.1 AA Touch Target Accessibility (>= 44x44px for Primary Tap Targets)', () => {
  const composerContent = fs.readFileSync(terminalComposerPath, 'utf8')

  const sharedComposer = fs.readFileSync(path.join(webDir, 'src/components/ui/Composer.tsx'), 'utf8')
  assert.ok(composerContent.includes('<ComposerSendButton'), 'Terminal delegates send layout to shared composer')
  assert.ok(sharedComposer.includes('min-h-11') && sharedComposer.includes('w-12'), 'Shared send target is at least44px high and48px wide')

  // 2. Attachment button must meet >= 44x44px touch target on mobile
  assert.ok(
    composerContent.includes('wa-action-btn min-h-[44px] min-w-[44px]') ||
    (composerContent.includes('wa-action-btn') && composerContent.includes('min-h-[44px]')),
    'Attachment button must satisfy min-h-[44px] min-w-[44px] touch target requirement',
  )

  // 3. Stop button in Quick Action Chips must meet >= 44x44px touch target on mobile
  assert.ok(
    composerContent.includes('min-h-[44px] min-w-[44px] sm:min-h-7'),
    'Stop button and quick chips must satisfy min-h-[44px] touch target requirement on mobile',
  )

  // 4. Mobile text input must have min-h-[44px] on mobile to prevent accidental miss-touches
  assert.ok(
    fs.readFileSync(path.join(webDir, 'src/components/ui/ComposerInput.tsx'), 'utf8').includes('min-h-[44px]'),
    'Input textarea must have min-h-[44px] on mobile',
  )
})

test('4. S64-T3: Zero Visual Noise & Low Cognitive Fatigue Standards', () => {
  const composerContent = fs.readFileSync(terminalComposerPath, 'utf8')
  const terminalContent = fs.readFileSync(taskTerminalPath, 'utf8')

  // 1. Dark theme monochromatic palette (#141414 / #262626 / #222)
  assert.ok(
    composerContent.includes('bg-[#141414]') && composerContent.includes('border-[#262626]'),
    'Composer must use subtle dark monochromatic background and border (#141414 / #262626)',
  )
  assert.ok(
    terminalContent.includes('bg-[#141414]'),
    'Terminal container must use calm Things 3 background #141414',
  )

  // 2. Stop button uses calm rose muted tones (not screaming neon)
  assert.ok(
    composerContent.includes('border-rose-500/30') &&
    composerContent.includes('bg-rose-500/10') &&
    composerContent.includes('text-rose-300'),
    'Stop button must use low-noise calm rose styling (border-rose-500/30, bg-rose-500/10, text-rose-300)',
  )
})
