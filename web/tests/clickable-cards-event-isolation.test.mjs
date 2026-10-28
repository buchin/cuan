import test from 'node:test'
import assert from 'node:assert/strict'
import fs from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'

const __dirname = path.dirname(fileURLToPath(import.meta.url))
const taskCardPath = path.resolve(__dirname, '../src/features/inbox/TaskCard.tsx')
const roomCardPath = path.resolve(__dirname, '../src/features/inbox/RoomCard.tsx')
const taskItemCardPath = path.resolve(__dirname, '../src/features/tasks/TaskItemCard.tsx')
const chatMarkdownPath = path.resolve(__dirname, '../src/features/inbox/ChatMarkdown.tsx')
const messageContentPath = path.resolve(__dirname, '../src/features/inbox/MessageContent.tsx')
const idLocalePath = path.resolve(__dirname, '../src/locales/id.json')

test('1. TaskCard Event Bubbling Isolation (e.stopPropagation & e.preventDefault)', () => {
  const src = fs.readFileSync(taskCardPath, 'utf8')

  // Check handleCardClick on container
  assert.ok(
    src.includes('const handleCardClick ='),
    'TaskCard must have handleCardClick for card-level navigation'
  )
  assert.ok(
    src.includes('onClick={handleCardClick}'),
    'TaskCard container must trigger handleCardClick'
  )
  assert.ok(
    src.includes('onKeyDown={handleKeyDown}'),
    'TaskCard container must support keyboard accessibility'
  )

  // Verify all inner button handlers stop propagation and prevent default
  assert.ok(
    src.includes('handleCopyLink'),
    'TaskCard must define handleCopyLink'
  )
  assert.ok(
    src.includes('e.stopPropagation()') && src.includes('e.preventDefault()'),
    'Inner button handlers must call e.stopPropagation() and e.preventDefault()'
  )

  // Check specific handlers for stopPropagation
  const copyLinkMatch = src.match(/const handleCopyLink = [^{]+{([^}]+)}/)
  assert.ok(copyLinkMatch, 'handleCopyLink function body must exist')
  assert.ok(
    copyLinkMatch[1].includes('e.stopPropagation()'),
    'handleCopyLink must call e.stopPropagation()'
  )
  assert.ok(
    copyLinkMatch[1].includes('e.preventDefault()'),
    'handleCopyLink must call e.preventDefault()'
  )

  const actionClickMatch = src.match(/const handleActionClick = [^{]+{([\s\S]+?)\n  }/)
  assert.ok(actionClickMatch, 'handleActionClick function body must exist')
  assert.ok(
    actionClickMatch[1].includes('e.stopPropagation()'),
    'handleActionClick must call e.stopPropagation()'
  )
  assert.ok(
    actionClickMatch[1].includes('e.preventDefault()'),
    'handleActionClick must call e.preventDefault()'
  )

  const retryMatch = src.match(/const handleRetry = [^{]+{([\s\S]+?)\n  }/)
  assert.ok(retryMatch, 'handleRetry function body must exist')
  assert.ok(
    retryMatch[1].includes('e.stopPropagation()'),
    'handleRetry must call e.stopPropagation()'
  )
  assert.ok(
    retryMatch[1].includes('e.preventDefault()'),
    'handleRetry must call e.preventDefault()'
  )
})

test('2. RoomCard Event Bubbling Isolation (e.stopPropagation & e.preventDefault)', () => {
  const src = fs.readFileSync(roomCardPath, 'utf8')

  // Check handleCardClick on container
  assert.ok(
    src.includes('const handleCardClick ='),
    'RoomCard must have handleCardClick for card-level navigation'
  )
  assert.ok(
    src.includes('onClick={handleCardClick}'),
    'RoomCard container must trigger handleCardClick'
  )
  assert.ok(
    src.includes('onKeyDown={handleKeyDown}'),
    'RoomCard container must support keyboard accessibility'
  )

  // Verify inner button handlers stop propagation
  const copyLinkMatch = src.match(/const handleCopyLink = [^{]+{([^}]+)}/)
  assert.ok(copyLinkMatch, 'handleCopyLink function body must exist')
  assert.ok(
    copyLinkMatch[1].includes('e.stopPropagation()'),
    'handleCopyLink in RoomCard must call e.stopPropagation()'
  )
  assert.ok(
    copyLinkMatch[1].includes('e.preventDefault()'),
    'handleCopyLink in RoomCard must call e.preventDefault()'
  )

  const openRoomMatch = src.match(/const handleOpenRoom = [^{]+{([\s\S]+?)\n  }/)
  assert.ok(openRoomMatch, 'handleOpenRoom function body must exist')
  assert.ok(
    openRoomMatch[1].includes('e.stopPropagation()'),
    'handleOpenRoom must call e.stopPropagation()'
  )
  assert.ok(
    openRoomMatch[1].includes('e.preventDefault()'),
    'handleOpenRoom must call e.preventDefault()'
  )
})

test('3. WCAG 2.1 AA Touch Target Standard (>= 44x44px)', () => {
  const taskCardSrc = fs.readFileSync(taskCardPath, 'utf8')
  const roomCardSrc = fs.readFileSync(roomCardPath, 'utf8')
  const taskItemCardSrc = fs.readFileSync(taskItemCardPath, 'utf8')

  // TaskCard touch targets
  assert.ok(
    taskCardSrc.includes('min-h-[44px] min-w-[44px]'),
    'TaskCard inner buttons must meet min-h-[44px] min-w-[44px]'
  )
  assert.ok(
    taskCardSrc.includes('min-h-[44px]'),
    'TaskCard container must have min-h-[44px]'
  )

  // RoomCard touch targets
  assert.ok(
    roomCardSrc.includes('min-h-[44px] min-w-[44px]'),
    'RoomCard inner buttons must meet min-h-[44px] min-w-[44px]'
  )
  assert.ok(
    roomCardSrc.includes('min-h-[44px]'),
    'RoomCard container must have min-h-[44px]'
  )

  // TaskItemCard touch targets without overriding min-h-9
  assert.ok(
    !taskItemCardSrc.includes('min-h-11 min-w-[44px] min-h-9'),
    'TaskItemCard must not have conflicting min-h-9 overriding min-h-11'
  )
  assert.ok(
    taskItemCardSrc.includes('min-h-11 min-w-[44px]'),
    'TaskItemCard must have min-h-11 min-w-[44px] touch target standard'
  )
})

test('4. Mobile Viewport Constraints (360px - 390px responsive safety)', () => {
  const taskCardSrc = fs.readFileSync(taskCardPath, 'utf8')
  const roomCardSrc = fs.readFileSync(roomCardPath, 'utf8')

  // Both cards must be w-full max-w-full min-w-0 and break-words
  for (const [name, src] of [['TaskCard', taskCardSrc], ['RoomCard', roomCardSrc]]) {
    assert.ok(
      src.includes('w-full') && src.includes('max-w-full'),
      `${name} must be w-full and max-w-full to prevent horizontal layout blowout`
    )
    assert.ok(
      src.includes('break-words'),
      `${name} must use break-words to handle long titles safely`
    )
    assert.ok(
      src.includes('flex-wrap'),
      `${name} header and footer must use flex-wrap for 360px-390px mobile screens`
    )
  }
})

test('5. Deep Link Detection (/tasks/:id and /inbox/:id) in Chat', () => {
  const chatMarkdownSrc = fs.readFileSync(chatMarkdownPath, 'utf8')
  const messageContentSrc = fs.readFileSync(messageContentPath, 'utf8')

  // ChatMarkdown deep link integration
  assert.ok(
    chatMarkdownSrc.includes("import { TaskCard } from './TaskCard'"),
    'ChatMarkdown must import TaskCard'
  )
  assert.ok(
    chatMarkdownSrc.includes("import { RoomCard } from './RoomCard'"),
    'ChatMarkdown must import RoomCard'
  )
  assert.ok(
    chatMarkdownSrc.includes('/tasks/') || chatMarkdownSrc.includes('\\/tasks\\/'),
    'ChatMarkdown must detect /tasks/ links'
  )
  assert.ok(
    chatMarkdownSrc.includes('/inbox/') || chatMarkdownSrc.includes('\\/inbox\\/'),
    'ChatMarkdown must detect /inbox/ links'
  )
  assert.ok(
    chatMarkdownSrc.includes('<TaskCard taskId='),
    'ChatMarkdown must render TaskCard for task deep links'
  )
  assert.ok(
    chatMarkdownSrc.includes('<RoomCard roomId='),
    'ChatMarkdown must render RoomCard for inbox deep links'
  )

  // MessageContent deep link integration
  assert.ok(
    messageContentSrc.includes("import { TaskCard } from './TaskCard'"),
    'MessageContent must import TaskCard'
  )
  assert.ok(
    messageContentSrc.includes("import { RoomCard } from './RoomCard'"),
    'MessageContent must import RoomCard'
  )
  assert.ok(
    messageContentSrc.includes('<TaskCard key='),
    'MessageContent must render TaskCard for task action deep links'
  )
  assert.ok(
    messageContentSrc.includes('<RoomCard key='),
    'MessageContent must render RoomCard for room action deep links'
  )
})

test('6. Indonesian Locale Support for TaskCard and RoomCard', () => {
  const idLocale = JSON.parse(fs.readFileSync(idLocalePath, 'utf8'))

  assert.ok(idLocale['Customer Room'], 'Locale must have Customer Room translation')
  assert.ok(idLocale['Agent Room'], 'Locale must have Agent Room translation')
  assert.ok(idLocale['Open room'], 'Locale must have Open room translation')
  assert.ok(idLocale['Copy link'], 'Locale must have Copy link translation')
  assert.ok(idLocale['Participants'], 'Locale must have Participants translation')
  assert.ok(idLocale['Open task'], 'Locale must have Open task translation')
})
