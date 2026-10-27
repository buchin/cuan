import test from 'node:test'
import assert from 'node:assert/strict'
import fs from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'

const __dirname = path.dirname(fileURLToPath(import.meta.url))
const chatApiPath = path.resolve(__dirname, '../src/api/chat.ts')
const inboxSubPanelPath = path.resolve(__dirname, '../src/features/inbox/InboxSubPanel.tsx')
const inboxViewPath = path.resolve(__dirname, '../src/features/inbox/InboxView.tsx')
const localesIdPath = path.resolve(__dirname, '../src/locales/id.json')

test('1. ChatContact interface defines online visitor fields', () => {
  const src = fs.readFileSync(chatApiPath, 'utf8')
  assert.ok(src.includes('has_messages?: boolean'), 'ChatContact must include has_messages')
  assert.ok(src.includes('message_count?: number'), 'ChatContact must include message_count')
  assert.ok(src.includes('is_online?: boolean'), 'ChatContact must include is_online')
  assert.ok(src.includes('current_url?: string'), 'ChatContact must include current_url')
  assert.ok(src.includes('page_title?: string'), 'ChatContact must include page_title')
})

test('2. InboxSubPanel filters out empty-message contacts from tab All', () => {
  const src = fs.readFileSync(inboxSubPanelPath, 'utf8')
  // Check that tab 'all' excludes contacts with has_messages === false
  assert.ok(
    src.includes("c.type === 'customer' && c.has_messages === false"),
    'Tab all must exclude customer contacts with has_messages === false'
  )
})

test('3. InboxSubPanel supports customerSubFilter for Chats and Online visitors', () => {
  const src = fs.readFileSync(inboxSubPanelPath, 'utf8')
  assert.ok(
    src.includes("customerSubFilter === 'chats'"),
    'InboxSubPanel must support filtering customers by chats'
  )
  assert.ok(
    src.includes("customerSubFilter === 'online'"),
    'InboxSubPanel must support filtering customers by online status'
  )
  assert.ok(
    src.includes('setCustomerSubFilter'),
    'InboxSubPanel must provide toggle to switch between chats and online visitors'
  )
})

test('4. InboxSubPanel displays online indicator badge for live visitors', () => {
  const src = fs.readFileSync(inboxSubPanelPath, 'utf8')
  assert.ok(
    src.includes('contact.is_online'),
    'InboxSubPanel must check contact.is_online for badge display'
  )
  assert.ok(
    src.includes('bg-emerald-500'),
    'InboxSubPanel must display green dot for online status'
  )
})

test('5. InboxView displays proactive greeting card for empty customer visitors', () => {
  const src = fs.readFileSync(inboxViewPath, 'utf8')
  assert.ok(
    src.includes("activeContact?.type === 'customer'"),
    'InboxView must identify customer contacts for empty state greeting'
  )
  assert.ok(
    src.includes("Say hello"),
    'InboxView must include Say hello proactive action'
  )
  assert.ok(
    src.includes("Offer assistance"),
    'InboxView must include Offer assistance proactive action'
  )
})

test('6. Indonesian locale provides translations for online visitor UI', () => {
  const src = fs.readFileSync(localesIdPath, 'utf8')
  const json = JSON.parse(src)
  assert.ok(json['Chats'], 'Must have translation for Chats')
  assert.ok(json['Online'], 'Must have translation for Online')
  assert.ok(json['Online Visitors'], 'Must have translation for Online Visitors')
  assert.ok(json['Say hello'], 'Must have translation for Say hello')
  assert.ok(json['Offer assistance'], 'Must have translation for Offer assistance')
  assert.ok(json['is online right now'], 'Must have translation for is online right now')
})
