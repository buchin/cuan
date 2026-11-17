import test from 'node:test'
import assert from 'node:assert/strict'
import fs from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'
import {
  isAgentMention,
  extractMentionHandle,
  isComposerWhisper,
} from '../src/features/inbox/mention-whisper.ts'

const __dirname = path.dirname(fileURLToPath(import.meta.url))
const inboxViewPath = path.resolve(__dirname, '../src/features/inbox/InboxView.tsx')
const localesIdPath = path.resolve(__dirname, '../src/locales/id.json')

test('1. Inbox composer: Reply mode tab bar is completely eliminated (DoD 1)', () => {
  const inboxViewSrc = fs.readFileSync(inboxViewPath, 'utf8')

  // The old tab buttons and tablist must not exist in InboxView
  assert.ok(
    !inboxViewSrc.includes('id="compose-public-tab"'),
    'compose-public-tab must be removed from InboxView',
  )
  assert.ok(
    !inboxViewSrc.includes('id="compose-internal-tab"'),
    'compose-internal-tab must be removed from InboxView',
  )
  assert.ok(
    !inboxViewSrc.includes('role="tablist" aria-label={t(\'Reply mode\')}'),
    'Reply mode tablist must be removed from InboxView',
  )

  // Composer retains clean wrapper without dead zone
  assert.ok(
    inboxViewSrc.includes('data-inbox-composer'),
    'Composer wrapper retains data-inbox-composer attribute',
  )
  assert.ok(
    inboxViewSrc.includes('data-primary-composer'),
    'Primary composer textarea is present',
  )
})

test('2. Mention detection helpers: accurately detect @agent and ignore non-mentions', () => {
  // Plain customer replies: never whisper
  assert.equal(isAgentMention(''), false)
  assert.equal(isAgentMention('Halo apa kabar'), false)
  assert.equal(isAgentMention('Terima kasih atas bantuannya!'), false)
  assert.equal(isAgentMention('user@example.com'), false)
  assert.equal(isAgentMention('Kirim email ke info@cuan.id ya'), false)

  // Mentions at start, middle, or with punctuation
  assert.equal(isAgentMention('@cuan'), true)
  assert.equal(isAgentMention('@cuan tolong cek ini'), true)
  assert.equal(isAgentMention('Halo @cuan tolong cek'), true)
  assert.equal(isAgentMention('Tolong ya (@cuan)'), true)
  assert.equal(isAgentMention('Halo mas @dev, tolong cek order #1234'), true)
  assert.equal(isAgentMention('@team update status'), true)
  assert.equal(isAgentMention('@sm tolong review'), true)

  // Typing '@' alone immediately starts whisper mode
  assert.equal(isAgentMention('@'), true)
  assert.equal(isAgentMention('Halo @'), true)

  // Extract mention handle correctly
  assert.equal(extractMentionHandle(''), '')
  assert.equal(extractMentionHandle('Halo apa kabar'), '')
  assert.equal(extractMentionHandle('@cuan'), '@cuan')
  assert.equal(extractMentionHandle('@cuan tolong cek'), '@cuan')
  assert.equal(extractMentionHandle('Halo @cuan tolong cek'), '@cuan')
  assert.equal(extractMentionHandle('Tolong ya (@cuan)'), '@cuan')
  assert.equal(extractMentionHandle('Halo @dev, tolong'), '@dev')
  assert.equal(extractMentionHandle('user@example.com'), '')
})

test('3. Composer whisper mode: active only for customer rooms with mention (DoD 2, 3, 4)', () => {
  // Default is public reply for customer room without mention
  assert.equal(isComposerWhisper('customer', ''), false)
  assert.equal(isComposerWhisper('customer', 'Halo kak, barang sudah dikirim?'), false)

  // Switching to whisper when mention is typed
  assert.equal(isComposerWhisper('customer', '@cuan tolong carikan data resi'), true)
  assert.equal(isComposerWhisper('customer', 'Tolong carikan data @cuan'), true)

  // Reverting to public reply when mention is deleted
  assert.equal(isComposerWhisper('customer', 'Tolong carikan data'), false)

  // Agent rooms are never whisper mode
  assert.equal(isComposerWhisper('agent', '@cuan tolong buatkan task'), false)
  assert.equal(isComposerWhisper(undefined, '@cuan tolong buatkan task'), false)
})

test('4. Real-time visual feedback: ring, lock icon, placeholder, and status indicator', () => {
  const inboxViewSrc = fs.readFileSync(inboxViewPath, 'utf8')

  // Visual feedback: Amber ring on composer when whisper
  assert.ok(
    inboxViewSrc.includes('ring-1 ring-amber-500/50 border-amber-500/40'),
    'Composer must apply amber border and ring when whisper is active',
  )

  // Visual feedback: Send button switches to Lock icon with amber styling
  assert.ok(
    inboxViewSrc.includes('<Lock className="h-4 w-4" />'),
    'Send button must render Lock icon in whisper mode',
  )
  assert.ok(
    inboxViewSrc.includes('text-amber-500 hover:text-amber-400 bg-amber-500/15'),
    'Send button must have amber accent in whisper mode',
  )

  // Visual feedback: Accessible labels and tooltips
  assert.ok(
    inboxViewSrc.includes("t('Send internal whisper to AI')"),
    'Send button must have accessible whisper label',
  )

  // Visual feedback: Dynamic placeholder
  assert.ok(
    inboxViewSrc.includes("t('Internal message…')"),
    'Placeholder must show Internal message… in whisper mode',
  )
  assert.ok(
    inboxViewSrc.includes("t('Write a reply…')"),
    'Placeholder must show Write a reply… in public reply mode',
  )

  // Visual feedback: Dynamic whisper indicator above composer
  assert.ok(
    inboxViewSrc.includes("t('Whisper @cuan')"),
    'Status indicator must display Whisper @cuan',
  )
  assert.ok(
    inboxViewSrc.includes("t('Internal · Invisible to visitor')"),
    'Status indicator must explain internal visibility',
  )
})

test('5. Localization completeness: Indonesian strings for whisper and public reply exist', () => {
  const locales = JSON.parse(fs.readFileSync(localesIdPath, 'utf8'))

  assert.equal(locales['Whisper @cuan'], 'Bisikkan ke @cuan')
  assert.equal(locales['Public reply'], 'Balas pengunjung')
  assert.equal(locales['Internal message…'], 'Pesan internal…')
  assert.equal(locales['Write a reply…'], 'Tulis balasan…')
  assert.equal(locales['Send internal whisper to AI'], 'Kirim bisikan internal ke AI')
  assert.equal(locales['Internal · Invisible to visitor'], 'Internal · Tidak terlihat oleh lawan bicara')
})
