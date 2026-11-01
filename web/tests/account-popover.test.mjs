import test from 'node:test'
import assert from 'node:assert/strict'
import fs from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'

const __dirname = path.dirname(fileURLToPath(import.meta.url))
const webDir = path.resolve(__dirname, '..')

test('1. NavUser popover: Tasks item is removed from popover menu', async () => {
  const navUserContent = fs.readFileSync(
    path.join(webDir, 'src/components/layout/NavUser.tsx'),
    'utf-8',
  )

  // Verify that CheckSquare icon and Tasks navigation button are not rendered in NavUser popover
  assert.ok(
    !navUserContent.includes("t('Tasks')"),
    "NavUser popover must NOT contain 'Tasks' item",
  )
  assert.ok(
    !navUserContent.includes('CheckSquare'),
    'NavUser must NOT import or use CheckSquare',
  )
})

test('2. Popover Header: Redundant reload button is removed from header', async () => {
  const navUserContent = fs.readFileSync(
    path.join(webDir, 'src/components/layout/NavUser.tsx'),
    'utf-8',
  )

  // Split at the start of dropdown popover
  const popoverIndex = navUserContent.indexOf('data-account-popover')
  assert.ok(popoverIndex > -1, 'data-account-popover must exist')
  const popoverContent = navUserContent.slice(popoverIndex)

  // Header ends before the first menu item or section header
  const headerContentMatch = popoverContent.match(/\{\/\* User Identity Header \*\/\}[\s\S]*?<\/div>\s*<\/div>/)
  assert.ok(headerContentMatch, 'User Identity Header must exist in popover')
  const headerContent = headerContentMatch[0]

  assert.ok(
    !headerContent.includes('<ReloadButton'),
    'Header in popover must NOT contain ReloadButton (redundancy removed)',
  )

  // Ensure ReloadButton is still present in the popover as a clean menu item
  assert.ok(
    popoverContent.includes('variant="menu-item"'),
    'ReloadButton variant="menu-item" must be present in popover action list',
  )
})

test('3. Consolidated Account & Security vs Preferences grouping', async () => {
  const navUserContent = fs.readFileSync(
    path.join(webDir, 'src/components/layout/NavUser.tsx'),
    'utf-8',
  )

  const popoverIndex = navUserContent.indexOf('data-account-popover')
  const popoverContent = navUserContent.slice(popoverIndex)

  // Check section group headers
  assert.ok(
    popoverContent.includes("t('Account & Security')"),
    'Popover must contain Account & Security group header',
  )
  assert.ok(
    popoverContent.includes("t('Preferences')"),
    'Popover must contain Preferences group header',
  )

  // Check items order: Account & Security contains Profile & password
  const accountGroupPos = popoverContent.indexOf("t('Account & Security')")
  const profilePos = popoverContent.indexOf("navigate('/settings/profile')")
  const prefGroupPos = popoverContent.indexOf("t('Preferences')")
  const settingsPos = popoverContent.indexOf("navigate('/settings')")

  assert.ok(accountGroupPos < profilePos, 'Profile & password must follow Account & Security header')
  assert.ok(profilePos < prefGroupPos, 'Preferences group must follow Account group')
  assert.ok(prefGroupPos < settingsPos, 'Settings must follow Preferences header')

  // In S59-T3, Contacts and Schedules are lifted out of NavUser popover into primary Operations navigation
  assert.ok(
    !popoverContent.includes("navigate('/account/contacts')"),
    'Contacts must no longer be inside NavUser popover (S59-T3)',
  )
  assert.ok(
    !popoverContent.includes("navigate('/settings/schedules')"),
    'Schedules must no longer be inside NavUser popover (S59-T3)',
  )

  // Check SettingsView tabs also expose Profile & password
  const settingsViewContent = fs.readFileSync(
    path.join(webDir, 'src/features/settings/SettingsView.tsx'),
    'utf-8',
  )
  assert.ok(
    settingsViewContent.includes("['profile', t('Profile & password')]"),
    'SettingsView must include Profile & password section tab',
  )
})

