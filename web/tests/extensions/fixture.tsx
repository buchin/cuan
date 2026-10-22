// Test-only entry, not imported by the application or included in production.
import { useState } from 'react'
import { createRoot } from 'react-dom/client'
import { BrowserRouter } from 'react-router-dom'
import '../../src/index.css'
import { ChatMarkdown } from '../../src/features/inbox/ChatMarkdown'
import { ProjectWorkView } from '../../src/features/projects/ProjectWorkView'
import { installExampleExtension } from '../../src/extensions/example'
import { registerExtension, enableExtension, disableExtension, listExtensions, unregisterExtension } from '../../src/extensions/sdk'
import type { Project, Task } from '../../src/api/types'

installExampleExtension()
disableExtension('example.productivity')
registerExtension({manifest:{id:'test.renderer',name:'Broken renderer fixture',version:'1.0.0',api_version:1,capabilities:['markdown:broken']},fences:[{language:'broken',component:()=>{throw Error('Expected renderer fixture failure')}}]})
registerExtension({manifest:{id:'test.parser',name:'Broken parser fixture',version:'1.0.0',api_version:1,capabilities:['parser:broken']},parsers:[{id:'broken',plugin:()=>()=>{throw Error('Expected parser fixture failure')}}]})
const message=':cuan:\n\n```callout\nHello <b>reader</b>\n```\n\n```unknown\nUnsupported format remains readable\n```\n\n```broken\nRenderer original content\n```\n\n<script>window.fixtureInjected=1</script>'
function Fixture() {
 const [loading,setLoading]=useState(false)
 return <main className="mx-auto max-w-5xl space-y-4 p-3">
  <nav className="flex flex-wrap gap-3">
   <button onClick={()=>enableExtension('example.productivity')}>Enable example</button>
   <button onClick={()=>disableExtension('example.productivity')}>Disable example</button>
   <button onClick={()=>enableExtension('test.renderer')}>Break renderer</button>
   <button onClick={()=>enableExtension('test.parser')}>Break parser</button>
   <button onClick={()=>disableExtension('test.parser')}>Disable parser</button>
   <button onClick={()=>setLoading(value=>!value)}>Toggle loading</button>
   <button onClick={()=>{unregisterExtension('example.productivity');console.info(listExtensions().length)}}>Unregister example</button>
  </nav>
  <section aria-label="Markdown fixture"><ChatMarkdown content={message}/></section>
  <section aria-label="Project fixture" className="h-[600px]">
   <ProjectWorkView project={{id:'fixture',name:'Fixture'} as Project}
    tasks={['backlog','doing','review','blocked','done'].map((status,index)=>({id:'task'+index,project_id:'fixture',title:status+' task',status})) as Task[]}
    loading={loading} error="" onRetry={()=>{}}/>
  </section>
 </main>
}
createRoot(document.getElementById('root')!).render(<BrowserRouter><Fixture/></BrowserRouter>)
