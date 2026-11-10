import { useState } from 'react'
import { createRoot } from 'react-dom/client'
import { MemoryRouter } from 'react-router-dom'
import { LanguageProvider } from '../../src/context/LanguageContext'
import { ConversationItem, type ConversationItemProps } from '../../src/features/inbox/InboxSubPanel'
import { ComposerTasks } from '../../src/features/inbox/ComposerTasks'
import { Composer, ComposerSendButton } from '../../src/components/ui/Composer'
import { ComposerInput } from '../../src/components/ui/ComposerInput'
import type { ChatContact } from '../../src/api/chat'
import type { Task } from '../../src/api/types'
import { Send } from 'lucide-react'
import '../../src/index.css'
import '../../src/features/inbox/chat-calm.css'
const at='2026-11-10T08:50:00Z'
const initial={id:'release',source_room_id:'room',title:'Rapikan indikator persetujuan',latest_progress:'Pengujian perubahan sudah lulus, tetapi proses rilisnya belum tuntas.',status:'queued',created_at:at,latest_attempt:{id:'verifier',status:'succeeded'},launch:{state:'queued',kind:'native_release',available_at:at}} as Task
const archive={loaded:true,unreadCount:()=>0,markRead:async()=>{},archive:async()=>{}} as unknown as ConversationItemProps['archive']
function Fixture(){
 const [task,setTask]=useState(initial)
 const contacts=[{id:'queue',name:'Perapihan indikator persetujuan',queued_tasks:1},{id:'check',name:'Pemeriksaan hasil',review_tasks:1},{id:'owner',name:'Pilihan domain',failed_tasks:1},{id:'done',name:'Balasan pelanggan',done_tasks:1}].map(c=>({...c,conversation_id:c.id,type:'agent',agent_id:'agent_cuan',status:'online',task_count:1,updated_at:at,last_message:'Hasil pekerjaan tersimpan.',avatar:'',unread_count:0})) as ChatContact[]
 return <LanguageProvider><MemoryRouter><main style={{display:'grid',gridTemplateColumns:innerWidth>=900?'340px minmax(0,1fr)':'minmax(0,1fr)'}} className="mx-auto h-dvh max-w-6xl">
 <section className="overflow-auto border-r border-border/40"><header className="p-4 font-medium">Inbox</header>{contacts.map(contact=><ConversationItem key={contact.id} contact={contact} isSelected={contact.id==='queue'} isLast={false} now={Date.now()} archive={archive} showArchived={false} onSelectContact={()=>{}}/>)}</section>
 <section className="flex min-h-0 flex-col px-4"><header className="py-4 text-sm font-medium">Perapihan indikator persetujuan</header><nav className="flex gap-3 text-xs"><button onClick={()=>setTask(initial)}>Queued release</button><button onClick={()=>setTask({...initial,status:'doing',latest_attempt:{id:'publisher',status:'running'},launch:{state:'started',kind:'native_release',available_at:at}})}>Deploy now</button></nav><div className="min-h-0 flex-1 py-6 text-sm text-muted-foreground">Perubahan sudah diperiksa dan siap dirilis.</div><div className="wa-composer pb-4"><ComposerTasks tasks={[task]} onDetails={()=>{}} onTerminal={()=>{}}/><Composer><ComposerInput aria-label="Message" placeholder="Tulis pesan…"/><ComposerSendButton aria-label="Send"><Send size={18}/></ComposerSendButton></Composer></div></section>
 </main></MemoryRouter></LanguageProvider>
}
createRoot(document.getElementById('root')!).render(<Fixture/> )
