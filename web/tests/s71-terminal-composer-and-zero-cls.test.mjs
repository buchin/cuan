import test from 'node:test'
import assert from 'node:assert/strict'
import fs from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'

const __dirname = path.dirname(fileURLToPath(import.meta.url))
const rootDir = path.resolve(__dirname, '..')

test('S71: TerminalComposer matches Inbox composer simplicity and capsule styling', () => {
  const composerPath = path.join(rootDir, 'src/features/tasks/terminal/TerminalComposer.tsx')
  assert.ok(fs.existsSync(composerPath), 'TerminalComposer.tsx must exist')
  const content = fs.readFileSync(composerPath, 'utf8')

  // Must import shared Composer capsule
  assert.match(content, /import\s*\{[^}]*Composer[^}]*\}\s*from\s*['"].*Composer['"]/, 'Must import shared Composer')
  assert.match(content, /import\s*\{[^}]*ComposerSendButton[^}]*\}\s*from\s*['"].*Composer['"]/, 'Must import ComposerSendButton')

  // Capsule must delegate Send and Voice to ComposerSendButton without an intrusive stop/Ban icon inside the input capsule
  assert.doesNotMatch(
    content.slice(content.indexOf('<Composer role="group"'), content.indexOf('</Composer>')),
    /<Ban\b/,
    'The input capsule in TerminalComposer must not contain a red Ban/stop icon (matches Inbox calm design)'
  )

  // Multiline expansion support matching Inbox
  assert.match(content, /text\.includes\('\\n'\)\s*\?\s*'is-multiline'\s*:\s*''/, 'Must support is-multiline class on Composer')

  // Responsive centered width constraint matching Inbox max-w-[75ch]
  assert.match(content, /max-w-\[75ch\]/, 'Must apply max-w-[75ch] container constraint')
})

test('S71: Zero-CLS layout stabilization in TaskTerminal (notice floats without height disruption)', () => {
  const terminalPath = path.join(rootDir, 'src/features/tasks/TaskTerminal.tsx')
  assert.ok(fs.existsSync(terminalPath), 'TaskTerminal.tsx must exist')
  const content = fs.readFileSync(terminalPath, 'utf8')

  // Notice must not be a layout-pushing block element directly in the vertical flex container
  assert.doesNotMatch(
    content,
    /<p[^>]*role="status"[^>]*className="[^"]*shrink-0[^"]*"[^>]*>\s*\{notice\}\s*<\/p>/,
    'TaskTerminal must not render notice as a vertical shrink-0 block pushing down the xterm canvas'
  )

  // Notice must be positioned as a floating overlay inside the relative canvas container
  assert.match(
    content,
    /absolute top-3 right-3 z-30 inline-flex items-center/,
    'TaskTerminal must render notice as a gentle floating overlay without layout shift'
  )
})

test('S71: Buffer preservation on sameSession reconnect in TaskTerminal', () => {
  const terminalPath = path.join(rootDir, 'src/features/tasks/TaskTerminal.tsx')
  const content = fs.readFileSync(terminalPath, 'utf8')

  // Must only reset terminal when !sameSession, preserving scrollback buffer during reconnection blips
  assert.match(
    content,
    /if\s*\(!sameSession\)\s*\{\s*t\.reset\(\)/,
    'TaskTerminal must preserve buffer on sameSession reconnect instead of wiping with unconditional t.reset()'
  )
})

test('S71: TaskSessionPage suppresses error_message warning button while session is active', () => {
  const sessionPagePath = path.join(rootDir, 'src/features/tasks/TaskSessionPage.tsx')
  assert.ok(fs.existsSync(sessionPagePath), 'TaskSessionPage.tsx must exist')
  const content = fs.readFileSync(sessionPagePath, 'utf8')

  // Must only render attention banner when session is not active
  assert.match(
    content,
    /\{!active\s*&&\s*attempt\.error_message\s*&&/,
    'TaskSessionPage must suppress Session needs attention banner while active session is running'
  )
})
