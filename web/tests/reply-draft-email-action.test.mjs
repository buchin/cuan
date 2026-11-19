import test from 'node:test'
import assert from 'node:assert/strict'
import fs from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'

const __dirname = path.dirname(fileURLToPath(import.meta.url))
const replyDraftCardPath = path.resolve(__dirname, '../src/features/inbox/ReplyDraftCard.tsx')
const inboxViewPath = path.resolve(__dirname, '../src/features/inbox/InboxView.tsx')
const chatApiPath = path.resolve(__dirname, '../src/api/chat.ts')
const localesIdPath = path.resolve(__dirname, '../src/locales/id.json')
const localesEnPath = path.resolve(__dirname, '../src/locales/en.json')

test('1. chat.ts exports sendCustomerReplyEmail and sendConversationEmail with typed contracts', () => {
  const chatApiSrc = fs.readFileSync(chatApiPath, 'utf8')
  assert.ok(chatApiSrc.includes('export async function sendCustomerReplyEmail'), 'sendCustomerReplyEmail must be exported')
  assert.ok(chatApiSrc.includes('export async function sendConversationEmail'), 'sendConversationEmail must be exported')
  assert.ok(chatApiSrc.includes('export interface SendCustomerEmailPayload'), 'SendCustomerEmailPayload interface must be declared')
  assert.ok(chatApiSrc.includes('export interface SendCustomerEmailResponse'), 'SendCustomerEmailResponse interface must be declared')
  assert.ok(chatApiSrc.includes('/messages/${encodeURIComponent(messageId)}/email'), 'Endpoint path must match message email action route')
  assert.ok(chatApiSrc.includes('/send-email'), 'Endpoint path must match conversation send-email route')
})

test('2. ReplyDraftCard provides 1-tap Send Email action button, recipient row, and status feedback', () => {
  const src = fs.readFileSync(replyDraftCardPath, 'utf8')

  // Recipient row & editing
  assert.ok(src.includes('data-reply-recipient-row'), 'ReplyDraftCard must include recipient row')
  assert.ok(src.includes('data-recipient-input'), 'ReplyDraftCard must include recipient input field')
  assert.ok(src.includes('data-recipient-display'), 'ReplyDraftCard must include recipient display element')
  assert.ok(src.includes('data-edit-recipient-btn'), 'ReplyDraftCard must include edit recipient button')

  // Send Email Action
  assert.ok(src.includes('data-send-email-action'), 'ReplyDraftCard must include data-send-email-action button')
  assert.ok(src.includes("t('Send Email')"), 'Button must have localized Send Email label')
  assert.ok(src.includes("t('Sending email…')"), 'Button must have localized Sending email… loading state')

  // Sent Badge (Success feedback)
  assert.ok(src.includes('data-email-sent-badge'), 'ReplyDraftCard must include email sent badge on success')
  assert.ok(src.includes("t('Email sent')"), 'Sent badge must display localized Email sent label')
  assert.ok(src.includes('text-emerald-600'), 'Success feedback must use calm emerald tone conforming to Low Visual Noise')

  // Error feedback
  assert.ok(src.includes('data-email-error'), 'ReplyDraftCard must include error feedback element on failure')

  // Touch target DoD >= 44px
  const minHeights = (src.match(/min-h-11/g) || []).length
  assert.ok(minHeights >= 3, 'Action buttons and sent badge must maintain min-h-11 (>=44px touch target)')
})

test('3. InboxView binds ReplyDraftCard with activeContact email, message uuid, and onSendEmail handler', () => {
  const inboxViewSrc = fs.readFileSync(inboxViewPath, 'utf8')
  assert.ok(inboxViewSrc.includes('sendCustomerReplyEmail'), 'InboxView must import sendCustomerReplyEmail')
  assert.ok(inboxViewSrc.includes('messageId={msg.uuid}'), 'InboxView must pass messageId')
  assert.ok(inboxViewSrc.includes('defaultRecipient={activeContact.email}'), 'InboxView must pass defaultRecipient from activeContact.email')
  assert.ok(inboxViewSrc.includes('emailDelivery={msg.email_delivery}'), 'InboxView must pass emailDelivery state')
  assert.ok(inboxViewSrc.includes('onSendEmail='), 'InboxView must wire onSendEmail handler')
})

test('4. Human-in-the-Loop validation: email only sends on explicit operator click', () => {
  const src = fs.readFileSync(replyDraftCardPath, 'utf8')
  assert.ok(src.includes('onClick={handleSendEmail}'), 'Sending is exclusively wired to handleSendEmail on user button click')
  assert.ok(!src.includes('useEffect(() => {\n    void handleSendEmail()'), 'No automatic background sending on mount')
})

test('5. Localizations exist in id.json and en.json for email sending actions and feedback', () => {
  const idLocales = JSON.parse(fs.readFileSync(localesIdPath, 'utf8'))
  const enLocales = JSON.parse(fs.readFileSync(localesEnPath, 'utf8'))

  const requiredKeys = [
    'Send Email',
    'Sending email…',
    'Email sent',
    'To:',
    'Recipient email is required.',
    'Invalid recipient email format.',
    'Failed to send email. Please try again.',
  ]

  for (const key of requiredKeys) {
    assert.ok(idLocales[key], `Indonesian locale missing key: ${key}`)
    assert.ok(enLocales[key], `English locale missing key: ${key}`)
  }

  assert.equal(idLocales['Send Email'], 'Kirim Email')
  assert.equal(idLocales['Email sent'], 'Email terkirim')
  assert.equal(idLocales['Sending email…'], 'Mengirim email…')
})
