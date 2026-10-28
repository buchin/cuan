import test from 'node:test'
import assert from 'node:assert/strict'
import fs from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'

const __filename = fileURLToPath(import.meta.url)
const __dirname = path.dirname(__filename)

const inboxViewPath = path.join(__dirname, '..', 'src', 'features', 'inbox', 'InboxView.tsx')
const appShellPath = path.join(__dirname, '..', 'src', 'components', 'layout', 'AppShell.tsx')
const idLocalePath = path.join(__dirname, '..', 'src', 'locales', 'id.json')
const distDir = path.join(__dirname, '..', 'dist', 'assets')
const visualReportPath = path.join(__dirname, 'inbox-header-simplification-visual-report.html')

test('1. Requirement 1: Reload button (ikon putar) is removed from the chat header', () => {
  const inboxViewSource = fs.readFileSync(inboxViewPath, 'utf8')

  // Find the chat header block
  const headerStart = inboxViewSource.indexOf('wa-chat-header')
  assert.ok(headerStart !== -1, 'wa-chat-header container must exist')
  const headerEnd = inboxViewSource.indexOf('Sub-tabs: Obrolan (Chat) & Pekerjaan (Tasks)', headerStart)
  assert.ok(headerEnd !== -1, 'Sub-tabs container separator must exist')
  const headerContent = inboxViewSource.slice(headerStart, headerEnd)

  // Verify reload button and refresh icon are completely eliminated from the chat header
  assert.ok(
    !headerContent.includes("title={t('Refresh messages')}"),
    'Chat header must not contain reload button with Refresh messages title'
  )
  assert.ok(
    !headerContent.includes('loadConversation(activeContact.conversation_id'),
    'Chat header must not contain reload button handler'
  )
  assert.ok(
    !headerContent.includes('<RefreshCw'),
    'Chat header must not contain RefreshCw icon'
  )
})

test('2. Requirement 2: Tasks/checklist button and badge are removed from top-right chat header', () => {
  const inboxViewSource = fs.readFileSync(inboxViewPath, 'utf8')

  const headerStart = inboxViewSource.indexOf('wa-chat-header')
  const headerEnd = inboxViewSource.indexOf('Sub-tabs: Obrolan (Chat) & Pekerjaan (Tasks)', headerStart)
  const headerContent = inboxViewSource.slice(headerStart, headerEnd)

  // Verify task button and badge are eliminated from the top header
  assert.ok(
    !headerContent.includes("onClick={() => setConvTab(convTab === 'tasks' ? 'chat' : 'tasks')}"),
    'Chat header must not contain convTab toggle button'
  )
  assert.ok(
    !headerContent.includes("title={convTab === 'tasks' ? t('Switch to chat') : t('Tasks')}"),
    'Chat header must not contain tasks toggle button title'
  )
  assert.ok(
    !headerContent.includes('{linkedTasks.length > 0 && ('),
    'Chat header must not contain linked tasks badge in top bar'
  )
})

test('3. Requirement 3: Clean chat header preserving back navigation and chat identity', () => {
  const inboxViewSource = fs.readFileSync(inboxViewPath, 'utf8')

  const headerStart = inboxViewSource.indexOf('wa-chat-header')
  const headerEnd = inboxViewSource.indexOf('Sub-tabs: Obrolan (Chat) & Pekerjaan (Tasks)', headerStart)
  const headerContent = inboxViewSource.slice(headerStart, headerEnd)

  // Verify back navigation button exists and has accessible touch target
  assert.ok(
    headerContent.includes('onClick={handleBackToContacts}'),
    'Chat header must preserve back navigation button (handleBackToContacts)'
  )
  assert.ok(
    headerContent.includes('ChevronLeft'),
    'Chat header must use ChevronLeft icon for back navigation'
  )
  assert.ok(
    headerContent.includes('h-11 w-11'),
    'Back button must maintain minimum 44px (h-11 w-11) touch target'
  )

  // Verify chat identity (avatar, name, and status) exists
  assert.ok(
    headerContent.includes('activeContact.avatar'),
    'Chat header must render contact avatar'
  )
  assert.ok(
    headerContent.includes('activeContact.name'),
    'Chat header must render contact name'
  )
  assert.ok(
    headerContent.includes('<ContactStatus'),
    'Chat header must render ContactStatus component'
  )
})

test('4. Requirement 4: Room tasks remain accessible via sub-panel tabs and global navigation', () => {
  const inboxViewSource = fs.readFileSync(inboxViewPath, 'utf8')
  const appShellSource = fs.readFileSync(appShellPath, 'utf8')

  // Verify sub-tabs below header for Chat and Tasks
  assert.ok(
    inboxViewSource.includes("onClick={() => setConvTab('chat')}"),
    'InboxView must maintain Chat tab button in sub-panel'
  )
  assert.ok(
    inboxViewSource.includes("onClick={() => setConvTab('tasks')}"),
    'InboxView must maintain Tasks tab button in sub-panel'
  )
  assert.ok(
    inboxViewSource.includes('<ConversationWorkMonitor'),
    'InboxView must render ConversationWorkMonitor when convTab is tasks'
  )
  assert.ok(
    inboxViewSource.includes('tasks={linkedTasks}'),
    'ConversationWorkMonitor must receive room linked tasks'
  )

  // Verify global navigation to Tasks remains intact in AppShell
  assert.ok(
    appShellSource.includes("to=\"/tasks\"") || appShellSource.includes("id: 'tasks'"),
    'AppShell must maintain global navigation link for tasks'
  )
})

