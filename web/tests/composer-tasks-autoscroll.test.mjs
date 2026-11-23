import test from 'node:test'
import assert from 'node:assert/strict'
import fs from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'

const __filename = fileURLToPath(import.meta.url)
const __dirname = path.dirname(__filename)

const composerTasksPath = path.join(__dirname, '../src/features/inbox/ComposerTasks.tsx')
const composerTasksSource = fs.readFileSync(composerTasksPath, 'utf8')

test('1. ComposerTasks: Auto-scroll to bottom on disclosure transition (DoD 1)', () => {
  assert.ok(composerTasksSource.includes('panelRef = useRef<HTMLDivElement>(null)'), 'Must define panelRef for tasks panel')
  assert.ok(composerTasksSource.includes('ref={panelRef}'), 'Must bind panelRef to composer-tasks-panel element')
  assert.ok(composerTasksSource.includes('prevOpenRef = useRef(false)'), 'Must track previous open state to detect transition')
  assert.ok(composerTasksSource.includes('wasOpened = open && !prevOpenRef.current'), 'Must detect when tasks list opens')
  assert.ok(composerTasksSource.includes('scrollToBottom'), 'Must define scrollToBottom handler')
  assert.ok(composerTasksSource.includes('el.scrollHeight') || composerTasksSource.includes('panel.scrollHeight'), 'Must scroll to bottom of panel (scrollHeight)')
  assert.ok(composerTasksSource.includes('requestAnimationFrame'), 'Must schedule scroll after layout computation')
})

test('2. ComposerTasks: Smooth scroll with accessibility fallback (prefers-reduced-motion)', () => {
  assert.ok(
    composerTasksSource.includes('prefers-reduced-motion'),
    'Must respect user prefers-reduced-motion setting for smooth scroll'
  )
  assert.ok(
    composerTasksSource.includes("behavior = prefersReducedMotion ? 'auto' : 'smooth'"),
    'Must use smooth behavior unless reduced motion is preferred'
  )
  assert.ok(
    composerTasksSource.includes('typeof el.scrollTo === \'function\''),
    'Must check scrollTo availability with robust fallback to scrollTop'
  )
})

test('3. ComposerTasks: Last task item full visibility & breathing room (DoD 2)', () => {
  assert.ok(
    composerTasksSource.includes('scroll-pb-2'),
    'composer-tasks-panel must include scroll-pb-2 for scroll container padding'
  )
  assert.ok(
    composerTasksSource.includes('pb-2.5') || composerTasksSource.includes('p-4 sm:p-6'),
    'Timeline must have bottom padding to prevent last item from clipping against container border'
  )
})

test('4. ComposerTasks: Main chat scroll stability & ancestor isolation (DoD 3)', () => {
  // Must NOT use scrollIntoView which causes parent chat containers to scroll
  assert.ok(
    !composerTasksSource.includes('.scrollIntoView('),
    'Must avoid scrollIntoView to prevent disturbing parent chat history scroll position'
  )
  // Must scroll panel element directly
  assert.ok(
    composerTasksSource.includes('el.scrollTo({ top: el.scrollHeight'),
    'Must scroll panel container directly to keep outer conversation stream undisturbed'
  )
})
