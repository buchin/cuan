import test from 'node:test'
import assert from 'node:assert/strict'
import fs from 'node:fs'
import ts from 'typescript'
const source=fs.readFileSync(new URL('../src/features/inbox/task-message-bindings.ts',import.meta.url),'utf8')
const {taskMessageBindings:bind}=await import('data:text/javascript;base64,'+Buffer.from(ts.transpileModule(source,{compilerOptions:{module:ts.ModuleKind.ESNext,target:ts.ScriptTarget.ES2022}}).outputText).toString('base64'))
const tasks=[{id:'task_a',source_message_id:'user'},{id:'task_b'}]
const message=(id,author_type,content,extra={})=>({id,uuid:id,author_type,content,...extra})
test('user source/reference cannot consume the only incoming task preview',()=>{
 const user=message('user','user','Please inspect /tasks/task_a')
 assert.equal(bind(tasks,[user],[],false).size,0)
 const reply=message('reply','agent','I will inspect it')
 assert.equal(bind(tasks,[user,reply],[{message_id:'user',reply_id:'reply'}],false).get('task_a'),'reply')
 const quote=message('quote','user','What happened to /tasks/task_a?')
 assert.equal(bind(tasks,[user,reply,quote],[{message_id:'user',reply_id:'reply'}],false).get('task_a'),'reply')
})
test('latest actual system update owns its task preview; synthetic prompts never do',()=>{
 const messages=[message('agent','agent','[View work](/tasks/task_a)'),message('system','system','Could not start. [View work](/tasks/task_a)'),message('review','system','Review /tasks/task_a',{author_id:'cuan_work_review'})]
 assert.equal(bind(tasks,messages,[],false).get('task_a'),'system')
})
test('one reply can attach several tasks without losing the second preview',()=>{
 const result=bind(tasks,[message('reply','agent','[First](/tasks/task_a) [Second](/tasks/task_b)')],[],false)
 assert.deepEqual([...result],[['task_a','reply'],['task_b','reply']])
})
test('customer task attachments require internal agent authorship',()=>{
 const publicMessages=[message('public','agent','/tasks/task_a'),message('system','system','/tasks/task_a')]
 assert.equal(bind(tasks,publicMessages,[],true).size,0)
 assert.equal(bind(tasks,[...publicMessages,message('internal','agent','/tasks/task_a',{is_internal:true})],[],true).get('task_a'),'internal')
})
