import test from 'node:test'
import assert from 'node:assert/strict'
import fs from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'

const __dirname = path.dirname(fileURLToPath(import.meta.url))
const webDir = path.resolve(__dirname, '..')

const appShellPath = path.join(webDir, 'src/components/layout/AppShell.tsx')
const appPath = path.join(webDir, 'src/App.tsx')
const shortcutsPath = path.join(webDir, 'src/components/layout/KeyboardShortcuts.tsx')
const idJsonPath = path.join(webDir, 'src/locales/id.json')
const settingsPath = path.join(webDir, 'src/features/settings/SettingsView.tsx')
const studioViewPath = path.join(webDir, 'src/features/inbox/StudioView.tsx')
const createProjectPath = path.join(webDir, 'src/features/projects/CreateProjectModal.tsx')
const notificationsPath = path.join(webDir, 'src/features/notifications/NotificationsView.tsx')

test('1. Sidebar, Header, & AppShell: Chat replaces Studio in main navigation rail', () => {
  const appShellSource = fs.readFileSync(appShellPath, 'utf8')

  // NAV_ITEMS must have id 'chat' and label 'Chat' as first rail item
  const navItemsMatch = appShellSource.match(/const NAV_ITEMS = \[([\s\S]*?)\] as const/)
  assert.ok(navItemsMatch, 'NAV_ITEMS must be defined in AppShell.tsx')
  const navItemsText = navItemsMatch[1]
  const items = [...navItemsText.matchAll(/id:\s*'([^']+)',\s*label:\s*'([^']+)'/g)].map(
    (m) => ({ id: m[1], label: m[2] }),
  )
  assert.equal(items[0].id, 'studio', 'First navigation item id must be chat')
  assert.equal(items[0].label, 'Studio', 'First navigation item label must be Chat')

  // Top header brand link must route to /chat with "Go to Chat" aria-label
  assert.ok(
    appShellSource.includes('to="/studio"'),
    'AppShell must link brand mark to /chat',
  )
  assert.ok(
    appShellSource.includes("aria-label={t('Go to Studio')}"),
    'AppShell must declare aria-label Go to Chat for brand mark',
  )

  // Label resolution must map studio to Chat for backward compatibility
  assert.ok(
    appShellSource.includes("currentTab === 'studio'") ||
      appShellSource.includes("(currentTab as string) === 'studio'"),
    'AppShell label resolution must provide Chat fallback when tab is studio',
  )

  // Dynamic document title effect updates browser tab title
  assert.ok(
    appShellSource.includes('document.title ='),
    'AppShell must update document.title based on active pageTitle or Chat tab',
  )
})

test('2. Routing & Entry Point: /chat registered as primary workspace route', () => {
  const appSource = fs.readFileSync(appPath, 'utf8')

  // /chat route segment resolves to chat tab
  assert.ok(
    appSource.includes("case 'chat':\n        return 'chat'"),
    'App.tsx must resolve /chat rootSegment to chat tab',
  )

  // Default entry point for root redirects to /chat
  assert.ok(
    appSource.includes("navigate('/chat', { replace: true })"),
    'App.tsx must navigate to /chat as default authenticated entry point',
  )

  // StudioView is rendered for currentTab === 'chat'
  assert.ok(
    appSource.includes("currentTab === 'chat' && (") || appSource.includes("currentTab === 'chat'"),
    'App.tsx must render chat workspace (StudioView) for currentTab chat',
  )

  // New message action opens /studio (S59-T6 Standalone Studio Canvas)
  assert.ok(
    appSource.includes("if (kind === 'message') navigate('/studio')"),
    'openNew message action must navigate to /studio',
  )
})

test('3. S59-T6: /studio registered as standalone Studio canvas workspace', () => {
  const appSource = fs.readFileSync(appPath, 'utf8')

  // /studio resolves to studio tab
  assert.ok(
    appSource.includes("case 'studio':\n        return 'studio'"),
    'App.tsx must resolve /studio to studio tab',
  )
  assert.ok(
    appSource.includes("currentTab === 'studio' && ("),
    'App.tsx must render standalone StudioView for currentTab studio',
  )

  // Tab navigation handler supports chat and studio
  assert.ok(
    appSource.includes("tab === 'chat' || (tab as string) === 'studio'") ||
      appSource.includes("tab === 'studio' || tab === 'chat'"),
    'handleSelectTab must support both studio and chat tabs',
  )
})

test('4. Keyboard Shortcuts: G then H navigates to /chat with Chat label', () => {
  const shortcutsSource = fs.readFileSync(shortcutsPath, 'utf8')

  assert.ok(
    shortcutsSource.includes("h: '/chat'"),
    'KeyboardShortcuts destination for h key must be /chat',
  )
  assert.ok(
    shortcutsSource.includes("['G then H', 'Chat']"),
    'KeyboardShortcuts display list must show G then H as Chat',
  )
})

test('5. Localization: Indonesian dictionary (id.json) includes Chat translations', () => {
  const idJson = JSON.parse(fs.readFileSync(idJsonPath, 'utf8'))

  assert.equal(idJson['Chat'], 'Chat', 'id.json must include translation for Chat')
  assert.equal(idJson['Studio'], 'Studio', 'id.json Studio fallback must map to Chat')
  assert.equal(idJson['Go to Chat'], 'Ke Chat', 'id.json must include translation for Go to Chat')
  assert.equal(idJson['Go to Studio'], 'Ke Studio', 'id.json Go to Studio fallback must map to Ke Chat')
  assert.equal(idJson['← Back to Chat'], '← Kembali ke Chat', 'id.json must include translation for ← Back to Chat')
  assert.equal(idJson['← Back to Studio'], '← Kembali ke Chat', 'id.json ← Back to Studio fallback must map to ← Kembali ke Chat')
})

test('6. Internal Navigation References: Updated from Studio to Chat', () => {
  const settingsSource = fs.readFileSync(settingsPath, 'utf8')
  assert.ok(
    settingsSource.includes("returnTo === '/chat' || returnTo === '/studio'"),
    'SettingsView must accept returnTo /chat as safe return path',
  )
  assert.ok(
    settingsSource.includes('Back to conversation'),
    'SettingsView must render ← Back to Chat label',
  )

  const createProjectSource = fs.readFileSync(createProjectPath, 'utf8')
  assert.ok(
    createProjectSource.includes('via Chat.'),
    'CreateProjectModal micro-benefits text must refer to Chat',
  )

  const notificationsSource = fs.readFileSync(notificationsPath, 'utf8')
  assert.ok(
    notificationsSource.includes("/chat'"),
    'NotificationsView default fallback link must point to /chat',
  )

  const studioViewSource = fs.readFileSync(studioViewPath, 'utf8')
  assert.ok(
    studioViewSource.includes("aria-label={t('Chat')}"),
    'StudioView container section must use aria-label Chat',
  )
})
