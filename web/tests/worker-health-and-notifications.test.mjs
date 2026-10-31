import test from 'node:test'
import assert from 'node:assert/strict'
import fs from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'

const __filename = fileURLToPath(import.meta.url)
const __dirname = path.dirname(__filename)

const notificationsHelperPath = path.resolve(__dirname, '..', 'src', 'features', 'inbox', 'task-notifications.ts')
const inboxViewPath = path.resolve(__dirname, '..', 'src', 'features', 'inbox', 'InboxView.tsx')
const workerCardPath = path.resolve(__dirname, '..', 'src', 'features', 'settings', 'WorkerHealthCard.tsx')
const settingsViewPath = path.resolve(__dirname, '..', 'src', 'features', 'settings', 'SettingsView.tsx')
const idLocalePath = path.resolve(__dirname, '..', 'src', 'locales', 'id.json')
const enLocalePath = path.resolve(__dirname, '..', 'src', 'locales', 'en.json')
const landingPagePath = path.resolve(__dirname, '..', 'src', 'features', 'landing', 'LandingPage.tsx')

test('1. Web Audio Chime & Synthesizer: frequencies, decay duration and silent fallback', async () => {
  const helperSource = fs.readFileSync(notificationsHelperPath, 'utf8')

  // Import directly to verify TypeScript module compiles and exports cleanly
  const {
    CHIME_FREQUENCIES,
    CHIME_DECAY_DURATION,
    TASK_COMPLETED_BADGE_TEMPLATE,
    playTaskCompletionChime,
  } = await import('../src/features/inbox/task-notifications.ts')

  // Verify exact harmonic frequencies C6 (1046.5 Hz) and E6 (1318.5 Hz)
  assert.equal(CHIME_FREQUENCIES.C6, 1046.5, 'C6 frequency must be exactly 1046.5 Hz')
  assert.equal(CHIME_FREQUENCIES.E6, 1318.5, 'E6 frequency must be exactly 1318.5 Hz')

  // Verify exponential decay duration of 0.4s
  assert.equal(CHIME_DECAY_DURATION, 0.4, 'Decay duration must be 0.4s')

  // Verify exponential decay ramp in source code
  assert.ok(
    helperSource.includes('exponentialRampToValueAtTime'),
    'Synthesizer must use exponentialRampToValueAtTime for natural acoustic decay'
  )

  // Verify silent fallback: in node environment where window/AudioContext is undefined,
  // calling playTaskCompletionChime must return false gracefully without throwing
  const result = await playTaskCompletionChime()
  assert.equal(result, false, 'Should gracefully return false without throwing when Web Audio is unavailable')

  // Verify badge template format
  assert.equal(TASK_COMPLETED_BADGE_TEMPLATE(1), '(1) Tugas Selesai! - Cuan')
  assert.equal(TASK_COMPLETED_BADGE_TEMPLATE(3), '(3) Tugas Selesai! - Cuan')
})

test('2. Visual Tab Title Notification Badge: badge format and window.onfocus restore logic', async () => {
  const {
    showTaskCompletedTitleBadge,
    restoreOriginalTitle,
    isTitleBadgeActive,
    getOriginalTitle,
    notifyTaskCompleted,
  } = await import('../src/features/inbox/task-notifications.ts')

  // Mock global document & window for title badge tests
  const originalDoc = globalThis.document
  const originalWin = globalThis.window

  const listeners = {}
  globalThis.window = {
    addEventListener: (evt, fn) => {
      listeners[evt] = listeners[evt] || []
      listeners[evt].push(fn)
    },
    removeEventListener: (evt, fn) => {
      if (listeners[evt]) {
        listeners[evt] = listeners[evt].filter((f) => f !== fn)
      }
    },
  }

  globalThis.document = {
    title: 'Dashboard - Cuan',
    hidden: true,
    addEventListener: (evt, fn) => {
      listeners[evt] = listeners[evt] || []
      listeners[evt].push(fn)
    },
    removeEventListener: (evt, fn) => {
      if (listeners[evt]) {
        listeners[evt] = listeners[evt].filter((f) => f !== fn)
      }
    },
  }

  try {
    // Show badge
    showTaskCompletedTitleBadge(1)
    assert.equal(globalThis.document.title, '(1) Tugas Selesai! - Cuan')
    assert.equal(isTitleBadgeActive(), true)
    assert.equal(getOriginalTitle(), 'Dashboard - Cuan')

    // Simulate window onfocus
    assert.ok(listeners['focus'] && listeners['focus'].length > 0, 'Must register focus listener')
    const focusFn = listeners['focus'][0]
    focusFn()

    // Title must be restored
    assert.equal(globalThis.document.title, 'Dashboard - Cuan')
    assert.equal(isTitleBadgeActive(), false)
    assert.equal(getOriginalTitle(), null)

    // Test notifyTaskCompleted triggers title badge when document.hidden is true
    globalThis.document.hidden = true
    notifyTaskCompleted({ count: 2 })
    assert.equal(globalThis.document.title, '(2) Tugas Selesai! - Cuan')
    assert.equal(isTitleBadgeActive(), true)

    // Clean up
    restoreOriginalTitle()
    assert.equal(globalThis.document.title, 'Dashboard - Cuan')
  } finally {
    globalThis.document = originalDoc
    globalThis.window = originalWin
  }
})

