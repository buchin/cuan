import { useState } from 'react'
import { createRoot } from 'react-dom/client'
import { Lock, Plus, Send } from 'lucide-react'
import '../../src/index.css'
import '../../src/features/inbox/chat-calm.css'
import { LanguageProvider } from '../../src/context/LanguageContext'
import { ComposerTasks } from '../../src/features/inbox/ComposerTasks'
import { Composer, ComposerAccessory, ComposerSendButton } from '../../src/components/ui/Composer'
import { ComposerInput } from '../../src/components/ui/ComposerInput'
import type { Task } from '../../src/api/types'
const initial = [
 {id:'active',created_at:'2026-11-08T09:00:00Z',title:'Perbaiki gate build rilis paralel',status:'doing', latest_attempt:{id:'fixture-attempt',status:'running'},latest_progress:'Perubahan sudah diterapkan. Sedang memeriksa hasil pengujian.'},
 {id:'queued',created_at:'2026-11-08T10:00:00Z',title:'Verifikasi production tanpa mengulang deployment yang sudah selesai',status:'queued'},
 {id:'done',created_at:'2026-11-08T08:00:00Z',title:'Rapikan balasan pelanggan',status:'done'},
] as Task[]
function Fixture() {
 const [tasks,setTasks]=useState(initial)
 const [version,setVersion]=useState(0)
 const [action,setAction]=useState('')
 const [visitor,setVisitor]=useState(false)
 const [whisper,setWhisper]=useState(false)
 return <LanguageProvider><main className="mx-auto flex h-dvh max-w-3xl flex-col p-4">
  <header className="border-b border-border/40 pb-3 text-lg font-medium">Cuan</header>
  <nav className="flex flex-wrap gap-3 py-3 text-xs"><button onClick={()=>setVisitor(v=>!v)}>Visitor chat</button><button onClick={()=>setTasks([...tasks])}>Poll update</button><button onClick={()=>setTasks(tasks.map(task=>task.id==='active'?{...task,review_state:'running'}:task))}>Start verification</button><button onClick={()=>setTasks([])}>Empty room</button><button onClick={()=>{setTasks(initial);setVersion(v=>v+1)}}>Switch room</button><button onClick={()=>setTasks(tasks.map(t=>({...t,review_state:'queued',status:'review',latest_attempt:undefined,launch:undefined})))}>Queued review</button></nav>
  <div className="flex-1 overflow-auto"><p className="max-w-md rounded-lg bg-muted/40 p-3 text-sm">Perbaiki proses rilisnya, lalu verifikasi sampai selesai ya.</p><p className="py-4 text-sm text-muted-foreground">Ane cek hasilnya dan lanjutkan verifikasinya.</p><output>{action}</output></div>
  <div data-inbox-composer className="wa-composer"><ComposerTasks key={version} tasks={tasks} modes={visitor && <div role="tablist" aria-label="Reply mode" className="inline-flex"><button role="tab" aria-selected={!whisper} onClick={()=>setWhisper(false)} className="composer-mode-tab inline-flex min-h-11 items-center px-3 text-xs font-medium">Balasan publik</button><button role="tab" aria-selected={whisper} onClick={()=>setWhisper(true)} className="composer-mode-tab inline-flex min-h-11 items-center gap-1.5 px-3 text-xs font-medium"><Lock className="h-3 w-3 text-amber-500"/>Whisper @cuan</button></div>} onDetails={t=>setAction('Details '+t.id)} onTerminal={t=>setAction('Terminal '+t.id)}/><Composer><ComposerAccessory><Plus size={18}/></ComposerAccessory><ComposerInput aria-label="Message" placeholder="Tulis pesan…" rows={1}/><ComposerSendButton aria-label="Send"><Send size={18}/></ComposerSendButton></Composer></div>
 </main></LanguageProvider>
}
createRoot(document.getElementById('root')!).render(<Fixture/> )
