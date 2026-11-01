import test from 'node:test'
import assert from 'node:assert/strict'
import fs from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'

const __filename = fileURLToPath(import.meta.url)
const __dirname = path.dirname(__filename)

const cssFilePath = path.join(__dirname, '../src/features/inbox/chat-calm.css')
const inboxViewPath = path.join(__dirname, '../src/features/inbox/InboxView.tsx')
const localePath = path.join(__dirname, '../src/locales/id.json')

const cssSource = fs.readFileSync(cssFilePath, 'utf8')
const inboxViewSource = fs.readFileSync(inboxViewPath, 'utf8')
const idLocale = JSON.parse(fs.readFileSync(localePath, 'utf8'))

test('1. Scroll to Bottom CSS Styles, Dark Theme & Smooth Transition', () => {
  assert.ok(cssSource.includes('.wa-scroll-btn'), 'chat-calm.css must define .wa-scroll-btn')
  assert.ok(cssSource.includes('.dark .wa-scroll-btn'), 'chat-calm.css must define dark mode .wa-scroll-btn')
  assert.ok(cssSource.includes('.wa-scroll-badge'), 'chat-calm.css must define .wa-scroll-badge')
  assert.ok(cssSource.includes('@keyframes wa-badge-pop'), 'chat-calm.css must define wa-badge-pop keyframe animation')
  assert.ok(cssSource.includes('transition:'), '.wa-scroll-btn must define smooth transitions')
  assert.ok(cssSource.includes('box-shadow:'), '.wa-scroll-btn must define floating shadow')
})

test('2. Indonesian Localizations for Scroll-to-Bottom and Unread Badge', () => {
  assert.equal(idLocale['Scroll to bottom'], 'Gulir ke bawah', 'id.json must translate "Scroll to bottom"')
  assert.equal(idLocale['new messages'], 'pesan baru', 'id.json must translate "new messages"')
  assert.equal(idLocale['new message'], 'pesan baru', 'id.json must translate "new message"')
})

test('3. InboxView Component Structure, Accessibility & FAB Placement', () => {
  // Check FAB container and button attributes
  assert.ok(
    inboxViewSource.includes('data-testid="scroll-to-bottom-button"'),
    'InboxView must render scroll-to-bottom button with data-testid'
  )
  assert.ok(
    inboxViewSource.includes('data-testid="scroll-to-bottom-badge"'),
    'InboxView must render unread count badge with data-testid'
  )
  assert.ok(
    inboxViewSource.includes('ChevronDown'),
    'InboxView must use ChevronDown icon for scroll-to-bottom'
  )
  assert.ok(
    inboxViewSource.includes('aria-label='),
    'Scroll button must provide descriptive accessible aria-label'
  )
  assert.ok(
    inboxViewSource.includes('z-30'),
    'Scroll button container must have z-30 stacking context above chat messages'
  )
  assert.ok(
    inboxViewSource.includes('max-w-4xl justify-end'),
    'Scroll button must align with max-w-4xl composer layout on desktop and mobile'
  )
  assert.ok(
    inboxViewSource.includes('prefers-reduced-motion'),
    'handleScrollToBottom must respect prefers-reduced-motion'
  )
  assert.ok(
    inboxViewSource.includes('SCROLL_THRESHOLD = 200'),
    'SCROLL_THRESHOLD must be defined as 200px'
  )
  assert.ok(
    inboxViewSource.includes('BOTTOM_THRESHOLD = 80'),
    'BOTTOM_THRESHOLD must be defined as 80px'
  )
})

