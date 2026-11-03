import test from 'node:test'
import assert from 'node:assert/strict'
import fs from 'node:fs'
import ts from 'typescript'
const source=fs.readFileSync(new URL('../src/features/inbox/message-group.ts',import.meta.url),'utf8')
const compiled=ts.transpileModule(source,{compilerOptions:{module:ts.ModuleKind.ESNext}}).outputText
const {startsMessageGroup}=await import('data:text/javascript;base64,'+Buffer.from(compiled).toString('base64'))
const message={author_type:'agent',author_id:'cuan',author_name:'@cuan',sender_type:'operator',created_at:'2026-11-04T10:00:00+07:00'}
test('only first message of the same speaker group gets a tail',()=>{
 assert.equal(startsMessageGroup(message),true)
 assert.equal(startsMessageGroup({...message,created_at:'2026-11-04T10:04:59+07:00'},message),false)
 assert.equal(startsMessageGroup({...message,author_type:'user',author_id:'owner'},message),true)
 assert.equal(startsMessageGroup({...message,author_id:'other'},message),true)
 assert.equal(startsMessageGroup({...message,is_internal:true},message),true)
})
test('a five-minute gap, new day, invalid or out-of-order timestamps start a group',()=>{
 assert.equal(startsMessageGroup({...message,created_at:'2026-11-04T10:05:00+07:00'},message),true)
 assert.equal(startsMessageGroup({...message,created_at:'invalid'},message),true)
 assert.equal(startsMessageGroup({...message,created_at:'2026-11-04T09:59:00+07:00'},message),true)
 const d=new Date(2026,10,4,23,59,0)
 assert.equal(startsMessageGroup({...message,created_at:new Date(+d+120000).toISOString()},{...message,created_at:d.toISOString()}),true)
})
