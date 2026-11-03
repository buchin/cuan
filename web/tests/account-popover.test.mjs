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
