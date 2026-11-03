import test from 'node:test'
import assert from 'node:assert/strict'
import fs from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'

const __dirname = path.dirname(fileURLToPath(import.meta.url))
const rootDir = path.resolve(__dirname, '..')

test('1. ResendSettingsCard: component structure, security masking, and interactive actions', () => {
  const filePath = path.join(rootDir, 'src/features/settings/ResendSettingsCard.tsx')
  assert.ok(fs.existsSync(filePath), 'ResendSettingsCard.tsx must exist')
  const content = fs.readFileSync(filePath, 'utf-8')

  // Check endpoints and payload structure
  assert.match(content, /\/api\/v1\/settings\/resend/, 'Must fetch and post to /api/v1/settings/resend')
  assert.match(content, /\/api\/v1\/settings\/resend\/validate/, 'Must include key validation endpoint')
  assert.match(content, /\/api\/v1\/settings\/resend\/test/, 'Must include email test endpoint')

  // Check state and form controls
  assert.match(content, /apiKeyInput/, 'Must manage apiKey input state')
  assert.match(content, /showKey/, 'Must support masking toggle')
  assert.match(content, /idleMinutes/, 'Must manage idle threshold duration')
  assert.match(content, /handleValidateKey/, 'Must have validate API key handler')
  assert.match(content, /handleSendTestEmail/, 'Must have send test email handler')
  assert.match(content, /handleSave/, 'Must have settings save handler')

  // Check security masking and vault notification
  assert.match(content, /AES-256-GCM/, 'Must emphasize AES-256-GCM encrypted vault security')
  assert.match(content, /masked_api_key/, 'Must display masked API key safely')
})

test('2. IntegrationOverview: displays Resend Email integration card linking to settings', () => {
  const filePath = path.join(rootDir, 'src/features/settings/IntegrationOverview.tsx')
  assert.ok(fs.existsSync(filePath), 'IntegrationOverview.tsx must exist')
  const content = fs.readFileSync(filePath, 'utf-8')

  assert.match(content, /Resend Email/, 'Must show Resend Email integration card')
  assert.match(content, /\/settings\/integrations\/resend/, 'Must link to /settings/notifications for setup')
  assert.match(content, /Transkrip obrolan otomatis/, 'Must describe auto transcript capability')
})

test('3. NotificationSettings: embeds ResendSettingsCard', () => {
  const filePath = path.join(rootDir, 'src/features/settings/NotificationSettings.tsx')
  assert.ok(fs.existsSync(filePath), 'NotificationSettings.tsx must exist')
  const content = fs.readFileSync(filePath, 'utf-8')

  assert.ok(!content.includes('import { ResendSettingsCard }'), 'Notifications must link to canonical integration instead of duplicating its form')
  assert.match(content, /settings\/integrations\/resend/, 'Notifications must link to email integration')
})

test('4. LandingPage: promotes Resend email alerts and idle chat transcripts capability', () => {
  const filePath = path.join(rootDir, 'src/features/landing/LandingPage.tsx')
  assert.ok(fs.existsSync(filePath), 'LandingPage.tsx must exist')
  const content = fs.readFileSync(filePath, 'utf-8')

  assert.match(content, /Resend email alerts & idle chat transcripts/, 'Landing page must feature Resend capability')
  assert.match(content, /idle \(>5-10m\)/, 'Landing page must mention idle detection window')
})

test('5. Localization: en.json and id.json include Resend email keys', () => {
  const enPath = path.join(rootDir, 'src/locales/en.json')
  const idPath = path.join(rootDir, 'src/locales/id.json')

  assert.ok(fs.existsSync(enPath), 'en.json must exist')
  assert.ok(fs.existsSync(idPath), 'id.json must exist')

  const en = JSON.parse(fs.readFileSync(enPath, 'utf-8'))
  const id = JSON.parse(fs.readFileSync(idPath, 'utf-8'))

  const requiredKeys = [
    'Integrasi Email Resend & Transkrip Chat Idle',
    'Resend API Key',
    'Ambang Batas Waktu Chat Idle',
    'Simpan Pengaturan Resend',
    'Kirim Uji Coba',
  ]

  for (const k of requiredKeys) {
    assert.ok(en[k], `en.json must have translation for key: "${k}"`)
    assert.ok(id[k], `id.json must have translation for key: "${k}"`)
  }
})
