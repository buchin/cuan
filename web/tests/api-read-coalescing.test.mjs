import test from 'node:test'
import assert from 'node:assert/strict'
import fs from 'node:fs'
import ts from 'typescript'
const source = fs.readFileSync(new URL('../src/api/client.ts', import.meta.url),'utf8')
const {apiFetch}=await import('data:text/javascript;base64,'+Buffer.from(ts.transpileModule(source,{compilerOptions:{module:ts.ModuleKind.ESNext}}).outputText).toString('base64'))
test('simultaneous reads share work without caching stale data, writes, aborted calls or user identities', async()=>{
 const originalFetch=global.fetch,originalWindow=global.window,originalStorage=global.localStorage
 let token='owner-a',calls=[]
 global.window={};global.localStorage={getItem:()=>token,setItem:(_,value)=>{token=value},removeItem:()=>{token=''}}
 global.fetch=(url,options)=>new Promise(resolve=>calls.push({url,options,resolve}))
 const respond=(index,body={},status=200)=>calls[index].resolve({ok:status===200,status,statusText:'fixture',json:async()=>body})
 try {
  const first=apiFetch('/read'),second=apiFetch('/read')
  assert.equal(calls.length,1);respond(0,{value:1});assert.deepEqual(await first,await second)
  const fresh=apiFetch('/read');assert.equal(calls.length,2);respond(1,{value:2});assert.deepEqual(await fresh,{value:2})
  const a=apiFetch('/identity');token='owner-b';const b=apiFetch('/identity');assert.equal(calls.length,4);assert.equal(calls[2].options.headers.Authorization,'Bearer owner-a');assert.equal(calls[3].options.headers.Authorization,'Bearer owner-b');respond(2,{},401);respond(3,{});await assert.rejects(a);await b;assert.equal(token,'owner-b')
  const postA=apiFetch('/write',{method:'POST'}),postB=apiFetch('/write',{method:'POST'});assert.equal(calls.length,6);respond(4);respond(5);await Promise.all([postA,postB])
  const readA=apiFetch('/abort',{signal:new AbortController().signal}),readB=apiFetch('/abort',{signal:new AbortController().signal});assert.equal(calls.length,8);respond(6);respond(7);await Promise.all([readA,readB])
  const failedA=apiFetch('/error'),failedB=apiFetch('/error');respond(8,{error:'Unavailable'},503);await Promise.all([assert.rejects(failedA),assert.rejects(failedB)])
  const retry=apiFetch('/error');assert.equal(calls.length,10);respond(9,{recovered:true});await retry
 } finally {global.fetch=originalFetch;global.window=originalWindow;global.localStorage=originalStorage}
})

test('a stalled shared read times out and releases its entry without retrying a write',async()=>{
 const original={fetch:global.fetch,window:global.window,storage:global.localStorage,setTimeout:global.setTimeout}
 let calls=0
 global.window={};global.localStorage={getItem:()=> 'owner',setItem(){},removeItem(){}}
 global.setTimeout=(fn,delay,...args)=>original.setTimeout(fn,delay===20000?1:delay,...args)
 global.fetch=(_,options)=>{calls++;return new Promise((_,reject)=>options.signal.addEventListener('abort',()=>reject(new Error('timeout')),{once:true}))}
 try{
  await assert.rejects(apiFetch('/stalled'))
  assert.equal(calls,1)
  global.fetch=async()=>{calls++;return {ok:true,status:200,json:async()=>({recovered:true})}}
  assert.deepEqual(await apiFetch('/stalled'),{recovered:true})
  assert.equal(calls,2)
 }finally{global.fetch=original.fetch;global.window=original.window;global.localStorage=original.storage;global.setTimeout=original.setTimeout}
})
