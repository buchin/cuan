import test from 'node:test'
import assert from 'node:assert/strict'
import fs from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'

const __filename = fileURLToPath(import.meta.url)
const __dirname = path.dirname(__filename)
const webDir = path.resolve(__dirname, '..')

const taskCardPath = path.resolve(webDir, 'src/features/inbox/TaskCard.tsx')
const idLocalePath = path.resolve(webDir, 'src/locales/id.json')
const enLocalePath = path.resolve(webDir, 'src/locales/en.json')

const idLocale = JSON.parse(fs.readFileSync(idLocalePath, 'utf8'))
const enLocale = JSON.parse(fs.readFileSync(enLocalePath, 'utf8'))

test('4. Localization: Dictionaries include Riwayat Sesi, Sesi, and toggle labels', () => {
  assert.equal(idLocale['Riwayat Sesi'], 'Riwayat Sesi')
  assert.equal(enLocale['Riwayat Sesi'], 'Session History')
  assert.equal(idLocale['Sesi'], 'Sesi')
  assert.equal(enLocale['Sesi'], 'Session')
  assert.equal(idLocale['Lihat'], 'Lihat')
  assert.equal(enLocale['Lihat'], 'View')
  assert.equal(idLocale['Sembunyikan'], 'Sembunyikan')
  assert.equal(enLocale['Sembunyikan'], 'Hide')
})
