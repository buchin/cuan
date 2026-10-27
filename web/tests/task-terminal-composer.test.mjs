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
  assert.match(content, /onToggleMobileKbd/, 'Must render mobile Kbd toggle chip')
})

test('6. Single Unified Input on Mobile: No duplicate prompt form or floating collisions', () => {
  const mobileKbdPath = path.join(
    rootDir,
    'src/features/tasks/terminal/MobileTerminalKeyboard.tsx',
  )
  const terminalPath = path.join(
    rootDir,
    'src/features/tasks/TaskTerminal.tsx',
  )
  const mobileKbdContent = fs.readFileSync(mobileKbdPath, 'utf8')
  const terminalContent = fs.readFileSync(terminalPath, 'utf8')

  // MobileTerminalKeyboard must not render duplicate prompt bar by default
  assert.match(
    mobileKbdContent,
    /showPromptInput\s*=\s*false/,
    'MobileTerminalKeyboard must default showPromptInput to false to prevent duplicate input bars',
  )

  // TaskTerminal must not render floating bottom-right Kbd button over composer
  assert.ok(
    !terminalContent.includes('absolute bottom-3 right-3'),
    'TaskTerminal must not render overlapping floating button at bottom-3 right-3',
  )
})

test('7. Inbox-Style Upload (Camera, Photos, Files) & WhatsApp Input Styling', () => {
  const composerPath = path.join(
    rootDir,
    'src/features/tasks/terminal/TerminalComposer.tsx',
  )
  const content = fs.readFileSync(composerPath, 'utf8')

  // Inbox style classnames
  assert.match(content, /wa-input-capsule/, 'Must use wa-input-capsule class')
  assert.match(content, /wa-input/, 'Must use wa-input class')
  assert.match(content, /wa-send/, 'Must use wa-send class')
  assert.match(content, /wa-chat/, 'Must use wa-chat palette class')

  // Attachment choices: Camera, Photos, Files
  assert.match(content, /capture="environment"/, 'Must support camera capture')
  assert.match(content, /Camera/, 'Must render Camera attachment option')
  assert.match(content, /Images/, 'Must render Photos attachment option')
  assert.match(content, /FolderOpen/, 'Must render Files attachment option')

  // Attachment Previews strip
  assert.match(content, /previewUrl/, 'Must generate previewUrl for image thumbnails')
  assert.match(content, /handleRemoveFile/, 'Must allow removing attachment items')
})

test('8. Pin Terminal Toggle & Auto-Close Prevention in TaskSessionPage', () => {
  const sessionPagePath = path.join(
    rootDir,
    'src/features/tasks/TaskSessionPage.tsx',
  )
  const content = fs.readFileSync(sessionPagePath, 'utf8')

  // Must import Pin and PinOff
  assert.match(content, /import\s*\{[^}]*\bPin\b[^}]*\}\s*from\s*'lucide-react'/, 'Must import Pin from lucide-react')
  assert.match(content, /import\s*\{[^}]*\bPinOff\b[^}]*\}\s*from\s*'lucide-react'/, 'Must import PinOff from lucide-react')

  // Must have pinTerminal state persisted via localStorage
  assert.match(content, /cuan_terminal_pin/, 'Must persist pin preference in cuan_terminal_pin localStorage key')
  assert.match(content, /pinTerminal,\s*setPinTerminal/, 'Must manage pinTerminal state')

  // Auto-close prevention when task completes
  assert.match(content, /prevActiveRef\.current\s*&&\s*!active\s*&&\s*pinTerminal/, 'Must keep terminal open when task transitions to completed if pinned')

  // Pin toggle button in navbar
  assert.match(content, /handleTogglePin/, 'Must provide handleTogglePin toggle handler')
  assert.match(content, /<PinOff\b/, 'Must render PinOff icon when unpinned')
  assert.match(content, /<Pin\b/, 'Must render Pin icon when pinned')

  // Pinned notification banner
  assert.match(content, /Terminal pinned · Work finished/, 'Must render pinned banner with status and saved output')
})

