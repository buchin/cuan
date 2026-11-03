import test from 'node:test'
import assert from 'node:assert/strict'
import fs from 'node:fs'
import ts from 'typescript'
const source = fs.readFileSync(new URL('../src/features/tasks/terminal-input-buffer.ts', import.meta.url), 'utf8')
const compiled = ts.transpileModule(source, { compilerOptions: { module: ts.ModuleKind.ESNext } }).outputText
const { TerminalInputBuffer } = await import('data:text/javascript;base64,' + Buffer.from(compiled).toString('base64'))

test('reconnect delivers unsent input once and never repeats delivered bytes', () => {
  const queue = new TerminalInputBuffer()
  queue.bind('task:a')
  const sent = []
  queue.write('task:a', 'first', value => sent.push(value))
  queue.write('task:a', 'second')
  queue.write('task:a', '\n')
  queue.flush('task:a', value => sent.push(value))
  queue.flush('task:a', value => sent.push(value))
  assert.deepEqual(sent, ['first', 'second\n'])
})

test('failed handoff retains bytes but a replacement attempt cannot receive them', () => {
  const queue = new TerminalInputBuffer()
  queue.bind('task:a')
  assert.equal(queue.write('task:a', 'pending', () => { throw Error('closed') }), false)
  const sent = []
  queue.flush('task:a', value => sent.push(value))
  assert.deepEqual(sent, ['pending'])
  queue.write('task:a', 'old attempt')
  queue.bind('task:b')
  assert.equal(queue.write('task:a', 'stale'), false)
  queue.flush('task:b', value => sent.push(value))
  queue.write('task:b', 'new', value => sent.push(value))
  assert.deepEqual(sent, ['pending', 'new'])
})

test('overflow rejects new input without losing queued bytes', () => {
  const queue = new TerminalInputBuffer()
  queue.bind('a')
  assert.equal(queue.write('a', 'x'.repeat(65536)), true)
  assert.equal(queue.write('a', 'overflow'), false)
  let delivered = ''
  queue.flush('a', value => { delivered = value })
  assert.equal(delivered, 'x'.repeat(65536))
})
