import { InboxActivity } from '../../src/features/inbox/InboxActivity'
import { TaskTimeline } from '../../src/features/tasks/TaskTimeline'
import type { TaskDetail } from '../../src/api/types'
import { ConversationItem, type ConversationItemProps } from '../../src/features/inbox/InboxSubPanel'
import type { ChatContact } from '../../src/api/chat'
import { useRef, useState } from 'react'
import { MemoryRouter } from 'react-router-dom'
import { TaskItemCard } from '../../src/features/tasks/TaskItemCard'
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
const archive = {loaded:true, unreadCount:()=>0, markRead:async()=>{}, archive:async()=>{}} as unknown as ConversationItemProps['archive']
const contacts = [
 {id:'single',name:'Satu task selesai',task_count:1,done_tasks:1},
 {id:'partial',name:'Sebagian selesai',task_count:2,done_tasks:1},
 {id:'canceled',name:'Task dibatalkan',task_count:1,done_tasks:0,canceled_tasks:1},
].map(c=>({...c,conversation_id:c.id,type:'agent',agent_id:'agent_cuan',avatar:'',status:'online',unread_count:0,updated_at:'2026-11-08T09:00:00Z',last_message:'Hasil pekerjaan tersimpan.'})) as ChatContact[]
function Fixture() {
 const [tasks,setTasks]=useState(initial)
 const detailRef=useRef<HTMLDivElement>(null)
 const [version,setVersion]=useState(0)
 const [action,setAction]=useState('')
 const [visitor,setVisitor]=useState(false)
 const [whisper,setWhisper]=useState(false)
 const [rows,setRows]=useState(false)
 return <LanguageProvider><main className="mx-auto flex h-dvh max-w-3xl flex-col p-4">
  <header className="border-b border-border/40 pb-3 text-lg font-medium">Cuan</header>
  <nav className="flex flex-wrap gap-3 py-3 text-xs"><button onClick={()=>setRows(v=>!v)}>Task rows</button><button onClick={()=>setTasks(tasks.map(task=>({...task,status:'blocked',review_action:'needs_owner',review_finding:'Pilih nama domain yang ingin digunakan agar pekerjaan bisa dilanjutkan.',review_state:undefined,latest_attempt:{id:'finished',status:'succeeded'},launch:undefined})))}>Needs owner</button><button onClick={()=>setTasks(tasks.map(task=>({...task,status:'blocked',review_action:'continue',review_finding:'Bukti tampilan belum terbaca. Cuan perlu memperbaiki pemeriksaannya.',review_state:undefined,latest_attempt:{id:'finished',status:'succeeded'},launch:undefined})))}>Work paused</button><button onClick={()=>setTasks(tasks.map(task=>({...task,status:'blocked',review_action:'wait',review_finding:'Layanan hosting sedang tidak tersedia. Menunggu layanan kembali normal.',review_state:undefined,latest_attempt:{id:'finished',status:'succeeded'},launch:undefined})))}>Prerequisite</button><button onClick={()=>setTasks(tasks.map(task=>({...task,status:'done',review_state:undefined})))}>Complete work</button><button onClick={()=>setVisitor(v=>!v)}>Visitor chat</button><button onClick={()=>setTasks([...tasks])}>Poll update</button><button onClick={()=>setTasks(tasks.map(task=>task.id==='active'?{...task,review_state:'running'}:task))}>Start verification</button><button onClick={()=>setTasks([])}>Empty room</button><button onClick={()=>{setTasks(initial);setVersion(v=>v+1)}}>Switch room</button><button onClick={()=>setTasks(tasks.map(t=>({...t,review_state:'queued',status:'review',latest_attempt:undefined,launch:undefined})))}>Queued review</button></nav>
  <div className="flex-1 overflow-auto"><p className="max-w-md rounded-lg bg-muted/40 p-3 text-sm">Perbaiki proses rilisnya, lalu verifikasi sampai selesai ya.</p><p className="py-4 text-sm text-muted-foreground">Ane cek hasilnya dan lanjutkan verifikasinya.</p><section aria-label="Response fixture"><MemoryRouter><InboxActivity request={{request_id:'timeline-fixture',message_id:'fixture',reply_id:'reply',state:'running',created_at:new Date().toISOString(),steps:[{seq:1,kind:'accepted',text:'Message received.',at:'2026-11-08T09:00:00Z'},{seq:2,kind:'runtime_tool_finished',text:'Pemeriksaan selesai.\nSemua pengujian terkait sudah lulus.',at:'2026-11-08T09:01:00Z'}]}} onCancel={async()=>{}} /></MemoryRouter></section><output>{action}</output>{action==='Details active' && <div ref={detailRef} className="max-h-80 overflow-y-auto" data-fixture-task-detail><TaskTimeline scrollRef={detailRef} task={{...tasks[0],activities:[{id:'created',activity_type:'created',created_at:'2026-11-08T09:00:00Z',message:''},{id:'report',activity_type:'agent_report',new_status:'review',created_at:'2026-11-08T09:10:00Z',message:'Perubahan sudah diuji. **Semua pemeriksaan lulus.**'}],attempts:[]} as TaskDetail}/></div>}<section aria-label="Inbox completion"><MemoryRouter>{contacts.map(contact=><ConversationItem key={contact.id} contact={contact} isSelected={false} isLast={false} now={Date.now()} archive={archive} showArchived={false} onSelectContact={()=>{}}/>)}</MemoryRouter></section>{rows && <section aria-label="Task list"><MemoryRouter>{tasks.map(task=><TaskItemCard key={task.id} task={task} variant="row" managedUpdates/>)}</MemoryRouter></section>}</div>
  <div data-inbox-composer className="wa-composer"><ComposerTasks key={version} tasks={tasks} modes={visitor && <div role="tablist" aria-label="Reply mode" className="inline-flex"><button role="tab" aria-selected={!whisper} onClick={()=>setWhisper(false)} className="composer-mode-tab inline-flex min-h-11 items-center px-3 text-xs font-medium">Balasan publik</button><button role="tab" aria-selected={whisper} onClick={()=>setWhisper(true)} className="composer-mode-tab inline-flex min-h-11 items-center gap-1.5 px-3 text-xs font-medium"><Lock className="h-3 w-3 text-amber-500"/>Whisper @cuan</button></div>} onDetails={t=>setAction('Details '+t.id)} onTerminal={t=>setAction('Terminal '+t.id)}/><Composer><ComposerAccessory><Plus size={18}/></ComposerAccessory><ComposerInput aria-label="Message" placeholder="Tulis pesan…" rows={1}/><ComposerSendButton aria-label="Send"><Send size={18}/></ComposerSendButton></Composer></div>
 </main></LanguageProvider>
}
createRoot(document.getElementById('root')!).render(<Fixture/> )
