import test from 'node:test'
import assert from 'node:assert/strict'
import fs from 'node:fs'
import ts from 'typescript'
const source = fs.readFileSync(new URL('../src/features/tasks/terminal/terminal-geometry.ts', import.meta.url), 'utf8')
const compiled = ts.transpileModule(source, {compilerOptions:{module:ts.ModuleKind.ESNext}}).outputText
const {terminalGrid,TerminalResizeGate} = await import('data:text/javascript;base64,'+Buffer.from(compiled).toString('base64'))

test('fractional mobile widths never allocate a cell outside the container',()=>{
  for(const width of [337.1,337.8,338.2,338.7]){
    const grid=terminalGrid(width,411.3,7.2,15.6)
    assert(grid.cols*7.2<=width)
    assert(grid.rows*15.6<=411.3)
  }
  assert.equal(terminalGrid(338.2,411.3,7.2,15.6).cols,terminalGrid(337.8,411.3,7.2,15.6).cols)
})
test('hidden or unmeasured panes do not send a provisional tiny grid',()=>{
  for(const args of [[0,400,7,15],[390,0,7,15],[390,400,0,15],[390,400,NaN,15],[1,400,7,15]])assert.equal(terminalGrid(...args),null)
})
test('unchanged observer notifications never generate additional PTY resizes',()=>{
  const gate=new TerminalResizeGate(),socket={},sent=[]
  for(let i=0;i<20;i++)gate.send(socket,{cols:45,rows:28},grid=>sent.push(grid))
  assert.deepEqual(sent,[{cols:45,rows:28}])
  gate.send(socket,{cols:45,rows:16},grid=>sent.push(grid))
  assert.equal(sent.length,2)
  gate.send({}, {cols:45,rows:16},grid=>sent.push(grid))
  assert.equal(sent.length,3,'a new connection must receive its own initial dimensions')
})
test('failed sends do not suppress the next resize attempt',()=>{
  const gate=new TerminalResizeGate(),socket={}
  assert.throws(()=>gate.send(socket,{cols:45,rows:28},()=>{throw Error('closed')}))
  let sent=0
  gate.send(socket,{cols:45,rows:28},()=>sent++)
  assert.equal(sent,1)
})
