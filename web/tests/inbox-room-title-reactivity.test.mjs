import test from 'node:test'
import assert from 'node:assert/strict'
import fs from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'

const __filename = fileURLToPath(import.meta.url)
const __dirname = path.dirname(__filename)
const webDir = path.resolve(__dirname, '..')
const appPath = path.resolve(webDir, 'src/App.tsx')
const chatApiPath = path.resolve(webDir, 'src/api/chat.ts')
const inboxViewPath = path.resolve(webDir, 'src/features/inbox/InboxView.tsx')
const inboxSubPanelPath = path.resolve(webDir, 'src/features/inbox/InboxSubPanel.tsx')

test('Inbox Title Reactivity: App.tsx listens to cuan-conversation-updated event and updates sidebar list without reload', () => {
  assert.ok(fs.existsSync(appPath), 'App.tsx must exist')
  const src = fs.readFileSync(appPath, 'utf8')

  assert.ok(
    src.includes("window.addEventListener('cuan-conversation-updated', changed)"),
    'App.tsx must listen to cuan-conversation-updated'
  )
  assert.ok(
    src.includes("window.removeEventListener('cuan-conversation-updated', changed)"),
    'App.tsx must clean up cuan-conversation-updated listener'
  )
  assert.ok(
    src.includes('targetId') && src.includes('newTitle') && src.includes('setContacts'),
    'App.tsx must re-map local contacts state immediately upon receiving event'
  )
})

test('Inbox Title Reactivity: chatApi defines updateTitle endpoint and stream snapshot includes room_title', () => {
  assert.ok(fs.existsSync(chatApiPath), 'chat.ts must exist')
  const src = fs.readFileSync(chatApiPath, 'utf8')

  assert.ok(src.includes('updateTitle: async'), 'chatApi must export updateTitle')
  assert.ok(src.includes("method: 'PATCH'"), 'updateTitle must send PATCH request')
  assert.ok(src.includes('room_title?: string'), 'InboxSnapshot must include room_title')
})

test('Inbox Title Reactivity: InboxView propagates room_title and task creations to sidebar', () => {
  assert.ok(fs.existsSync(inboxViewPath), 'InboxView.tsx must exist')
  const src = fs.readFileSync(inboxViewPath, 'utf8')

  assert.ok(
    src.includes("detail: { roomId: convId, title: data.room_title }"),
    'InboxView must dispatch cuan-conversation-updated on stream data.room_title change'
  )
  assert.ok(
    src.includes("detail: { roomId: convId, title: card.task_title }"),
    'InboxView must dispatch cuan-conversation-updated when task is approved'
  )
})

test('Inbox Title Reactivity: InboxSubPanel provides conversation rename action', () => {
  assert.ok(fs.existsSync(inboxSubPanelPath), 'InboxSubPanel.tsx must exist')
  const src = fs.readFileSync(inboxSubPanelPath, 'utf8')

  assert.ok(src.includes('Rename conversation'), 'InboxSubPanel must provide Rename conversation action')
  assert.ok(src.includes('chatApi.updateTitle'), 'InboxSubPanel must call chatApi.updateTitle')
})
