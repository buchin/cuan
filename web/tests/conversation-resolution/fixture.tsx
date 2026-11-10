import { createRoot } from 'react-dom/client'
import { useEffect, useState } from 'react'
import { MemoryRouter } from 'react-router-dom'
import { InboxSubPanel } from '../../src/features/inbox/InboxSubPanel'
import { ComposerTasks } from '../../src/features/inbox/ComposerTasks'
import { ConversationResolutionAction } from '../../src/features/inbox/conversation-resolution'
import { LanguageProvider } from '../../src/context/LanguageContext'
import { Composer, ComposerSendButton } from '../../src/components/ui/Composer'
import { ComposerInput } from '../../src/components/ui/ComposerInput'
import type { ChatContact } from '../../src/api/chat'
import type { Task } from '../../src/api/types'
import { Send } from 'lucide-react'
import '../../src/index.css'
const at='2026-11-10T08:00:00Z'
const mobileChat = new URLSearchParams(location.search).has('chat')
const initial=[
 {id:'done',name:'Perbaiki pengiriman email',task_count:1,done_tasks:1,last_message:'Email pelanggan sudah terkirim dan terverifikasi.'},
 {id:'taskless',name:'Tanya jadwal layanan',task_count:0,last_message:'Terima kasih, informasinya sudah jelas.'},
 {id:'working',name:'Siapkan situs baru',task_count:1,active_tasks:1,last_message:'Sedang memeriksa hasil pekerjaan.'},
 {id:'resolved',name:'Rapikan halaman kontak',resolved:true,task_count:1,done_tasks:1,last_message:'Hasil sudah disimpan.'},
].map(c=>({...c,conversation_id:c.id,type:'agent',agent_id:'agent_cuan',avatar:'',status:'online',unread_count:2,updated_at:at,last_message_at:at,has_messages:true})) as ChatContact[]
function Fixture(){
 const [contacts,setContacts]=useState(initial)
 const [selected,setSelected]=useState('done')
 const current=contacts.find(c=>c.id===selected)!
 useEffect(()=>{const change=(event:Event)=>{const value=(event as CustomEvent).detail;setContacts(previous=>previous.map(c=>c.id===value.id?{...c,...value}:c))};window.addEventListener('cuan-conversation-resolution',change);return()=>window.removeEventListener('cuan-conversation-resolution',change)},[])
 return <LanguageProvider><MemoryRouter><main style={{display:"grid",height:"100dvh",gridTemplateColumns:innerWidth>=768?"360px minmax(0,1fr)":"minmax(0,1fr)"}}>
 <div style={{display:innerWidth<768&&mobileChat?"none":"block",minWidth:0,height:"100%"}}><InboxSubPanel contacts={contacts} onSelectContact={c=>setSelected(c.id)} activeContactId={selected}/></div>
 <section style={{display:innerWidth>=768||mobileChat?"flex":"none",flexDirection:"column",minWidth:0}} className="border-l border-border/40"><header className="flex min-h-16 items-center justify-between border-b border-border/40 px-4"><span>{current.name}</span><ConversationResolutionAction contact={current} compact/></header><div className="flex-1 p-6 text-sm">{current.last_message}</div><div className="px-4 pb-4"><ComposerTasks key={current.id} tasks={[{id:current.id,title:current.name,status:current.id==='working'?'doing':'done',created_at:at} as Task]} onDetails={()=>{}} onTerminal={()=>{}} resolution={<ConversationResolutionAction contact={current}/>}/><Composer><ComposerInput aria-label="Message" placeholder="Tulis pesan…"/><ComposerSendButton aria-label="Send"><Send size={18}/></ComposerSendButton></Composer></div></section>
 </main></MemoryRouter></LanguageProvider>
}
createRoot(document.getElementById('root')!).render(<Fixture/> )
