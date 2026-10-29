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
    masterNavSrc.includes("id: 'agents'"),
    'Agents setting must be within AI Intelligence & Memory',
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
    masterNavSrc.includes("id: 'contacts'"),
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
    masterNavSrc.includes('bg-accent text-primary'),
    'Active navigation item must feature bg-accent and text-primary styling',
  )

  // Verify Touch Accessibility (min-h-11 for WCAG 2.1 AA)
  assert.ok(
    masterNavSrc.includes('min-h-11'),
    'Navigation items must enforce min-h-11 touch target',
  )
})

test('3. Layout: Two-Column Master-Detail in SettingsView.tsx', () => {
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

  // Left master sidebar with w-72 / w-80 and border-r
  assert.ok(
    settingsViewSrc.includes('w-full md:w-72 lg:w-80 shrink-0 border-b md:border-b-0 md:border-r border-border') ||
      (settingsViewSrc.includes('md:w-72') || settingsViewSrc.includes('md:w-80')) &&
      settingsViewSrc.includes('border-r border-border'),
    'Master sidebar must have consistent w-72/w-80 width and border-r border on desktop',
  )

  // Right content detail area with flex-1 min-w-0 max-w-4xl
  assert.ok(
    settingsViewSrc.includes('flex-1 min-w-0') &&
      settingsViewSrc.includes('max-w-4xl'),
    'Content detail area must feature flex-1 min-w-0 and max-w-4xl max width',
  )

  // Overflow-y protection to prevent horizontal layout blowout
  assert.ok(
    settingsViewSrc.includes('overflow-y-auto'),
    'Containers must protect against layout clipping with overflow-y-auto',
  )
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
    settingsViewSrc.includes("hasExplicitSection ? 'hidden md:block' : 'block'"),
    'Sidebar must show on mobile root and hide when detail is active',
  )
  assert.ok(
    settingsViewSrc.includes("!hasExplicitSection ? 'hidden md:block' : 'block'"),
    'Detail view must hide on mobile root and display when section is selected',
  )

  // Mobile back button with ChevronLeft and ← Back to Settings
  assert.ok(
    settingsViewSrc.includes('ChevronLeft'),
    'SettingsView must import and render ChevronLeft icon for mobile back action',
  )
  assert.ok(
    settingsViewSrc.includes("t('← Back to Settings')"),
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
    settingsViewSrc.includes("returnTo === '/chat' || returnTo === '/studio'"),
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

  // Subtitles
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

  // Mobile Back Button
  assert.equal(
    idLocale['← Back to Settings'],
    '← Kembali ke Pengaturan',
    'Mobile back button label must be translated',
  )
})
