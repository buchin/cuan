import test from 'node:test'
import assert from 'node:assert/strict'
import fs from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'

const __dirname = path.dirname(fileURLToPath(import.meta.url))
const roomCardPath = path.resolve(__dirname, '../src/features/inbox/RoomCard.tsx')
const chatPreviewCardPath = path.resolve(__dirname, '../src/features/inbox/ChatPreviewCard.tsx')
const roomLinkPath = path.resolve(__dirname, '../src/features/inbox/room-link.ts')
const chatMarkdownPath = path.resolve(__dirname, '../src/features/inbox/ChatMarkdown.tsx')
const messageContentPath = path.resolve(__dirname, '../src/features/inbox/MessageContent.tsx')
const appPath = path.resolve(__dirname, '../src/App.tsx')

test('1. Room Link Detection Parser: relative routes and internal host cuan formats', async () => {
  const { extractConversationRoomId, isCuanHost, isConversationRoute, extractTaskId } = await import(
    '../src/features/inbox/room-link.ts'
  )

  // Relative /inbox/:id and /chat/:id routes
  assert.equal(extractConversationRoomId('/inbox/room-123'), 'room-123')
  assert.equal(extractConversationRoomId('/inbox/room-123/'), 'room-123')
  assert.equal(extractConversationRoomId('/inbox/room-123?search=test#pos'), 'room-123')
  assert.equal(extractConversationRoomId('/chat/room-456'), 'room-456')
  assert.equal(extractConversationRoomId('/chat/room-456/'), 'room-456')
  assert.equal(extractConversationRoomId('/chat/room-456?filter=all'), 'room-456')

  // Cuan internal hosts
  assert.ok(isCuanHost('cuan.dojo.cc'))
  assert.ok(isCuanHost('staging.cuan.dojo.cc'))
  assert.ok(isCuanHost('localhost'))
  assert.ok(isCuanHost('127.0.0.1'))
  assert.ok(isCuanHost('app.cuan.id'))
  assert.ok(!isCuanHost('google.com'))
  assert.ok(!isCuanHost('evil-cuan.attacker.com'))
  assert.ok(!isCuanHost('github.com'))

  // Internal host absolute URLs
  assert.equal(extractConversationRoomId('https://cuan.dojo.cc/inbox/conv-abc'), 'conv-abc')
  assert.equal(extractConversationRoomId('https://cuan.dojo.cc/chat/conv-xyz'), 'conv-xyz')
  assert.equal(extractConversationRoomId('http://localhost:5173/inbox/conv-local'), 'conv-local')
  assert.equal(extractConversationRoomId('http://localhost:8080/chat/conv-8080'), 'conv-8080')
  assert.equal(extractConversationRoomId('http://127.0.0.1:8080/inbox/conv-ip'), 'conv-ip')
  assert.equal(extractConversationRoomId('https://app.cuan.id/chat/conv-id'), 'conv-id')

  // External URLs must NOT be detected as internal conversation cards
  assert.equal(extractConversationRoomId('https://google.com/chat/room-123'), null)
  assert.equal(extractConversationRoomId('https://github.com/inbox/room-123'), null)
  assert.equal(extractConversationRoomId('https://attacker.com/chat/room-123'), null)

  // Route check boolean helper
  assert.equal(isConversationRoute('/inbox/room-123'), true)
  assert.equal(isConversationRoute('/chat/room-456'), true)
  assert.equal(isConversationRoute('https://cuan.dojo.cc/inbox/room-789'), true)
  assert.equal(isConversationRoute('https://google.com/chat/room-123'), false)

  // Task links
  assert.equal(extractTaskId('/tasks/task-999'), 'task-999')
  assert.equal(extractTaskId('https://cuan.dojo.cc/tasks/task-888'), 'task-888')
  assert.equal(extractTaskId('https://attacker.com/tasks/task-777'), null)
})

test('2. ChatMarkdown & MessageContent integration: parser expands to /inbox, /chat, and internal host cuan', () => {
  const chatMarkdownSrc = fs.readFileSync(chatMarkdownPath, 'utf8')
  const messageContentSrc = fs.readFileSync(messageContentPath, 'utf8')

  // ChatMarkdown imports and usage
  assert.ok(
    chatMarkdownSrc.includes('extractConversationRoomId'),
    'ChatMarkdown must import and use extractConversationRoomId'
  )
  assert.ok(
    chatMarkdownSrc.includes('RoomCard') && chatMarkdownSrc.includes('ChatPreviewCard'),
    'ChatMarkdown imports RoomCard and ChatPreviewCard'
  )
  assert.ok(
    chatMarkdownSrc.includes('/chat/') || chatMarkdownSrc.includes('\\/chat\\/'),
    'ChatMarkdown must explicitly recognize /chat/ routes'
  )
  assert.ok(
    chatMarkdownSrc.includes('/inbox/') || chatMarkdownSrc.includes('\\/inbox\\/'),
    'ChatMarkdown must explicitly recognize /inbox/ routes'
  )
  assert.ok(
    chatMarkdownSrc.includes('autolinkConversationPaths') || chatMarkdownSrc.includes('prepareChatMarkdown'),
    'ChatMarkdown must autolink conversation paths'
  )

  // MessageContent imports and usage
  assert.ok(
    messageContentSrc.includes('extractConversationRoomId'),
    'MessageContent must import and use extractConversationRoomId'
  )
  assert.ok(
    messageContentSrc.includes('/chat/') || messageContentSrc.includes('\\/chat\\/'),
    'MessageContent must explicitly recognize /chat/ routes'
  )
  assert.ok(
    messageContentSrc.includes('/inbox/') || messageContentSrc.includes('\\/inbox\\/'),
    'MessageContent must explicitly recognize /inbox/ routes'
  )
})

