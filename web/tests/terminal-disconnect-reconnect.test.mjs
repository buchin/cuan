import test from 'node:test'
import assert from 'node:assert/strict'
import fs from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'

const __filename = fileURLToPath(import.meta.url)
const __dirname = path.dirname(__filename)
const webDir = path.resolve(__dirname, '..')

const taskTerminalPath = path.join(webDir, 'src/features/tasks/TaskTerminal.tsx')
const idLocalePath = path.join(webDir, 'src/locales/id.json')
const enLocalePath = path.join(webDir, 'src/locales/en.json')
const landingPagePath = path.join(webDir, 'src/features/landing/LandingPage.tsx')

test('1. UI Penanganan Disconnect Terminal: Banner Ramah & Eliminasi Pesan Error Merah', () => {
  assert.ok(fs.existsSync(taskTerminalPath), 'TaskTerminal.tsx must exist')
  const content = fs.readFileSync(taskTerminalPath, 'utf8')

  // Pastikan pesan error merah statis 'Could not connect to this terminal' digantikan dan ditekan
  assert.ok(
    content.includes("!error.includes('Could not connect to this terminal')"),
    'TaskTerminal must suppress legacy static red error for terminal connection',
  )
  assert.ok(
    content.includes("!error.includes('not restarted')"),
    'TaskTerminal must suppress "The task was not restarted" text from red alert',
  )

  // Banner pemulihan ramah saat socket disconnect atau reconnecting
  assert.ok(
    content.includes('isDisconnectedOrRecovering'),
    'TaskTerminal must define isDisconnectedOrRecovering state',
  )
  assert.ok(
    content.includes('showDisconnectBanner'),
    'TaskTerminal must define showDisconnectBanner',
  )
  assert.ok(
    content.includes("t('Terminal disconnected. The task is still running in the background.')"),
    'TaskTerminal must render friendly background execution reassurance message',
  )

  // Desain banner yang ramah (Things 3 / Bear Notes zero cognitive fatigue)
  assert.ok(
    content.includes('border-amber-500/20 bg-amber-500/10'),
    'Banner must use low-contrast calm amber styling matching design guidelines',
  )
  assert.ok(
    content.includes('WifiOff'),
    'Banner must render WifiOff visual indicator',
  )
})

test('2. Tombol Aksi "Sambungkan Ulang" & Reconnection Auto-Retry Bertahap', () => {
  const content = fs.readFileSync(taskTerminalPath, 'utf8')

  // Tombol aksi Sambungkan Ulang
  assert.ok(
    content.includes("language === 'id' ? 'Sambungkan Ulang' : t('Reconnect')"),
    'Banner must provide "Sambungkan Ulang" (id) and "Reconnect" action button',
  )
  assert.ok(
    content.includes('onClick={handleReconnect}'),
    'Reconnect button must trigger handleReconnect callback',
  )
  assert.ok(
    content.includes("connection === 'connecting' ? 'animate-spin' : ''"),
    'Reconnect button must show spinning state while reconnecting',
  )

  // Auto-retry bertahap pada visibilitychange, focus, dan online
  assert.ok(
    content.includes("document.addEventListener('visibilitychange', handleActive)"),
    'TaskTerminal must listen to visibilitychange for mobile screen unlock / tab reactivation',
  )
  assert.ok(
    content.includes("window.addEventListener('focus', handleActive)"),
    'TaskTerminal must listen to window focus event',
  )
  assert.ok(
    content.includes("window.addEventListener('online', handleActive)"),
    'TaskTerminal must listen to window online event',
  )
})

test('3. Pengambilan State Terakhir & Snapshot Log Terminal Tanpa Reload Halaman', () => {
  const content = fs.readFileSync(taskTerminalPath, 'utf8')

  // handleReconnect mengambil snapshot log terbaru tanpa reload
  assert.ok(
    content.includes('tasksApi.logs(taskId, attempt.id, lastId)'),
    'handleReconnect must fetch latest logs snapshot from server',
  )
  assert.ok(
    content.includes('onRefresh()'),
    'handleReconnect must refresh execution status without full page reload',
  )
  assert.ok(
    !content.includes('window.location.reload()'),
    'TaskTerminal must never perform full window reload during reconnection',
  )
  assert.ok(
    content.includes('activeTerm.write(nextLogs.map'),
    'Latest logs snapshot must be appended to active terminal buffer',
  )
})

test('4. Penanganan Status Socket Closed & Error', () => {
  const content = fs.readFileSync(taskTerminalPath, 'utf8')

  // ws.onerror mengatur status disconnected tanpa error merah
  assert.ok(
    content.includes("ws.onerror = () => {"),
    'TaskTerminal must register ws.onerror handler',
  )
  assert.ok(
    content.includes("setConnection('disconnected')"),
    'ws.onerror and ws.onclose must cleanly set disconnected state',
  )

  // ws.onclose menjadwalkan auto-retry jika closed tidak bersih (event.code !== 1000)
  assert.ok(
    content.includes('event.code !== 1000'),
    'ws.onclose must detect unexpected socket drop and trigger auto-retry',
  )
})

test('5. Kamus Lokalisasi Indonesia & Landing Page Sinkron', () => {
  const idLocale = JSON.parse(fs.readFileSync(idLocalePath, 'utf8'))
  const enLocale = JSON.parse(fs.readFileSync(enLocalePath, 'utf8'))
  const landingPageContent = fs.readFileSync(landingPagePath, 'utf8')

  // Translation keys
  assert.equal(
    idLocale['Terminal disconnected. The task is still running in the background.'],
    'Koneksi terminal terputus. Proses di server tetap berjalan aman.',
    'id.json must map terminal disconnect to friendly server safety message',
  )
  assert.equal(
    idLocale['Sambungkan Ulang'],
    'Sambungkan Ulang',
    'id.json must include "Sambungkan Ulang"',
  )
  assert.equal(
    idLocale['Auto-retry when tab becomes active'],
    'Mencoba ulang otomatis saat tab kembali aktif',
    'id.json must include auto-retry translation',
  )
  assert.ok(
    enLocale['Terminal disconnected. The task is still running in the background.'],
    'en.json must include disconnect message',
  )

  // Landing Page capability reflection
  assert.ok(
    landingPageContent.includes('Live Terminal Resilience & Mobile Auto-Reconnect'),
    'LandingPage.tsx must reflect live terminal resilience & auto-reconnect capability',
  )
})
