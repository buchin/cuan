import test from 'node:test'
import assert from 'node:assert/strict'
import fs from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'
import ts from 'typescript'

const __filename = fileURLToPath(import.meta.url)
const __dirname = path.dirname(__filename)

function createStorageMock() {
  const store = new Map()
  return {
    get length() {
      return store.size
    },
    key(index) {
      return Array.from(store.keys())[index] ?? null
    },
    getItem(key) {
      return store.has(key) ? store.get(key) : null
    },
    setItem(key, value) {
      store.set(key, String(value))
    },
    removeItem(key) {
      store.delete(key)
    },
    clear() {
      store.clear()
    },
  }
}

globalThis.localStorage = createStorageMock()
globalThis.sessionStorage = createStorageMock()

const cacheSource = fs.readFileSync(
  path.resolve(__dirname, '../src/features/inbox/contacts-cache.ts'),
  'utf8',
)
const cacheTranspiled = ts.transpileModule(cacheSource, {
  compilerOptions: { module: ts.ModuleKind.ESNext, target: ts.ScriptTarget.ES2022 },
}).outputText
const cacheUrl =
  'data:text/javascript;base64,' + Buffer.from(cacheTranspiled).toString('base64')

const {
  INBOX_CONTACTS_CACHE_KEY,
  trimContactSnapshot,
  readInboxContactsCache,
  writeInboxContactsCache,
  clearInboxContactsCache,
} = await import(cacheUrl)

test('writeInboxContactsCache & readInboxContactsCache preserve task status, resolved, and substage', () => {
  localStorage.clear()
  sessionStorage.clear()
  clearInboxContactsCache()

  const longMessage = 'x'.repeat(300)
  const contacts = [
    {
      id: 'topic_1',
      conversation_id: 'room_1',
      type: 'agent',
      agent_id: 'agent_cuan',
      name: 'Optimasi Server',
      avatar: '🤖',
      status: 'online',
      unread_count: 1,
      updated_at: '2026-11-27T01:00:00Z',
      last_message: longMessage,
      last_message_at: '2026-11-27T01:00:00Z',
      resolved: false,
      task_count: 3,
      active_tasks: 1,
      queued_tasks: 1,
      review_tasks: 0,
      done_tasks: 1,
      canceled_tasks: 0,
      failed_tasks: 0,
      active_substage: 'Checking result',
    },
    {
      id: 'cust_2',
      conversation_id: 'room_2',
      type: 'customer',
      name: 'Rachmat Ajis',
      avatar: '',
      status: 'offline',
      unread_count: 0,
      updated_at: '2026-11-26T10:00:00Z',
      resolved: true,
      task_count: 1,
      done_tasks: 1,
      is_online: true,
    },
  ]

  writeInboxContactsCache('user_a', contacts)
  const hydrated = readInboxContactsCache('user_a')
  assert.ok(Array.isArray(hydrated))
  assert.equal(hydrated.length, 2)
  assert.equal(hydrated[0].active_tasks, 1)
  assert.equal(hydrated[0].queued_tasks, 1)
  assert.equal(hydrated[0].done_tasks, 1)
  assert.equal(hydrated[0].task_count, 3)
  assert.equal(hydrated[0].resolved, false)
  assert.equal(hydrated[0].active_substage, 'Checking result')
  assert.equal(hydrated[0].last_message.length, 240)
  assert.equal(hydrated[1].resolved, true)
  assert.equal(hydrated[1].is_online, true)
  assert.ok(localStorage.getItem(INBOX_CONTACTS_CACHE_KEY))
})

test('readInboxContactsCache enforces cross-account isolation and clears mismatched user snapshot', () => {
  localStorage.clear()
  clearInboxContactsCache()

  writeInboxContactsCache('user_a', [
    {
      id: 'room_a',
      conversation_id: 'room_a',
      type: 'agent',
      name: 'Private A',
      avatar: '',
      status: 'online',
      unread_count: 0,
      updated_at: '2026-11-27T01:00:00Z',
      active_tasks: 1,
    },
  ])

  assert.equal(readInboxContactsCache(undefined), undefined)
  assert.equal(readInboxContactsCache('user_b'), undefined)
  assert.equal(localStorage.getItem(INBOX_CONTACTS_CACHE_KEY), null)
})

test('clearInboxContactsCache clears both inbox contacts cache and room chat caches', () => {
  localStorage.clear()
  sessionStorage.clear()
  clearInboxContactsCache()

  writeInboxContactsCache('user_a', [
    trimContactSnapshot({
      id: 'room_1',
      conversation_id: 'room_1',
      type: 'agent',
      name: 'Room 1',
      avatar: '',
      status: 'online',
      unread_count: 0,
      updated_at: '2026-11-27T01:00:00Z',
    }),
  ])
  sessionStorage.setItem('cuan_chat_cache:room_1', '{"messages":[]}')
  sessionStorage.setItem('unrelated_key', 'keep')

  clearInboxContactsCache()

  assert.equal(localStorage.getItem(INBOX_CONTACTS_CACHE_KEY), null)
  assert.equal(sessionStorage.getItem('cuan_chat_cache:room_1'), null)
  assert.equal(sessionStorage.getItem('unrelated_key'), 'keep')
  assert.equal(readInboxContactsCache('user_a'), undefined)
})

test('App.tsx hydrates contacts from cache and revalidates in background; AuthContext.tsx clears cache on logout and 401', () => {
  const appSource = fs.readFileSync(path.resolve(__dirname, '../src/App.tsx'), 'utf8')
  const authSource = fs.readFileSync(path.resolve(__dirname, '../src/context/AuthContext.tsx'), 'utf8')

  assert.match(appSource, /readInboxContactsCache\(user\?\.id\)/)
  assert.match(appSource, /writeInboxContactsCache\(userIdRef\.current,\s*merged\)/)
  assert.match(authSource, /clearInboxContactsCache\(\)/)
  // Both 401 and logout paths must call clearInboxContactsCache()
  const clearCalls = authSource.match(/clearInboxContactsCache\(\)/g) || []
  assert.ok(clearCalls.length >= 2, 'Expected clearInboxContactsCache() in both 401 and logout handlers')
})
