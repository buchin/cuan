// Targeted transport checks; runs with Node and the existing TypeScript compiler.
const fs = require('node:fs')
const path = require('node:path')
const vm = require('node:vm')
const assert = require('node:assert/strict')
const ts = require('typescript')
const source = ts.transpileModule(fs.readFileSync(path.join(__dirname,'chat.ts'),'utf8'), {compilerOptions:{module:ts.ModuleKind.CommonJS,target:ts.ScriptTarget.ES2020}}).outputText
function load(fetch) {
 const timers = new Map();let next=0
 class ApiError extends Error {constructor(message,status){super(message);this.status=status}}
 const context={exports:{},require:()=>({getAuthToken:()=> 'fixture-token',ApiError}),fetch,AbortController,TextDecoder,setTimeout:fn=>{timers.set(++next,fn);return next},clearTimeout:id=>timers.delete(id)}
 vm.runInNewContext(source,context);return{api:context.exports,timers}
}
;(async()=>{
 const data={messages:[{message:'Pesan ☕'}],requests:[]}
 const encoded=new TextEncoder().encode('event: snapshot\r\ndata: '+JSON.stringify(data)+'\r\n\r\n')
 let requested
 const transport=load(async(url,options)=>{requested={url,options};return new Response(new ReadableStream({start(c){for(const b of encoded)c.enqueue(Uint8Array.of(b));c.close()}}),{headers:{'content-type':'text/event-stream'}})})
 const received=[];await transport.api.streamConversation('room fixture',new AbortController().signal,x=>received.push(x))
 assert.equal(received.length,1);assert.equal(received[0].messages[0].message,'Pesan ☕');assert.equal(transport.timers.size,0)
 assert.equal(requested.options.headers.Authorization,'Bearer fixture-token');assert(!requested.url.includes('fixture-token'))
 let started
 const opened=new Promise(resolve=>started=resolve)
 const silent=load(async(_url,options)=>new Response(new ReadableStream({start(c){options.signal.addEventListener('abort',()=>c.error(new Error('silent stream aborted')));started()}}),{headers:{'content-type':'text/event-stream'}}))
 const work=silent.api.streamConversation('room',new AbortController().signal,()=>{});await opened;for(const fn of [...silent.timers.values()])fn()
 await assert.rejects(work,/silent stream aborted/);assert.equal(silent.timers.size,0)
 console.log('PASS: chunked UTF-8/CRLF snapshot, authenticated URL-safe transport, idle reconnect abort, timer cleanup')
})().catch(e=>{console.error(e.message);process.exitCode=1})
