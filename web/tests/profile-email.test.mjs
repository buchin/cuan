import test from 'node:test'
import assert from 'node:assert/strict'
import fs from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'

const __dirname = path.dirname(fileURLToPath(import.meta.url))
const profileSettingsPath = path.resolve(__dirname, '../src/features/settings/ProfileSettings.tsx')
const localesIdPath = path.resolve(__dirname, '../src/locales/id.json')

test('1. ProfileSettings declares email, emailPassword and emailDirty state', () => {
  const src = fs.readFileSync(profileSettingsPath, 'utf8')
  assert.ok(src.includes('user?.email'), 'ProfileSettings must read initial email from user')
  assert.ok(src.includes('[email, setEmail] = useState'), 'ProfileSettings must manage email state')
  assert.ok(src.includes('[emailPassword, setEmailPassword] = useState'), 'ProfileSettings must manage emailPassword state')
  assert.ok(src.includes('emailDirty = email.trim().toLowerCase() !== savedEmail.trim().toLowerCase()'), 'ProfileSettings must compute emailDirty')
})

test('2. ProfileSettings save handler routes email update to /api/v1/auth/email via PATCH', () => {
  const src = fs.readFileSync(profileSettingsPath, 'utf8')
  assert.ok(src.includes("kind === 'email'"), 'ProfileSettings save() must support email kind')
  assert.ok(src.includes("'/api/v1/auth/email'"), 'ProfileSettings must call /api/v1/auth/email')
  assert.ok(src.includes('current_password: emailPassword'), 'ProfileSettings must supply current_password')
  assert.ok(src.includes('Email address saved. Use it the next time you sign in.'), 'ProfileSettings must display success notice')
})

test('3. ProfileSettings renders dedicated Email address form with password confirmation', () => {
  const src = fs.readFileSync(profileSettingsPath, 'utf8')
  assert.ok(src.includes("id=\"profile-email\""), 'Email input must have id profile-email')
  assert.ok(src.includes('type="email"'), 'Email input must have type="email"')
  assert.ok(src.includes('id=\"profile-email-current-password\"'), 'Current password input must have id profile-email-current-password')
  assert.ok(src.includes("'Change email'"), 'Form must have Change email submit button')
})

test('4. id.json contains complete Indonesian translations for email management', () => {
  const idLocale = JSON.parse(fs.readFileSync(localesIdPath, 'utf8'))
  assert.equal(idLocale['Email address'], 'Alamat email')
  assert.equal(idLocale['Change email'], 'Ubah email')
  assert.equal(idLocale['Email address saved. Use it the next time you sign in.'], 'Alamat email disimpan. Gunakan saat masuk berikutnya.')
  assert.equal(idLocale['Your sign-in and system notifications will be sent to this email.'], 'Akses masuk dan notifikasi sistem akan dikirimkan ke email ini.')
})
