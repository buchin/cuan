import test from 'node:test'
import assert from 'node:assert/strict'
import fs from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'

const __dirname = path.dirname(fileURLToPath(import.meta.url))
const rootDir = path.resolve(__dirname, '..')

test('1. TerminalComposer component exists and provides universal composer capsule', () => {
  const composerPath = path.join(
    rootDir,
    'src/features/tasks/terminal/TerminalComposer.tsx',
  )
  assert.ok(fs.existsSync(composerPath), 'TerminalComposer.tsx must exist')
  const content = fs.readFileSync(composerPath, 'utf8')

  assert.match(content, /export function TerminalComposer/, 'Must export TerminalComposer component')
  assert.match(content, /handleSend/, 'Must have handleSend logic')
  assert.match(content, /handleKeyDown/, 'Must handle keyboard navigation and submission')
})

test('2. Anti-Jitter configuration in TaskTerminal (Adopted from Fural)', () => {
  const terminalPath = path.join(
    rootDir,
    'src/features/tasks/TaskTerminal.tsx',
  )
  const content = fs.readFileSync(terminalPath, 'utf8')

  assert.match(content, /scrollOnUserInput:\s*false/, 'TaskTerminal must set scrollOnUserInput: false to prevent cursor jumping')
  assert.match(content, /reflowCursorLine:\s*true/, 'TaskTerminal must set reflowCursorLine: true for prompt stabilization')
  assert.match(content, /<TerminalComposer/, 'TaskTerminal must render universal TerminalComposer')
})

test('3. Workspace File Upload in Terminal Composer', () => {
  const composerPath = path.join(
    rootDir,
    'src/features/tasks/terminal/TerminalComposer.tsx',
  )
  const content = fs.readFileSync(composerPath, 'utf8')

  assert.match(content, /Paperclip/, 'Must render Paperclip upload button')
  assert.match(content, /handleFileUpload/, 'Must have handleFileUpload handler')
  assert.match(content, /\/api\/v1\/tasks\/.*\/files/, 'Must upload files to task files endpoint')
  assert.match(content, /uploadedFiles/, 'Must track uploaded file state')
})

test('4. Interactive Slash Commands in Terminal Composer (/usage, /upload, /diff, etc.)', () => {
  const composerPath = path.join(
    rootDir,
    'src/features/tasks/terminal/TerminalComposer.tsx',
  )
  const content = fs.readFileSync(composerPath, 'utf8')

  assert.match(content, /\/usage/, 'Must support /usage slash command')
  assert.match(content, /\/upload/, 'Must support /upload slash command')
  assert.match(content, /\/diff/, 'Must support /diff slash command')
  assert.match(content, /\/files/, 'Must support /files slash command')
  assert.match(content, /\/clear/, 'Must support /clear slash command')
  assert.match(content, /\/stop/, 'Must support /stop slash command')

  // Autocomplete popup
  assert.match(content, /showSlashPopup/, 'Must support floating slash autocomplete popup')
  assert.match(content, /filteredCommands/, 'Must filter slash commands based on user query')
})

test('5. Quick Action Chips (y/n, Ctrl+C, Clear, Tab, Kbd)', () => {
  const composerPath = path.join(
    rootDir,
    'src/features/tasks/terminal/TerminalComposer.tsx',
  )
  const content = fs.readFileSync(composerPath, 'utf8')

  assert.match(content, /y \(Yes\)/, 'Must render y (Yes) quick chip')
  assert.match(content, /n \(No\)/, 'Must render n (No) quick chip')
  assert.match(content, /Ctrl\+C/, 'Must render Ctrl+C stop quick chip')
  assert.match(content, /Clear/, 'Must render Clear screen quick chip')
  assert.match(content, /Tab/, 'Must render Tab autocomplete quick chip')
})
