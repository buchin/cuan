import test from 'node:test'
import assert from 'node:assert/strict'
import fs from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'

const __dirname = path.dirname(fileURLToPath(import.meta.url))
const rootDir = path.resolve(__dirname, '..')
const inboxViewPath = path.resolve(rootDir, 'src/features/inbox/InboxView.tsx')
const idLocalePath = path.resolve(rootDir, 'src/locales/id.json')
const enLocalePath = path.resolve(rootDir, 'src/locales/en.json')

test('InboxView eliminates floating Resolution Card and retains header checklist and Archived Banner', () => {
  const content = fs.readFileSync(inboxViewPath, 'utf8')

  // 1. Floating Resolution Card and Tandai Tuntas banner above composer must be eliminated
  assert.ok(!content.includes('data-testid="conversation-resolution-card"'), 'floating conversation-resolution-card must be eliminated')
  assert.ok(!content.includes('data-testid="mark-resolved-button"'), 'mark-resolved-button above composer must be eliminated')
  assert.ok(!content.includes('Tandai Tuntas (Inbox Zero)'), 'Tandai Tuntas (Inbox Zero) text in composer must be eliminated')
  assert.ok(!content.includes('showResolutionCard'), 'showResolutionCard state must be eliminated')

  // 2. Primary resolve control remains in header via ConversationResolutionAction
  assert.ok(content.includes('<ConversationResolutionAction'), 'header must retain ConversationResolutionAction as primary resolve control')

  // 3. Must have data-testid="conversation-resolved-banner" and reopen-conversation-button
  assert.ok(content.includes('data-testid="conversation-resolved-banner"'), 'must include conversation-resolved-banner testid')
  assert.ok(content.includes('data-testid="reopen-conversation-button"'), 'must include reopen-conversation-button testid')
  assert.ok(content.includes('Buka kembali'), 'must include Buka kembali button text')

  // 4. Mobile awareness (< 1200px redirects back to contacts list on resolve)
  assert.ok(content.includes('window.innerWidth < 1200'), 'must check window.innerWidth < 1200 for mobile responsive back navigation')
  assert.ok(content.includes('handleBackToContacts()'), 'must call handleBackToContacts on mobile resolve')

  // 5. Auto-reopen when new message is sent while resolved
  assert.ok(content.includes('if (isResolved)'), 'must handle isResolved state transition')
  assert.ok(content.includes('activeContact.resolved = false'), 'must reopen activeContact on sending new message')
})

test('Locales contain translation keys for resolution flow', () => {
  const idLocale = JSON.parse(fs.readFileSync(idLocalePath, 'utf8'))
  const enLocale = JSON.parse(fs.readFileSync(enLocalePath, 'utf8'))

  assert.equal(idLocale['Semua pekerjaan telah selesai diverifikasi.'], 'Semua pekerjaan telah selesai diverifikasi.')
  assert.equal(idLocale['Selesai berdiskusi?'], 'Selesai berdiskusi?')
  assert.equal(idLocale['Tandai Tuntas (Inbox Zero)'], 'Tandai Tuntas (Inbox Zero)')
  assert.equal(idLocale['Buka kembali'], 'Buka kembali')

  assert.equal(enLocale['Semua pekerjaan telah selesai diverifikasi.'], 'All work has been completed and verified.')
  assert.equal(enLocale['Selesai berdiskusi?'], 'Finished discussing?')
  assert.equal(enLocale['Tandai Tuntas (Inbox Zero)'], 'Mark Resolved (Inbox Zero)')
  assert.equal(enLocale['Buka kembali'], 'Reopen')
})
