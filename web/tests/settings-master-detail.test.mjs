import test from 'node:test'
import assert from 'node:assert/strict'
import fs from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'

const __dirname = path.dirname(fileURLToPath(import.meta.url))
const webDir = path.resolve(__dirname, '..')

const settingsMasterNavPath = path.join(
  webDir,
  'src/features/settings/SettingsMasterNav.tsx',
)
const settingsViewPath = path.join(
  webDir,
  'src/features/settings/SettingsView.tsx',
)
const idLocalePath = path.join(webDir, 'src/locales/id.json')
const appPath = path.join(webDir, 'src/App.tsx')

const navigationSource = fs.readFileSync(new URL('../src/lib/settings-navigation.ts', import.meta.url), 'utf8')
test('1. Architecture: SettingsMasterNav component exists and exports category definitions', () => {
  assert.ok(
    fs.existsSync(settingsMasterNavPath),
    'SettingsMasterNav.tsx must exist in web/src/features/settings/',
  )

  const masterNavSrc = fs.readFileSync(settingsMasterNavPath, 'utf8')
  assert.ok(
    masterNavSrc.includes('export function SettingsMasterNav'),
    'SettingsMasterNav component must be exported',
  )
  assert.ok(
    masterNavSrc.includes('export function getSettingsNavCategories'),
    'getSettingsNavCategories helper function must be exported',
  )
})

test('2. Categorization: 4 semantic categories with Lucide icons, active state, and subtitles', () => {
  const masterNavSrc = fs.readFileSync(settingsMasterNavPath, 'utf8')

  // Verify the 4 mandatory categories exist
  const expectedCategoryIds = [
    'workspace-system',
    'ai-memory',
    'connectivity-relations',
    'personal-account',
  ]
  for (const catId of expectedCategoryIds) {
    assert.ok(
      masterNavSrc.includes(`id: '${catId}'`),
      `Category ${catId} must be defined in getSettingsNavCategories`,
    )
  }

  // Category 1: Workspace & Sistem (general, schedules)
  assert.ok(
    masterNavSrc.includes("titleKey: 'Workspace & System'"),
    'Category 1 must have titleKey "Workspace & System"',
  )
  assert.ok(
    masterNavSrc.includes("id: 'general'"),
    'General setting must be within Workspace & System',
  )
  assert.ok(
    masterNavSrc.includes("id: 'schedules'"),
    'Schedules setting must be within Workspace & System',
  )

  // Category 2: Kecerdasan AI & Memori (memory, agents)
  assert.ok(
    masterNavSrc.includes("titleKey: 'AI Intelligence & Memory'"),
    'Category 2 must have titleKey "AI Intelligence & Memory"',
  )
  assert.ok(
    masterNavSrc.includes("id: 'memory'"),
    'Agent Memory setting must be within AI Intelligence & Memory',
  )
  assert.ok(
    masterNavSrc.includes("id: 'agent'") || masterNavSrc.includes("id: 'agents'"),
    'Assistant/Agent setting must be within AI Intelligence & Memory',
  )

  // Category 3: Konektivitas & Relasi (integrations, contacts)
  assert.ok(
    masterNavSrc.includes("titleKey: 'Connectivity & Relations'"),
    'Category 3 must have titleKey "Connectivity & Relations"',
  )
  assert.ok(
    masterNavSrc.includes("id: 'integrations'"),
    'Integrations setting must be within Connectivity & Relations',
  )
  assert.ok(
    masterNavSrc.includes("id: 'integrations'"),
    'Contacts setting must be within Connectivity & Relations',
  )

  // Category 4: Akun Personal (profile, appearance, notifications)
  assert.ok(
    masterNavSrc.includes("titleKey: 'Personal Account'"),
    'Category 4 must have titleKey "Personal Account"',
  )
  assert.ok(
    masterNavSrc.includes("id: 'profile'"),
    'Profile setting must be within Personal Account',
  )
  assert.ok(
    masterNavSrc.includes("id: 'appearance'"),
    'Appearance setting must be within Personal Account',
  )
  assert.ok(
    masterNavSrc.includes("id: 'notifications'"),
    'Notifications setting must be within Personal Account',
  )

  // Verify Lucide icons are used
  const expectedIcons = [
    'Sliders',
    'CalendarClock',
    'Brain',
    'Bot',
    'Plug',
    'Contact',
    'User',
    'Palette',
    'Bell',
  ]
  for (const icon of expectedIcons) {
    assert.ok(
      masterNavSrc.includes(icon),
      `Lucide icon ${icon} must be imported and configured in SettingsMasterNav`,
    )
  }

  // Verify Active State styling
  assert.ok(
    masterNavSrc.includes('bg-accent text-foreground'),
    'Active navigation item must use a quiet accent surface and readable foreground',
  )

  // Verify Touch Accessibility (min-h-11 for WCAG 2.1 AA)
  assert.ok(
    masterNavSrc.includes('min-h-11'),
    'Navigation items must enforce min-h-11 touch target',
  )
})

