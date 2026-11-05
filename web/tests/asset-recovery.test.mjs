import test from 'node:test'
import assert from 'node:assert/strict'
import fs from 'node:fs'
import ts from 'typescript'
const source=fs.readFileSync(new URL('../src/lib/asset-recovery.ts',import.meta.url),'utf8')
const compiled=ts.transpileModule(source,{compilerOptions:{module:ts.ModuleKind.ESNext}}).outputText
const {recoverStaleAsset,isAssetLoadFailure}=await import('data:text/javascript;base64,'+Buffer.from(compiled).toString('base64'))
const failure=new TypeError('Failed to fetch dynamically imported module: /assets/Tasks-old.js')
test('stale entry recovers once without touching persisted account or draft data',()=>{
 const values=new Map([['cuan_token','saved-token'],['studio-draft','unsent idea']]);let reloads=0
 const storage={getItem:k=>values.get(k)||null,setItem:(k,v)=>values.set(k,v)}
 assert.equal(recoverStaleAsset(failure,'/assets/index-old.js',true,storage,()=>reloads++),true)
 assert.equal(recoverStaleAsset(failure,'/assets/index-old.js',true,storage,()=>reloads++),false)
 assert.equal(reloads,1);assert.equal(values.get('cuan_token'),'saved-token');assert.equal(values.get('studio-draft'),'unsent idea')
 assert.equal(recoverStaleAsset(failure,'/assets/index-new.js',true,storage,()=>reloads++),true)
})
test('offline, application bugs and inaccessible storage never cause a reload loop',()=>{
 const storage={getItem:()=>{throw Error('denied')},setItem:()=>{throw Error('denied')}}
 const reload=()=>assert.fail('unexpected reload')
 assert.equal(recoverStaleAsset(failure,'entry',false,storage,reload),false)
 assert.equal(recoverStaleAsset(Error('Invalid hook call'),'entry',true,storage,reload),false)
 assert.equal(recoverStaleAsset(failure,'entry',true,storage,reload),false)
 assert.equal(isAssetLoadFailure(new TypeError('Importing a module script failed.')),true)
})
