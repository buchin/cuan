import test from 'node:test'
import assert from 'node:assert/strict'
import fs from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'
import { execSync } from 'node:child_process'

const __filename = fileURLToPath(import.meta.url)
const __dirname = path.dirname(__filename)

const repoRoot = path.resolve(__dirname, '..', '..')
const webSrcRoot = path.resolve(__dirname, '..', 'src')

const legacyCssPath = path.resolve(webSrcRoot, 'features', 'inbox', 'chat-whatsapp.css')
const calmCssPath = path.resolve(webSrcRoot, 'features', 'inbox', 'chat-calm.css')
const inboxViewPath = path.resolve(webSrcRoot, 'features', 'inbox', 'InboxView.tsx')
const studioViewPath = path.resolve(webSrcRoot, 'features', 'inbox', 'StudioView.tsx')
const inboxSubPanelPath = path.resolve(webSrcRoot, 'features', 'inbox', 'InboxSubPanel.tsx')
const appPath = path.resolve(webSrcRoot, 'App.tsx')
const appShellPath = path.resolve(webSrcRoot, 'components', 'layout', 'AppShell.tsx')
const idLocalePath = path.resolve(webSrcRoot, 'locales', 'id.json')
const enLocalePath = path.resolve(webSrcRoot, 'locales', 'en.json')

test('1. S59-T2: Total elimination of chat-whatsapp.css and legacy WhatsApp styling', () => {
  // chat-whatsapp.css must be deleted
  assert.equal(
    fs.existsSync(legacyCssPath),
    false,
    'chat-whatsapp.css must be completely removed from the codebase'
  )

  // chat-calm.css must exist
  assert.equal(
    fs.existsSync(calmCssPath),
    true,
    'chat-calm.css must exist as the calm replatforming stylesheet'
  )

  // Zero occurrences of chat-whatsapp in web/src
  const grepCheck = execSync(`grep -rn "chat-whatsapp" web/src || true`, {
    cwd: repoRoot,
    encoding: 'utf8',
  }).trim()
  assert.equal(
    grepCheck,
    '',
    `Found lingering references to chat-whatsapp in web/src:\n${grepCheck}`
  )

  const calmCssContent = fs.readFileSync(calmCssPath, 'utf8')

  // Zero old WhatsApp green colors
  const forbiddenGreens = ['#d9fdd3', '#005c4b', '#008069', '#00a884']
  for (const green of forbiddenGreens) {
    assert.equal(
      calmCssContent.toLowerCase().includes(green.toLowerCase()),
      false,
      `chat-calm.css must not contain WhatsApp green color ${green}`
    )
  }

  // Zero clip-path property declarations for bubble tails
  assert.equal(
    /clip-path\s*:/i.test(calmCssContent),
    false,
    'chat-calm.css must not declare clip-path property for bubble tails'
  )

  // Zero dot wallpaper pattern
  assert.equal(
    calmCssContent.includes('radial-gradient'),
    false,
    'chat-calm.css must not contain wallpaper dot/radial patterns'
  )
})

test('2. S59-T2: Calm Monochromatic Chat Stream & Desktop 65-80ch Constraints', () => {
  const calmCssContent = fs.readFileSync(calmCssPath, 'utf8')
  const inboxViewContent = fs.readFileSync(inboxViewPath, 'utf8')

  // Calm CSS defines incoming and outgoing bubble tokens and neutral surfaces
  assert.ok(
    calmCssContent.includes('.calm-incoming') && calmCssContent.includes('.calm-outgoing'),
    'chat-calm.css must define .calm-incoming and .calm-outgoing styles'
  )

  // Symmetrical rounded corners (no asymmetric tail)
  assert.ok(
    calmCssContent.includes('border-radius: 16px'),
    'chat-calm.css must specify symmetrical rounded bubble borders (16px)'
  )

  // Desktop Column Constraint 65-80ch in InboxView
  assert.ok(
    inboxViewContent.includes('max-w-[75ch]'),
    'InboxView conversation stream and composer must enforce 65-80ch desktop constraint (max-w-[75ch])'
  )
})