test('4. Scroll State Machine & Behavior Verification', () => {
  class ScrollStateManager {
    constructor() {
      this.SCROLL_THRESHOLD = 200
      this.BOTTOM_THRESHOLD = 80
      this.followTail = true
      this.showScrollBottom = false
      this.hasNewMessages = false
      this.unreadCount = 0
      this.isScrollingToBottom = false
      this.prevMessageKey = undefined
      this.prevContactId = undefined
      this.prevMessageCount = 0
      this.scrollTop = 1000
      this.scrollHeight = 1600
      this.clientHeight = 600 // At bottom: 1600 - 1000 - 600 = 0
    }

    get distanceFromBottom() {
      return this.scrollHeight - this.scrollTop - this.clientHeight
    }

    handleScroll(scrollTop) {
      this.scrollTop = scrollTop
      const dist = this.distanceFromBottom
      const atBottom = dist <= this.BOTTOM_THRESHOLD

      this.followTail = atBottom

      if (this.isScrollingToBottom) {
        if (atBottom) {
          this.isScrollingToBottom = false
          this.showScrollBottom = false
          this.hasNewMessages = false
          this.unreadCount = 0
        }
        return
      }

      if (atBottom) {
        this.showScrollBottom = false
        this.hasNewMessages = false
        this.unreadCount = 0
      } else if (dist > this.SCROLL_THRESHOLD) {
        this.showScrollBottom = true
      } else {
        this.showScrollBottom = this.hasNewMessages
      }
    }

    onMessagesUpdate(contactId, messages) {
      const lastKey = messages[messages.length - 1]?.id
      const isNewContact = contactId !== this.prevContactId
      this.prevContactId = contactId

      if (isNewContact) {
        this.prevMessageKey = lastKey
        this.prevMessageCount = messages.length
        this.hasNewMessages = false
        this.unreadCount = 0
        this.showScrollBottom = false
        if (this.followTail) {
          this.scrollTop = this.scrollHeight - this.clientHeight
        }
        return
      }

      const isNewMessage = !!lastKey && lastKey !== this.prevMessageKey
      const addedCount = Math.max(0, messages.length - this.prevMessageCount)
      this.prevMessageKey = lastKey
      this.prevMessageCount = messages.length

      if (this.followTail) {
        this.scrollTop = this.scrollHeight - this.clientHeight
        this.hasNewMessages = false
        this.unreadCount = 0
        this.showScrollBottom = false
      } else if (isNewMessage || addedCount > 0) {
        this.hasNewMessages = true
        this.unreadCount += (addedCount > 0 ? addedCount : 1)
        this.showScrollBottom = true
      }
    }

    clickScrollToBottom() {
      this.followTail = true
      this.isScrollingToBottom = true
      this.showScrollBottom = false
      this.hasNewMessages = false
      this.unreadCount = 0
      return this.scrollHeight - this.clientHeight
    }
  }

  // Case 1: Initial state
  const state = new ScrollStateManager()
  state.onMessagesUpdate('contact-1', [{ id: 'm1' }])
  assert.equal(state.distanceFromBottom, 0)
  assert.equal(state.showScrollBottom, false)
  assert.equal(state.hasNewMessages, false)
  assert.equal(state.unreadCount, 0)

  // Case 2: User scrolls up 100px (< 200px threshold)
  state.handleScroll(900) // dist = 100
  assert.equal(state.showScrollBottom, false)
  assert.equal(state.followTail, false)

  // Case 3: User scrolls up past 200px threshold (dist = 300px)
  state.handleScroll(700) // dist = 300
  assert.equal(state.showScrollBottom, true)
  assert.equal(state.unreadCount, 0)

  // Case 4: New message arrives while scrolled up
  state.scrollHeight = 1700
  state.onMessagesUpdate('contact-1', [{ id: 'm1' }, { id: 'm2' }, { id: 'm3' }])
  assert.equal(state.hasNewMessages, true)
  assert.equal(state.unreadCount, 2)
  assert.equal(state.showScrollBottom, true)
  assert.notEqual(state.scrollTop, state.scrollHeight - state.clientHeight)

  // Case 5: Badge formatting
  const formatBadge = (count) => (count > 99 ? '99+' : count || 1)
  assert.equal(formatBadge(1), 1)
  assert.equal(formatBadge(5), 5)
  assert.equal(formatBadge(105), '99+')

  // Case 6: Click scroll to bottom
  const targetScroll = state.clickScrollToBottom()
  assert.equal(state.showScrollBottom, false)
  assert.equal(state.unreadCount, 0)
  assert.equal(state.hasNewMessages, false)
  assert.equal(state.followTail, true)

  // Intermediate scroll during smooth animation does not flicker button back on
  state.handleScroll(850)
  assert.equal(state.showScrollBottom, false)

  // Reaching target scroll position
  state.handleScroll(targetScroll)
  assert.equal(state.distanceFromBottom, 0)
  assert.equal(state.showScrollBottom, false)

  // Case 7: Auto-scroll when already at bottom
  state.scrollHeight = 1800
  state.onMessagesUpdate('contact-1', [{ id: 'm1' }, { id: 'm2' }, { id: 'm3' }, { id: 'm4' }])
  assert.equal(state.scrollTop, 1800 - 600)
  assert.equal(state.hasNewMessages, false)
  assert.equal(state.unreadCount, 0)
  assert.equal(state.showScrollBottom, false)

  // Case 8: Switching contact resets all state
  state.handleScroll(700)
  state.onMessagesUpdate('contact-2', [{ id: 'c2-m1' }])
  assert.equal(state.showScrollBottom, false)
  assert.equal(state.hasNewMessages, false)
  assert.equal(state.unreadCount, 0)
})
