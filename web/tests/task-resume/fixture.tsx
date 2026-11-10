import { createRoot } from 'react-dom/client'
import { MemoryRouter } from 'react-router-dom'
import { LanguageProvider } from '../../src/context/LanguageContext'
import { InboxWorkPanel } from '../../src/features/inbox/InboxWorkPanel'
import type { Task } from '../../src/api/types'
import '../../src/index.css'
import '../../src/features/inbox/chat-calm.css'
const task = { id:'paused', title:'Verifikasi hasil perbaikan rilis', status:'blocked', source_room_id:'room', review_action:'continue', latest_attempt:{id:'finished',status:'failed'}, created_at:'2026-11-10T08:00:00Z' } as Task
createRoot(document.getElementById('root')!).render(<LanguageProvider><MemoryRouter><main className="h-dvh"><InboxWorkPanel task={task} view="details" onBack={()=>{}} /></main></MemoryRouter></LanguageProvider>)
