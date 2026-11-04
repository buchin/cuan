import test from 'node:test'
import assert from 'node:assert/strict'
import fs from 'node:fs'
import ts from 'typescript'
const source=fs.readFileSync(new URL('../src/features/inbox/task-launch-text.ts',import.meta.url),'utf8')
const {withoutTaskLaunchAcknowledgement:clean}=await import('data:text/javascript;base64,'+Buffer.from(ts.transpileModule(source,{compilerOptions:{module:ts.ModuleKind.ESNext}}).outputText).toString('base64'))
test('remove generated queued/started paragraphs while preserving the actual update',()=>{
 for(const state of ['queued','started'])for(const quote of ['’',"'"]){
  const ack=`Work ${state}. I${quote}ll report progress and the result here.`
  assert.equal(clean(ack,true),'')
  assert.equal(clean('Saya lanjutkan pemeriksaannya.\n\n'+ack,true),'Saya lanjutkan pemeriksaannya.')
  assert.equal(clean(ack,false),ack)
  assert.equal(clean(ack,true,true),ack)
 }
})
test('keep admission failures, capacity explanations and ordinary quoted text',()=>{
 for(const text of ['The work is saved, but could not start. GitHub access is unavailable.','Waiting for available AI capacity.','Explain "Work queued. I’ll report progress and the result here." please.'])assert.equal(clean(text,true),text)
})
