import test from 'node:test'
import assert from 'node:assert/strict'
import fs from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'

const __filename = fileURLToPath(import.meta.url)
const __dirname = path.dirname(__filename)
const webDir = path.resolve(__dirname, '..')

const chatCalmCssPath = path.resolve(webDir, 'src/features/inbox/chat-calm.css')
const indexCssPath = path.resolve(webDir, 'src/index.css')
const inboxViewPath = path.resolve(webDir, 'src/features/inbox/InboxView.tsx')

const chatCalmCss = fs.readFileSync(chatCalmCssPath, 'utf8')
const indexCss = fs.readFileSync(indexCssPath, 'utf8')
const inboxView = fs.readFileSync(inboxViewPath, 'utf8')

test('1. chat-calm.css: bubble chat applies smooth rounded corners and clean tail integration', () => {
  // Symmetrical rounded corners (16px) on message bubbles
  assert.ok(
    chatCalmCss.includes('border-radius: 16px !important;'),
    'chat-calm.css must specify border-radius: 16px !important on message bubbles'
  )

  // Incoming tail integration: top-left corner is 0 to seamlessly join the tail
  assert.ok(
    chatCalmCss.includes('.wa-message-bubble[data-bubble-tail="incoming"]') &&
      chatCalmCss.includes('border-top-left-radius: 0 !important;'),
    'chat-calm.css must set border-top-left-radius: 0 on incoming speech bubbles with tail'
  )

  // Outgoing tail integration: top-right corner is 0 to seamlessly join the tail
  assert.ok(
    chatCalmCss.includes('.wa-message-bubble[data-bubble-tail="outgoing"]') &&
      chatCalmCss.includes('border-top-right-radius: 0 !important;'),
    'chat-calm.css must set border-top-right-radius: 0 on outgoing speech bubbles with tail'
  )
})

test('2. index.css: message bubbles are exempt from zero border-radius overrides', () => {
  // .wa-message-bubble must not be in the 0px !important list
  assert.ok(
    !indexCss.includes('.wa-message-bubble,\n.calm-message-bubble,\n.wa-input-capsule'),
    'index.css must not force .wa-message-bubble to 0px'
  )

  // .rounded-2xl must exempt message bubbles
  assert.ok(
    indexCss.includes('.rounded-2xl:not(.wa-message-bubble):not(.calm-message-bubble)'),
    'index.css must exempt message bubbles from 0px override on .rounded-2xl'
  )
})

test('3. InboxView.tsx: message bubble elements include rounded-2xl styling class', () => {
  assert.ok(
    inboxView.includes('wa-message-bubble relative min-w-0 rounded-2xl'),
    'InboxView.tsx must include rounded-2xl on message bubbles'
  )
})
