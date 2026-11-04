import test from 'node:test'
import assert from 'node:assert/strict'
import fs from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'

const __dirname = path.dirname(fileURLToPath(import.meta.url))
const inboxViewPath = path.resolve(__dirname, '../src/features/inbox/InboxView.tsx')
const chatApiPath = path.resolve(__dirname, '../src/api/chat.ts')
const localesIdPath = path.resolve(__dirname, '../src/locales/id.json')

test('1. ChatMessage interface declares is_internal, mention_handle, and reply_draft', () => {
  const chatApiSrc = fs.readFileSync(chatApiPath, 'utf8')
  assert.ok(chatApiSrc.includes('is_internal?: boolean'), 'ChatMessage must declare is_internal')
  assert.ok(chatApiSrc.includes('mention_handle?: string'), 'ChatMessage must declare mention_handle')
  assert.ok(chatApiSrc.includes('reply_draft?: string'), 'ChatMessage must declare reply_draft')
})

test('2. InboxView uses the shared internal visibility explanation', () => {
  const inboxViewSrc = fs.readFileSync(inboxViewPath, 'utf8')
  assert.ok(inboxViewSrc.includes('msg.is_internal'), 'InboxView must check msg.is_internal')
  assert.ok(
    inboxViewSrc.includes('<InternalVisibility'),
    'Internal surfaces must expose the reusable privacy explanation'
  )
  const indicator = fs.readFileSync(path.resolve(__dirname, '../src/components/ui/InternalVisibility.tsx'), 'utf8')
  assert.ok(indicator.includes("t('Internal · Invisible to visitor')"), 'Internal marker retains its accessible label')
})

test('3. InboxView renders ready-to-use customer draft reply card with Review & Use action', () => {
  const inboxViewSrc = fs.readFileSync(inboxViewPath, 'utf8')
  assert.ok(inboxViewSrc.includes('msg.reply_draft'), 'InboxView must check msg.reply_draft')
  const draftCardSrc = fs.readFileSync(path.resolve(__dirname, '../src/features/inbox/ReplyDraftCard.tsx'), 'utf8')
  assert.ok(inboxViewSrc.includes('<ReplyDraftCard'), 'Inbox connects the separate draft attachment')
  assert.ok(
    draftCardSrc.includes("t('Customer reply draft · Not sent yet')"),
    'InboxView must show draft title'
  )
  assert.ok(
    draftCardSrc.includes("t('Review & Use')"),
    'InboxView must have Review & Use button'
  )
  assert.ok(
    inboxViewSrc.includes('setInputMessage(draft)'),
    'Clicking Review & Use must populate inputMessage with draft'
  )
})

test('4. InboxView renders clean single-agent whisper mode (@cuan) without clutter pills', () => {
  const inboxViewSrc = fs.readFileSync(inboxViewPath, 'utf8')
  assert.ok(inboxViewSrc.includes("@cuan"), 'Composer must provide @cuan whisper mode')
  assert.ok(
    inboxViewSrc.includes("t('Whisper @cuan')"),
    'Composer must display Whisper @cuan toggle'
  )
})

test('5. Indonesian localizations for whisper and draft reply exist', () => {
  const locales = JSON.parse(fs.readFileSync(localesIdPath, 'utf8'))
  assert.ok(locales['Review & Use'], 'Translation for Review & Use must exist')
  assert.ok(
    locales['Customer reply draft · Not sent yet'],
    'Translation for Customer reply draft must exist'
  )
  assert.ok(locales['Whisper AI:'], 'Translation for Whisper AI: must exist')
})
