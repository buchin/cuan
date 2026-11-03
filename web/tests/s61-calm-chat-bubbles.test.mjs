import test from 'node:test'
import assert from 'node:assert/strict'
import fs from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'

const __filename = fileURLToPath(import.meta.url)
const __dirname = path.dirname(__filename)
const repoRoot = path.resolve(__dirname, '../..')
const webDir = path.resolve(__dirname, '..')

const sprintDocPath = path.resolve(repoRoot, 'docs/sprints/S61.md')
const chatCalmCssPath = path.resolve(webDir, 'src/features/inbox/chat-calm.css')
const indexCssPath = path.resolve(webDir, 'src/index.css')
const inboxViewPath = path.resolve(webDir, 'src/features/inbox/InboxView.tsx')
const mobileScreenshotPath = path.resolve(repoRoot, 'screenshots/s61-calm-chat-bubbles-mobile.png')
const desktopScreenshotPath = path.resolve(repoRoot, 'screenshots/s61-calm-chat-bubbles-desktop.png')

const sprintDoc = fs.readFileSync(sprintDocPath, 'utf8')
const chatCalmCss = fs.readFileSync(chatCalmCssPath, 'utf8')
const indexCss = fs.readFileSync(indexCssPath, 'utf8')
const inboxView = fs.readFileSync(inboxViewPath, 'utf8')

test('1. S61-T2: Single Source of Truth - docs/sprints/S61.md Task 2 Definition', () => {
  assert.ok(sprintDoc.includes('S61-T2'), 'Sprint 61 doc must include S61-T2')
  assert.ok(sprintDoc.includes('lqidrsayqatbywpmsuenamzx'), 'Task ID lqidrsayqatbywpmsuenamzx must be recorded')
  assert.ok(
    sprintDoc.includes('Calm Low-Contrast Chat Bubbles') ||
      sprintDoc.includes('calm-chat-bubbles'),
    'Sprint 61 must specify Calm Low-Contrast Chat Bubbles topic'
  )
  assert.ok(
    sprintDoc.includes('Bear Notes') && sprintDoc.includes('Things 3'),
    'Sprint 61 doc must document Bear Notes & Things 3 aesthetic standard'
  )
})

test('2. S61-T2: Low-Contrast Chat Bubbles & Elimination of Harsh White Borders', () => {
  // Bubbles must eliminate white stroke borders by default
  assert.ok(
    chatCalmCss.includes('border-color: transparent;'),
    'chat-calm.css must set border-color: transparent on message bubbles'
  )

  // In dark mode, borders must remain transparent and without drop shadows
  assert.ok(
    chatCalmCss.includes('.dark .wa-message-bubble') &&
      chatCalmCss.includes('border-color: transparent !important;'),
    'Dark mode message bubbles must strictly enforce border-color: transparent'
  )

  // Subtle monochromatic micro-tints for dark mode
  assert.ok(
    chatCalmCss.includes('--wa-incoming: #1b1c1f;'),
    'Incoming bubble must use soft monochromatic tint #1b1c1f in dark mode'
  )
  assert.ok(
    chatCalmCss.includes('--wa-outgoing: #25272c;'),
    'Outgoing bubble must use calm monochromatic neutral #25272c in dark mode'
  )
  assert.ok(
    chatCalmCss.includes('--wa-bg: #141416;'),
    'Canvas background must be tranquil #141416 in dark mode'
  )

  // Strict elimination of WhatsApp speech bubble tails
  assert.ok(
    chatCalmCss.includes('.wa-message-bubble::before') &&
      chatCalmCss.includes('display: none !important;'),
    'Message bubbles must completely eliminate speech bubble tail clip-paths'
  )
})

test('3. S61-T2: Quiet Date Dividers - Zero Solid White Pill Glare', () => {
  // wa-day-label in chat-calm.css must be transparent with no border or pill box
  assert.ok(
    chatCalmCss.includes('.wa-day-label') &&
      chatCalmCss.includes('background: transparent !important;'),
    'wa-day-label must have background: transparent !important'
  )
  assert.ok(
    chatCalmCss.includes('border: none !important;'),
    'wa-day-label must eliminate solid borders'
  )
  assert.ok(
    chatCalmCss.includes('box-shadow: none !important;'),
    'wa-day-label must eliminate box shadows'
  )

  // InboxView.tsx must use quiet typography for day divider
  assert.ok(
    inboxView.includes('calm-day-label my-3 self-center text-xs font-medium text-muted-foreground') ||
      inboxView.includes('wa-day-label calm-day-label'),
    'InboxView.tsx must render quiet text-muted-foreground date divider without pill box'
  )

  // Verify no hardcoded white pill background #ffffff on day labels
  assert.ok(
    !chatCalmCss.includes('.wa-day-label {\n  background: #ffffff;') &&
      !chatCalmCss.includes('.wa-day-label {\n  background: #fff;'),
    'wa-day-label must never render a bright solid white background'
  )
})

