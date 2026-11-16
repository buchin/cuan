import test from 'node:test'
import assert from 'node:assert/strict'
import fs from 'node:fs'
import ts from 'typescript'

function loadTs(relativePath, base = import.meta.url) {
  const fileUrl = new URL(relativePath, base)
  const source = fs.readFileSync(fileUrl, 'utf8')
  const compiled = ts.transpileModule(source, {
    compilerOptions: { module: ts.ModuleKind.CommonJS, target: ts.ScriptTarget.ES2022 },
  }).outputText
  const m = { exports: {} }
  const req = (id) => {
    if (id.startsWith('./') || id.startsWith('../')) {
      const resolved = id.endsWith('.ts') ? id : id + '.ts'
      return loadTs(resolved, fileUrl)
    }
    return {}
  }
  const fn = new Function('module', 'exports', 'require', compiled)
  fn(m, m.exports, req)
  return m.exports
}

const { sanitizeTerminalOutput } = loadTs(
  '../src/features/tasks/terminal/sanitize-output.ts',
)
const { TerminalStreamDecoder } = loadTs(
  '../src/features/tasks/terminal/stream-decoder.ts',
)
const {
  createTerminalWriteQueue,
  trimPendingTail,
  TERMINAL_LIVE_WRITE_CHUNK_CHARS,
  TERMINAL_HIDDEN_BACKLOG_CAP,
} = loadTs('../src/features/tasks/terminal/write-queue.ts')

test('sanitizeTerminalOutput replaces braille spinner frames with bullets', () => {
  const brailleChars = ['⠋', '⠙', '⠹', '⠸', '⠼', '⠴', '⠦', '⠧', '⠇', '⠏']
  for (const char of brailleChars) {
    const text = `Working ${char} please wait`
    const sanitized = sanitizeTerminalOutput(text)
    assert(!sanitized.includes('⠇'), `braille character ${char} should be sanitized`)
    assert(!sanitized.includes(char), `braille character ${char} should be sanitized`)
    assert(sanitized.includes('•'), 'should contain bullet')
  }
})

test('sanitizeTerminalOutput normalizes missing Nerd Font glyphs and dingbats', () => {
  const recordCircle = 'Status: \u23FA\uFE0F recording'
  assert.equal(sanitizeTerminalOutput(recordCircle), 'Status: \u25CF recording')

  const arrow = 'Down: \u21E3'
  assert.equal(sanitizeTerminalOutput(arrow), 'Down: \u2193')

  const vertBar = 'Border: \u2759'
  assert.equal(sanitizeTerminalOutput(vertBar), 'Border: \u2503')
})

test('TerminalStreamDecoder preserves multibyte UTF-8 across chunks', () => {
  const decoder = new TerminalStreamDecoder()
  const text = 'Terminal 日本語 🚀 test'
  const bytes = new TextEncoder().encode(text)

  // Split in the middle of a 3-byte or 4-byte UTF-8 character
  const split1 = 11 // inside '日'
  const split2 = 19 // inside emoji

  const chunk1 = bytes.slice(0, split1)
  const chunk2 = bytes.slice(split1, split2)
  const chunk3 = bytes.slice(split2)

  const out1 = decoder.decode(chunk1)
  const out2 = decoder.decode(chunk2)
  const out3 = decoder.decode(chunk3)

  assert.equal(out1 + out2 + out3, text)
})

test('trimPendingTail cuts at safe escape boundaries and caps length', () => {
  const str = 'abcdefghij\x1b[32mGREEN\x1b[0m'
  const trimmed = trimPendingTail(str, 12)
  assert(trimmed.length <= 12)
  assert(trimmed.startsWith('\x1b') || trimmed.length <= 12)

  const unclosed = 'leading-noise-filler-characters-'.repeat(2) + 'abc\x1b[?2026hdef'
  const repaired = trimPendingTail(unclosed, 20)
  assert(repaired.includes('\x1b[?2026l'), 'unpaired DEC synchronized-update should be closed')
})

test('createTerminalWriteQueue chunks rapid output and yields without locking main thread', async () => {
  const writes = []
  const mockTerm = {
    write(data, cb) {
      writes.push(data)
      cb?.()
    },
  }

  const queue = createTerminalWriteQueue(mockTerm, {
    chunkSize: 64,
  })

  // Enqueue 200 chars
  const data = 'A'.repeat(200)
  queue.enqueue(data)

  // Wait a tick for microtask/MessageChannel pump
  await new Promise((resolve) => setTimeout(resolve, 50))

  assert(writes.length > 1, 'should slice 200 chars into multiple chunks')
  assert.equal(writes.join(''), data)
})
