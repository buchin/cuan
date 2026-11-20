import test from 'node:test'
import assert from 'node:assert/strict'
import fs from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'

const __filename = fileURLToPath(import.meta.url)
const __dirname = path.dirname(__filename)
const rootDir = path.resolve(__dirname, '..')

test('1. App.tsx: Elimination of dead ActionDialog and clean newFlow state', () => {
  const appPath = path.join(rootDir, 'src/App.tsx')
  const content = fs.readFileSync(appPath, 'utf8')

  // ActionDialog should not be imported or used in App.tsx
  assert.doesNotMatch(content, /import\s*\{\s*ActionDialog\s*\}\s*from/, 'App.tsx must not import ActionDialog')
  assert.doesNotMatch(content, /<ActionDialog/, 'App.tsx must not render dead ActionDialog')
  assert.doesNotMatch(content, /openNew\(\s*kind\s*:\s*[^)]*['"]menu['"]/, 'openNew must not accept dead "menu" kind')
  assert.match(content, /const\s*\[newFlow,\s*setNewFlow\]\s*=\s*useState<['"]contact['"]\s*\|\s*null>\(null\)/, 'newFlow must be typed cleanly as "contact" | null')
})

test('2. InboxView.tsx: Agent Empty State Starter Chips', () => {
  const inboxViewPath = path.join(rootDir, 'src/features/inbox/InboxView.tsx')
  const content = fs.readFileSync(inboxViewPath, 'utf8')

  // Must render agent-prompt-starters and starter-chip
  assert.match(content, /data-testid="agent-prompt-starters"/, 'InboxView must render agent-prompt-starters container')
  assert.match(content, /data-testid="starter-chip"/, 'InboxView must render starter-chip buttons')

  // Check 4 business goal categories aligned with LP promises
  assert.match(content, /Analisis CRM & Draf Broadcast/, 'Must include CRM & Broadcast starter')
  assert.match(content, /Webhook & Bot WhatsApp/, 'Must include Webhook & WhatsApp starter')
  assert.match(content, /Kesehatan Server & Antrean/, 'Must include Server Health & Queue starter')
  assert.match(content, /Fitur Baru & Kriteria Selesai \(DoD\)/, 'Must include New Feature & DoD starter')

  // Check that clicking fills inputMessage and focuses textarea
  assert.match(content, /setInputMessage\(item\.text\)/, 'Clicking starter chip must set inputMessage')
  assert.match(content, /textareaRef\.current\?\.focus\(\)/, 'Clicking starter chip must focus textarea')
})

test('3. Locales: 100% complete translation keys for agent starters', () => {
  const idPath = path.join(rootDir, 'src/locales/id.json')
  const enPath = path.join(rootDir, 'src/locales/en.json')
  const id = JSON.parse(fs.readFileSync(idPath, 'utf8'))
  const en = JSON.parse(fs.readFileSync(enPath, 'utf8'))

  const keys = [
    'Cuan Assistant',
    'Sampaikan kebutuhan operasional atau pilih contoh tugas di bawah:',
    'Analisis CRM & Draf Broadcast',
    'Tolong analisis segmen kontak pelanggan CRM dan siapkan draf email broadcast promosi yang relevan.',
    'Webhook & Bot WhatsApp',
    'Tolong hubungkan webhook form order ke bot WhatsApp notifikasi admin.',
    'Kesehatan Server & Antrean',
    'Tolong periksa status kesehatan daemon runner, antrean tugas yang berjalan, dan metrik sistem VPS saat ini.',
    'Fitur Baru & Kriteria Selesai (DoD)',
    'Saya ingin membuat peningkatan fitur baru di project. Tolong pelajari codebase, buatkan Approval Card dengan rencana kerja dan kriteria pengujian (DoD) sebelum eksekusi.',
  ]

  for (const k of keys) {
    assert.ok(id[k], `id.json missing key: ${k}`)
    assert.ok(en[k], `en.json missing key: ${k}`)
  }
})
