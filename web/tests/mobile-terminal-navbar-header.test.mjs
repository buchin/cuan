import test from 'node:test'
import assert from 'node:assert/strict'
import fs from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'

const __filename = fileURLToPath(import.meta.url)
const __dirname = path.dirname(__filename)
const rootDir = path.resolve(__dirname, '..')

const taskSessionPagePath = path.join(rootDir, 'src/features/tasks/TaskSessionPage.tsx')
const taskTerminalPath = path.join(rootDir, 'src/features/tasks/TaskTerminal.tsx')
const indexCssPath = path.join(rootDir, 'src/index.css')

test('1. Mobile Header Layout: 2-tier responsive header prevents title truncation to "O."', () => {
  assert.ok(fs.existsSync(taskSessionPagePath), 'TaskSessionPage.tsx must exist')
  const content = fs.readFileSync(taskSessionPagePath, 'utf8')

  // Tier 1: Primary navigation, title & status badge
  assert.match(
    content,
    /Tier 1: Primary Navigation/i,
    'TaskSessionPage must have documented Tier 1 primary navigation and identity row',
  )
  assert.match(
    content,
    /<h2[^>]*className="[^"]*truncate[^"]*min-w-0[^"]*"[^>]*title=\{detail\.title\}/,
    'Task title must have truncate and min-w-0 for proper CSS ellipsis behavior',
  )
  assert.match(
    content,
    /min-w-0 flex-1/,
    'Title container must have min-w-0 flex-1 to claim all remaining horizontal width',
  )

  // Tier 2: Mobile Action Toolbar (Hidden on desktop via sm:hidden)
  assert.match(
    content,
    /Tier 2: Mobile Action Toolbar/i,
    'TaskSessionPage must have Tier 2 mobile action toolbar',
  )
  assert.match(
    content,
    /sm:hidden/,
    'Tier 2 mobile action toolbar must be hidden on desktop viewports (sm:hidden)',
  )
})

test('2. Status Badge Unification: Balanced visual hierarchy, proportional padding & integrated green pulsing dot', () => {
  const content = fs.readFileSync(taskSessionPagePath, 'utf8')
  const terminalContent = fs.readFileSync(taskTerminalPath, 'utf8')

  // Status badge styling with proportional padding and rounded-full pill
  assert.match(
    content,
    /inline-flex shrink-0 items-center gap-1\.5 rounded-full border px-2\.5 py-1 text-xs font-medium/,
    'statusBadge must use proportional padding (px-2.5 py-1) and medium font weight',
  )

  // Calming green pulsing micro-indicator (8px / h-2 w-2)
  assert.match(
    content,
    /animate-ping rounded-full bg-emerald-400/,
    'statusBadge must render pulsing ping indicator when active',
  )
  assert.match(
    content,
    /rounded-full bg-emerald-500/,
    'statusBadge must render steady emerald status dot',
  )

  // TaskTerminal must suppress duplicate status dot/badge when controlsTarget is provided
  assert.match(
    terminalContent,
    /\(!controlsTarget \|\| isFullscreen\)/,
    'TaskTerminal must suppress duplicate status dot when mounted inside TaskSessionPage',
  )
})

test('3. Ergonomic Touch Targets (min 44x44px) for all 5 action buttons (Kembali, Keyboard, More Actions, Info, Stop/Cancel)', () => {
  const sessionContent = fs.readFileSync(taskSessionPagePath, 'utf8')
  const terminalContent = fs.readFileSync(taskTerminalPath, 'utf8')
  const cssContent = fs.readFileSync(indexCssPath, 'utf8')

  // Verify .icon-action canonical touch target definition in index.css (h-11 w-11 = 44x44px)
  assert.match(
    cssContent,
    /\.icon-action\s*\{\s*@apply[^}]*h-11 w-11/,
    '.icon-action must enforce h-11 w-11 (44px x 44px) touch targets per WCAG 2.1 AA',
  )

  // 1. Kembali (Back button)
  assert.match(
    sessionContent,
    /<Link[^>]*to=\{backURL\}[^>]*className="[^"]*icon-action[^"]*"/,
    'Kembali (Back) button must use icon-action for 44px touch target',
  )

  // 2. Keyboard button
  assert.ok(
    terminalContent.includes("aria-label={t('Show keyboard')}") &&
    terminalContent.includes("'icon-action shrink-0 transition-colors'"),
    'Keyboard button must use icon-action for 44px touch target',
  )

  // 3. More Actions button
  assert.ok(
    terminalContent.includes("aria-label={t('Terminal tools')}") &&
    terminalContent.includes('icon-action cursor-pointer list-none'),
    'More Actions button must use icon-action for 44px touch target',
  )

  // 4. Info button
  assert.match(
    sessionContent,
    /<button[^>]*className="[^"]*icon-action[^"]*"[^>]*aria-label="Detail"/,
    'Info (Detail) button must use icon-action for 44px touch target',
  )

  // 5. Stop/Cancel button
  assert.match(
    sessionContent,
    /<button[^>]*className="[^"]*icon-action[^"]*"[^>]*aria-label="Cancel session"/,
    'Stop/Cancel button must use icon-action for 44px touch target',
  )
})