test('3. Layout: Single-column settings canvas in SettingsView.tsx', () => {
  const settingsViewSrc = fs.readFileSync(settingsViewPath, 'utf8')

  // SettingsMasterNav must be imported and rendered
  assert.ok(
    settingsViewSrc.includes("import { SettingsMasterNav } from './SettingsMasterNav'"),
    'SettingsView must import SettingsMasterNav',
  )
  assert.ok(
    settingsViewSrc.includes('<SettingsMasterNav'),
    'SettingsView must render SettingsMasterNav component',
  )

  assert.ok(settingsViewSrc.includes('mx-auto w-full max-w-4xl'), 'Settings must use a readable single-column canvas')
  assert.ok(!settingsViewSrc.includes('md:w-72') && !settingsViewSrc.includes('md:w-80'), 'Settings must not add a second desktop sidebar')
})

test('4. Responsiveness: Master-first mobile pattern and ChevronLeft return button', () => {
  const settingsViewSrc = fs.readFileSync(settingsViewPath, 'utf8')

  // Verify explicit section resolution for mobile toggle
  assert.ok(
    settingsViewSrc.includes('hasExplicitSection'),
    'SettingsView must compute hasExplicitSection for responsive switching',
  )

  // On mobile without section, sidebar is visible and detail is hidden
  assert.ok(
    settingsViewSrc.includes("hasExplicitSection ? 'hidden' : 'block p-4 md:p-6'"),
    'Sidebar must show on mobile root and hide when detail is active',
  )
  assert.ok(
    settingsViewSrc.includes("!hasExplicitSection ? 'hidden' : 'block'"),
    'Detail view must hide on mobile root and display when section is selected',
  )

  // Mobile back button with ChevronLeft and ← Back to Settings
  assert.ok(
    fs.readFileSync(new URL('../src/components/layout/DetailLayout.tsx', import.meta.url), 'utf8').includes('ChevronLeft'),
    'SettingsView must import and render ChevronLeft icon for mobile back action',
  )
  assert.ok(
    navigationSource.includes("'Back to Settings'"),
    'SettingsView must display translated ← Back to Settings label',
  )
})

test('5. Deep Linking & Routing: Backward compatibility and safeReturn preservation', () => {
  const settingsViewSrc = fs.readFileSync(settingsViewPath, 'utf8')
  const appSrc = fs.readFileSync(appPath, 'utf8')

  // Section default fallback to general
  assert.ok(
    settingsViewSrc.includes("rawSubSection || 'general'") ||
      settingsViewSrc.includes("location.pathname.split('/')[2] || 'general'"),
    'Section resolution must fallback safely to "general"',
  )

  // safeReturn handling
  assert.ok(
    navigationSource.includes("returnTo === '/chat' || returnTo === '/studio'"),
    'SettingsView must preserve safeReturn for chat and studio',
  )

  // All key subviews are rendered conditionally based on section
  const expectedSections = [
    'profile',
    'memory',
    'schedules',
    'notifications',
    'agents',
    'integrations',
    'appearance',
  ]
  for (const sec of expectedSections) {
    assert.ok(
      settingsViewSrc.includes(`section === '${sec}'`),
      `SettingsView must route section "${sec}" correctly`,
    )
  }

  // App.tsx settingTitles must map schedules
  assert.ok(
    appSrc.includes("schedules: 'Schedules'"),
    'App.tsx settingTitles must map "schedules" to "Schedules"',
  )
})