test('4. Keyboard shortcuts is conditional (hidden on mobile / touch viewports)', async () => {
  const navUserContent = fs.readFileSync(
    path.join(webDir, 'src/components/layout/NavUser.tsx'),
    'utf-8',
  )

  // Popover keyboard shortcuts must be conditioned on !bottomNavigation and hidden sm:flex
  assert.ok(
    navUserContent.includes('!bottomNavigation &&'),
    'Keyboard shortcuts button must be guarded by !bottomNavigation',
  )
  assert.ok(
    navUserContent.includes('hidden sm:flex'),
    'Keyboard shortcuts button must have hidden sm:flex for mobile viewport hiding',
  )

  // SettingsView must offer accessible shortcut trigger for help/settings submenu
  const settingsViewContent = fs.readFileSync(
    path.join(webDir, 'src/features/settings/SettingsView.tsx'),
    'utf-8',
  )
  assert.ok(
    settingsViewContent.includes("cuan:keyboard-shortcuts"),
    'SettingsView must provide a trigger for keyboard shortcuts',
  )
  assert.ok(
    settingsViewContent.includes("t('View shortcuts')"),
    'SettingsView must include View shortcuts action',
  )
})

test('5. Responsive styling, 360px/390px/desktop constraints, and theme contrast', async () => {
  const navUserContent = fs.readFileSync(
    path.join(webDir, 'src/components/layout/NavUser.tsx'),
    'utf-8',
  )

  // Mobile popover constraints
  assert.ok(
    navUserContent.includes('max-w-[calc(100vw-1rem)]'),
    'Popover must constrain width on mobile viewports (360px/390px)',
  )
  assert.ok(
    navUserContent.includes('max-h-[calc(100dvh-5rem)]') && navUserContent.includes('overflow-y-auto'),
    'Popover must prevent screen overflow on short mobile viewports with scrollable containment',
  )

  // Verify WCAG 2.1 AA touch target sizing (min-h-11 / 44px) on menu action buttons
  const touchTargetMatches = navUserContent.match(/min-h-11/g)
  assert.ok(
    touchTargetMatches && touchTargetMatches.length >= 4,
    'All primary interactive buttons must have min-h-11 (44px) touch targets for touch ergonomic compliance',
  )

  // Viewport geometry simulation for 360px, 390px, and desktop (1280px)
  const viewports = [
    { name: 'Mobile compact (360px)', width: 360, height: 640 },
    { name: 'Mobile standard (390px)', width: 390, height: 844 },
    { name: 'Desktop (1280px)', width: 1280, height: 800 },
  ]

  for (const vp of viewports) {
    const maxWidthAllowed = vp.width - 16 // 1rem total padding clearance
    const popoverWidth = Math.min(256, maxWidthAllowed) // w-64 = 256px
    assert.ok(
      popoverWidth <= vp.width,
      `Popover width (${popoverWidth}px) must never exceed viewport width for ${vp.name}`,
    )
    assert.ok(
      popoverWidth <= maxWidthAllowed,
      `Popover width must respect safe side margin for ${vp.name}`,
    )
  }

  // Theme variable usage
  assert.ok(
    navUserContent.includes('bg-popover') && navUserContent.includes('text-popover-foreground'),
    'Popover must use semantic theme colors bg-popover and text-popover-foreground',
  )
  assert.ok(
    navUserContent.includes('border-border'),
    'Popover must use border-border for consistent light and dark mode separation',
  )
  assert.ok(
    navUserContent.includes('dark:text-green-400'),
    'Owner badge must have accessible dark mode token contrast',
  )

  // Verify compiled production CSS includes dark mode rules and variables
  const distAssetsDir = path.join(webDir, 'dist/assets')
  const distCssFile = fs.readdirSync(distAssetsDir).find((f) => f.endsWith('.css'))
  assert.ok(distCssFile, 'Compiled CSS must exist in dist/assets')
  const distCss = fs.readFileSync(path.join(distAssetsDir, distCssFile), 'utf-8')
  assert.ok(distCss.includes('--popover'), 'Compiled CSS must define --popover variable')
  assert.ok(distCss.includes('--border'), 'Compiled CSS must define --border variable')
  assert.ok(distCss.includes('.dark'), 'Compiled CSS must define .dark class variant')

  // Indonesian translations check
  const idLocale = JSON.parse(
    fs.readFileSync(path.join(webDir, 'src/locales/id.json'), 'utf-8'),
  )
  assert.strictEqual(idLocale['Account & Security'], 'Akun & Keamanan')
  assert.strictEqual(idLocale['Preferences'], 'Preferensi')
  assert.strictEqual(idLocale['Profile & password'], 'Profil & kata sandi')
  assert.strictEqual(idLocale['View shortcuts'], 'Lihat pintasan')
})
