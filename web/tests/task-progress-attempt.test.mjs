import test from 'node:test'
import assert from 'node:assert/strict'
import fs from 'node:fs'
import ts from 'typescript'
const source=fs.readFileSync(new URL('../src/features/tasks/task-ui.ts',import.meta.url),'utf8')
const {taskLiveActivityText}=await import('data:text/javascript;base64,'+Buffer.from(ts.transpileModule(source,{compilerOptions:{module:ts.ModuleKind.ESNext}}).outputText).toString('base64'))
test('current activity never borrows old or unbound attempt reports',()=>{
 const old={attempt_id:'old',activity_type:'agent_report',new_status:'progress',message:'Audit lama selesai'}
 assert.equal(taskLiveActivityText({latest_attempt:{id:'new'},activities:[old]},'Aktif'),'Aktif')
 assert.equal(taskLiveActivityText({activities:[old]},'Aktif'),'Aktif')
 assert.equal(taskLiveActivityText({latest_attempt:{id:'new'},activities:[{...old,attempt_id:undefined}]},'Aktif'),'Aktif')
 assert.equal(taskLiveActivityText({latest_attempt:{id:'new'},activities:[{...old,attempt_id:'new',message:'Memeriksa data terbaru'},old]},'Aktif'),'Memeriksa data terbaru')
 assert.equal(taskLiveActivityText({latest_attempt:{id:'new'},latest_progress:'Progress server terbaru',activities:[old]},'Aktif'),'Progress server terbaru')
})