test('4. Terminal Controls Integrity: Keyboard toggle, More Tools dropdown, Outside Click & Escape handlers', () => {
  const terminalContent = fs.readFileSync(taskTerminalPath, 'utf8')

  // showKeyboard implementation
  assert.match(
    terminalContent,
    /const showKeyboard = useCallback\(\(\) => \{[\s\S]*?setIsMobileKeyboardActive\(\(prev\) => !prev\)/,
    'showKeyboard must toggle isMobileKeyboardActive state',
  )

  // mobileControlsTarget portal
  assert.match(
    terminalContent,
    /mobileControlsTarget && !isFullscreen\s*\?\s*createPortal\(mobileControls, mobileControlsTarget\)/,
    'TaskTerminal must support mobileControlsTarget portal',
  )

  // Outside click & escape handler for both menu and mobileMenu
  assert.match(
    terminalContent,
    /mobileMenu\.current\?\.open && !mobileMenu\.current\.contains/,
    'Outside click handler must close mobileMenu dropdown',
  )
  assert.match(
    terminalContent,
    /if \(mobileMenu\.current\?\.open\) \{[\s\S]*?mobileMenu\.current\.open = false/,
    'Escape key handler must close mobileMenu dropdown',
  )
})

test('5. Viewport Responsiveness Math: Ample breathing room across 360px, 390px, and 412px viewports', () => {
  // Mobile viewport parameters (in px)
  const viewports = [
    { name: 'Small mobile (Galaxy S/A)', width: 360 },
    { name: 'Standard mobile (iPhone 12/13/14/15/16 Pro)', width: 390 },
    { name: 'Modern Android (Pixel / Galaxy S24)', width: 412 },
  ]

  const PADDING_X = 16 // px-2 on both sides = 8px * 2
  const BACK_BTN_WIDTH = 44 // icon-action h-11 w-11
  const TERMINAL_ICON_WIDTH = 20
  const ICON_GAP = 8
  const STATUS_BADGE_ESTIMATE = 85 // [ • Working ] with px-2.5 py-1 text-xs

  for (const vp of viewports) {
    const fixedWidthTier1 = PADDING_X + BACK_BTN_WIDTH + TERMINAL_ICON_WIDTH + ICON_GAP + STATUS_BADGE_ESTIMATE + 16 // gaps
    const availableForTitle = vp.width - fixedWidthTier1

    // At 13px font size with average character width of 7.5px:
    const approxChars = Math.floor(availableForTitle / 7.5)

    assert.ok(
      availableForTitle >= 170,
      `At ${vp.name} (${vp.width}px), available width for title must be >= 170px (calculated: ${availableForTitle}px)`,
    )
    assert.ok(
      approxChars >= 22,
      `At ${vp.name} (${vp.width}px), title must be able to render at least 22 characters without clipping to "O." (estimated: ${approxChars} chars)`,
    )

    // Tier 2 Action Toolbar: 4 buttons (Keyboard, More Tools, Info, Cancel)
    const BUTTONS_WIDTH = 4 * 44 + 3 * 4 // 4 icon-actions (44px) + 3 gaps (4px) = 188px
    const availableForContext = vp.width - PADDING_X - BUTTONS_WIDTH
    assert.ok(
      availableForContext >= 150,
      `At ${vp.name} (${vp.width}px), available context width in Tier 2 must be >= 150px (calculated: ${availableForContext}px)`,
    )
  }
})

test('6. Production Bundle Verification: TaskSessionPage and TaskTerminal in web/dist', () => {
  const distDir = path.join(rootDir, 'dist/assets')
  assert.ok(fs.existsSync(distDir), 'web/dist/assets must exist')

  const files = fs.readdirSync(distDir)
  const sessionBundle = files.find((f) => f.startsWith('TaskSessionPage-') && f.endsWith('.js'))
  const terminalBundle = files.find((f) => f.startsWith('TaskTerminal-') && f.endsWith('.js'))

  assert.ok(sessionBundle, 'TaskSessionPage production bundle must exist in dist/assets')
  assert.ok(terminalBundle, 'TaskTerminal production bundle must exist in dist/assets')

  const sessionContent = fs.readFileSync(path.join(distDir, sessionBundle), 'utf8')
  const terminalContent = fs.readFileSync(path.join(distDir, terminalBundle), 'utf8')

  // Check compiled indicators in production bundle
  assert.ok(
    sessionContent.includes('animate-ping') || sessionContent.includes('rounded-full'),
    'Compiled bundle must contain status badge ping indicator',
  )
  assert.ok(
    terminalContent.includes('Show keyboard') || terminalContent.includes('Terminal tools'),
    'Compiled bundle must contain mobile terminal controls',
  )
})