test('3. RoomCard / ChatPreviewCard component architecture & direct navigation actions', () => {
  const roomCardSrc = fs.readFileSync(roomCardPath, 'utf8')
  const previewCardSrc = fs.readFileSync(chatPreviewCardPath, 'utf8')

  // Both RoomCard and ChatPreviewCard must be exported
  assert.ok(
    roomCardSrc.includes('export const ChatPreviewCard = RoomCard'),
    'RoomCard.tsx must export ChatPreviewCard alias'
  )
  assert.ok(
    previewCardSrc.includes('ChatPreviewCard'),
    'ChatPreviewCard.tsx must re-export ChatPreviewCard component'
  )

  // Displays room title / name
  assert.ok(
    roomCardSrc.includes('displayName') && roomCardSrc.includes('<h4'),
    'RoomCard must display room title / contact name'
  )

  // Displays status / last message snippet
  assert.ok(
    roomCardSrc.includes('contact.last_message') || roomCardSrc.includes('last_message'),
    'RoomCard must display brief snippet / last message'
  )
  assert.ok(
    roomCardSrc.includes('isOnline') || roomCardSrc.includes('online'),
    'RoomCard must display status indicators'
  )

  // Direct navigation action button
  assert.ok(
    roomCardSrc.includes('handleOpenRoom'),
    'RoomCard must have handleOpenRoom handler'
  )
  assert.ok(
    roomCardSrc.includes('t(\'Open room\')') || roomCardSrc.includes('Open room'),
    'RoomCard must have direct action button for Open room'
  )
  assert.ok(
    roomCardSrc.includes('t(\'Copy link\')') || roomCardSrc.includes('Copy link'),
    'RoomCard must have direct action button for Copy link'
  )
})

test('4. WCAG touch target accessibility (>= 44x44px) and mobile responsiveness (360px-390px)', () => {
  const roomCardSrc = fs.readFileSync(roomCardPath, 'utf8')

  // Touch target >= 44x44px
  assert.ok(
    roomCardSrc.includes('min-h-[44px] min-w-[44px]'),
    'All inner action buttons must have min 44x44px touch targets'
  )
  assert.ok(
    roomCardSrc.includes('min-h-[44px]'),
    'Card container must have min-h-[44px]'
  )

  // Mobile safety (360px-390px)
  assert.ok(
    roomCardSrc.includes('w-full') && roomCardSrc.includes('max-w-full') && roomCardSrc.includes('min-w-0'),
    'RoomCard must have w-full max-w-full min-w-0 to prevent horizontal scroll'
  )
  assert.ok(
    roomCardSrc.includes('break-words'),
    'RoomCard must use break-words to handle long titles safely'
  )
  assert.ok(
    roomCardSrc.includes('flex-wrap'),
    'RoomCard header and footer must use flex-wrap for 360px-390px mobile screens'
  )
})

test('5. Click event isolation (stopPropagation & preventDefault) preventing double navigation', () => {
  const roomCardSrc = fs.readFileSync(roomCardPath, 'utf8')

  // Container click handler
  assert.ok(
    roomCardSrc.includes('const handleCardClick ='),
    'RoomCard must define handleCardClick'
  )
  assert.ok(
    roomCardSrc.includes('onClick={handleCardClick}'),
    'RoomCard container must bind onClick={handleCardClick}'
  )

  // Copy link button isolation
  const copyLinkMatch = roomCardSrc.match(/const handleCopyLink = [^{]+{([^}]+)}/)
  assert.ok(copyLinkMatch, 'handleCopyLink function must exist')
  assert.ok(
    copyLinkMatch[1].includes('e.stopPropagation()') && copyLinkMatch[1].includes('e.preventDefault()'),
    'handleCopyLink must isolate event bubbling with stopPropagation & preventDefault'
  )

  // Open room button isolation
  const openRoomMatch = roomCardSrc.match(/const handleOpenRoom = [^{]+{([\s\S]+?)\n  }/)
  assert.ok(openRoomMatch, 'handleOpenRoom function must exist')
  assert.ok(
    openRoomMatch[1].includes('e.stopPropagation()') && openRoomMatch[1].includes('e.preventDefault()'),
    'handleOpenRoom must isolate event bubbling with stopPropagation & preventDefault'
  )

  // Keyboard navigation isolation
  assert.ok(
    roomCardSrc.includes('if (e.target !== e.currentTarget) return'),
    'handleKeyDown must isolate inner keyboard focus from container navigation'
  )
  assert.ok(
    roomCardSrc.includes('onKeyDown={(e) => e.stopPropagation()}'),
    'Inner action buttons must isolate keydown events with stopPropagation'
  )
})

test('6. App.tsx deep link synchronization: /chat/:roomId routes to /inbox/:roomId', () => {
  const appSrc = fs.readFileSync(appPath, 'utf8')

  assert.ok(
    appSrc.includes('rootSegment === \'chat\' && subParam'),
    'App.tsx must handle /chat/:roomId route'
  )
  assert.ok(
    appSrc.includes('/inbox/\' + encodeURIComponent(subParam)'),
    'App.tsx must navigate to /inbox/:roomId for /chat/:roomId URLs'
  )
  assert.ok(
    appSrc.includes('rootSegment === \'inbox\' || rootSegment === \'chat\''),
    'App.tsx visibleContact must resolve for both /inbox and /chat routes'
  )
})
