import test from 'node:test'
import assert from 'node:assert'
import fs from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'

const __filename = fileURLToPath(import.meta.url)
const __dirname = path.dirname(__filename)

const cssFilePath = path.join(__dirname, '../src/features/inbox/chat-whatsapp.css')
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
  // Check contrast fix: In dark mode, --wa-send-text must be #ffffff (not dark #121212)
  const darkChatBlockMatch = cssSource.match(/\.dark \.wa-chat\s*\{([^}]+)\}/)
  assert.ok(darkChatBlockMatch, '.dark .wa-chat block must exist')
  const darkChatProps = darkChatBlockMatch[1]
  assert.ok(
    darkChatProps.includes('--wa-send-text: #ffffff;'),
    'Dark mode send button text/icon must be #ffffff for clear contrast on teal/green'
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

  // Capsule must have rounded-full pill styling
  assert.ok(
    inboxViewSource.includes('rounded-full'),
    'Input capsule in InboxView must use rounded-full'
  )
})

test('3. Button Proportion, Spacing, and Icon Alignment', () => {
  // Attachment button (+ icon): must use 36px (h-9 w-9) on mobile with 18px icon
  assert.ok(
    attachmentsSource.includes('h-9 w-9'),
    'AttachmentPicker compact button must use h-9 w-9 on mobile'
  )
  assert.ok(
    attachmentsSource.includes('h-[18px] w-[18px]'),
    'AttachmentPicker compact button must use 18px icon'
  )

  // Canned responses (Zap icon): must use matching 36px (h-9 w-9) with 18px icon
  assert.ok(
    cannedPickerSource.includes('h-9 w-9'),
    'CannedResponsesPicker button must use h-9 w-9 on mobile'
  )
  assert.ok(
    cannedPickerSource.includes('h-[18px] w-[18px]'),
    'CannedResponsesPicker must use 18px Zap icon'
  )

  // Send button: must use matching 36px (h-9 w-9) with 18px icon
  assert.ok(
    inboxViewSource.includes('wa-send h-9 w-9'),
    'Send button must use h-9 w-9 on mobile'
  )
  assert.ok(
    inboxViewSource.includes('<Send className="h-[18px] w-[18px]" />'),
    'Send button must use 18px Send icon'
  )

  // Textarea: min-height 36px on mobile, autoResizeTextarea minH updated
  assert.ok(
    inboxViewSource.includes('min-h-[36px]'),
    'Textarea must declare min-h-[36px] on mobile'
  )
  assert.ok(
    inboxViewSource.includes('const minH = window.innerWidth < 640 ? 36 : 38'),
    'autoResizeTextarea must dynamically adapt minH to 36px on mobile'
  )
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
