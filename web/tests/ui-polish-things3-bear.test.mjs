import test from 'node:test'
import assert from 'node:assert/strict'
import fs from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'

const __dirname = path.dirname(fileURLToPath(import.meta.url))
const webDir = path.resolve(__dirname, '..')

const authGatePath = path.join(webDir, 'src/components/auth/AuthGate.tsx')
const loginScreenPath = path.join(webDir, 'src/components/auth/LoginScreen.tsx')
const inboxSubPanelPath = path.join(webDir, 'src/features/inbox/InboxSubPanel.tsx')
const notificationsPopoverPath = path.join(webDir, 'src/features/notifications/NotificationsPopover.tsx')
const indexCssPath = path.join(webDir, 'src/index.css')

test('1. Splash Screen & Login: warm monochrome / deep graphite (#121214) and high-contrast monochrome buttons', () => {
  const authGateSrc = fs.readFileSync(authGatePath, 'utf8')
  const loginSrc = fs.readFileSync(loginScreenPath, 'utf8')
  const indexCss = fs.readFileSync(indexCssPath, 'utf8')

  // AuthGate splash screen uses calm CuanMark brand mark and eliminates spinning blue RefreshCw
  assert.ok(
    authGateSrc.includes('<CuanMark') && authGateSrc.includes('animate-pulse'),
    'AuthGate splash screen must render CuanMark brand mark with calm pulse'
  )
  assert.ok(
    !authGateSrc.includes('<RefreshCw'),
    'AuthGate must eliminate spinning RefreshCw spinner'
  )
  assert.ok(
    authGateSrc.includes('#121214'),
    'AuthGate must use deep graphite #121214 in dark mode'
  )

  // LoginScreen uses deep graphite #121214 dark background and alabaster #fafafa light background
  assert.ok(
    loginSrc.includes('#121214') && loginSrc.includes('#fafafa'),
    'LoginScreen must use warm monochrome palette (#121214 in dark mode, #fafafa in light mode)'
  )

  // Primary action button uses high-contrast monochrome (bg-foreground text-background)
  assert.ok(
    loginSrc.includes('bg-foreground') && loginSrc.includes('text-background'),
    'LoginScreen primary submit button must use high-contrast monochrome styling'
  )

  // Brand mark uses high-contrast text-foreground
  assert.ok(
    loginSrc.includes('<CuanMark className="h-8 w-8 text-foreground" />'),
    'LoginScreen brand mark must use high-contrast monochrome text-foreground'
  )

  // index.css eliminates saturated blue light-mode primary
  assert.ok(
    !indexCss.includes('--primary: 0.5334 0.2049 258.8101;'),
    'index.css must eliminate saturated SaaS blue light-mode primary (0.5334 0.2049 258.8101)'
  )
  assert.ok(
    indexCss.includes('--background: 0.1831 0.0040 286;'),
    'index.css dark mode background must be deep graphite #121214 (0.1831 0.0040 286)'
  )
})

test('2. Inbox Room List: visual differentiation of Unread (bold + 6px micro-dot) vs Unresolved (open ring ○)', () => {
  const inboxSubPanelSrc = fs.readFileSync(inboxSubPanelPath, 'utf8')

  // Workflow status on left side: open ring (Circle) for unresolved, CheckCircle2 for resolved
  assert.ok(
    inboxSubPanelSrc.includes('data-testid="workflow-status-indicator"'),
    'ConversationItem must include workflow-status-indicator'
  )
  assert.ok(
    inboxSubPanelSrc.includes('<Circle') && inboxSubPanelSrc.includes('<CheckCircle2'),
    'ConversationItem must differentiate unresolved with Circle (○) and resolved with CheckCircle2 (✓)'
  )
  assert.ok(
    inboxSubPanelSrc.includes('setConversationResolved'),
    'Workflow status indicator allows toggling conversation resolution'
  )

  // Unread status on right side: bold text typography + 6px solid micro-dot
  assert.ok(
    inboxSubPanelSrc.includes('unread > 0 ? "font-bold text-foreground"'),
    'ConversationItem must render unread contact name in font-bold text-foreground'
  )
  assert.ok(
    inboxSubPanelSrc.includes('unread > 0 ? "font-bold text-foreground/90'),
    'ConversationItem must render unread message snippet in bold font weight'
  )
  assert.ok(
    inboxSubPanelSrc.includes('data-testid="unread-micro-dot"'),
    'ConversationItem must render unread micro-dot'
  )
  assert.ok(
    inboxSubPanelSrc.includes('h-1.5 w-1.5 rounded-full'),
    'Unread micro-dot must be exactly 6px (h-1.5 w-1.5 rounded-full)'
  )
  assert.ok(
    inboxSubPanelSrc.includes('archive.markRead(id, true'),
    'Opening contact must call archive.markRead to automatically clear unread status and micro-dot'
  )
})

test('3. Notification Icon: clean neutral-muted Bell with soft amber micro-dot badge', () => {
  const popoverSrc = fs.readFileSync(notificationsPopoverPath, 'utf8')

  // Neutral-muted Bell icon
  assert.ok(
    popoverSrc.includes('text-muted-foreground hover:text-foreground transition-colors'),
    'NotificationsPopover trigger button must use neutral-muted styling with smooth hover transition'
  )

  // Soft amber micro-dot badge
  assert.ok(
    popoverSrc.includes('data-testid="notification-unread-dot"'),
    'NotificationsPopover must render notification-unread-dot'
  )
  assert.ok(
    popoverSrc.includes('bg-amber-500/90 dark:bg-amber-400'),
    'NotificationsPopover unread dot must use soft amber color'
  )
  assert.ok(
    popoverSrc.includes('h-2 w-2 rounded-full'),
    'NotificationsPopover unread dot must be a subtle micro-dot (h-2 w-2 rounded-full)'
  )

  // Elimination of large neon number badge
  assert.ok(
    !popoverSrc.includes('bg-primary px-1 text-[10px] leading-4 text-primary-foreground'),
    'NotificationsPopover must eliminate the large primary neon number badge pill'
  )
})
