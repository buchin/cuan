import test from 'node:test'
import assert from 'node:assert'
import fs from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'

const __filename = fileURLToPath(import.meta.url)
const __dirname = path.dirname(__filename)

const chatApiPath = path.join(__dirname, '../src/api/chat.ts')
const inboxActivityPath = path.join(__dirname, '../src/features/inbox/InboxActivity.tsx')
const idLocalePath = path.join(__dirname, '../src/locales/id.json')

const chatApiSource = fs.readFileSync(chatApiPath, 'utf8')
const inboxActivitySource = fs.readFileSync(inboxActivityPath, 'utf8')
const idLocale = JSON.parse(fs.readFileSync(idLocalePath, 'utf8'))

test('1. InboxRequest interface supports continue_analysis in recovery_action', () => {
  assert.ok(
    chatApiSource.includes("'continue_analysis'"),
    'InboxRequest must support continue_analysis recovery action'
  )
})

test('2. InboxActivity detects tool budget exhaustion and provides Lanjutkan Analisis / Coba Lagi buttons', () => {
  assert.ok(
    inboxActivitySource.includes('isToolBudget'),
    'InboxActivity must detect tool budget state'
  )
  assert.ok(
    inboxActivitySource.includes('Continue analysis'),
    'InboxActivity must render Continue analysis action button'
  )
  assert.ok(
    inboxActivitySource.includes('runtime_tool_budget_exceeded'),
    'InboxActivity must recognize runtime_tool_budget_exceeded in latestActivity'
  )
  assert.ok(
    inboxActivitySource.includes('min-h-11'),
    'Action buttons must maintain WCAG touch accessibility standard min-h-11'
  )
})

test('3. Indonesian locale dictionary contains complete translations for tool budget recovery', () => {
  assert.strictEqual(idLocale['Continue analysis'], 'Lanjutkan analisis')
  assert.strictEqual(idLocale['Continuing analysis…'], 'Melanjutkan analisis…')
  assert.ok(idLocale['The tool budget was reached for this turn. Your message is saved.'])
  assert.ok(idLocale['This attempt cannot be safely repeated. Check its status and details.'])
})
