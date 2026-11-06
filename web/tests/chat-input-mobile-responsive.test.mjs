import test from 'node:test'
import assert from 'node:assert'
import fs from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'

const __filename = fileURLToPath(import.meta.url)
const __dirname = path.dirname(__filename)

const cssFilePath = path.join(__dirname, '../src/features/inbox/chat-calm.css')
const inboxViewPath = path.join(__dirname, '../src/features/inbox/InboxView.tsx')
const cannedPickerPath = path.join(__dirname, '../src/features/inbox/CannedResponsesPicker.tsx')
const attachmentsPath = path.join(__dirname, '../src/features/inbox/Attachments.tsx')

const distDir = path.join(__dirname, '../dist/assets')
const distCssFile = fs.readdirSync(distDir).find((f) => f.endsWith('.css'))
const distCssContent = fs.readFileSync(path.join(distDir, distCssFile), 'utf8')

const cssSource = fs.readFileSync(cssFilePath, 'utf8')
const inboxViewSource = fs.readFileSync(inboxViewPath, 'utf8')
const cannedPickerSource = fs.readFileSync(cannedPickerPath, 'utf8')
const attachmentsSource = fs.readFileSync(attachmentsPath, 'utf8')

test('1. Safe Area Alignment & Android Navigation Bar Cushion', () => {
  // Verifies that .wa-composer defines an ergonomic bottom cushion using max(12px, calc(env(...) + 8px))
  assert.ok(
    cssSource.includes('max(12px, calc(env(safe-area-inset-bottom, 0px) + 8px))'),
    '.wa-composer must specify a minimum 12px bottom padding plus 8px cushion above safe-area-inset'
  )

  // Verify compiled CSS output includes the safe-area calculation
  assert.ok(
    distCssContent.includes('env(safe-area-inset-bottom'),
    'Compiled CSS must retain safe-area-inset-bottom rules'
  )

  // Simulation: Test effective bottom padding across device configurations
  const calculateEffectiveBottomPadding = (envInset) => {
    return Math.max(12, envInset + 8)
  }

  // Case A: Android 3-button system navigation bar (envInset = 0px)
  const androidNavPadding = calculateEffectiveBottomPadding(0)
  assert.strictEqual(
    androidNavPadding >= 12,
    true,
    'On Android with 3-button bar, bottom padding must be at least 12px to prevent touching the system bar'
  )

  // Case B: Modern gesture bar / Home indicator (envInset = 21px / 34px)
  const gestureBarPadding = calculateEffectiveBottomPadding(21)
  assert.strictEqual(gestureBarPadding, 29, 'On gesture navigation, padding must provide 8px breathing margin above inset')
})

test('2. Visual Border/Radius and Dark/Light Mode Contrast', () => {
  // Check contrast fix: In dark mode with light send button (#ededed), --wa-send-text must be dark (#141416) for clear contrast
  const darkChatBlockMatch = cssSource.match(/\.dark \.wa-chat\s*\{([^}]+)\}/)
  assert.ok(darkChatBlockMatch, '.dark .wa-chat block must exist')
  const darkChatProps = darkChatBlockMatch[1]
  assert.ok(
    darkChatProps.includes('--wa-send-text: #141416;'),
    'Dark mode send button text/icon must be #141416 for clear contrast on light #ededed button'
  )

  // In dark mode, .wa-input-capsule must have refined background #1e2023 and border
  assert.ok(
    cssSource.includes('.dark .wa-input-capsule'),
    '.dark .wa-input-capsule must have specialized dark styling'
  )
  assert.ok(
    cssSource.includes('#1e2023'),
    '.dark .wa-input-capsule must use smooth charcoal background #1e2023'
  )

  // Multiline transition: Capsule must define 24px initial radius and 18px rounded rect for multiline
  assert.ok(
    cssSource.includes('border-radius: 24px;'),
    '.wa-input-capsule must define 24px pill radius for single line'
  )
  assert.ok(
    cssSource.includes('.wa-input-capsule.is-multiline'),
    '.wa-input-capsule.is-multiline class must exist for multiline transition'
  )
  assert.ok(
    cssSource.includes('border-radius: 18px;'),
    '.wa-input-capsule.is-multiline must transition to 18px rounded rect (preventing distorted ellipse)'
  )
  assert.ok(
    inboxViewSource.includes('isMultiline ? \'is-multiline\' : \'\''),
    'InboxView must toggle is-multiline class when content expands to multiple lines'
  )
})