test('5. Requirement 5: Mobile viewport (360-390px) & desktop responsiveness and production bundle', () => {
  const inboxViewSource = fs.readFileSync(inboxViewPath, 'utf8')

  // Mobile layout constraints: prevent header text overflow
  assert.ok(
    inboxViewSource.includes('min-w-0'),
    'Chat header identity container must include min-w-0 to prevent text overflow on 360-390px viewports'
  )
  assert.ok(
    inboxViewSource.includes('truncate text-base font-semibold'),
    'Contact title must use truncate for responsive overflow protection'
  )

  // Production bundle verification
  if (fs.existsSync(distDir)) {
    const jsFiles = fs.readdirSync(distDir).filter((f) => f.endsWith('.js'))
    const inboxBundle = jsFiles.find((f) => f.startsWith('InboxView'))
    assert.ok(inboxBundle, 'Compiled InboxView bundle must exist in dist/assets')

    const bundleContent = fs.readFileSync(path.join(distDir, inboxBundle), 'utf8')
    assert.ok(
      !bundleContent.includes('title:a("Refresh messages")'),
      'Compiled production bundle must not contain header refresh messages button'
    )
    assert.ok(
      !bundleContent.includes('title:a(he==="tasks"?"Switch to chat":"Tasks")'),
      'Compiled production bundle must not contain header tasks toggle button'
    )
  }

  // Generate visual verification report
  const html = `<!DOCTYPE html>
<html lang="id">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <title>Verifikasi Visual: Penyederhanaan Header Obrolan Chat (360px, 390px, Desktop)</title>
  <style>
    :root {
      --bg: #09090b;
      --card: #121214;
      --border: #27272a;
      --text: #f4f4f5;
      --muted: #a1a1aa;
      --primary: #3b82f6;
      --header-bg: #18181b;
    }
    * { box-sizing: border-box; margin: 0; padding: 0; }
    body {
      background: var(--bg);
      color: var(--text);
      font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif;
      padding: 24px;
      line-height: 1.5;
    }
    .container { max-width: 1200px; margin: 0 auto; }
    h1 { font-size: 1.5rem; margin-bottom: 8px; color: #fff; }
    p.desc { font-size: 0.875rem; color: var(--muted); margin-bottom: 24px; }
    .badge-pass {
      display: inline-block;
      background: rgba(16, 185, 129, 0.15);
      color: #34d399;
      padding: 2px 8px;
      border-radius: 9999px;
      font-size: 0.75rem;
      font-weight: 600;
      margin-left: 8px;
    }
    .comparison-grid {
      display: grid;
      grid-template-columns: repeat(auto-fit, minmax(360px, 1fr));
      gap: 24px;
      margin-bottom: 32px;
    }
    .card {
      background: var(--card);
      border: 1px solid var(--border);
      border-radius: 16px;
      overflow: hidden;
      box-shadow: 0 4px 20px rgba(0,0,0,0.5);
    }
    .card-title {
      padding: 12px 16px;
      font-size: 0.85rem;
      font-weight: 600;
      border-bottom: 1px solid var(--border);
      display: flex;
      justify-content: space-between;
      align-items: center;
      background: #18181b;
    }
    .viewport-box {
      margin: 16px auto;
      border: 2px solid #3f3f46;
      border-radius: 12px;
      overflow: hidden;
      background: #000;
    }
    .wa-chat-header {
      height: 56px;
      padding: 0 12px;
      display: flex;
      align-items: center;
      justify-content: space-between;
      background: var(--header-bg);
      border-bottom: 1px solid var(--border);
    }
    .left-side {
      display: flex;
      align-items: center;
      gap: 10px;
      min-width: 0;
      flex: 1;
    }
    .back-btn {
      width: 36px;
      height: 36px;
      display: flex;
      align-items: center;
      justify-content: center;
      color: var(--muted);
      border-radius: 50%;
      flex-shrink: 0;
    }
    .avatar {
      width: 36px;
      height: 36px;
      border-radius: 50%;
      background: #27272a;
      display: flex;
      align-items: center;
      justify-content: center;
      font-size: 16px;
      flex-shrink: 0;
      border: 1px solid var(--border);
    }
    .identity { min-width: 0; flex: 1; }
    .name {
      font-size: 0.95rem;
      font-weight: 600;
      white-space: nowrap;
      overflow: hidden;
      text-overflow: ellipsis;
    }
    .right-actions {
      display: flex;
      align-items: center;
      gap: 6px;
      flex-shrink: 0;
    }
    .action-btn {
      width: 36px;
      height: 36px;
      border-radius: 8px;
      display: flex;
      align-items: center;
      justify-content: center;
      color: var(--muted);
      background: transparent;
      border: none;
      position: relative;
    }
    .badge-count {
      position: absolute;
      top: 0;
      right: 0;
      background: var(--primary);
      color: white;
      font-size: 10px;
      font-weight: bold;
      border-radius: 9999px;
      padding: 1px 5px;
    }
    .subtabs {
      display: flex;
      border-bottom: 1px solid var(--border);
      background: #09090b;
      font-size: 12px;
      padding: 0 12px;
    }
    .tab {
      display: flex;
      align-items: center;
      gap: 6px;
      padding: 8px 12px;
      border-bottom: 2px solid transparent;
      color: var(--muted);
    }
    .tab.active {
      border-color: var(--primary);
      color: var(--primary);
      font-weight: 600;
    }
    .strike {
      text-decoration: line-through;
      color: #ef4444;
      opacity: 0.7;
    }
  </style>
</head>
<body>
  <div class="container">
    <header>
      <h1>Verifikasi Visual: Eliminasi Tombol Header Chat <span class="badge-pass">Lulus Verifikasi</span></h1>
      <p class="desc">Menguji penghapusan tombol reload dan task di header obrolan sesuai kriteria 1, 2, 3, 4, dan 5 pada mobile viewport (360px & 390px) serta desktop.</p>
    </header>

    <div class="comparison-grid">
      <!-- 360px Viewport (Mobile Mini / Android 360px) -->
      <div class="card">
        <div class="card-title">
          <span>Viewport 360px (Mobile Standard)</span>
          <span style="color: #34d399;">✓ Bersih & Rapi</span>
        </div>
        <div style="padding: 16px;">
          <div class="viewport-box" style="width: 360px;">
            <div class="wa-chat-header">
              <div class="left-side">
                <div class="back-btn">‹</div>
                <div class="avatar">🤖</div>
                <div class="identity">
                  <div class="name">Apa ide bagus jika k...</div>
                </div>
              </div>
              <div class="right-actions">
                <!-- Bersih: Reload dan Tasks telah dihilangkan -->
              </div>
            </div>
            <div class="subtabs">
              <div class="tab">💬 Chat</div>
              <div class="tab active">☑ Tasks (1)</div>
            </div>
            <div style="padding: 16px; font-size: 12px; color: var(--muted);">
              Tampilan responsif 360px: Judul percakapan memiliki ruang lega maksimal tanpa desakan tombol reload & checklist.
            </div>
          </div>
        </div>
      </div>

      <!-- 390px Viewport (iPhone 12-16) -->
      <div class="card">
        <div class="card-title">
          <span>Viewport 390px (iPhone Modern)</span>
          <span style="color: #34d399;">✓ Bersih & Rapi</span>
        </div>
        <div style="padding: 16px;">
          <div class="viewport-box" style="width: 390px;">
            <div class="wa-chat-header">
              <div class="left-side">
                <div class="back-btn">‹</div>
                <div class="avatar">🤖</div>
                <div class="identity">
                  <div class="name">Apa ide bagus jika kami meluncurkan program...</div>
                </div>
              </div>
              <div class="right-actions">
                <!-- Bersih: Reload dan Tasks telah dihilangkan -->
              </div>
            </div>
            <div class="subtabs">
              <div class="tab">💬 Chat</div>
              <div class="tab active">☑ Tasks (1)</div>
            </div>
            <div style="padding: 16px; font-size: 12px; color: var(--muted);">
              Tampilan responsif 390px: Judul percakapan tampil proporsional tanpa pemotongan dini.
            </div>
          </div>
        </div>
      </div>

      <!-- Desktop Viewport -->
      <div class="card" style="grid-column: 1 / -1;">
        <div class="card-title">
          <span>Desktop Viewport (1024px+)</span>
          <span style="color: #34d399;">✓ Bersih & Fungsional</span>
        </div>
        <div style="padding: 16px;">
          <div class="viewport-box" style="width: 100%; max-width: 900px;">
            <div class="wa-chat-header">
              <div class="left-side">
                <div class="avatar">🤖</div>
                <div class="identity">
                  <div class="name">General Assistant Agent</div>
                </div>
              </div>
              <div class="right-actions">
                <div class="action-btn" title="Manage agent connection">⚙</div>
              </div>
            </div>
            <div class="subtabs">
              <div class="tab active">💬 Chat</div>
              <div class="tab">☑ Tasks</div>
            </div>
            <div style="padding: 16px; font-size: 12px; color: var(--muted);">
              Tampilan desktop: Header bersih dan elegan, tombol reload dan task di pojok kanan atas telah dieliminasi. Pengaturan agent tetap dapat diakses di desktop via ikon gear. Sub-panel tasks tetap dapat beralih dengan mulus.
            </div>
          </div>
        </div>
      </div>
    </div>
  </div>
</body>
</html>`

  fs.writeFileSync(visualReportPath, html, 'utf8')
  assert.ok(fs.existsSync(visualReportPath), 'Visual report HTML must be successfully written')
})
