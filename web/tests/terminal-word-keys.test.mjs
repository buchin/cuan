import test from 'node:test'
import assert from 'node:assert/strict'
import fs from 'node:fs'
import ts from 'typescript'

const source = fs.readFileSync(
  new URL('../src/features/tasks/terminal/word-keys.ts', import.meta.url),
  'utf8',
)
const compiled = ts.transpileModule(source, {
  compilerOptions: { module: ts.ModuleKind.ESNext },
}).outputText
const {
  desktopWordEditSequence,
  WORD_BACKSPACE,
  WORD_LEFT,
  WORD_RIGHT,
} = await import(
  'data:text/javascript;base64,' + Buffer.from(compiled).toString('base64')
)

test('desktopWordEditSequence: Alt+Left maps to readline backward-word (ESC b)', () => {
  const seq = desktopWordEditSequence({
    type: 'keydown',
    key: 'ArrowLeft',
    code: 'ArrowLeft',
    altKey: true,
    ctrlKey: false,
    shiftKey: false,
    metaKey: false,
  })
  assert.equal(seq, WORD_LEFT)
})

test('desktopWordEditSequence: Alt+Right maps to readline forward-word (ESC f)', () => {
  const seq = desktopWordEditSequence({
    type: 'keydown',
    key: 'ArrowRight',
    code: 'ArrowRight',
    altKey: true,
    ctrlKey: false,
    shiftKey: false,
    metaKey: false,
  })
  assert.equal(seq, WORD_RIGHT)
})

test('desktopWordEditSequence: Alt+Backspace and Ctrl+Backspace map to readline backward-kill-word (ESC DEL)', () => {
  const altBack = desktopWordEditSequence({
    type: 'keydown',
    key: 'Backspace',
    code: 'Backspace',
    altKey: true,
    ctrlKey: false,
    shiftKey: false,
    metaKey: false,
  })
  assert.equal(altBack, WORD_BACKSPACE)

  const ctrlBack = desktopWordEditSequence({
    type: 'keydown',
    key: 'Backspace',
    code: 'Backspace',
    altKey: false,
    ctrlKey: true,
    shiftKey: false,
    metaKey: false,
  })
  assert.equal(ctrlBack, WORD_BACKSPACE)
})

test('desktopWordEditSequence: Meta chords (Cmd/Win) are preserved for browser/OS shortcuts', () => {
  const seq = desktopWordEditSequence({
    type: 'keydown',
    key: 'ArrowLeft',
    code: 'ArrowLeft',
    altKey: true,
    ctrlKey: false,
    shiftKey: false,
    metaKey: true,
  })
  assert.equal(seq, null)
})

test('desktopWordEditSequence: ignores non-keydown events and plain typing', () => {
  assert.equal(
    desktopWordEditSequence({
      type: 'keyup',
      key: 'ArrowLeft',
      code: 'ArrowLeft',
      altKey: true,
      ctrlKey: false,
      shiftKey: false,
      metaKey: false,
    }),
    null,
  )
  assert.equal(
    desktopWordEditSequence({
      type: 'keydown',
      key: 'a',
      code: 'KeyA',
      altKey: false,
      ctrlKey: false,
      shiftKey: false,
      metaKey: false,
    }),
    null,
  )
})
