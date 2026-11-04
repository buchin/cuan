import test from 'node:test'
import assert from 'node:assert/strict'
import fs from 'node:fs'
import ts from 'typescript'

// Execute the actual component callbacks, including their early guards. Delayed
// events from a replaced socket must never reach terminal or UI state mutations.
const source = fs.readFileSync(new URL('../src/features/tasks/TaskTerminal.tsx', import.meta.url), 'utf8')
const ast = ts.createSourceFile('TaskTerminal.tsx', source, ts.ScriptTarget.Latest, true, ts.ScriptKind.TSX)
const callbacks = new Map()
function visit(node) {
  if (ts.isBinaryExpression(node) && /^ws\.on(message|error|close)$/.test(node.left.getText(ast))) {
    callbacks.set(node.left.getText(ast), node.right.getText(ast))
  }
  ts.forEachChild(node, visit)
}
visit(ast)

for (const name of ['onmessage', 'onerror', 'onclose']) {
  test(`replaced socket ${name} cannot reset, write, disable input or reconnect`, async () => {
    const oldSocket = {}, activeSocket = {}
    const socket = { current: activeSocket }
    const term = { current: new Proxy({}, { get() { throw Error('obsolete socket accessed terminal') } }) }
    const code = ts.transpileModule(`const callback = ${callbacks.get('ws.' + name)}`, {
      compilerOptions: { target: ts.ScriptTarget.ES2022 },
    }).outputText
    const callback = new Function('disposed', 'socket', 'ws', 'term', `${code}; return callback`)(false, socket, oldSocket, term)
    await callback({ data: JSON.stringify({type:'connected',session_id:'old'}), code:1000 })
    await callback({ data: new ArrayBuffer(8), code:1012 })
    assert.equal(socket.current, activeSocket)
  })
}

test('active socket output still reaches the terminal', () => {
  const ws = {}, writes = []
  const term = {current:{write(data, done) {writes.push(data);done?.()}}}
  const code = ts.transpileModule(`const callback = ${callbacks.get('ws.onmessage')}`, {
    compilerOptions: { target: ts.ScriptTarget.ES2022 },
  }).outputText
  const callback = new Function('disposed','socket','ws','term','following',`${code}; return callback`)(false,{current:ws},ws,term,{current:false})
  callback({data:new TextEncoder().encode('live output').buffer})
  assert.equal(new TextDecoder().decode(writes[0]),'live output')
})