test('3. S59-T2: 2-Column Desktop Architecture & Fusion of Studio + Inbox', () => {
  const inboxSubPanelContent = fs.readFileSync(inboxSubPanelPath, 'utf8')
  const inboxViewContent = fs.readFileSync(inboxViewPath, 'utf8')
  const studioViewContent = fs.readFileSync(studioViewPath, 'utf8')

  // Quick entry bar in SubPanel
  assert.ok(
    inboxSubPanelContent.includes('data-testid="new-goal-quick-bar"'),
    'InboxSubPanel must contain quick entry bar with data-testid="new-goal-quick-bar"'
  )

  // Global Cmd+K / Ctrl+K keyboard shortcut listener
  assert.ok(
    inboxSubPanelContent.includes("e.key.toLowerCase() === 'k'") &&
      inboxSubPanelContent.includes('(e.metaKey || e.ctrlKey)'),
    'InboxSubPanel must register global Cmd+K / Ctrl+K keyboard shortcut to launch studio'
  )

  // Uses HairlineList and StatusDot primitives
  assert.ok(
    inboxSubPanelContent.includes('HairlineList'),
    'InboxSubPanel must use HairlineList layout primitive for thread list'
  )
  assert.ok(
    inboxSubPanelContent.includes('StatusDot'),
    'InboxSubPanel must use StatusDot micro-indicators'
  )

  // Right column: Dynamic Workspace Canvas
  assert.ok(
    inboxViewContent.includes('data-testid="studio-dynamic-canvas"'),
    'InboxView must render dynamic Studio Canvas with data-testid="studio-dynamic-canvas"'
  )
  assert.ok(
    inboxViewContent.includes('data-testid="header-new-studio-btn"'),
    'InboxView header must include [+ Ide Baru] button with data-testid="header-new-studio-btn"'
  )

  // StudioView accepts onConversationCreated callback and autoFocus prop
  assert.ok(
    studioViewContent.includes('onConversationCreated'),
    'StudioView must accept onConversationCreated callback prop'
  )
  assert.ok(
    studioViewContent.includes('autoFocus'),
    'StudioView must accept autoFocus prop for instant braindump focus'
  )

  // Smooth transition from Studio submission to live agent chat without redirect
  assert.ok(
    inboxViewContent.includes('handleStudioConversationCreated') ||
      inboxViewContent.includes("setCanvasMode('chat')"),
    'InboxView must smoothly switch canvas mode from studio to chat upon conversation creation'
  )
})

test('4. S59-T2: Mobile (Things 3 Inspired) Studio Sheet & Magic Plus FAB', () => {
  const inboxViewContent = fs.readFileSync(inboxViewPath, 'utf8')

  // Magic Plus (+) FAB
  assert.ok(
    inboxViewContent.includes('data-testid="magic-plus-fab"'),
    'InboxView must render Magic Plus FAB on mobile with data-testid="magic-plus-fab"'
  )

  // Slide-Up Studio Sheet
  assert.ok(
    inboxViewContent.includes('data-testid="studio-slide-up-sheet"'),
    'InboxView must render Slide-Up Studio Sheet on mobile with data-testid="studio-slide-up-sheet"'
  )

  // FAB hidden on desktop viewports
  assert.ok(
    inboxViewContent.includes('min-[1200px]:hidden') && inboxViewContent.includes('magic-plus-fab'),
    'Magic Plus FAB must be hidden on desktop (min-[1200px]:hidden) to keep desktop 2-column clean'
  )
})

test('5. S59-T2: Routing & Backward Compatibility for /chat, /inbox, /inbox/:id', () => {
  const appContent = fs.readFileSync(appPath, 'utf8')
  const appShellContent = fs.readFileSync(appShellPath, 'utf8')

  // App.tsx renders InboxView for both chat and inbox tabs
  assert.ok(
    appContent.includes("currentTab === 'chat' && (\n            <InboxView") &&
      appContent.includes("currentTab === 'inbox' && (\n            <InboxView"),
    'App.tsx must render unified workspace InboxView for both inbox and chat routes'
  )

  // AppShell.tsx renders conversation-sidebar for both /inbox and /chat
  assert.ok(
    appShellContent.includes("currentTab === 'inbox' || currentTab === 'chat'"),
    'AppShell.tsx desktop sidebar must activate for both inbox and chat tabs'
  )
})

test('6. S59-T2: Localization Completeness (id.json & en.json)', () => {
  assert.ok(fs.existsSync(idLocalePath), 'id.json must exist')
  assert.ok(fs.existsSync(enLocalePath), 'en.json must exist')

  const idLocale = JSON.parse(fs.readFileSync(idLocalePath, 'utf8'))
  const enLocale = JSON.parse(fs.readFileSync(enLocalePath, 'utf8'))

  // Studio / Inbox fusion translation keys
  assert.equal(idLocale['Ide / Tugas Baru'], 'Ide / Tugas Baru')
  assert.equal(idLocale['Ide Baru'], 'Ide Baru')
  assert.equal(idLocale['Ide / Tugas Baru (Cmd+K)'], 'Ide / Tugas Baru (Cmd+K)')
  assert.equal(idLocale['Ide / Tugas Baru (Quick Dump)'], 'Ide / Tugas Baru (Quick Dump)')

  assert.equal(enLocale['Ide / Tugas Baru'], 'New Idea / Task')
  assert.equal(enLocale['Ide Baru'], 'New Idea')
  assert.equal(enLocale['Ide / Tugas Baru (Cmd+K)'], 'New Idea / Task (Cmd+K)')
  assert.equal(enLocale['Ide / Tugas Baru (Quick Dump)'], 'New Idea / Task (Quick Dump)')
})
