import test from 'node:test'
import assert from 'node:assert/strict'
import fs from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'

const __dirname = path.dirname(fileURLToPath(import.meta.url))
const rootDir = path.resolve(__dirname, '..')

test('1. TaskTerminal has Drag & Drop and Clipboard Paste event listeners', () => {
  const terminalPath = path.join(
    rootDir,
    'src/features/tasks/TaskTerminal.tsx',
  )
  assert.ok(fs.existsSync(terminalPath), 'TaskTerminal.tsx must exist')
  const content = fs.readFileSync(terminalPath, 'utf8')

  assert.match(content, /onDragEnter=\{handleDragEnter\}/, 'Must listen to onDragEnter on container')
  assert.match(content, /onDragOver=\{handleDragOver\}/, 'Must listen to onDragOver on container')
  assert.match(content, /onDragLeave=\{handleDragLeave\}/, 'Must listen to onDragLeave on container')
  assert.match(content, /onDrop=\{handleDrop\}/, 'Must listen to onDrop on container')
  assert.match(content, /onPaste=\{handleContainerPaste\}/, 'Must listen to onPaste on container for screenshot pastes')
})

test('2. Calm, Zero-Visual-Noise Drop Zone Overlay in TaskTerminal', () => {
  const terminalPath = path.join(
    rootDir,
    'src/features/tasks/TaskTerminal.tsx',
  )
  const content = fs.readFileSync(terminalPath, 'utf8')

  // Check drop overlay elements and styling adhering to Bear Notes / Things 3 standards
  assert.match(content, /isDraggingOver\s*&&/, 'Must conditionally render overlay when isDraggingOver is true')
  assert.match(content, /border-dashed\s+border-emerald-500\/60/, 'Must use subtle emerald dashed border')
  assert.match(content, /backdrop-blur/, 'Must use calm backdrop blur')
  assert.match(content, /Drop files to upload to workspace/, 'Must show localized drop guidance text')
  assert.match(content, /pointer-events-none/, 'Overlay must have pointer-events-none to prevent drag flicker')
})

test('3. Upload Progress Indicator in TaskTerminal', () => {
  const terminalPath = path.join(
    rootDir,
    'src/features/tasks/TaskTerminal.tsx',
  )
  const content = fs.readFileSync(terminalPath, 'utf8')

  assert.match(content, /uploadingFiles\s*&&/, 'Must render uploading status when uploadingFiles is true')
  assert.match(content, /Uploading file to workspace…/, 'Must display localized uploading text')
})

test('4. Automatic Path Injection into Terminal Stdin', () => {
  const terminalPath = path.join(
    rootDir,
    'src/features/tasks/TaskTerminal.tsx',
  )
  const content = fs.readFileSync(terminalPath, 'utf8')

  // Filenames with spaces should be quoted
  assert.match(content, /f\.name\.includes\('\s*'\)\s*\?\s*`"\.\/\$\{f\.name\}"`\s*:\s*`\.\/\$\{f\.name\}`/, 'Must wrap filenames containing spaces in quotes')
  assert.match(content, /sendKey\(paths\s*\+\s*'\s*'\)/, 'Must send injected paths with trailing space into terminal stdin')
})

test('5. TerminalComposer Drag & Drop & Imperative Handle Integration', () => {
  const composerPath = path.join(
    rootDir,
    'src/features/tasks/terminal/TerminalComposer.tsx',
  )
  const terminalPath = path.join(
    rootDir,
    'src/features/tasks/TaskTerminal.tsx',
  )
  const composerContent = fs.readFileSync(composerPath, 'utf8')
  const terminalContent = fs.readFileSync(terminalPath, 'utf8')

  assert.match(composerContent, /export interface TerminalComposerHandle/, 'Must export TerminalComposerHandle interface')
  assert.match(composerContent, /useImperativeHandle\(\s*composerRef/, 'Must bind handle to composerRef')
  assert.match(composerContent, /onDragOver=/, 'TerminalComposer container must handle onDragOver')
  assert.match(composerContent, /onDrop=/, 'TerminalComposer container must handle onDrop')

  // TaskTerminal wires composerRef and delegates upload when composer is open
  assert.match(terminalContent, /composerRef=\{composerRef\}/, 'TaskTerminal must pass composerRef to TerminalComposer')
  assert.match(terminalContent, /showComposer\s*&&\s*composerRef\.current/, 'TaskTerminal must forward dropped files to composer if open')
})

test('6. Localization: id.json and en.json contain all drag-and-drop strings', () => {
  const idLocalePath = path.join(rootDir, 'src/locales/id.json')
  const enLocalePath = path.join(rootDir, 'src/locales/en.json')
  const idLocale = JSON.parse(fs.readFileSync(idLocalePath, 'utf8'))
  const enLocale = JSON.parse(fs.readFileSync(enLocalePath, 'utf8'))

  const requiredKeys = [
    'Drop files to upload to workspace',
    'Files will be uploaded and path inserted',
    'Uploading file to workspace…',
    'File uploaded to workspace',
    'Failed to upload file to workspace',
  ]

  for (const key of requiredKeys) {
    assert.ok(idLocale[key], `id.json must contain translation for "${key}"`)
    assert.ok(enLocale[key], `en.json must contain translation for "${key}"`)
  }
})