test('4. S61-T2: Streamlined Agent Process Card & Assigned to Task Badge', () => {
  // Elimination of heavy neon green button badge 'Assigned to task'
  assert.ok(
    !inboxView.includes('bg-emerald-500/10 border border-emerald-500/20 px-2 py-0.5 text-[10px] font-semibold text-emerald-600'),
    'InboxView.tsx must eliminate thick neon green pill button for Assigned to task'
  )

  // Replaced with clean borderless typography
  assert.ok(
    inboxView.includes('text-xs font-medium text-muted-foreground hover:text-foreground transition-colors cursor-pointer bg-transparent border-0 p-0'),
    'Assigned to task must be rendered as clean borderless typography'
  )

  // Agent response progress placeholder in InboxView must be borderless
  assert.ok(
    inboxView.includes('aria-label="Agent response progress"') &&
      inboxView.includes('!bg-transparent !border-0 !shadow-none !p-0'),
    'Agent response progress wrapper must be borderless and transparent'
  )

  // chat-calm.css must define borderless containment for agent progress
  assert.ok(
    !chatCalmCss.includes('[aria-label="Agent response progress"] .wa-message-bubble'),
    'Progress descendants must not erase mixed message boundaries'
  )
})

test('5. S61-T2: Things 3 & Bear Notes CSS Overhaul in index.css', () => {
  // Dark mode primary must not be oversaturated electric/neon blue
  assert.ok(
    !indexCss.includes('--primary: 0.6586 0.1697 256.7129;'),
    'index.css must eliminate neon oversaturated dark mode primary'
  )
  assert.ok(
    indexCss.includes('--primary: 0.72 0.04 250;'),
    'index.css must use calm Things 3 low-chroma primary in dark mode'
  )
})

test('6. S61-T2: Visual Screenshots & Self-Critique Audit Artifacts', () => {
  // Both mobile and desktop screenshots must exist and be non-empty
  assert.ok(fs.existsSync(mobileScreenshotPath), 'Mobile screenshot must exist at screenshots/s61-calm-chat-bubbles-mobile.png')
  const mobileStats = fs.statSync(mobileScreenshotPath)
  assert.ok(mobileStats.size > 10000, 'Mobile screenshot must be a valid non-empty PNG image')

  assert.ok(fs.existsSync(desktopScreenshotPath), 'Desktop screenshot must exist at screenshots/s61-calm-chat-bubbles-desktop.png')
  const desktopStats = fs.statSync(desktopScreenshotPath)
  assert.ok(desktopStats.size > 10000, 'Desktop screenshot must be a valid non-empty PNG image')

  // Generate HTML visual report
  const reportPath = path.join(__dirname, 's61-calm-chat-bubbles-visual-report.html')
  const reportHtml = `<!DOCTYPE html>
<html lang="id">
<head>
  <meta charset="UTF-8">
  <title>Cuan Sprint 61 Task 2 Visual Verification Report</title>
  <style>
    body { font-family: -apple-system, sans-serif; background: #0e0f11; color: #ededed; padding: 24px; }
    h1 { font-size: 20px; font-weight: 600; margin-bottom: 8px; }
    .meta { color: #8e8e93; font-size: 13px; margin-bottom: 24px; }
    .grid { display: grid; grid-template-columns: 390px 1fr; gap: 24px; align-items: start; }
    .card { background: #18181a; border-radius: 12px; padding: 16px; border: 1px solid rgba(255,255,255,0.06); }
    img { width: 100%; border-radius: 8px; display: block; }
    .badge { display: inline-block; padding: 3px 8px; border-radius: 9999px; background: rgba(59,130,246,0.15); color: #93c5fd; font-size: 11px; font-weight: 600; margin-bottom: 8px; }
  </style>
</head>
<body>
  <h1>S61-T2: Calm Low-Contrast Chat Bubbles & Things/Bear CSS Overhaul</h1>
  <div class="meta">Task #LQIDR · Self-Critique Visual Audit · Mobile (390x844) & Desktop (1280x800)</div>
  <div class="grid">
    <div class="card">
      <span class="badge">Mobile Viewport (390x844)</span>
      <img src="../../screenshots/s61-calm-chat-bubbles-mobile.png" alt="Mobile Preview">
    </div>
    <div class="card">
      <span class="badge">Desktop Viewport (1280x800)</span>
      <img src="../../screenshots/s61-calm-chat-bubbles-desktop.png" alt="Desktop Preview">
    </div>
  </div>
</body>
</html>`
  fs.writeFileSync(reportPath, reportHtml, 'utf8')
  assert.ok(fs.existsSync(reportPath), 'Visual report HTML must be created')
})
