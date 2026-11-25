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
  const callback = new Function('disposed','socket','ws','term','following',`let outputSequence=0;const acknowledgeOutput=false;${code}; return callback`)(false,{current:ws},ws,term,{current:false})
  const output = 'live output\r\n\x1b[2K{"type":"stdin","data":"valid terminal output"}\r\n日本語🙂'
  const bytes = new TextEncoder().encode(output)
  // Deliberately split inside multibyte UTF-8. Each PTY byte must survive;
  // xterm's streaming parser owns decoding rather than each WS callback.
  const split = bytes.length - 2
  callback({data:bytes.slice(0,split).buffer})
  callback({data:bytes.slice(split).buffer})
  assert(writes.every(x=>x instanceof Uint8Array))
  assert.deepEqual(Buffer.concat(writes.map(x=>Buffer.from(x))),Buffer.from(bytes))
})

test('output credit waits for xterm parsing and yields without touching pending input', () => {
  const callbacksPending=[],scheduled=[],sent=[]
  const ws={readyState:1,send:data=>sent.push(JSON.parse(data))},socket={current:null};socket.current=ws
  const term={current:{textarea:{value:'pending composition'},write(data,done){callbacksPending.push(done)}}}
  const code=ts.transpileModule(`const callback = ${callbacks.get('ws.onmessage')}`,{compilerOptions:{target:ts.ScriptTarget.ES2022}}).outputText
  const callback=new Function('disposed','socket','ws','term','following','setTimeout','WebSocket',`let outputSequence=0;const acknowledgeOutput=true;${code};return callback`)(false,socket,ws,term,{current:false},fn=>scheduled.push(fn),{OPEN:1})
  callback({data:new TextEncoder().encode('output').buffer})
  assert.equal(sent.length,0)
  callbacksPending.shift()()
  assert.equal(sent.length,0,'credit must yield to input before admitting more output')
  scheduled.shift()()
  assert.deepEqual(sent,[{type:'output_ack',sequence:1}])
  callback({data:new TextEncoder().encode('more').buffer});callbacksPending.shift()()
  socket.current={};scheduled.shift()()
  assert.equal(sent.length,1,'replaced socket cannot receive stale credit')
  assert.equal(term.current.textarea.value,'pending composition')
})

test('initial terminal attach bounds seed_max_bytes to avoid freeze', () => {
  assert.match(source, /url\.searchParams\.set\('seed_max_bytes',\s*'16384'\)/)
})

test('terminal lifecycle effect is decoupled from attempt id', () => {
  assert.match(source, /}, \[taskId, isDedicatedTerminal\]\)/)
  assert.doesNotMatch(source, /}, \[taskId, attempt\.id\]\)/)
})

test('input buffer generation remains stable across attempt updates', () => {
  assert.match(source, /const inputGeneration = taskId/)
  assert.doesNotMatch(source, /const inputGeneration = taskId \+ ':' \+ attemptId/)
})

test('dedicated Project Terminal reports connecting while retrying and disconnected when exhausted', async () => {
  const states = []
  const ws = {}
  const socket = { current: ws }
  const term = { current: { options: { disableStdin: false } } }
  const code = ts.transpileModule(`const callback = ${callbacks.get('ws.onclose')}`, {
    compilerOptions: { target: ts.ScriptTarget.ES2022 },
  }).outputText
  const makeCallback = (retryCountVal) => new Function(
    'disposed', 'socket', 'ws', 'term', 'heartbeatTimer', 'interactive', 'isTaskActiveRef',
    'setInput', 'reconnectNoticeTimer', 'reconnectTimer', 'retryCount', 'isReconnecting',
    'isDedicated', 'probeTerminalAvailability', 'setConnection', 'setNotice', 'setError',
    'refreshRef', 'connectSocket', 'pollTimerRef', 'pollNextLogsRef', 'setTimeout', 'clearTimeout',
    'clearInterval', 't',
    `${code}; return callback`,
  )(
    false, socket, ws, term, { current: null }, { current: true }, { current: true },
    () => {}, { current: null }, { current: null }, { current: retryCountVal }, { current: false },
    true, async () => 400, (s) => states.push(s), () => {}, () => {},
    { current: () => {} }, () => {}, { current: null }, { current: () => {} },
    () => 1, () => {}, () => {}, (k) => k,
  )

  await makeCallback(0)({ code: 1006 })
  await makeCallback(20)({ code: 1006 })
  assert.deepEqual(states, ['connecting', 'disconnected'])
})