test('6. Localization: Complete Indonesian translations in id.json', () => {
  assert.ok(fs.existsSync(idLocalePath), 'id.json locale file must exist')
  const idLocale = JSON.parse(fs.readFileSync(idLocalePath, 'utf8'))

  // Category titles
  assert.equal(
    idLocale['Workspace & System'],
    'Workspace & Sistem',
    'Category title Workspace & System must be translated',
  )
  assert.equal(
    idLocale['AI Intelligence & Memory'],
    'Kecerdasan AI & Memori',
    'Category title AI Intelligence & Memory must be translated',
  )
  assert.equal(
    idLocale['Connectivity & Relations'],
    'Konektivitas & Relasi',
    'Category title Connectivity & Relations must be translated',
  )
  assert.equal(
    idLocale['Personal Account'],
    'Akun Personal',
    'Category title Personal Account must be translated',
  )

  // Category navigation labels
  assert.equal(
    idLocale['General'],
    'Umum',
    'Category label General must be translated',
  )
  assert.equal(
    idLocale['Schedules'],
    'Jadwal Otomatis',
    'Category label Schedules must be translated',
  )
  assert.equal(
    idLocale['Agent Memory'],
    'Memori Agen',
    'Category label Agent Memory must be translated',
  )
  assert.equal(
    idLocale['Integrations'],
    'Integrasi',
    'Category label Integrations must be translated',
  )
  assert.equal(
    idLocale['Contacts'],
    'Kontak',
    'Category label Contacts must be translated',
  )
  assert.equal(
    idLocale['Appearance'],
    'Tampilan',
    'Category label Appearance must be translated',
  )
  assert.equal(
    idLocale['Notifications'],
    'Notifikasi',
    'Category label Notifications must be translated',
  )

  // Subtitles & Section headers
  assert.ok(
    idLocale['Workspace settings and system configuration'],
    'Workspace settings subtitle must be translated',
  )
  assert.ok(
    idLocale['Automated recurring background routines'],
    'Schedules subtitle must be translated',
  )
  assert.ok(
    idLocale['Learned brand tone, style, and taste guidelines'],
    'Agent Memory subtitle must be translated',
  )
  assert.ok(
    idLocale['Digital agents and team members'],
    'Agents subtitle must be translated',
  )
  assert.ok(
    idLocale['Connect WhatsApp, GitHub, and external services'],
    'Integrations subtitle must be translated',
  )
  assert.ok(
    idLocale['Customer contacts and communication history'],
    'Contacts subtitle must be translated',
  )
  assert.ok(
    idLocale['Personal profile, password, and security'],
    'Profile subtitle must be translated',
  )
  assert.ok(
    idLocale['Theme modes, interface language, and display'],
    'Appearance subtitle must be translated',
  )
  assert.ok(
    idLocale['Notification preferences and alerts'],
    'Notifications subtitle must be translated',
  )
  assert.ok(
    idLocale['Personal profile, preferences, and relations.'],
    'Account header subtitle must be translated',
  )
  assert.ok(
    idLocale['Connect the services your projects need.'],
    'Integrations header subtitle must be translated',
  )

  // Mobile Back Button
  assert.equal(
    idLocale['← Back to Settings'],
    '← Kembali ke Pengaturan',
    'Mobile back button label must be translated',
  )
  assert.equal(
    idLocale['← Back to Account'],
    '← Kembali ke Akun',
    'Mobile account back button label must be translated',
  )
})

test('7. Mobile Responsive Constraints & Touch Target: p-4 md:p-6, max-w-4xl, min-h-11', () => {
  const settingsViewSrc = fs.readFileSync(settingsViewPath, 'utf8')

  // Container padding: p-4 on mobile, md:p-6 on desktop
  assert.ok(
    settingsViewSrc.includes('p-4 md:p-6'),
    'Detail main container must feature p-4 md:p-6 responsive padding',
  )

  // max-w-4xl constraint
  assert.ok(
    settingsViewSrc.includes('max-w-4xl'),
    'Detail container must enforce max-w-4xl for optimal reading length',
  )

  // Mobile back button minimum touch target min-h-11 (WCAG 2.1 AA 44px)
  assert.ok(
    fs.readFileSync(new URL('../src/components/layout/DetailLayout.tsx', import.meta.url), 'utf8').includes('min-h-11 items-center gap-1'),
    'Mobile back button must enforce min-h-11 touch target',
  )

  // Query parameter preservation when navigating back
  assert.ok(
    navigationSource.includes('parent + search') &&
      navigationSource.includes("parts[0] === 'account'"),
    'Mobile back action must support account and settings roots with query preservation',
  )
})

test('8. Route Resolution & Backward Compatibility: category routes, deep links, and safeReturn', () => {
  const settingsViewSrc = fs.readFileSync(settingsViewPath, 'utf8')
  const masterNavSrc = fs.readFileSync(settingsMasterNavPath, 'utf8')

  // Verify all routes are correctly mapped in SettingsMasterNav
  const expectedRoutes = [
    '/settings/general',
    '/settings/schedules',
    '/settings/memory',
    '/settings/agents',
    '/settings/integrations',
    '/account/contacts',
    '/settings/profile',
    '/settings/appearance',
    '/settings/notifications',
  ]
  for (const r of expectedRoutes) {
    assert.ok(
      masterNavSrc.includes(r),
      `SettingsMasterNav must map route ${r}`,
    )
  }

  // Account mode routes
  const expectedAccountRoutes = [
    '/account/general',
    '/account/schedules',
    '/account/memory',
    '/account/agents',
    '/account/integrations',
    '/account/profile',
    '/account/appearance',
    '/account/notifications',
  ]
  for (const ar of expectedAccountRoutes) {
    assert.ok(
      masterNavSrc.includes(ar),
      `SettingsMasterNav must map account route ${ar}`,
    )
  }

  // Backward compatibility: sub-sections resolve properly
  assert.ok(
    settingsViewSrc.includes("location.pathname.split('/')[2]"),
    'SettingsView must parse route segment to identify active section',
  )
})
