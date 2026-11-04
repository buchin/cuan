import test from 'node:test'
import assert from 'node:assert/strict'
import fs from 'node:fs'
import ts from 'typescript'
const source = fs.readFileSync(new URL('../src/features/tasks/terminal/mobile-input.ts', import.meta.url), 'utf8')
const compiled = ts.transpileModule(source, { compilerOptions: { module: ts.ModuleKind.ESNext } }).outputText
const { TerminalHelperTextareaInputFilter, MobileInputNormalizer } = await import('data:text/javascript;base64,' + Buffer.from(compiled).toString('base64'))

test('SwiftKey growing composition sends each new character once', () => {
  const textarea = { value: '' }
  const filter = new TerminalHelperTextareaInputFilter(() => textarea)
  const sent = []
  for (const data of ['j', 'ji', 'jiu', 'jiuu', 'jiuuh']) {
    textarea.value = data
    const payload = filter.filter(data)
    if (payload !== null) sent.push(payload)
  }
  assert.equal(sent.join(''), 'jiuuh')
})

test('ordinary input retains keyboard sentence context and repeated characters', () => {
  const textarea = { value: 'a' }
  const filter = new TerminalHelperTextareaInputFilter(() => textarea)
  assert.equal(filter.filter('a'), 'a')
  assert.equal(textarea.value, 'a')
  assert.equal(filter.filter('a'), 'a')
  assert.equal(filter.filter(' '), ' ')
  assert.equal(filter.filter(' '), ' ')
})

test('autocorrection rewrites the suffix instead of duplicating the sentence', () => {
  const norm = new MobileInputNormalizer()
  assert.equal(norm.normalize('hello world '), 'hello world ')
  assert.equal(norm.normalize('hello world. '), '\x7f. ')
  assert.equal(norm.normalize('\r'), '\r')
  assert.equal(norm.normalize('next'), 'next')
})

test('reconnect retains pending IME context but a replacement session forgets it', () => {
  const textarea = { value: 'hello' }
  const filter = new TerminalHelperTextareaInputFilter(() => textarea)
  filter.sync()
  assert.equal(filter.filter('hello!'), '!')
  filter.reset()
  assert.equal(textarea.value, '')
  assert.equal(filter.filter('hello!'), 'hello!')
})

test('submit and interrupt clear stale helper text, arrow and control bytes pass through', () => {
  const textarea = { value: 'hello' }
  const filter = new TerminalHelperTextareaInputFilter(() => textarea)
  assert.equal(filter.filter('hello'), 'hello')
  for (const key of ['\x1b[D', '\x1b[A', '\t', '\x1b[<0;10;5M']) assert.equal(filter.filter(key), key)
  assert.equal(filter.filter('\n'), '\r')
  assert.equal(textarea.value, '')
  textarea.value = 'new'
  assert.equal(filter.filter('\x03'), '\x03')
  assert.equal(textarea.value, '')
})

test('coalesced typing is not erased as autocorrection', () => {
  const norm = new MobileInputNormalizer()
  assert.equal(norm.normalize('hel'), 'hel')
  assert.equal(norm.normalize('lo'), 'lo')
})