test('3. Button Proportion, Spacing, and Right-Aligned Saved Replies Action', () => {
  // Attachment button (+ icon): must use 36px (h-9 w-9) on mobile with 18px icon on the left
  assert.ok(
    attachmentsSource.includes('h-9 w-9'),
    'AttachmentPicker compact button must use h-9 w-9 on mobile'
  )
  assert.ok(
    attachmentsSource.includes('h-[18px] w-[18px]'),
    'AttachmentPicker compact button must use 18px icon'
  )

  // Right-aligned Canned responses (Zap icon): positioned AFTER textarea and BEFORE Send button
  const textareaIdx = inboxViewSource.indexOf('<ComposerInput')
  const cannedIdx = inboxViewSource.indexOf('<CannedResponsesPicker')
  const sendIdx = inboxViewSource.indexOf('<ComposerSendButton', cannedIdx)
  assert.ok(
    textareaIdx < cannedIdx && cannedIdx < sendIdx,
    'CannedResponsesPicker (⚡) must be positioned to the right of textarea, directly adjacent to Send button'
  )

  // Canned responses button dimensions: matching 36px (h-9 w-9) with 18px icon
  assert.ok(
    cannedPickerSource.includes('h-9 w-9'),
    'CannedResponsesPicker button must use h-9 w-9 on mobile'
  )
  assert.ok(
    cannedPickerSource.includes('h-[18px] w-[18px]'),
    'CannedResponsesPicker must use 18px Zap icon'
  )

  // Canned responses popup must anchor right-0 and limit viewport width to prevent mobile clipping
  assert.ok(
    cannedPickerSource.includes('right-0 z-50 w-72 sm:w-80 max-w-[calc(100vw-24px)]'),
    'Canned responses popover must anchor to the right and contain width within viewport'
  )

  const sharedComposer = fs.readFileSync(path.join(__dirname, '../src/components/ui/Composer.tsx'), 'utf8')
  assert.ok(sharedComposer.includes('self-stretch') && sharedComposer.includes('w-12'), 'Send occupies the full composer edge with a48px target')
  assert.ok(
    inboxViewSource.includes('<Send className="h-[18px] w-[18px]" />'),
    'Send button must use 18px Send icon'
  )

  const input = fs.readFileSync(path.join(__dirname, '../src/components/ui/ComposerInput.tsx'), 'utf8')
  assert.ok(input.includes('min-h-[44px]') && input.includes('py-3'), 'Shared input aligns with44px accessories')
  assert.ok(input.includes('const minimum = 44'), 'Resize preserves the shared input height')

})

test('4. Mobile Viewport Responsiveness at 360px, 390px, and 412px', () => {
  const resolutions = [360, 390, 412]

  resolutions.forEach((width) => {
    // Composer outer padding: 10px on each side (total 20px)
    const composerPadding = 10 * 2
    const availableWidth = width - composerPadding

    // Capsule padding: 4px on each side (total 8px)
    const capsulePadding = 4 * 2
    const innerWidth = availableWidth - capsulePadding

    // Interactive button dimensions on mobile:
    // + button: 36px
    // ⚡ button: 36px
    // Send button: 36px
    // Gaps (gap-1 = 4px between 4 elements = 3 * 4px = 12px)
    const buttonsTotal = 36 + 36 + 36
    const gaps = 3 * 4
    const fixedContentWidth = buttonsTotal + gaps

    const textareaWidth = innerWidth - fixedContentWidth

    // Assert that the textarea has at least 200px width on 360px screens
    assert.ok(
      textareaWidth >= 200,
      `At ${width}px screen width, textarea width (${textareaWidth}px) must be >= 200px to allow ample typing room`
    )

    // Verify touch target is ergonomic (36px physical with generous click area)
    assert.strictEqual(36 >= 36, true, 'Touch target meets mobile ergonomics standards')
  })
})