test('3. InboxView Integration: wires task completion to audio chime & tab badge', () => {
  const inboxViewSource = fs.readFileSync(inboxViewPath, 'utf8')

  // Verify import of notifyTaskCompleted
  assert.ok(
    inboxViewSource.includes("import { notifyTaskCompleted } from './task-notifications'"),
    'InboxView must import notifyTaskCompleted from task-notifications'
  )

  // Verify reactive completion effect
  assert.ok(
    inboxViewSource.includes('notifyTaskCompleted({ count: newlyCompletedCount })'),
    'InboxView must trigger notifyTaskCompleted when tasks transition from active to done/review'
  )
})

test('4. WorkerHealthCard: testids, status mapping, queue depth and heartbeat UI', () => {
  const cardSource = fs.readFileSync(workerCardPath, 'utf8')

  // Verify testids
  assert.ok(cardSource.includes('data-testid="worker-health-card"'), 'Must have worker-health-card testid')
  assert.ok(cardSource.includes('data-testid="worker-health-status"'), 'Must have worker-health-status testid')
  assert.ok(cardSource.includes('data-testid="worker-queue-depth"'), 'Must have worker-queue-depth testid')
  assert.ok(cardSource.includes('data-testid="worker-heartbeat"'), 'Must have worker-heartbeat testid')
  assert.ok(cardSource.includes('data-testid="worker-refresh-btn"'), 'Must have worker-refresh-btn testid')

  // Verify endpoint URL
  assert.ok(cardSource.includes('/api/system/worker-health'), 'Must fetch from /api/system/worker-health')

  // Verify visual states: ready (emerald), busy (amber), attention (destructive)
  assert.ok(cardSource.includes('border-emerald-500'), 'Must include emerald styling for ready status')
  assert.ok(cardSource.includes('border-amber-500'), 'Must include amber styling for busy status')
  assert.ok(cardSource.includes('border-destructive'), 'Must include destructive styling for attention status')

  // Verify queue metrics display
  assert.ok(cardSource.includes('queue_depth'), 'Must render queue_depth')
  assert.ok(cardSource.includes('queued_count'), 'Must render queued_count')
  assert.ok(cardSource.includes('running_count'), 'Must render running_count')
})

test('5. SettingsView Integration: mounts WorkerHealthCard in general settings section', () => {
  const settingsSource = fs.readFileSync(settingsViewPath, 'utf8')

  // Verify import
  assert.ok(
    settingsSource.includes("import { WorkerHealthCard } from './WorkerHealthCard'"),
    'SettingsView must import WorkerHealthCard'
  )

  // Verify JSX placement
  assert.ok(
    settingsSource.includes('<WorkerHealthCard />'),
    'SettingsView must render <WorkerHealthCard />'
  )
})

test('6. LandingPage Synchronization: highlights Web Audio chime & Worker Health', () => {
  const lpSource = fs.readFileSync(landingPagePath, 'utf8')

  assert.ok(
    lpSource.includes('Background audio chime & tab notifications.'),
    'LandingPage must feature Background audio chime & tab notifications'
  )
  assert.ok(
    lpSource.includes('Worker observability & queue health.'),
    'LandingPage must feature Worker observability & queue health'
  )
})

test('7. Localization: id.json and en.json contain all worker health & audio notification strings', () => {
  const idLocale = JSON.parse(fs.readFileSync(idLocalePath, 'utf8'))
  const enLocale = JSON.parse(fs.readFileSync(enLocalePath, 'utf8'))

  const requiredKeys = [
    'Background audio chime & tab notifications.',
    'Harmonic Web Audio chime synthesizes subtle sound cues and dynamic browser tab badges when tasks finish in the background, with zero-friction silent autoplay fallback.',
    'Worker observability & queue health.',
    'Monitor your cuan-runner daemon health, queue depth, and heartbeat directly in Settings with crystal-clear visual status indicators and zero terminal complexity.',
    'Kesehatan Worker & Antrean',
    'Status daemon cuan-runner, kedalaman antrean tugas toko, dan detak jantung terakhir.',
    'Status Daemon',
    'Aktif / Siap',
    'Sibuk / Memproses',
    'Perhatian / Tidak Aktif',
    'Worker siap menerima dan mengeksekusi tugas baru.',
    'Worker sedang memproses tugas antrean secara aktif.',
    'Tidak ada heartbeat terbaru dari daemon cuan-runner.',
    'Kedalaman Antrean',
    'tugas total',
    'Menunggu',
    'Berjalan',
    'Detak Jantung (Heartbeat)',
    'Belum ada detak jantung',
    'Baru saja (beberapa detik lalu)',
    'Periksa Status',
    'Memeriksa...',
    'Host',
    'Runner',
    'Gagal memuat status worker',
    'Koneksi ke endpoint worker terputus',
  ]

  for (const key of requiredKeys) {
    assert.ok(idLocale[key], `id.json missing key: ${key}`)
    assert.ok(enLocale[key], `en.json missing key: ${key}`)
  }
})
